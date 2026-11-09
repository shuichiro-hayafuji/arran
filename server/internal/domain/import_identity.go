package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Snapshot identifies the immutable record fields confirmed by the user. Category
// corrections are deliberately excluded and never overwritten by a match.
func ImportIdentitySnapshot(t Transaction) string {
	value, _ := json.Marshal([]any{t.ID, t.TransactionDate, t.Amount, t.TransactionType, t.Description, t.Source, t.SourceAccountName})
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
