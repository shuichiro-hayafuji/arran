package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strings"
	"testing"
	"time"
)

func TestBankChangesHoldUnknownInclusionAndSaveBothDimensionsAtomically(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 3434)
	change := func(key, value string) consultation.Change {
		return consultation.Change{Key: key, Value: value, Kind: "fact", Period: "2035-01", Operation: "set", Basis: "user_report"}
	}
	service := consultation.Service{Repository: r, DataChanges: r, Reconciliation: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		return consultation.Reply{Status: "answered", Answer: "保存", Changes: []consultation.Change{change("current_balance", "80000円"), change("monthly_free_budget", "10000円")}}
	})}
	thread, err := service.Send(ctx, 0, "残高8万円、自由予算1万円")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE STORE,2500\n2035-01-04,PRIVATE DEPOSIT,-5000\n"), csvimport.Options{Source: "PRIVATE BANK", SourceAccountName: "PRIVATE ACCOUNT"})
	batchResult, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := r.ImportDetails(ctx, batchResult.ImportID)
	result, err := r.MutateCSV(ctx, batch.ID, domain.MutationInput{ID: "cancel-finance", Action: "cancel", Version: batch.Version, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot})
	if err != nil || result.Pending != 2 {
		t.Fatal(result, err)
	}
	contexts, err := r.LoadDataChanges(ctx, "")
	if err != nil || len(contexts) != 2 {
		t.Fatal(contexts, err)
	}
	directions := map[string]bool{}
	for _, c := range contexts {
		directions[c.Operation] = true
		if len(c.Before) != 1 || len(c.After) != 0 || c.BalanceResolved || c.BudgetResolved {
			t.Fatal(c)
		}
	}
	if !directions["cancel_income"] || !directions["cancel_expense"] {
		t.Fatal(directions)
	}
	// A model attempting to complete only one financial dimension is rejected after
	// the existing maximum repairs, without posting facts or consuming the pending refs.
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		b, _ := json.Marshal(in)
		for _, private := range []string{"PRIVATE STORE", "PRIVATE BANK", "PRIVATE ACCOUNT", "fingerprint", "provider_transaction_id", "raw_data"} {
			if strings.Contains(string(b), private) {
				t.Fatal("private bank metadata sent", private)
			}
		}
		var updates []consultation.ReconciliationUpdate
		for _, c := range in.DataChangeContext {
			updates = append(updates, consultation.ReconciliationUpdate{Ref: c.Ref, Status: "applied"})
		}
		money := change("current_balance", "80000円")
		money.Basis = "reconciliation"
		return consultation.Reply{Status: "answered", Answer: "片側だけ", DataChangeUpdates: updates, Changes: []consultation.Change{money}}
	})
	failed, err := service.Send(ctx, thread.ID, "包含は不明です")
	if err != nil || failed.Source != "unavailable" {
		t.Fatal(failed.Source, err)
	}
	contexts, _ = r.LoadDataChanges(ctx, "")
	if len(contexts) != 2 {
		t.Fatal("partial response completed refs")
	}
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		var updates []consultation.ReconciliationUpdate
		for _, c := range in.DataChangeContext {
			updates = append(updates, consultation.ReconciliationUpdate{Ref: c.Ref, Status: "needs_information"})
		}
		return consultation.Reply{Status: "needs_information", Answer: "包含を確認します", Question: "取消した入出金は残高と自由予算に含まれていますか？", DataChangeUpdates: updates}
	})
	pending, err := service.Send(ctx, thread.ID, "別の相談")
	if err != nil || pending.Source == "unavailable" {
		t.Fatal(err, pending.Source)
	}
	state, _ := r.LoadChatState(ctx)
	for _, f := range state.Facts {
		if f.Active && f.Key == "current_balance" && f.Value != "80000円" {
			t.Fatal("unknown inclusion posted", f)
		}
	}
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		var updates []consultation.ReconciliationUpdate
		for _, c := range in.DataChangeContext {
			updates = append(updates, consultation.ReconciliationUpdate{Ref: c.Ref, Status: "applied"})
		}
		balance, budget := change("current_balance", "80000円"), change("monthly_free_budget", "10000円")
		balance.Basis = "reconciliation"
		budget.Basis = "reconciliation"
		return consultation.Reply{Status: "answered", Answer: "本人が両基準を確認", DataChangeUpdates: updates, Changes: []consultation.Change{balance, budget}}
	})
	saved, err := service.Send(ctx, thread.ID, "取消後の現在残高8万円、自由予算1万円です")
	if err != nil || saved.Source == "unavailable" {
		t.Fatal(err, saved.Source)
	}
	contexts, err = r.LoadDataChanges(ctx, "")
	if err != nil || len(contexts) != 0 {
		t.Fatal(contexts, err)
	}
	got, err := r.LoadChatThread(ctx, thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(saved)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Fatal("saved consultation differs")
	}
	count, _ := r.ExpenseCount(ctx, "")
	if count.PendingRecalculation != 0 {
		t.Fatal(count)
	}
	audit, err := r.ChangeAudit(ctx, contextsRef(saved.Reply))
	if err != nil || len(audit) != 2 {
		t.Fatal(audit, err)
	} // needs_information and applied
	rows, _ := r.ListTransactions(ctx, "", 100)
	if len(rows) != 0 {
		t.Fatal("financial answer restored bank rows")
	}
	// Ownership applies to bank-only operation refs too.
	if foreign, e := r.LoadDataChanges(identity.WithUser(context.Background(), 3435), ""); e != nil || len(foreign) != 0 {
		t.Fatal(foreign, e)
	}
}
func contextsRef(r consultation.Reply) string { return r.DataChangeUpdates[0].Ref }

