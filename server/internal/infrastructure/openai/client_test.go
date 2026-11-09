package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
)

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
	if _, err := parseResponseOutput([]byte(`{"output":[]}`)); err == nil {
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
	client.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"output":[{"content":[{"type":"output_text","text":"{\"value\":\"ok\"}"}]}]}`,
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
	client.SetUsageRecorder(recorder)
	client.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
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
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"output":[{"content":[{"type":"output_text","text":"{\"status\":\"provisional\",\"answer\":\"概算で考えてみます\",\"reason\":\"収入が不明\",\"assumptions\":\"今月の情報と仮定\",\"question\":\"\",\"purchase_change\":null,\"changes\":[],\"strictness\":null}"}]}]}`))}, nil
	})}
	result, err := client.Advise(context.Background(), consultation.Input{Message: "買い物の相談", Strictness: 5})
	if err != nil || result.Status != "provisional" || result.PlannedAmount != nil {
		t.Fatalf("chat response: %#v %v", result, err)
	}
}
