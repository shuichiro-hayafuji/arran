// Package evidenceflow contains an unconnected evidence-only retry protocol.
// It does not perform account mutations or authorize external side effects.
package evidenceflow

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
	"time"
)

var ErrUnconfirmed = errors.New("evidence remains unconfirmed")
var ErrManualHold = errors.New("evidence attempt conflicts with authority head; manual reconciliation required")

type Intent struct {
	Expected   restoreevidence.Entry
	Witnessed  bool
	Generation uint64
	Digest     string
	Attempt    *restoreevidence.Entry
}
type Journal interface {
	// Load must recover the exact previously committed operation and owner after
	// restart. Missing, wrong owner, or uncertain DB state must return an error.
	Load(context.Context, string, string) (Intent, error)
	// BindAttempt durably saves the exact append before any external write.
	// A conflicting prior attempt or unknown commit is an error.
	BindAttempt(context.Context, Intent, restoreevidence.Entry) error
	// Witness must verify the receipt against the persisted intent atomically.
	// An unknown commit must return an error; retry starts again with Load.
	Witness(context.Context, Lease, Intent, restoreevidence.Entry) error
}
type Lookup int

const (
	Unknown Lookup = iota
	Absent
	Found
)

type Authority interface {
	// Acquire must authenticate account, table and ledger, and return an
	// exclusive durable lease. No production implementation exists yet.
	Acquire(context.Context, restoreevidence.Entry) (Lease, error)
	// Lookup must use a strongly consistent read from a verified table. Absent
	// is permitted only on a confirmed successful read of the exact operation key.
	Lookup(context.Context, Lease, restoreevidence.Entry) (Lookup, restoreevidence.Entry, error)
	// Head must read the verified authority strongly and must not invent genesis.
	Head(context.Context, Lease) (restoreevidence.Head, error)
	// Append uses atomic expected-head and operation-ID conditions. Errors include
	// conditional conflict and unknown commit and never grant permission to act.
	// Append must atomically check the still-valid lease as well as head and ID;
	// a preceding Valid call alone cannot fence a stale lease holder.
	Append(context.Context, Lease, restoreevidence.Head, restoreevidence.Entry) error
}
type Lease interface {
	Valid(context.Context) error
	Release(context.Context) error
	ConfirmOperation(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error)
}

func same(a, b restoreevidence.Entry) bool {
	return a.LedgerID == b.LedgerID && a.OwnerID == b.OwnerID && a.OperationID == b.OperationID && a.RelatedOperationID == b.RelatedOperationID && a.Kind == b.Kind && a.PolicyVersion == b.PolicyVersion
}

// Resume first reconciles by operation ID. It returns success only after the
// exact event is independently re-read and the journal confirms its receipt.
// No caller may infer a right to apply a withdrawal or perform an AI send.
func Resume(ctx context.Context, j Journal, a Authority, ownerID, operationID string) (result error) {
	if j == nil || a == nil || ownerID == "" || operationID == "" {
		return ErrUnconfirmed
	}
	intent, err := j.Load(ctx, ownerID, operationID)
	if err != nil || intent.Expected.OwnerID != ownerID || intent.Expected.OperationID != operationID {
		return ErrUnconfirmed
	}
	lease, err := a.Acquire(ctx, intent.Expected)
	if err != nil || lease == nil {
		return ErrUnconfirmed
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if lease.Release(releaseCtx) != nil {
			result = errors.Join(result, ErrUnconfirmed)
		}
	}()
	if lease.Valid(ctx) != nil {
		return ErrUnconfirmed
	}
	state, event, err := a.Lookup(ctx, lease, intent.Expected)
	if err != nil || state == Unknown {
		return ErrUnconfirmed
	}
	if intent.Witnessed && (intent.Generation == 0 || intent.Digest == "" || state != Found) {
		return ErrUnconfirmed
	}
	if intent.Attempt != nil && (!same(*intent.Attempt, intent.Expected) || intent.Attempt.Generation == 0 || intent.Attempt.Digest != restoreevidence.Digest(*intent.Attempt)) {
		return ErrUnconfirmed
	}
	if state == Absent {
		if lease.Valid(ctx) != nil {
			return ErrUnconfirmed
		}
		head, err := a.Head(ctx, lease)
		if err != nil || head.LedgerID != intent.Expected.LedgerID || head.Generation == ^uint64(0) {
			return ErrUnconfirmed
		}
		next := intent.Expected
		next.Generation = head.Generation + 1
		next.Previous = head.Digest
		next.Digest = restoreevidence.Digest(next)
		if intent.Attempt != nil && *intent.Attempt != next {
			return ErrManualHold
		}
		if intent.Attempt == nil {
			if err = j.BindAttempt(ctx, intent, next); err != nil {
				return ErrUnconfirmed
			}
			intent.Attempt = &next
		}
		if lease.Valid(ctx) != nil {
			return ErrUnconfirmed
		}
		if err = a.Append(ctx, lease, head, next); err != nil {
			return ErrUnconfirmed
		}
		state, event, err = a.Lookup(ctx, lease, intent.Expected)
		if err != nil || state != Found || event != next {
			return ErrUnconfirmed
		}
	}
	if lease.Valid(ctx) != nil {
		return ErrUnconfirmed
	}
	if state != Found || !same(event, intent.Expected) || event.Generation == 0 || event.Digest != restoreevidence.Digest(event) {
		return ErrUnconfirmed
	}
	if intent.Attempt == nil || *intent.Attempt != event {
		return ErrUnconfirmed
	}
	if intent.Witnessed && (intent.Generation != event.Generation || intent.Digest != event.Digest) {
		return ErrUnconfirmed
	}
	if err = j.Witness(ctx, lease, intent, event); err != nil {
		return ErrUnconfirmed
	}
	return nil
}
