package handler

import (
	"context"
	"net/http"
)

type WithdrawalRequestStore interface {
	WithdrawalRequested(context.Context) (bool, error)
	RequestWithdrawal(context.Context) error
}

func (h *handler) getWithdrawalRequest(w http.ResponseWriter, r *http.Request) {
	if h.withdrawalRequests == nil {
		writeError(w, 503, "unavailable", "申し込み状況を確認できません。")
		return
	}
	requested, err := h.withdrawalRequests.WithdrawalRequested(r.Context())
	if err != nil {
		writeError(w, 503, "unavailable", "申し込み状況を確認できません。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"requested": requested})
}
func (h *handler) putWithdrawalRequest(w http.ResponseWriter, r *http.Request) {
	if h.withdrawalRequests == nil {
		writeError(w, 503, "unavailable", "申し込み結果を確認できません。")
		return
	}
	if err := h.withdrawalRequests.RequestWithdrawal(r.Context()); err != nil {
		writeError(w, 503, "unavailable", "申し込み結果を確認できません。状況を確認してください。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"requested": true})
}
