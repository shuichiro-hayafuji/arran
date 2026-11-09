package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSVIdentityHTTPPreviewConfirmationCommit(t *testing.T) {
	repo, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	app := application.New(application.Config{Repository: repo})
	api := New(app)
	owner := identity.WithUser(context.Background(), 1)
	request := func(method, path string, payload []byte, content string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(payload)).WithContext(owner)
		r.Header.Set("Content-Type", content)
		out := httptest.NewRecorder()
		api.ServeHTTP(out, r)
		return out
	}
	preview := func(data string, decisions []domain.ImportIdentityDecision) domain.ImportPreview {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		file, _ := writer.CreateFormFile("file", "sample.csv")
		file.Write([]byte(data))
		writer.WriteField("source", "bank")
		writer.WriteField("source_account_name", "account")
		encoded, _ := json.Marshal(decisions)
		writer.WriteField("identity_decisions", string(encoded))
		writer.Close()
		response := request(http.MethodPost, "/transactions/import/preview", body.Bytes(), writer.FormDataContentType())
		if response.Code != 200 {
			t.Fatalf("preview %d %s", response.Code, response.Body.String())
		}
		var p domain.ImportPreview
		decodeResponse(t, response, &p)
		return p
	}
	commit := func(id string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"preview_id": id})
		return request(http.MethodPost, "/transactions/import/commit", body, "application/json")
	}
	// A repeated selected provider ID is an editable 400 input error, with no save.
	var invalidBody bytes.Buffer
	invalidWriter := multipart.NewWriter(&invalidBody)
	invalidFile, _ := invalidWriter.CreateFormFile("file", "repeated.csv")
	invalidFile.Write([]byte("date,description,amount,id\n2034-01-01,store,2500,a\n2034-01-01,store,2500,a\n"))
	invalidWriter.WriteField("source", "bank")
	invalidWriter.WriteField("source_account_name", "account")
	invalidWriter.WriteField("mapping", `{"transaction_id_column":"id"}`)
	invalidWriter.Close()
	invalidResponse := request(http.MethodPost, "/transactions/import/preview", invalidBody.Bytes(), invalidWriter.FormDataContentType())
	if invalidResponse.Code != 400 || !strings.Contains(invalidResponse.Body.String(), "取引ID列") || !strings.Contains(invalidResponse.Body.String(), "重複行") {
		t.Fatalf("repeated ID response %d %s", invalidResponse.Code, invalidResponse.Body.String())
	}
	if rows, err := repo.ListTransactions(owner, "2034-01", 100); err != nil || len(rows) != 0 {
		t.Fatalf("invalid preview saved rows: %+v %v", rows, err)
	}
	base := "date,description,amount\n2031-01-01,SHOP 123,2500\n"
	first := preview(base, nil)
	if first.PreviewID == "" {
		t.Fatal("first file cannot be committed")
	}
	if out := commit(first.PreviewID); out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	changed := "date,description,amount\n2031-01-01,shop 123,2500\n2031-01-02,another,2600\n"
	p := preview(changed, nil)
	if p.PreviewID != "" || len(p.IdentityConflicts) != 1 {
		t.Fatal("unknown identity auto accepted")
	}
	if out := commit(p.PreviewID); out.Code != 400 {
		t.Fatal("unresolved preview committed")
	}
	candidate := p.IdentityConflicts[0].Candidates[0]
	resolved := preview(changed, []domain.ImportIdentityDecision{{Row: 2, Action: "same", TransactionID: candidate.ID, Snapshot: p.IdentityConflicts[0].CandidateSnapshots[candidate.ID]}})
	if resolved.PreviewID == "" {
		t.Fatal("explicit match not accepted")
	}
	out := commit(resolved.PreviewID)
	if out.Code != 200 {
		t.Fatalf("commit %d %s", out.Code, out.Body.String())
	}
	var result domain.ImportCommitResult
	decodeResponse(t, out, &result)
	if result.ImportedCount != 1 || result.DuplicateCount != 1 {
		t.Fatal(result)
	}
	repeat := preview(changed, nil)
	if len(repeat.IdentityConflicts) != 0 || repeat.PreviewID == "" {
		t.Fatal("alias not reused")
	}
	out = commit(repeat.PreviewID)
	decodeResponse(t, out, &result)
	if result.ImportedCount != 0 || result.DuplicateCount != 2 {
		t.Fatal(result)
	}
	row, err := repo.GetTransaction(owner, candidate.ID)
	if err != nil || row.Description != "SHOP 123" {
		t.Fatal("original record rewritten")
	}
}
