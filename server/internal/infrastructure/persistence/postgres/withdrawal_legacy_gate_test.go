package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestWithdrawalLegacyAdmission(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	for _, kind := range []string{"grant", "consultation"} {
		t.Run(kind, func(t *testing.T) {
			r := openReconciliationRepository(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			owner := identity.WithUser(ctx, 701)
			if _, err := r.db.DB.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v1',description='policy' WHERE singleton=TRUE`); err != nil {
				t.Fatal(err)
			}
			var schema string
			if err := r.db.DB.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
			if err != nil {
				t.Fatal(err)
			}
			q := u.Query()
			q.Set("search_path", schema)
			u.RawQuery = q.Encode()
			peer, err := sql.Open("pgx", u.String())
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			blocker, err := peer.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			var pid int
			if err := blocker.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			if _, err := blocker.ExecContext(ctx, `SELECT id FROM users WHERE id=701 FOR NO KEY UPDATE`); err != nil {
				t.Fatal(err)
			}
			if _, err := blocker.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id) VALUES(701)`); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if kind == "grant" {
					done <- r.GrantConsent(owner, 701, "v1")
				} else {
					_, e := r.CreateConsultation(owner, domain.Consultation{})
					done <- e
				}
			}()
			deadline := time.Now().Add(4 * time.Second)
			for {
				var waiting bool
				if err := peer.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("write did not wait for admission lock")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := blocker.Commit(); err != nil {
				t.Fatal(err)
			}
			select {
			case e := <-done:
				if e == nil {
					t.Fatal("operation succeeded after admission")
				}
			case <-ctx.Done():
				t.Fatal("operation did not return")
			}
			var n int
			table := "consultations"
			if kind == "grant" {
				table = "ai_user_consents"
			}
			if err := peer.QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE user_id=701").Scan(&n); err != nil || n != 0 {
				t.Fatalf("blocked write persisted %d: %v", n, err)
			}
		})
	}
}
