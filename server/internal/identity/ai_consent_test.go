package identity

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

type testStore struct {
	activeVersion string
	lock          chan struct{}
	grants        map[int64]string
	failure       error
}

func newTestStore(grants map[int64]string) *testStore {
	return &testStore{activeVersion: "v1", lock: make(chan struct{}, 1), grants: grants}
}

func (s *testStore) WithConsent(ctx context.Context, id int64, version string, fn func(context.Context) error) error {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()
	if s.failure != nil {
		return s.failure
	}
	if version != s.activeVersion || s.grants[id] != version {
		return ErrConsentRequired
	}
	return fn(ctx)
}

func (s *testStore) GrantConsent(_ context.Context, id int64, version string) error {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()
	if s.failure != nil {
		return s.failure
	}
	if version != s.activeVersion {
		return ErrConsentRequired
	}
	s.grants[id] = version
	return nil
}

func (s *testStore) WithdrawConsent(_ context.Context, id int64) error {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()
	if s.failure != nil {
		return s.failure
	}
	delete(s.grants, id)
	return nil
}

func TestGateConsentVersionOwnerAndFailure(t *testing.T) {
	ctx := WithUser(context.Background(), 1)
	store := newTestStore(map[int64]string{})
	gate := AIConsentGate{Store: store, Version: "v1"}
	called := 0
	send := func(context.Context) error { called++; return nil }
	if !errors.Is(gate.Execute(ctx, send), ErrConsentRequired) {
		t.Fatal("missing grant accepted")
	}
	if err := gate.Grant(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Execute(ctx, send); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(WithUser(ctx, 2), send), ErrConsentRequired) {
		t.Fatal("grant leaked to other user")
	}
	next := AIConsentGate{Store: store, Version: "v2"}
	if !errors.Is(next.Execute(ctx, send), ErrConsentRequired) {
		t.Fatal("old grant accepted")
	}
	if !errors.Is(next.Grant(ctx, "v1"), ErrConsentRequired) {
		t.Fatal("old version accepted")
	}
	if err := gate.Withdraw(ctx); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(ctx, send), ErrConsentRequired) {
		t.Fatal("withdrawn grant accepted")
	}
	store.failure = errors.New("storage failure")
	if gate.Execute(ctx, send) == nil {
		t.Fatal("storage failure accepted")
	}
	if called != 1 {
		t.Fatalf("sent %d times", called)
	}
}

func TestGateRejectsInvalidConfigurationAndCancelledContext(t *testing.T) {
	gate := AIConsentGate{}
	if !errors.Is(gate.Execute(context.Background(), func(context.Context) error { return nil }), ErrInvalidUser) {
		t.Fatal("invalid user accepted")
	}
	if !errors.Is(gate.Execute(WithUser(context.Background(), 1), func(context.Context) error { return nil }), ErrConsentRequired) {
		t.Fatal("missing store accepted")
	}
	ctx, cancel := context.WithCancel(WithUser(context.Background(), 1))
	cancel()
	gate = AIConsentGate{Store: newTestStore(map[int64]string{}), Version: "v1"}
	if !errors.Is(gate.Execute(ctx, func(context.Context) error { return nil }), context.Canceled) {
		t.Fatal("cancelled context accepted")
	}
}

func TestWithdrawalWaitsForAuthorizedRequestAndBlocksLaterRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := WithUser(context.Background(), 1)
		store := newTestStore(map[int64]string{1: "v1"})
		gate := AIConsentGate{Store: store, Version: "v1"}
		started := make(chan struct{})
		finish := make(chan struct{})
		sent := make(chan error, 1)
		go func() {
			sent <- gate.Execute(ctx, func(context.Context) error { close(started); <-finish; return nil })
		}()
		<-started
		revoked := make(chan error, 1)
		go func() { revoked <- gate.Withdraw(ctx) }()
		synctest.Wait()
		select {
		case err := <-revoked:
			close(finish)
			t.Fatalf("withdrawal finished while authorized request was active: %v", err)
		default:
		}
		close(finish)
		if err := <-sent; err != nil {
			t.Fatal(err)
		}
		if err := <-revoked; err != nil {
			t.Fatal(err)
		}
		if err := gate.Execute(ctx, func(context.Context) error { t.Error("request started after withdrawal"); return nil }); !errors.Is(err, ErrConsentRequired) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestConsentRequiresAuthenticatedOwner(t *testing.T) {
	gate := AIConsentGate{Store: newTestStore(map[int64]string{1: "v1"}), Version: "v1"}
	ctx := context.Background()
	if err := gate.Grant(ctx, "v1"); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("unauthenticated grant: %v", err)
	}
	if err := gate.Withdraw(ctx); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("unauthenticated withdrawal: %v", err)
	}
	if err := gate.Execute(ctx, func(context.Context) error { t.Fatal("unauthenticated send"); return nil }); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("unauthenticated send: %v", err)
	}
}

func TestWithdrawalAfterVersionChange(t *testing.T) {
	store := newTestStore(map[int64]string{1: "v1"})
	ctx := WithUser(context.Background(), 1)
	store.activeVersion = "v2"
	next := AIConsentGate{Store: store, Version: "v2"}
	if err := next.Withdraw(WithUser(ctx, 2)); err != nil {
		t.Fatal(err)
	}
	if store.grants[1] != "v1" {
		t.Fatal("another user withdrew the grant")
	}
	if err := next.Withdraw(ctx); err != nil {
		t.Fatalf("withdrawal from older client: %v", err)
	}
	if _, ok := store.grants[1]; ok {
		t.Fatal("withdrawal kept the old grant")
	}
	old := AIConsentGate{Store: store, Version: "v1"}
	if err := old.Grant(ctx, "v1"); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("stale server granted consent: %v", err)
	}
	if err := old.Execute(ctx, func(context.Context) error { t.Fatal("withdrawn grant was used"); return nil }); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("old server after withdrawal: %v", err)
	}
}
