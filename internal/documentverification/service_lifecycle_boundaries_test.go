package documentverification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceValidSnapshotAndStartBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	spaceSnap := Snapshot{
		ID:                caseID,
		SubjectID:         subject,
		SpaceID:           space,
		Purpose:           PurposeSpace,
		State:             CaseDraft,
		DocumentExpiresAt: now.Add(24 * time.Hour),
	}
	if !validSnapshot(spaceSnap) {
		t.Fatal("expected base spaceSnap to be valid")
	}

	// Space snapshot cannot have capture consent fields
	withConsentVer := spaceSnap
	withConsentVer.CaptureConsentVersion = IdentityCaptureConsentVersion
	if validSnapshot(withConsentVer) {
		t.Fatal("expected validSnapshot=false for space snap with CaptureConsentVersion")
	}

	withConsentAt := spaceSnap
	withConsentAt.CaptureConsentedAt = now
	if validSnapshot(withConsentAt) {
		t.Fatal("expected validSnapshot=false for space snap with CaptureConsentedAt")
	}

	// Start with PurposeSpace invalid branches
	r := &memoryRepository{}
	svc, err := NewService(r)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetClock(func() time.Time { return now })

	// 1. Space with DocumentFormatDeclared != ""
	_, err = svc.Start(context.Background(), subject, CreateInput{
		Purpose:                PurposeSpace,
		SpaceID:                space,
		DocumentFormatDeclared: DocumentFormatLegacy,
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Start space with format declared err=%v, want ErrInvalid", err)
	}

	// 2. Space with DocumentExpiresAt in the past
	_, err = svc.Start(context.Background(), subject, CreateInput{
		Purpose:           PurposeSpace,
		SpaceID:           space,
		DocumentExpiresAt: now.Add(-1 * time.Hour),
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Start space with expired date err=%v, want ErrInvalid", err)
	}

	// 3. Space with CaptureConsentGiven=true
	_, err = svc.Start(context.Background(), subject, CreateInput{
		Purpose:             PurposeSpace,
		SpaceID:             space,
		CaptureConsentGiven: true,
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Start space with consent given err=%v, want ErrInvalid", err)
	}

	// 4. Space with CaptureConsentVersion != ""
	_, err = svc.Start(context.Background(), subject, CreateInput{
		Purpose:               PurposeSpace,
		SpaceID:               space,
		CaptureConsentVersion: IdentityCaptureConsentVersion,
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Start space with consent version err=%v, want ErrInvalid", err)
	}
}

func TestServiceAddFileExhaustiveBoundaries(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.ActorID = subject
	r.state.DocumentExpiresAt = time.Time{}
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.Files = nil
	validFile := File{
		ID:            uuid.NewString(),
		EvidenceRole:  EvidenceRoleIdentityFront,
		State:         FilePending,
		Size:          1024,
		MIME:          "image/jpeg",
		Pages:         1,
		ContentSHA256: [32]byte{1},
	}

	// 1. x.SubjectID != actor -> ErrCapability
	otherSubject := "99999999-9999-4999-8999-999999999999"
	r.state.State = CaseDraft
	r.state.SubjectID = otherSubject
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision, validFile); !errors.Is(err, ErrCapability) {
		t.Fatalf("AddFile actor mismatch err=%v, want ErrCapability", err)
	}
	r.state.SubjectID = subject

	// 2. Space case where x.SubjectID != a.SpaceOwnerID -> ErrCapability
	sSpace, rSpace, _ := fixture(t, PurposeSpace)
	rSpace.auth.ActorID = subject
	rSpace.state.State = CaseDraft
	rSpace.auth.SpaceOwnerID = otherSubject
	spaceFile := validFile
	spaceFile.EvidenceRole = ""
	if err := sSpace.AddFile(context.Background(), subject, caseID, PurposeSpace, rSpace.state.Revision, spaceFile); !errors.Is(err, ErrCapability) {
		t.Fatalf("AddFile space owner mismatch err=%v, want ErrCapability", err)
	}

	// 3. x.State not Draft or NeedsEvidence -> ErrTransition
	r.state.State = CaseSubmitted
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision, validFile); !errors.Is(err, ErrTransition) {
		t.Fatalf("AddFile submitted state err=%v, want ErrTransition", err)
	}
	r.state.State = CaseDraft

	// 4. Identity case with DocumentFormatDeclared == "" but file.EvidenceRole != "" -> ErrInvalid
	r.state.DocumentFormatDeclared = ""
	r.state.IdentityDeclarationRevisionID = ""
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision, validFile); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddFile undeclared format with role err=%v, want ErrInvalid", err)
	}
	r.state.IdentityDeclarationRevisionID = declarationRevisionID

	// 5. Identity case with DocumentFormatDeclared != "" but file.EvidenceRole == "" -> ErrInvalid
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	emptyRoleFile := validFile
	emptyRoleFile.EvidenceRole = ""
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision, emptyRoleFile); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddFile declared format with empty role err=%v, want ErrInvalid", err)
	}

	// 6. Identity case with DocumentFormatDeclared != "" but file.MIME == "application/pdf" -> ErrInvalid
	pdfFile := validFile
	pdfFile.MIME = "application/pdf"
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision, pdfFile); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddFile declared format with PDF MIME err=%v, want ErrInvalid", err)
	}
}

