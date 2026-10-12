package sqlite

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"testing"
	"time"
)

func TestForgottenCSVDoesNotRequireOrReviveReconciliation(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 713)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2020-01-01,FORGOTTEN MERCHANT,100\n"), csvimport.Options{Source: "bank", SourceAccountName: "account", Now: old})
	if _, err := r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	state, _ := r.LoadChatState(ctx)
	state.Facts = append(state.Facts, consultation.Fact{Key: "forgotten-topic", Kind: "deleted", RecordedAt: "2021-01-01T00:00:00Z"})
	amount := int64(200)
	state.Purchases = append(state.Purchases, consultation.Purchase{ID: "new", Description: "新しい購入", ActualAmount: &amount, Status: "purchased", RecordedAt: "2022-01-01T00:00:00Z"})
	_, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2022-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "新しい購入"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}})
	if err != nil {
		t.Fatal(err)
	}
	required, err := r.RequiresCSVReconciliation(ctx, "new")
	if err != nil || required {
		t.Fatalf("forgotten CSV forced hold: %v %v", required, err)
	}
	candidates, err := r.PurchaseCSVCandidates(ctx, "new")
	if err != nil || len(candidates) != 0 {
		t.Fatal("forgotten candidate revived", candidates, err)
	}
	contexts, err := r.LoadReconciliations(ctx, "2021-01-01T00:00:00Z")
	if err != nil || len(contexts) != 0 {
		t.Fatal(contexts, err)
	}
	rows, _ := r.ListTransactions(ctx, "", 100)
	if len(rows) != 1 {
		t.Fatal("forget deleted bank record")
	}
	preview, err := r.PreviewReconciliations(ctx, parsed.Transactions)
	if err != nil || len(preview) != 0 {
		t.Fatal("reimport revived forgotten candidate", preview, err)
	}
	result, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil || result.ImportedCount != 0 || result.DuplicateCount != 1 {
		t.Fatal(result, err)
	}
	service := consultation.Service{Repository: r, Reconciliation: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.ReconciliationRequiredIDs) != 0 || len(in.ReconciliationContext) != 0 || len(in.Transactions) != 0 {
			t.Fatal("forgotten data revived in model input")
		}
		return consultation.Reply{Status: "answered", Answer: "新しい購入のみ確認"}
	})}
	if _, err = service.Send(ctx, 0, "新しい購入について"); err != nil {
		t.Fatal(err)
	}
}
