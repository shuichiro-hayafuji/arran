package postgres

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"testing"
)

func verifyInitialSameOnlyCompletesProvenAlreadyAppliedObservation(t *testing.T, r *Repository) {
	for i, tc := range []struct {
		name, classification, status string
		budget                       bool
		existing                     bool
		bankAmount                   string
		pending                      int
	}{
		{"new reflected free", "free", "purchased", true, false, "2500", 0},
		{"new reflected necessary", "necessary", "purchased", false, false, "2500", 0},
		{"unknown classification", "unknown", "needs_classification", false, false, "2500", 1},
		{"unresolved budget", "free", "purchased", false, false, "2500", 1},
		{"amount contradiction", "free", "purchased", true, false, "3000", 1},
		{"not purchased contradiction", "necessary", "not_purchased", false, false, "2500", 1},
		{"existing unknown inclusion", "free", "purchased", true, true, "2500", 1},
	} {
		t.Log(tc.name)
		{
			ctx := identity.WithUser(context.Background(), int64(920+i))
			amount := int64(2500)
			parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-03,BANK OBSERVATION,"+tc.bankAmount+"\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
			if tc.existing {
				if _, err := r.ImportTransactions(ctx, parsed.Transactions); err != nil {
					t.Fatal(err)
				}
			}
			state, _ := r.LoadChatState(ctx)
			p := consultation.Purchase{ID: "p", Status: tc.status, Classification: tc.classification, ActualAmount: &amount, BalanceAppliedAmount: &amount, AccountingTracked: true, RecordedAt: "2035-01-04T00:00:00Z"}
			if tc.budget {
				p.FreeBudgetAppliedAmount = &amount
			}
			if tc.status == "not_purchased" {
				p.ActualAmount = nil
				p.BalanceAppliedAmount = nil
			}
			state.Purchases = []consultation.Purchase{p}
			if _, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-04T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "record"}}, Reply: consultation.Reply{Status: "answered", Answer: "saved"}}); err != nil {
				t.Fatal(err)
			}
			conflicts, err := r.PreviewReconciliations(ctx, parsed.Transactions)
			if err != nil || len(conflicts) != 1 {
				t.Fatal(conflicts, err)
			}
			parsed.Transactions[0].Reconciliation = &domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: "p", Snapshot: conflicts[0].Candidates[0].Snapshot, StateVersion: conflicts[0].StateVersion}
			if _, err = r.ImportTransactions(ctx, parsed.Transactions); err != nil {
				t.Fatal(err)
			}
			count, err := r.ExpenseCount(ctx, "")
			if err != nil || count.PendingRecalculation != tc.pending {
				t.Fatal("incorrect automatic completion", count, err)
			}
			contexts, err := r.LoadReconciliations(ctx, "")
			if err != nil || len(contexts) != tc.pending {
				t.Fatal("pending context not available", contexts, err)
			}
			var receipts int
			if err = r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT count(*) FROM purchase_csv_receipts WHERE user_id=:user_id AND status='applied'")).Scan(&receipts); err != nil || receipts != 1-tc.pending {
				t.Fatal("initial receipt inconsistency", receipts, err)
			}
			if tc.status == "not_purchased" && count.Unresolved != 1 {
				t.Fatal("not-purchased conflict erased", count)
			}
		}
	}
}
