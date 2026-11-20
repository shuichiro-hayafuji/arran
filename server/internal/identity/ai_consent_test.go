package identity

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type testStore struct {
	mu      sync.Mutex
	grants  map[int64]string
	failure error
}

func (s *testStore) WithConsent(ctx context.Context, id int64, version string, fn func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if s.grants[id] != version {
		return ErrConsentRequired
	}
	return fn(ctx)
}
func (s *testStore) SetConsent(_ context.Context, id int64, version string, granted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if granted {
		s.grants[id] = version
	} else {
		delete(s.grants, id)
	}
	return nil
}
func TestGateConsentVersionOwnerAndFailure(t *testing.T) {
	ctx := context.Background()
	store := &testStore{grants: map[int64]string{}}
	gate := AIConsentGate{Store: store, Version: "v1"}
	called := 0
	send := func(context.Context) error { called++; return nil }
	if !errors.Is(gate.Execute(ctx, 1, send), ErrConsentRequired) {
		t.Fatal("missing grant accepted")
	}
	if err := gate.Set(ctx, 1, "v1", true); err != nil {
		t.Fatal(err)
	}
	if err := gate.Execute(ctx, 1, send); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(ctx, 2, send), ErrConsentRequired) {
		t.Fatal("grant leaked to other user")
	}
	next := AIConsentGate{Store: store, Version: "v2"}
	if !errors.Is(next.Execute(ctx, 1, send), ErrConsentRequired) {
		t.Fatal("old grant accepted")
	}
	if !errors.Is(next.Set(ctx, 1, "v1", true), ErrConsentRequired) {
		t.Fatal("old version accepted")
	}
	if err := gate.Set(ctx, 1, "v1", false); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gate.Execute(ctx, 1, send), ErrConsentRequired) {
		t.Fatal("withdrawn grant accepted")
	}
	store.failure = errors.New("storage failure")
	if gate.Execute(ctx, 1, send) == nil {
		t.Fatal("storage failure accepted")
	}
	if called != 1 {
		t.Fatalf("sent %d times", called)
	}
}
func TestGateRejectsInvalidConfigurationAndCancelledContext(t *testing.T) {
	gate := AIConsentGate{}
	if !errors.Is(gate.Execute(context.Background(), 0, func(context.Context) error { return nil }), ErrInvalidUser) {
		t.Fatal("invalid user accepted")
	}
	if !errors.Is(gate.Execute(context.Background(), 1, func(context.Context) error { return nil }), ErrConsentRequired) {
		t.Fatal("missing store accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gate = AIConsentGate{Store: &testStore{grants: map[int64]string{}}, Version: "v1"}
	if !errors.Is(gate.Execute(ctx, 1, func(context.Context) error { return nil }), context.Canceled) {
		t.Fatal("cancelled context accepted")
	}
}

func TestWithdrawalWaitsForAuthorizedRequestAndBlocksLaterRequests(t *testing.T) {
	ctx := context.Background()
	store := &testStore{grants: map[int64]string{1: "v1"}}
	gate := AIConsentGate{Store: store, Version: "v1"}
	started := make(chan struct{})
	finish := make(chan struct{})
	sent := make(chan error, 1)
	go func() {
		sent <- gate.Execute(ctx, 1, func(context.Context) error { close(started); <-finish; return nil })
	}()
	<-started
	revoked := make(chan error, 1)
	go func() { revoked <- gate.Set(ctx, 1, "v1", false) }()
	close(finish)
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	if err := gate.Execute(ctx, 1, func(context.Context) error { t.Error("request started after withdrawal"); return nil }); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("got %v", err)
	}
}
