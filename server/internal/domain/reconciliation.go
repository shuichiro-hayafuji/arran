package domain

// ReconciliationDecision is an explicit local identity choice, not a money update.
type ReconciliationDecision struct {
	Row          int    `json:"row"`
	Action       string `json:"action"`
	PurchaseID   string `json:"purchase_id"`
	Snapshot     string `json:"snapshot"`
	StateVersion int64  `json:"state_version"`
}
type ReconciliationCandidate struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	PlannedAmount  int64  `json:"planned_amount"`
	ActualAmount   *int64 `json:"actual_amount"`
	Status         string `json:"status"`
	Classification string `json:"classification"`
	RecordedAt     string `json:"recorded_at"`
	PurchaseDate   string `json:"purchase_date"`
	Snapshot       string `json:"snapshot"`
}
type ReconciliationConflict struct {
	Row          int                       `json:"row"`
	Transaction  Transaction               `json:"transaction"`
	Candidates   []ReconciliationCandidate `json:"candidates"`
	StateVersion int64                     `json:"state_version"`
}
type ExpenseCount struct {
	Unresolved           int    `json:"unresolved"`
	Month                string `json:"month"`
	Confirmed            int    `json:"confirmed"`
	Estimated            int    `json:"estimated"`
	Undated              int    `json:"undated"`
	PendingRecalculation int    `json:"pending_recalculation"`
	Scope                string `json:"scope"`
}
