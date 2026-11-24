package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func verifyWithdrawalBatch(t *testing.T) {
	t.Helper()
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	verifyWithdrawalChatCSVLockOrder(t, r, owner)
	if err := r.RequestWithdrawal(owner, 701); err == nil {
		t.Fatalf("request accepted without scheduler health: %v", err)
	}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	consultationRecord, err := r.CreateConsultation(owner, domain.Consultation{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO chat_purchase_schedule(user_id,purchase_id,consultation_id,due_at,first_sent) VALUES(701,'pending-owner',$1,'2026-01-01T00:00:00Z',FALSE)`, consultationRecord.ID); err != nil {
		t.Fatal(err)
	}
	owners, err := r.DuePurchaseOwners(ctx, time.Now().Add(24*time.Hour))
	if err != nil || len(owners) != 1 || owners[0] != 701 {
		t.Fatalf("due owner not discovered: %v %v", owners, err)
	}
	other := identity.WithUser(ctx, 702)
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO ai_consent_inflight(id,user_id,version) VALUES('batch-busy',701,'test')`); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("busy request was accepted: %v", err)
	}
	var exists bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=701)`).Scan(&exists); err != nil || exists {
		t.Fatalf("busy request persisted: %v %v", exists, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `DELETE FROM ai_consent_inflight WHERE id='batch-busy'`); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(other, 701); err == nil {
		t.Fatal("cross-owner request accepted")
	}
	if err := r.RequestWithdrawal(owner, 701); err != nil {
		t.Fatal(err)
	}
	owners, err = r.DuePurchaseOwners(ctx, time.Now().Add(24*time.Hour))
	if err != nil || len(owners) != 0 {
		t.Fatalf("pending owner still eligible for reminders: %v %v", owners, err)
	}
	if _, err := r.SaveChatTurn(owner, consultation.State{}, consultation.Thread{}); err == nil {
		t.Fatal("background chat write accepted after withdrawal admission")
	}
	if err := r.RequestWithdrawal(owner, 701); err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := lockActiveWithdrawalUser(ctx, tx, 701); err == nil {
		t.Fatal("pending withdrawal admitted new work")
	}
	_ = tx.Rollback()
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE withdrawal_requests SET requested_at='2026-11-24 14:59:59+00' WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO auth_sessions(token_hash,user_id,password_hash,expires_at) VALUES('batch-owner',701,'disabled-test-only',CURRENT_TIMESTAMP+interval '1 hour'),('batch-other',702,'disabled-test-only',CURRENT_TIMESTAMP+interval '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO memories(user_id,type,content,evidence,confidence,created_at,updated_at) VALUES(701,'test','retained','',0,'2026-01-01','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	before := time.Date(2026, 11, 24, 14, 59, 59, 0, time.UTC)
	if err := r.RunWithdrawalBatch(ctx, before); err != nil {
		t.Fatal(err)
	}
	var active bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=701`).Scan(&active); err != nil || !active {
		t.Fatalf("processed before midnight: %v %v", active, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO ai_consent_inflight(id,user_id,version) VALUES('batch-late',701,'test')`); err != nil {
		t.Fatal(err)
	}
	midnight := time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)
	if err := r.RunWithdrawalBatch(ctx, midnight); err == nil {
		t.Fatal("unresolved send did not block batch")
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=701`).Scan(&active); err != nil || !active {
		t.Fatalf("blocked batch changed owner: %v %v", active, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `DELETE FROM ai_consent_inflight WHERE id='batch-late'`); err != nil {
		t.Fatal(err)
	}
	if err := r.RunWithdrawalBatch(ctx, midnight); err != nil {
		t.Fatal(err)
	}
	if err := r.RunWithdrawalBatch(ctx, midnight); err != nil {
		t.Fatal(err)
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=701`).Scan(&active); err != nil || active {
		t.Fatalf("owner still active: %v %v", active, err)
	}
	if _, err := r.ResolveSession(ctx, "batch-owner"); !errors.Is(err, auth.ErrInvalid) {
		t.Fatalf("owner session alive: %v", err)
	}
	if _, err := r.ResolveSession(ctx, "batch-other"); err != nil {
		t.Fatalf("other owner session changed: %v", err)
	}
	var retained, completed bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memories WHERE user_id=701 AND content='retained'), EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=701 AND completed_at IS NOT NULL)`).Scan(&retained, &completed); err != nil || !retained || !completed {
		t.Fatalf("lost data or completion: %v %v %v", retained, completed, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE users SET is_active=TRUE WHERE id=701`); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("reactivated user request silently accepted: %v", err)
	}
	// A request admitted before midnight may remain uncommitted while the batch
	// begins. The batch must wait for commit before taking its due-work snapshot.
	requestTx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer requestTx.Rollback()
	var blocker int
	if err := requestTx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	if _, err := requestTx.ExecContext(ctx, `SELECT pg_advisory_xact_lock_shared(1634890337,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := requestTx.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id,requested_at) VALUES(702,'2026-11-24 14:59:59+00')`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- r.RunWithdrawalBatch(ctx, midnight) }()
	deadline := time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT pg_advisory_xact_lock(1634890337,1)%')`, blocker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("midnight batch did not wait for request commit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := requestTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("batch after delayed request commit: %v", err)
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=702`).Scan(&active); err != nil || active {
		t.Fatalf("committed pre-midnight request missed: %v %v", active, err)
	}
	// Exercise the production admission path too: block its user row after it
	// has acquired the shared gate, then start a midnight batch. A synthetic
	// trigger fixes the request timestamp before midnight without changing the
	// host clock, allowing this race to run deterministically.
	if _, err := r.db.DB.ExecContext(ctx, `CREATE FUNCTION test_withdrawal_time() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.requested_at='2026-11-24 14:59:59+00'; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `CREATE TRIGGER test_withdrawal_time BEFORE INSERT ON withdrawal_requests FOR EACH ROW EXECUTE FUNCTION test_withdrawal_time()`); err != nil {
		t.Fatal(err)
	}
	rowTx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rowTx.Rollback()
	var rowBlocker int
	if err := rowTx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&rowBlocker); err != nil {
		t.Fatal(err)
	}
	if _, err := rowTx.ExecContext(ctx, `UPDATE users SET username=username WHERE id=711`); err != nil {
		t.Fatal(err)
	}
	requestDone := make(chan error, 1)
	go func() { requestDone <- r.RequestWithdrawal(identity.WithUser(ctx, 711), 711) }()
	deadline = time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT is_active FROM users%FOR NO KEY UPDATE')`, rowBlocker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("production request did not wait for owner lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	batchDone := make(chan error, 1)
	go func() { batchDone <- r.RunWithdrawalBatch(ctx, midnight) }()
	deadline = time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE cardinality(pg_blocking_pids(pid))>0 AND query LIKE 'SELECT pg_advisory_xact_lock(1634890337,1)%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("production request did not hold boundary gate")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := rowTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-requestDone; err != nil {
		t.Fatalf("production request failed: %v", err)
	}
	if err := <-batchDone; err != nil {
		t.Fatalf("batch after production request failed: %v", err)
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=711`).Scan(&active); err != nil || active {
		t.Fatalf("production request missed midnight batch: %v %v", active, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id) VALUES(712),(713)`); err != nil {
		t.Fatal(err)
	}
	blockedTx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blockedTx.Rollback()
	if _, err := blockedTx.ExecContext(ctx, `UPDATE users SET username=username WHERE id=712`); err != nil {
		t.Fatal(err)
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	deadlineErr := r.RunWithdrawalBatch(deadlineCtx, midnight)
	cancel()
	if deadlineErr == nil {
		t.Fatal("deadline did not stop the batch")
	}
	var stillPending bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT completed_at IS NULL FROM withdrawal_requests WHERE user_id=712`).Scan(&stillPending); err != nil || !stillPending {
		t.Fatalf("deadline was reported as completion: %v %v", stillPending, err)
	}
	started := time.Now()
	if err := r.RunWithdrawalBatch(ctx, midnight); err == nil {
		t.Fatal("locked owner did not leave batch incomplete")
	}
	if time.Since(started) > 10*time.Second {
		t.Fatal("one locked user stalled the batch")
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=713`).Scan(&active); err != nil || active {
		t.Fatalf("another eligible owner did not complete: %v %v", active, err)
	}
	if err := blockedTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := r.RunWithdrawalBatch(ctx, midnight); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=712`).Scan(&active); err != nil || active {
		t.Fatalf("locked owner not completed after retry: %v %v", active, err)
	}
}

// CSV confirmation holds an owner advisory lock and then checks a users FK
// (KEY SHARE); chat saving holds the user row while waiting for that advisory
// lock. The user lock must be NO KEY UPDATE so FK checks can finish.
func verifyWithdrawalChatCSVLockOrder(t *testing.T, r *Repository, owner context.Context) {
	t.Helper()
	ctx := context.Background()
	state, err := r.LoadChatState(owner)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var blocker int
	if err := tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout='2s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, -int64(702)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.SaveChatTurn(owner, state, consultation.Thread{CreatedAt: "2026-11-25T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "synthetic"}}})
		done <- err
	}()
	deadline := time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT pg_advisory_xact_lock($1)%')`, blocker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("chat did not wait for the CSV advisory lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=701 FOR KEY SHARE`).Scan(&id); err != nil {
		t.Fatalf("CSV FK lock blocked by chat owner lock: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("chat save after CSV transaction: %v", err)
	}
}
