package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type allowConsent struct{}

func (allowConsent) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type recordingConsent struct{ callbackErr error }

type cleanupFailureConsent struct{ err error }

type cancelAfterResponseConsent struct {
	cancel context.CancelFunc
	err    error
}

func (s cancelAfterResponseConsent) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	s.cancel()
	return s.err
}

func (s cleanupFailureConsent) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return s.err
}

func (s *recordingConsent) Execute(ctx context.Context, fn func(context.Context) error) error {
	s.callbackErr = fn(ctx)
	return s.callbackErr
}

func TestConsentMarkerCompletionRequiresBoundedCompleteResponse(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		unknown bool
		success bool
	}{
		{"unauthorized", 401, "{}", false, false},
		{"forbidden", 403, "{}", false, false},
		{"rate limited", 429, "{}", false, false},
		{"upstream error", 500, "{}", true, false},
		{"oversized", 200, strings.Repeat("x", (2<<20)+1), true, false},
		{"accepted", 202, `{"status":"completed"}`, true, false},
		{"queued", 200, `{"status":"queued"}`, true, false},
		{"in progress", 200, `{"status":"in_progress"}`, true, false},
		{"terminal failed", 200, `{"status":"failed"}`, false, false},
		{"terminal incomplete", 200, `{"status":"incomplete"}`, false, false},
		{"terminal cancelled", 200, `{"status":"cancelled"}`, false, false},
		{"missing status", 200, `{}`, true, false},
		{"invalid JSON", 200, `{`, true, false},
		{"completed", 200, `{"status":"completed","output":[{"content":[{"type":"output_text","text":"{}"}]}]}`, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewOpenAIClient("key", "model", "medium")
			gate := &recordingConsent{}
			client.SetConsentGate(gate)
			client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			err := client.structured(context.Background(), "test", map[string]string{}, "test", map[string]any{}, &map[string]any{})
			if (err == nil) != tc.success {
				t.Fatalf("success=%v: %v", tc.success, err)
			}
			if errors.Is(err, consultation.ErrAIOutcomeUnknown) != tc.unknown {
				t.Fatalf("outcome: %v", err)
			}
			if (gate.callbackErr != nil) != tc.unknown {
				t.Fatalf("marker completion: %v", gate.callbackErr)
			}
		})
	}
}

type consentStore struct {
	owner   int64
	granted bool
	fail    error
}

func (s *consentStore) WithConsent(ctx context.Context, id int64, version string, fn func(context.Context) error) error {
	if s.fail != nil {
		return s.fail
	}
	if id != s.owner || !s.granted || version != "v1" {
		return identity.ErrConsentRequired
	}
	return fn(ctx)
}
func (*consentStore) GrantConsent(context.Context, int64, string) error { return nil }
func (*consentStore) WithdrawConsent(context.Context, int64) error      { return nil }

func TestStructuredFailsClosedBeforeExternalRequest(t *testing.T) {
	client := NewOpenAIClient("key", "model", "medium")
	calls := 0
	client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{}"}]}]}`)), Header: make(http.Header)}, nil
	})}
	store := &consentStore{owner: 1}
	call := func(ctx context.Context) error {
		return client.structured(ctx, "test", map[string]string{}, "test", map[string]any{}, &map[string]any{})
	}
	if err := call(identity.WithUser(context.Background(), 1)); !errors.Is(err, consultation.ErrConsentRequired) {
		t.Fatalf("nil gate: %v", err)
	}
	client.SetConsentGate(identity.AIConsentGate{Store: store, Version: "v1"})
	if err := call(context.Background()); !errors.Is(err, identity.ErrInvalidUser) {
		t.Fatalf("anonymous: %v", err)
	}
	if err := call(identity.WithUser(context.Background(), 1)); !errors.Is(err, identity.ErrConsentRequired) {
		t.Fatalf("not granted: %v", err)
	}
	store.granted = true
	if err := call(identity.WithUser(context.Background(), 2)); !errors.Is(err, identity.ErrConsentRequired) {
		t.Fatalf("other owner: %v", err)
	}
	store.fail = errors.New("db unavailable")
	if err := call(identity.WithUser(context.Background(), 1)); err == nil {
		t.Fatal("db failure accepted")
	}
	if calls != 0 {
		t.Fatalf("unauthorized sends: %d", calls)
	}
	store.fail = nil
	if err := call(identity.WithUser(context.Background(), 1)); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("authorized sends: %d", calls)
	}
}

