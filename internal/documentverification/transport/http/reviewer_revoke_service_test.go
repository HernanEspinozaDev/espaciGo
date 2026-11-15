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

type revokeLifecycleRepository struct {
	snapshot          dv.Snapshot
	authorization     dv.Authorization
	audit             dv.TransitionAudit
	transitionActor   string
	loadActor         string
	loadErr           error
	transitionCalls   int
	reviewerLoadCalls int
}

func (r *revokeLifecycleRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}

func (r *revokeLifecycleRepository) Transition(ctx context.Context, actor string, purpose dv.Purpose, id string, revision uint64, audit dv.TransitionAudit, change func(*dv.Snapshot, dv.Authorization) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.transitionCalls++
	r.transitionActor = actor
	if purpose != r.snapshot.Purpose || id != r.snapshot.ID {
		return dv.ErrNotFound
	}
	if revision != r.snapshot.Revision {
		return dv.ErrConflict
	}
	candidate := r.snapshot
	if err := change(&candidate, r.authorization); err != nil {
		return err
	}
	candidate.Revision++
	r.snapshot = candidate
	r.audit = audit
	return nil
}

func (r *revokeLifecycleRepository) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, nil
}

func (r *revokeLifecycleRepository) LoadReviewerCase(_ context.Context, actor, id string) (dv.Snapshot, error) {
	r.reviewerLoadCalls++
	r.loadActor = actor
	if actor != r.authorization.ActorID || id != r.snapshot.ID {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	if r.loadErr != nil {
		return dv.Snapshot{}, r.loadErr
	}
	return r.snapshot, nil
}

func (r *revokeLifecycleRepository) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	snapshot, err := r.LoadReviewerCase(ctx, actor, id)
	if err != nil {
		return err
	}
	return read(ctx, snapshot, r.authorization)
}

type revokeLifecycleDocumentStore struct{}

func (revokeLifecycleDocumentStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func (revokeLifecycleDocumentStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func TestReviewerRevokeTransitionsActualServiceAndReadsBackInMemoryState(t *testing.T) {
	const ownerID = "88888888-8888-4888-8888-888888888888"
	now := time.Date(2026, 11, 11, 17, 0, 0, 0, time.UTC)
	verifiedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	verifiedUntil := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseApproved, Revision: 7,
			VerifiedAt:    verifiedAt,
			VerifiedUntil: verifiedUntil,
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
	}
	cases, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	cases.SetClock(func() time.Time { return now })
	access, err := dv.NewReviewerAccessService(repo, cases, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}

	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`)
	response := httptest.NewRecorder()
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"8"` {
		t.Fatalf("status=%d etag=%q body=%s", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	var body reviewerCaseDTO
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != reviewerID || body.State != dv.CaseRevoked || body.Revision != 8 {
		t.Fatalf("response case=%+v, want revoked revision 8", body.ownerCaseDTO)
	}
	if repo.snapshot.State != dv.CaseRevoked || repo.snapshot.Revision != 8 || repo.snapshot.ReviewerID != reviewerActor ||
		repo.snapshot.RevokedAt == nil || !repo.snapshot.RevokedAt.Equal(now) ||
		!repo.snapshot.VerifiedAt.Equal(verifiedAt) || !repo.snapshot.VerifiedUntil.Equal(verifiedUntil) {
		t.Fatalf("in-memory transition=%+v, want revoked by authenticated reviewer at %s", repo.snapshot, now)
	}
	if repo.audit.Action != "revoked" || repo.audit.ResultCode != string(dv.CaseRevoked) || repo.audit.ReasonCode != dv.ReasonOwnerRequest {
		t.Fatalf("audit=%+v, want the service-generated revocation event", repo.audit)
	}
	if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 1 || repo.loadActor != reviewerActor {
		t.Fatalf("transition calls/actor=%d/%q, readback calls/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
}

func TestReviewerRevokeWithInvalidGrantIsNeutralAndDoesNotReadBack(t *testing.T) {
	const ownerID = "88888888-8888-4888-8888-888888888888"
	const otherOwnerID = "77777777-7777-4777-8777-777777777777"
	const otherReviewerID = "55555555-5555-4555-8555-555555555555"
	tests := []struct {
		name   string
		grants []dv.ReviewerGrant
	}{
		{name: "missing grant"},
		{
			name: "grant assigned to another reviewer",
			grants: []dv.ReviewerGrant{{
				ReviewerID: otherReviewerID, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
		{
			name: "grant scoped to another subject",
			grants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: otherOwnerID,
			}},
		},
		{
			name: "space capability cannot revoke identity",
			grants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilitySpace, ScopeID: ownerID,
			}},
		},
		{
			name: "dual-purpose grants are not accepted",
			grants: []dv.ReviewerGrant{
				{ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID},
				{ReviewerID: reviewerActor, Capability: dv.CapabilitySpace, ScopeID: otherOwnerID},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := dv.Snapshot{
				ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
				State: dv.CaseApproved, Revision: 7,
				VerifiedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				VerifiedUntil: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			}
			repo := &revokeLifecycleRepository{
				snapshot: before,
				authorization: dv.Authorization{
					ActorID: reviewerActor, ReviewerGrants: tc.grants,
				},
			}
			domain, err := dv.NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
			if err != nil {
				t.Fatal(err)
			}
			request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`)
			response := httptest.NewRecorder()
			NewReviewerCasesHandler(reviewerAuth{}, access).ServeHTTP(response, request)

			if response.Code != http.StatusNotFound || response.Body.String() != "not found\n" {
				t.Fatalf("status=%d body=%q, want neutral 404", response.Code, response.Body.String())
			}
			if repo.snapshot.State != before.State || repo.snapshot.Revision != before.Revision || repo.snapshot.RevokedAt != nil ||
				repo.snapshot.ReviewerID != before.ReviewerID || repo.snapshot.VerifiedAt != before.VerifiedAt ||
				repo.snapshot.VerifiedUntil != before.VerifiedUntil || repo.audit != (dv.TransitionAudit{}) {
				t.Fatalf("denied revocation mutated snapshot/audit: snapshot=%+v audit=%+v", repo.snapshot, repo.audit)
			}
			if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 0 || repo.loadActor != "" {
				t.Fatalf("transition calls/actor=%d/%q, readback calls/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
			}
		})
	}
}

func TestReviewerRevokeReadbackFailureCanBeResolvedByGet(t *testing.T) {
	const ownerID = "88888888-8888-4888-8888-888888888888"
	now := time.Date(2026, 11, 11, 17, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseApproved, Revision: 7,
			VerifiedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			VerifiedUntil: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
		loadErr: dv.ErrDocumentStorage,
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
	handler := NewReviewerCasesHandler(reviewerAuth{}, access)

	post := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`)
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusServiceUnavailable || postResponse.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("POST status=%d body=%q, want neutral 503 after readback error", postResponse.Code, postResponse.Body.String())
	}
	if repo.snapshot.State != dv.CaseRevoked || repo.snapshot.Revision != 8 || repo.snapshot.RevokedAt == nil ||
		!repo.snapshot.RevokedAt.Equal(now) || repo.transitionCalls != 1 || repo.reviewerLoadCalls != 1 {
		t.Fatalf("post/readback failure state=%+v transitions=%d reads=%d", repo.snapshot, repo.transitionCalls, repo.reviewerLoadCalls)
	}

	repo.loadErr = nil
	get := reviewerReq(http.MethodGet, reviewerCasesRoute+"/"+reviewerID, "")
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK || getResponse.Header().Get("ETag") != `"8"` {
		t.Fatalf("GET status=%d etag=%q body=%s", getResponse.Code, getResponse.Header().Get("ETag"), getResponse.Body.String())
	}
	var body reviewerCaseDTO
	if err := json.Unmarshal(getResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode reconciled case: %v", err)
	}
	if body.State != dv.CaseRevoked || body.Revision != 8 || repo.transitionCalls != 1 || repo.reviewerLoadCalls != 2 {
		t.Fatalf("reconciled case=%+v transitions=%d reads=%d", body.ownerCaseDTO, repo.transitionCalls, repo.reviewerLoadCalls)
	}
}
