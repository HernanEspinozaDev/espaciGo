package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type serviceRevisionGuardRepository struct {
	*revokeLifecycleRepository
	callbackInvocations int
}

// Transition intentionally forwards the callback without an adapter-side CAS
// check so the domain service's expected-revision guard is exercised directly.
func (r *serviceRevisionGuardRepository) Transition(ctx context.Context, actor string, purpose dv.Purpose, id string, _ uint64, audit dv.TransitionAudit, change func(*dv.Snapshot, dv.Authorization) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.transitionCalls++
	r.transitionActor = actor
	if purpose != r.snapshot.Purpose || id != r.snapshot.ID {
		return dv.ErrNotFound
	}
	candidate := r.snapshot
	r.callbackInvocations++
	if err := change(&candidate, r.authorization); err != nil {
		return err
	}
	candidate.Revision++
	r.snapshot = candidate
	r.audit = audit
	return nil
}

func TestReviewerRevokeStaleRevisionIsRejectedByServiceBeforeMutation(t *testing.T) {
	const ownerID = "88888888-8888-4888-8888-888888888888"
	verifiedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	verifiedUntil := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	base := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseApproved, Revision: 8,
			VerifiedAt: verifiedAt, VerifiedUntil: verifiedUntil,
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
	}
	repo := &serviceRevisionGuardRepository{revokeLifecycleRepository: base}
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

	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`)
	response := httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, access).ServeHTTP(response, request)

	if response.Code != http.StatusConflict || response.Body.String() != "case revision conflict\n" {
		t.Fatalf("status=%d body=%q, want 409 revision conflict", response.Code, response.Body.String())
	}
	if repo.callbackInvocations != 1 || !reflect.DeepEqual(repo.snapshot, beforeSnapshot) || repo.audit != beforeAudit {
		t.Fatalf("service revision guard failed: callbacks=%d snapshot=%+v audit=%+v", repo.callbackInvocations, repo.snapshot, repo.audit)
	}
	if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 0 || repo.loadActor != "" {
		t.Fatalf("transition calls/actor=%d/%q, readback calls/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
}
