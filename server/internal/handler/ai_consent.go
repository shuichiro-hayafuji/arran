package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
)

func (h *handler) getAIConsent(w http.ResponseWriter, r *http.Request) {
	if h.consent == nil {
		writeError(w, http.StatusServiceUnavailable, "consent_unavailable", "同意の説明を現在表示できません。")
		return
	}
	status, err := h.consent.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "consent_unavailable", "同意の説明を現在表示できません。")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) grantAIConsent(w http.ResponseWriter, r *http.Request) {
	if h.consent == nil {
		writeError(w, http.StatusServiceUnavailable, "consent_unavailable", "同意を現在受け付けられません。")
		return
	}
	var request struct {
		Version string `json:"version"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.consent.Grant(r.Context(), request.Version); err != nil {
		if errors.Is(err, consultation.ErrWithdrawalPending) {
			writeError(w, http.StatusConflict, "withdrawal_pending", "退会申し込みの処理中は新しく同意できません。")
		} else if errors.Is(err, consultation.ErrConsentRequired) {
			writeError(w, http.StatusConflict, "consent_not_granted", "説明を読み直してからお試しください。")
		} else {
			writeError(w, http.StatusServiceUnavailable, "consent_unavailable", "同意を保存できたか確認できません。状態を再確認してください。")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "granted"})
}

func (h *handler) withdrawAIConsent(w http.ResponseWriter, r *http.Request) {
	if h.consent == nil {
		writeError(w, http.StatusServiceUnavailable, "consent_unavailable", "撤回を現在受け付けられません。")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.withdrawalTimeout)
	defer cancel()
	if err := h.consent.Withdraw(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "withdrawal_unconfirmed", "撤回を完了できたか確認できません。状態を確認してください。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}
