package evidenceauthority

import (
	"context"
	"errors"
	"sync"
)

// ReleaseGuard is an unconnected, process-local experiment. It is not an AWS
// fence, cannot stop other clients, and must not authorize witnessed state or
// application operations. An unknown remote release is never retried here.
// Copies share state. A zero value is invalid.
type ReleaseGuard struct{ state *releaseGuardState }
type releaseGuardState struct {
	mu      sync.Mutex
	lease   interface{ Release(context.Context) error }
	pending map[*GuardedOperation]bool
	closed  bool
}
type GuardedOperation struct{ owner *releaseGuardState }

func NewReleaseGuard(lease interface{ Release(context.Context) error }) (*ReleaseGuard, error) {
	if lease == nil {
		return nil, ErrUnconfirmed
	}
	return &ReleaseGuard{state: &releaseGuardState{lease: lease, pending: make(map[*GuardedOperation]bool)}}, nil
}
func (g *ReleaseGuard) Begin() (*GuardedOperation, error) {
	if g == nil || g.state == nil {
		return nil, ErrUnconfirmed
	}
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrUnconfirmed
	}
	op := &GuardedOperation{owner: s}
	s.pending[op] = false
	return op, nil
}

// Confirm marks the operation in progress before invoking caller code without a
// lock. Error or panic leaves it pending. Success is only local bookkeeping.
func (op *GuardedOperation) Confirm(ctx context.Context, confirm func(context.Context) error) (err error) {
	if op == nil || op.owner == nil || confirm == nil || ctx == nil {
		return ErrUnconfirmed
	}
	s := op.owner
	s.mu.Lock()
	running, ok := s.pending[op]
	if s.closed || !ok || running || ctx.Err() != nil {
		s.mu.Unlock()
		return ErrUnconfirmed
	}
	s.pending[op] = true
	s.mu.Unlock()
	success := false
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if success {
			delete(s.pending, op)
		} else {
			s.pending[op] = false
		}
	}()
	if err = confirm(ctx); err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	success = true
	return nil
}

// Release closes shared state before calling the remote lease. An error, panic,
// or ambiguous result cannot cause this guard to retry the release.
func (g *ReleaseGuard) Release(ctx context.Context) error {
	if g == nil || g.state == nil || ctx == nil {
		return ErrUnconfirmed
	}
	s := g.state
	s.mu.Lock()
	if s.closed || len(s.pending) != 0 || ctx.Err() != nil {
		s.mu.Unlock()
		return ErrUnconfirmed
	}
	s.closed = true
	s.mu.Unlock()
	if err := s.lease.Release(ctx); err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	return nil
}
