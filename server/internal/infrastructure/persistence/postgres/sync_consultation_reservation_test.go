package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestSyncConsultationReservation(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	a := identity.WithUser(ctx, 701)
	b := identity.WithUser(ctx, 702)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := r.ReserveSyncConsultation(a, "reservation-one", "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ReserveSyncConsultation(a, "reservation-one", "2026-11"); err == nil {
		t.Fatal("retry incorrectly granted a second execution")
	}
	if _, err = r.ReserveSyncConsultation(b, "reservation-one", "2026-11"); err != nil {
		t.Fatalf("same ID for another owner must be independent: %v", err)
	}
	if _, err = r.ReserveSyncConsultation(a, "reservation-two", "2026-11"); err != nil {
		t.Fatal(err)
	}
	if err = r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("active marker accepted: %v", err)
	}
	if err = r.FinishSyncConsultation(b, "reservation-two", true); err == nil {
		t.Fatal("other owner released reservation")
	}
	if err = r.FinishSyncConsultation(a, "reservation-one", true); err != nil {
		t.Fatal(err)
	}
	if err = r.FinishSyncConsultation(a, "reservation-one", true); err != nil {
		t.Fatal(err)
	}
	usage, err := r.MonthlyConsultationUsage(a, "2026-11")
	if err != nil || usage.Count != 1 {
		t.Fatalf("idempotent release changed another reservation: %+v %v", usage, err)
	}
	if err = r.FinishSyncConsultation(a, "reservation-two", false); err != nil {
		t.Fatal(err)
	}
	if err = r.FinishSyncConsultation(a, "reservation-two", true); err == nil {
		t.Fatal("completed reservation released")
	}
	if err = r.RequestWithdrawal(a, 701); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ReserveSyncConsultation(a, "reservation-three", "2026-11"); !errors.Is(err, identity.ErrWithdrawalPending) {
		t.Fatalf("pending owner reserved: %v", err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `UPDATE withdrawal_requests SET requested_at='2026-11-24 14:59:59+00' WHERE user_id=701; UPDATE sync_consultation_reservations SET state='active' WHERE id='reservation-two'`); err != nil {
		t.Fatal(err)
	}
	if err = r.RunWithdrawalBatch(ctx, time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("batch accepted active marker")
	}
}

func TestSyncConsultationReservationConcurrent(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 711)
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO monthly_consultation_usage(user_id,month,consultation_count) VALUES(711,'2026-11',4); INSERT INTO user_consultation_limits(user_id,monthly_limit) VALUES(711,7)`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		usage domain.ConsultationUsage
		err   error
	}
	reserveResults := make(chan result, 2)
	results := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			usage, err := r.ReserveSyncConsultation(owner, "same", "2026-11")
			reserveResults <- result{usage, err}
		}()
	}
	close(start)
	successes := 0
	for i := 0; i < 2; i++ {
		r := <-reserveResults
		if r.err == nil {
			successes++
			if r.usage.Count != 5 || r.usage.Limit != 7 || r.usage.Month != "2026-11" {
				t.Fatalf("reserved quota: %+v", r.usage)
			}
		}
	}
	if successes != 1 {
		t.Fatalf("execution grants = %d", successes)
	}
	usage, err := r.MonthlyConsultationUsage(owner, "2026-11")
	if err != nil || usage.Count != 5 || usage.Limit != 7 || usage.Month != "2026-11" {
		t.Fatalf("quota %v %v", usage, err)
	}
	if _, err = r.ReserveSyncConsultation(owner, "same", "2026-12"); err == nil {
		t.Fatal("same ID granted in different month")
	}
	start = make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- r.FinishSyncConsultation(owner, "same", true) }()
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	usage, err = r.MonthlyConsultationUsage(owner, "2026-11")
	if err != nil || usage.Count != 4 {
		t.Fatalf("duplicate release: %v %v", usage, err)
	}
}

func TestSyncConsultationContinuationFence(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	other := identity.WithUser(ctx, 702)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.BeginSyncContinuation(owner, "continue"); err != nil {
		t.Fatal(err)
	}
	if err := r.BeginSyncContinuation(owner, "continue"); err == nil {
		t.Fatal("duplicate granted")
	}
	if err := r.FinishSyncConsultation(other, "continue", false); err == nil {
		t.Fatal("other owner completed marker")
	}
	if err := r.FinishSyncConsultation(owner, "continue", true); err == nil {
		t.Fatal("continuation released quota")
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("active continuation admitted: %v", err)
	}
	if err := r.FinishSyncConsultation(owner, "continue", false); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); err != nil {
		t.Fatal(err)
	}
	if err := r.BeginSyncContinuation(owner, "later"); !errors.Is(err, identity.ErrWithdrawalPending) {
		t.Fatalf("pending continuation admitted: %v", err)
	}
}
