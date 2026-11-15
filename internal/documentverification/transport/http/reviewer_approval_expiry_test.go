package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityApprovalWithoutObservedExpiryDoesNotMutate(t *testing.T) {
	const ownerID = "88888888-8888-4888-8888-888888888888"
	const declarationRevisionID = "99999999-9999-4999-8999-999999999999"
	consentedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseSubmitted, Revision: 7,
			IdentityDeclarationRevisionID: declarationRevisionID,
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            consentedAt,
			DocumentFormatDeclared:        dv.DocumentFormatNew,
			Files: []dv.File{
				{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			},
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
	}
	beforeSnapshot := repo.snapshot
	beforeAudit := repo.audit
	domain, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	domain.SetClock(func() time.Time { return time.Date(2026, 11, 11, 17, 0, 0, 0, time.UTC) })
	access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}

	body := `{"decision":"approve","reason_code":"","expected_revision":7,"document_format_observed":"new","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body)
	response := httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, access).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || response.Body.String() != "invalid request\n" {
		t.Fatalf("status=%d body=%q, want invalid request", response.Code, response.Body.String())
	}
	if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 1 || repo.loadActor != reviewerActor {
		t.Fatalf("transition calls/actor=%d/%q, loads/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
	if !reflect.DeepEqual(repo.snapshot, beforeSnapshot) || repo.audit != beforeAudit {
		t.Fatalf("approval without observed expiry mutated state: snapshot=%+v audit=%+v", repo.snapshot, repo.audit)
	}
}

func TestReviewerIdentityApprovalHTTPPreservesCivilExpiryThroughService(t *testing.T) {
	const (
		ownerID               = "88888888-8888-4888-8888-888888888888"
		declarationRevisionID = "99999999-9999-4999-8999-999999999999"
		expiryDate            = "2032-02-29"
	)
	boundary, err := observedExpiryBoundary(expiryDate)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2032, 2, 1, 12, 0, 0, 0, time.UTC)
	consentedAt := time.Date(2030, 10, 1, 12, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseSubmitted, Revision: 7,
			IdentityDeclarationRevisionID: declarationRevisionID,
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            consentedAt,
			DocumentFormatDeclared:        dv.DocumentFormatNew,
			Files: []dv.File{
				{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			},
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
	}
	domain, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	domain.SetClock(func() time.Time { return now })
	access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}

	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2032-02-29","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body)
	NewReviewerCasesHandler(reviewerAuth{}, access).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"8"` {
		t.Fatalf("status=%d ETag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	if repo.snapshot.State != dv.CaseApproved || repo.snapshot.Revision != 8 ||
		!repo.snapshot.DocumentExpiresAt.Equal(boundary) || !repo.snapshot.VerifiedUntil.Equal(boundary) || !repo.snapshot.VerifiedAt.Equal(now) {
		t.Fatalf("approved snapshot=%+v, boundary=%s now=%s", repo.snapshot, boundary, now)
	}
	if !repo.audit.DocumentExpiresAtObserved.Equal(boundary) || repo.audit.IdentityReviewChecklist == nil || !repo.audit.IdentityReviewChecklist.AllPassed() {
		t.Fatalf("approval audit=%+v, want observed boundary %s and complete passing checklist", repo.audit, boundary)
	}
	var responseDTO reviewerCaseDTO
	if err := json.Unmarshal(response.Body.Bytes(), &responseDTO); err != nil {
		t.Fatal(err)
	}
	if responseDTO.DocumentExpiresAtObserved == nil || *responseDTO.DocumentExpiresAtObserved != expiryDate {
		t.Fatalf("response expiry=%v, want civil date %q", responseDTO.DocumentExpiresAtObserved, expiryDate)
	}
}
