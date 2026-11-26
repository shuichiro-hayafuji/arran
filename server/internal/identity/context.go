// Package identity は認証済みの利用者IDを context でアプリケーション各層へ渡す。
package identity

import (
	"context"
	"errors"
)

var ErrWithdrawalBlocked = errors.New("withdrawal cannot be accepted")
var ErrWithdrawalPending = errors.New("account withdrawal is pending")

type key struct{}
type usageTrackingKey struct{}
type usageOperationKey struct{}

func WithUser(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// UserID は認証情報がない場合に0を返す。実利用者のIDとして扱わないこと。
func UserID(ctx context.Context) int64 { id, _ := ctx.Value(key{}).(int64); return id }

// RequireUsageTracking requests an accounting marker in the same transaction
// as consent admission. It does not itself authorize an external request.
func RequireUsageTracking(ctx context.Context) context.Context {
	return context.WithValue(ctx, usageTrackingKey{}, true)
}

func UsageTrackingRequired(ctx context.Context) bool {
	v, _ := ctx.Value(usageTrackingKey{}).(bool)
	return v
}

func WithUsageOperation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, usageOperationKey{}, id)
}

func UsageOperation(ctx context.Context) string {
	id, _ := ctx.Value(usageOperationKey{}).(string)
	return id
}
