package consultation

import (
	"context"
	"errors"
	"strings"
	"time"
)

// DataChangeContext exposes only direction, opaque operation refs, and financial triplets.
// Bank identity and provenance remain inside persistence.
type DataChangeContext struct {
	Ref                string        `json:"ref"`
	Operation          string        `json:"operation"`
	Before             []Transaction `json:"before"`
	After              []Transaction `json:"after"`
	Included           int           `json:"included_count"`
	Omitted            int           `json:"omitted_count"`
	BalanceResolved    bool          `json:"balance_resolved"`
	BudgetResolved     bool          `json:"budget_resolved"`
	RelatedPurchaseIDs []string      `json:"related_purchase_ids"`
}
type DataChangeRepository interface {
	LoadDataChanges(context.Context, string) ([]DataChangeContext, error)
}

func reconciliationSet(reply Reply, key string) bool {
	for _, c := range reply.Changes {
		if c.Key == key && c.Operation == "set" && (c.Basis == "reconciliation" || c.Basis == "user_report") {
			return true
		}
	}
	return false
}
func validateDataChanges(reply Reply, in Input) error {
	contexts := map[string]DataChangeContext{}
	for _, c := range in.DataChangeContext {
		contexts[c.Ref] = c
	}
	seen := map[string]bool{}
	for _, u := range reply.DataChangeUpdates {
		c, ok := contexts[u.Ref]
		if !ok || seen[u.Ref] {
			return errors.New("data change ref is absent or duplicated")
		}
		seen[u.Ref] = true
		if u.Status != "applied" && u.Status != "needs_information" && u.Status != "baseline_adopted" {
			return errors.New("invalid data change status")
		}
		if u.Status == "needs_information" {
			if reply.Question == "" {
				return errors.New("data change inclusion uncertainty requires a question")
			}
			continue
		}
		if u.Status == "baseline_adopted" {
			at, err := time.Parse(time.RFC3339, in.Now)
			if err != nil {
				return errors.New("baseline adoption requires the current consultation time")
			}
			if err = ValidateDataChangeBaseline(reply.Changes, at); err != nil {
				return err
			}
			continue
		}
		if c.Omitted > 0 {
			return errors.New("partial data change coverage cannot be marked applied")
		}
		if !c.BalanceResolved && !reconciliationSet(reply, "current_balance") {
			return errors.New("data change balance inclusion requires an explicit financial basis")
		}
		if !c.BudgetResolved && !reconciliationSet(reply, "monthly_free_budget") {
			return errors.New("data change free-budget inclusion requires an explicit financial basis")
		}
	}
	for _, c := range in.DataChangeContext {
		applied := false
		for _, u := range reply.DataChangeUpdates {
			if u.Ref == c.Ref && (u.Status == "applied" || u.Status == "baseline_adopted") {
				applied = true
			}
		}
		if applied {
			continue
		}
		if reply.Question == "" {
			return errors.New("unresolved data changes require an inclusion question")
		}
		for _, change := range reply.Changes {
			if financialKey(change.Key) && change.Operation == "set" && change.Basis != "user_report" && !unchangedFinancialFact(in.Facts, change) {
				return errors.New("unresolved data changes cannot change financial facts")
			}
		}
	}
	return nil
}

// ValidateDataChangeBaseline is shared by model validation and the durable save
// boundary. Provenance and periods are structural; money interpretation stays in LLM.
// RecordedAt is observation time, not evidence that a historical period is current.
func ValidateDataChangeBaseline(changes []Change, at time.Time) error {
	now := at.In(time.FixedZone("Asia/Tokyo", 9*3600))
	month, day := now.Format("2006-01"), now.Format("2006-01-02")
	seen := map[string]bool{}
	for _, c := range changes {
		if c.Key != "current_balance" && c.Key != "monthly_free_budget" {
			continue
		}
		if seen[c.Key] || c.Operation != "set" || c.Basis != "user_report" || c.Kind != "fact" || strings.TrimSpace(c.Value) == "" {
			return errors.New("baseline adoption requires both unique explicit user-reported facts")
		}
		if c.Period != "" && c.Period != month && !(c.Key == "current_balance" && c.Period == day) {
			return errors.New("baseline adoption requires current-compatible financial periods")
		}
		seen[c.Key] = true
	}
	if !seen["current_balance"] || !seen["monthly_free_budget"] {
		return errors.New("baseline adoption requires both explicit user-reported financial facts")
	}
	return nil
}
