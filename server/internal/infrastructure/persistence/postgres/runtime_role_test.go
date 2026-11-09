package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
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
	exec("GRANT SELECT ON " + schema + ".schema_migrations TO " + runtimeRole)
	exec("GRANT SELECT ON " + schema + ".service_settings TO " + runtimeRole)
	exec("GRANT INSERT, SELECT ON " + schema + ".users TO " + runtimeRole)
	r, err := Open(ctx, runtimeRoleURL, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.db.DB.ExecContext(ctx, "INSERT INTO users(id,username,password_hash) VALUES(703,'role-check','synthetic')"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, "CREATE TABLE denied(id int)"); err == nil {
		t.Fatal("runtime role can create tables")
	}
	if _, err := r.db.DB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version=3"); err == nil {
		t.Fatal("runtime role can modify migration history")
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
