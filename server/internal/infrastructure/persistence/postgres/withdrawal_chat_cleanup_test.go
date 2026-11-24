package postgres

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"os"
	"testing"
	"time"
)

func TestWithdrawalChatCleanup(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	owner := identity.WithUser(ctx, 701)
	other := identity.WithUser(ctx, 702)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO monthly_consultation_usage(user_id,month,consultation_count) VALUES(701,'2026-11',1),(702,'2026-11',1); INSERT INTO chat_requests(user_id,request_id,payload_hash,month,new_consultation,quota_reserved,status,expires_at,cancel_requested,error_code) VALUES(701,'cleanup-a','x','2026-11',TRUE,TRUE,'processing',9999999999999,FALSE,''),(702,'cleanup-b','x','2026-11',TRUE,TRUE,'processing',9999999999999,FALSE,'')`); err != nil {
		t.Fatal(err)
	}
	if err := r.FailChatRequest(other, "cleanup-a", "cancelled"); err == nil {
		t.Fatal("other owner released reservation")
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT 1 FROM chat_requests WHERE user_id=701 AND request_id='cleanup-a' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- r.FailChatRequest(owner, "cleanup-a", "cancelled") }()
	if err = r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("accepted before cleanup: %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if err = r.FailChatRequest(owner, "cleanup-a", "different"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	receipt, err := r.GetChatRequest(owner, "cleanup-a")
	if err != nil || receipt.ErrorCode != "cancelled" {
		t.Fatalf("retry changed receipt: %+v %v", receipt, err)
	}
	var a, b int
	if err = r.db.DB.QueryRowContext(ctx, `SELECT consultation_count FROM monthly_consultation_usage WHERE user_id=701 AND month='2026-11'`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err = r.db.DB.QueryRowContext(ctx, `SELECT consultation_count FROM monthly_consultation_usage WHERE user_id=702 AND month='2026-11'`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	if a != 0 || b != 1 {
		t.Fatalf("quota after retry: owner=%d other=%d", a, b)
	}
	if err = r.RequestWithdrawal(owner, 701); err != nil {
		t.Fatalf("complete cleanup blocked: %v", err)
	}
}

func TestWithdrawalFailedChatReservedBlocks(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO chat_requests(user_id,request_id,payload_hash,month,new_consultation,quota_reserved,status,expires_at,cancel_requested,error_code) VALUES(701,'unfinished-cleanup','x','2026-11',TRUE,TRUE,'failed',1,FALSE,'cancelled')`); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("unreturned reservation admitted: %v", err)
	}
	if _, err := r.GetChatRequest(owner, "unfinished-cleanup"); err == nil {
		t.Fatal("missing quota incorrectly treated as released")
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("failed release cleared marker: %v", err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO monthly_consultation_usage(user_id,month,consultation_count) VALUES(701,'2026-11',1)`); err != nil {
		t.Fatal(err)
	}
	receipt, err := r.GetChatRequest(owner, "unfinished-cleanup")
	if err != nil || receipt.ErrorCode != "cancelled" || receipt.Status != "failed" {
		t.Fatalf("GET cleanup changed receipt: %+v %v", receipt, err)
	}
	if err := r.FailChatRequest(owner, "unfinished-cleanup", "different"); err != nil {
		t.Fatal(err)
	}
	receipt, err = r.GetChatRequest(owner, "unfinished-cleanup")
	if err != nil || receipt.ErrorCode != "cancelled" {
		t.Fatalf("retry changed receipt: %+v %v", receipt, err)
	}
	if err := r.RequestWithdrawal(owner, 701); err != nil {
		t.Fatalf("released reservation blocked: %v", err)
	}
}
