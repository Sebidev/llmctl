package counter

func Add(s State, prompt, completion int64) State {
	s.PromptTokens += prompt
	s.CompletionTokens += completion
	s.TotalTokens += prompt + completion
	return s
}
