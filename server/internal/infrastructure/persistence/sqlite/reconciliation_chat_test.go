package sqlite

import (
	"context"
	"encoding/json"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strings"
	"testing"
	"time"
)

type reconciliationAgent func(consultation.Input) consultation.Reply

func (f reconciliationAgent) Advise(_ context.Context, in consultation.Input) (consultation.Reply, error) {
	return f(in), nil
}
func TestReconciledCSVDoesNotPostAgainAndKeepsLatestCorrection(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 711)
	amount := int64(2500)
	product := "イヤホン"
	service := consultation.Service{Repository: r, Reconciliation: r, Now: func() time.Time { return time.Date(2035, 1, 4, 0, 0, 0, 0, time.UTC) }}
	change := func(key, value, basis string) consultation.Change {
		return consultation.Change{Key: key, Value: value, Kind: "fact", Period: "2035-01", Operation: "set", Basis: basis}
	}
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "予定を保存", PurchaseChange: &consultation.PurchaseChange{Operation: "add", Description: &product, Amount: &amount}, Changes: []consultation.Change{change("current_balance", "50000円", "user_report"), change("monthly_free_budget", "30000円", "user_report")}}
	})
	thread, err := service.Send(ctx, 0, "イヤホン2500円。残高5万円、予算3万円")
	if err != nil {
		t.Fatal(err)
	}
	pid := thread.Purchases[0].ID
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "購入実績", PurchaseUpdates: []consultation.PurchaseUpdate{{ID: pid, Outcome: "purchased", Amount: &amount, Classification: "free"}}, Changes: []consultation.Change{change("current_balance", "47500円", "purchase"), change("monthly_free_budget", "27500円", "purchase")}}
	})
	if _, err = service.Send(ctx, thread.ID, "2500円で購入した"); err != nil {
		t.Fatal(err)
	}
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE STORE,2500\n"), csvimport.Options{Source: "PRIVATE BANK", SourceAccountName: "PRIVATE ACCOUNT"})
	conflicts, err := r.PreviewReconciliations(ctx, parsed.Transactions)
	if err != nil || len(conflicts) != 1 {
		t.Fatal(conflicts, err)
	}
	parsed.Transactions[0].Reconciliation = &domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: pid, Snapshot: conflicts[0].Candidates[0].Snapshot, StateVersion: conflicts[0].StateVersion}
	if _, err = r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.Transactions) != 0 || len(in.ReconciliationContext) != 0 || in.TransactionCoverage.LinkedExcluded != 1 {
			t.Fatalf("duplicate input: %+v", in)
		}
		body, _ := json.Marshal(in)
		if strings.Contains(string(body), "PRIVATE") {
			t.Fatal("private CSV fields leaked")
		}
		return consultation.Reply{Status: "answered", Answer: "同じ購入のため追加反映なし"}
	})
	if _, err = service.Send(ctx, thread.ID, "確認して"); err != nil {
		t.Fatal(err)
	}
	state, _ := r.LoadChatState(ctx)
	assertFinancial := func(balance, budget string) {
		t.Helper()
		s, _ := r.LoadChatState(ctx)
		values := map[string]string{}
		for _, f := range s.Facts {
			if f.Active {
				values[f.Key] = f.Value
			}
		}
		if values["current_balance"] != balance || values["monthly_free_budget"] != budget {
			t.Fatalf("values=%+v", values)
		}
	}
	assertFinancial("47500円", "27500円")
	if len(state.Purchases[0].Events) != 1 {
		t.Fatal("no-change added a purchase event")
	}
	if _, err = r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	assertFinancial("47500円", "27500円")
	corrected := int64(1800)
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "実額訂正", PurchaseUpdates: []consultation.PurchaseUpdate{{ID: pid, Outcome: "purchased", Amount: &corrected, Classification: "free"}}, Changes: []consultation.Change{change("current_balance", "48200円", "purchase"), change("monthly_free_budget", "28200円", "purchase")}}
	})
	if _, err = service.Send(ctx, thread.ID, "実額は1800円だった"); err != nil {
		t.Fatal(err)
	}
	assertFinancial("48200円", "28200円")
	rows, _ := r.ListTransactions(ctx, "", 100)
	if len(rows) != 1 || rows[0].Amount != 2500 {
		t.Fatal("raw bank amount overwritten or difference expense fabricated")
	}
	count, _ := r.ExpenseCount(ctx, "")
	if count.Confirmed != 1 || count.Undated != 0 || count.PendingRecalculation != 0 {
		t.Fatalf("latest reflected correction is still pending: count=%+v", count)
	}
	if _, err = r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.Transactions) != 0 || len(in.ReconciliationContext) != 0 {
			t.Fatal("completed correction revived duplicate CSV or pending context")
		}
		return consultation.Reply{Status: "answered", Answer: "latest correction retained"}
	})
	if _, err = service.Send(ctx, thread.ID, "repeat check"); err != nil {
		t.Fatal(err)
	}
	assertFinancial("48200円", "28200円")
	count, _ = r.ExpenseCount(ctx, "")
	if count.Confirmed != 1 || count.PendingRecalculation != 0 {
		t.Fatal(count)
	}
	state, _ = r.LoadChatState(ctx)
	var revisionCount int
	if err = r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT count(*) FROM purchase_csv_receipts WHERE user_id=:user_id AND purchase_revision=? AND status='applied'"), consultation.PurchaseSnapshot(state.Purchases[0])).Scan(&revisionCount); err != nil || revisionCount != 1 {
		t.Fatal("missing completed latest revision receipt", revisionCount, err)
	}

}
func TestCSVFirstUnknownInclusionHoldsFinancialFacts(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 712)
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE STORE,2500\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if _, err := r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	amount := int64(2500)
	product := "イヤホン"
	s := consultation.Service{Repository: r, Reconciliation: r}
	change := func(key, value, basis string) consultation.Change {
		return consultation.Change{Key: key, Value: value, Kind: "fact", Period: "2035-01", Operation: "set", Basis: basis}
	}
	s.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "予定", PurchaseChange: &consultation.PurchaseChange{Operation: "add", Description: &product, Amount: &amount}, Changes: []consultation.Change{change("current_balance", "50000円", "user_report"), change("monthly_free_budget", "30000円", "user_report")}}
	})
	thread, err := s.Send(ctx, 0, "イヤホン2500円、残高5万円、予算3万円")
	if err != nil {
		t.Fatal(err)
	}
	pid := thread.Purchases[0].ID
	s.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.ReconciliationRequiredIDs) != 1 {
			t.Fatal("missing CSV identity hold")
		}
		return consultation.Reply{Status: "needs_information", Answer: "購入実額を保存、金融反映は照合待ち", Question: "同じCSV明細ですか", PurchaseUpdates: []consultation.PurchaseUpdate{{ID: pid, Outcome: "purchased", Amount: &amount, Classification: "free"}}}
	})
	if _, err = s.Send(ctx, thread.ID, "2500円で購入"); err != nil {
		t.Fatal(err)
	}
	state, _ := r.LoadChatState(ctx)
	if state.Purchases[0].BalanceAppliedAmount != nil || state.Purchases[0].FreeBudgetAppliedAmount != nil || !state.Purchases[0].CSVReconciliationPending {
		t.Fatal("unresolved identity posted money")
	}
	for _, f := range state.Facts {
		if f.Active && f.Key == "current_balance" && f.Value != "50000円" {
			t.Fatal("balance changed before same/distinct")
		}
	}
	candidates, err := r.PurchaseCSVCandidates(ctx, pid)
	if err != nil || len(candidates) != 1 {
		t.Fatal(candidates, err)
	}
	candidate := candidates[0]
	if err = r.ResolvePurchaseCSV(ctx, candidate.Transaction.ID, domain.ReconciliationDecision{Action: "same", PurchaseID: pid, Snapshot: candidate.Candidates[0].Snapshot, StateVersion: candidate.StateVersion}); err != nil {
		t.Fatal(err)
	}
	s.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.ReconciliationContext) != 1 || in.ReconciliationContext[0].BalanceResolved || in.ReconciliationContext[0].BudgetResolved {
			t.Fatal("unknown dimensions disappeared")
		}
		return consultation.Reply{Status: "needs_information", Answer: "残高だけ反映、予算は確認待ち", Question: "自由予算にも含まれていますか", ReconciliationUpdates: []consultation.ReconciliationUpdate{{Ref: in.ReconciliationContext[0].Ref, Status: "needs_information"}}, Changes: []consultation.Change{change("current_balance", "47500円", "reconciliation")}}
	})
	if _, err = s.Send(ctx, thread.ID, "残高には含まれていない。予算は不明"); err != nil {
		t.Fatal(err)
	}
	partial, _ := r.LoadChatState(ctx)
	if partial.Purchases[0].BalanceAppliedAmount == nil || partial.Purchases[0].FreeBudgetAppliedAmount != nil {
		t.Fatal("partial receipt lost axis separation")
	}
	s.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.ReconciliationContext) != 1 || !in.ReconciliationContext[0].BalanceResolved || in.ReconciliationContext[0].BudgetResolved {
			t.Fatal("settled balance or unknown budget lost")
		}
		return consultation.Reply{Status: "answered", Answer: "自由予算を反映", ReconciliationUpdates: []consultation.ReconciliationUpdate{{Ref: in.ReconciliationContext[0].Ref, Status: "applied"}}, Changes: []consultation.Change{change("monthly_free_budget", "27500円", "reconciliation")}}
	})
	if _, err = s.Send(ctx, thread.ID, "同じ購入。前の残高には含まれていない"); err != nil {
		t.Fatal(err)
	}
	state, _ = r.LoadChatState(ctx)
	if state.Purchases[0].BalanceAppliedAmount == nil || state.Purchases[0].CSVReconciliationPending {
		t.Fatal("receipt did not finish posting")
	}
	count, _ := r.ExpenseCount(ctx, "")
	if count.Confirmed != 1 || count.PendingRecalculation != 0 {
		t.Fatalf("count=%+v", count)
	}
}
