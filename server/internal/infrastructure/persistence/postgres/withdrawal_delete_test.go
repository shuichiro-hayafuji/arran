package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWithdrawalDeleteRows(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("ARRAN_TEST_DATABASE_URL is not set")
	}
	verifyWithdrawalDeleteRows(t)
}

func verifyWithdrawalDeleteRows(t *testing.T) {
	t.Helper()
	r := openReconciliationRepository(t)
	ctx := context.Background()
	if err := r.RequestWithdrawal(ctx, 701); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`UPDATE withdrawal_requests SET reviewed_at=CURRENT_TIMESTAMP WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	var identity withdrawalIdentity
	if err := r.db.QueryRow(`SELECT u.id,u.username,u.created_at,w.requested_at FROM users u JOIN withdrawal_requests w ON w.user_id=u.id WHERE u.id=701`).Scan(&identity.userID, &identity.username, &identity.createdAt, &identity.requestedAt); err != nil {
		t.Fatal(err)
	}
	userHash := sha256.Sum256([]byte("user:" + identity.username))
	userKey := hex.EncodeToString(userHash[:])
	if _, err := r.db.Exec(`INSERT INTO auth_attempts(key,attempts,window_end) VALUES($1,1,CURRENT_TIMESTAMP),('shared-ip',1,CURRENT_TIMESTAMP)`, userKey); err != nil {
		t.Fatal(err)
	}
	for _, user := range []any{int64(701), int64(702), nil} {
		if _, err := r.db.Exec(`INSERT INTO memories(user_id,type,content,evidence,confidence,created_at,updated_at) VALUES($1,'test','private','test',1,'','')`, user); err != nil {
			t.Fatal(err)
		}
	}
	run := func(id withdrawalIdentity, commit bool) error {
		tx, err := r.db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err = deleteWithdrawalRows(ctx, tx, id); err != nil {
			return err
		}
		if commit {
			return tx.Commit()
		}
		return nil
	}
	// The legitimate v17 trigger on users is an exact, enabled BEFORE
	// INSERT/UPDATE OF is_active trigger. A disabled trigger or a replacement
	// function with different privileges must not pass the schema contract.
	if _, err := r.db.Exec(`ALTER TABLE users DISABLE TRIGGER users_enforce_free_user_limit`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("disabled users trigger accepted: %v", err)
	}
	if _, err := r.db.Exec(`ALTER TABLE users ENABLE TRIGGER users_enforce_free_user_limit; ALTER FUNCTION enforce_free_user_limit() SECURITY DEFINER`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("altered users trigger function accepted: %v", err)
	}
	if _, err := r.db.Exec(`ALTER FUNCTION enforce_free_user_limit() SECURITY INVOKER`); err != nil {
		t.Fatal(err)
	}
	// RLS can hide a user from a restore snapshot even where table names and
	// constraints match. Reject the flags regardless of the test role's bypass.
	if _, err := r.db.Exec(`ALTER TABLE users ENABLE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("row security accepted: %v", err)
	}
	if _, err := r.db.Exec(`ALTER TABLE users DISABLE ROW LEVEL SECURITY; ALTER TABLE users FORCE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("forced row security flag accepted: %v", err)
	}
	if _, err := r.db.Exec(`ALTER TABLE users NO FORCE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	// A table added outside the known schema must not silently escape deletion.
	if _, err := r.db.Exec(`CREATE TABLE unexpected_owner_data(user_id BIGINT)`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("unknown owner table: %v", err)
	}
	if _, err := r.db.Exec(`DROP TABLE unexpected_owner_data`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`CREATE TABLE unexpected_named_owner(owner_id BIGINT)`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("unknown non-user_id owner table: %v", err)
	}
	if _, err := r.db.Exec(`DROP TABLE unexpected_named_owner`); err != nil {
		t.Fatal(err)
	}
	// DELETE against a regular parent includes inherited children, even when
	// a child lives in a different schema and has no FK to a known table.
	inheritSchema := fmt.Sprintf("withdrawal_inherit_%d", time.Now().UnixNano())
	quotedInheritSchema := pgx.Identifier{inheritSchema}.Sanitize()
	if _, err := r.db.Exec(`CREATE SCHEMA ` + quotedInheritSchema + `; CREATE TABLE ` + quotedInheritSchema + `.child () INHERITS (memories)`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := r.db.Exec(`DROP SCHEMA IF EXISTS ` + quotedInheritSchema + ` CASCADE`); err != nil {
			t.Errorf("cleanup inherited schema: %v", err)
		}
	}()
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("cross-schema inherited child accepted: %v", err)
	}
	if _, err := r.db.Exec(`DROP SCHEMA ` + quotedInheritSchema + ` CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`CREATE RULE withdrawal_test_rule AS ON DELETE TO memories DO INSTEAD NOTHING`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("user-defined delete rule accepted: %v", err)
	}
	if _, err := r.db.Exec(`DROP RULE withdrawal_test_rule ON memories`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`CREATE TABLE unexpected_user_fk(owner_ref BIGINT REFERENCES users(id) ON DELETE CASCADE)`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("unknown foreign key: %v", err)
	}
	if _, err := r.db.Exec(`DROP TABLE unexpected_user_fk`); err != nil {
		t.Fatal(err)
	}
	// A new CASCADE between two known tables is still unknown and could
	// delete another owner's rows. OID/column/action validation must reject it.
	if _, err := r.db.Exec(`ALTER TABLE memories ADD CONSTRAINT unexpected_cascade FOREIGN KEY(id) REFERENCES users(id) ON DELETE CASCADE NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("unexpected cascade: %v", err)
	}
	if _, err := r.db.Exec(`ALTER TABLE memories DROP CONSTRAINT unexpected_cascade`); err != nil {
		t.Fatal(err)
	}
	// search_path may resolve one name to a shadow relation. No writes may be
	// attempted when the resolved table OIDs are from mixed schemas.
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `CREATE TEMP TABLE users(id BIGINT) ON COMMIT DROP`); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err = deleteWithdrawalRows(ctx, tx, identity); !errors.Is(err, errWithdrawalUnsafe) {
		_ = tx.Rollback()
		t.Fatalf("shadow users: %v", err)
	}
	_ = tx.Rollback()
	// A NOT VALID constraint has the same definition as the trusted FK but
	// can retain pre-existing inconsistent rows belonging to another user.
	// The legacy single-column CASCADE would delete that other user's row.
	var dropComposite string
	if err := r.db.QueryRow(`SELECT format('ALTER TABLE consultation_messages DROP CONSTRAINT %I',conname) FROM pg_constraint WHERE conrelid='consultation_messages'::regclass AND confrelid='consultations'::regclass AND contype='f' AND cardinality(conkey)=2`).Scan(&dropComposite); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(dropComposite); err != nil {
		t.Fatal(err)
	}
	var inconsistentConsultation, inconsistentMessage int64
	if err := r.db.QueryRow(`INSERT INTO consultations(user_id,created_at,user_message,inferred_category,ai_recommendation,ai_reasoning_summary,current_situation,alternative,final_question,budget_snapshot_json,status,response_source) VALUES(701,'','','','','','','','','{}','','') RETURNING id`).Scan(&inconsistentConsultation); err != nil {
		t.Fatal(err)
	}
	if err := r.db.QueryRow(`INSERT INTO consultation_messages(consultation_id,user_id,role,content,created_at) VALUES($1,702,'user','other','') RETURNING id`, inconsistentConsultation).Scan(&inconsistentMessage); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`ALTER TABLE consultation_messages ADD CONSTRAINT fixture_composite FOREIGN KEY(consultation_id,user_id) REFERENCES consultations(id,user_id) ON DELETE CASCADE NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("unvalidated foreign key: %v", err)
	}
	if _, err := r.db.Exec(`DELETE FROM consultation_messages WHERE id=$1`, inconsistentMessage); err != nil {
		t.Fatal(err)
	}
	// Even after removing the inconsistent row, the constraint itself is
	// unvalidated and must independently block deletion until validated.
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("empty but unvalidated foreign key: %v", err)
	}
	if _, err := r.db.Exec(`ALTER TABLE consultation_messages VALIDATE CONSTRAINT fixture_composite`); err != nil {
		t.Fatal(err)
	}
	wrong := identity
	wrong.username = "different"
	if err := run(wrong, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("identity mismatch: %v", err)
	}
	if _, err := r.db.Exec(`INSERT INTO ai_consent_inflight(id,user_id,version) VALUES('unknown',701,'v1')`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("AI inflight: %v", err)
	}
	if _, err := r.db.Exec(`DELETE FROM ai_consent_inflight WHERE id='unknown'`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO admin_notifications(user_id,month,notification_type,consultation_count,consultation_limit,status,attempts,occurred_at) VALUES(701,'2026-11','limit',1,1,'sending',1,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("sending notification: %v", err)
	}
	if _, err := r.db.Exec(`UPDATE admin_notifications SET status='failed' WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO chat_requests(user_id,request_id,payload_hash,month,new_consultation,quota_reserved,status,expires_at) VALUES(701,'pending','hash','2026-11',true,false,'processing',0)`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("processing request: %v", err)
	}
	if _, err := r.db.Exec(`UPDATE chat_requests SET status='completed' WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	// Legacy messages can have no owner while still referencing an owned
	// consultation through the old single-column cascading FK. Refuse the
	// entire delete rather than erase data with an unknown owner.
	var consultationID int64
	if err := r.db.QueryRow(`INSERT INTO consultations(user_id,created_at,user_message,inferred_category,ai_recommendation,ai_reasoning_summary,current_situation,alternative,final_question,budget_snapshot_json,status,response_source) VALUES(701,'','','','','','','','','{}','','') RETURNING id`).Scan(&consultationID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO consultation_messages(consultation_id,user_id,role,content,created_at) VALUES($1,NULL,'user','legacy','')`, consultationID); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("unowned message: %v", err)
	}
	if _, err := r.db.Exec(`DELETE FROM consultation_messages WHERE consultation_id=$1 AND user_id IS NULL`, consultationID); err != nil {
		t.Fatal(err)
	}
	// A user-defined trigger can alter other rows during deletion. Refuse the
	// entire transaction before any delete, retaining evidence and identities.
	var transactionID int64
	if err := r.db.QueryRow(`INSERT INTO transactions(user_id,transaction_date,description,normalized_merchant,amount,transaction_type,category,source,source_account_name,fingerprint,imported_at,raw_data_json) VALUES(701,'2026-11-01','','',1,'expense','','','','fixture','','{}') RETURNING id`).Scan(&transactionID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO csv_import_batches(id,user_id,commit_key,file_key,source_scope,account_scope,metadata,status,recorded_at,result) VALUES('batch',701,'commit','','','','{}','active','','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO csv_import_evidence(user_id,batch_id,row_number,transaction_type,transaction_id,observation) VALUES(701,'batch',1,'expense',$1,'{}')`, transactionID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`INSERT INTO auth_sessions(token_hash,user_id,password_hash,expires_at) VALUES('other-session',702,'disabled-test-only',CURRENT_TIMESTAMP+INTERVAL '1 day')`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`CREATE FUNCTION withdrawal_test_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER withdrawal_test_fail BEFORE DELETE ON transactions FOR EACH STATEMENT EXECUTE FUNCTION withdrawal_test_fail()`); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("user-defined delete trigger accepted: %v", err)
	}
	var evidenceCount int
	if err := r.db.QueryRow(`SELECT count(*) FROM csv_import_evidence WHERE user_id=701`).Scan(&evidenceCount); err != nil || evidenceCount != 1 {
		t.Fatalf("rollback lost CSV evidence: %d %v", evidenceCount, err)
	}
	if _, err := r.db.Exec(`DROP TRIGGER withdrawal_test_fail ON transactions; DROP FUNCTION withdrawal_test_fail()`); err != nil {
		t.Fatal(err)
	}
	// PostgreSQL permissions can force failure at a later DELETE after the
	// schema guard passes. Unlike an injected trigger this is a genuine
	// mid-deletion SQL failure and exercises transaction rollback.
	t.Run("permission failure rolls back prior child deletes", func(t *testing.T) {
		role := fmt.Sprintf("arran_withdrawal_%d", time.Now().UnixNano())
		quotedRole := pgx.Identifier{role}.Sanitize()
		if _, err := r.db.Exec(`CREATE ROLE ` + quotedRole + ` NOLOGIN`); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "42501" {
				t.Skip("isolated test DB user requires CREATEROLE for permission-failure rollback")
			}
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := r.db.Exec(`DROP OWNED BY ` + quotedRole + `; DROP ROLE ` + quotedRole); err != nil {
				t.Errorf("cleanup test role: %v", err)
			}
		})
		var schema string
		if err := r.db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
			t.Fatal(err)
		}
		quotedSchema := pgx.Identifier{schema}.Sanitize()
		if _, err := r.db.Exec(`GRANT USAGE ON SCHEMA ` + quotedSchema + ` TO ` + quotedRole + `; GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA ` + quotedSchema + ` TO ` + quotedRole + `; REVOKE DELETE ON transactions FROM ` + quotedRole); err != nil {
			t.Fatal(err)
		}
		tx, err := r.db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE `+quotedRole); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		err = deleteWithdrawalRows(ctx, tx, identity)
		_ = tx.Rollback()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" || !strings.Contains(err.Error(), "delete transactions") {
			t.Fatalf("expected late transactions permission failure, got: %v", err)
		}
		for _, query := range []string{
			`SELECT count(*) FROM csv_import_evidence WHERE user_id=701`,
			`SELECT count(*) FROM withdrawal_requests WHERE user_id=701`,
			`SELECT count(*) FROM auth_sessions WHERE user_id=702 AND token_hash='other-session'`,
		} {
			var count int
			if err := r.db.QueryRow(query).Scan(&count); err != nil || count != 1 {
				t.Fatalf("rollback lost data for %q: %d %v", query, count, err)
			}
		}
	})
	// A backup may predate the original application. A completed entry from
	// the independently authenticated ledger identifies the exact user and
	// permits reconstructing the request within the deletion transaction.
	if _, err := r.db.Exec(`DELETE FROM withdrawal_requests WHERE user_id=701`); err != nil {
		t.Fatal(err)
	}
	prepared := withdrawalLedgerEntry{sequence: 1, state: "prepared", identity: identity}
	completed := withdrawalLedgerEntry{sequence: 2, previous: ledgerEntryDigest(prepared), state: "completed", identity: identity}
	head := withdrawalLedgerCheckpoint{sequence: 2, digest: ledgerEntryDigest(completed)}
	plan, err := planWithdrawalReplay([]withdrawalLedgerEntry{prepared, completed}, &head, []withdrawalRestoreUser{{userID: identity.userID, username: identity.username, createdAt: identity.createdAt}})
	if err != nil {
		t.Fatal(err)
	}
	replayTx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &withdrawalSQLRestoreTx{tx: replayTx}
	candidate, err := adapter.candidateFingerprint(ctx)
	if err != nil {
		_ = replayTx.Rollback()
		t.Fatal(err)
	}
	// Retain the same DB name while changing one fingerprint element at a
	// time. A different real schema OID exercises same-database ambiguity.
	parts := strings.SplitN(candidate, "/", 4)
	if len(parts) != 4 {
		_ = replayTx.Rollback()
		t.Fatalf("malformed candidate fingerprint")
	}
	otherSchema := fmt.Sprintf("withdrawal_fingerprint_%d", time.Now().UnixNano())
	quotedOtherSchema := pgx.Identifier{otherSchema}.Sanitize()
	if _, err := replayTx.ExecContext(ctx, `CREATE SCHEMA `+quotedOtherSchema); err != nil {
		_ = replayTx.Rollback()
		t.Fatal(err)
	}
	var otherSchemaOID string
	if err := replayTx.QueryRowContext(ctx, `SELECT oid::text FROM pg_namespace WHERE nspname=$1`, otherSchema).Scan(&otherSchemaOID); err != nil {
		_ = replayTx.Rollback()
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		index int
		value string
	}{
		{"different cluster", 0, parts[0] + "0"},
		{"different database oid", 1, parts[1] + "0"},
		{"different real schema", 2, otherSchemaOID},
		{"different database name", 3, parts[3] + "_wrong"},
	} {
		altered := append([]string(nil), parts...)
		altered[tc.index] = tc.value
		if err := adapter.validate(ctx, strings.Join(altered, "/")); !errors.Is(err, errWithdrawalUnsafe) {
			_ = replayTx.Rollback()
			t.Fatalf("%s candidate accepted: %v", tc.name, err)
		}
	}
	if err := adapter.validate(ctx, candidate); err != nil {
		_ = replayTx.Rollback()
		t.Fatalf("candidate validation: %v", err)
	}
	if users, err := adapter.users(ctx); err != nil || len(users) < 2 {
		_ = replayTx.Rollback()
		t.Fatalf("candidate users: %d %v", len(users), err)
	}
	if err := replayWithdrawalRows(ctx, replayTx, plan, identity); err != nil {
		_ = replayTx.Rollback()
		t.Fatalf("replay pre-request backup: %v", err)
	}
	if err := replayTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var requestCount int
	if err := r.db.QueryRow(`SELECT count(*) FROM withdrawal_requests WHERE user_id=701`).Scan(&requestCount); err != nil || requestCount != 0 {
		t.Fatalf("reconstruction escaped rollback: %d %v", requestCount, err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) {
		t.Fatalf("ordinary delete without request: %v", err)
	}
	for _, tc := range []struct {
		name      string
		requested any
		reviewed  any
		wantError bool
	}{
		{"matching unreviewed request", identity.requestedAt, nil, false},
		{"different request time", identity.requestedAt.Add(time.Second), nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.db.Exec(`INSERT INTO withdrawal_requests(user_id,requested_at,reviewed_at) VALUES($1,$2,$3)`, identity.userID, tc.requested, tc.reviewed); err != nil {
				t.Fatal(err)
			}
			tx, err := r.db.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = replayWithdrawalRows(ctx, tx, plan, identity)
			_ = tx.Rollback()
			if tc.wantError != errors.Is(err, errWithdrawalUnsafe) || (!tc.wantError && err != nil) {
				t.Fatalf("unexpected replay result: %v", err)
			}
			if _, err := r.db.Exec(`DELETE FROM withdrawal_requests WHERE user_id=701`); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := r.db.Exec(`INSERT INTO withdrawal_requests(user_id,requested_at,reviewed_at) VALUES($1,$2,CURRENT_TIMESTAMP)`, identity.userID, identity.requestedAt); err != nil {
		t.Fatal(err)
	}
	if err := run(identity, false); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=701`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rollback lost user: %d %v", n, err)
	}
	if err := run(identity, true); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=701`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("user remains: %d %v", n, err)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM memories WHERE user_id=702 OR user_id IS NULL`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("other/unowned modified: %d %v", n, err)
	}
	if err := r.db.QueryRow(`SELECT count(*) FROM auth_sessions WHERE token_hash='other-session' AND user_id=702`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("other session changed: %d %v", n, err)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM auth_attempts WHERE key=$1`, userKey).Scan(&n); err != nil || n != 0 {
		t.Fatalf("username limiter remains: %d %v", n, err)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM auth_attempts WHERE key='shared-ip'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("shared IP limiter changed: %d %v", n, err)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM withdrawal_requests WHERE user_id=701`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("request remains: %d %v", n, err)
	}
	if err := run(identity, false); !errors.Is(err, errWithdrawalUnsafe) && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("absent identity accepted: %v", err)
	}
}
