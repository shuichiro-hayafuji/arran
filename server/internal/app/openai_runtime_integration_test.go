package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
	"github.com/shuichirohayafuji/spendable-today/server/internal/config"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/postgres"
)

type runtimeFakeTransport func(*http.Request) (*http.Response, error)

func (f runtimeFakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Requires a disposable cluster. Roles and schema are unique; HTTP is fake only.
func TestOpenAIAccountingInRuntimeApp(t *testing.T) {
	raw := os.Getenv("ARRAN_ROLE_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("disposable role test cluster required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(random[:])
	schema := "arran_ai_" + suffix
	owner := "migrate_" + suffix
	runtime := "runtime_" + suffix
	exec := func(q string) {
		t.Helper()
		if _, err := admin.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	cleanup := func(q string) {
		c, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.ExecContext(c, q); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}
	roleURL := func(role string) string {
		t.Helper()
		var secret [24]byte
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
		pw := hex.EncodeToString(secret[:])
		exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s'", role, pw))
		t.Cleanup(func() { cleanup("DROP ROLE " + role) })
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		u.User = url.UserPassword(role, pw)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String()
	}
	ownerURL := roleURL(owner)
	runtimeURL := roleURL(runtime)
	exec("CREATE SCHEMA " + schema + " AUTHORIZATION " + owner)
	t.Cleanup(func() { cleanup("DROP SCHEMA " + schema + " CASCADE") })
	if err := postgres.MigrateURL(ctx, ownerURL); err != nil {
		t.Fatal(err)
	}
	grants, err := os.ReadFile("../../../infra/operations/grant-runtime.sql")
	if err != nil {
		t.Fatal(err)
	}
	grant := strings.ReplaceAll(string(grants), "SCHEMA public", "SCHEMA "+schema)
	grant = strings.ReplaceAll(grant, "public.", schema+".")
	grant = strings.ReplaceAll(grant, "arran_runtime", runtime)
	exec(grant)
	ownerDB, err := sql.Open("pgx", ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer ownerDB.Close()
	pw, err := auth.HashPassword("fixture password")
	if err != nil {
		t.Fatal(err)
	}
	var uid int64
	if err = ownerDB.QueryRowContext(ctx, `INSERT INTO users(username,password_hash) VALUES('fixture-user',$1) RETURNING id`, pw).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if _, err = ownerDB.ExecContext(ctx, `UPDATE ai_consent_policy SET version='fixture-v1',description='fixture' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	calls := 0
	oldTransport := http.DefaultTransport
	http.DefaultTransport = runtimeFakeTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://api.openai.com/v1/responses" {
			return nil, fmt.Errorf("unexpected URL")
		}
		var payload struct {
			Text struct {
				Format struct {
					Name string `json:"name"`
				} `json:"format"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		answer := fmt.Sprintf(`{"status":"provisional","answer":"fixture answer %d","reason":"fixture","assumptions":"fixture","question":"","purchase_change":null,"changes":[],"strictness":null}`, calls)
		if payload.Text.Format.Name == "monthly_review" {
			answer = `{"candidates":[]}`
		}
		encoded, err := json.Marshal(answer)
		if err != nil {
			return nil, err
		}
		body := fmt.Sprintf(`{"status":"completed","output":[{"content":[{"type":"output_text","text":%s}]}],"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`, encoded)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer func() { http.DefaultTransport = oldTransport }()
	application, err := New(ctx, config.Config{DatabaseURL: runtimeURL, Environment: "production", OpenAIAPIKey: "fixture", OpenAIModel: "fixture", OpenAIReasoningEffort: "medium", AIConsentVersion: "fixture-v1"})
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		application.Server.Handler.ServeHTTP(rec, req)
		return rec
	}
	login := request("POST", "/auth/login", `{"username":"fixture-user","password":"fixture password"}`, "")
	var logged struct {
		AccessToken string `json:"access_token"`
	}
	if login.Code != 200 || json.Unmarshal(login.Body.Bytes(), &logged) != nil || logged.AccessToken == "" {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	consent := request("PUT", "/ai-consent", `{"version":"fixture-v1"}`, logged.AccessToken)
	if consent.Code != 200 {
		t.Fatalf("consent: %d %s", consent.Code, consent.Body.String())
	}
	first := request("POST", "/consultations", `{"message":"fixture"}`, logged.AccessToken)
	if first.Code != 201 {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	var chat struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal(first.Body.Bytes(), &chat) != nil || chat.ID == 0 {
		t.Fatal("invalid consultation")
	}
	nextPath := fmt.Sprintf("/consultations/%d/messages", chat.ID)
	second := request("POST", nextPath, `{"message":"again"}`, logged.AccessToken)
	if second.Code != 200 {
		t.Fatalf("second: %d %s", second.Code, second.Body.String())
	}
	count := func() (int, int, int) {
		t.Helper()
		var a, b, c int
		if err := ownerDB.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER (WHERE usage_id IS NOT NULL) FROM ai_usage_operations WHERE user_id=$1`, uid).Scan(&a, &b); err != nil {
			t.Fatal(err)
		}
		if err := ownerDB.QueryRowContext(ctx, `SELECT count(*) FROM llm_usage WHERE user_id=$1`, uid).Scan(&c); err != nil {
			t.Fatal(err)
		}
		return a, b, c
	}
	if a, b, c := count(); calls != 2 || a != 2 || b != 2 || c != 2 {
		t.Fatalf("initial calls=%d operations=%d receipts=%d usage=%d", calls, a, b, c)
	}
	exec("REVOKE INSERT ON " + schema + ".ai_usage_operations FROM " + runtime)
	denied := request("POST", nextPath, `{"message":"third"}`, logged.AccessToken)
	var rejected struct {
		Status           string `json:"status"`
		ResponseSource   string `json:"response_source"`
		AIRecommendation string `json:"ai_recommendation"`
	}
	deniedErr := json.Unmarshal(denied.Body.Bytes(), &rejected)
	if calls != 2 || denied.Code != 200 || deniedErr != nil || rejected.Status != "unavailable" || rejected.ResponseSource != "unavailable" || rejected.AIRecommendation != "外部AIへの送信許可を確認できません。相談内容は保存しました。" {
		t.Fatalf("insert denied: calls=%d status=%d %s", calls, denied.Code, denied.Body.String())
	}
	if a, b, c := count(); a != 2 || b != 2 || c != 2 {
		t.Fatalf("insert denied: %d %d %d", a, b, c)
	}
	exec("GRANT INSERT ON " + schema + ".ai_usage_operations TO " + runtime)
	exec("REVOKE UPDATE ON " + schema + ".ai_usage_operations FROM " + runtime)
	uncertain := request("POST", nextPath, `{"message":"fourth"}`, logged.AccessToken)
	var result struct {
		AIRecommendation string `json:"ai_recommendation"`
		Messages         []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	decodeErr := json.Unmarshal(uncertain.Body.Bytes(), &result)
	if calls != 3 || uncertain.Code != 200 || decodeErr != nil || result.AIRecommendation != "fixture answer 3" || len(result.Messages) == 0 || result.Messages[len(result.Messages)-1].Role != "assistant" || result.Messages[len(result.Messages)-1].Content != "fixture answer 3" {
		t.Fatalf("update denied: calls=%d status=%d %s", calls, uncertain.Code, uncertain.Body.String())
	}
	if a, b, c := count(); a != 3 || b != 2 || c != 2 {
		t.Fatalf("update denied: %d %d %d", a, b, c)
	}
	exec("GRANT UPDATE ON " + schema + ".ai_usage_operations TO " + runtime)
	profile := request("PUT", "/profile", `{"monthly_free_budget":1000,"advice_strictness":"バランス型","reduce_categories":[],"allowed_categories":[]}`, logged.AccessToken)
	if profile.Code != 200 {
		t.Fatalf("profile: %d %s", profile.Code, profile.Body.String())
	}
	review := request("POST", "/reviews/monthly", `{"month":"2026-11"}`, logged.AccessToken)
	if review.Code != 201 || calls != 4 {
		t.Fatalf("review: calls=%d status=%d %s", calls, review.Code, review.Body.String())
	}
	var reviewReceipts int
	if err := ownerDB.QueryRowContext(ctx, `SELECT count(*) FROM ai_usage_operations o JOIN llm_usage l ON l.id=o.usage_id AND l.user_id=o.user_id WHERE o.user_id=$1 AND l.operation='monthly_review'`, uid).Scan(&reviewReceipts); err != nil || reviewReceipts != 1 {
		t.Fatalf("review receipt=%d err=%v", reviewReceipts, err)
	}
}
