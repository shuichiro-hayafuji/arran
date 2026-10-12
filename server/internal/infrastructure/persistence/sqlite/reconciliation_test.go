package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strings"
	"testing"
)

func TestPurchaseCSVDecisionAtomicityAndCount(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 701)
	other := identity.WithUser(context.Background(), 702)
	state, err := r.LoadChatState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	amount := int64(2500)
	state.Purchases = []consultation.Purchase{{ID: "purchase-one", Description: "イヤホン", Amount: 3000, ActualAmount: &amount, Status: "purchased", Classification: "free", AccountingTracked: true, BalanceAppliedAmount: &amount, FreeBudgetAppliedAmount: &amount, RecordedAt: "2035-01-01T00:00:00Z"}}
	thread, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "購入"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}})
	if err != nil {
		t.Fatal(err)
	}
	state, err = r.LoadChatState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oldState := state
	parsed, err := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE STORE,2500\n"), csvimport.Options{Source: "PRIVATE BANK", SourceAccountName: "PRIVATE ACCOUNT"})
	if err != nil {
		t.Fatal(err)
	}
	conflicts, err := r.PreviewReconciliations(ctx, parsed.Transactions)
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("conflicts=%+v %v", conflicts, err)
	}
	if _, err = r.ImportTransactions(ctx, parsed.Transactions); err == nil {
		t.Fatal("unconfirmed purchase/CSV saved")
	}
	rows, _ := r.ListTransactions(ctx, "", 100)
	if len(rows) != 0 {
		t.Fatal("unconfirmed batch partially saved")
	}
	parsed.Transactions[0].Reconciliation = &domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: "purchase-one", Snapshot: conflicts[0].Candidates[0].Snapshot, StateVersion: conflicts[0].StateVersion}
	result, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil || result.ImportedCount != 1 {
		t.Fatalf("result=%+v %v", result, err)
	}
	count, err := r.ExpenseCount(ctx, "2035-01")
	if err != nil || count.Confirmed != 1 || count.Undated != 0 || count.PendingRecalculation != 0 {
		t.Fatalf("count=%+v %v", count, err)
	}
	count, err = r.ExpenseCount(other, "")
	if err != nil || count.Confirmed+count.Undated != 0 {
		t.Fatalf("owner leak %+v %v", count, err)
	}
	if _, err = r.SaveChatTurn(ctx, oldState, thread); !errors.Is(err, consultation.ErrConflict) {
		t.Fatalf("CSV did not fence stale chat: %v", err)
	}
	result, err = r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil || result.ImportedCount != 0 || result.DuplicateCount != 1 {
		t.Fatalf("reimport=%+v %v", result, err)
	}
	contexts, err := r.LoadReconciliations(ctx, "")
	if err != nil || len(contexts) != 0 {
		t.Fatalf("contexts=%+v %v", contexts, err)
	}
	data, _ := json.Marshal(contexts)
	if strings.Contains(string(data), "PRIVATE") {
		t.Fatalf("private bank fields in model context")
	}
	rows, _ = r.ListTransactions(ctx, "", 100)
	if len(rows) != 1 || rows[0].Description != "PRIVATE STORE" || rows[0].Amount != 2500 {
		t.Fatal("bank record changed")
	}
	if err = r.ResolvePurchaseCSV(other, rows[0].ID, domain.ReconciliationDecision{Action: "same", PurchaseID: "purchase-one"}); err == nil {
		t.Fatal("other owner resolved link")
	}
}
