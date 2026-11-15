package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityApprovalCapsCallerExpiryAtObservedDocumentDate(t *testing.T) {
	const (
		ownerID               = "88888888-8888-4888-8888-888888888888"
		declarationRevisionID = "99999999-9999-4999-8999-999999999999"
		expiryDate            = "2031-04-05"
	)
	boundary, err := observedExpiryBoundary(expiryDate)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseSubmitted, Revision: 7,
			IdentityDeclarationRevisionID: declarationRevisionID,
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            time.Date(2030, 2, 1, 12, 0, 0, 0, time.UTC),
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

	body := `{"decision":"approve","expected_revision":7,"verified_until":"2099-01-01T00:00:00Z","document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body)
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"8"` {
		t.Fatalf("status=%d ETag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	if !repo.snapshot.VerifiedUntil.Equal(boundary) || !repo.snapshot.DocumentExpiresAt.Equal(boundary) || !repo.snapshot.VerifiedAt.Equal(now) {
		t.Fatalf("snapshot verified_until=%s document_expires_at=%s verified_at=%s, want expiry boundary %s and decision time %s", repo.snapshot.VerifiedUntil, repo.snapshot.DocumentExpiresAt, repo.snapshot.VerifiedAt, boundary, now)
	}
	if !repo.audit.DocumentExpiresAtObserved.Equal(boundary) || repo.transitionCalls != 1 {
		t.Fatalf("audit=%+v transitions=%d, want observed expiry boundary and one transition", repo.audit, repo.transitionCalls)
	}
}
