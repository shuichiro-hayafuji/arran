package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPurchaseCSVHTTPConfirmationCountAndOwnerIsolation(t *testing.T) {
	repo, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "reconciliation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	owner := identity.WithUser(context.Background(), 1)
	state, _ := repo.LoadChatState(owner)
	amount := int64(2500)
	state.Purchases = []consultation.Purchase{{ID: "one", Description: "購入", Amount: 2500, ActualAmount: &amount, Status: "purchased", RecordedAt: "2035-01-01T00:00:00Z"}}
	if _, err = repo.SaveChatTurn(owner, state, consultation.Thread{CreatedAt: "2035-01-01T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "購入"}}, Reply: consultation.Reply{Status: "answered", Answer: "保存"}}); err != nil {
		t.Fatal(err)
	}
	app := application.New(application.Config{Repository: repo, ChatRepository: repo})
	api := New(app)
	request := func(user int64, method, path string, payload []byte, content string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(payload)).WithContext(identity.WithUser(context.Background(), user))
		r.Header.Set("Content-Type", content)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	preview := func(decisions []domain.ReconciliationDecision) domain.ImportPreview {
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		file, _ := writer.CreateFormFile("file", "sample.csv")
		file.Write([]byte("date,description,amount\n2035-01-03,PRIVATE BANK MERCHANT,2500\n"))
		writer.WriteField("source", "bank")
		writer.WriteField("source_account_name", "account")
		encoded, _ := json.Marshal(decisions)
		writer.WriteField("reconciliation_decisions", string(encoded))
		writer.Close()
		out := request(1, "POST", "/transactions/import/preview", b.Bytes(), writer.FormDataContentType())
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
		var p domain.ImportPreview
		decodeResponse(t, out, &p)
		return p
	}
	p := preview(nil)
	if len(p.ReconciliationConflicts) != 1 || p.PreviewID != "" {
		t.Fatal("unconfirmed purchase CSV became committable", p)
	}
	conflict := p.ReconciliationConflicts[0]
	d := domain.ReconciliationDecision{Row: 2, Action: "same", PurchaseID: "one", Snapshot: conflict.Candidates[0].Snapshot, StateVersion: conflict.StateVersion}
	p = preview([]domain.ReconciliationDecision{d})
	if p.PreviewID == "" {
		t.Fatal("explicit choice not committable")
	}
	body, _ := json.Marshal(map[string]string{"preview_id": p.PreviewID})
	out := request(1, "POST", "/transactions/import/commit", body, "application/json")
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	out = request(1, "GET", "/expenses/count?month=2035-01", nil, "")
	var count domain.ExpenseCount
	decodeResponse(t, out, &count)
	if out.Code != 200 || count.Confirmed != 1 || count.PendingRecalculation != 1 {
		t.Fatal(out.Code, count)
	}
	rows, _ := repo.ListTransactions(owner, "", 100)
	body, _ = json.Marshal(d)
	out = request(2, "POST", fmt.Sprintf("/transactions/%d/reconciliation", rows[0].ID), body, "application/json")
	if out.Code != 404 {
		t.Fatal("foreign owner link accepted", out.Code, out.Body.String())
	}
	out = request(2, "GET", "/purchases/one/csv-candidates", nil, "")
	if out.Code != 404 {
		t.Fatal("foreign purchase visible", out.Code)
	}
	out = request(1, "GET", "/expenses/count?month=wrong", nil, "")
	if out.Code != 400 {
		t.Fatal("invalid month accepted", out.Code)
	}
	p = preview(nil)
	if len(p.ReconciliationConflicts) != 0 || p.PreviewID == "" {
		t.Fatal("confirmed identity not reused", p)
	}
}
