package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/agentadapter"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
)

func TestPrimaryHandlerFlow(t *testing.T) {
	api := newTestHandler(t)

	missing := jsonRequest(t, api, http.MethodGet, "/profile", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing profile status=%d body=%s", missing.Code, missing.Body.String())
	}

	profile := domain.Profile{
		MonthlyIncome:      400000,
		CurrentBalance:     1200000,
		MonthlyFixedCosts:  180000,
		MonthlyFreeBudget:  60000,
		MonthlySavingsGoal: 80000,
		ReduceCategories:   []string{"酒・飲み会"},
		AllowedCategories:  []string{"学習", "健康"},
		LongTermGoal:       "1年で100万円貯める",
		AdviceStrictness:   "バランス型",
	}
	response := jsonRequest(t, api, http.MethodPut, "/profile", profile)
	if response.Code != http.StatusOK {
		t.Fatalf("put profile status=%d body=%s", response.Code, response.Body.String())
	}
	response = jsonRequest(t, api, http.MethodGet, "/profile", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("get profile status=%d body=%s", response.Code, response.Body.String())
	}
	var savedProfile domain.Profile
	decodeResponse(t, response, &savedProfile)
	if savedProfile.MonthlyFreeBudget != profile.MonthlyFreeBudget ||
		savedProfile.AdviceStrictness != profile.AdviceStrictness {
		t.Fatalf("saved profile = %#v", savedProfile)
	}

	preview := previewRequest(t, api)
	if preview.PreviewID == "" || preview.ReadCount != 6 {
		t.Fatalf("preview = %#v", preview)
	}
	response = jsonRequest(t, api, http.MethodPost, "/transactions/import/commit", map[string]any{
		"preview_id": preview.PreviewID,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("commit status=%d body=%s", response.Code, response.Body.String())
	}
	var commit domain.ImportCommitResult
	decodeResponse(t, response, &commit)
	if commit.ImportedCount != 6 {
		t.Fatalf("commit = %#v", commit)
	}

	response = jsonRequest(t, api, http.MethodGet, "/dashboard/monthly?month=2026-07", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d body=%s", response.Code, response.Body.String())
	}
	var dashboard domain.Dashboard
	decodeResponse(t, response, &dashboard)
	if dashboard.ExpenseTotal != 18090 {
		t.Fatalf("dashboard = %#v", dashboard)
	}

	amount := int64(6000)
	response = jsonRequest(t, api, http.MethodPost, "/consultations", map[string]any{
		"message":        "今から飲みに行っていい？",
		"planned_amount": amount,
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("consult status=%d body=%s", response.Code, response.Body.String())
	}
	var consultation domain.Consultation
	decodeResponse(t, response, &consultation)
	if consultation.ID == 0 || consultation.AIRecommendation == "" {
		t.Fatalf("consultation = %#v", consultation)
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := sqlite.Open(
		context.Background(),
		filepath.Join(t.TempDir(), "handler.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	now := time.Date(
		2026,
		7,
		31,
		12,
		0,
		0,
		0,
		time.FixedZone("JST", 9*60*60),
	)
	fallback := agent.MockClient{}
	agents := agentadapter.New(fallback, fallback, "mock")
	return New(application.New(application.Config{
		Repository: repo, ConsultationAgent: agents, ReviewAgent: agents, MemoryAgent: agents,
		Now: func() time.Time { return now },
	}))
}

func previewRequest(t *testing.T, api http.Handler) domain.ImportPreview {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "sample_data", "utf8_general.csv"))
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("source", "test_card")
	_ = writer.WriteField("source_account_name", "main")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/transactions/import/preview",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", response.Code, response.Body.String())
	}
	var preview domain.ImportPreview
	decodeResponse(t, response, &preview)
	return preview
}

func jsonRequest(
	t *testing.T,
	api http.Handler,
	method string,
	path string,
	value any,
) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if value != nil {
		if err := json.NewEncoder(&body).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, &body)
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