func TestParseResponseOutput(t *testing.T) {
	response := []byte(`{
		"output": [{
			"content": [{
				"type": "output_text",
				"text": "{\"recommendation\":\"見送る\"}"
			}]
		}]
	}`)
	got, err := parseResponseOutput(response)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"recommendation":"見送る"}` {
		t.Fatalf("output = %q", got)
	}
}

func TestParseResponseOutputRejectsMissingStructuredText(t *testing.T) {
	if _, err := parseResponseOutput([]byte(`{"status":"completed","output":[]}`)); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseResponseUsage(t *testing.T) {
	response := []byte(`{
		"output":[{"content":[{"type":"output_text","text":"{}"}]}],
		"usage":{
			"input_tokens":1200,
			"input_tokens_details":{"cached_tokens":300},
			"output_tokens":240,
			"output_tokens_details":{"reasoning_tokens":80},
			"total_tokens":1440
		}
	}`)
	usage, err := parseResponseUsage(response)
	if err != nil {
		t.Fatal(err)
	}
	if usage.InputTokens != 1200 || usage.CachedInputTokens != 300 ||
		usage.OutputTokens != 240 || usage.ReasoningTokens != 80 || usage.TotalTokens != 1440 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestStructuredRequestDisablesStorage(t *testing.T) {
	var requestBody map[string]any
	client := NewOpenAIClient("test-key", "test-model", "medium")
	client.SetConsentGate(allowConsent{})
	client.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"value\":\"ok\"}"}]}]}`,
			)),
			Header: make(http.Header),
		}, nil
	})}
	var output struct {
		Value string `json:"value"`
	}
	err := client.structured(
		context.Background(),
		"test",
		map[string]string{"safe": "summary only"},
		"test_output",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]string{"type": "string"},
			},
			"required":             []string{"value"},
			"additionalProperties": false,
		},
		&output,
	)
	if err != nil {
		t.Fatal(err)
	}
	if requestBody["store"] != false {
		t.Fatalf("store = %#v", requestBody["store"])
	}
	reasoning, ok := requestBody["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "medium" {
		t.Fatalf("reasoning = %#v", requestBody["reasoning"])
	}
	text, ok := requestBody["text"].(map[string]any)
	if !ok {
		t.Fatalf("text = %#v", requestBody["text"])
	}
	format, ok := text["format"].(map[string]any)
	if !ok || format["type"] != "json_schema" || format["strict"] != true {
		t.Fatalf("format = %#v", text["format"])
	}
}

func TestStructuredResponsePersistsUsageWithoutContent(t *testing.T) {
	var records []UsageRecord
	recorder := UsageRecorder(func(_ context.Context, usage UsageRecord) error {
		records = append(records, usage)
		return nil
	})
	client := NewOpenAIClient("test-key", "gpt-5.6-terra", "medium")
	client.SetConsentGate(allowConsent{})
	client.SetUsageRecorder(recorder)
	client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"status":"completed",
				"output":[{"content":[{"type":"output_text","text":"{\"value\":\"secret answer\"}"}]}],
				"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":25},"output_tokens":40,"output_tokens_details":{"reasoning_tokens":10},"total_tokens":140}
			}`)),
			Header: make(http.Header),
		}, nil
	})}
	var output struct {
		Value string `json:"value"`
	}
	err := client.structured(
		context.Background(), "instruction", map[string]string{"private": "input"},
		"spending_advice", map[string]any{"type": "object"}, &output,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("usage records = %#v", records)
	}
	record := records[0]
	if record.Operation != "spending_advice" || record.Model != "gpt-5.6-terra" ||
		record.InputTokens != 100 || record.CachedInputTokens != 25 || record.OutputTokens != 40 ||
		record.ReasoningTokens != 10 || record.TotalTokens != 140 {
		t.Fatalf("usage record = %#v", record)
	}
}

func TestTerminalErrorResponsesRecordUsageOnceAndRemainErrors(t *testing.T) {
	for _, status := range []string{"failed", "cancelled", "incomplete"} {
		t.Run(status, func(t *testing.T) {
			client := NewOpenAIClient("test-key", "model", "medium")
			client.SetConsentGate(allowConsent{})
			var records []UsageRecord
			client.SetUsageRecorder(func(_ context.Context, usage UsageRecord) error { records = append(records, usage); return nil })
			client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				body := fmt.Sprintf(`{"status":%q,"output":[],"usage":{"input_tokens":100,"output_tokens":40,"total_tokens":140}}`, status)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			out := map[string]any{}
			err := client.structured(context.Background(), "test", map[string]any{}, "test", map[string]any{}, &out)
			if err == nil || errors.Is(err, consultation.ErrAIOutcomeUnknown) {
				t.Fatalf("terminal response must be an error: %v", err)
			}
			if len(records) != 1 || records[0].TotalTokens != 140 || records[0].InputTokens != 100 || records[0].OutputTokens != 40 {
				t.Fatalf("usage: %+v", records)
			}
		})
	}
}

func TestCompletedResponseRecordsUsageWhenCompletionMarkerCannotBeCleared(t *testing.T) {
	client := NewOpenAIClient("test-key", "model", "medium")
	cleanupErr := errors.New("marker cleanup failed")
	client.SetConsentGate(cleanupFailureConsent{err: cleanupErr})
	var records []UsageRecord
	client.SetUsageRecorder(func(_ context.Context, usage UsageRecord) error { records = append(records, usage); return nil })
	client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{}"}]}],"usage":{"input_tokens":9,"output_tokens":3,"total_tokens":12}}`)), Header: make(http.Header)}, nil
	})}
	err := client.structured(context.Background(), "test", map[string]any{}, "test", map[string]any{}, &map[string]any{})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup error: %v", err)
	}
	if len(records) != 1 || records[0].TotalTokens != 12 {
		t.Fatalf("usage: %+v", records)
	}
}

