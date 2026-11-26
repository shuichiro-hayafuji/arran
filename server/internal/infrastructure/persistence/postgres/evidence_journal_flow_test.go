package postgres

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceflow"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
	"os"
	"testing"
)

type journalFlowAuthority struct {
	event   *restoreevidence.Entry
	lost    bool
	appends int
	ledger  string
}

func (a *journalFlowAuthority) Acquire(context.Context, restoreevidence.Entry) (evidenceflow.Lease, error) {
	return a, nil
}
func (a *journalFlowAuthority) Valid(context.Context) error   { return nil }
func (a *journalFlowAuthority) Release(context.Context) error { return nil }
func (a *journalFlowAuthority) ConfirmOperation(_ context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
	if a.event == nil || a.event.OperationID != e.OperationID || a.event.OwnerID != e.OwnerID {
		return restoreevidence.Entry{}, errors.New("not found")
	}
	return *a.event, nil
}
func (a *journalFlowAuthority) Lookup(_ context.Context, _ evidenceflow.Lease, _ restoreevidence.Entry) (evidenceflow.Lookup, restoreevidence.Entry, error) {
	if a.event == nil {
		return evidenceflow.Absent, restoreevidence.Entry{}, nil
	}
	return evidenceflow.Found, *a.event, nil
}
func (a *journalFlowAuthority) Head(context.Context, evidenceflow.Lease) (restoreevidence.Head, error) {
	return restoreevidence.Head{LedgerID: a.ledger}, nil
}
func (a *journalFlowAuthority) Append(_ context.Context, _ evidenceflow.Lease, _ restoreevidence.Head, e restoreevidence.Entry) error {
	a.appends++
	a.event = &e
	if a.lost {
		return errors.New("response lost after commit")
	}
	return nil
}
func TestEvidenceJournalResumeAfterLostExternalResponse(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := identity.WithUser(context.Background(), 701)
	owner, err := r.activeEvidenceOwner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	in := evidenceIntent{OwnerID: owner, LedgerID: "0123456789abcdef0123456789abcdef", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	if _, err = r.prepareEvidenceIntent(ctx, in); err != nil {
		t.Fatal(err)
	}
	authority := &journalFlowAuthority{ledger: in.LedgerID, lost: true}
	journal := evidenceJournal{repo: r}
	if err = evidenceflow.Resume(ctx, journal, authority, owner, in.OperationID); err == nil || authority.appends != 1 {
		t.Fatal("unknown external response accepted")
	}
	// Reconstruct journal after the interrupted call and use its DB state.
	journal = evidenceJournal{repo: r}
	state, err := journal.Load(ctx, owner, in.OperationID)
	if err != nil || state.Attempt == nil || state.Witnessed {
		t.Fatal("attempt not durably recovered")
	}
	authority.lost = false
	if err = evidenceflow.Resume(ctx, journal, authority, owner, in.OperationID); err != nil || authority.appends != 1 {
		t.Fatalf("restart: %v appends=%d", err, authority.appends)
	}
	state, err = journal.Load(ctx, owner, in.OperationID)
	if err != nil || !state.Witnessed || state.Attempt == nil || state.Digest != state.Attempt.Digest {
		t.Fatal("receipt not persisted")
	}
}
