package postgres

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestAIConsentPostgres(t *testing.T) {
	verifyAIConsentPostgres(t)
}

func TestAIConsentIsolation(t *testing.T) { verifyAIConsentIsolation(t) }

func verifyAIConsentIsolation(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := identity.WithUser(context.Background(), 701)
	if _, err := r.db.Exec(`UPDATE ai_consent_policy SET version='v1',description='policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	gate := identity.AIConsentGate{Store: r, Version: "v1"}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := r.db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	params := address.Query()
	params.Set("search_path", schema)
	address.RawQuery = params.Encode()
	pool, err := sql.Open("pgx", address.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	if _, err = pool.Exec(`SET default_transaction_isolation = 'repeatable read'`); err != nil {
		t.Fatal(err)
	}
	repeatableDefault := &Repository{db: &database{DB: pool}}
	freshGate := identity.AIConsentGate{Store: repeatableDefault, Version: "v1"}

	// Reproduce a fixed snapshot that continues to see a grant after revocation.
	stale, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Rollback()
	var grant sql.NullString
	if err = stale.QueryRowContext(ctx, `SELECT granted_version FROM ai_user_consents WHERE user_id=701`).Scan(&grant); err != nil || grant.String != "v1" {
		t.Fatalf("snapshot: %v %v", grant, err)
	}
	if err = freshGate.Withdraw(ctx); err != nil {
		t.Fatalf("withdraw on RR-default session: %v", err)
	}
	if err = stale.QueryRowContext(ctx, `SELECT granted_version FROM ai_user_consents WHERE user_id=701`).Scan(&grant); err != nil || grant.String != "v1" {
		t.Fatalf("snapshot not fixed: %v %v", grant, err)
	}
	if err = freshGate.Execute(ctx, func(context.Context) error { t.Error("send after withdrawal"); return nil }); !errors.Is(err, identity.ErrConsentRequired) {
		t.Fatalf("stale grant accepted: %v", err)
	}
	if err = stale.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Admission reads policy and then waits on the user row. The withdrawal
	// commits while it is waiting, so an RR admission transaction would retain
	// the old grant snapshot unless the production code explicitly requests RC.
	if err = gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	blocker, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	var ownerID int64
	if err = blocker.QueryRowContext(ctx, `SELECT id FROM users WHERE id=701 FOR UPDATE`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	var blockerPID, admissionPID int
	if err = blocker.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&admissionPID); err != nil {
		t.Fatal(err)
	}
	admission := make(chan error, 1)
	go func() {
		admission <- freshGate.Execute(ctx, func(context.Context) error { t.Error("admission sent after withdrawal"); return nil })
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var blocked bool
		if err = r.db.QueryRowContext(ctx, `SELECT $1 = ANY(pg_blocking_pids($2))`, blockerPID, admissionPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("admission did not wait on user row")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = blocker.ExecContext(ctx, `UPDATE ai_user_consents SET granted_version=NULL,withdrawal_pending=FALSE,updated_at=CURRENT_TIMESTAMP WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	if err = blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-admission; !errors.Is(err, identity.ErrConsentRequired) {
		t.Fatalf("admission after concurrent withdrawal: %v", err)
	}

	// A policy writer holding an older snapshot must not miss a later marker.
	stale, err = r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Rollback()
	var current string
	if err = stale.QueryRowContext(ctx, `SELECT version FROM ai_consent_policy WHERE singleton=TRUE`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.ExecContext(ctx, `INSERT INTO ai_consent_inflight(id,user_id,version) VALUES('snapshot-send',701,'v1')`); err != nil {
		t.Fatal(err)
	}
	if _, err = stale.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`); err == nil || !strings.Contains(err.Error(), "read committed isolation") {
		t.Fatalf("RR policy update: %v", err)
	}
	if err = stale.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`); err == nil {
		t.Fatal("read committed update ignored marker")
	}
	if _, err = r.db.ExecContext(ctx, `DELETE FROM ai_consent_inflight WHERE id='snapshot-send'`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`); err != nil {
		t.Fatalf("read committed update: %v", err)
	}
}

func verifyAIConsentPostgres(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("set ARRAN_TEST_DATABASE_URL to an isolated PostgreSQL database")
	}
	r := openReconciliationRepository(t)
	ctx := identity.WithUser(context.Background(), 701)
	other := identity.WithUser(context.Background(), 702)
	gate := identity.AIConsentGate{Store: r, Version: "v1"}
	send := func(context.Context) error { return nil }
	if !errors.Is(gate.Grant(ctx, "v1"), identity.ErrConsentRequired) {
		t.Fatal("unset policy allowed grant")
	}
	if !errors.Is(r.GrantConsent(other, 701, "v1"), identity.ErrInvalidUser) {
		t.Fatal("other user was accepted")
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v1',description='test policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	status, err := gate.Status(ctx)
	if err != nil || status.Description != "test policy" || status.Version != "v1" || status.Granted {
		t.Fatalf("status before grant: %+v %v", status, err)
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET description='different text' WHERE singleton=TRUE`); err == nil {
		t.Fatal("policy description changed without version")
	}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	status, err = gate.Status(ctx)
	if err != nil || !status.Granted {
		t.Fatalf("status after grant: %+v %v", status, err)
	}
	// The policy writer waits for in-progress admission, then must observe its
	// committed marker rather than validating against its earlier snapshot.
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = lockConsentPolicy(ctx, tx, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO ai_consent_inflight(id,user_id,version) VALUES('policy-race',701,'v1')`); err != nil {
		t.Fatal(err)
	}
	conn, err := r.db.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var writerPID int
	if err = conn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID); err != nil {
		t.Fatal(err)
	}
	updated := make(chan error, 1)
	go func() {
		_, e := conn.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`)
		updated <- e
	}()
	until := time.Now().Add(3 * time.Second)
	for {
		var blocked bool
		if err = r.db.DB.QueryRowContext(ctx, `SELECT cardinality(pg_blocking_pids($1)) > 0`, writerPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(until) {
			t.Fatal("policy writer did not block")
		}
		time.Sleep(time.Millisecond)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-updated; err == nil {
		t.Fatal("policy writer missed concurrently admitted send")
	}
	if _, err = r.db.DB.Exec(`DELETE FROM ai_consent_inflight WHERE id='policy-race'`); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(other, send), identity.ErrConsentRequired) {
		t.Fatal("grant leaked to another user")
	}
	if err := gate.Execute(ctx, send); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	finish := make(chan struct{})
	sent := make(chan error, 1)
	go func() {
		sent <- gate.Execute(ctx, func(context.Context) error { close(started); <-finish; return nil })
	}()
	<-started
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`); err == nil {
		t.Fatal("policy changed during admitted send")
	}
	withdrawn := make(chan error, 1)
	withdrawCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	go func() { withdrawn <- gate.Withdraw(withdrawCtx) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var pending bool
		if err := r.db.DB.QueryRow(`SELECT withdrawal_pending FROM ai_user_consents WHERE user_id=701`).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("withdrawal never entered pending state")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-withdrawn:
		t.Fatalf("withdrawal finished during send: %v", err)
	default:
	}
	secondWithdrawn := make(chan error, 1)
	go func() { secondWithdrawn <- gate.Withdraw(withdrawCtx) }()
	if !errors.Is(gate.Grant(ctx, "v1"), identity.ErrConsentRequired) {
		t.Fatal("grant accepted during withdrawal")
	}
	if !errors.Is(gate.Execute(ctx, send), identity.ErrConsentRequired) {
		t.Fatal("send accepted during withdrawal")
	}
	close(finish)
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if err := <-withdrawn; err != nil {
		t.Fatal(err)
	}
	if err := <-secondWithdrawn; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(ctx, send), identity.ErrConsentRequired) {
		t.Fatal("withdrawn grant accepted")
	}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v1',description='altered policy' WHERE singleton=TRUE`); err == nil {
		t.Fatal("old version reused with different description")
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v1',description='test policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(ctx, send), identity.ErrConsentRequired) {
		t.Fatal("old grant revived after policy rollback")
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v2' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Grant(ctx, "v1"), identity.ErrConsentRequired) {
		t.Fatal("old server granted stale version")
	}
	next := identity.AIConsentGate{Store: r, Version: "v2"}
	if err := next.Grant(ctx, "v2"); err != nil {
		t.Fatal(err)
	}
	uncertain := errors.New("remote result unknown")
	if !errors.Is(next.Execute(ctx, func(context.Context) error { return uncertain }), uncertain) {
		t.Fatal("callback error lost")
	}
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	if !errors.Is(next.Withdraw(short), context.DeadlineExceeded) {
		t.Fatal("uncertain send did not block withdrawal")
	}
	if !errors.Is(next.Grant(ctx, "v2"), identity.ErrConsentRequired) {
		t.Fatal("pending withdrawal did not block grant")
	}
	if !errors.Is(next.Execute(ctx, send), identity.ErrConsentRequired) {
		t.Fatal("pending withdrawal did not block send")
	}
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v3' WHERE singleton=TRUE`); err == nil {
		t.Fatal("uncertain send did not block policy change")
	}
	// The policy row cannot be removed to bypass version-change safeguards.
	if _, err := r.db.DB.Exec(`DELETE FROM ai_consent_policy`); err == nil {
		t.Fatal("policy deletion bypassed safeguards")
	}
	// An unavailable database must never invoke a callback.
	called := false
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := next.Execute(ctx, func(context.Context) error { called = true; return nil }); err == nil || called {
		t.Fatal("closed DB allowed send")
	}
}