func TestServiceSubmitExhaustiveBoundaries(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.ActorID = subject
	r.state.State = CaseDraft
	r.state.DocumentExpiresAt = time.Time{}
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.Files = []File{
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
	}

	// 1. x.SubjectID != actor -> ErrCapability
	otherSubject := "99999999-9999-4999-8999-999999999999"
	r.state.SubjectID = otherSubject
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrCapability) {
		t.Fatalf("Submit actor mismatch err=%v, want ErrCapability", err)
	}
	r.state.SubjectID = subject

	// 2. Space case where x.SubjectID != a.SpaceOwnerID -> ErrConflict
	sSpace, rSpace, _ := fixture(t, PurposeSpace)
	rSpace.auth.ActorID = subject
	rSpace.state.State = CaseDraft
	rSpace.auth.SpaceOwnerID = otherSubject
	if err := sSpace.Submit(context.Background(), subject, caseID, PurposeSpace, rSpace.state.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("Submit space owner mismatch err=%v, want ErrConflict", err)
	}

	// 3. x.State not Draft and not NeedsEvidence -> ErrTransition
	r.state.State = CaseApproved
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrTransition) {
		t.Fatalf("Submit approved state err=%v, want ErrTransition", err)
	}
	r.state.State = CaseDraft

	// 4. Space draft with non-clean file -> ErrFilesNotClean
	rSpace.auth.SpaceOwnerID = subject
	rSpace.state.State = CaseDraft
	rSpace.state.Files = []File{
		{ID: uuid.NewString(), State: FilePending, Size: 100, MIME: "application/pdf", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	if err := sSpace.Submit(context.Background(), subject, caseID, PurposeSpace, rSpace.state.Revision); !errors.Is(err, ErrFilesNotClean) {
		t.Fatalf("Submit space pending file err=%v, want ErrFilesNotClean", err)
	}

	// 5. Identity draft with missing evidence role (only 2 files) -> ErrEvidenceIncomplete
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.Files = []File{
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
	}
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrEvidenceIncomplete) {
		t.Fatalf("Submit missing evidence role err=%v, want ErrEvidenceIncomplete", err)
	}
}

func TestServiceReviewExhaustiveBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	s, r, _ := fixture(t, PurposeIdentity)
	s.SetClock(func() time.Time { return now })
	r.auth.ActorID = actor
	r.state.State = CaseSubmitted
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.Files = []File{
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
	}
	boolTrue := true
	validChecklist := &IdentityReviewChecklist{
		NamesMatch:                 &boolTrue,
		RUTMatches:                 &boolTrue,
		BirthDateAndMajorityMatch:  &boolTrue,
		PortraitMatchesSelfie:      &boolTrue,
		EvidenceLegible:            &boolTrue,
		DocumentUnexpired:          &boolTrue,
		IssueDateAndFormatVerified: &boolTrue,
	}
	expiry := now.AddDate(2, 0, 0)

	// 1. Observed format on non-identity capability -> ErrInvalid
	if err := s.reviewWithObservation(context.Background(), actor, caseID, CapabilitySpace, DecisionApprove, "", DocumentFormatNew, expiry, nil, true, time.Time{}, r.state.Revision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reviewWithObservation space with observed format err=%v, want ErrInvalid", err)
	}

	// 2. Case in CaseDraft state -> ErrTransition
	r.state.State = CaseDraft
	if err := s.ReviewIdentityWithReason(context.Background(), actor, caseID, DecisionApprove, "", DocumentFormatNew, expiry, validChecklist, time.Time{}, r.state.Revision); !errors.Is(err, ErrTransition) {
		t.Fatalf("Review draft state err=%v, want ErrTransition", err)
	}
	r.state.State = CaseSubmitted

	// 3. Invalid decision code -> ErrInvalid
	if err := s.reviewWithObservation(context.Background(), actor, caseID, CapabilityIdentity, Decision("invalid_decision"), "", DocumentFormatNew, expiry, validChecklist, true, time.Time{}, r.state.Revision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Review invalid decision err=%v, want ErrInvalid", err)
	}

	// 4. Space case with non-nil checklist -> ErrInvalid
	sSpace, rSpace, _ := fixture(t, PurposeSpace)
	sSpace.SetClock(func() time.Time { return now })
	rSpace.auth.ActorID = actor
	rSpace.auth.ReviewerGrants = []ReviewerGrant{{
		ReviewerID: actor,
		Capability: CapabilitySpace,
		ScopeID:    space,
	}}
	rSpace.state.State = CaseSubmitted
	if err := sSpace.reviewWithObservation(context.Background(), actor, caseID, CapabilitySpace, DecisionApprove, "", "", time.Time{}, validChecklist, false, time.Time{}, rSpace.state.Revision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Review space with checklist err=%v, want ErrInvalid", err)
	}

	// 5. Approved review with requestedExpiry shortening the deadline
	requestedExpiry := now.AddDate(0, 6, 0) // 6 months instead of 12
	if err := s.ReviewIdentityWithReason(context.Background(), actor, caseID, DecisionApprove, "", DocumentFormatNew, expiry, validChecklist, requestedExpiry, r.state.Revision); err != nil {
		t.Fatalf("Review with requestedExpiry failed: %v", err)
	}
	if !r.state.VerifiedUntil.Equal(requestedExpiry) {
		t.Fatalf("VerifiedUntil=%v, want requestedExpiry=%v", r.state.VerifiedUntil, requestedExpiry)
	}

	// 6. Approved review with requestedExpiry in the past -> ErrInvalid
	r.state.State = CaseSubmitted
	if err := s.ReviewIdentityWithReason(context.Background(), actor, caseID, DecisionApprove, "", DocumentFormatNew, expiry, validChecklist, now.Add(-1*time.Hour), r.state.Revision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Review with past requestedExpiry err=%v, want ErrInvalid", err)
	}
}

func TestServiceRevokeExhaustiveBoundaries(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.ActorID = actor
	r.state.State = CaseApproved

	// 1. x.SubjectID == actor (self revocation) -> ErrSelfReview
	r.auth.ReviewerGrants = append(r.auth.ReviewerGrants, ReviewerGrant{
		ReviewerID: actor,
		Capability: CapabilityIdentity,
		ScopeID:    actor,
	})
	r.state.SubjectID = actor
	if err := s.Revoke(context.Background(), actor, caseID, PurposeIdentity, ReasonDocumentInvalid, r.state.Revision); !errors.Is(err, ErrSelfReview) {
		t.Fatalf("Revoke self subject err=%v, want ErrSelfReview", err)
	}
	r.state.SubjectID = subject

	// 2. x.State != CaseApproved (e.g. CaseDraft) -> ErrTransition
	r.state.State = CaseDraft
	if err := s.Revoke(context.Background(), actor, caseID, PurposeIdentity, ReasonDocumentInvalid, r.state.Revision); !errors.Is(err, ErrTransition) {
		t.Fatalf("Revoke draft state err=%v, want ErrTransition", err)
	}
}

func TestServiceTransactAndReviewerGrantBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	sSpace, rSpace, _ := fixture(t, PurposeSpace)
	rSpace.auth.ActorID = subject
	rSpace.state.State = CaseDraft

	// 1. x.SupersededAt != nil -> ErrConflict (allowed by validSnapshot for PurposeSpace)
	supersededTime := now
	rSpace.state.SupersededAt = &supersededTime
	if err := sSpace.Submit(context.Background(), subject, caseID, PurposeSpace, rSpace.state.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("Submit superseded snapshot err=%v, want ErrConflict", err)
	}
	rSpace.state.SupersededAt = nil

	// 2. x.Revision != rev -> ErrConflict
	if err := sSpace.Submit(context.Background(), subject, caseID, PurposeSpace, rSpace.state.Revision+99); !errors.Is(err, ErrConflict) {
		t.Fatalf("Submit revision mismatch err=%v, want ErrConflict", err)
	}

	// 3. cap != "" and purposeForCap(cap) != p -> ErrCapability
	err := sSpace.transact(context.Background(), PurposeSpace, caseID, rSpace.state.Revision, subject, CapabilityIdentity, TransitionAudit{}, func(*Snapshot, Authorization) error {
		return nil
	})
	if !errors.Is(err, ErrCapability) {
		t.Fatalf("transact mismatched capability err=%v, want ErrCapability", err)
	}

	// 4. hasReviewerGrant boundary tests
	validScope := subject
	validGrant := ReviewerGrant{
		ReviewerID: actor,
		Capability: CapabilityIdentity,
		ScopeID:    validScope,
	}
	authBase := Authorization{
		ActorID:        actor,
		ReviewerGrants: []ReviewerGrant{validGrant},
	}
	if !hasReviewerGrant(authBase, CapabilityIdentity, validScope) {
		t.Fatal("expected valid grant to return true")
	}

	// a. Empty ActorID
	noActor := authBase
	noActor.ActorID = ""
	if hasReviewerGrant(noActor, CapabilityIdentity, validScope) {
		t.Fatal("expected empty actor to return false")
	}

	// b. Invalid Capability argument
	if hasReviewerGrant(authBase, Capability("unknown"), validScope) {
		t.Fatal("expected invalid capability to return false")
	}

	// c. Invalid scopeID argument
	if hasReviewerGrant(authBase, CapabilityIdentity, "not-a-uuid") {
		t.Fatal("expected invalid scopeID to return false")
	}

	// d. Grant with invalid ReviewerID
	badGrantReviewer := authBase
	badGrantReviewer.ReviewerGrants = []ReviewerGrant{{
		ReviewerID: "not-a-uuid",
		Capability: CapabilityIdentity,
		ScopeID:    validScope,
	}}
	if hasReviewerGrant(badGrantReviewer, CapabilityIdentity, validScope) {
		t.Fatal("expected grant with invalid ReviewerID to return false")
	}

	// e. Grant with ReviewerID mismatching ActorID
	mismatchGrantReviewer := authBase
	mismatchGrantReviewer.ReviewerGrants = []ReviewerGrant{{
		ReviewerID: "22222222-2222-4222-8222-222222222222",
		Capability: CapabilityIdentity,
		ScopeID:    validScope,
	}}
	if hasReviewerGrant(mismatchGrantReviewer, CapabilityIdentity, validScope) {
		t.Fatal("expected grant with mismatching ReviewerID to return false")
	}

	// f. Grant with invalid Capability
	badGrantCap := authBase
	badGrantCap.ReviewerGrants = []ReviewerGrant{{
		ReviewerID: actor,
		Capability: Capability("unknown"),
		ScopeID:    validScope,
	}}
	if hasReviewerGrant(badGrantCap, CapabilityIdentity, validScope) {
		t.Fatal("expected grant with invalid Capability to return false")
	}

	// g. Grant with invalid ScopeID
	badGrantScope := authBase
	badGrantScope.ReviewerGrants = []ReviewerGrant{{
		ReviewerID: actor,
		Capability: CapabilityIdentity,
		ScopeID:    "not-a-uuid",
	}}
	if hasReviewerGrant(badGrantScope, CapabilityIdentity, validScope) {
		t.Fatal("expected grant with invalid ScopeID to return false")
	}

	// h. Dual-purpose grants (both identity and space) denied fail-closed
	dualGrantAuth := authBase
	dualGrantAuth.ReviewerGrants = []ReviewerGrant{
		validGrant,
		{
			ReviewerID: actor,
			Capability: CapabilitySpace,
			ScopeID:    space,
		},
	}
	if hasReviewerGrant(dualGrantAuth, CapabilityIdentity, validScope) {
		t.Fatal("expected dual-purpose reviewer assignment to be denied")
	}
}
