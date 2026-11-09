package consultation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// ReconciliationContext contains no bank identifiers or merchant text.
type ReconciliationContext struct {
	BalanceResolved  bool        `json:"balance_resolved"`
	BudgetResolved   bool        `json:"budget_resolved"`
	Ref              string      `json:"ref"`
	PurchaseID       string      `json:"purchase_id"`
	CSV              Transaction `json:"csv"`
	PriorInclusion   string      `json:"prior_inclusion"`
	PurchaseRevision string      `json:"-"`
}
type ReconciliationUpdate struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

// PurchaseSnapshot is local concurrency metadata, never an external bank identity.
func PurchaseSnapshot(p Purchase) string {
	data, _ := json.Marshal(p)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ReconciliationRepository is a consultation-owned read boundary.
type ReconciliationRepository interface {
	LoadReconciliations(context.Context, string) ([]ReconciliationContext, error)
	RequiresCSVReconciliation(context.Context, string) (bool, error)
}

func validateReconciliation(reply Reply, in Input) error {
	required := map[string]bool{}
	for _, id := range in.ReconciliationRequiredIDs {
		required[id] = true
	}
	for _, u := range reply.PurchaseUpdates {
		if required[u.ID] && u.Amount != nil {
			if reply.Question == "" {
				return errors.New("CSV identity is unresolved; ask which record it is before financial posting")
			}
			for _, c := range reply.Changes {
				if financialKey(c.Key) && c.Operation == "set" && c.Basis != "user_report" && !unchangedFinancialFact(in.Facts, c) {
					return errors.New("unresolved CSV/purchase identity cannot change financial facts")
				}
			}
		}
	}
	contexts := map[string]ReconciliationContext{}
	for _, c := range in.ReconciliationContext {
		contexts[c.Ref] = c
	}
	set := func(key string) bool {
		for _, change := range reply.Changes {
			if change.Key == key && change.Operation == "set" && (change.Basis == "reconciliation" || change.Basis == "user_report") {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	balanceNeeded, budgetNeeded := false, false
	for _, u := range reply.ReconciliationUpdates {
		c, ok := contexts[u.Ref]
		if !ok || seen[u.Ref] {
			return errors.New("reconciliation ref is absent or duplicated")
		}
		seen[u.Ref] = true
		if u.Status != "applied" && u.Status != "needs_information" {
			return errors.New("invalid reconciliation status")
		}
		var purchase *Purchase
		for i := range in.Purchases {
			if in.Purchases[i].ID == c.PurchaseID {
				purchase = &in.Purchases[i]
			}
		}
		if purchase == nil {
			return errors.New("reconciliation purchase is outside input coverage")
		}
		latest := *purchase
		for _, update := range reply.PurchaseUpdates {
			if update.ID == latest.ID && update.Outcome != "classification_required" {
				latest.Status, latest.ActualAmount, latest.Classification = update.Outcome, update.Amount, update.Classification
			}
		}
		purchase = &latest
		if !c.BalanceResolved {
			balanceNeeded = true
		}
		if !c.BudgetResolved && purchase.Classification != "unknown" {
			budgetNeeded = true
		}
		if u.Status == "needs_information" && reply.Question == "" {
			return errors.New("reconciliation uncertainty requires a question")
		}
		if u.Status == "applied" {
			if purchase.Status == "not_purchased" && c.CSV.Amount > 0 {
				return errors.New("bank debit conflicts with not-purchased outcome; financial observations alone cannot resolve purchase identity")
			}
			if (purchase.ActualAmount != nil && *purchase.ActualAmount != c.CSV.Amount) && !set("current_balance") && !set("monthly_free_budget") {
				return errors.New("CSV observation contradicts the latest outcome; ask explicitly and retain the latest purchase")
			}
			if !c.BalanceResolved && !set("current_balance") {
				return errors.New("balance inclusion is unresolved; keep reconciliation pending")
			}
			if purchase.Classification == "unknown" && purchase.Status != "not_purchased" {
				return errors.New("classification is unknown; preserve free-budget uncertainty")
			}
			if !c.BudgetResolved && !set("monthly_free_budget") {
				return errors.New("free-budget inclusion is unresolved; keep reconciliation pending")
			}
		}
	}
	for _, u := range reply.DataChangeUpdates {
		for _, c := range in.DataChangeContext {
			if u.Ref == c.Ref {
				seen[u.Ref] = true
				balanceNeeded = balanceNeeded || !c.BalanceResolved
				budgetNeeded = budgetNeeded || !c.BudgetResolved
			}
		}
	}
	for _, change := range reply.Changes {
		if change.Basis != "reconciliation" {
			continue
		}
		if len(seen) == 0 {
			return errors.New("reconciliation changes require a pending identified operation")
		}
		needed := balanceNeeded
		if change.Key == "monthly_free_budget" {
			needed = budgetNeeded
		}
		if financialKey(change.Key) && !needed && !unchangedFinancialFact(in.Facts, change) {
			return errors.New("settled reconciliation dimension cannot be posted again; explicit user reports remain separate")
		}
	}
	return nil
}
func applyReconciliation(state *State, reply Reply, in Input, timestamp string) {
	required := map[string]bool{}
	for _, id := range in.ReconciliationRequiredIDs {
		required[id] = true
	}
	for _, u := range reply.PurchaseUpdates {
		for i := range state.Purchases {
			if state.Purchases[i].ID == u.ID && u.Amount != nil && required[u.ID] {
				state.Purchases[i].CSVReconciliationPending = true
			}
		}
	}
	for _, u := range reply.PurchaseUpdates {
		for i := range state.Purchases {
			p := &state.Purchases[i]
			if p.ID == u.ID && !required[u.ID] && (p.BalanceAppliedAmount != nil || u.Outcome == "not_purchased") {
				p.CSVReconciliationPending = false
			}
		}
	}
	for _, u := range reply.ReconciliationUpdates {
		if u.Status != "applied" && u.Status != "needs_information" {
			continue
		}
		for _, c := range in.ReconciliationContext {
			if c.Ref != u.Ref {
				continue
			}
			for i := range state.Purchases {
				p := &state.Purchases[i]
				if p.ID != c.PurchaseID {
					continue
				}
				p.CSVReconciliationPending = u.Status != "applied"
				changed := false
				for _, change := range reply.Changes {
					if change.Operation != "set" || (change.Basis != "reconciliation" && change.Basis != "user_report") {
						continue
					}
					if change.Key == "current_balance" {
						if !sameAmount(p.BalanceAppliedAmount, p.ActualAmount) {
							changed = true
						}
						p.BalanceAppliedAmount = p.ActualAmount
					}
					if change.Key == "monthly_free_budget" && p.Classification != "unknown" {
						var target *int64
						if p.Classification == "free" && p.Status != "not_purchased" {
							target = p.ActualAmount
						}
						if !sameAmount(p.FreeBudgetAppliedAmount, target) {
							changed = true
						}
						p.FreeBudgetAppliedAmount = target
					}
				}
				if changed {
					p.AccountingTracked = true
					p.Events = append(p.Events, PurchaseEvent{Outcome: p.Status, Amount: p.ActualAmount, Classification: p.Classification, RecordedAt: timestamp, Source: "reconciliation", BalanceAppliedAmount: p.BalanceAppliedAmount, FreeBudgetAppliedAmount: p.FreeBudgetAppliedAmount, BalanceBasis: "reconciliation"})
				}
			}
		}
	}
}

func sameAmount(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
