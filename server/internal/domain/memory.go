package domain

type MemoryItem struct {
	ID         int64   `json:"id"`
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}
