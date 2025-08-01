package formatter

type GroupedData struct {
	Date          string
	Models        []string
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
	TotalTokens   int
	Cost          float64
	ShowBreakdown bool
	ModelDetails  []ModelDetail
}

type ModelDetail struct {
	Model         string
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
	TotalTokens   int
	Cost          float64
}
