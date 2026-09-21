package domain

import "time"

// LLMUsage は外部モデルの費用集計に必要な数値だけを保持する。
// 相談内容、家計情報、モデル出力は含めない。
type LLMUsage struct {
	UserID            int64
	Operation         string
	Model             string
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	ReasoningTokens   int
	TotalTokens       int
	OccurredAt        time.Time
}
