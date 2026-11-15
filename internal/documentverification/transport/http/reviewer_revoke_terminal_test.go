package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerRevokeOfAlreadyRevokedCaseIsNeutralWithoutReadback(t *testing.T) {
	const ownerID = "88888888-8888-4888-8888-888888888888"
	revokedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	verifiedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	verifiedUntil := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	beforeAudit := dv.TransitionAudit{
		Action: "revoked", ResultCode: string(dv.CaseRevoked), ReasonCode: dv.ReasonOwnerRequest,
	}
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseRevoked, Revision: 8, ReviewerID: reviewerActor,
			RevokedAt: &revokedAt, VerifiedAt: verifiedAt, VerifiedUntil: verifiedUntil,
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
		audit: beforeAudit,
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

	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":8}`)
	response := httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, access).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || response.Body.String() != "not found\n" {
		t.Fatalf("status=%d body=%q, want neutral 404", response.Code, response.Body.String())
	}
	if repo.snapshot.State != dv.CaseRevoked || repo.snapshot.Revision != 8 || repo.snapshot.ReviewerID != reviewerActor ||
		repo.snapshot.RevokedAt == nil || !repo.snapshot.RevokedAt.Equal(revokedAt) ||
		repo.snapshot.SupersededAt != nil ||
		repo.snapshot.VerifiedAt != verifiedAt || repo.snapshot.VerifiedUntil != verifiedUntil || repo.audit != beforeAudit {
		t.Fatalf("repeated revoke mutated terminal snapshot/audit: snapshot=%+v audit=%+v", repo.snapshot, repo.audit)
	}
	if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 0 || repo.loadActor != "" {
		t.Fatalf("transition calls/actor=%d/%q, readback calls/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
}
