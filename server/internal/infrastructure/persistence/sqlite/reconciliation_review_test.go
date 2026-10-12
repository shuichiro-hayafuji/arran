package sqlite

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"testing"
)

func TestNotPurchasedBankConflictCannotDisappearWithBalanceEcho(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 901)
	state, _ := r.LoadChatState(ctx)
	state.Facts = []consultation.Fact{{Key: "current_balance", Value: "50000円", Kind: "fact", Active: true}, {Key: "monthly_free_budget", Value: "30000円", Kind: "fact", Active: true}}
	state.Purchases = []consultation.Purchase{{ID: "p", Status: "not_purchased", Classification: "necessary", RecordedAt: "2035-01-01T00:00:00Z", AccountingTracked: true}}
	thread, e := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "買ってない"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}})
	if e != nil {
		t.Fatal(e)
	}
	parsed, e := csvimport.Parse([]byte("date,description,amount\n2035-01-03,bank debit,2500\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if e != nil {
		t.Fatal(e)
	}
	c, e := r.PreviewReconciliations(ctx, parsed.Transactions)
	if e != nil {
		t.Fatal(e)
	}
	parsed.Transactions[0].Reconciliation = &domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: "p", Snapshot: c[0].Candidates[0].Snapshot, StateVersion: c[0].StateVersion}
	if _, e = r.ImportTransactions(ctx, parsed.Transactions); e != nil {
		t.Fatal(e)
	}
	s := consultation.Service{Repository: r, Reconciliation: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "残高の再掲", ReconciliationUpdates: []consultation.ReconciliationUpdate{{Ref: in.ReconciliationContext[0].Ref, Status: "applied"}}, Changes: []consultation.Change{{Key: "current_balance", Value: "50000円", Kind: "fact", Operation: "set", Basis: "user_report"}, {Key: "monthly_free_budget", Value: "30000円", Kind: "fact", Operation: "set", Basis: "user_report"}}}
	})}
	if _, e = s.Send(ctx, thread.ID, "残高5万円、予算3万円です"); e != nil {
		t.Fatal(e)
	}
	count, e := r.ExpenseCount(ctx, "")
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("count=%+v", count)
	if count.Unresolved == 0 && count.Confirmed == 0 && count.PendingRecalculation == 0 {
		t.Fatal("bank debit vs unpurchased conflict disappeared without purchase/identity resolution")
	}
}
func TestLegacyEstimatedBasisSurvivesClassificationOnly(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 902)
	a := int64(1000)
	state, _ := r.LoadChatState(ctx)
	state.Purchases = []consultation.Purchase{{ID: "p", Status: "needs_classification", ActualAmount: &a, Classification: "unknown", BalanceAppliedAmount: &a, AccountingTracked: true, PurchaseDate: "2035-01-01", RecordedAt: "2035-01-01T00:00:00Z", Events: []consultation.PurchaseEvent{{Outcome: "unanswered", Amount: &a, Classification: "unknown"}}}}
	thread, e := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "予定"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}})
	if e != nil {
		t.Fatal(e)
	}
	before, _ := r.ExpenseCount(ctx, "")
	s := consultation.Service{Repository: r, Reconciliation: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "仕事用", PurchaseUpdates: []consultation.PurchaseUpdate{{ID: "p", Outcome: "purchased", Amount: &a, Classification: "necessary", AmountBasis: "unchanged"}}}
	})}
	if _, e = s.Send(ctx, thread.ID, "仕事用です"); e != nil {
		t.Fatal(e)
	}
	after, _ := r.ExpenseCount(ctx, "")
	t.Logf("estimated before=%d after=%d", before.Estimated, after.Estimated)
	if after.Estimated != before.Estimated {
		t.Fatal("classification-only answer converted legacy estimated amount into reported amount")
	}
	state, _ = r.LoadChatState(ctx)
	if state.Purchases[0].ActualAmountBasis != "estimated" {
		t.Fatal("legacy basis not persisted before appending classification event")
	}
	s.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "explicit amount confirmed", PurchaseUpdates: []consultation.PurchaseUpdate{{ID: "p", Outcome: "purchased", Amount: &a, Classification: "necessary", AmountBasis: "reported"}}}
	})
	if _, e = s.Send(ctx, thread.ID, "actual payment confirmed"); e != nil {
		t.Fatal(e)
	}
	after, _ = r.ExpenseCount(ctx, "")
	if after.Estimated != 0 {
		t.Fatal("explicit reported confirmation retained estimated basis")
	}

}
