package evidenceauthority

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type guardedMockLease struct {
	mu       sync.Mutex
	releases int
	fail     bool
}

func (l *guardedMockLease) Release(context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.releases++
	if l.fail {
		return errors.New("unknown remote outcome")
	}
	return nil
}
func (l *guardedMockLease) count() int { l.mu.Lock(); defer l.mu.Unlock(); return l.releases }

func TestGuardedLeasePendingAndUnknown(t *testing.T) {
	ctx := context.Background()
	l := &guardedMockLease{}
	g, _ := NewReleaseGuard(l)
	op, _ := g.Begin()
	if g.Release(ctx) == nil || l.count() != 0 {
		t.Fatal("released unresolved operation")
	}
	if op.Confirm(ctx, func(context.Context) error { return errors.New("commit response lost") }) == nil {
		t.Fatal("unknown commit accepted")
	}
	if g.Release(ctx) == nil || l.count() != 0 {
		t.Fatal("released after unknown commit")
	}
	if err := op.Confirm(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := g.Release(ctx); err != nil || l.count() != 1 {
		t.Fatalf("release %v", err)
	}
	if g.Release(ctx) == nil {
		t.Fatal("duplicate release accepted")
	}
}
func TestGuardedLeaseConfirmAndReleaseSerialize(t *testing.T) {
	ctx := context.Background()
	l := &guardedMockLease{}
	g, _ := NewReleaseGuard(l)
	op, _ := g.Begin()
	entered := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- op.Confirm(ctx, func(context.Context) error { close(entered); <-finish; return nil }) }()
	<-entered
	released := make(chan error, 1)
	go func() { released <- g.Release(ctx) }()
	if err := <-released; err == nil || l.count() != 0 {
		t.Fatal("release during confirmation")
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := g.Release(ctx); err != nil || l.count() != 1 {
		t.Fatalf("release %v", err)
	}
}
func TestGuardedLeasePanicAndReleaseFailureRemainClosed(t *testing.T) {
	ctx := context.Background()
	l := &guardedMockLease{}
	g, _ := NewReleaseGuard(l)
	op, _ := g.Begin()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		_ = op.Confirm(ctx, func(context.Context) error { panic("process error") })
	}()
	if g.Release(ctx) == nil || l.count() != 0 {
		t.Fatal("released after panic")
	}
	l2 := &guardedMockLease{fail: true}
	g2, _ := NewReleaseGuard(l2)
	if g2.Release(ctx) == nil || g2.Release(ctx) == nil || l2.count() != 1 {
		t.Fatal("ambiguous release retried")
	}
	if _, err := g2.Begin(); err == nil {
		t.Fatal("started after ambiguous release")
	}
}

type reentrantLease struct {
	guard   *ReleaseGuard
	callErr error
}

func (l *reentrantLease) Release(ctx context.Context) error {
	l.callErr = l.guard.Release(ctx)
	return nil
}
func TestGuardedLeaseReentryCopiesAndMultiplePending(t *testing.T) {
	ctx := context.Background()
	l := &guardedMockLease{}
	g, _ := NewReleaseGuard(l)
	copied := *g
	a, _ := g.Begin()
	b, _ := copied.Begin()
	copiedOp := *a
	if copiedOp.Confirm(ctx, func(context.Context) error { return nil }) == nil {
		t.Fatal("copied operation accepted")
	}
	if err := a.Confirm(ctx, func(ctx context.Context) error {
		if copied.Release(ctx) == nil {
			t.Fatal("release while confirming")
		}
		if a.Confirm(ctx, func(context.Context) error { return nil }) == nil {
			t.Fatal("reentrant confirm accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if a.Confirm(ctx, func(context.Context) error { return nil }) == nil {
		t.Fatal("confirmed operation accepted again")
	}
	if copied.Release(ctx) == nil || l.count() != 0 {
		t.Fatal("released while another operation pending")
	}
	if err := b.Confirm(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := copied.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if g.Release(ctx) == nil || l.count() != 1 {
		t.Fatal("copied guard allowed duplicate release")
	}
	var zero ReleaseGuard
	if _, err := zero.Begin(); err == nil {
		t.Fatal("zero guard accepted")
	}
	if zero.Release(ctx) == nil {
		t.Fatal("zero guard released")
	}
	r := &reentrantLease{}
	gr, _ := NewReleaseGuard(r)
	r.guard = gr
	if err := gr.Release(ctx); err != nil || r.callErr == nil {
		t.Fatal("reentrant release not rejected")
	}
}
