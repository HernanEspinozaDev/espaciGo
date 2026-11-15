package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityRejectHTTPUsesActualServiceAndReadsBack(t *testing.T) {
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
	domain, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	domain.SetClock(func() time.Time { return time.Date(2026, 11, 11, 17, 0, 0, 0, time.UTC) })
	access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}

	body := `{"decision":"reject","reason_code":"documento_invalido","expected_revision":7,"document_format_observed":"new","identity_review_checklist":{"names_match":false,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body)
	response := httptest.NewRecorder()
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"8"` {
		t.Fatalf("status=%d ETag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	var result reviewerCaseDTO
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != reviewerID || result.State != dv.CaseRejected || result.Revision != 8 || result.DocumentFormatObserved != dv.DocumentFormatNew {
		t.Fatalf("review response=%+v", result)
	}
	if repo.snapshot.State != dv.CaseRejected || repo.snapshot.Revision != 8 || repo.snapshot.ReviewerID != reviewerActor ||
		repo.snapshot.DocumentFormatObserved != dv.DocumentFormatNew || !repo.snapshot.DocumentExpiresAt.IsZero() ||
		!repo.snapshot.VerifiedAt.IsZero() || !repo.snapshot.VerifiedUntil.IsZero() {
		t.Fatalf("service transition=%+v", repo.snapshot)
	}
	if repo.audit.Action != "review_decision" || repo.audit.ResultCode != string(dv.CaseRejected) ||
		repo.audit.ReasonCode != dv.ReasonDocumentInvalid || repo.audit.DocumentFormatObserved != dv.DocumentFormatNew ||
		repo.audit.IdentityReviewChecklist == nil || !repo.audit.IdentityReviewChecklist.Complete() ||
		*repo.audit.IdentityReviewChecklist.NamesMatch {
		t.Fatalf("review audit=%+v", repo.audit)
	}
	if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 2 || repo.loadActor != reviewerActor {
		t.Fatalf("transition calls/actor=%d/%q, load calls/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
}
