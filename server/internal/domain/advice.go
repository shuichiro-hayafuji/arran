package domain

// AdviceFacts are deterministic facts calculated by Go before an LLM is
// called. The model may explain these facts, but it must not replace them.
type AdviceFacts struct {
	Category              string `json:"category"`
	PlannedAmount         *int64 `json:"planned_amount"`
	FreeBudgetRemaining   int64  `json:"free_budget_remaining"`
	RemainingAfterPlanned int64  `json:"remaining_after_planned"`
	CategoryTotal         int64  `json:"category_total"`
	CategoryIsReduced     bool   `json:"category_is_reduced"`
	CategoryIsAllowed     bool   `json:"category_is_allowed"`
	BudgetExceeded        bool   `json:"budget_exceeded"`
	RegretPattern         string `json:"regret_pattern"`
	Verdict               string `json:"verdict"`
}

type AdviceDraft struct {
	Recommendation   string `json:"recommendation"`
	Verdict          string `json:"verdict"`
	ReasoningSummary string `json:"reasoning_summary"`
	CurrentSituation string `json:"current_situation"`
	Alternative      string `json:"alternative"`
	FinalQuestion    string `json:"final_question"`
	InferredCategory string `json:"inferred_category"`
	NeedsFollowUp    bool   `json:"needs_follow_up"`
	FollowUpQuestion string `json:"follow_up_question"`
}