func TestUsageRecordedAfterCallerCancellationAndMarkerFailure(t *testing.T) {
	base, cancel := context.WithCancel(identity.WithUser(context.Background(), 701))
	defer cancel()
	cleanupErr := fmt.Errorf("%w: completion marker failure", consultation.ErrAIOutcomeUnknown)
	client := NewOpenAIClient("test-key", "model", "medium")
	client.SetConsentGate(cancelAfterResponseConsent{cancel: cancel, err: cleanupErr})
	client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{}"}]}],"usage":{"input_tokens":9,"output_tokens":3,"total_tokens":12}}`)), Header: make(http.Header)}, nil
	})}
	var calls int
	client.SetUsageRecorder(func(ctx context.Context, usage UsageRecord) error {
		calls++
		if err := ctx.Err(); err != nil {
			t.Errorf("usage context cancelled: %v", err)
		}
		if identity.UserID(ctx) != 701 {
			t.Errorf("usage owner: %d", identity.UserID(ctx))
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("usage recording lacks deadline")
		}
		if usage.TotalTokens != 12 {
			t.Errorf("usage: %+v", usage)
		}
		return ctx.Err()
	})
	err := client.structured(base, "test", map[string]any{}, "test", map[string]any{}, &map[string]any{})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("usage recording calls=%d", calls)
	}
}

