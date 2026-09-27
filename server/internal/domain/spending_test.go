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
	facts := EvaluateSpending(SpendingEvaluationInput{Profile: Profile{CurrentBalance: 1000}, Category: "学習"})
	if facts.Verdict != "insufficient_data" {
		t.Fatalf("verdict = %q", facts.Verdict)
	}
	if facts.RegisteredCurrentBalance != 1000 || facts.RegisteredBalanceAfterPlanned != nil || facts.RegisteredBalanceInsufficient {
		t.Fatalf("balance facts = %#v", facts)
	}
}

func TestEvaluateSpendingAvoidsWhenPlannedAmountExceedsRegisteredBalance(t *testing.T) {
	amount := int64(5000)
	facts := EvaluateSpending(SpendingEvaluationInput{
		Profile:       Profile{CurrentBalance: 1000},
		Dashboard:     Dashboard{FreeBudgetRemaining: 10000},
		PlannedAmount: &amount,
		Category:      "買い物",
	})
	if facts.Verdict != "avoid" || !facts.RegisteredBalanceInsufficient || facts.BudgetExceeded {
		t.Fatalf("facts = %#v", facts)
	}
	if facts.RegisteredBalanceAfterPlanned == nil || *facts.RegisteredBalanceAfterPlanned != -4000 {
		t.Fatalf("registered balance after planned = %#v", facts.RegisteredBalanceAfterPlanned)
	}
}

func TestEvaluateSpendingAllowsAmountEqualToRegisteredBalanceAtBalanceBoundary(t *testing.T) {
	amount := int64(5000)
	facts := EvaluateSpending(SpendingEvaluationInput{
		Profile:       Profile{CurrentBalance: amount},
		Dashboard:     Dashboard{FreeBudgetRemaining: amount},
		PlannedAmount: &amount,
		Category:      "買い物",
	})
	if facts.Verdict != "safe" || facts.RegisteredBalanceInsufficient || facts.BudgetExceeded {
		t.Fatalf("facts = %#v", facts)
	}
	if facts.RegisteredBalanceAfterPlanned == nil || *facts.RegisteredBalanceAfterPlanned != 0 {
		t.Fatalf("registered balance after planned = %#v", facts.RegisteredBalanceAfterPlanned)
	}
}

func TestEvaluateSpendingDistinguishesFreeBudgetExceededFromRegisteredBalance(t *testing.T) {
	amount := int64(5001)
	facts := EvaluateSpending(SpendingEvaluationInput{
		Profile:       Profile{CurrentBalance: 10000},
		Dashboard:     Dashboard{FreeBudgetRemaining: 5000},
		PlannedAmount: &amount,
		Category:      "買い物",
	})
	if facts.Verdict != "avoid" || !facts.BudgetExceeded || facts.RegisteredBalanceInsufficient {
		t.Fatalf("facts = %#v", facts)
	}
}
