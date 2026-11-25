package restoreevidence

import "testing"

func TestCheckRestoreEvidenceFailsClosed(t *testing.T) {
	ledger := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	start := Entry{LedgerID: ledger, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: 1, Kind: "send_started", PolicyVersion: "v1"}
	start.Digest = Digest(start)
	end := Entry{LedgerID: ledger, OwnerID: owner, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", RelatedOperationID: start.OperationID, Generation: 2, Kind: "send_completed", PolicyVersion: "v1", Previous: start.Digest}
	end.Digest = Digest(end)
	head := Head{LedgerID: ledger, Generation: 2, Digest: end.Digest}
	if err := CheckRestoreEvidence(head, []Entry{start, end}, []string{owner}); err != nil {
		t.Fatalf("complete send should pass the limited check: %v", err)
	}
	if err := CheckRestoreEvidence(head, []Entry{start, end}, []string{owner, "11111111111111111111111111111111"}); err != nil {
		t.Fatalf("candidate-only owner is not reconciled by this limited check: %v", err)
	}
	cases := map[string]struct {
		head    Head
		entries []Entry
		owners  []string
	}{
		"missing owner":   {head, []Entry{start, end}, nil},
		"other owner":     {head, []Entry{start, end}, []string{"11111111111111111111111111111111"}},
		"invalid owner":   {head, []Entry{start, end}, []string{"invalid"}},
		"duplicate owner": {head, []Entry{start, end}, []string{owner, owner}},
		"missing history": {head, []Entry{start}, []string{owner}},
		"unresolved send": {Head{LedgerID: ledger, Generation: 1, Digest: start.Digest}, []Entry{start}, []string{owner}},
		"empty history":   {Head{LedgerID: ledger}, nil, []string{owner}},
	}
	for _, kind := range []string{"withdrawal_requested", "withdrawal_completed", "consent_granted", "consent_withdrawn"} {
		e := Entry{LedgerID: ledger, OwnerID: owner, OperationID: "cccccccccccccccccccccccccccccccc", Generation: 1, Kind: kind, PolicyVersion: "v1"}
		e.Digest = Digest(e)
		cases[kind] = struct {
			head    Head
			entries []Entry
			owners  []string
		}{Head{LedgerID: ledger, Generation: 1, Digest: e.Digest}, []Entry{e}, []string{owner}}
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := CheckRestoreEvidence(tc.head, tc.entries, tc.owners); err == nil {
				t.Fatal("unsafe evidence was accepted")
			}
		})
	}
}
