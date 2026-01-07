package counter

func Add(cur State, add State) State {
	cur.PromptTokens += add.PromptTokens
	cur.CompletionTokens += add.CompletionTokens
	cur.TotalTokens += add.TotalTokens
	return cur
}
