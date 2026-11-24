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

func TestWithdrawalNotificationFence(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	event := domain.MonthlyLimitEvent{UserID: 701, Month: "2026-11", Count: 12, Limit: 12, OccurredAt: time.Now()}
	if err := r.CompleteMonthlyLimitNotification(owner, event, nil); err == nil {
		t.Fatal("missing notification completion accepted")
	}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	claimed, err := r.ClaimMonthlyLimitNotification(owner, event)
	if err != nil || !claimed {
		t.Fatalf("claim %v: %v", claimed, err)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("sending accepted: %v", err)
	}
	if err := r.CompleteMonthlyLimitNotification(owner, event, errors.New("delivery uncertain")); err != nil {
		t.Fatal(err)
	}
	claimed, err = r.ClaimMonthlyLimitNotification(owner, event)
	if err != nil || !claimed {
		t.Fatalf("retry %v: %v", claimed, err)
	}
	if err := r.CompleteMonthlyLimitNotification(owner, event, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("uncertain retry accepted: %v", err)
	}
	// Simulate a late unresolved marker against an already-pending request, to
	// verify the batch fails closed even after an earlier admission.
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id,requested_at) VALUES(701,'2026-11-24 14:59:59+00')`); err != nil {
		t.Fatal(err)
	}
	if err := r.RunWithdrawalBatch(ctx, time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("batch accepted uncertain notification")
	}
	var active bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=701`).Scan(&active); err != nil || !active {
		t.Fatalf("account disabled: %v %v", active, err)
	}
	event.Month = "2026-12"
	if claimed, err := r.ClaimMonthlyLimitNotification(owner, event); err == nil || claimed {
		t.Fatalf("pending owner claimed notification: %v %v", claimed, err)
	}
}

func TestWithdrawalNotificationNotSent(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	event := domain.MonthlyLimitEvent{UserID: 701, Month: "2026-10", Count: 12, Limit: 12, OccurredAt: time.Now()}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if claimed, err := r.ClaimMonthlyLimitNotification(owner, event); err != nil || !claimed {
		t.Fatalf("claim %v %v", claimed, err)
	}
	if err := r.CompleteMonthlyLimitNotification(owner, event, domain.ErrNotificationNotSent); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); err != nil {
		t.Fatalf("definitely unsent notification blocked withdrawal: %v", err)
	}
}

func TestWithdrawalNotificationMigration(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openMigrationFixture(t, 17)
	ctx := context.Background()
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO users(id,username,password_hash) VALUES(701,'migration','x'); INSERT INTO admin_notifications(user_id,month,notification_type,consultation_count,consultation_limit,status,attempts,occurred_at,delivered_at) VALUES(701,'2026-01','monthly_consultation_limit',1,1,'delivered',1,now(),now()),(701,'2026-02','monthly_consultation_limit',1,1,'delivered',2,now(),now()),(701,'2026-03','monthly_consultation_limit',1,1,'sending',1,now(),NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, r.db.DB); err != nil {
		t.Fatal(err)
	}
	var safe, ambiguous int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE NOT withdrawal_uncertain), count(*) FILTER (WHERE withdrawal_uncertain) FROM admin_notifications`).Scan(&safe, &ambiguous); err != nil || safe != 1 || ambiguous != 2 {
		t.Fatalf("migration safety: %d %d %v", safe, ambiguous, err)
	}
}

func TestWithdrawalNotificationRace(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	owner := identity.WithUser(ctx, 701)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := lockActiveWithdrawalUser(ctx, tx, 701); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_notifications(user_id,month,notification_type,consultation_count,consultation_limit,status,attempts,occurred_at) VALUES(701,'2026-11','monthly_consultation_limit',12,12,'sending',1,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- r.RequestWithdrawal(owner, 701) }()
	deadline := time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("withdrawal did not wait for notification admission")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("notification race accepted: %v", err)
	}
}
