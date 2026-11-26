// Package evidencebridge connects candidate components for testing only. It is
// not used by the application; deployment trust and writer isolation are open.
package evidencebridge

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceauthority"
	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceflow"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
)

type Authority struct {
	authority     *evidenceauthority.Authority
	table, ledger string
}

func New(a *evidenceauthority.Authority, table, ledger string) (*Authority, error) {
	if a == nil || !a.Matches(table, ledger) {
		return nil, evidenceflow.ErrUnconfirmed
	}
	return &Authority{authority: a, table: table, ledger: ledger}, nil
}

type session struct {
	owner *Authority
	lease *evidenceauthority.Lease
	store restoreevidence.DynamoStore
}

func (s *session) Valid(ctx context.Context) error   { return s.lease.Valid(ctx) }
func (s *session) Release(ctx context.Context) error { return s.lease.Release(ctx) }
func (s *session) ConfirmOperation(ctx context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
	return s.store.ConfirmOperation(ctx, e)
}
func (a *Authority) Acquire(ctx context.Context, expected restoreevidence.Entry) (evidenceflow.Lease, error) {
	if a == nil || expected.LedgerID != a.ledger {
		return nil, evidenceflow.ErrUnconfirmed
	}
	lease, err := a.authority.Acquire(ctx)
	if err != nil {
		return nil, evidenceflow.ErrUnconfirmed
	}
	return &session{owner: a, lease: lease, store: restoreevidence.DynamoStore{Client: lease.Client(), Table: a.table, LedgerID: a.ledger}}, nil
}
func (a *Authority) session(l evidenceflow.Lease) (*session, error) {
	s, ok := l.(*session)
	if !ok || s == nil || s.owner != a {
		return nil, evidenceflow.ErrUnconfirmed
	}
	return s, nil
}
func (a *Authority) Lookup(ctx context.Context, l evidenceflow.Lease, e restoreevidence.Entry) (evidenceflow.Lookup, restoreevidence.Entry, error) {
	s, err := a.session(l)
	if err != nil {
		return evidenceflow.Unknown, restoreevidence.Entry{}, err
	}
	result, event, err := s.store.LookupOperation(ctx, e)
	if err != nil {
		return evidenceflow.Unknown, restoreevidence.Entry{}, evidenceflow.ErrUnconfirmed
	}
	switch result {
	case restoreevidence.LookupAbsent:
		return evidenceflow.Absent, restoreevidence.Entry{}, nil
	case restoreevidence.LookupFound:
		return evidenceflow.Found, event, nil
	default:
		return evidenceflow.Unknown, restoreevidence.Entry{}, errors.New("unknown authority result")
	}
}
func (a *Authority) Head(ctx context.Context, l evidenceflow.Lease) (restoreevidence.Head, error) {
	s, err := a.session(l)
	if err != nil {
		return restoreevidence.Head{}, err
	}
	return s.store.ReadHead(ctx)
}
func (a *Authority) Append(ctx context.Context, l evidenceflow.Lease, h restoreevidence.Head, e restoreevidence.Entry) error {
	s, err := a.session(l)
	if err != nil {
		return err
	}
	return s.store.Append(ctx, h, e)
}
