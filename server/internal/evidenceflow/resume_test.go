package evidenceflow

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
	"testing"
)

type fakeLease struct{ expired, releaseFail bool }

func (l *fakeLease) Valid(context.Context) error {
	if l.expired {
		return errors.New("expired")
	}
	return nil
}
func (l *fakeLease) ConfirmOperation(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error) {
	return restoreevidence.Entry{}, errors.New("test confirmer unavailable")
}
func (l *fakeLease) Release(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("deadline missing")
	}
	if l.releaseFail {
		return errors.New("release failed")
	}
	return nil
}

type fakeJournal struct {
	intent         Intent
	marked         bool
	fail           bool
	lostAfter      bool
	bindBeforeFail bool
	bindAfterFail  bool
}

func (j *fakeJournal) Load(_ context.Context, owner, op string) (Intent, error) {
	if owner != j.intent.Expected.OwnerID || op != j.intent.Expected.OperationID {
		return Intent{}, errors.New("wrong owner")
	}
	return j.intent, nil
}
func (j *fakeJournal) BindAttempt(_ context.Context, _ Intent, e restoreevidence.Entry) error {
	if j.bindBeforeFail {
		return errors.New("bind failed")
	}
	if j.intent.Attempt != nil && *j.intent.Attempt != e {
		return errors.New("attempt changed")
	}
	copy := e
	j.intent.Attempt = &copy
	if j.bindAfterFail {
		return errors.New("bind response lost")
	}
	return nil
}
func (j *fakeJournal) Witness(_ context.Context, _ Lease, i Intent, e restoreevidence.Entry) error {
	if j.fail {
		return errors.New("commit lost")
	}
	if !same(i.Expected, e) {
		return errors.New("mismatch")
	}
	j.marked = true
	j.intent.Witnessed = true
	j.intent.Generation = e.Generation
	j.intent.Digest = e.Digest
	if j.lostAfter {
		return errors.New("response lost after commit")
	}
	return nil
}

type fakeAuthority struct {
	event                                       *restoreevidence.Entry
	lease                                       *fakeLease
	failRead, failWrite, lost, alterAfterAppend bool
	appends                                     int
	head                                        restoreevidence.Head
}

func (a *fakeAuthority) Acquire(context.Context, restoreevidence.Entry) (Lease, error) {
	return a.lease, nil
}
func (a *fakeAuthority) Lookup(_ context.Context, _ Lease, _ restoreevidence.Entry) (Lookup, restoreevidence.Entry, error) {
	if a.failRead {
		return Unknown, restoreevidence.Entry{}, errors.New("read unavailable")
	}
	if a.event == nil {
		return Absent, restoreevidence.Entry{}, nil
	}
	return Found, *a.event, nil
}
func (a *fakeAuthority) Head(context.Context, Lease) (restoreevidence.Head, error) {
	return a.head, nil
}
func (a *fakeAuthority) Append(_ context.Context, l Lease, _ restoreevidence.Head, e restoreevidence.Entry) error {
	if l.Valid(context.Background()) != nil {
		return errors.New("lease invalid")
	}
	a.appends++
	if a.failWrite {
		return errors.New("write unavailable")
	}
	a.event = &e
	if a.alterAfterAppend {
		x := e
		x.Generation++
		x.Previous = "0000000000000000000000000000000000000000000000000000000000000000"
		x.Digest = restoreevidence.Digest(x)
		a.event = &x
	}
	if a.lost {
		return errors.New("response lost")
	}
	return nil
}

