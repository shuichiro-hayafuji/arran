package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shuichiro-hayafuji/arran_agent"
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

func TestStructuredRequestDisablesStorage(t *testing.T) {
	var requestBody map[string]any
	client := NewOpenAIClient("test-key", "test-model")
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
	text, ok := requestBody["text"].(map[string]any)
	if !ok {
		t.Fatalf("text = %#v", requestBody["text"])
	}
	format, ok := text["format"].(map[string]any)
	if !ok || format["type"] != "json_schema" || format["strict"] != true {
		t.Fatalf("format = %#v", text["format"])
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
