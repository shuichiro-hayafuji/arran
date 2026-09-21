package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

const maxBodySize = 20 << 20

type handler struct {
	application *application.Application
}

func New(apiApplication *application.Application) http.Handler {
	handler := &handler{application: apiApplication}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /profile", handler.getProfile)
	mux.HandleFunc("PUT /profile", handler.putProfile)
	mux.HandleFunc("POST /transactions/import/preview", handler.previewImport)
	mux.HandleFunc("POST /transactions/import/commit", handler.commitImport)
	mux.HandleFunc("GET /transactions", handler.listTransactions)
	mux.HandleFunc("PATCH /transactions/{id}", handler.updateTransaction)
	mux.HandleFunc("GET /dashboard/monthly", handler.dashboard)
	mux.HandleFunc("POST /consultations", handler.startConsultation)
	mux.HandleFunc("POST /consultations/{id}/messages", handler.continueConsultation)
	mux.HandleFunc("PATCH /consultations/{id}/result", handler.updateConsultationResult)
	mux.HandleFunc("GET /consultations", handler.listConsultations)
	mux.HandleFunc("GET /consultations/{id}", handler.getConsultation)
	mux.HandleFunc("POST /reviews/monthly", handler.createReview)
	mux.HandleFunc("GET /reviews/latest", handler.latestReview)
	mux.HandleFunc("GET /memories", handler.listMemories)
	mux.HandleFunc("POST /memories", handler.createMemory)
	mux.HandleFunc("PATCH /memories/{id}", handler.updateMemory)
	mux.HandleFunc("DELETE /memories/{id}", handler.deleteMemory)
	return recoverMiddleware(logMiddleware(corsMiddleware(mux)))
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) getProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.application.GetProfile(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *handler) putProfile(w http.ResponseWriter, r *http.Request) {
	var profile domain.Profile
	if !decodeJSON(w, r, &profile) {
		return
	}
	profile, err := h.application.PutProfile(r.Context(), profile)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *handler) previewImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_csv", "CSVファイルを読み取れません。")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file_required", "CSVファイルを選択してください。")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBodySize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_csv", "CSVファイルを読み取れません。")
		return
	}
	var mapping domain.CSVMapping
	if mappingJSON := r.FormValue("mapping"); mappingJSON != "" {
		if err := json.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_mapping", "列マッピングが不正です。")
			return
		}
	}
	preview, err := h.application.PreviewImport(
		r.Context(),
		data,
		mapping,
		r.FormValue("source"),
		r.FormValue("source_account_name"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *handler) commitImport(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PreviewID string `json:"preview_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.application.CommitImport(r.Context(), request.PreviewID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) listTransactions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.application.ListTransactions(
		r.Context(),
		r.URL.Query().Get("month"),
		limit,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":      items,
		"categories": domain.Categories,
	})
}

func (h *handler) updateTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var request struct {
		Category         string `json:"category"`
		RememberMerchant bool   `json:"remember_merchant"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	transaction, err := h.application.UpdateTransactionCategory(
		r.Context(),
		id,
		request.Category,
		request.RememberMerchant,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transaction)
}

func (h *handler) dashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := h.application.Dashboard(
		r.Context(),
		r.URL.Query().Get("month"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (h *handler) startConsultation(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Message       string `json:"message"`
		PlannedAmount *int64 `json:"planned_amount"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	consultation, err := h.application.StartConsultation(
		r.Context(),
		request.Message,
		request.PlannedAmount,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, consultation)
}

func (h *handler) continueConsultation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var request struct {
		Message       string `json:"message"`
		PlannedAmount *int64 `json:"planned_amount"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	consultation, err := h.application.ContinueConsultation(
		r.Context(),
		id,
		request.Message,
		request.PlannedAmount,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, consultation)
}

func (h *handler) updateConsultationResult(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var update domain.ConsultationResultUpdate
	if !decodeJSON(w, r, &update) {
		return
	}
	consultation, err := h.application.UpdateConsultationResult(
		r.Context(),
		id,
		update,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, consultation)
}

func (h *handler) listConsultations(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.application.ListConsultations(r.Context(), limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) getConsultation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	consultation, err := h.application.GetConsultation(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, consultation)
}

func (h *handler) createReview(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Month string `json:"month"`
	}
	if !decodeOptionalJSON(w, r, &request) {
		return
	}
	review, err := h.application.CreateMonthlyReview(r.Context(), request.Month)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (h *handler) latestReview(w http.ResponseWriter, r *http.Request) {
	review, err := h.application.LatestReview(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h *handler) listMemories(w http.ResponseWriter, r *http.Request) {
	items, err := h.application.ListMemories(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) createMemory(w http.ResponseWriter, r *http.Request) {
	var item domain.MemoryItem
	if !decodeJSON(w, r, &item) {
		return
	}
	item.ID = 0
	saved, err := h.application.SaveMemory(r.Context(), item)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (h *handler) updateMemory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var item domain.MemoryItem
	if !decodeJSON(w, r, &item) {
		return
	}
	item.ID = id
	saved, err := h.application.SaveMemory(r.Context(), item)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *handler) deleteMemory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.application.DeleteMemory(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", "IDが不正です。")
		return 0, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "JSONリクエストを読み取れません。")
		return false
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "JSONは1つだけ送信してください。")
		return false
	}
	return true
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	return decodeJSON(w, r, target)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case application.IsNotFound(err):
		writeError(w, http.StatusNotFound, "not_found", "対象データがありません。")
	case isUserError(err):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		log.Printf("request failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "処理に失敗しました。再試行してください。")
	}
}

func isUserError(err error) bool {
	text := err.Error()
	keywords := []string{
		"入力", "必須", "不正", "選択", "指定", "CSV", "列", "日付", "金額",
		"プレビュー", "カテゴリ", "メッセージ", "相談内容", "評価",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("response encoding failed: %v", err)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		log.Printf(
			"%s %s status=%d duration=%s",
			r.Method,
			r.URL.Path,
			writer.status,
			time.Since(start).Round(time.Millisecond),
		)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v", recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "処理に失敗しました。")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
