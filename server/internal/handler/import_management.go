package handler

import (
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"net/http"
)

func (h *handler) importHistory(w http.ResponseWriter, r *http.Request) {
	out, err := h.application.ImportHistory(r.Context(), r.URL.Query().Get("before"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *handler) importDetails(w http.ResponseWriter, r *http.Request) {
	out, err := h.application.ImportDetails(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *handler) csvMutation(w http.ResponseWriter, r *http.Request) {
	out, err := h.application.CSVMutation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *handler) changeAudit(w http.ResponseWriter, r *http.Request) {
	out, err := h.application.ChangeAudit(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *handler) mutateImport(w http.ResponseWriter, r *http.Request) {
	var in domain.MutationInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Action = r.PathValue("action")
	out, err := h.application.MutateCSV(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *handler) transactionDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	out, err := h.application.TransactionDetails(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
