// Package identity は認証済みの利用者IDを context でアプリケーション各層へ渡す。
package identity

import "context"

type key struct{}

func WithUser(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// UserID は認証情報がない場合に0を返す。実利用者のIDとして扱わないこと。
func UserID(ctx context.Context) int64 { id, _ := ctx.Value(key{}).(int64); return id }
