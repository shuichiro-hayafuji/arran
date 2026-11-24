package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
)

type WithdrawalRequestStore interface {
	WithdrawalRequested(context.Context) (bool, error)
	RequestWithdrawal(context.Context) error
	WithdrawalAcceptanceEnabled() bool
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
	writeJSON(w, http.StatusOK, map[string]bool{"requested": requested, "accepting": h.withdrawalRequests.WithdrawalAcceptanceEnabled()})
}
func (h *handler) putWithdrawalRequest(w http.ResponseWriter, r *http.Request) {
	if h.withdrawalRequests == nil {
		writeError(w, 503, "unavailable", "申し込み結果を確認できません。")
		return
	}
	if err := h.withdrawalRequests.RequestWithdrawal(r.Context()); err != nil {
		if errors.Is(err, consultation.ErrRequestBusy) {
			writeError(w, 409, "withdrawal_blocked", "現在の状態では退会を申し込めません。処理中の相談や再有効化済みの会員についてはお問い合わせください。")
			return
		}
		writeError(w, 503, "unavailable", "申し込み結果を確認できません。状況を確認してください。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"requested": true})
}
