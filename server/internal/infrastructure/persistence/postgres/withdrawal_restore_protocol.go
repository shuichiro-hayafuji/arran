package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// An isolated restore transaction owns one consistent snapshot until commit or
// rollback. Implementations must prevent all other writers, and the caller must
// retain the deployment and independently authenticated ledger leases until the
// cutover completes. These interfaces are intentionally not wired to production.
type withdrawalRestoreTx interface {
	validate(context.Context, string) error
	users(context.Context) ([]withdrawalRestoreUser, error)
	replay(context.Context, withdrawalReplayPlan, withdrawalIdentity) error
	commit() error
	rollback() error
}
type withdrawalRestoreDB interface {
	candidateID() string
	begin(context.Context) (withdrawalRestoreTx, error)
}
type withdrawalRestoreCutover interface {
	// checkOtherSafety must prove consent, backup identity and all other restore
	// constraints. Withdrawal reconciliation alone cannot authorise a cutover.
	checkOtherSafety(context.Context, string) error
	// switchWithoutPublishing atomically switches only the explicitly verified
	// candidate under the same release lease. It MUST NOT publish an API or
	// enable external AI; a separate verified operational decision is required.
	switchWithoutPublishing(context.Context, string) error
}

// reconcileAndSwitchWithdrawalRestore performs only a fail-closed offline
// cutover. It cannot authenticate a ledger, prove freshness or stop a process;
// all injected dependencies must implement those guarantees, held continuously.
// No production caller currently supplies them. Any uncertain read, database
// commit or cutover outcome is an error, and the API must remain unpublished.
func reconcileAndSwitchWithdrawalRestore(ctx context.Context, guard withdrawalQuiescence, ledger withdrawalLedgerLease, target withdrawalRestoreDB, cutover withdrawalRestoreCutover) error {
	if guard == nil || ledger == nil || target == nil || cutover == nil {
		return errWithdrawalProtocolUnsafe
	}
	candidate := target.candidateID()
	if candidate == "" {
		return errWithdrawalProtocolUnsafe
	}
	if err := guard.check(ctx); err != nil {
		return fmt.Errorf("check stopped writers: %w", err)
	}
	if err := cutover.checkOtherSafety(ctx, candidate); err != nil {
		return fmt.Errorf("check restore safety: %w", err)
	}
	entries, head, err := ledger.read(ctx)
	if err != nil {
		return fmt.Errorf("read independent ledger: %w", err)
	}
	if head == nil {
		return errWithdrawalLedgerUnsafe
	}
	// A ledger implementation may reuse and mutate its checkpoint buffer on a
	// subsequent read. Pin the initial authenticated value, not its pointer.
	initialHead := *head
	tx, err := target.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin restore reconciliation: %w", err)
	}
	if tx == nil {
		return errWithdrawalProtocolUnsafe
	}
	defer tx.rollback()
	if err := tx.validate(ctx, candidate); err != nil {
		return fmt.Errorf("validate restore candidate: %w", err)
	}
	users, err := tx.users(ctx)
	if err != nil {
		return fmt.Errorf("read restored users: %w", err)
	}
	plan, err := planWithdrawalReplay(entries, &initialHead, users)
	if err != nil {
		return err
	}
	for _, id := range plan.deleteIdentities {
		if err := tx.replay(ctx, plan, id); err != nil {
			return fmt.Errorf("replay withdrawal: %w", err)
		}
	}
	if err := verifyWithdrawalRestoreHead(ctx, ledger, initialHead); err != nil {
		return err
	}
	if err := guard.check(ctx); err != nil {
		return fmt.Errorf("recheck stopped writers: %w", err)
	}
	if err := tx.commit(); err != nil {
		return fmt.Errorf("commit restore reconciliation: %w", err)
	}
	// Commit may have succeeded even when the response was lost. On any error,
	// the caller must keep the candidate isolated and reconcile it again.
	if err := verifyWithdrawalRestoreHead(ctx, ledger, initialHead); err != nil {
		return err
	}
	if err := guard.check(ctx); err != nil {
		return fmt.Errorf("recheck stopped writers after commit: %w", err)
	}
	if err := cutover.checkOtherSafety(ctx, candidate); err != nil {
		return fmt.Errorf("recheck restore safety: %w", err)
	}
	if err := cutover.switchWithoutPublishing(ctx, candidate); err != nil {
		return fmt.Errorf("switch isolated restore: %w", err)
	}
	return nil
}

