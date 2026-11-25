package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestWithdrawalEvidenceV2Persistence(t *testing.T) {
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
	// Reconstruct the real pre-v23 schema, retain a v1 row and migrate in place.
	for _, q := range []string{`ALTER TABLE evidence_intents DROP COLUMN request_operation_id, DROP COLUMN request_time_utc, DROP COLUMN format_version`, `DELETE FROM schema_migrations WHERE version=23`} {
		if _, err = r.db.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	legacy := evidenceIntent{OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	if _, err = r.db.DB.ExecContext(ctx, `INSERT INTO evidence_intents(user_id,owner_id,ledger_id,operation_id,kind,policy_version) VALUES(701,$1,$2,$3,$4,'')`, owner, legacy.LedgerID, legacy.OperationID, legacy.Kind); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, r.db.DB); err != nil {
		t.Fatal(err)
	}
	if got, err := r.readEvidenceIntent(a, legacy); err != nil || got != legacy {
		t.Fatalf("v1 row after migration: %+v %v", got, err)
	}
	p := withdrawalV2Payload{OwnerID: owner, LedgerID: legacy.LedgerID, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Kind: "withdrawal_requested", RequestedAt: "2026-11-24T15:00:00Z"}
	if err = r.prepareWithdrawalV2(a, p); err != nil {
		t.Fatal(err)
	}
	if err = r.prepareWithdrawalV2(a, p); err != nil {
		t.Fatalf("retry: %v", err)
	}
	// Discard idle connections so the next read establishes a fresh DB session.
	r.db.DB.SetMaxIdleConns(0)
	if got, err := r.readWithdrawalV2(a, owner, p.OperationID); err != nil || got != p {
		t.Fatalf("reload: %+v %v", got, err)
	}
	if _, err = r.readWithdrawalV2(b, owner, p.OperationID); err == nil {
		t.Fatal("other owner read accepted")
	}
	v1Alias := evidenceIntent{OwnerID: owner, LedgerID: p.LedgerID, OperationID: p.OperationID, Kind: p.Kind}
	if _, err = r.readEvidenceIntent(a, v1Alias); err == nil {
		t.Fatal("v2 read as v1")
	}
	if _, err = r.prepareEvidenceIntent(a, v1Alias); err == nil {
		t.Fatal("v2 collision prepared as v1")
	}
	conflict := p
	conflict.OperationID = legacy.OperationID
	if err = r.prepareWithdrawalV2(a, conflict); err == nil {
		t.Fatal("v1 operation reused by v2")
	}
	changed := p
	changed.RequestedAt = "2026-11-24T15:00:01Z"
	if err = r.prepareWithdrawalV2(a, changed); err == nil {
		t.Fatal("changed payload reused operation")
	}
	otherOwner, err := r.activeEvidenceOwner(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "2026-11-25T00:00:00+09:00", "2026-11-24T15:00:00.000Z"} {
		changed = p
		changed.OwnerID = otherOwner
		changed.OperationID = "dddddddddddddddddddddddddddddddd"
		changed.RequestedAt = bad
		if err = r.prepareWithdrawalV2(b, changed); err == nil {
			t.Fatalf("bad time %q accepted", bad)
		}
		var exists bool
		if err = r.db.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM evidence_intents WHERE operation_id=$1)`, changed.OperationID).Scan(&exists); err != nil || exists {
			t.Fatalf("invalid payload left a row: %v", err)
		}
	}
	end := withdrawalV2Payload{OwnerID: owner, LedgerID: p.LedgerID, OperationID: "cccccccccccccccccccccccccccccccc", Kind: "withdrawal_completed", RequestedAt: p.RequestedAt, RequestOperationID: p.OperationID}
	if err = r.prepareWithdrawalV2(a, end); err != nil {
		t.Fatal(err)
	}
	if got, err := r.readWithdrawalV2(a, owner, end.OperationID); err != nil || got != end {
		t.Fatalf("completion reload: %+v %v", got, err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `UPDATE evidence_intents SET request_time_utc='2026-11-24T15:00:01Z' WHERE user_id=701 AND operation_id=$1`, end.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.readWithdrawalV2(a, owner, end.OperationID); err == nil {
		t.Fatal("inconsistent reference accepted")
	}
}
