package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// Use an isolated PostgreSQL test cluster: the bootstrap user must be able
// to create login roles. No grants are made on existing application schemas.
func TestRuntimeRoleChecksMigrationsWithoutDDL(t *testing.T) {
	raw := os.Getenv("ARRAN_ROLE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set ARRAN_ROLE_TEST_DATABASE_URL to a disposable PostgreSQL cluster")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal("invalid test DB connection settings")
	}
	defer admin.Close()
	if err := admin.PingContext(ctx); err != nil {
		t.Fatal("cannot connect to the role test DB")
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(nonce[:])
	schema := "arran_" + suffix
	migrationRole := "migrate_" + suffix
	runtimeRole := "runtime_" + suffix
	exec := func(q string) {
		t.Helper()
		if _, err := admin.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	cleanup := func(q string) {
		cleanupCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.ExecContext(cleanupCtx, q); err != nil {
			t.Error("could not remove disposable PostgreSQL role or schema")
		}
	}
	var migrationRoleURL, runtimeRoleURL string
	for _, role := range []string{migrationRole, runtimeRole} {
		var pw [24]byte
		if _, err := rand.Read(pw[:]); err != nil {
			t.Fatal(err)
		}
		password := hex.EncodeToString(pw[:])
		exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s'", role, password))
		defer cleanup("DROP ROLE " + role)
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal("invalid test URL")
		}
		u.User = url.UserPassword(role, password)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		if role == migrationRole {
			migrationRoleURL = u.String()
		} else {
			runtimeRoleURL = u.String()
		}
	}
	exec("CREATE SCHEMA " + schema + " AUTHORIZATION " + migrationRole)
	defer cleanup("DROP SCHEMA " + schema + " CASCADE")
	exec("GRANT USAGE ON SCHEMA " + schema + " TO " + runtimeRole)
	if r, err := Open(ctx, runtimeRoleURL, false); err == nil {
		r.Close()
		t.Fatal("started without a migration ledger")
	}
	if err := MigrateURL(ctx, migrationRoleURL); err != nil {
		t.Fatal(err)
	}
	// Apply the actual grant script to a version-16 restore, then after migration.
	grantSQL, err := os.ReadFile("../../../../../infra/operations/grant-runtime.sql")
	if err != nil {
		t.Fatal(err)
	}
	grant := strings.ReplaceAll(string(grantSQL), "SCHEMA public", "SCHEMA "+schema)
	grant = strings.ReplaceAll(grant, "public.", schema+".")
	grant = strings.ReplaceAll(grant, "arran_runtime", runtimeRole)
	exec("DROP TABLE " + schema + ".evidence_intents")
	exec("DROP TRIGGER users_evidence_owner_id_immutable ON " + schema + ".users")
	exec("ALTER TABLE " + schema + ".users DROP COLUMN evidence_owner_id")
	exec("DROP FUNCTION " + schema + ".prevent_evidence_owner_id_change()")
	exec("DROP TABLE " + schema + ".ai_usage_operations")
	exec("ALTER TABLE " + schema + ".llm_usage DROP CONSTRAINT llm_usage_owner_id_unique")
	exec("DROP TABLE " + schema + ".sync_consultation_reservations")
	exec("DROP TABLE " + schema + ".withdrawal_requests")
	exec("DROP TABLE " + schema + ".withdrawal_batch_health")
	exec("ALTER TABLE " + schema + ".admin_notifications DROP COLUMN withdrawal_uncertain")
	exec("DELETE FROM " + schema + ".schema_migrations WHERE version > 16")
	exec(grant)
	if err := MigrateURL(ctx, migrationRoleURL); err != nil {
		t.Fatal(err)
	}
	exec(grant)
	// A failed grant must roll back the earlier all-table GRANT; otherwise a
	// failed deployment could silently leave broad withdrawal INSERT access.
	conn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	end := strings.Index(grant, "GRANT USAGE, SELECT ON ALL SEQUENCES")
	if end < 0 {
		conn.Close()
		t.Fatal("grant script no longer has expected failure point")
	}
	// Execute one statement at a time like psql. A single multi-statement
	// Exec can create an implicit transaction and mask a missing BEGIN.
	for _, statement := range strings.Split(grant[:end], ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			conn.Close()
			t.Fatalf("grant prefix failed: %v", err)
		}
	}
	var grantedInside bool
	if err := conn.QueryRowContext(ctx, "SELECT has_table_privilege($1,$2,'INSERT')", runtimeRole, schema+".withdrawal_requests").Scan(&grantedInside); err != nil || !grantedInside {
		conn.Close()
		t.Fatalf("broad grant was not exercised: %v %v", grantedInside, err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT 1/0"); err == nil {
		conn.Close()
		t.Fatal("injected grant failure succeeded")
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	var broadInsert bool
	if err := admin.QueryRowContext(ctx, "SELECT has_table_privilege($1,$2,'INSERT')", runtimeRole, schema+".withdrawal_requests").Scan(&broadInsert); err != nil || broadInsert {
		t.Fatalf("failed grant left table INSERT: %v %v", broadInsert, err)
	}
	r, err := Open(ctx, runtimeRoleURL, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	exec("BEGIN; SET LOCAL search_path TO " + schema + "; INSERT INTO users(id,username,password_hash) VALUES(703,'role-check','synthetic'); COMMIT")
	if _, err := r.db.DB.ExecContext(ctx, "INSERT INTO users(username,password_hash,evidence_owner_id) VALUES('forged','x','0123456789abcdef0123456789abcdef')"); err == nil {
		t.Fatal("runtime inserted an arbitrary evidence identity")
	}
	if _, err := r.db.DB.ExecContext(ctx, "DELETE FROM users WHERE id=703"); err == nil {
		t.Fatal("runtime deleted an account")
	}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	// Each required privilege is independently necessary: PostgreSQL treats a
	// comma-separated privilege list as an OR, not as an AND.
	for _, p := range []struct{ privilege, table string }{
		{"SELECT", "users"}, {"UPDATE", "users"},
		{"SELECT", "auth_sessions"}, {"DELETE", "auth_sessions"}, {"UPDATE", "auth_sessions"},
		{"SELECT", "withdrawal_requests"}, {"UPDATE (completed_at)", "withdrawal_requests"},
		{"SELECT", "chat_requests"}, {"SELECT", "ai_consent_inflight"}, {"SELECT", "admin_notifications"}, {"SELECT", "sync_consultation_reservations"}, {"SELECT", "ai_usage_operations"}, {"SELECT", "evidence_intents"},
	} {
		exec("REVOKE " + p.privilege + " ON " + schema + "." + p.table + " FROM " + runtimeRole)
		tx, err := r.db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = withdrawalBatchReady(ctx, tx)
		_ = tx.Rollback()
		if err == nil {
			t.Fatalf("preflight accepted missing %s on %s", p.privilege, p.table)
		}
		if (p.table == "auth_sessions" && p.privilege == "UPDATE") || (p.table == "admin_notifications" && p.privilege == "SELECT") {
			if err := r.RequestWithdrawal(identity.WithUser(ctx, 703), 703); err == nil {
				t.Fatal("admitted a request without the row-lock privilege")
			}
			var count int
			if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM withdrawal_requests WHERE user_id=703`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("request persisted without row-lock privilege: %d %v", count, err)
			}
		}
		exec("GRANT " + p.privilege + " ON " + schema + "." + p.table + " TO " + runtimeRole)
	}
	if err := r.RequestWithdrawal(identity.WithUser(ctx, 703), 703); err != nil {
		t.Fatalf("runtime cannot record normal request: %v", err)
	}
	if err := r.RequestWithdrawal(identity.WithUser(ctx, 703), 703); err != nil {
		t.Fatalf("runtime cannot retry request: %v", err)
	}
	if _, err := r.db.DB.ExecContext(ctx, "INSERT INTO withdrawal_requests(user_id,requested_at) VALUES(703,now()) ON CONFLICT DO NOTHING"); err == nil {
		t.Fatal("runtime may supply requested_at")
	}
	if _, err := r.db.DB.ExecContext(ctx, "INSERT INTO withdrawal_requests(user_id,reviewed_at) VALUES(703,now()) ON CONFLICT DO NOTHING"); err == nil {
		t.Fatal("runtime may mark request reviewed during insertion")
	}
	if _, err := r.db.DB.ExecContext(ctx, "CREATE TABLE denied(id int)"); err == nil {
		t.Fatal("runtime role can create tables")
	}
	if _, err := r.db.DB.ExecContext(ctx, "DELETE FROM withdrawal_requests"); err == nil {
		t.Fatal("runtime may delete withdrawal requests")
	}
	if _, err := r.db.DB.ExecContext(ctx, "UPDATE withdrawal_requests SET requested_at=now()"); err == nil {
		t.Fatal("runtime may update withdrawal requests")
	}
	if _, err := r.db.DB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version=3"); err == nil {
		t.Fatal("runtime role can modify migration history")
	}
	if _, err := r.db.DB.ExecContext(ctx, "UPDATE ai_consent_policy SET version='bad', description='bad' WHERE singleton=TRUE"); err == nil {
		t.Fatal("runtime role can update AI policy")
	}
	if _, err := r.db.DB.ExecContext(ctx, "INSERT INTO ai_consent_policy_versions(version,description) VALUES ('bad','bad')"); err == nil {
		t.Fatal("runtime role can create AI policy history")
	}
	if _, err := r.db.DB.ExecContext(ctx, "DELETE FROM ai_consent_policy_lock"); err == nil {
		t.Fatal("runtime role can delete policy lock")
	}
	exec("BEGIN; SET LOCAL search_path TO " + schema + "; INSERT INTO users(id,username,password_hash) VALUES(704,'role-consent','synthetic'); COMMIT")
	owner := identity.WithUser(ctx, 704)
	gate := identity.AIConsentGate{Store: r, Version: "v1"}
	if err := gate.Grant(owner, "v1"); !errors.Is(err, identity.ErrConsentRequired) {
		t.Fatalf("unset policy: %v", err)
	}
	exec("BEGIN; SET LOCAL search_path TO " + schema + "; UPDATE ai_consent_policy SET version='v1', description='reviewed test description' WHERE singleton=TRUE; COMMIT")
	if err := gate.Grant(owner, "v1"); err != nil {
		t.Fatalf("runtime grant: %v", err)
	}
	called := false
	if err := gate.Execute(owner, func(context.Context) error { called = true; return nil }); err != nil || !called {
		t.Fatalf("runtime send: %v", err)
	}
	if err := gate.Withdraw(owner); err != nil {
		t.Fatalf("runtime withdraw: %v", err)
	}
	if err := gate.Execute(owner, func(context.Context) error { t.Error("withdrawn send started"); return nil }); !errors.Is(err, identity.ErrConsentRequired) {
		t.Fatalf("withdrawn: %v", err)
	}
	exec("UPDATE " + schema + ".withdrawal_requests SET requested_at='2026-11-24 14:59:59+00' WHERE user_id=703")
	if err := r.RunWithdrawalBatch(ctx, time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("runtime batch failed: %v", err)
	}
	var inactive, completed bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT NOT is_active, completed_at IS NOT NULL FROM users JOIN withdrawal_requests ON users.id=withdrawal_requests.user_id WHERE users.id=703`).Scan(&inactive, &completed); err != nil || !inactive || !completed {
		t.Fatalf("runtime batch state: %v %v %v", inactive, completed, err)
	}
	history := func() string {
		var ledger string
		if err := admin.QueryRowContext(ctx, "SELECT string_agg(version::text || ':' || applied_at::text, ',' ORDER BY version) FROM "+schema+".schema_migrations").Scan(&ledger); err != nil {
			t.Fatal(err)
		}
		return ledger
	}
	complete := history()
	if err := MigrateURL(ctx, migrationRoleURL); err != nil || complete != history() {
		t.Fatal("repeat migration changed completed schema history", err)
	}
	verifyRejected := func() {
		before := history()
		if err := MigrateURL(ctx, migrationRoleURL); err == nil || !strings.Contains(err.Error(), "schema history does not match") {
			t.Fatal("incompatible history was not rejected before applying migration SQL", err)
		}
		if before != history() {
			t.Fatal("migration modified incompatible history")
		}
	}
	exec("INSERT INTO " + schema + ".schema_migrations(version,applied_at) VALUES (999,now())")
	verifyRejected()
	if r, err := Open(ctx, runtimeRoleURL, false); err == nil {
		r.Close()
		t.Fatal("started with future schema")
	}
	exec("DELETE FROM " + schema + ".schema_migrations WHERE version=999")
	exec("DELETE FROM " + schema + ".schema_migrations WHERE version=3")
	verifyRejected()
	if r, err := Open(ctx, runtimeRoleURL, false); err == nil {
		r.Close()
		t.Fatal("started with a gap in migration history")
	}
}
