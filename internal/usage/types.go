package usage

import "time"

// TokenUsage represents token counts from a Claude session
type TokenUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// Add adds another TokenUsage to this one
func (t *TokenUsage) Add(other TokenUsage) {
	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.CacheCreationInputTokens += other.CacheCreationInputTokens
	t.CacheReadInputTokens += other.CacheReadInputTokens
}

// TotalInput returns the total input tokens (regular + cache creation + cache read)
func (t *TokenUsage) TotalInput() int64 {
	return t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
}

// SessionUsage represents usage data for a Claude session
type SessionUsage struct {
	SessionID     string
	ProjectPath   string
	TotalUsage    TokenUsage
	EstimatedCost float64
	Model         string
	LastUpdated   time.Time
}
