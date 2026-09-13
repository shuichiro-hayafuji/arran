package domain

// Transaction はCSVから正規化した明細。Amount は整数の円で、入出金は TransactionType で区別する。
// RawData にはCSV原文を保持するため、この型をそのままLLMへ送信しない。
type Transaction struct {
	ID                 int64          `json:"id"`
	TransactionDate    string         `json:"transaction_date"`
	Description        string         `json:"description"`
	NormalizedMerchant string         `json:"normalized_merchant"`
	Amount             int64          `json:"amount"`
	TransactionType    string         `json:"transaction_type"`
	Category           string         `json:"category"`
	Source             string         `json:"source"`
	SourceAccountName  string         `json:"source_account_name"`
	Fingerprint        string         `json:"fingerprint"`
	ImportedAt         string         `json:"imported_at"`
	RawData            map[string]any `json:"raw_data,omitempty"`
}
