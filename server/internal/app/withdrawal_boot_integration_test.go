package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
	"github.com/shuichirohayafuji/spendable-today/server/internal/config"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// Integration test uses a new isolated schema, never the application's schema.
func TestWithdrawalIntakeDisabledInAppComposition(t *testing.T) {
	raw := os.Getenv("ARRAN_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := fmt.Sprintf("withdrawal_boot_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	cfg := config.Config{DatabaseURL: parsed.String(), Environment: "local", UseMockLLM: true, WithdrawalBatchEnabled: true}
	application, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	scoped, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer scoped.Close()
	password, err := auth.HashPassword("synthetic password")
	if err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err = scoped.QueryRowContext(ctx, `INSERT INTO users(username,password_hash) VALUES('synthetic-user',$1) RETURNING id`, password).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	// Make the scheduler-health prerequisite valid before testing the app
	// composition. Otherwise the repository itself would reject admission and
	// hide a broken application-level safety gate.
	if _, err = scoped.ExecContext(ctx, `UPDATE withdrawal_batch_health SET checked_at=CURRENT_TIMESTAMP WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":"synthetic-user","password":"synthetic password"}`))
	rec := httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d", rec.Code)
	}
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &login); err != nil || login.AccessToken == "" {
		t.Fatal("invalid login response")
	}
	req = httptest.NewRequest(http.MethodPut, "/withdrawal-request", nil)
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	rec = httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("enabled flag bypassed safety gate: %d", rec.Code)
	}
	var count int
	if err := scoped.QueryRowContext(ctx, `SELECT count(*) FROM withdrawal_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("disabled intake recorded %d requests: %v", count, err)
	}
	req = httptest.NewRequest(http.MethodGet, "/withdrawal-request", nil)
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	rec = httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(rec, req)
	var status struct {
		Requested bool `json:"requested"`
		Accepting bool `json:"accepting"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &status) != nil || status.Requested || status.Accepting {
		t.Fatalf("incorrect status response: %d %s", rec.Code, rec.Body.String())
	}
	// With the identical healthy DB and owner, bypassing only the app-level
	// switch must be able to record the request. This proves the earlier 503 is
	// not an accidental consequence of missing heartbeat or other preconditions.
	if err := application.repository.RequestWithdrawal(identity.WithUser(ctx, userID), userID); err != nil {
		t.Fatalf("healthy repository prerequisites were not actually satisfied: %v", err)
	}
	// An already-pending request must not be processed by the scheduled CLI
	// while the same runtime safety gate keeps new requests closed.
	if _, err := scoped.ExecContext(ctx, `UPDATE withdrawal_requests SET requested_at=CURRENT_TIMESTAMP-INTERVAL '2 days' WHERE user_id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if err := RunWithdrawalBatch(ctx, cfg, time.Now()); err == nil {
		t.Fatal("batch ran while intake safety gate is closed")
	}
	var active bool
	var completed sql.NullTime
	if err := scoped.QueryRowContext(ctx, `SELECT u.is_active,w.completed_at FROM users u JOIN withdrawal_requests w ON w.user_id=u.id WHERE u.id=$1`, userID).Scan(&active, &completed); err != nil || !active || completed.Valid {
		t.Fatalf("batch changed a pending user while disabled: active=%v completed=%v err=%v", active, completed.Valid, err)
	}
	if err := RefreshWithdrawalBatchHealth(ctx, cfg); err == nil {
		t.Fatal("disabled batch refreshed scheduler health")
	}
}
