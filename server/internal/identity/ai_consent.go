package identity

import (
	"context"
	"errors"
)

var ErrConsentRequired = errors.New("external AI consent required")
var ErrInvalidUser = errors.New("authenticated user required")

// AIConsentStore must serialize WithConsent and SetConsent for each user across server
// instances. WithConsent invokes fn only for a matching current grant and holds
// its authorization until fn returns. A completed withdrawal therefore cannot
// be followed by a newly started request under the withdrawn grant.
// Calls already authorized may finish before the withdrawal completes.
type AIConsentStore interface {
	WithConsent(ctx context.Context, userID int64, version string, fn func(context.Context) error) error
	SetConsent(ctx context.Context, userID int64, version string, granted bool) error
}

type AIConsentGate struct {
	Store   AIConsentStore
	Version string
}

func (g AIConsentGate) Execute(ctx context.Context, userID int64, fn func(context.Context) error) error {
	if userID <= 0 {
		return ErrInvalidUser
	}
	if g.Store == nil || g.Version == "" {
		return ErrConsentRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return g.Store.WithConsent(ctx, userID, g.Version, fn)
}

func (g AIConsentGate) Set(ctx context.Context, userID int64, version string, granted bool) error {
	if userID <= 0 {
		return ErrInvalidUser
	}
	if g.Store == nil || g.Version == "" || version != g.Version {
		return ErrConsentRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return g.Store.SetConsent(ctx, userID, version, granted)
}