func verifyAIConsentDatabaseFailure(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	var schema string
	if err := r.db.DB.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	q := address.Query()
	q.Set("search_path", schema)
	address.RawQuery = q.Encode()
	db, err := sql.Open("pgx", address.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	other := &Repository{db: &database{DB: db}}
	if _, err = db.Exec(`UPDATE ai_consent_policy SET version='v1',description='test policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	ctx := identity.WithUser(context.Background(), 701)
	gate := identity.AIConsentGate{Store: r, Version: "v1"}
	if err = gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	// After admission, lose the connection pool before completion can be recorded.
	err = gate.Execute(ctx, func(context.Context) error { return r.Close() })
	if err == nil {
		t.Fatal("failed completion persistence reported success")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM ai_consent_inflight WHERE user_id=701`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("inflight marker lost: %d %v", count, err)
	}
	retry := identity.AIConsentGate{Store: other, Version: "v1"}
	short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if !errors.Is(retry.Withdraw(short), context.DeadlineExceeded) {
		t.Fatal("withdrawal ignored unknown completion")
	}
	if !errors.Is(retry.Grant(ctx, "v1"), identity.ErrConsentRequired) {
		t.Fatal("grant accepted despite pending withdrawal")
	}
}

func TestAIConsentDatabaseFailure(t *testing.T) { verifyAIConsentDatabaseFailure(t) }

// Two already-recorded withdrawals share a completion point. A subsequent
// explicit grant may authorize a new send even if an older caller has yet to
// observe that its withdrawal was completed.
func verifyAIConsentOverlappingWithdrawal(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := identity.WithUser(context.Background(), 701)
	if _, err := r.db.DB.Exec(`UPDATE ai_consent_policy SET version='v1',description='test policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	gate := identity.AIConsentGate{Store: r, Version: "v1"}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	// Both calls committed their withdrawal requests before either finishes.
	if err := r.beginConsentWithdrawal(ctx, 701); err != nil {
		t.Fatal(err)
	}
	if err := r.beginConsentWithdrawal(ctx, 701); err != nil {
		t.Fatal(err)
	}
	if done, err := r.finishConsentWithdrawal(ctx, 701); err != nil || !done {
		t.Fatalf("first completion: %v %v", done, err)
	}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finish := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- gate.Execute(ctx, func(context.Context) error { close(started); <-finish; return nil })
	}()
	<-started
	if done, err := r.finishConsentWithdrawal(ctx, 701); err != nil || !done {
		t.Fatalf("coalesced completion: %v %v", done, err)
	}
	close(finish)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := gate.Execute(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal("older withdrawal erased renewed grant", err)
	}
	// A new withdrawal may begin after the first completion and before the
	// older caller observes it. With no generation number, the older caller
	// safely waits for that later withdrawal too.
	if err := r.beginConsentWithdrawal(ctx, 701); err != nil {
		t.Fatal(err)
	}
	if err := r.beginConsentWithdrawal(ctx, 701); err != nil {
		t.Fatal(err)
	}
	if done, err := r.finishConsentWithdrawal(ctx, 701); err != nil || !done {
		t.Fatalf("first withdrawal: %v %v", done, err)
	}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	unknown := errors.New("remote outcome unknown")
	if err := gate.Execute(ctx, func(context.Context) error { return unknown }); !errors.Is(err, unknown) {
		t.Fatal(err)
	}
	if err := r.beginConsentWithdrawal(ctx, 701); err != nil {
		t.Fatal(err)
	}
	if done, err := r.finishConsentWithdrawal(ctx, 701); err != nil || done {
		t.Fatalf("older withdrawal did not wait for later one: %v %v", done, err)
	}
	// Test-only proof of external completion; no operational recovery exists.
	if _, err := r.db.DB.Exec(`DELETE FROM ai_consent_inflight WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	if done, err := r.finishConsentWithdrawal(ctx, 701); err != nil || !done {
		t.Fatalf("later withdrawal: %v %v", done, err)
	}
	if done, err := r.finishConsentWithdrawal(ctx, 701); err != nil || !done {
		t.Fatalf("older withdrawal: %v %v", done, err)
	}
}
func TestAIConsentOverlappingWithdrawal(t *testing.T) { verifyAIConsentOverlappingWithdrawal(t) }

func verifyAIConsentMigration(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openMigrationFixture(t, 15)
	ctx := context.Background()
	if _, err := r.db.DB.Exec(`INSERT INTO users(id,username,password_hash) VALUES(7001,'pre-migration','test');
	INSERT INTO csv_import_previews(user_id,id,payload,expires_at) VALUES(7001,'preexisting','{"sample":true}',12345);
	CREATE FUNCTION ai_consent_change_policy() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$;`); err != nil {
		t.Fatal(err)
	}
	var pgErr *pgconn.PgError
	if err := Migrate(ctx, r.db.DB); !errors.As(err, &pgErr) || pgErr.Code != "42723" {
		t.Fatalf("expected duplicate function, got %v", err)
	}
	var v16, tables int
	if err := r.db.DB.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=16`).Scan(&v16); err != nil || v16 != 0 {
		t.Fatalf("failed migration changed ledger: %d %v", v16, err)
	}
	if err := r.db.DB.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('ai_consent_policy_versions','ai_consent_policy','ai_user_consents','ai_consent_inflight')`).Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("partial DDL remains: %d %v", tables, err)
	}
	if _, err := r.db.DB.Exec(`DROP FUNCTION ai_consent_change_policy()`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, r.db.DB); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, r.db.DB); err != nil {
		t.Fatal(err)
	}
	var payload string
	var expiry int64
	if err := r.db.DB.QueryRow(`SELECT payload,expires_at FROM csv_import_previews WHERE user_id=7001 AND id='preexisting'`).Scan(&payload, &expiry); err != nil || payload != `{"sample":true}` || expiry != 12345 {
		t.Fatalf("preexisting preview changed: %q %d %v", payload, expiry, err)
	}
	var active sql.NullString
	if err := r.db.DB.QueryRow(`SELECT version FROM ai_consent_policy WHERE singleton=TRUE`).Scan(&active); err != nil || active.Valid {
		t.Fatalf("policy should default to unset: %v %v", active, err)
	}
}
func TestAIConsentMigration(t *testing.T) { verifyAIConsentMigration(t) }
