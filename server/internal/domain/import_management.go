package domain

type ImportPreviewOptions struct {
	FileName            string
	ConfirmReactivation bool
}
type ImportMetadata struct {
	ConfirmReactivation bool       `json:"-"`
	CommitKey           string     `json:"-"`
	FileName            string     `json:"file_name"`
	Encoding            string     `json:"encoding"`
	Mapping             CSVMapping `json:"mapping"`
}
type ImportBatch struct {
	CancelAffected  int                `json:"cancel_affected_count"`
	CancelProtected int                `json:"cancel_protected_count"`
	ID              string             `json:"id"`
	FileKey         string             `json:"file_key"`
	Source          string             `json:"source"`
	Account         string             `json:"account"`
	Metadata        ImportMetadata     `json:"metadata"`
	Status          string             `json:"status"`
	Version         int64              `json:"version"`
	StateVersion    int64              `json:"state_version"`
	RecordedAt      string             `json:"recorded_at"`
	Legacy          bool               `json:"legacy"`
	Result          ImportCommitResult `json:"result"`
	Rows            []ImportEvidence   `json:"rows,omitempty"`
	Snapshot        string             `json:"snapshot"`
}
type ImportEvidence struct {
	Row            int         `json:"row"`
	TransactionID  int64       `json:"transaction_id"`
	Original       Transaction `json:"original"`
	Current        Transaction `json:"current"`
	Active         bool        `json:"active"`
	LegacyRetained bool        `json:"legacy_retained"`
}
type MutationInput struct {
	ID           string  `json:"mutation_id"`
	Action       string  `json:"action"`
	Version      int64   `json:"expected_version"`
	StateVersion int64   `json:"state_version"`
	Snapshot     string  `json:"snapshot"`
	Amount       *int64  `json:"amount"`
	Date         *string `json:"transaction_date"`
	Category     *string `json:"category"`
	Remember     bool    `json:"remember_merchant"`
	Reason       string  `json:"reason"`
}
type MutationResult struct {
	ID           string `json:"mutation_id"`
	Status       string `json:"status"`
	Affected     int    `json:"affected_count"`
	Protected    int    `json:"protected_count"`
	Pending      int    `json:"pending_recalculation"`
	StateVersion int64  `json:"state_version"`
}
type TransactionChangeEvent struct {
	ID         string `json:"id"`
	MutationID string `json:"mutation_id"`
	Kind       string `json:"kind"`
	Target     string `json:"target"`
	RecordedAt string `json:"recorded_at"`
	Before     string `json:"before_state"`
	After      string `json:"after_state"`
}

type TransactionRecord struct {
	Original     Transaction `json:"original"`
	Current      Transaction `json:"current"`
	Active       bool        `json:"active"`
	StateVersion int64       `json:"state_version"`
}
