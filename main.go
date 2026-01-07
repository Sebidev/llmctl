package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"llmctl/counter"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

func readAllStdin() (string, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		return "", nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func tailString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// grobe, absichtlich simple Schätzung (~4 chars / token)
func approxTokens(s string) int64 {
	return int64(len(s) / 4)
}

func main() {
	var (
		model       = flag.String("model", getenv("LLM_MODEL", "gpt-5.2"), "Model name")
		system      = flag.String("system", os.Getenv("LLM_SYSTEM"), "System prompt")
		contextFile = flag.String("context", "", "Optional context file")
		tail        = flag.Int("tail", 12000, "Max chars from context")
		timeout     = flag.Duration("timeout", 5*time.Minute, "Request timeout")
		baseURL     = flag.String("base-url", os.Getenv("OPENAI_BASE_URL"), "OpenAI-compatible base URL")
		verbose     = flag.Bool("v", false, "Verbose (stderr only)")
	)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `llmctl – minimal LLM CLI

USAGE:
  llmctl [options] "prompt"
  llmctl counter
  llmctl counter reset

OPTIONS:
  --model <name>
  --context <file>
  --tail <chars>
  --timeout <duration>
  --base-url <url>
  -v

ENV:
  OPENAI_API_KEY
  OPENAI_BASE_URL
  LLM_MODEL
  LLM_SYSTEM
`)
	}

	flag.Parse()

	// ---- subcommand: counter ----
	if len(os.Args) > 1 && os.Args[1] == "counter" {
		runCounter(os.Args[2:])
		return
	}

	promptArgs := strings.TrimSpace(strings.Join(flag.Args(), " "))
	stdinText, err := readAllStdin()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdin read error:", err)
		os.Exit(2)
	}

	if promptArgs == "" && strings.TrimSpace(stdinText) == "" {
		fmt.Fprintln(os.Stderr, "usage: llmctl [options] \"your prompt\"")
		os.Exit(2)
	}

	// ---- context ----
	var ctxBuf bytes.Buffer
	if *contextFile != "" {
		b, err := os.ReadFile(*contextFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "context read error:", err)
			os.Exit(2)
		}
		ctxBuf.WriteString(tailString(string(b), *tail))
	}

	// ---- build prompt ----
	var userBuf strings.Builder
	if ctxBuf.Len() > 0 {
		userBuf.WriteString("CONTEXT:\n")
		userBuf.Write(ctxBuf.Bytes())
		userBuf.WriteString("\n\n")
	}
	if strings.TrimSpace(stdinText) != "" {
		userBuf.WriteString("STDIN:\n")
		userBuf.WriteString(stdinText)
		userBuf.WriteString("\n\n")
	}
	if promptArgs != "" {
		userBuf.WriteString(promptArgs)
	}

	// ---- token estimate (prompt) ----
	promptTokenEstimate := approxTokens(userBuf.String())

	// ---- client ----
	opts := []option.RequestOption{}
	if *baseURL != "" {
		opts = append(opts, option.WithBaseURL(*baseURL))
	}
	client := openai.NewClient(opts...)

	var msgs []openai.ChatCompletionMessageParamUnion
	if strings.TrimSpace(*system) != "" {
		msgs = append(msgs, openai.SystemMessage(*system))
	}
	msgs = append(msgs, openai.UserMessage(userBuf.String()))

	reqCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	stream := client.Chat.Completions.NewStreaming(reqCtx, openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(*model),
		Messages: msgs,
	})

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	var completionChars int64

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			if delta := chunk.Choices[0].Delta.Content; delta != "" {
				completionChars += int64(len(delta))
				io.WriteString(w, delta)
			}
		}
	}

	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "stream error:", err)
		os.Exit(1)
	}

	fmt.Fprintln(w) // newline safety

	// ---- token estimate (completion) ----
	completionTokenEstimate := completionChars / 4

	// ---- persist counter ----
	state, err := counter.Load()
	if err == nil {
		state = counter.Add(state, promptTokenEstimate, completionTokenEstimate)
		_ = counter.Save(state)
	}

	if *verbose {
		fmt.Fprintf(
			os.Stderr,
			"[llmctl] token estimate ~ prompt:%d completion:%d total:%d\n",
			promptTokenEstimate,
			completionTokenEstimate,
			promptTokenEstimate+completionTokenEstimate,
		)
	}
}
