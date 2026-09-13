// Package auth はサーバー側で失効できるランダムな Bearer トークンで認証する。
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

var ErrInvalid = errors.New("invalid credentials or session")
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.@+-]{2,127}$`)

const SessionLifetime = 7 * 24 * time.Hour

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Active       bool   `json:"-"`
}

type Store interface {
	FindUser(context.Context, string) (User, error)
	AllowLogin(context.Context, string, string) (bool, error)
	CreateSession(context.Context, string, User, time.Time) error
	ResolveSession(context.Context, string) (User, error)
	DeleteSession(context.Context, string) error
}

type Service struct {
	store     Store
	dummyHash string
	slots     chan struct{}
}

func New(store Store) *Service {
	// 未登録ユーザーでも同じハッシュ検証を行い、処理時間から登録有無を推測しにくくする。
	hash, err := HashPassword("dummy-password-never-used")
	if err != nil {
		panic(err)
	}
	return &Service{store: store, dummyHash: hash, slots: make(chan struct{}, 4)}
}

// digest はセッションの照合や試行制限に使い、トークンや識別子の生値をDBへ渡さない。
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// Wrap は明示的に公開したログイン・ヘルスチェック等を除き、全ルートに認証を要求する。
// 業務処理へ渡す context に認証済みの利用者IDを設定する。
func (s *Service) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet && (r.URL.Path == "/health" || r.URL.Path == "/healthz") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/auth/login" {
			s.login(w, r)
			return
		}
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || len(fields[1]) != 43 {
			unauthorized(w)
			return
		}
		tokenHash := digest(fields[1])
		user, err := s.store.ResolveSession(r.Context(), tokenHash)
		if errors.Is(err, ErrInvalid) {
			unauthorized(w)
			return
		}
		if err != nil {
			respond(w, 503, map[string]string{"error": "unavailable", "message": "認証サービスに接続できません。"})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/auth/logout" {
			if err := s.store.DeleteSession(r.Context(), tokenHash); err != nil {
				respond(w, 503, map[string]string{"error": "unavailable"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/auth/me" {
			respond(w, 200, user)
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.WithUser(r.Context(), user.ID)))
	})
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		respond(w, 400, map[string]string{"error": "invalid_request"})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		respond(w, 400, map[string]string{"error": "invalid_request"})
		return
	}
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	if !usernamePattern.MatchString(input.Username) || len(input.Password) > 1024 {
		unauthorized(w)
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	allowed, err := s.store.AllowLogin(r.Context(), digest("ip:"+ip), digest("user:"+input.Username))
	if err != nil {
		respond(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		respond(w, 429, map[string]string{"error": "rate_limited", "message": "しばらく待ってから再試行してください。"})
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		w.Header().Set("Retry-After", "60")
		respond(w, 429, map[string]string{"error": "rate_limited"})
		return
	}
	user, err := s.store.FindUser(r.Context(), input.Username)
	if err != nil && !errors.Is(err, ErrInvalid) {
		respond(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	hash := s.dummyHash
	if err == nil {
		hash = user.PasswordHash
	}
	valid := VerifyPassword(hash, input.Password)
	if !valid || err != nil || !user.Active {
		unauthorized(w)
		return
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		respond(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().UTC().Add(SessionLifetime)
	if err = s.store.CreateSession(r.Context(), digest(token), user, expires); err != nil {
		respond(w, 503, map[string]string{"error": "unavailable"})
		return
	}
	respond(w, 200, map[string]any{"access_token": token, "token_type": "Bearer", "expires_at": expires, "user": user})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	respond(w, 401, map[string]string{"error": "unauthorized", "message": "ログイン情報が無効か、有効期限が切れています。"})
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
