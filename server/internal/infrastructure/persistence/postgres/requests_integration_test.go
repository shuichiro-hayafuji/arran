package postgres

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"testing"
	"time"
)

func verifyRequestReceipts(t *testing.T, r *Repository, owner, other context.Context) {
	t.Helper()
	month := "2030-01"
	before, _ := r.MonthlyConsultationUsage(owner, month)
	input := consultation.RequestReceipt{ID: "postgres-request-001", PayloadHash: "same", Month: month, NewConsultation: true, ExpiresAt: time.Now().Add(time.Second)}
	receipt, claimed, err := r.ClaimChatRequest(owner, input)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	_, claimed, err = r.ClaimChatRequest(owner, input)
	if err != nil || claimed {
		t.Fatal("duplicate claim")
	}
	if _, err = r.GetChatRequest(other, input.ID); !errors.Is(err, consultation.ErrNotFound) {
		t.Fatal("receipt owner leak")
	}
	otherPayload := input
	otherPayload.PayloadHash = "different"
	if _, _, err = r.ClaimChatRequest(owner, otherPayload); !errors.Is(err, consultation.ErrRequestPayload) {
		t.Fatal("changed payload accepted")
	}
	state, _ := r.LoadChatState(owner)
	thread := consultation.Thread{CreatedAt: time.Now().UTC().Format(time.RFC3339), Messages: []consultation.Message{{Role: "user", Content: "atomic result"}}, Reply: consultation.Reply{Status: "answered", Answer: "result"}}
	saved, err := r.SaveChatTurn(consultation.WithRequestReference(owner, consultation.RequestReference{ID: receipt.ID, Deadline: receipt.ExpiresAt}), state, thread)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := r.GetChatRequest(owner, receipt.ID)
	if err != nil || stored.Result == nil || stored.Result.ID != saved.ID || stored.Status != "completed" {
		t.Fatal("atomic receipt missing")
	}
	if err = r.FailChatRequest(owner, receipt.ID, "read_failure"); err != nil {
		t.Fatal(err)
	}
	usage, _ := r.MonthlyConsultationUsage(owner, month)
	if usage.Count != before.Count+1 {
		t.Fatal("completed quota released")
	}
	state, _ = r.LoadChatState(owner)
	if _, err = r.SaveChatTurn(consultation.WithRequestReference(owner, consultation.RequestReference{ID: receipt.ID, Deadline: receipt.ExpiresAt}), state, saved); !errors.Is(err, consultation.ErrRequestFinished) {
		t.Fatalf("duplicate writer: %v", err)
	}
	input.ID = "postgres-cancel-001"
	input.NewConsultation = false
	_, _, err = r.ClaimChatRequest(owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CancelChatRequest(owner, input.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.SaveChatTurn(consultation.WithRequestReference(owner, consultation.RequestReference{ID: input.ID, Deadline: input.ExpiresAt}), state, thread); !errors.Is(err, consultation.ErrRequestCancelled) {
		t.Fatalf("cancelled financial save: %v", err)
	}
	if err = r.FailChatRequest(owner, input.ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	input.ID = "postgres-expired-001"
	input.NewConsultation = true
	input.ExpiresAt = time.Now().Add(-time.Second)
	_, _, err = r.ClaimChatRequest(owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.SaveChatTurn(consultation.WithRequestReference(owner, consultation.RequestReference{ID: input.ID, Deadline: input.ExpiresAt}), state, thread); !errors.Is(err, consultation.ErrRequestFinished) {
		t.Fatalf("expired save: %v", err)
	}
	expired, err := r.GetChatRequest(owner, input.ID)
	if err != nil || expired.Status != "failed" {
		t.Fatal("restart expiry")
	}
	usage, _ = r.MonthlyConsultationUsage(owner, month)
	if usage.Count != before.Count+1 {
		t.Fatal("expiry quota unreleased")
	}
	retired, err := r.CancelChatRequest(owner, "postgres-unknown-001")
	if err != nil || retired.Status != "cancelled" {
		t.Fatal("tombstone")
	}
	// Hold the state row beyond the request deadline: even a caller without a
	// cancellable context must roll back after waiting for the SQL lock.
	state, _ = r.LoadChatState(owner)
	late := consultation.RequestReceipt{ID: "postgres-late-lock", PayloadHash: "late", Month: month, ExpiresAt: time.Now().Add(150 * time.Millisecond)}
	if _, _, err = r.ClaimChatRequest(owner, late); err != nil {
		t.Fatal(err)
	}
	lock, err := r.db.DB.BeginTx(owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	var version int64
	if err = lock.QueryRowContext(owner, chatQuery(owner, "SELECT version FROM chat_states WHERE user_id=:user_id FOR UPDATE")).Scan(&version); err != nil {
		lock.Rollback()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, e := r.SaveChatTurn(consultation.WithRequestReference(owner, consultation.RequestReference{ID: late.ID, Deadline: late.ExpiresAt}), state, thread)
		done <- e
	}()
	time.Sleep(180 * time.Millisecond)
	if err = lock.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, consultation.ErrRequestFinished) {
		t.Fatalf("late SQL commit: %v", err)
	}
	fresh, _ := r.LoadChatState(owner)
	if fresh.Version != state.Version {
		t.Fatal("deadline rollback changed state")
	}
	if _, err = r.GetChatRequest(owner, late.ID); err != nil {
		t.Fatal(err)
	}

}
