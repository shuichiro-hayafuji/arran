package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"testing"
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
	// Delete earlier child rows, then inject an error at a later table. The
	// caller's rollback must restore every child and the identity.
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
	if err := run(identity, false); err == nil {
		t.Fatal("injected failure accepted")
	}
	var evidenceCount int
	if err := r.db.QueryRow(`SELECT count(*) FROM csv_import_evidence WHERE user_id=701`).Scan(&evidenceCount); err != nil || evidenceCount != 1 {
		t.Fatalf("rollback lost CSV evidence: %d %v", evidenceCount, err)
	}
	if _, err := r.db.Exec(`DROP TRIGGER withdrawal_test_fail ON transactions; DROP FUNCTION withdrawal_test_fail()`); err != nil {
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
