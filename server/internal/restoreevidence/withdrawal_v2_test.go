package restoreevidence

import "testing"

func TestWithdrawalV2ValidationAndV1Compatibility(t *testing.T) {
	ledger := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	request := WithdrawalV2{Version: 2, LedgerID: ledger, Generation: 1, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "withdrawal_requested", RequestedAt: "2026-11-24T15:00:00Z"}
	request.Digest = WithdrawalV2Digest(request)
	if request.Digest != "e6475f433b89e89728c9b5f10167773312ab1347fd8369742e4e5b3599aa109a" {
		t.Fatalf("v2 digest changed: %s", request.Digest)
	}
	completion := WithdrawalV2{Version: 2, LedgerID: ledger, Generation: 2, OwnerID: owner, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", RequestOperationID: request.OperationID, Kind: "withdrawal_completed", RequestedAt: request.RequestedAt, Previous: request.Digest}
	completion.Digest = WithdrawalV2Digest(completion)
	head := Head{LedgerID: ledger, Generation: 2, Digest: completion.Digest}
	if err := ValidateWithdrawalV2(head, []WithdrawalV2{request, completion}); err != nil {
		t.Fatal(err)
	}
	for _, ts := range []string{"", "2026-11-25T00:00:00+09:00", "2026-11-24T15:00:00.000Z"} {
		e := request
		e.RequestedAt = ts
		e.Digest = WithdrawalV2Digest(e)
		if ValidateWithdrawalV2(Head{LedgerID: ledger, Generation: 1, Digest: e.Digest}, []WithdrawalV2{e}) == nil {
			t.Fatalf("invalid request timestamp %q accepted", ts)
		}
	}
	for _, history := range [][]WithdrawalV2{{request}, {request, completion}} {
		last := history[len(history)-1]
		duplicate := request
		duplicate.Generation = last.Generation + 1
		duplicate.OperationID = "cccccccccccccccccccccccccccccccc"
		duplicate.Previous = last.Digest
		duplicate.Digest = WithdrawalV2Digest(duplicate)
		entries := append(append([]WithdrawalV2{}, history...), duplicate)
		if ValidateWithdrawalV2(Head{LedgerID: ledger, Generation: duplicate.Generation, Digest: duplicate.Digest}, entries) == nil {
			t.Fatal("second request for same owner accepted")
		}
	}
	cases := map[string]func(*WithdrawalV2){
		"unknown version":        func(e *WithdrawalV2) { e.Version = 3 },
		"old version":            func(e *WithdrawalV2) { e.Version = 1 },
		"missing time":           func(e *WithdrawalV2) { e.RequestedAt = "" },
		"offset time":            func(e *WithdrawalV2) { e.RequestedAt = "2026-11-25T00:00:00+09:00" },
		"noncanonical time":      func(e *WithdrawalV2) { e.RequestedAt = "2026-11-24T15:00:00.000Z" },
		"missing request id":     func(e *WithdrawalV2) { e.RequestOperationID = "" },
		"wrong request id":       func(e *WithdrawalV2) { e.RequestOperationID = "cccccccccccccccccccccccccccccccc" },
		"wrong owner":            func(e *WithdrawalV2) { e.OwnerID = "11111111111111111111111111111111" },
		"different request time": func(e *WithdrawalV2) { e.RequestedAt = "2026-11-24T15:00:01Z" },
		"unknown event":          func(e *WithdrawalV2) { e.Kind = "other" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := completion
			mutate(&e)
			e.Digest = WithdrawalV2Digest(e)
			h := head
			h.Digest = e.Digest
			if ValidateWithdrawalV2(h, []WithdrawalV2{request, e}) == nil {
				t.Fatal("invalid event accepted")
			}
		})
	}
	altered := completion
	altered.RequestedAt = "2026-11-24T15:00:01Z"
	if ValidateWithdrawalV2(head, []WithdrawalV2{request, altered}) == nil {
		t.Fatal("tampered event accepted")
	}
	v1 := Entry{LedgerID: ledger, Generation: 1, OwnerID: owner, OperationID: request.OperationID, Kind: "withdrawal_requested"}
	v1.Digest = Digest(v1)
	if v1.Digest != "99a9770d8f9b23b90e6cd751b31b3d02516d4983598974fbfcecdd656e58d0dd" {
		t.Fatalf("legacy v1 digest changed: %s", v1.Digest)
	}
	if err := ValidateChain(Head{LedgerID: ledger, Generation: 1, Digest: v1.Digest}, []Entry{v1}); err != nil {
		t.Fatalf("legacy v1 rejected: %v", err)
	}
	if v1.Digest == WithdrawalV2Digest(request) {
		t.Fatal("v1 and v2 digests lack domain separation")
	}
}
