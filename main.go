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

func main() {
	var (
		model       = flag.String("model", getenv("LLM_MODEL", "gpt-5.2"), "Model name")
		system      = flag.String("system", os.Getenv("LLM_SYSTEM"), "System prompt")
		contextFile = flag.String("context", "", "Optional context file to append")
		tail        = flag.Int("tail", 12000, "Max chars to take from context file (tail)")
		timeout     = flag.Duration("timeout", 5*time.Minute, "Request timeout")
		baseURL     = flag.String("base-url", os.Getenv("OPENAI_BASE_URL"), "Optional base URL (OpenAI-compatible)")
	)
	flag.Parse()

	promptArgs := strings.TrimSpace(strings.Join(flag.Args(), " "))
	stdinText, err := readAllStdin()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdin read error:", err)
		os.Exit(2)
	}

	if promptArgs == "" && strings.TrimSpace(stdinText) == "" {
		fmt.Fprintln(os.Stderr, "usage: llm [--model ...] [--context file] \"your prompt\"")
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
	// API Key kommt standardmäßig aus OPENAI_API_KEY :contentReference[oaicite:1]{index=1}
	if *baseURL != "" {
		opts = append(opts, option.WithBaseURL(*baseURL))
	}

	client := openai.NewClient(opts...)

	// Messages
	msgs := []openai.ChatCompletionMessageParamUnion{}
	if strings.TrimSpace(*system) != "" {
		msgs = append(msgs, openai.SystemMessage(*system))
	}
	msgs = append(msgs, openai.UserMessage(userBuf.String()))

	reqCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Streaming
	stream := client.Chat.Completions.NewStreaming(reqCtx, openai.ChatCompletionNewParams{
		Messages: msgs,
		Model:    shared.ChatModel(*model), // Model als String
	})

	// Optional: Accumulator für “am Ende nochmal alles haben”
	acc := openai.ChatCompletionAccumulator{}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		// delta content direkt nach stdout
		if len(chunk.Choices) > 0 {
			io.WriteString(w, chunk.Choices[0].Delta.Content)
		}
	}

	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "\nstream error:", err)
		os.Exit(1)
	}

	// Wenn du willst: am Ende newline erzwingen
	if !strings.HasSuffix(acc.Choices[0].Message.Content, "\n") {
		fmt.Fprintln(w)
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
