package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
	"os"
	"testing"
	"time"
)

func TestEvidenceIntentPreparation(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	a := identity.WithUser(ctx, 701)
	b := identity.WithUser(ctx, 702)
	owner, err := r.activeEvidenceOwner(a)
	if err != nil {
		t.Fatal(err)
	}
	in := evidenceIntent{OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", Kind: "withdrawal_requested"}
	type answer struct {
		intent evidenceIntent
		err    error
	}
	ch := make(chan answer, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := r.prepareEvidenceIntent(a, in); ch <- answer{v, e} }()
	}
	x, y := <-ch, <-ch
	if x.err != nil || y.err != nil || x.intent != y.intent || len(x.intent.OwnerID) != 32 {
		t.Fatalf("idempotent prepare: %+v %+v", x, y)
	}
	for _, tc := range []struct {
		generation int
		previous   string
	}{{1, "0000000000000000000000000000000000000000000000000000000000000000"}, {2, ""}} {
		if _, err := r.db.DB.ExecContext(ctx, `UPDATE evidence_intents SET attempt_generation=$1,attempt_previous=$2,attempt_digest=$3 WHERE user_id=701 AND operation_id=$4`, tc.generation, tc.previous, "0000000000000000000000000000000000000000000000000000000000000000", in.OperationID); err == nil {
			t.Fatal("invalid generation/previous accepted")
		}
	}
	if _, err := r.prepareEvidenceIntent(b, in); err == nil {
		t.Fatal("other owner reused operation")
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO evidence_intents(user_id,operation_id,owner_id,ledger_id,kind,policy_version) VALUES (702,'dddddddddddddddddddddddddddddddd',$1,'0123456789abcdef0123456789abcdef','withdrawal_requested','')`, x.intent.OwnerID); err == nil {
		t.Fatal("database accepted another owner's identity")
	}
	changed := in
	changed.Kind = "consent_withdrawn"
	changed.PolicyVersion = "v1"
	if _, err := r.prepareEvidenceIntent(a, changed); err == nil {
		t.Fatal("different payload reused operation")
	}
	missing := evidenceIntent{OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", Kind: "consent_granted"}
	if _, err := r.prepareEvidenceIntent(a, missing); err == nil {
		t.Fatal("missing policy accepted")
	}
	send := evidenceIntent{OperationID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", Kind: "send_started"}
	if _, err := r.prepareEvidenceIntent(a, send); err == nil {
		t.Fatal("send without policy accepted")
	}
	if _, err := r.db.DB.ExecContext(ctx, `INSERT INTO evidence_intents(user_id,operation_id,owner_id,ledger_id,kind,policy_version) VALUES(701,$1,$2,'0123456789abcdef0123456789abcdef','send_started','')`, send.OperationID, owner); err == nil {
		t.Fatal("database accepted send without policy")
	}
	send.PolicyVersion = "v1"
	if _, err := r.prepareEvidenceIntent(a, send); err != nil {
		t.Fatalf("valid send after rejection: %v", err)
	}
	finish := evidenceIntent{OperationID: "cccccccccccccccccccccccccccccccc", OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", Kind: "send_completed", RelatedOperationID: send.OperationID, PolicyVersion: "v1"}
	if _, err := r.prepareEvidenceIntent(a, finish); err == nil {
		t.Fatal("unverified completion accepted")
	}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("prepared start admitted: %v", err)
	}
	started := restoreevidence.Entry{LedgerID: send.LedgerID, OwnerID: owner, OperationID: send.OperationID, Kind: send.Kind, PolicyVersion: send.PolicyVersion, Generation: 1}
	started.Digest = restoreevidence.Digest(started)
	if err := r.bindEvidenceAttempt(a, send, started); err != nil {
		t.Fatal(err)
	}
	confirmed := fakeEvidenceConfirmer(func(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error) { return started, nil })
	if err := r.witnessEvidenceIntent(a, send, confirmed); err != nil {
		t.Fatal(err)
	}
	finish.RelatedOperationID = send.OperationID
	finish.PolicyVersion = "v2"
	if _, err := r.prepareEvidenceIntent(a, finish); err == nil {
		t.Fatal("different start policy accepted")
	}
	finish.PolicyVersion = "v1"
	if _, err := r.prepareEvidenceIntent(a, finish); err != nil {
		t.Fatalf("confirmed start rejected: %v", err)
	}
	if _, err := r.prepareEvidenceIntent(a, finish); err != nil {
		t.Fatalf("completion retry rejected: %v", err)
	}
	duplicate := finish
	duplicate.OperationID = "ffffffffffffffffffffffffffffffff"
	if _, err := r.prepareEvidenceIntent(a, duplicate); err == nil {
		t.Fatal("duplicate completion accepted")
	}
	if err := r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("prepared completion admitted: %v", err)
	}
	completed := restoreevidence.Entry{LedgerID: finish.LedgerID, OwnerID: owner, OperationID: finish.OperationID, RelatedOperationID: send.OperationID, Kind: finish.Kind, PolicyVersion: finish.PolicyVersion, Generation: 2, Previous: started.Digest}
	completed.Digest = restoreevidence.Digest(completed)
	if err := r.bindEvidenceAttempt(a, finish, completed); err != nil {
		t.Fatal(err)
	}
	confirmedFinish := fakeEvidenceConfirmer(func(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error) { return completed, nil })
	if err := r.witnessEvidenceIntent(a, finish, confirmedFinish); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE evidence_intents SET policy_version='v2' WHERE user_id=701 AND operation_id=$1`, finish.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("mismatched restored completion admitted: %v", err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE evidence_intents SET policy_version='v1' WHERE user_id=701 AND operation_id=$1`, finish.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(a, 701); err != nil {
		t.Fatalf("witnessed completion blocked: %v", err)
	}
	var count int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM evidence_intents WHERE user_id=701`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("unexpected intents: %d %v", count, err)
	}
	if _, err := r.prepareEvidenceIntent(a, in); err == nil {
		t.Fatal("pending owner created or read through prepare")
	}
	// Reconcile using the payload retained before prepare, without relying on
	// either prepare response (which may have been lost after commit).
	if got, err := r.readEvidenceIntent(a, in); err != nil || got != in {
		t.Fatalf("pending read: %+v %v", got, err)
	}
	if _, err := r.readEvidenceIntent(b, x.intent); err == nil {
		t.Fatal("other owner read intent")
	}
	wrong := x.intent
	wrong.Kind = "send_started"
	if _, err := r.readEvidenceIntent(a, wrong); err == nil {
		t.Fatal("different payload read")
	}
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE users SET is_active=FALSE WHERE id=701`); err != nil {
		t.Fatal(err)
	}
	if got, err := r.readEvidenceIntent(a, x.intent); err != nil || got != x.intent {
		t.Fatalf("inactive read: %+v %v", got, err)
	}
}

func TestEvidenceSendAdmissionRace(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	ownerCtx := identity.WithUser(ctx, 701)
	owner, err := r.activeEvidenceOwner(ownerCtx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	send := evidenceIntent{OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "send_started", PolicyVersion: "v1"}
	begin := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-begin; _, e := r.prepareEvidenceIntent(ownerCtx, send); results <- e }()
	go func() { <-begin; results <- r.RequestWithdrawal(ownerCtx, 701) }()
	close(begin)
	a, b := <-results, <-results
	if (a == nil) == (b == nil) {
		t.Fatalf("expected one winner, got %v and %v", a, b)
	}
	var pending, started bool
	if err = r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=701), EXISTS(SELECT 1 FROM evidence_intents WHERE user_id=701 AND kind='send_started')`).Scan(&pending, &started); err != nil || pending == started {
		t.Fatalf("invalid state pending=%v started=%v err=%v", pending, started, err)
	}
}
func TestEvidenceUnfinishedSendBlocksBatch(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	ownerCtx := identity.WithUser(ctx, 701)
	owner, err := r.activeEvidenceOwner(ownerCtx)
	if err != nil {
		t.Fatal(err)
	}
	send := evidenceIntent{OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "send_started", PolicyVersion: "v1"}
	if _, err = r.prepareEvidenceIntent(ownerCtx, send); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id,requested_at) VALUES(701,'2026-11-24 14:59:59+00')`); err != nil {
		t.Fatal(err)
	}
	if err = r.RunWithdrawalBatch(ctx, time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("batch accepted unresolved start")
	}
	var active bool
	var completed sql.NullTime
	if err = r.db.DB.QueryRowContext(ctx, `SELECT u.is_active,w.completed_at FROM users u JOIN withdrawal_requests w ON w.user_id=u.id WHERE u.id=701`).Scan(&active, &completed); err != nil || !active || completed.Valid {
		t.Fatalf("batch changed owner active=%v completed=%v err=%v", active, completed.Valid, err)
	}
}
