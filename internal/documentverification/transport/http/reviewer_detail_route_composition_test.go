package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const reviewerDetailRouteSubject = "77777777-7777-4777-8777-777777777777"
const reviewerDetailRouteRevision = "88888888-8888-4888-8888-888888888888"

type reviewerDetailRouteRepository struct {
	snapshot      dv.Snapshot
	authorization dv.Authorization
	actors        []string
	caseIDs       []string
}

func (*reviewerDetailRouteRepository) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, dv.ErrNotFound
}

func (*reviewerDetailRouteRepository) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrNotFound
}

func (r *reviewerDetailRouteRepository) WithReviewerCaseForRead(ctx context.Context, actor, caseID string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	r.actors = append(r.actors, actor)
	r.caseIDs = append(r.caseIDs, caseID)
	if err := ctx.Err(); err != nil {
		return err
	}
	return read(ctx, r.snapshot, r.authorization)
}

type reviewerDetailRouteStore struct {
	identityReads int
	spaceReads    int
}

func (s *reviewerDetailRouteStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.identityReads++
	return nil, dv.ErrNotFound
}

func (s *reviewerDetailRouteStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.spaceReads++
	return nil, dv.ErrNotFound
}

func TestReviewerIdentityDetailRegisteredRouteComposesServiceAndAllowlistedDTO(t *testing.T) {
	const (
		firstName   = "SyntheticIdentityFirst"
		lastName    = "SyntheticIdentityLast"
		rut         = "11.111.111-1"
		dateOfBirth = "1980-01-02"
	)
	snapshot := dv.Snapshot{
		ID: reviewerID, SubjectID: reviewerDetailRouteSubject, Purpose: dv.PurposeIdentity,
		State: dv.CaseInReview, Revision: 5,
		IdentityDeclarationRevisionID: reviewerDetailRouteRevision,
		DocumentFormatDeclared:        dv.DocumentFormatNew,
	}
	repository := &reviewerDetailRouteRepository{
		snapshot: snapshot,
		authorization: dv.Authorization{
			ActorID: reviewerQueueProjectionActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerQueueProjectionActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerDetailRouteSubject,
			}},
		},
	}
	store := &reviewerDetailRouteStore{}
	declarationCalls := 0
	declarationAccount, declarationRevision := "", ""
	declarations := dv.ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, revisionID string) (dv.ReviewerIdentityDeclaration, error) {
		declarationCalls++
		declarationAccount, declarationRevision = accountID, revisionID
		return dv.ReviewerIdentityDeclaration{
			FirstName: firstName, LastName: lastName, RUT: rut, DateOfBirth: dateOfBirth,
		}, nil
	})
	cases, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessServiceWithIdentityDeclarationReader(repository, cases, store, declarations)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerQueueProjectionAuth{}, access))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, reviewerCasesRoute+"/"+reviewerID, nil)
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != `"5"` || response.Header().Get("Cache-Control") != "private, no-store" ||
		response.Header().Get("Pragma") != "no-cache" || response.Header().Get("Vary") != "Authorization" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("detail response headers=%v", response.Header())
	}
	if len(repository.actors) != 3 || len(repository.caseIDs) != 3 {
		t.Fatalf("repository reads actors=%v caseIDs=%v, want three scoped authorization/readback checks", repository.actors, repository.caseIDs)
	}
	for i := range repository.actors {
		if repository.actors[i] != reviewerQueueProjectionActor || repository.caseIDs[i] != reviewerID {
			t.Fatalf("repository read %d actor=%q case=%q", i, repository.actors[i], repository.caseIDs[i])
		}
	}
	if declarationCalls != 1 || declarationAccount != reviewerDetailRouteSubject || declarationRevision != reviewerDetailRouteRevision {
		t.Fatalf("declaration reads=%d account=%q revision=%q", declarationCalls, declarationAccount, declarationRevision)
	}
	if store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("detail read file store: identity=%d space=%d", store.identityReads, store.spaceReads)
	}

	var dto map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	declaration, ok := dto["identity_declaration"].(map[string]any)
	if !ok || declaration["first_name"] != firstName || declaration["last_name"] != lastName ||
		declaration["rut"] != rut || declaration["date_of_birth"] != dateOfBirth {
		t.Fatalf("allowlisted declaration=%v response=%q", declaration, response.Body.String())
	}
	if dto["id"] != reviewerID || dto["purpose"] != string(dv.PurposeIdentity) || dto["document_format_declared"] != string(dv.DocumentFormatNew) {
		t.Fatalf("reviewer DTO identity fields=%v", dto)
	}
	for _, forbidden := range []string{reviewerDetailRouteSubject, reviewerDetailRouteRevision, "identity_declaration_revision_id", "storage_key"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("private detail metadata leaked %q: %q", forbidden, response.Body.String())
		}
	}
}
