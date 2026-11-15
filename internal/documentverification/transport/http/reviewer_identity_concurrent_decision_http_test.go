package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const (
	identityRaceReviewerA      = "99999999-9999-4999-8999-999999999991"
	identityRaceReviewerB      = "99999999-9999-4999-8999-999999999992"
	identityRaceReviewerTokenA = "synthetic-reviewer-a"
	identityRaceReviewerTokenB = "synthetic-reviewer-b"
	identityRaceSubject        = "22222222-2222-4222-8222-222222222222"
	identityRaceCaseID         = "33333333-3333-4333-8333-333333333333"
	identityRaceDeclarationID  = "44444444-4444-4444-8444-444444444444"
)

type concurrentIdentityReviewRepository struct {
	mu                 sync.Mutex
	snapshot           dv.Snapshot
	authorizations     map[string]dv.Authorization
	audit              dv.TransitionAudit
	transitionAttempts int
	auditWrites        int
	readCalls          atomic.Int32
	preflightArrivals  atomic.Int32
	preflightGate      chan struct{}
}

func (r *concurrentIdentityReviewRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}

func (r *concurrentIdentityReviewRepository) Transition(ctx context.Context, actor string, purpose dv.Purpose, id string, revision uint64, audit dv.TransitionAudit, change func(*dv.Snapshot, dv.Authorization) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transitionAttempts++
	if err := ctx.Err(); err != nil {
		return err
	}
	if purpose != r.snapshot.Purpose || id != r.snapshot.ID {
		return dv.ErrNotFound
	}
	if revision != r.snapshot.Revision {
		return dv.ErrConflict
	}
	authorization, ok := r.authorizations[actor]
	if !ok {
		return dv.ErrCapability
	}
	candidate := r.snapshot
	if err := change(&candidate, authorization); err != nil {
		return err
	}
	candidate.Revision++
	r.snapshot = candidate
	r.audit = audit
	r.auditWrites++
	return nil
}

func (r *concurrentIdentityReviewRepository) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, nil
}

func (r *concurrentIdentityReviewRepository) LoadReviewerCase(ctx context.Context, actor, id string) (dv.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return dv.Snapshot{}, err
	}
	r.mu.Lock()
	authorization, authorized := r.authorizations[actor]
	snapshot := r.snapshot
	r.mu.Unlock()
	assigned := false
	for _, grant := range authorization.ReviewerGrants {
		if grant.ReviewerID == actor && grant.Capability == dv.CapabilityIdentity && grant.ScopeID == snapshot.SubjectID {
			assigned = true
			break
		}
	}
	if !authorized || !assigned || authorization.ActorID != actor || id != snapshot.ID || snapshot.SubjectID == actor {
		return dv.Snapshot{}, dv.ErrNotFound
	}

	arrival := r.preflightArrivals.Add(1)
	if arrival <= 2 {
		if arrival == 2 {
			close(r.preflightGate)
		}
		select {
		case <-r.preflightGate:
		case <-ctx.Done():
			return dv.Snapshot{}, ctx.Err()
		}
	}
	r.readCalls.Add(1)
	return snapshot, nil
}

func (r *concurrentIdentityReviewRepository) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	snapshot, err := r.LoadReviewerCase(ctx, actor, id)
	if err != nil {
		return err
	}
	r.mu.Lock()
	authorization := r.authorizations[actor]
	r.mu.Unlock()
	return read(ctx, snapshot, authorization)
}

type concurrentIdentityReviewDocumentStore struct{}

func (concurrentIdentityReviewDocumentStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func (concurrentIdentityReviewDocumentStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

type concurrentIdentityReviewAuthenticator struct{}

func (concurrentIdentityReviewAuthenticator) Authorize(_ context.Context, secret identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	switch string(secret) {
	case identityRaceReviewerTokenA:
		return identity.Principal{AccountID: identityRaceReviewerA}, nil
	case identityRaceReviewerTokenB:
		return identity.Principal{AccountID: identityRaceReviewerB}, nil
	default:
		return identity.Principal{}, errors.New("unknown synthetic reviewer")
	}
}

func TestConcurrentReviewerIdentityDecisionsReturnOneSuccessAndOneConflict(t *testing.T) {
	const expectedRevision = uint64(7)
	consentedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	file := func(id string, role dv.EvidenceRole, contentHash byte, mime string) dv.File {
		return dv.File{ID: id, EvidenceRole: role, State: dv.FileClean, Size: 4, MIME: mime, Pages: 1, ContentSHA256: [32]byte{contentHash}}
	}
	authorizationFor := func(reviewer string) dv.Authorization {
		return dv.Authorization{
			ActorID: reviewer, SubjectID: identityRaceSubject,
			IdentityDeclarationRevisionID: identityRaceDeclarationID,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewer, Capability: dv.CapabilityIdentity, ScopeID: identityRaceSubject,
			}},
			IdentityCurrent: true,
		}
	}
	repo := &concurrentIdentityReviewRepository{
		snapshot: dv.Snapshot{
			ID: identityRaceCaseID, Purpose: dv.PurposeIdentity, SubjectID: identityRaceSubject,
			State: dv.CaseInReview, Revision: expectedRevision,
			IdentityDeclarationRevisionID: identityRaceDeclarationID,
			DocumentFormatDeclared:        dv.DocumentFormatNew,
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            consentedAt,
			Files: []dv.File{
				file("11111111-1111-4111-8111-111111111121", dv.EvidenceRoleIdentityFront, 1, "image/png"),
				file("11111111-1111-4111-8111-111111111122", dv.EvidenceRoleIdentityBack, 2, "image/png"),
				file("11111111-1111-4111-8111-111111111123", dv.EvidenceRoleIdentitySelfie, 3, "image/jpeg"),
			},
		},
		authorizations: map[string]dv.Authorization{
			identityRaceReviewerA: authorizationFor(identityRaceReviewerA),
			identityRaceReviewerB: authorizationFor(identityRaceReviewerB),
		},
		preflightGate: make(chan struct{}),
	}
	domain, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	domain.SetClock(func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) })
	access, err := dv.NewReviewerAccessService(repo, domain, concurrentIdentityReviewDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(concurrentIdentityReviewAuthenticator{}, access))
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2030-05-01","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	type response struct {
		status int
		actor  string
		etag   string
		body   string
	}
	responses := make(chan response, 2)
	for _, request := range []struct{ token, actor string }{
		{identityRaceReviewerTokenA, identityRaceReviewerA},
		{identityRaceReviewerTokenB, identityRaceReviewerB},
	} {
		go func(token, actor string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req := httptest.NewRequest(http.MethodPost, reviewerCasesRoute+"/"+identityRaceCaseID+"/review", strings.NewReader(body)).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			responses <- response{status: w.Code, actor: actor, etag: w.Header().Get("ETag"), body: w.Body.String()}
		}(request.token, request.actor)
	}

	successes, conflicts := 0, 0
	winner := ""
	for range 2 {
		got := <-responses
		switch got.status {
		case http.StatusOK:
			successes++
			winner = got.actor
			if got.etag != `"8"` {
				t.Errorf("successful readback ETag=%q, want revision 8", got.etag)
			}
		case http.StatusConflict:
			conflicts++
			if got.body != "case revision conflict\n" || got.etag != "" {
				t.Errorf("conflict response body=%q ETag=%q", got.body, got.etag)
			}
		default:
			t.Errorf("unexpected HTTP status=%d body=%q", got.status, got.body)
		}
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want exactly one of each", successes, conflicts)
	}
	if repo.snapshot.State != dv.CaseApproved || repo.snapshot.Revision != expectedRevision+1 ||
		repo.snapshot.ReviewerID != winner {
		t.Fatalf("committed snapshot=%+v, successful response actor=%s", repo.snapshot, winner)
	}
	if repo.transitionAttempts != 2 || repo.readCalls.Load() != 3 || repo.auditWrites != 1 || repo.audit.Action != "review_decision" || repo.audit.ResultCode != string(dv.CaseApproved) {
		t.Fatalf("transition attempts=%d reads=%d audit writes=%d audit=%+v, want two decisions, two preflights, one readback and one audit", repo.transitionAttempts, repo.readCalls.Load(), repo.auditWrites, repo.audit)
	}
}
