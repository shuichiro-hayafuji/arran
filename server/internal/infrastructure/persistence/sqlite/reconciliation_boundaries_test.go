package sqlite

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"testing"
)

func TestDistinctAllResumesActualWithoutGuessingIdentity(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 714)
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE STORE,2500\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if _, err := r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	amount := int64(2500)
	state, _ := r.LoadChatState(ctx)
	state.Facts = []consultation.Fact{{Key: "current_balance", Value: "50000円", Active: true}, {Key: "monthly_free_budget", Value: "30000円", Active: true}}
	state.Purchases = []consultation.Purchase{{ID: "distinct", Description: "別の購入", Amount: 2500, ActualAmount: &amount, Status: "purchased", Classification: "free", AccountingTracked: true, CSVReconciliationPending: true, RecordedAt: "2035-01-04T00:00:00Z"}}
	thread, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-04T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "購入"}}, Reply: consultation.Reply{Status: "answered", Answer: "照合待ち"}})
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := r.PurchaseCSVCandidates(ctx, "distinct")
	if err != nil || len(candidates) != 1 {
		t.Fatal(candidates, err)
	}
	d := domain.ReconciliationDecision{Action: "distinct", PurchaseID: "distinct", Snapshot: candidates[0].Candidates[0].Snapshot, StateVersion: candidates[0].StateVersion}
	stale := d
	stale.StateVersion--
	if err = r.ResolvePurchaseCSV(ctx, 0, stale); err == nil {
		t.Fatal("stale choice saved")
	}
	if err = r.ResolvePurchaseCSV(ctx, 0, d); err != nil {
		t.Fatal(err)
	}
	required, err := r.RequiresCSVReconciliation(ctx, "distinct")
	if err != nil || required {
		t.Fatal(required, err)
	}
	count, err := r.ExpenseCount(ctx, "")
	if err != nil || count.Confirmed != 1 || count.Undated != 1 || count.Unresolved != 0 || count.PendingRecalculation != 1 {
		t.Fatalf("count=%+v %v", count, err)
	}
	service := consultation.Service{Repository: r, Reconciliation: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.ReconciliationRequiredIDs) != 0 || len(in.ReconciliationContext) != 0 || len(in.Transactions) != 1 {
			t.Fatal("distinct bank observation missing or hold retained")
		}
		return consultation.Reply{Status: "answered", Answer: "別の支出を反映", PurchaseUpdates: []consultation.PurchaseUpdate{{ID: "distinct", Outcome: "purchased", Amount: &amount, Classification: "free", AmountBasis: "unchanged"}}, Changes: []consultation.Change{{Key: "current_balance", Value: "47500円", Kind: "fact", Operation: "set", Basis: "purchase"}, {Key: "monthly_free_budget", Value: "27500円", Kind: "fact", Operation: "set", Basis: "purchase"}}}
	})}
	if _, err = service.Send(ctx, thread.ID, "別の購入でした"); err != nil {
		t.Fatal(err)
	}
	state, _ = r.LoadChatState(ctx)
	if state.Purchases[0].CSVReconciliationPending || state.Purchases[0].BalanceAppliedAmount == nil {
		t.Fatal("distinct outcome not posted")
	}
	count, _ = r.ExpenseCount(ctx, "")
	if count.Confirmed+count.Undated != 2 || count.PendingRecalculation != 0 {
		t.Fatal(count)
	}
}

func TestExpenseCountDateProvenanceAndUnpurchasedConflict(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 715)
	state, _ := r.LoadChatState(ctx)
	amount := int64(100)
	state.Purchases = []consultation.Purchase{
		{ID: "dated", ActualAmount: &amount, Status: "purchased", PurchaseDate: "2035-02-01", RecordedAt: "2035-01-01T00:00:00Z", ActualAmountBasis: "reported"},
		{ID: "estimated", ActualAmount: &amount, Status: "purchased", PurchaseDate: "2035-02-02", RecordedAt: "2035-01-01T00:00:00Z", ActualAmountBasis: "estimated"},
		{ID: "undated", ActualAmount: &amount, Status: "purchased", RecordedAt: "2035-01-01T00:00:00Z"},
		{ID: "planned", Amount: 100, Status: "pending", RecordedAt: "2035-01-01T00:00:00Z"},
		{ID: "cancelled", Status: "not_purchased", RecordedAt: "2035-01-01T00:00:00Z"},
	}
	if _, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "保存"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}}); err != nil {
		t.Fatal(err)
	}
	count, err := r.ExpenseCount(ctx, "2035-02")
	if err != nil || count.Confirmed != 2 || count.Estimated != 1 || count.Undated != 1 {
		t.Fatal(count, err)
	}
	count, _ = r.ExpenseCount(ctx, "2035-01")
	if count.Confirmed != 0 || count.Undated != 1 {
		t.Fatal("consultation date inferred as purchase date", count)
	}
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-02-03,BANK OBSERVATION,100\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	conflicts, err := r.PreviewReconciliations(ctx, parsed.Transactions)
	if err != nil || len(conflicts) != 1 {
		t.Fatal(conflicts, err)
	}
	var candidate domain.ReconciliationCandidate
	for _, c := range conflicts[0].Candidates {
		if c.ID == "cancelled" {
			candidate = c
		}
	}
	parsed.Transactions[0].Reconciliation = &domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: "cancelled", Snapshot: candidate.Snapshot, StateVersion: conflicts[0].StateVersion}
	if _, err = r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	count, _ = r.ExpenseCount(ctx, "2035-02")
	if count.Confirmed != 2 || count.Unresolved != 1 || count.PendingRecalculation != 1 {
		t.Fatal("bank debit contradiction falsely confirmed zero", count)
	}
	rows, _ := r.ListTransactions(ctx, "", 100)
	if len(rows) != 1 {
		t.Fatal("bank observation removed")
	}
}
