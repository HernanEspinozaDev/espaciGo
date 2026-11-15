package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const reviewerQueueProjectionActor = "66666666-6666-4666-8666-666666666666"

type reviewerQueueProjectionRepo struct {
	listed    []dv.Snapshot
	listActor string
	listCalls int
}

type reviewerQueueProjectionDomainRepo struct{}

func (reviewerQueueProjectionDomainRepo) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}

func (reviewerQueueProjectionDomainRepo) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrCapability
}

type reviewerQueueProjectionAuth struct{}

func (reviewerQueueProjectionAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: reviewerQueueProjectionActor}, nil
}

type reviewerQueueProjectionStore struct{}

func (reviewerQueueProjectionStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func (reviewerQueueProjectionStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func (r *reviewerQueueProjectionRepo) ListReviewerCases(ctx context.Context, actor string) ([]dv.Snapshot, error) {
	r.listCalls++
	r.listActor = actor
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.listed, nil
}

func (*reviewerQueueProjectionRepo) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrNotFound
}

func (*reviewerQueueProjectionRepo) WithReviewerCaseForRead(context.Context, string, string, func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrNotFound
}

func TestReviewerQueueHandlerSerializesOnlyReviewableAssignedSnapshots(t *testing.T) {
	const (
		identitySubject = "11111111-1111-4111-8111-111111111111"
		spaceSubject    = "22222222-2222-4222-8222-222222222222"
		spaceID         = "33333333-3333-4333-8333-333333333333"
		identityCaseID  = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
		spaceCaseID     = "44444444-4444-4444-8444-444444444444"
		invalidCaseID   = "55555555-5555-4555-8555-555555555555"
		selfCaseID      = "66666666-6666-4666-8666-666666666666"
		revokedCaseID   = "77777777-7777-4777-8777-777777777777"
		supersededID    = "88888888-8888-4888-8888-888888888888"
		unknownCaseID   = "99999999-9999-4999-8999-999999999991"
		spaceNoIDCaseID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		declarationID   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	repo := &reviewerQueueProjectionRepo{
		listed: []dv.Snapshot{
			{
				ID: identityCaseID, SubjectID: identitySubject, Purpose: dv.PurposeIdentity,
				State: dv.CaseSubmitted, Revision: 3,
				IdentityDeclarationRevisionID: declarationID,
				DocumentFormatDeclared:        dv.DocumentFormatNew,
			},
			{
				ID: spaceCaseID, SubjectID: spaceSubject, SpaceID: spaceID,
				Purpose: dv.PurposeSpace, State: dv.CaseNeedsEvidence, Revision: 4,
			},
			{ID: selfCaseID, SubjectID: reviewerQueueProjectionActor, Purpose: dv.PurposeIdentity, State: dv.CaseInReview},
			{ID: revokedCaseID, SubjectID: identitySubject, Purpose: dv.PurposeIdentity, State: dv.CaseInReview, RevokedAt: &now},
			{ID: supersededID, SubjectID: identitySubject, Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted, SupersededAt: &now},
			{ID: unknownCaseID, SubjectID: identitySubject, Purpose: dv.Purpose("unsupported"), State: dv.CaseInReview},
			{ID: spaceNoIDCaseID, SubjectID: spaceSubject, Purpose: dv.PurposeSpace, State: dv.CaseInReview},
			{ID: "invalid-case-id", SubjectID: identitySubject, Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted},
			{ID: invalidCaseID, SubjectID: "invalid-subject-id", Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted},
			{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", SubjectID: identitySubject, Purpose: dv.PurposeIdentity, State: dv.CaseApproved},
		},
	}
	cases, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, cases, reviewerQueueProjectionStore{})
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, reviewerCasesRoute, nil)
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	NewReviewerCasesHandler(reviewerQueueProjectionAuth{}, access).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if repo.listCalls != 1 || repo.listActor != reviewerQueueProjectionActor {
		t.Fatalf("list calls=%d actor=%q, want one call for authenticated reviewer", repo.listCalls, repo.listActor)
	}
	if response.Header().Get("Cache-Control") != "private, no-store" ||
		response.Header().Get("Pragma") != "no-cache" ||
		response.Header().Get("Vary") != "Authorization" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("reviewer queue headers=%v", response.Header())
	}

	var result struct {
		Items []reviewerCaseDTO `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{identityCaseID, spaceCaseID}
	if len(result.Items) != len(wantIDs) {
		t.Fatalf("queue items=%+v, want only identity/space reviewable cases", result.Items)
	}
	for i, wantID := range wantIDs {
		if result.Items[i].ID != wantID {
			t.Fatalf("queue item %d id=%q, want %q", i, result.Items[i].ID, wantID)
		}
	}
	if strings.Contains(response.Body.String(), declarationID) {
		t.Fatalf("queue serialized identity declaration revision: %s", response.Body.String())
	}
}
