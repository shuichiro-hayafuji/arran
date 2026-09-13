package domain

type ReviewCandidate struct {
	Label            string `json:"label"`
	Judgement        string `json:"judgement"`
	TotalAmount      int64  `json:"total_amount"`
	Count            int    `json:"count"`
	Reason           string `json:"reason"`
	AnnualizedAmount int64  `json:"annualized_amount"`
	Question         string `json:"question"`
}

type MonthlyReview struct {
	ID         int64             `json:"id"`
	Month      string            `json:"month"`
	CreatedAt  string            `json:"created_at"`
	Summary    string            `json:"summary"`
	Candidates []ReviewCandidate `json:"candidates"`
}
