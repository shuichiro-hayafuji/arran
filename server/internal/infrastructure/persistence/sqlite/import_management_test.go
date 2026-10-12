package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strconv"
	"testing"
)

func TestImportEvidenceCancelRestoreCorrectionAndAudit(t *testing.T) {
	r := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 3430)
	other := identity.WithUser(context.Background(), 3431)
	state, _ := r.LoadChatState(ctx)
	state.Facts = []consultation.Fact{{Key: "current_balance", Value: "80000円", Active: true}, {Key: "monthly_free_budget", Value: "10000円", Active: true}}
	if _, err := r.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "baseline"}}, Reply: consultation.Reply{Status: "answered", Answer: "saved"}}); err != nil {
		t.Fatal(err)
	}
	parsed, err := csvimport.Parse([]byte("date,description,amount\n2035-01-03,PRIVATE STORE,2500\n2035-01-04,PRIVATE DEPOSIT,-5000\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil || b.ImportedCount != 0 || b.DuplicateCount != 2 {
		t.Fatal(b, err)
	}
	cancel := func(id, key string) domain.MutationResult {
		t.Helper()
		batch, e := r.ImportDetails(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		result, e := r.MutateCSV(ctx, id, domain.MutationInput{ID: key, Action: "cancel", Version: batch.Version, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot})
		if e != nil {
			t.Fatal(e)
		}
		return result
	}
	protected := cancel(a.ImportID, "cancel-a")
	if protected.Affected != 0 || protected.Protected != 2 || protected.Pending != 0 {
		t.Fatal("duplicate evidence not protected", protected)
	}
	rows, err := r.ListTransactions(ctx, "", 100)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	expense := rows[1]
	if expense.TransactionType != "expense" {
		expense = rows[0]
	}
	version, _ := r.LoadChatState(ctx)
	amt := int64(1800)
	date := "2035-02-01"
	correction, err := r.MutateCSV(ctx, strconv.FormatInt(expense.ID, 10), domain.MutationInput{ID: "correct-amount-date", Action: "correct", Version: expense.Revision, StateVersion: version.Version, Amount: &amt, Date: &date})
	if err != nil || correction.Affected != 1 {
		t.Fatal(correction, err)
	}
	staleThread, _ := r.LoadChatThread(ctx, 1)
	if _, e := r.SaveChatTurn(ctx, version, staleThread); !errors.Is(e, consultation.ErrConflict) {
		t.Fatal("correction did not fence stale model", e)
	}
	updated, err := r.GetTransaction(ctx, expense.ID)
	if err != nil || updated.Amount != 1800 || updated.TransactionDate != "2035-02-01" || updated.Revision != 1 {
		t.Fatal(updated, err)
	}
	var originalAmount int64
	var originalDate string
	if err = r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT amount,transaction_date FROM transactions WHERE user_id=:user_id AND id=?"), expense.ID).Scan(&originalAmount, &originalDate); err != nil || originalAmount != 2500 || originalDate != "2035-01-03" {
		t.Fatal("original bank identity overwritten", err)
	}
	// Old CSV repeats preserve corrected values, and create their own evidence.
	c, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil || c.ImportedCount != 0 {
		t.Fatal(c, err)
	}
	cancel(c.ImportID, "cancel-c")
	removed := cancel(b.ImportID, "cancel-b")
	if removed.Affected != 2 || removed.Pending != 2 {
		t.Fatal("mixed directions not retained", removed)
	}
	rows, _ = r.ListTransactions(ctx, "", 100)
	if len(rows) != 0 {
		t.Fatal("cancelled rows still visible")
	}
	dashboard, err := r.MonthlyDashboard(ctx, "2035-02", domain.Profile{MonthlyFreeBudget: 30000})
	if err != nil || dashboard.ExpenseTotal != 0 {
		t.Fatal(dashboard, err)
	}
	count, err := r.ExpenseCount(ctx, "")
	if err != nil || count.Confirmed != 0 {
		t.Fatal(count, err)
	}
	required, err := r.RequiresCSVReconciliation(ctx, "new")
	if err != nil || required {
		t.Fatal("inactive rows require purchase reconciliation", err)
	}
	state, _ = r.LoadChatState(ctx)
	if state.Facts[0].Value != "80000円" || state.Facts[1].Value != "10000円" {
		t.Fatal("cancel command posted financial changes")
	}
	if _, err = r.ImportDetails(other, b.ImportID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("owner leak", err)
	}
	batch, _ := r.ImportDetails(ctx, b.ImportID)
	stale := domain.MutationInput{ID: "stale", Action: "restore", Version: batch.Version - 1, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot}
	if _, err = r.MutateCSV(ctx, b.ImportID, stale); !errors.Is(err, consultation.ErrConflict) {
		t.Fatal("stale version accepted", err)
	}
	if _, err = r.ImportTransactions(ctx, parsed.Transactions); err == nil {
		t.Fatal("unconfirmed reactivation saved")
	}
	for i := range parsed.Transactions {
		parsed.Transactions[i].ImportMetadata = &domain.ImportMetadata{ConfirmReactivation: true}
	}
	reimport, err := r.ImportTransactions(ctx, parsed.Transactions)
	if err != nil || reimport.ImportedCount != 0 || reimport.ReactivatedCount != 2 {
		t.Fatal(reimport, err)
	}
	updated, _ = r.GetTransaction(ctx, expense.ID)
	if updated.Amount != 1800 || updated.ID != expense.ID {
		t.Fatal("reactivation overwrote latest correction")
	}
	audit, err := r.ChangeAudit(ctx, strconv.FormatInt(expense.ID, 10))
	if err != nil || len(audit) != 1 {
		t.Fatal(audit, err)
	}
	var before []domain.Transaction
	json.Unmarshal([]byte(audit[0].Before), &before)
	if len(before) != 1 || before[0].Amount != 2500 {
		t.Fatal("audit before image missing")
	}
	saved, err := r.Mutation(ctx, removed.ID)
	if err != nil || saved != removed {
		t.Fatal("saved result differs", saved, err)
	}
}
