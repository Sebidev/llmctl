package counter

type State struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}