func TestResumeRejectsDifferentReadbackAfterAppend(t *testing.T) {
	e := restoreevidence.Entry{LedgerID: "0123456789abcdef0123456789abcdef", OwnerID: "fedcba9876543210fedcba9876543210", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	j := &fakeJournal{intent: Intent{Expected: e}}
	a := &fakeAuthority{lease: &fakeLease{}, head: restoreevidence.Head{LedgerID: e.LedgerID}, alterAfterAppend: true}
	if Resume(context.Background(), j, a, e.OwnerID, e.OperationID) == nil || j.marked {
		t.Fatal("different readback witnessed")
	}
	if Resume(context.Background(), j, a, e.OwnerID, e.OperationID) == nil || j.marked || a.appends != 1 {
		t.Fatal("restart accepted contradictory readback")
	}
}
func TestResumeFailsClosedAndRetriesLostResponses(t *testing.T) {
	e := restoreevidence.Entry{LedgerID: "0123456789abcdef0123456789abcdef", OwnerID: "fedcba9876543210fedcba9876543210", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	j := &fakeJournal{intent: Intent{Expected: e}}
	a := &fakeAuthority{lease: &fakeLease{}, head: restoreevidence.Head{LedgerID: e.LedgerID}, failRead: true}
	run := func() error { return Resume(context.Background(), j, a, e.OwnerID, e.OperationID) }
	if run() == nil || a.appends != 0 || j.marked {
		t.Fatal("read failure authorized append")
	}
	a.failRead = false
	a.lease.expired = true
	if run() == nil || a.appends != 0 {
		t.Fatal("expired lease authorized append")
	}
	a.lease.expired = false
	a.lost = true
	if run() == nil || a.appends != 1 || j.marked || a.event == nil {
		t.Fatal("ambiguous append accepted")
	}
	a.lost = false
	j.fail = true
	if run() == nil || a.appends != 1 || j.marked {
		t.Fatal("ambiguous journal commit accepted")
	}
	j.fail = false
	j.lostAfter = true
	if run() == nil || !j.marked || !j.intent.Witnessed || a.appends != 1 {
		t.Fatal("committed journal result not treated as unknown")
	}
	j.lostAfter = false
	if err := run(); err != nil || !j.marked || a.appends != 1 {
		t.Fatalf("restart recovery: %v", err)
	}
	correct := *a.event
	a.event = nil
	if run() == nil || a.appends != 1 {
		t.Fatal("witnessed absence caused another append")
	}
	valid := restoreevidence.Entry{LedgerID: e.LedgerID, OwnerID: e.OwnerID, OperationID: e.OperationID, Kind: e.Kind, Generation: 2, Previous: "0000000000000000000000000000000000000000000000000000000000000000"}
	valid.Digest = restoreevidence.Digest(valid)
	a.event = &valid
	if run() == nil || a.appends != 1 {
		t.Fatal("different witnessed generation accepted")
	}
	a.event = &correct
	a.lease.releaseFail = true
	if run() == nil {
		t.Fatal("release failure was hidden")
	}
	a.lease.releaseFail = false
	wrong := e
	wrong.Kind = "consent_granted"
	wrong.PolicyVersion = "v1"
	wrong.Generation = 1
	wrong.Digest = restoreevidence.Digest(wrong)
	a.event = &wrong
	j.marked = false
	if run() == nil || j.marked {
		t.Fatal("different payload witnessed")
	}
	if Resume(context.Background(), j, a, "other", e.OperationID) == nil {
		t.Fatal("other owner accepted")
	}
}

func TestResumeBindsBeforeAppendAndHoldsConflictingHead(t *testing.T) {
	e := restoreevidence.Entry{LedgerID: "0123456789abcdef0123456789abcdef", OwnerID: "fedcba9876543210fedcba9876543210", OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested"}
	j := &fakeJournal{intent: Intent{Expected: e}, bindBeforeFail: true}
	a := &fakeAuthority{lease: &fakeLease{}, head: restoreevidence.Head{LedgerID: e.LedgerID}}
	run := func() error { return Resume(context.Background(), j, a, e.OwnerID, e.OperationID) }
	if run() == nil || j.intent.Attempt != nil || a.appends != 0 {
		t.Fatal("append after failed bind")
	}
	j.bindBeforeFail = false
	j.bindAfterFail = true
	if run() == nil || j.intent.Attempt == nil || a.appends != 0 {
		t.Fatal("ambiguous bind authorized append")
	}
	j.bindAfterFail = false
	a.head.Generation = 1
	a.head.Digest = "0000000000000000000000000000000000000000000000000000000000000000"
	if err := run(); !errors.Is(err, ErrManualHold) || a.appends != 0 {
		t.Fatalf("conflict not held: %v", err)
	}
	a.lease.releaseFail = true
	if err := run(); !errors.Is(err, ErrManualHold) || !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("lost conflict on release failure: %v", err)
	}
}
