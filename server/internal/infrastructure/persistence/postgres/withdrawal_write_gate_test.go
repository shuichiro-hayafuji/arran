package postgres

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// This tests only writes executed on the gated transaction. It does not claim
// that existing HTTP handlers, detached work or external side effects are wired.
func verifyWithdrawalWriteGate(t *testing.T) {
	t.Helper()
	r := openReconciliationRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := r.db.DB.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	params := address.Query()
	params.Set("search_path", schema)
	address.RawQuery = params.Encode()
	peer, err := Open(ctx, address.String(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	owner := identity.WithUser(ctx, 701)
	entered := make(chan int, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseWrite := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseWrite()
	done := make(chan error, 1)
	go func() {
		done <- r.withWithdrawalWriteTx(owner, 701, func(c context.Context, tx withdrawalWriteSession) error {
			if _, ok := tx.(interface{ Commit() error }); ok {
				return errors.New("callback can commit the gate transaction")
			}
			var pid int
			if err := tx.QueryRowContext(c, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				return err
			}
			entered <- pid
			select {
			case <-release:
			case <-c.Done():
				return c.Err()
			}
			_, err := tx.ExecContext(c, `INSERT INTO memories(user_id,type,content,evidence,confidence,created_at,updated_at) VALUES(701,'test','gated write','',0,'2026','2026')`)
			return err
		})
	}()
	var pid int
	select {
	case pid = <-entered:
	case err := <-done:
		t.Fatalf("write exited before acquiring gate: %v", err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for write gate")
	}
	requested := make(chan error, 1)
	go func() { requested <- peer.RequestWithdrawal(owner, 701) }()
	deadline := time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT is_active FROM users%FOR NO KEY UPDATE')`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("admission did not wait for transaction")
		}
		time.Sleep(10 * time.Millisecond)
	}
	releaseWrite()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for gated write")
	}
	select {
	case err := <-requested:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for withdrawal")
	}
	called := false
	if err := peer.withWithdrawalWriteTx(owner, 701, func(context.Context, withdrawalWriteSession) error { called = true; return nil }); err == nil || called {
		t.Fatalf("write began after admission: called=%v err=%v", called, err)
	}
	var count int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM memories WHERE user_id=701 AND content='gated write'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed write: %d %v", count, err)
	}

	// Terminate the actual transaction connection after a write. A later commit
	// must fail and the synthetic write must not persist; no second lease exists.
	other := identity.WithUser(ctx, 702)
	killCalled, killed, callbackReturned := false, false, false
	var killErr error
	err = peer.withWithdrawalWriteTx(other, 702, func(c context.Context, tx withdrawalWriteSession) error {
		if _, err := tx.ExecContext(c, `INSERT INTO memories(user_id,type,content,evidence,confidence,created_at,updated_at) VALUES(702,'test','terminated write','',0,'2026','2026')`); err != nil {
			return err
		}
		var backend int
		if err := tx.QueryRowContext(c, `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
			return err
		}
		killCalled = true
		killErr = r.db.DB.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, backend).Scan(&killed)
		if killErr != nil || !killed {
			return errors.New("test backend termination failed")
		}
		callbackReturned = true
		return nil
	})
	if !killCalled || killErr != nil || !killed || !callbackReturned {
		t.Fatalf("backend termination was not confirmed: called=%v killed=%v returned=%v error=%v", killCalled, killed, callbackReturned, killErr)
	}
	if err == nil {
		t.Fatal("terminated transaction was reported committed")
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM memories WHERE user_id=702 AND content='terminated write'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("terminated transaction persisted: %d %v", count, err)
	}

	// A detached worker's transaction survives cancellation of the HTTP parent;
	// the other server must still wait until the actual worker transaction ends.
	parent, stopParent := context.WithCancel(ctx)
	detachedTimeout, cancelDetached := context.WithTimeout(context.WithoutCancel(parent), 15*time.Second)
	defer cancelDetached()
	detached := identity.WithUser(detachedTimeout, 711)
	detachedStarted := make(chan int, 1)
	detachedRelease := make(chan struct{})
	var detachedReleaseOnce sync.Once
	releaseDetached := func() { detachedReleaseOnce.Do(func() { close(detachedRelease) }) }
	defer releaseDetached()
	detachedDone := make(chan error, 1)
	go func() {
		detachedDone <- peer.withWithdrawalWriteTx(detached, 711, func(c context.Context, tx withdrawalWriteSession) error {
			var id int
			if err := tx.QueryRowContext(c, `SELECT pg_backend_pid()`).Scan(&id); err != nil {
				return err
			}
			detachedStarted <- id
			select {
			case <-detachedRelease:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	var detachedPID int
	select {
	case detachedPID = <-detachedStarted:
	case err := <-detachedDone:
		t.Fatalf("detached worker exited before acquiring gate: %v", err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for detached worker")
	}
	stopParent()
	detachedRequest := make(chan error, 1)
	go func() { detachedRequest <- r.RequestWithdrawal(identity.WithUser(ctx, 711), 711) }()
	deadline = time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		if err := r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT is_active FROM users%FOR NO KEY UPDATE')`, detachedPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("detached worker lost its transactional gate")
		}
		time.Sleep(10 * time.Millisecond)
	}
	releaseDetached()
	select {
	case err := <-detachedDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for detached worker")
	}
	select {
	case err := <-detachedRequest:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for detached withdrawal")
	}

	// A saturated write pool must fail closed rather than borrowing another
	// connection or reporting success before a transaction has begun.
	peer.db.DB.SetMaxOpenConns(1)
	poolStarted := make(chan struct{})
	poolRelease := make(chan struct{})
	var poolReleaseOnce sync.Once
	releasePool := func() { poolReleaseOnce.Do(func() { close(poolRelease) }) }
	defer releasePool()
	poolDone := make(chan error, 1)
	go func() {
		poolDone <- peer.withWithdrawalWriteTx(identity.WithUser(ctx, 712), 712, func(context.Context, withdrawalWriteSession) error {
			close(poolStarted)
			select {
			case <-poolRelease:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-poolStarted:
	case err := <-poolDone:
		t.Fatalf("pool worker exited before acquiring gate: %v", err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for pool worker")
	}
	short, cancelShort := context.WithTimeout(identity.WithUser(ctx, 713), 150*time.Millisecond)
	called = false
	err = peer.withWithdrawalWriteTx(short, 713, func(context.Context, withdrawalWriteSession) error { called = true; return nil })
	cancelShort()
	if err == nil || called {
		t.Fatalf("exhausted pool admitted write: called=%v err=%v", called, err)
	}
	releasePool()
	select {
	case err := <-poolDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for pool worker completion")
	}
	if err := peer.withWithdrawalWriteTx(identity.WithUser(ctx, 713), 713, func(context.Context, withdrawalWriteSession) error { return nil }); err != nil {
		t.Fatalf("pool did not recover: %v", err)
	}
}
