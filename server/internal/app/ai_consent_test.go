package app

import (
	"context"
	"errors"
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type gateStore struct {
	before error
	after  error
}

func (s gateStore) WithConsent(ctx context.Context, _ int64, _ string, fn func(context.Context) error) error {
	if s.before != nil {
		return s.before
	}
	if err := fn(ctx); err != nil {
		return err
	}
	return s.after
}
func (gateStore) GrantConsent(context.Context, int64, string) error { return nil }
func (gateStore) WithdrawConsent(context.Context, int64) error      { return nil }

func TestConsentExecutorDistinguishesDenialFromStoreAndSendFailure(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	storeFailure := errors.New("database failed")
	sendFailure := errors.New("network failed")
	tests := []struct {
		name   string
		store  gateStore
		send   error
		want   error
		called bool
	}{
		{"denied", gateStore{before: identity.ErrConsentRequired}, nil, consultation.ErrConsentRequired, false},
		{"storage failure", gateStore{before: storeFailure}, nil, consultation.ErrConsentUnavailable, false},
		{"send failure", gateStore{}, sendFailure, sendFailure, true},
		{"completion save failure", gateStore{after: storeFailure}, nil, consultation.ErrAIOutcomeUnknown, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			executor := consentExecutor{identity.AIConsentGate{Store: tc.store, Version: "v1"}}
			err := executor.Execute(ctx, func(context.Context) error { called = true; return tc.send })
			if !errors.Is(err, tc.want) || called != tc.called {
				t.Fatalf("err=%v called=%v", err, called)
			}
		})
	}
}
