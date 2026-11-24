package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// The isolated schema is provided by check postgres. This covers only the
// explicitly gated writes; it is not evidence that all application writes drain.
func verifyWithdrawalDirectWrites(t *testing.T) {
	r := openReconciliationRepository(t)
	ctx := context.Background()
	a := identity.WithUser(ctx, 701)
	b := identity.WithUser(ctx, 702)
	profile := domain.Profile{MonthlyIncome: 100, AdviceStrictness: "normal", UpdatedAt: "2026-01-01"}
	if err := r.PutProfile(a, profile); err != nil {
		t.Fatal(err)
	}
	item, err := r.SaveMemory(a, domain.MemoryItem{Type: "preference", Content: "before"})
	if err != nil {
		t.Fatal(err)
	}
	item.Content = "updated"
	if item, err = r.SaveMemory(a, item); err != nil || item.Content != "updated" {
		t.Fatalf("normal update failed: %#v %v", item, err)
	}
	if _, err := r.SaveMemory(b, item); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner updated memory: %v", err)
	}
	if err := r.DeleteMemory(b, item.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner deleted memory: %v", err)
	}
	temporary, err := r.SaveMemory(a, domain.MemoryItem{Type: "preference", Content: "temporary"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteMemory(a, temporary.ID); err != nil {
		t.Fatalf("normal delete failed: %v", err)
	}
	if _, err := r.SaveReview(a, domain.MonthlyReview{Month: "2026-01", CreatedAt: "2026-01-01", Summary: "before"}); err != nil {
		t.Fatal(err)
	}
	consultation, err := r.CreateConsultation(a, domain.Consultation{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v1',description='policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	if err := r.GrantConsent(a, 701, "v1"); err != nil {
		t.Fatalf("normal grant: %v", err)
	}
	if _, err := r.CancelChatRequest(a, "cancel-before"); err != nil {
		t.Fatalf("normal cancellation: %v", err)
	}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(a, 701); err != nil {
		t.Fatal(err)
	}
	profile.MonthlyIncome = 999
	if err := r.PutProfile(a, profile); err == nil {
		t.Fatal("pending user updated profile")
	}
	item.Content = "after"
	if _, err := r.SaveMemory(a, item); err == nil {
		t.Fatal("pending user updated memory")
	}
	if _, err := r.SaveMemory(a, domain.MemoryItem{Type: "preference", Content: "new"}); err == nil {
		t.Fatal("pending user added memory")
	}
	if err := r.DeleteMemory(a, item.ID); err == nil {
		t.Fatal("pending user deleted memory")
	}
	if _, err := r.SaveReview(a, domain.MonthlyReview{Month: "2026-02", CreatedAt: "2026-02-01"}); err == nil {
		t.Fatal("pending user added review")
	}
	if _, err := r.UpdateConsultationResult(a, consultation.ID, domain.ConsultationResultUpdate{Status: "done"}); err == nil {
		t.Fatal("pending user updated consultation")
	}
	if _, err := r.CreateConsultation(a, domain.Consultation{}); err == nil {
		t.Fatal("pending user created consultation")
	}
	if _, err := r.UpdateConsultationAdvice(a, consultation); err == nil {
		t.Fatal("pending user changed advice")
	}
	if err := r.AddMessage(a, consultation.ID, "user", "after"); err == nil {
		t.Fatal("pending user added message")
	}
	if err := r.GrantConsent(a, 701, "v1"); err == nil {
		t.Fatal("pending user granted consent")
	}
	// Revocation is safety-preserving cleanup and must remain possible even
	// after a withdrawal request has been accepted.
	if err := r.WithdrawConsent(a, 701); err != nil {
		t.Fatalf("consent withdrawal blocked: %v", err)
	}
	if _, err := r.CancelChatRequest(a, "cancel-pending"); err == nil {
		t.Fatal("pending user cancelled chat request")
	}
	row := domain.Transaction{TransactionDate: "2026-09-01", Amount: 10, TransactionType: "expense", Category: "学習", Fingerprint: "pending-import"}
	if _, err := r.ImportTransactions(a, []domain.Transaction{row}); err == nil {
		t.Fatal("pending user imported CSV")
	}
	var cancelled int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM chat_requests WHERE user_id=$1 AND request_id=$2`, 701, "cancel-pending").Scan(&cancelled); err != nil || cancelled != 0 {
		t.Fatalf("pending cancellation was persisted: %d %v", cancelled, err)
	}
	var imported int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE user_id=$1 AND fingerprint=$2`, 701, "pending-import").Scan(&imported); err != nil || imported != 0 {
		t.Fatalf("pending import was persisted: %d %v", imported, err)
	}
	got, err := r.GetProfile(a)
	if err != nil || got.MonthlyIncome != 100 {
		t.Fatalf("profile changed: %v %v", got.MonthlyIncome, err)
	}
	memories, err := r.ListMemories(a)
	if err != nil || len(memories) != 1 || memories[0].Content != "updated" {
		t.Fatalf("memory changed: %#v %v", memories, err)
	}
	var reviews int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM monthly_reviews WHERE user_id=$1`, 701).Scan(&reviews); err != nil || reviews != 1 {
		t.Fatalf("pending user's review count changed: %d %v", reviews, err)
	}
	if err := r.PutProfile(b, profile); err != nil {
		t.Fatalf("other owner blocked: %v", err)
	}
}
