package main

import (
	"fmt"
	"os"

	"llmctl/counter"
)

func runCounter(args []string) {
	if len(args) > 0 && args[0] == "reset" {
		if err := counter.Save(counter.State{}); err != nil {
			fmt.Fprintln(os.Stderr, "counter reset failed:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "counter reset")
		return
	}

	s, err := counter.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "counter load failed:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout,
		"prompt=%d completion=%d total=%d\n",
		s.PromptTokens,
		s.CompletionTokens,
		s.TotalTokens,
	)
}
