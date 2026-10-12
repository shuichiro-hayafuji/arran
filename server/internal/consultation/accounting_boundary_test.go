package consultation

import "testing"

func TestUnclearAndUnidentifiedAccountingChangesRejected(t *testing.T) {
	for _, key := range []string{"current_balance", "monthly_free_budget"} {
		for _, basis := range []string{"purchase", "memory"} {
			for _, periodOnly := range []bool{false, true} {
				in := Input{Facts: []Fact{{Key: key, Value: "50000円", Period: "2026-10", Active: true}}}
				c := Change{Key: key, Value: "47500円", Period: "2026-10", Operation: "set", Kind: "fact", Basis: basis}
				if periodOnly {
					c.Value = "50000円"
					c.Period = "2026-11"
				}
				r := Reply{PurchaseTargetUnclear: true, Question: "どちらの購入ですか？", Changes: []Change{c}}
				if err := validatePurchases(r, in); err == nil {
					t.Fatalf("unclear financial change accepted: key=%s basis=%s period=%v", key, basis, periodOnly)
				}
				r.PurchaseTargetUnclear = false
				if basis == "purchase" && validatePurchases(r, in) == nil {
					t.Fatal("purchase change with no outcome ID accepted")
				}
				product := "商品"
				amount := int64(2500)
				r.PurchaseChange = &PurchaseChange{Operation: "add", Description: &product, Amount: &amount}
				if validatePurchases(r, in) == nil {
					t.Fatal("plan-only change posted money")
				}
			}
		}
	}
}

func TestFinancialEchoAndExplicitUserReportsRemainAllowed(t *testing.T) {
	for _, key := range []string{"current_balance", "monthly_free_budget"} {
		in := Input{Facts: []Fact{{Key: key, Value: "50000円", Period: "2026-10", Active: true}}}
		for _, basis := range []string{"purchase", "memory"} {
			c := Change{Key: key, Value: "50000円", Period: "2026-10", Operation: "set", Kind: "fact", Basis: basis}
			for _, unclear := range []bool{false, true} {
				r := Reply{PurchaseTargetUnclear: unclear, Question: "どちらですか？", Changes: []Change{c}}
				if err := validatePurchases(r, in); err != nil {
					t.Fatalf("no-op echo rejected: %v", err)
				}
			}
		}
		c := Change{Key: key, Value: "70000円", Period: "2026-11", Operation: "set", Kind: "fact", Basis: "user_report"}
		r := Reply{PurchaseTargetUnclear: true, Question: "どちらですか？", Changes: []Change{c}}
		if err := validatePurchases(r, in); err != nil {
			t.Fatalf("explicit user report rejected: %v", err)
		}
	}
	r := Reply{PurchaseTargetUnclear: true, Question: "どちらですか？", Changes: []Change{{Key: "preference", Value: "青い物が好き", Operation: "set", Kind: "fact", Basis: "memory"}}}
	if err := validatePurchases(r, Input{}); err != nil {
		t.Fatalf("nonfinancial memory rejected: %v", err)
	}
}

func TestIdentifiedMultipleOutcomesKeepFinancialChanges(t *testing.T) {
	amount := int64(2500)
	in := Input{Purchases: []Purchase{{ID: "first", Status: "purchase_confirmation_pending"}, {ID: "second", Status: "purchase_confirmation_pending"}}, Facts: []Fact{{Key: "current_balance", Value: "50000円", Active: true}, {Key: "monthly_free_budget", Value: "30000円", Active: true}}}
	r := Reply{PurchaseUpdates: []PurchaseUpdate{{ID: "first", Outcome: "purchased", Amount: &amount, Classification: "free"}, {ID: "second", Outcome: "not_purchased", Classification: "unknown"}}, Changes: []Change{{Key: "current_balance", Value: "47500円", Operation: "set", Kind: "fact", Basis: "purchase"}, {Key: "monthly_free_budget", Value: "27500円", Operation: "set", Kind: "fact", Basis: "purchase"}}}
	if err := validatePurchases(r, in); err != nil {
		t.Fatalf("identified multiple accounting rejected: %v", err)
	}
	r.PurchaseUpdates[0].ID = "unknown"
	if validatePurchases(r, in) == nil {
		t.Fatal("unknown outcome ID authorized financial change")
	}
}
