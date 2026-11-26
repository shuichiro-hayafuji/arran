package restoreevidence

import (
	"errors"
	"fmt"
)

// CheckRestoreEvidence performs a limited, offline consistency check. The
// supplied head and owner IDs are untrusted inputs; passing this check neither
// establishes their authority nor authorizes applying data or publishing a DB.
// Caller-supplied owner IDs only detect missing identities, not whether the
// candidate database belongs to the correct individuals. Owners present only
// in the candidate list are allowed and are not reconciled by this check.
//
// No current event includes the time of a withdrawal request, so reconstructing
// its midnight deadline is unsafe. Consent replay semantics and the baseline of
// a candidate snapshot are also not yet defined. Histories containing those
// events must remain blocked until those contracts can be verified.
func CheckRestoreEvidence(head Head, entries []Entry, candidateOwners []string) error {
	if err := ValidateChain(head, entries); err != nil {
		return err
	}
	owners := make(map[string]struct{}, len(candidateOwners))
	for _, owner := range candidateOwners {
		if !validID(owner) {
			return errors.New("invalid candidate owner identity")
		}
		if _, exists := owners[owner]; exists {
			return errors.New("duplicate candidate owner identity")
		}
		owners[owner] = struct{}{}
	}
	started := make(map[string]bool)
	for _, event := range entries {
		if _, ok := owners[event.OwnerID]; !ok {
			return fmt.Errorf("candidate owner missing at generation %d", event.Generation)
		}
		switch event.Kind {
		case "send_started":
			started[event.OperationID] = false
		case "send_completed":
			started[event.RelatedOperationID] = true
		case "withdrawal_requested", "withdrawal_completed":
			return errors.New("withdrawal replay lacks verified request time and transition")
		case "consent_granted", "consent_withdrawn":
			return errors.New("consent replay baseline and transitions are not established")
		default:
			return errors.New("unsupported restore event")
		}
	}
	for _, complete := range started {
		if !complete {
			return errors.New("send outcome remains unresolved")
		}
	}
	return nil
}
