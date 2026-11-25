package postgres

import (
	"context"
	"os"
	"testing"
)

func TestEvidenceOwnerMigration(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	// Reconstruct the immediately preceding version with existing owners.
	for _, q := range []string{`DROP TABLE evidence_intents`, `DROP TRIGGER users_evidence_owner_id_immutable ON users`, `ALTER TABLE users DROP COLUMN evidence_owner_id`, `DROP FUNCTION prevent_evidence_owner_id_change()`, `DELETE FROM schema_migrations WHERE version IN (21,22,23)`} {
		if _, err := r.db.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	sqlBytes, err := migrationFiles.ReadFile("021_evidence_owner_id.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	sqlBytes, err = migrationFiles.ReadFile("022_evidence_intents.sql")
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	sqlBytes, err = migrationFiles.ReadFile("023_withdrawal_evidence_v2.sql")
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var a, b string
	if err = r.db.DB.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=701`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err = r.db.DB.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=702`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	if len(a) != 32 || len(b) != 32 || a == b {
		t.Fatalf("identity collision or malformed: %q %q", a, b)
	}
	unused := "11111111111111111111111111111111"
	if unused == a || unused == b {
		t.Fatal("fixture unexpectedly generated chosen unused identity")
	}
	if _, err = r.db.DB.ExecContext(ctx, `UPDATE users SET evidence_owner_id=$1 WHERE id=701`, unused); err == nil {
		t.Fatal("identifier changed")
	}
	var unchanged string
	if err = r.db.DB.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=701`).Scan(&unchanged); err != nil || unchanged != a {
		t.Fatalf("identity changed: %v", err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `INSERT INTO users(username,password_hash,evidence_owner_id) VALUES('duplicate-owner','x',$1)`, a); err == nil {
		t.Fatal("duplicate identity accepted")
	}
	if _, err = r.db.DB.ExecContext(ctx, `INSERT INTO users(username,password_hash) VALUES('new-evidence-owner','x')`); err != nil {
		t.Fatal(err)
	}
	var c string
	if err = r.db.DB.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE username='new-evidence-owner'`).Scan(&c); err != nil || len(c) != 32 || c == a || c == b {
		t.Fatalf("new identity invalid: %q %v", c, err)
	}
}
