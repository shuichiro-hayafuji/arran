package auth

import (
	"context"
	"encoding/json"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	user    User
	hash    string
	expires time.Time
	allowed bool
}

func (f *fakeStore) FindUser(_ context.Context, name string) (User, error) {
	if name != f.user.Username {
		return User{}, ErrInvalid
	}
	return f.user, nil
}
func (f *fakeStore) AllowLogin(context.Context, string, string) (bool, error) { return f.allowed, nil }
func (f *fakeStore) CreateSession(_ context.Context, hash string, _ User, expires time.Time) error {
	f.hash = hash
	f.expires = expires
	return nil
}
func (f *fakeStore) ResolveSession(_ context.Context, hash string) (User, error) {
	if hash != f.hash || !f.user.Active || time.Now().After(f.expires) {
		return User{}, ErrInvalid
	}
	return f.user, nil
}
func (f *fakeStore) DeleteSession(context.Context, string) error { f.hash = ""; return nil }

func TestPasswordHash(t *testing.T) {
	a, err := HashPassword("correct long password")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := HashPassword("correct long password")
	if a == b {
		t.Fatal("salts reused")
	}
	if !VerifyPassword(a, "correct long password") || VerifyPassword(a, "wrong") {
		t.Fatal("verification failed")
	}
	for _, value := range []string{"", "plaintext", passwordPrefix + "invalid$invalid", strings.Replace(a, "600000", "999999999", 1)} {
		if VerifyPassword(value, "correct long password") {
			t.Fatal("accepted malformed hash")
		}
	}
	if _, err = HashPassword("short"); err == nil {
		t.Fatal("weak password accepted")
	}
}
func TestSessionLifecycleAndBoundary(t *testing.T) {
	hash, _ := HashPassword("correct long password")
	store := &fakeStore{user: User{ID: 7, Username: "alice", PasswordHash: hash, Active: true}, allowed: true}
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" && identity.UserID(r.Context()) != 7 {
			t.Error("missing identity")
		}
		w.WriteHeader(200)
	})
	h := New(store).Wrap(protected)
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/profile", "/transactions", "/consultations/1", "/memories/1", "/future-endpoint", "/auth/me", "/auth/logout"} {
		if w := request("GET", path, "", ""); w.Code != 401 {
			t.Fatalf("unprotected %s: %d", path, w.Code)
		}
	}
	if request("GET", "/health", "", "").Code != 200 {
		t.Fatal("health protected")
	}
	if request("OPTIONS", "/profile", "", "").Code != 204 {
		t.Fatal("preflight failed")
	}
	for _, body := range []string{`{"username":"alice","password":"bad"}`, `{"username":"unknown","password":"bad"}`} {
		if w := request("POST", "/auth/login", body, ""); w.Code != 401 {
			t.Fatalf("bad login: %d", w.Code)
		}
	}
	w := request("POST", "/auth/login", `{"username":" ALICE ","password":"correct long password"}`, "")
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	var result struct {
		Token string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Token) != 43 || store.hash == result.Token || store.hash != digest(result.Token) {
		t.Fatal("raw token stored or invalid token")
	}
	if strings.Contains(w.Body.String(), hash) {
		t.Fatal("password hash leaked")
	}
	if request("GET", "/profile", "", result.Token).Code != 200 {
		t.Fatal("authorized request failed")
	}
	if request("GET", "/auth/me", "", result.Token).Code != 200 {
		t.Fatal("me failed")
	}
	store.user.Active = false
	if request("GET", "/profile", "", result.Token).Code != 401 {
		t.Fatal("disabled user accepted")
	}
	store.user.Active = true
	store.expires = time.Now().Add(-time.Second)
	if request("GET", "/profile", "", result.Token).Code != 401 {
		t.Fatal("expired token accepted")
	}
	store.expires = time.Now().Add(time.Hour)
	if request("POST", "/auth/logout", "", result.Token).Code != 204 {
		t.Fatal("logout failed")
	}
	if request("GET", "/profile", "", result.Token).Code != 401 {
		t.Fatal("revoked token accepted")
	}
	store.allowed = false
	if request("POST", "/auth/login", `{"username":"alice","password":"correct long password"}`, "").Code != 429 {
		t.Fatal("rate limit bypassed")
	}
}
