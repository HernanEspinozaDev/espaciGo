package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const identityCorrectionReplacementID = "99999999-9999-4999-8999-999999999999"

func TestIdentityCorrectionReplacesPendingRoleAndResubmitsAfterSyntheticScan(t *testing.T) {
	service, repository, original := fixture(t, PurposeIdentity)
	observedExpiry := time.Date(2030, 11, 13, 0, 0, 0, 0, time.UTC)
	repository.state = Snapshot{
		ID: original.ID, Purpose: PurposeIdentity, SubjectID: subject,
		State: CaseSubmitted, Revision: 5,
		IdentityDeclarationRevisionID: declarationRevisionID,
		DocumentFormatDeclared:        DocumentFormatNew,
		CaptureConsentVersion:         IdentityCaptureConsentVersion,
		CaptureConsentedAt:            time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
		Files: []File{
			{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
			{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
			{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
		},
	}
	repository.auth.ActorID = actor
	checklist := passedIdentityReviewChecklist()
	checklist.EvidenceLegible = boolPointer(false)
	if err := service.ReviewIdentityWithReason(context.Background(), actor, original.ID, DecisionNeedsEvidence, ReasonEvidenceIncomplete, DocumentFormatNew, observedExpiry, checklist, time.Time{}, repository.state.Revision); err != nil {
		t.Fatalf("request correction: %v", err)
	}
	if repository.state.State != CaseNeedsEvidence || repository.state.ReviewerID != actor || repository.state.Revision != 6 {
		t.Fatalf("correction decision snapshot=%+v", repository.state)
	}
	if repository.audit.Action != "review_decision" || repository.audit.ResultCode != string(CaseNeedsEvidence) ||
		repository.audit.ReasonCode != ReasonEvidenceIncomplete || repository.audit.DocumentFormatObserved != DocumentFormatNew ||
		!repository.audit.DocumentExpiresAtObserved.Equal(observedExpiry) || !reflect.DeepEqual(repository.audit.IdentityReviewChecklist, checklist) {
		t.Fatalf("correction decision audit=%+v", repository.audit)
	}

	repository.auth.ActorID = subject
	replacement := File{
		ID: identityCorrectionReplacementID, EvidenceRole: EvidenceRoleIdentityFront,
		State: FilePending, Size: 120, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{9},
	}
	if err := service.AddFile(context.Background(), subject, original.ID, PurposeIdentity, repository.state.Revision, replacement); err != nil {
		t.Fatalf("replace requested evidence role: %v", err)
	}
	if repository.state.State != CaseNeedsEvidence || repository.state.ReviewerID != actor || repository.state.Revision != 7 || len(repository.state.Files) != 3 {
		t.Fatalf("replacement snapshot=%+v", repository.state)
	}
	if repository.state.Files[0].ID != identityCorrectionReplacementID || repository.state.Files[0].State != FilePending ||
		repository.audit.Action != "file_added" || repository.audit.ResultCode != string(FilePending) {
		t.Fatalf("replacement file/audit=%+v/%+v", repository.state.Files[0], repository.audit)
	}
	for _, file := range repository.state.Files {
		if file.ID == fileID {
			t.Fatal("replaced front-side evidence remains in the current file set")
		}
	}

	beforeRetry := repository.state
	beforeRetry.Files = append([]File(nil), repository.state.Files...)
	beforeAudit := repository.audit
	if err := service.Submit(context.Background(), subject, original.ID, PurposeIdentity, repository.state.Revision); !errors.Is(err, ErrFilesNotClean) {
		t.Fatalf("submit with replacement pending error=%v, want ErrFilesNotClean", err)
	}
	if !reflect.DeepEqual(repository.state, beforeRetry) || !reflect.DeepEqual(repository.audit, beforeAudit) {
		t.Fatal("submit while replacement is pending mutated the case or audit")
	}

	// This direct fake-state update stands in for a clean scanner result; it does not test scanning.
	repository.state.Files[0].State = FileClean
	if err := service.Submit(context.Background(), subject, original.ID, PurposeIdentity, repository.state.Revision); err != nil {
		t.Fatalf("resubmit after synthetic clean scan: %v", err)
	}
	if repository.state.State != CaseSubmitted || repository.state.ReviewerID != "" || repository.state.Revision != 8 ||
		repository.audit.Action != "case_transitioned" || repository.audit.ResultCode != string(CaseSubmitted) {
		t.Fatalf("resubmission snapshot/audit=%+v/%+v", repository.state, repository.audit)
	}
}

func TestIdentityNeedsEvidenceCanRecordUndeterminedObservedFormat(t *testing.T) {
	service, repository, original := fixture(t, PurposeIdentity)
	repository.state = Snapshot{
		ID: original.ID, Purpose: PurposeIdentity, SubjectID: subject,
		State: CaseSubmitted, Revision: 5,
		IdentityDeclarationRevisionID: declarationRevisionID,
		DocumentFormatDeclared:        DocumentFormatUnsure,
		CaptureConsentVersion:         IdentityCaptureConsentVersion,
		CaptureConsentedAt:            time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
		Files: []File{
			{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
			{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
			{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
		},
	}
	repository.auth.ActorID = actor
	checklist := passedIdentityReviewChecklist()
	checklist.EvidenceLegible = boolPointer(false)
	checklist.IssueDateAndFormatVerified = boolPointer(false)
	if err := service.ReviewIdentityWithReason(context.Background(), actor, original.ID, DecisionNeedsEvidence, ReasonEvidenceIncomplete, DocumentFormatUndetermined, time.Time{}, checklist, time.Time{}, repository.state.Revision); err != nil {
		t.Fatalf("request evidence with undetermined observed format: %v", err)
	}
	if repository.state.State != CaseNeedsEvidence || repository.state.ReviewerID != actor || repository.state.Revision != 6 ||
		repository.state.DocumentFormatDeclared != DocumentFormatUnsure || repository.state.DocumentFormatObserved != DocumentFormatUndetermined ||
		!repository.state.DocumentExpiresAt.IsZero() || !repository.state.VerifiedAt.IsZero() || !repository.state.VerifiedUntil.IsZero() {
		t.Fatalf("case after evidence request=%+v", repository.state)
	}
	if repository.audit.Action != "review_decision" || repository.audit.ResultCode != string(CaseNeedsEvidence) ||
		repository.audit.ReasonCode != ReasonEvidenceIncomplete || repository.audit.DocumentFormatObserved != DocumentFormatUndetermined ||
		!repository.audit.DocumentExpiresAtObserved.IsZero() || !reflect.DeepEqual(repository.audit.IdentityReviewChecklist, checklist) {
		t.Fatalf("audit after evidence request=%+v", repository.audit)
	}
}

func TestIdentityNeedsEvidenceRejectsIncompleteChecklistWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name      string
		checklist func() *IdentityReviewChecklist
	}{
		{name: "missing checklist", checklist: func() *IdentityReviewChecklist { return nil }},
		{name: "missing format verification", checklist: func() *IdentityReviewChecklist {
			checklist := passedIdentityReviewChecklist()
			checklist.IssueDateAndFormatVerified = nil
			return checklist
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repository, original := fixture(t, PurposeIdentity)
			repository.state = Snapshot{
				ID: original.ID, Purpose: PurposeIdentity, SubjectID: subject,
				State: CaseSubmitted, Revision: 5,
				IdentityDeclarationRevisionID: declarationRevisionID,
				DocumentFormatDeclared:        DocumentFormatUnsure,
				CaptureConsentVersion:         IdentityCaptureConsentVersion,
				CaptureConsentedAt:            time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
				Files: []File{
					{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
					{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
					{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
				},
			}
			beforeSnapshot := repository.state
			beforeAudit := repository.audit
			err := service.ReviewIdentityWithReason(
				context.Background(), actor, original.ID, DecisionNeedsEvidence, ReasonEvidenceIncomplete,
				DocumentFormatUndetermined, time.Time{}, test.checklist(), time.Time{}, repository.state.Revision,
			)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("review error=%v, want ErrInvalid", err)
			}
			if !reflect.DeepEqual(repository.state, beforeSnapshot) || !reflect.DeepEqual(repository.audit, beforeAudit) {
				t.Fatalf("invalid review mutated snapshot/audit: snapshot=%+v audit=%+v", repository.state, repository.audit)
			}
			if repository.calls != 1 || repository.transitionActor != actor {
				t.Fatalf("transition calls/actor=%d/%q, want one attempted transition by reviewer", repository.calls, repository.transitionActor)
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }
