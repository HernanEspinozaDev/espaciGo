package documentverification

import (
	"strings"
	"testing"
)

func TestOwnerDecisionCopy(t *testing.T) {
	// 1. Non-identity purpose returns empty string regardless of state or reason
	for _, purpose := range []Purpose{PurposeSpace, Purpose("unknown"), Purpose("")} {
		if got := OwnerDecisionCopy(purpose, CaseNeedsEvidence, ReasonEvidenceIncomplete); got != "" {
			t.Errorf("OwnerDecisionCopy(%q, CaseNeedsEvidence, ReasonEvidenceIncomplete) = %q, want empty", purpose, got)
		}
	}

	// 2. Identity purpose with states other than CaseNeedsEvidence returns empty string
	otherStates := []CaseState{
		CaseDraft,
		CaseSubmitted,
		CaseInReview,
		CaseApproved,
		CaseRejected,
		CaseRevoked,
		CaseExpired,
		CaseState("custom_state"),
	}
	for _, st := range otherStates {
		if got := OwnerDecisionCopy(PurposeIdentity, st, ReasonEvidenceIncomplete); got != "" {
			t.Errorf("OwnerDecisionCopy(PurposeIdentity, %q, ReasonEvidenceIncomplete) = %q, want empty", st, got)
		}
	}

	// 3. Identity purpose with CaseNeedsEvidence and empty reason returns empty string
	if got := OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, ""); got != "" {
		t.Errorf("OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, \"\") = %q, want empty", got)
	}

	// 4. Identity purpose with CaseNeedsEvidence and ReasonEvidenceIncomplete returns approved specific copy
	gotIncomplete := OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, ReasonEvidenceIncomplete)
	if gotIncomplete != ownerNeedsEvidenceCopy {
		t.Errorf("OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, ReasonEvidenceIncomplete) = %q, want %q", gotIncomplete, ownerNeedsEvidenceCopy)
	}

	// 5. Identity purpose with CaseNeedsEvidence and other reasons returns approved fallback copy
	otherReasons := []ReasonCode{
		ReasonRetentionDue,
		ReasonDocumentInvalid,
		ReasonCode("antecedentes_ilegibiles"),
		ReasonCode("unknown_code"),
	}
	for _, r := range otherReasons {
		gotFallback := OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, r)
		if gotFallback != ownerNeedsEvidenceFallback {
			t.Errorf("OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, %q) = %q, want %q", r, gotFallback, ownerNeedsEvidenceFallback)
		}
	}

	// 6. Guarantee that internal reason codes or technical identifiers never leak into the output copy
	for _, r := range append(otherReasons, ReasonEvidenceIncomplete) {
		copyText := OwnerDecisionCopy(PurposeIdentity, CaseNeedsEvidence, r)
		if strings.Contains(copyText, string(r)) {
			t.Errorf("OwnerDecisionCopy leaked internal reason code %q in copy: %q", r, copyText)
		}
	}
}
