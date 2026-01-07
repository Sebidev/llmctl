package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"llmctl/counter"
	"os"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

func readAllStdin() (string, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	// Nur lesen, wenn gepiped wurde
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
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func main() {
	var (
		model       = flag.String("model", getenv("LLM_MODEL", "gpt-5.2"), "Model name")
		system      = flag.String("system", os.Getenv("LLM_SYSTEM"), "System prompt")
		contextFile = flag.String("context", "", "Optional context file to append")
		tail        = flag.Int("tail", 12000, "Max chars to take from context file (tail)")
		timeout     = flag.Duration("timeout", 5*time.Minute, "Request timeout")
		baseURL     = flag.String("base-url", os.Getenv("OPENAI_BASE_URL"), "Optional base URL (OpenAI-compatible)")
		verbose     = flag.Bool("v", false, "Verbose (stderr only)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `llmctl – minimal LLM CLI

USAGE:
llmctl [options] "prompt"
llmctl counter
llmctl counter reset

OPTIONS:
--model <name>        Model name (default from LLM_MODEL)
--context <file>      Append context file (tail)
--tail <chars>        Max chars from context (default 12000)
--timeout <duration> Request timeout
--base-url <url>      OpenAI-compatible base URL
-v                    Verbose (stderr only)

ENVIRONMENT:
OPENAI_API_KEY
OPENAI_BASE_URL
LLM_MODEL
LLM_SYSTEM

SUBCOMMANDS:
counter               Show total token usage
counter reset         Reset token counter

PIPE EXAMPLES:
llmctl "hello world c++"
llmctl --context notes.md "extend section 5" >> notes.md
cat file.txt | llmctl "summarize"

`)
	}
	flag.Parse()

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

	var ctxBuf bytes.Buffer
	if *contextFile != "" {
		b, err := os.ReadFile(*contextFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "context read error:", err)
			os.Exit(2)
		}
		ctxBuf.WriteString(tailString(string(b), *tail))
	}

	if len(os.Args) > 1 && os.Args[1] == "counter" {
		runCounter(os.Args[2:])
		return
	}

	// Prompt bauen: Kontext + stdin + args
	var userBuf strings.Builder
	if ctxBuf.Len() > 0 {
		userBuf.WriteString("CONTEXT (tail):\n")
		userBuf.Write(ctxBuf.Bytes())
		userBuf.WriteString("\n\n")
	}
	if strings.TrimSpace(stdinText) != "" {
		userBuf.WriteString("STDIN:\n")
		userBuf.WriteString(stdinText)
		userBuf.WriteString("\n\n")
	}
	if promptArgs != "" {
		userBuf.WriteString("PROMPT:\n")
		userBuf.WriteString(promptArgs)
	}

	// Client
	opts := []option.RequestOption{}
	if *baseURL != "" {
		opts = append(opts, option.WithBaseURL(*baseURL))
	}
	client := openai.NewClient(opts...)

	// Messages
	var msgs []openai.ChatCompletionMessageParamUnion
	if strings.TrimSpace(*system) != "" {
		msgs = append(msgs, openai.SystemMessage(*system))
	}
	msgs = append(msgs, openai.UserMessage(userBuf.String()))

	reqCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	stream := client.Chat.Completions.NewStreaming(reqCtx, openai.ChatCompletionNewParams{
		Messages: msgs,
		Model:    shared.ChatModel(*model),
	})

	// Accumulator (für usage & final content)
	var acc openai.ChatCompletionAccumulator

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 {
			if delta := chunk.Choices[0].Delta.Content; delta != "" {
				io.WriteString(w, delta)
			}
		}
	}

	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "stream error:", err)
		os.Exit(1)
	}

	// Newline erzwingen, falls Modell ohne endet
	if len(acc.Choices) > 0 {
		out := acc.Choices[0].Message.Content
		if out != "" && !strings.HasSuffix(out, "\n") {
			fmt.Fprintln(w)
		}
	}

	// ---- Usage / Token Counter Hook (stderr only!) ----
	if acc.Usage.TotalTokens > 0 {
		s, err := counter.Load()
		if err == nil {
			s = counter.Add(s, counter.State{
				PromptTokens:     int(acc.Usage.PromptTokens),
				CompletionTokens: int(acc.Usage.CompletionTokens),
				TotalTokens:      int(acc.Usage.TotalTokens),
			})
			_ = counter.Save(s)
		}
	} else if *verbose {
		fmt.Fprintln(os.Stderr, "[llmctl] backend did not provide token usage")
	}
}