func TestPartialBankChangesAllowOnlyExplicitBothUserBaselines(t *testing.T) {
	r := openTestRepository(t)
	for index, mode := range []string{"over_limit", "forgotten_rows"} {
		ctx := identity.WithUser(context.Background(), int64(3436+index))
		state, _ := r.LoadChatState(ctx)
		state.Facts = []consultation.Fact{{Key: "current_balance", Value: "80000円", Active: true}, {Key: "monthly_free_budget", Value: "10000円", Active: true}}
		if mode == "forgotten_rows" {
			state.Facts = append(state.Facts, consultation.Fact{Key: "old-topic", Kind: "deleted", RecordedAt: "2021-01-01T00:00:00Z"})
		}
		thread, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2026-10-12T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "baseline"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}})
		if err != nil {
			t.Fatal(err)
		}
		csv := "date,description,amount\n"
		count := 21
		if mode == "forgotten_rows" {
			count = 2
		}
		for i := 0; i < count; i++ {
			csv += fmt.Sprintf("2035-01-%02d,PRIVATE MERCHANT %d,%d\n", i+1, i, 1000+i)
		}
		parsed, _ := csvimport.Parse([]byte(csv), csvimport.Options{Source: "bank", SourceAccountName: "account"})
		if mode == "forgotten_rows" {
			parsed.Transactions[0].ImportedAt = "2000-01-01T00:00:00Z"
			parsed.Transactions[1].ImportedAt = "2030-01-01T00:00:00Z"
		}
		committed, err := r.ImportTransactions(ctx, parsed.Transactions)
		if err != nil {
			t.Fatal(err)
		}
		batch, _ := r.ImportDetails(ctx, committed.ImportID)
		if _, err = r.MutateCSV(ctx, batch.ID, domain.MutationInput{ID: "partial-cancel", Action: "cancel", Version: batch.Version, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot}); err != nil {
			t.Fatal(err)
		}
		contexts, err := r.LoadDataChanges(ctx, "")
		if err != nil || len(contexts) != 1 || contexts[0].Omitted != 1 {
			t.Fatal(mode, contexts, err)
		}
		ref := contexts[0].Ref
		service := consultation.Service{Repository: r, DataChanges: r}
		for _, invalid := range []string{"one_axis", "inferred_basis", "partial_applied", "inference_kind", "past_period", "budget_inference", "balance_past", "duplicate_axis"} {
			service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
				changes := []consultation.Change{{Key: "current_balance", Value: "80000円", Operation: "set", Kind: "fact", Basis: "user_report"}}
				if invalid != "one_axis" {
					changes = append(changes, consultation.Change{Key: "monthly_free_budget", Value: "10000円", Operation: "set", Kind: "fact", Basis: "user_report"})
				}
				status := "baseline_adopted"
				if invalid == "partial_applied" {
					status = "applied"
				}
				if invalid == "inferred_basis" {
					changes[0].Basis = "reconciliation"
					changes[1].Basis = "reconciliation"
				}
				if invalid == "inference_kind" {
					changes[0].Kind = "inference"
					changes[1].Kind = "inference"
				}
				if invalid == "past_period" {
					changes[0].Period = "2020-01"
					changes[1].Period = "2020-01"
				}
				if invalid == "budget_inference" {
					changes[1].Kind = "inference"
				}
				if invalid == "balance_past" {
					changes[0].Period = "2020-01"
				}
				if invalid == "duplicate_axis" {
					changes = append(changes, changes[0])
				}
				return consultation.Reply{Status: "answered", Answer: "採用", Changes: changes, DataChangeUpdates: []consultation.ReconciliationUpdate{{Ref: ref, Status: status}}}
			})
			result, e := service.Send(ctx, thread.ID, "基準は未確認")
			if e != nil || result.Source != "unavailable" {
				t.Fatal(mode, invalid, e, result.Source)
			}
		}
		for _, invalid := range []string{"inference_kind", "past_period", "one_axis"} {
			priorState, _ := r.LoadChatState(ctx)
			priorThread, _ := r.LoadChatThread(ctx, thread.ID)
			beforeState, _ := json.Marshal(priorState)
			beforeThread, _ := json.Marshal(priorThread)
			// Simulate an adapter caller that has already applied tentative facts.
			var tentative consultation.State
			json.Unmarshal(beforeState, &tentative)
			for i := range tentative.Facts {
				if tentative.Facts[i].Active && tentative.Facts[i].Key == "current_balance" {
					tentative.Facts[i].Value = "999999円"
				}
			}
			changes := []consultation.Change{{Key: "current_balance", Value: "999999円", Operation: "set", Kind: "fact", Basis: "user_report"}, {Key: "monthly_free_budget", Value: "12000円", Operation: "set", Kind: "fact", Basis: "user_report"}}
			if invalid == "inference_kind" {
				changes[0].Kind = "inference"
			}
			if invalid == "past_period" {
				changes[1].Period = "2020-01"
			}
			if invalid == "one_axis" {
				changes = changes[:1]
			}
			attempted := priorThread
			attempted.Reply = consultation.Reply{Status: "answered", Answer: "invalid direct save", Changes: changes, DataChangeUpdates: []consultation.ReconciliationUpdate{{Ref: ref, Status: "baseline_adopted"}}}
			if _, err = r.SaveChatTurn(ctx, tentative, attempted); !errors.Is(err, consultation.ErrConflict) {
				t.Fatal("direct invalid baseline accepted", mode, invalid, err)
			}
			afterState, _ := r.LoadChatState(ctx)
			afterThread, _ := r.LoadChatThread(ctx, thread.ID)
			stateJSON, _ := json.Marshal(afterState)
			threadJSON, _ := json.Marshal(afterThread)
			if string(beforeState) != string(stateJSON) || string(beforeThread) != string(threadJSON) {
				t.Fatal("invalid direct save was not atomic", mode, invalid)
			}
			pending, e := r.LoadDataChanges(ctx, "")
			if e != nil || len(pending) != 1 || pending[0].BalanceResolved || pending[0].BudgetResolved {
				t.Fatal("direct invalid baseline completed receipt", pending, e)
			}
			audit, e := r.ChangeAudit(ctx, ref)
			if e != nil || len(audit) != 0 {
				t.Fatal("rejected baseline appended audit", audit, e)
			}
		}
		service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
			return consultation.Reply{Status: "answered", Answer: "本人の新しい基準を採用", Changes: []consultation.Change{{Key: "current_balance", Value: "82000円", Operation: "set", Kind: "fact", Basis: "user_report", Period: func() string {
				if mode == "forgotten_rows" {
					return time.Now().In(time.FixedZone("Asia/Tokyo", 9*3600)).Format("2006-01")
				}
				return ""
			}()}, {Key: "monthly_free_budget", Value: "12000円", Operation: "set", Kind: "fact", Basis: "user_report", Period: func() string {
				if mode == "forgotten_rows" {
					return time.Now().In(time.FixedZone("Asia/Tokyo", 9*3600)).Format("2006-01")
				}
				return ""
			}()}}, DataChangeUpdates: []consultation.ReconciliationUpdate{{Ref: ref, Status: "baseline_adopted"}}}
		})
		result, err := service.Send(ctx, thread.ID, "変更後の現在残高82000円、自由予算12000円です")
		if err != nil || result.Source == "unavailable" {
			t.Fatal(mode, err, result.Source)
		}
		contexts, err = r.LoadDataChanges(ctx, "")
		if err != nil || len(contexts) != 0 {
			t.Fatal(mode, contexts, err)
		}
		state, _ = r.LoadChatState(ctx)
		found := 0
		for _, f := range state.Facts {
			if f.Active && ((f.Key == "current_balance" && f.Value == "82000円") || (f.Key == "monthly_free_budget" && f.Value == "12000円")) {
				found++
			}
		}
		if found != 2 {
			t.Fatal("baseline not persisted")
		}
		audit, err := r.ChangeAudit(ctx, ref)
		if err != nil || len(audit) != 1 || !strings.Contains(audit[0].After, "baseline_adopted") {
			t.Fatal(audit, err)
		}
		t.Log(mode, "both user baselines adopted; partial inference rejected")
	}
}