func verifyWithdrawalRestoreHead(ctx context.Context, ledger withdrawalLedgerLease, expected withdrawalLedgerCheckpoint) error {
	entries, current, err := ledger.read(ctx)
	if err != nil {
		return fmt.Errorf("recheck independent ledger: %w", err)
	}
	if current == nil || *current != expected {
		return errWithdrawalLedgerUnsafe
	}
	// Even with an authenticated unchanged head, independently validate all
	// received entries and reject pending or malformed records.
	_, err = planWithdrawalReplay(entries, current, []withdrawalRestoreUser{})
	return err
}

// withdrawalSQLRestoreTx adapts a caller-owned privileged transaction. The
// caller must independently bind the candidate fingerprint to the actual
// restore source, endpoint and cutover, prevent name/OID/cluster-ID reuse, hold
// all relevant leases, and supply trusted ledger/guard/cutover implementations.
// A PostgreSQL fingerprint is not authentication or an ABA-prevention lease.
type withdrawalSQLRestoreTx struct{ tx *sql.Tx }

func (t *withdrawalSQLRestoreTx) validate(ctx context.Context, candidate string) error {
	observed, err := t.candidateFingerprint(ctx)
	if err != nil {
		return err
	}
	if candidate == "" || observed != candidate {
		return errWithdrawalUnsafe
	}
	version, err := appliedPrefix(ctx, t.tx)
	if err != nil {
		return err
	}
	if version != 17 || len(migrations) != 17 {
		return errWithdrawalUnsafe
	}
	return verifyWithdrawalSchema(ctx, t.tx)
}

// candidateFingerprint needs a privileged connection to pg_control_system;
// insufficient privilege or an unreadable identity is an error, never an
// empty/verified identity. A signed independent record is still required.
func (t *withdrawalSQLRestoreTx) candidateFingerprint(ctx context.Context) (string, error) {
	var name, dbOID, schemaOID, clusterID string
	err := t.tx.QueryRowContext(ctx, `SELECT current_database(), d.oid::text, n.oid::text, c.system_identifier::text
FROM pg_database d CROSS JOIN pg_namespace n CROSS JOIN pg_control_system() c
WHERE d.datname=current_database() AND n.nspname=current_schema()`).Scan(&name, &dbOID, &schemaOID, &clusterID)
	if err != nil {
		return "", fmt.Errorf("read restore candidate identity: %w", err)
	}
	if name == "" || dbOID == "" || schemaOID == "" || clusterID == "" {
		return "", errWithdrawalUnsafe
	}
	return fmt.Sprintf("%s/%s/%s/%s", clusterID, dbOID, schemaOID, name), nil
}

func (t *withdrawalSQLRestoreTx) users(ctx context.Context) ([]withdrawalRestoreUser, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT id,username,created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []withdrawalRestoreUser{}
	for rows.Next() {
		var u withdrawalRestoreUser
		if err := rows.Scan(&u.userID, &u.username, &u.createdAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
func (t *withdrawalSQLRestoreTx) replay(ctx context.Context, plan withdrawalReplayPlan, id withdrawalIdentity) error {
	return replayWithdrawalRows(ctx, t.tx, plan, id)
}
func (t *withdrawalSQLRestoreTx) commit() error   { return t.tx.Commit() }
func (t *withdrawalSQLRestoreTx) rollback() error { return t.tx.Rollback() }
