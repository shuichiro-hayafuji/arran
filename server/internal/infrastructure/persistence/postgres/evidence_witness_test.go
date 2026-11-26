package postgres

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
	"os"
	"testing"
)

type fakeEvidenceConfirmer func(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error)

func (f fakeEvidenceConfirmer) ConfirmOperation(ctx context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
	return f(ctx, e)
}
func TestEvidenceWitnessReadback(t *testing.T) {
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
	in := evidenceIntent{OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LedgerID: "0123456789abcdef0123456789abcdef", Kind: "withdrawal_requested"}
	if _, err = r.prepareEvidenceIntent(a, in); err != nil {
		t.Fatal(err)
	}
	unavailable := fakeEvidenceConfirmer(func(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error) {
		return restoreevidence.Entry{}, errors.New("timeout")
	})
	if err = r.witnessEvidenceIntent(a, in, unavailable); err == nil {
		t.Fatal("unknown external response accepted")
	}
	bad := fakeEvidenceConfirmer(func(_ context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
		e.OwnerID = "11111111111111111111111111111111"
		e.Generation = 1
		e.Digest = restoreevidence.Digest(e)
		return e, nil
	})
	if err = r.witnessEvidenceIntent(a, in, bad); err == nil {
		t.Fatal("other owner receipt accepted")
	}
	good := fakeEvidenceConfirmer(func(_ context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
		e.Generation = 1
		e.Digest = restoreevidence.Digest(e)
		return e, nil
	})
	if err = r.witnessEvidenceIntent(a, in, good); err == nil {
		t.Fatal("unbound event witnessed")
	}
	bound := restoreevidence.Entry{LedgerID: in.LedgerID, OwnerID: in.OwnerID, OperationID: in.OperationID, Kind: in.Kind, PolicyVersion: in.PolicyVersion, Generation: 1}
	bound.Digest = restoreevidence.Digest(bound)
	binds := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { binds <- r.bindEvidenceAttempt(a, in, bound) }()
	}
	for i := 0; i < 2; i++ {
		if err = <-binds; err != nil {
			t.Fatalf("concurrent bind: %v", err)
		}
	}
	changedAttempt := bound
	changedAttempt.Generation = 2
	changedAttempt.Previous = "0000000000000000000000000000000000000000000000000000000000000000"
	changedAttempt.Digest = restoreevidence.Digest(changedAttempt)
	if err = r.bindEvidenceAttempt(a, in, changedAttempt); err == nil {
		t.Fatal("bound event changed")
	}
	journal := evidenceJournal{repo: r}
	restored, err := journal.Load(a, in.OwnerID, in.OperationID)
	if err != nil || restored.Attempt == nil || *restored.Attempt != bound || restored.Witnessed {
		t.Fatalf("fresh prepared readback: %+v %v", restored, err)
	}
	if _, err = journal.Load(b, in.OwnerID, in.OperationID); err == nil {
		t.Fatal("other owner read journal")
	}
	if err = r.witnessEvidenceIntent(b, in, good); err == nil {
		t.Fatal("other owner witnessed")
	}
	wrongLedger := in
	wrongLedger.LedgerID = "11111111111111111111111111111111"
	if err = r.witnessEvidenceIntent(a, wrongLedger, good); err == nil {
		t.Fatal("different ledger witnessed")
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- r.witnessEvidenceIntent(a, in, good) }()
	}
	for i := 0; i < 2; i++ {
		if err = <-results; err != nil {
			t.Fatalf("concurrent witness: %v", err)
		}
	}
	// A restarted caller repeats the readback using retained payload; no second
	// local change or external send is needed.
	if err = r.witnessEvidenceIntent(a, in, good); err != nil {
		t.Fatalf("idempotent readback: %v", err)
	}
	restored, err = journal.Load(a, in.OwnerID, in.OperationID)
	if err != nil || !restored.Witnessed || restored.Attempt == nil || *restored.Attempt != bound || restored.Generation != bound.Generation || restored.Digest != bound.Digest {
		t.Fatalf("fresh witnessed readback: %+v %v", restored, err)
	}
	changed := fakeEvidenceConfirmer(func(_ context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
		e.Generation = 2
		e.Previous = "0000000000000000000000000000000000000000000000000000000000000000"
		e.Digest = restoreevidence.Digest(e)
		return e, nil
	})
	if err = r.witnessEvidenceIntent(a, in, changed); err == nil {
		t.Fatal("different receipt accepted after witness")
	}
	var state string
	var generation int64
	var digest string
	if err = r.db.DB.QueryRowContext(ctx, `SELECT state,event_generation,event_digest FROM evidence_intents WHERE user_id=701 AND operation_id=$1`, in.OperationID).Scan(&state, &generation, &digest); err != nil || state != "witnessed" || generation != 1 || len(digest) != 64 {
		t.Fatalf("receipt: %s %d %v", state, generation, err)
	}
}