func TestBankCorrectionExcludesPendingTripletAndCancelKeepsSamePurchase(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 3438)
	actual := int64(2500)
	state, _ := r.LoadChatState(ctx)
	state.Purchases = []consultation.Purchase{{ID: "same-owner-purchase", Status: "purchased", Classification: "free", ActualAmount: &actual, BalanceAppliedAmount: &actual, FreeBudgetAppliedAmount: &actual, AccountingTracked: true, RecordedAt: "2035-01-01T00:00:00Z", PurchaseDate: "2035-01-03"}}
	state.Facts = []consultation.Fact{{Key: "current_balance", Value: "80000円", Active: true}, {Key: "monthly_free_budget", Value: "10000円", Active: true}}
	thread, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "実績"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE BANK RECORD,2500\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	conflicts, err := r.PreviewReconciliations(ctx, parsed.Transactions)
	if err != nil || len(conflicts) != 1 {
		t.Fatal(conflicts, err)
	}
	parsed.Transactions[0].Reconciliation = &domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: state.Purchases[0].ID, StateVersion: conflicts[0].StateVersion, Snapshot: conflicts[0].Candidates[0].Snapshot}
	imported, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := r.ImportDetails(ctx, imported.ImportID)
	result, err := r.MutateCSV(ctx, batch.ID, domain.MutationInput{ID: "cancel-same", Action: "cancel", Version: batch.Version, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot})
	if err != nil || result.Pending != 1 {
		t.Fatal(result, err)
	}
	contexts, err := r.LoadDataChanges(ctx, "")
	if err != nil || len(contexts) != 1 || len(contexts[0].RelatedPurchaseIDs) != 1 {
		t.Fatal(contexts, err)
	}
	state, _ = r.LoadChatState(ctx)
	if state.Purchases[0].Status != "purchased" || *state.Purchases[0].ActualAmount != actual {
		t.Fatal("bank cancellation erased actual purchase", state.Purchases)
	}
	count, _ := r.ExpenseCount(ctx, "2035-01")
	if count.Confirmed != 1 || count.PendingRecalculation != 1 {
		t.Fatal(count)
	}
	// The bank-only receipt is still visible even while its original bank row is inactive.
	service := consultation.Service{Repository: r, DataChanges: r, Reconciliation: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.Purchases) != 1 || len(in.DataChangeContext) != 1 || len(in.Transactions) != 0 {
			t.Fatal("same purchase/context coverage missing", in)
		}
		return consultation.Reply{Status: "needs_information", Answer: "購入は保持しています", Question: "取消後の残高と自由予算の基準を確認できますか？", DataChangeUpdates: []consultation.ReconciliationUpdate{{Ref: in.DataChangeContext[0].Ref, Status: "needs_information"}}}
	})}
	if _, err = service.Send(ctx, thread.ID, "銀行取込だけを取り消しました"); err != nil {
		t.Fatal(err)
	}
	// A separate unlinked correction is represented once: the pending version is
	// excluded from normal transaction triplets, then becomes readable after resolution.
	other := identity.WithUser(context.Background(), 3439)
	plain := parsed.Transactions[0]
	plain.Reconciliation = nil
	plain.Fingerprint += "-other"
	plain.IdentityVersion = 0
	plainResult, err := r.ImportTransactions(other, []domain.Transaction{plain})
	if err != nil {
		t.Fatal(err)
	}
	plainBatch, _ := r.ImportDetails(other, plainResult.ImportID)
	row := plainBatch.Rows[0].Current
	newAmount := int64(1800)
	if _, err = r.MutateCSV(other, fmt.Sprint(row.ID), domain.MutationInput{ID: "correct-unlinked", Action: "correct", Version: row.Revision, StateVersion: plainBatch.StateVersion, Amount: &newAmount}); err != nil {
		t.Fatal(err)
	}
	rows, coverage, err := r.LoadChatTransactions(other, 100, "")
	if err != nil || len(rows) != 0 || coverage.DataChangeExcluded != 1 {
		t.Fatal(rows, coverage, err)
	}
	service = consultation.Service{Repository: r, DataChanges: r, Agent: reconciliationAgent(func(in consultation.Input) consultation.Reply {
		var updates []consultation.ReconciliationUpdate
		for _, c := range in.DataChangeContext {
			updates = append(updates, consultation.ReconciliationUpdate{Ref: c.Ref, Status: "baseline_adopted"})
		}
		return consultation.Reply{Status: "answered", Answer: "新基準を保存", DataChangeUpdates: updates, Changes: []consultation.Change{{Key: "current_balance", Value: "82000円", Kind: "fact", Operation: "set", Basis: "user_report"}, {Key: "monthly_free_budget", Value: "12000円", Kind: "fact", Operation: "set", Basis: "user_report"}}}
	})}
	completed, err := service.Send(other, 0, "現在の残高82000円と自由予算12000円を採用してください")
	if err != nil || completed.Source == "unavailable" {
		t.Fatal(err, completed.Source)
	}
	service.Agent = reconciliationAgent(func(in consultation.Input) consultation.Reply {
		if len(in.DataChangeContext) != 0 || len(in.Transactions) != 1 || in.Transactions[0].Amount != 1800 {
			t.Fatal(in)
		}
		return consultation.Reply{Status: "answered", Answer: "基準を保持"}
	})
	if _, err = service.Send(other, completed.ID, "次の相談"); err != nil {
		t.Fatal(err)
	}
	state, _ = r.LoadChatState(other)
	for _, f := range state.Facts {
		if f.Active && f.Key == "current_balance" && f.Value != "82000円" {
			t.Fatal("settled bank record posted again", f)
		}
	}
}