func TestUsageWriteFailureIsLoggedWithoutMaskingMarkerFailure(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	cleanupErr := errors.New("marker failure")
	client := NewOpenAIClient("test-key", "model", "medium")
	client.SetConsentGate(cleanupFailureConsent{err: cleanupErr})
	client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[],"usage":{"input_tokens":9,"output_tokens":3,"total_tokens":12}}`)), Header: make(http.Header)}, nil
	})}
	writes := 0
	client.SetUsageRecorder(func(context.Context, UsageRecord) error { writes++; return errors.New("storage unavailable") })
	err := client.structured(context.Background(), "test", map[string]any{}, "test", map[string]any{}, &map[string]any{})
	if !errors.Is(err, cleanupErr) || writes != 1 {
		t.Fatalf("err=%v writes=%d", err, writes)
	}
	if !strings.Contains(logs.String(), "OpenAI usage persistence failed") {
		t.Fatalf("missing failure log: %q", logs.String())
	}
}

func TestSanitizedTransactionsExcludeMerchantAndRawText(t *testing.T) {
	items := sanitizedTransactions([]agent.Transaction{{
		TransactionDate: "2026-07-31",
		Amount:          6000,
		Category:        "酒・飲み会",
	}})
	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, sensitive := range []string{
		"生の明細本文",
		"secret merchant",
		"main account",
	} {
		if strings.Contains(text, sensitive) {
			t.Fatalf("request contains sensitive text %q: %s", sensitive, text)
		}
	}
}

func TestSummarizedConsultationsExcludeFreeFormText(t *testing.T) {
	items := summarizedConsultations([]agent.ConsultationSummary{{
		Category: "買い物", Status: "spent",
	}})
	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"加盟店名", "回答本文", "自由記述"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("payload contains %q: %s", forbidden, encoded)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestChatAdviceUsesFreeResponseSchemaAndNoProfileVerdict(t *testing.T) {
	client := NewOpenAIClient("test-key", "test-model", "medium")
	client.SetConsentGate(allowConsent{})
	client.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["store"] != false {
			t.Fatal("store must be false")
		}
		input := payload["input"].(string)
		if strings.Contains(input, "verdict") || strings.Contains(input, "dashboard") || strings.Contains(input, "profile") {
			t.Fatalf("legacy financial judgement entered context: %s", input)
		}
		schema := payload["text"].(map[string]any)["format"].(map[string]any)["schema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		purchase := props["purchase_change"].(map[string]any)
		if len(purchase["required"].([]any)) != 4 {
			t.Fatal("purchase change schema is incomplete")
		}
		outcome := props["purchase_updates"].(map[string]any)["items"].(map[string]any)
		if len(outcome["required"].([]any)) != 6 {
			t.Fatal("purchase outcome must include nullable purchase date and explicit amount provenance")
		}
		if _, ok := props["planned_amount"]; ok {
			t.Fatal("redundant planned amount model output")
		}
		if _, ok := props["recommendation"]; ok {
			t.Fatal("fixed recommendation contract must not be used")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"status\":\"provisional\",\"answer\":\"概算で考えてみます\",\"reason\":\"収入が不明\",\"assumptions\":\"今月の情報と仮定\",\"question\":\"\",\"purchase_change\":null,\"changes\":[],\"strictness\":null}"}]}]}`))}, nil
	})}
	result, err := client.Advise(context.Background(), consultation.Input{Message: "買い物の相談", Strictness: 5})
	if err != nil || result.Status != "provisional" || result.PlannedAmount != nil {
		t.Fatalf("chat response: %#v %v", result, err)
	}
}

func TestReviewPayloadNeverUsesLocalLabels(t *testing.T) {
	names := []string{"酒・飲み会", "秘密（高後悔）", "通常の加盟店"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			transactions := []domain.Transaction{
				{TransactionDate: "2026-06-01", TransactionType: "expense", NormalizedMerchant: name, Amount: 9000, Category: "食費"},
				{TransactionDate: "2026-07-01", TransactionType: "expense", NormalizedMerchant: name, Amount: 9000, Category: "食費"},
				{TransactionDate: "2026-07-02", TransactionType: "expense", NormalizedMerchant: name + "最大額", Amount: 15000, Category: "食費"},
			}
			local := application.BuildReviewCandidates("2026-07", time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), domain.Profile{ReduceCategories: []string{"食費"}}, domain.Dashboard{}, transactions, nil)
			hasCategory, hasMerchant, hasLargest := false, false, false
			input := agent.ReviewInput{}
			for _, item := range local {
				if item.Label == "食費" {
					hasCategory = true
				}
				if item.Label == name {
					hasMerchant = true
				}
				if item.Label == name+"最大額" {
					hasLargest = true
				}
				input.LocalCandidates = append(input.LocalCandidates, agent.ReviewCandidate{Label: item.Label, Judgement: item.Judgement, TotalAmount: item.TotalAmount, Count: item.Count, Reason: item.Reason, AnnualizedAmount: item.AnnualizedAmount, Question: item.Question})
			}
			if !hasCategory || !hasMerchant || !hasLargest {
				t.Fatalf("missing category or merchant candidates: %#v", local)
			}
			var payload map[string]any
			c := NewOpenAIClient("test", "model", "medium")
			c.SetConsentGate(allowConsent{})
			c.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(body["input"].(string)), &payload); err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"candidates\":[]}"}]}]}`))}, nil
			})}
			if _, err := c.GenerateReview(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			candidates := payload["local_candidates"].([]any)
			if len(candidates) != len(local) {
				t.Fatalf("candidates: %#v", candidates)
			}
			for i, candidate := range candidates {
				got := candidate.(map[string]any)["label"]
				if got != fmt.Sprintf("見直し候補%d", i+1) {
					t.Fatalf("unsafe label: %v", got)
				}
			}
			if input.LocalCandidates[0].Label != local[0].Label {
				t.Fatal("mutated local input")
			}
		})
	}
}
