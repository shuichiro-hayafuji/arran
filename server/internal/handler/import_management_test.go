package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestImportManagementHTTPResultsAndReactivation(t *testing.T) {
	r, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "management.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	app := application.New(application.Config{Repository: r, ChatRepository: r})
	api := New(app)
	request := func(owner int64, method, path string, payload any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(method, path, bytes.NewReader(b)).WithContext(identity.WithUser(context.Background(), owner))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		api.ServeHTTP(out, req)
		return out
	}
	preview := func(confirmed bool) domain.ImportPreview {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		file, _ := writer.CreateFormFile("file", "bank.csv")
		file.Write([]byte("date,description,amount\n2035-01-01,PRIVATE STORE,2500\n"))
		writer.WriteField("source", "bank")
		writer.WriteField("source_account_name", "account")
		if confirmed {
			writer.WriteField("confirm_reactivation", "true")
		}
		writer.Close()
		req := httptest.NewRequest("POST", "/transactions/import/preview", &buf).WithContext(identity.WithUser(context.Background(), 3432))
		req.Header.Set("Content-Type", writer.FormDataContentType())
		out := httptest.NewRecorder()
		api.ServeHTTP(out, req)
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
		var p domain.ImportPreview
		decodeResponse(t, out, &p)
		return p
	}
	p := preview(false)
	out := request(3432, "POST", "/transactions/import/commit", map[string]string{"preview_id": p.PreviewID})
	if out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	var first domain.ImportCommitResult
	decodeResponse(t, out, &first)
	if first.ImportID == "" {
		t.Fatal("missing durable import result")
	}
	out = request(3432, "POST", "/transactions/import/commit", map[string]string{"preview_id": p.PreviewID})
	var repeated domain.ImportCommitResult
	decodeResponse(t, out, &repeated)
	if out.Code != 200 || repeated != first {
		t.Fatal("same commit changed result", repeated)
	}
	out = request(3432, "GET", "/imports/"+first.ImportID, nil)
	var batch domain.ImportBatch
	decodeResponse(t, out, &batch)
	if batch.Metadata.FileName != "bank.csv" || batch.CancelAffected != 1 {
		t.Fatal(batch)
	}
	input := domain.MutationInput{ID: "cancel-http", Version: batch.Version, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot}
	out = request(3433, "POST", "/imports/"+batch.ID+"/cancel", input)
	if out.Code != 404 {
		t.Fatal("owner mutation leaked", out.Code, out.Body.String())
	}
	out = request(3432, "POST", "/imports/"+batch.ID+"/cancel", input)
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	var result domain.MutationResult
	decodeResponse(t, out, &result)
	if result.Affected != 1 || result.Pending != 1 {
		t.Fatal(result)
	}
	out = request(3432, "POST", "/imports/"+batch.ID+"/cancel", input)
	var again domain.MutationResult
	decodeResponse(t, out, &again)
	if out.Code != 200 || again != result {
		t.Fatal("duplicate cancellation changed result")
	}
	input.Reason = "different"
	out = request(3432, "POST", "/imports/"+batch.ID+"/cancel", input)
	if out.Code != 409 {
		t.Fatal("same ID different payload accepted", out.Code)
	}
	out = request(3432, "GET", "/mutations/cancel-http", nil)
	decodeResponse(t, out, &again)
	if again != result {
		t.Fatal("result lookup differs")
	}
	unconfirmed := preview(false)
	if len(unconfirmed.ReactivationIDs) != 1 || unconfirmed.PreviewID != "" {
		t.Fatal("reactivation unconfirmed", unconfirmed)
	}
	confirmed := preview(true)
	out = request(3432, "POST", "/transactions/import/commit", map[string]string{"preview_id": confirmed.PreviewID})
	decodeResponse(t, out, &repeated)
	if out.Code != 200 || repeated.ReactivatedCount != 1 || repeated.ImportedCount != 0 {
		t.Fatal(repeated, out.Code)
	}
	row := batch.Rows[0].Current
	out = request(3432, "GET", fmt.Sprintf("/transactions/%d", row.ID), nil)
	var record domain.TransactionRecord
	decodeResponse(t, out, &record)
	if !record.Active {
		t.Fatal("restored row hidden")
	}
	amt := int64(1800)
	date := "2035-02-01"
	change := domain.MutationInput{ID: "correct-http", Version: record.Current.Revision, StateVersion: record.StateVersion, Amount: &amt, Date: &date}
	out = request(3432, "PATCH", fmt.Sprintf("/transactions/%d", row.ID), change)
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	out = request(3432, "GET", fmt.Sprintf("/transactions/%d", row.ID), nil)
	decodeResponse(t, out, &record)
	if record.Current.Amount != 1800 || record.Original.Amount != 2500 {
		t.Fatal("bank original not preserved", record)
	}
}
