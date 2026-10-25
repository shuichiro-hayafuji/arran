package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCSVPreviewRestartAndLostHTTPResponse(t *testing.T) {
	owner := identity.WithUser(context.Background(), 1)
	path := filepath.Join(t.TempDir(), "restart.db")
	repo, err := sqlite.Open(owner, path)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(application.Config{Repository: repo})
	preview, err := app.PreviewImport(owner, []byte("date,description,amount\n2035-01-01,PRIVATE,500\n"), domain.CSVMapping{}, "bank", "account")
	if err != nil || preview.PreviewID == "" {
		t.Fatal(preview, err)
	}
	if err = repo.Close(); err != nil {
		t.Fatal(err)
	}
	var first domain.ImportCommitResult
	for attempt := 0; attempt < 2; attempt++ {
		repo, err = sqlite.Open(owner, path)
		if err != nil {
			t.Fatal(err)
		}
		api := New(application.New(application.Config{Repository: repo}))
		payload, _ := json.Marshal(map[string]string{"preview_id": preview.PreviewID})
		request := httptest.NewRequest("POST", "/transactions/import/commit", bytes.NewReader(payload)).WithContext(owner)
		request.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		api.ServeHTTP(out, request)
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
		var result domain.ImportCommitResult
		decodeResponse(t, out, &result)
		if attempt == 0 {
			first = result
		} else if result != first {
			t.Fatal("lost response replay changed result", first, result)
		}
		rows, err := repo.ListTransactions(owner, "", 100)
		if err != nil || len(rows) != 1 {
			t.Fatal("duplicate ledger", len(rows), err)
		}
		if err = repo.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
