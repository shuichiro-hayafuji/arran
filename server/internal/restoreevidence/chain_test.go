package restoreevidence

import "testing"

func TestValidateChainRejectsIncompleteOrAlteredEvidence(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	owner := "fedcba9876543210fedcba9876543210"
	a := Entry{LedgerID: id, Generation: 1, OwnerID: owner, OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PolicyVersion: "v1", Kind: "consent_withdrawn"}
	a.Digest = Digest(a)
	b := Entry{LedgerID: id, Generation: 2, OwnerID: owner, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PolicyVersion: "v1", Kind: "withdrawal_requested", Previous: a.Digest}
	b.Digest = Digest(b)
	head := Head{LedgerID: id, Generation: 2, Digest: b.Digest}
	if err := ValidateChain(head, []Entry{a, b}); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func() (Head, []Entry){
		"missing head":   func() (Head, []Entry) { return Head{}, []Entry{a, b} },
		"old history":    func() (Head, []Entry) { return head, []Entry{a} },
		"wrong identity": func() (Head, []Entry) { x := b; x.OwnerID = "1"; return head, []Entry{a, x} },
		"changed entry":  func() (Head, []Entry) { x := b; x.Kind = "consent_granted"; return head, []Entry{a, x} },
		"unknown event": func() (Head, []Entry) {
			x := b
			x.Kind = "unknown"
			x.Digest = Digest(x)
			h := head
			h.Digest = x.Digest
			return h, []Entry{a, x}
		},
		"wrong head":           func() (Head, []Entry) { h := head; h.Digest = a.Digest; return h, []Entry{a, b} },
		"duplicate generation": func() (Head, []Entry) { return head, []Entry{a, a} },
	}
	for name, makeCase := range cases {
		t.Run(name, func(t *testing.T) {
			h, e := makeCase()
			if ValidateChain(h, e) == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
}
