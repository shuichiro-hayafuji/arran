// Package openai adapts the OpenAI Responses API to the agent model boundary.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
)

type client struct {
	apiKey          string
	model           string
	reasoningEffort string
	client          *http.Client
	usageRecorder   UsageRecorder
}

type UsageRecorder func(context.Context, UsageRecord) error

// UsageRecord は費用集計に必要なOpenAI固有の利用量だけを通知する。
// 利用者との紐付けと永続化形式への変換はcomposition rootが担う。
type UsageRecord struct {
	Operation         string
	Model             string
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	ReasoningTokens   int
	TotalTokens       int
	OccurredAt        time.Time
}

var _ agent.Model = (*client)(nil)

func NewOpenAIClient(apiKey, model, reasoningEffort string) *client {
	if model == "" {
		model = "gpt-5.6-terra"
	}
	if reasoningEffort == "" {
		reasoningEffort = "medium"
	}
	return &client{
		apiKey:          apiKey,
		model:           model,
		reasoningEffort: reasoningEffort,
		client:          &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *client) SetUsageRecorder(recorder UsageRecorder) {
	c.usageRecorder = recorder
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
		"reasoning": map[string]any{
			"effort": c.reasoningEffort,
		},
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
	output, usage, err := parseResponse(responseBody)
	if usage.TotalTokens > 0 || usage.InputTokens > 0 || usage.OutputTokens > 0 {
		log.Printf(
			"OpenAI usage: operation=%s model=%s input_tokens=%d cached_input_tokens=%d output_tokens=%d reasoning_tokens=%d total_tokens=%d",
			name, c.model, usage.InputTokens, usage.CachedInputTokens,
			usage.OutputTokens, usage.ReasoningTokens, usage.TotalTokens,
		)
		if c.usageRecorder != nil {
			recordErr := c.usageRecorder(ctx, UsageRecord{
				Operation: name, Model: c.model,
				InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens,
				OutputTokens: usage.OutputTokens, ReasoningTokens: usage.ReasoningTokens,
				TotalTokens: usage.TotalTokens, OccurredAt: time.Now().UTC(),
			})
			if recordErr != nil {
				log.Printf("OpenAI usage persistence failed: operation=%s: %v", name, recordErr)
			}
		}
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return fmt.Errorf("OpenAI structured output is invalid: %w", err)
	}
	return nil
}

func parseResponseOutput(data []byte) (string, error) {
	output, _, err := parseResponse(data)
	return output, err
}

// Usage contains token counts returned by the Responses API. OutputTokens
// already includes reasoning tokens, so callers must not add them again.
type Usage struct {
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	ReasoningTokens   int
	TotalTokens       int
}

func parseResponseUsage(data []byte) (Usage, error) {
	_, usage, err := parseResponse(data)
	return usage, err
}

func parseResponse(data []byte) (string, Usage, error) {
	var response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens       int `json:"input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			TotalTokens       int `json:"total_tokens"`
			InputTokenDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			OutputTokenDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", Usage{}, fmt.Errorf("OpenAI response JSON is invalid: %w", err)
	}
	usage := Usage{
		InputTokens:       response.Usage.InputTokens,
		CachedInputTokens: response.Usage.InputTokenDetails.CachedTokens,
		OutputTokens:      response.Usage.OutputTokens,
		ReasoningTokens:   response.Usage.OutputTokenDetails.ReasoningTokens,
		TotalTokens:       response.Usage.TotalTokens,
	}
	for _, output := range response.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" && content.Text != "" {
				return content.Text, usage, nil
			}
		}
	}
	return "", usage, fmt.Errorf("OpenAI response did not contain output_text")
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
