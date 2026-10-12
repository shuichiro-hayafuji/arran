package handler

import (
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"net/http"
	"strconv"
)

func (h *handler) expenseCount(w http.ResponseWriter, r *http.Request) {
	out, err := h.application.ExpenseCount(r.Context(), r.URL.Query().Get("month"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *handler) purchaseCSVCandidates(w http.ResponseWriter, r *http.Request) {
	out, err := h.application.PurchaseCSVCandidates(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *handler) resolvePurchaseCSV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, 400, "invalid_id", "明細IDが不正です")
		return
	}
	var d domain.ReconciliationDecision
	if !decodeJSON(w, r, &d) {
		return
	}
	if err = h.application.ResolvePurchaseCSV(r.Context(), id, d); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved", "financial_status": "pending_next_chat"})
}

func (h *handler) distinctPurchaseCSV(w http.ResponseWriter, r *http.Request) {
	var d domain.ReconciliationDecision
	if !decodeJSON(w, r, &d) {
		return
	}
	d.PurchaseID = r.PathValue("id")
	if d.Action != "distinct" {
		writeError(w, 400, "invalid_choice", "distinct action required")
		return
	}
	if err := h.application.ResolvePurchaseCSV(r.Context(), 0, d); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved", "financial_status": "pending_next_chat"})
}
