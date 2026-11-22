package identity

import (
	"context"
	"errors"
)

var ErrConsentRequired = errors.New("external AI consent required")
var ErrInvalidUser = errors.New("authenticated user required")

// AIConsentStore must serialize authorization and changes for each user across
// server instances. WithConsent invokes fn only for a matching current grant
// and holds its authorization until fn returns. WithdrawConsent revokes every
// version for the user, including grants used by older server instances.
// During a rolling upgrade, GrantConsent and WithConsent must check the
// version against the active policy shared by all instances, rejecting stale
// servers with ErrConsentRequired even if their local version still matches.
type AIConsentStore interface {
	WithConsent(ctx context.Context, userID int64, version string, fn func(context.Context) error) error
	GrantConsent(ctx context.Context, userID int64, version string) error
	WithdrawConsent(ctx context.Context, userID int64) error
}

type AIConsentGate struct {
	Store   AIConsentStore
	Version string
}

func (g AIConsentGate) Execute(ctx context.Context, fn func(context.Context) error) error {
	userID, err := g.owner(ctx)
	if err != nil {
		return err
	}
	if g.Version == "" {
		return ErrConsentRequired
	}
	return g.Store.WithConsent(ctx, userID, g.Version, fn)
}

func (g AIConsentGate) Grant(ctx context.Context, version string) error {
	userID, err := g.owner(ctx)
	if err != nil {
		return err
	}
	if g.Version == "" || version != g.Version {
		return ErrConsentRequired
	}
	return g.Store.GrantConsent(ctx, userID, version)
}

func (g AIConsentGate) Withdraw(ctx context.Context) error {
	userID, err := g.owner(ctx)
	if err != nil {
		return err
	}
	return g.Store.WithdrawConsent(ctx, userID)
}

func (g AIConsentGate) owner(ctx context.Context) (int64, error) {
	userID := UserID(ctx)
	if userID <= 0 {
		return 0, ErrInvalidUser
	}
	if g.Store == nil {
		return 0, ErrConsentRequired
	}
	return userID, ctx.Err()
}
