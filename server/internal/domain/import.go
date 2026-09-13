package domain

type CSVMapping struct {
	DateColumn        string `json:"date_column"`
	DescriptionColumn string `json:"description_column"`
	AmountColumn      string `json:"amount_column"`
	DebitColumn       string `json:"debit_column,omitempty"`
	CreditColumn      string `json:"credit_column,omitempty"`
}

type ImportPreview struct {
	PreviewID        string        `json:"preview_id"`
	DetectedEncoding string        `json:"detected_encoding"`
	Headers          []string      `json:"headers"`
	Mapping          CSVMapping    `json:"mapping"`
	NeedsMapping     bool          `json:"needs_mapping"`
	ReadCount        int           `json:"read_count"`
	PeriodStart      string        `json:"period_start"`
	PeriodEnd        string        `json:"period_end"`
	ExpenseTotal     int64         `json:"expense_total"`
	Rows             []Transaction `json:"rows"`
	Warnings         []string      `json:"warnings"`
}

type ImportCommitResult struct {
	ImportedCount  int `json:"imported_count"`
	DuplicateCount int `json:"duplicate_count"`
}
