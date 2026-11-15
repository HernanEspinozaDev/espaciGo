package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerDecisionReadbackRepository struct {
	*revokeLifecycleRepository
	loadCalls  int
	failOnLoad int
}

func (r *reviewerDecisionReadbackRepository) LoadReviewerCase(ctx context.Context, actor, id string) (dv.Snapshot, error) {
	r.loadCalls++
	if r.loadCalls == r.failOnLoad {
		return dv.Snapshot{}, dv.ErrDocumentStorage
	}
	return r.revokeLifecycleRepository.LoadReviewerCase(ctx, actor, id)
}

func (r *reviewerDecisionReadbackRepository) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	snapshot, err := r.LoadReviewerCase(ctx, actor, id)
	if err != nil {
		return err
	}
	return read(ctx, snapshot, r.authorization)
}

func TestReviewerIdentityApprovalReadbackFailureCanBeResolvedWithoutRepeatingDecision(t *testing.T) {
	const (
		ownerID               = "88888888-8888-4888-8888-888888888888"
		declarationRevisionID = "99999999-9999-4999-8999-999999999999"
		expiryDate            = "2031-04-05"
	)
	boundary, err := observedExpiryBoundary(expiryDate)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)
	base := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseSubmitted, Revision: 7,
			IdentityDeclarationRevisionID: declarationRevisionID,
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            time.Date(2030, 10, 1, 12, 0, 0, 0, time.UTC),
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
	repo := &reviewerDecisionReadbackRepository{revokeLifecycleRepository: base, failOnLoad: 2}
	domain, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	domain.SetClock(func() time.Time { return now })
	access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewReviewerCasesHandler(reviewerAuth{}, access)
	postBody := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`

	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", postBody))
	if postResponse.Code != http.StatusServiceUnavailable || postResponse.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("POST status=%d body=%q, want neutral 503 after readback failure", postResponse.Code, postResponse.Body.String())
	}
	if base.snapshot.State != dv.CaseApproved || base.snapshot.Revision != 8 ||
		!base.snapshot.DocumentExpiresAt.Equal(boundary) || !base.audit.DocumentExpiresAtObserved.Equal(boundary) ||
		base.transitionCalls != 1 || repo.loadCalls != 2 {
		t.Fatalf("state after uncertain POST: snapshot=%+v audit=%+v transitions=%d reads=%d", base.snapshot, base.audit, base.transitionCalls, repo.loadCalls)
	}

	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", postBody))
	if replayResponse.Code != http.StatusConflict || replayResponse.Body.String() != "case revision conflict\n" || replayResponse.Header().Get("ETag") != "" {
		t.Fatalf("replayed decision status=%d ETag=%q body=%q, want neutral conflict without readback", replayResponse.Code, replayResponse.Header().Get("ETag"), replayResponse.Body.String())
	}
	if base.snapshot.State != dv.CaseApproved || base.snapshot.Revision != 8 ||
		!base.snapshot.DocumentExpiresAt.Equal(boundary) || !base.audit.DocumentExpiresAtObserved.Equal(boundary) ||
		base.audit.Action != "review_decision" || base.audit.ResultCode != string(dv.CaseApproved) ||
		base.transitionCalls != 2 || repo.loadCalls != 3 {
		t.Fatalf("replay changed decision or read it back: snapshot=%+v audit=%+v transitions=%d reads=%d", base.snapshot, base.audit, base.transitionCalls, repo.loadCalls)
	}

	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, reviewerReq(http.MethodGet, reviewerCasesRoute+"/"+reviewerID, ""))
	if getResponse.Code != http.StatusOK || getResponse.Header().Get("ETag") != `"8"` {
		t.Fatalf("reconciliation GET status=%d ETag=%q body=%q", getResponse.Code, getResponse.Header().Get("ETag"), getResponse.Body.String())
	}
	var result reviewerCaseDTO
	if err := json.Unmarshal(getResponse.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != dv.CaseApproved || result.Revision != 8 || result.DocumentExpiresAtObserved == nil || *result.DocumentExpiresAtObserved != expiryDate {
		t.Fatalf("reconciliation DTO=%+v, want approved revision 8 with civil expiry %q", result, expiryDate)
	}
	if base.transitionCalls != 2 || repo.loadCalls != 4 {
		t.Fatalf("explicit GET repeated the decision or skipped readback: transitions=%d total loads=%d", base.transitionCalls, repo.loadCalls)
	}
}
