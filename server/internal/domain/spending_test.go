package domain

import "testing"

func TestEvaluateSpendingKeepsVerdictInGo(t *testing.T) {
	amount := int64(6001)
	facts := EvaluateSpending(SpendingEvaluationInput{
		Profile:       Profile{MonthlyFreeBudget: 6000},
		Dashboard:     Dashboard{FreeBudgetRemaining: 6000},
		PlannedAmount: &amount,
		Category:      "買い物",
	})
	if facts.Verdict != "avoid" || !facts.BudgetExceeded || facts.RemainingAfterPlanned != -1 {
		t.Fatalf("facts = %#v", facts)
	}
}

func TestEvaluateSpendingRequiresAmount(t *testing.T) {
	facts := EvaluateSpending(SpendingEvaluationInput{Category: "学習"})
	if facts.Verdict != "insufficient_data" {
		t.Fatalf("verdict = %q", facts.Verdict)
	}
}
