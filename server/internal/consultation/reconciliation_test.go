package consultation

import (
	"fmt"
	"testing"
)

func TestReconciliationRequiresBothFinancialDimensions(t *testing.T) {
	amount := int64(2500)
	in := Input{Purchases: []Purchase{{ID: "p", ActualAmount: &amount, Classification: "free", Status: "purchased"}}, ReconciliationContext: []ReconciliationContext{{Ref: "op", PurchaseID: "p", PriorInclusion: "unknown", CSV: Transaction{Amount: 2500}}}, Facts: []Fact{{Key: "current_balance", Value: "50000円", Active: true}, {Key: "monthly_free_budget", Value: "30000円", Active: true}}}
	balance := Change{Key: "current_balance", Value: "47500円", Basis: "reconciliation", Operation: "set"}
	budget := Change{Key: "monthly_free_budget", Value: "27500円", Basis: "reconciliation", Operation: "set"}
	r := Reply{ReconciliationUpdates: []ReconciliationUpdate{{Ref: "op", Status: "applied"}}}
	for _, changes := range [][]Change{{balance}, {budget}, nil} {
		r.Changes = changes
		if err := validateReconciliation(r, in); err == nil {
			t.Fatal("one unresolved dimension completed receipt")
		}
	}
	r.Changes = []Change{balance, budget}
	if err := validateReconciliation(r, in); err != nil {
		t.Fatal(err)
	}
	r.ReconciliationUpdates[0].Status = "needs_information"
	r.Question = "自由予算にも含まれていますか"
	r.Changes = []Change{balance}
	if err := validateReconciliation(r, in); err != nil {
		t.Fatal(err)
	}
	in.ReconciliationContext[0].BalanceResolved = true
	r.ReconciliationUpdates[0].Status = "applied"
	r.Changes = []Change{budget}
	if err := validateReconciliation(r, in); err != nil {
		t.Fatal(err)
	}
	r.Changes = []Change{balance, budget}
	if err := validateReconciliation(r, in); err == nil {
		t.Fatal("settled balance posted again")
	}
	r.Changes[0].Basis = "user_report"
	if err := validateReconciliation(r, in); err != nil {
		t.Fatal("explicit new balance rejected", err)
	}
}

func TestPendingReconciliationHasPriorityWithinCoverageLimit(t *testing.T) {
	s := State{}
	for i := 0; i < 20; i++ {
		s.Purchases = append(s.Purchases, Purchase{ID: fmt.Sprint(i), RecordedAt: "2035-01-01T00:00:00Z", Status: "purchase_confirmation_pending"})
	}
	amount := int64(100)
	s.Purchases = append(s.Purchases, Purchase{ID: "linked", RecordedAt: "2035-01-01T00:00:00Z", Status: "purchased", ActualAmount: &amount, Classification: "free", CSVReconciliationPending: true})
	purchases, coverage := modelPurchases(s, "hello", 0, "linked")
	if len(purchases) != 20 || !coverage.Partial || coverage.Omitted != 1 {
		t.Fatal("limits changed", coverage)
	}
	for _, p := range purchases {
		if p.ID == "linked" {
			return
		}
	}
	t.Fatal("forced reconciliation target omitted")
}

func TestNotPurchasedBankConflictRequiresAnOutcomeCorrection(t *testing.T) {
	amount := int64(2500)
	in := Input{Purchases: []Purchase{{ID: "p", Status: "not_purchased", Classification: "necessary"}}, ReconciliationContext: []ReconciliationContext{{Ref: "op", PurchaseID: "p", CSV: Transaction{Amount: 2500}, PriorInclusion: "unknown"}}}
	reply := Reply{ReconciliationUpdates: []ReconciliationUpdate{{Ref: "op", Status: "applied"}}, Changes: []Change{{Key: "current_balance", Value: "50000円", Operation: "set", Basis: "user_report"}, {Key: "monthly_free_budget", Value: "30000円", Operation: "set", Basis: "user_report"}}}
	if err := validateReconciliation(reply, in); err == nil {
		t.Fatal("financial observations erased bank/outcome contradiction")
	}
	reply.PurchaseUpdates = []PurchaseUpdate{{ID: "p", Outcome: "purchased", Amount: &amount, Classification: "free", AmountBasis: "reported"}}
	if err := validateReconciliation(reply, in); err != nil {
		t.Fatal("explicit purchased correction rejected", err)
	}
}
