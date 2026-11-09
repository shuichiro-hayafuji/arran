package domain

type CSVMapping struct {
	TransactionIDColumn string `json:"transaction_id_column,omitempty"`
	DateColumn          string `json:"date_column"`
	DescriptionColumn   string `json:"description_column"`
	AmountColumn        string `json:"amount_column"`
	DebitColumn         string `json:"debit_column,omitempty"`
	CreditColumn        string `json:"credit_column,omitempty"`
}

type ImportIdentityDecision struct {
	Snapshot      string `json:"snapshot"`
	Row           int    `json:"row"`
	Action        string `json:"action"`
	TransactionID int64  `json:"transaction_id"`
}
type ImportIdentityConflict struct {
	CandidateSnapshots map[int64]string `json:"candidate_snapshots"`
	Row                int              `json:"row"`
	Transaction        Transaction      `json:"transaction"`
	Candidates         []Transaction    `json:"candidates"`
}

type ImportPreview struct {
	ReactivationIDs         []int64                  `json:"reactivation_ids"`
	ReconciliationConflicts []ReconciliationConflict `json:"reconciliation_conflicts"`
	IdentityConflicts       []ImportIdentityConflict `json:"identity_conflicts"`
	IdentityNotice          string                   `json:"identity_notice"`
	PreviewID               string                   `json:"preview_id"`
	DetectedEncoding        string                   `json:"detected_encoding"`
	Headers                 []string                 `json:"headers"`
	Mapping                 CSVMapping               `json:"mapping"`
	NeedsMapping            bool                     `json:"needs_mapping"`
	ReadCount               int                      `json:"read_count"`
	PeriodStart             string                   `json:"period_start"`
	PeriodEnd               string                   `json:"period_end"`
	ExpenseTotal            int64                    `json:"expense_total"`
	Rows                    []Transaction            `json:"rows"`
	Warnings                []string                 `json:"warnings"`
}

type ImportCommitResult struct {
	FinancialPending int    `json:"financial_pending"`
	ImportID         string `json:"import_id"`
	ReactivatedCount int    `json:"reactivated_count"`
	ImportedCount    int    `json:"imported_count"`
	DuplicateCount   int    `json:"duplicate_count"`
}
