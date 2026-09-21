// Package openai adapts the OpenAI Responses API to the agent model boundary.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
)

type client struct {
	apiKey string
	model  string
	client *http.Client
}

var _ agent.Model = (*client)(nil)

func NewOpenAIClient(apiKey, model string) *client {
	if model == "" {
		model = "gpt-5-mini"
	}
	return &client{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *client) GenerateAdvice(
	ctx context.Context,
	input agent.ConsultationInput,
) (agent.AdviceDraft, error) {
	payload := advicePayload(input)
	var result agent.AdviceDraft
	err := c.structured(ctx, agent.ConsultationPrompt, payload, "spending_advice", adviceSchema(input.Categories), &result)
	return result, err
}

func (c *client) GenerateAdviceRevision(
	ctx context.Context,
	input agent.AdviceRevisionInput,
) (agent.AdviceDraft, error) {
	payload := advicePayload(input.Input)
	payload["previous_draft"] = input.Previous
	payload["validation_issues"] = input.Issues
	var result agent.AdviceDraft
	err := c.structured(ctx, agent.ConsultationRevisionPrompt, payload, "spending_advice_revision", adviceSchema(input.Input.Categories), &result)
	return result, err
}

func (c *client) InferCategory(
	_ context.Context,
	_ string,
	_ []string,
) (string, error) {
	return "", fmt.Errorf(
		"OpenAI category inference is disabled to avoid sending merchant text",
	)
}

func (c *client) GenerateReview(
	ctx context.Context,
	input agent.ReviewInput,
) ([]agent.ReviewCandidate, error) {
	var result struct {
		Candidates []agent.ReviewCandidate `json:"candidates"`
	}
	payload := map[string]any{
		"profile": input.Profile, "dashboard": input.Dashboard,
		"transactions":     sanitizedTransactions(input.Transactions),
		"consultations":    summarizedConsultations(input.Consultations),
		"local_candidates": input.LocalCandidates,
	}
	err := c.structured(
		ctx,
		"ローカル検出候補を最大5件に整理し、断定しすぎない日本語で見直し理由と確認質問を返してください。",
		payload,
		"monthly_review",
		reviewSchema(),
		&result,
	)
	return result.Candidates, err
}

func (c *client) ExtractMemoryCandidates(
	ctx context.Context,
	consultation agent.MemoryInput,
) ([]agent.MemoryItem, error) {
	var result struct {
		Items []agent.MemoryItem `json:"items"`
	}
	err := c.structured(
		ctx,
		"相談結果から、今後も再利用でき、本人の発言または行動に根拠がある記憶候補だけを最大2件返してください。単発の事実は返さないでください。",
		consultation,
		"memory_candidates",
		memorySchema(),
		&result,
	)
	return result.Items, err
}

// structured は指定スキーマのJSONを要求し、応答の保存を無効にして送信する。
// 通信・JSON解析エラーを上位へ返し、再生成やフォールバックは Agent に委ねる。
func (c *client) structured(
	ctx context.Context,
	system string,
	input any,
	name string,
	schema map[string]any,
	target any,
) error {
	if strings.TrimSpace(c.apiKey) == "" {
		return fmt.Errorf("OPENAI_API_KEY is not configured")
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return err
	}
	body := map[string]any{
		"model":        c.model,
		"instructions": system,
		"input":        string(inputJSON),
		"store":        false,
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   name,
				"strict": true,
				"schema": schema,
			},
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/responses",
		bytes.NewReader(encoded),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("OpenAI returned HTTP %d", response.StatusCode)
	}
	output, err := parseResponseOutput(responseBody)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return fmt.Errorf("OpenAI structured output is invalid: %w", err)
	}
	return nil
}

func parseResponseOutput(data []byte) (string, error) {
	var response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("OpenAI response JSON is invalid: %w", err)
	}
	for _, output := range response.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" && content.Text != "" {
				return content.Text, nil
			}
		}
	}
	return "", fmt.Errorf("OpenAI response did not contain output_text")
}

func sanitizedTransactions(items []agent.Transaction) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"transaction_date": item.TransactionDate,
			"amount":           item.Amount,
			"category":         item.Category,
		})
	}
	return result
}

func advicePayload(input agent.ConsultationInput) map[string]any {
	return map[string]any{
		"current_time_jst":      input.Now.Format(time.RFC3339),
		"profile":               input.Profile,
		"monthly_summary":       input.Dashboard,
		"recent_consultations":  summarizedConsultations(input.RecentConsultations),
		"memory_items":          input.Memories,
		"relevant_transactions": sanitizedTransactions(input.RelevantTransactions),
		"decision_facts":        input.AdviceFacts,
		"user_message":          input.Message,
		"planned_amount":        input.PlannedAmount,
	}
}

// summarizedConsultations は過去の判断と結果を構造化して渡す。
// 過去の相談文や回答本文は送信項目に含めない。
func summarizedConsultations(items []agent.ConsultationSummary) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"created_at": item.CreatedAt, "planned_amount": item.PlannedAmount,
			"category": item.Category, "status": item.Status,
			"actual_amount": item.ActualAmount, "satisfaction_score": item.SatisfactionScore,
			"regret_score": item.RegretScore,
		})
	}
	return result
}

func adviceSchema(categories []string) map[string]any {
	properties := map[string]any{
		"recommendation":     map[string]any{"type": "string"},
		"verdict":            map[string]any{"type": "string", "enum": []string{"safe", "caution", "avoid", "insufficient_data"}},
		"reasoning_summary":  map[string]any{"type": "string"},
		"current_situation":  map[string]any{"type": "string"},
		"alternative":        map[string]any{"type": "string"},
		"final_question":     map[string]any{"type": "string"},
		"inferred_category":  map[string]any{"type": "string", "enum": categories},
		"needs_follow_up":    map[string]any{"type": "boolean"},
		"follow_up_question": map[string]any{"type": "string"},
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             keys(properties),
		"additionalProperties": false,
	}
}

func reviewSchema() map[string]any {
	itemProperties := map[string]any{
		"label":             map[string]any{"type": "string"},
		"judgement":         map[string]any{"type": "string"},
		"total_amount":      map[string]any{"type": "integer"},
		"count":             map[string]any{"type": "integer"},
		"reason":            map[string]any{"type": "string"},
		"annualized_amount": map[string]any{"type": "integer"},
		"question":          map[string]any{"type": "string"},
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"candidates": map[string]any{
				"type":     "array",
				"maxItems": 5,
				"items": map[string]any{
					"type":                 "object",
					"properties":           itemProperties,
					"required":             keys(itemProperties),
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"candidates"},
		"additionalProperties": false,
	}
}

func memorySchema() map[string]any {
	itemProperties := map[string]any{
		"id":         map[string]any{"type": "integer"},
		"type":       map[string]any{"type": "string"},
		"content":    map[string]any{"type": "string"},
		"evidence":   map[string]any{"type": "string"},
		"confidence": map[string]any{"type": "number"},
		"created_at": map[string]any{"type": "string"},
		"updated_at": map[string]any{"type": "string"},
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type":     "array",
				"maxItems": 2,
				"items": map[string]any{
					"type":                 "object",
					"properties":           itemProperties,
					"required":             keys(itemProperties),
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"items"},
		"additionalProperties": false,
	}
}

func keys(values map[string]any) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}
