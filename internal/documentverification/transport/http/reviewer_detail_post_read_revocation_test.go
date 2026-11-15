package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const (
	reviewerDetailRevocationActor    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	reviewerDetailRevocationCase     = "66666666-6666-4666-8666-666666666666"
	reviewerDetailRevocationSubject  = "77777777-7777-4777-8777-777777777777"
	reviewerDetailRevocationRevision = "88888888-8888-4888-8888-888888888888"
	reviewerDetailRevocationOther    = "99999999-9999-4999-8999-999999999999"
)

type reviewerDetailPostReadRevocationAuth struct{}

func (reviewerDetailPostReadRevocationAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: reviewerDetailRevocationActor}, nil
}

type reviewerDetailPostReadRevocationDomainRepo struct{}

func (reviewerDetailPostReadRevocationDomainRepo) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}

func (reviewerDetailPostReadRevocationDomainRepo) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrCapability
}

type reviewerDetailPostReadRevocationRepository struct {
	snapshot        dv.Snapshot
	authorization   dv.Authorization
	readCalls       int
	actors          []string
	caseIDs         []string
	revokeAfterRead bool
}

func (*reviewerDetailPostReadRevocationRepository) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, dv.ErrNotFound
}

func (*reviewerDetailPostReadRevocationRepository) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrNotFound
}

func (r *reviewerDetailPostReadRevocationRepository) WithReviewerCaseForRead(ctx context.Context, actor, caseID string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	r.readCalls++
	r.actors = append(r.actors, actor)
	r.caseIDs = append(r.caseIDs, caseID)
	if err := ctx.Err(); err != nil {
		return err
	}
	authorization := r.authorization
	err := read(ctx, r.snapshot, authorization)
	if err == nil && r.revokeAfterRead {
		kept := make([]dv.ReviewerGrant, 0, len(r.authorization.ReviewerGrants))
		for _, grant := range r.authorization.ReviewerGrants {
			if grant.ReviewerID == reviewerDetailRevocationActor && grant.Capability == dv.CapabilityIdentity && grant.ScopeID == reviewerDetailRevocationSubject {
				continue
			}
			kept = append(kept, grant)
		}
		r.authorization.ReviewerGrants = kept
		r.revokeAfterRead = false
	}
	return err
}

type reviewerDetailPostReadRevocationStore struct {
	identityReads int
	spaceReads    int
}

func (s *reviewerDetailPostReadRevocationStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.identityReads++
	return nil, dv.ErrNotFound
}

func (s *reviewerDetailPostReadRevocationStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.spaceReads++
	return nil, dv.ErrNotFound
}

func TestReviewerIdentityDetailSuppressesDeclarationWhenGrantRevokedAfterRead(t *testing.T) {
	const (
		firstName   = "SyntheticPrivateFirstName"
		lastName    = "SyntheticPrivateLastName"
		rut         = "11.111.111-1"
		dateOfBirth = "1980-01-02"
	)
	snapshot := dv.Snapshot{
		ID: reviewerDetailRevocationCase, SubjectID: reviewerDetailRevocationSubject,
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 5,
		IdentityDeclarationRevisionID: reviewerDetailRevocationRevision,
		DocumentFormatDeclared:        dv.DocumentFormatNew,
	}
	repository := &reviewerDetailPostReadRevocationRepository{
		snapshot: snapshot,
		authorization: dv.Authorization{
			ActorID: reviewerDetailRevocationActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerDetailRevocationActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerDetailRevocationSubject,
			}, {
				ReviewerID: reviewerDetailRevocationActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerDetailRevocationOther,
			}},
		},
	}
	store := &reviewerDetailPostReadRevocationStore{}
	declarationReads := 0
	declarations := dv.ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, revisionID string) (dv.ReviewerIdentityDeclaration, error) {
		declarationReads++
		if accountID != reviewerDetailRevocationSubject || revisionID != reviewerDetailRevocationRevision {
			t.Fatalf("declaration read account=%q revision=%q", accountID, revisionID)
		}
		repository.revokeAfterRead = true
		return dv.ReviewerIdentityDeclaration{
			FirstName: firstName, LastName: lastName, RUT: rut, DateOfBirth: dateOfBirth,
		}, nil
	})
	cases, err := dv.NewService(reviewerDetailPostReadRevocationDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessServiceWithIdentityDeclarationReader(repository, cases, store, declarations)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerDetailPostReadRevocationAuth{}, access))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, reviewerCasesRoute+"/"+reviewerDetailRevocationCase, nil)
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || strings.TrimSpace(response.Body.String()) != "not found" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("response headers=%v", response.Header())
	}
	for _, privateValue := range []string{firstName, lastName, rut, dateOfBirth, reviewerDetailRevocationRevision} {
		if strings.Contains(response.Body.String(), privateValue) {
			t.Fatalf("response leaked %q: %q", privateValue, response.Body.String())
		}
	}
	if repository.readCalls != 3 || declarationReads != 1 || len(repository.actors) != 3 || len(repository.caseIDs) != 3 {
		t.Fatalf("scoped reads=%d actors=%v caseIDs=%v declaration reads=%d, want three scoped reads and one declaration read", repository.readCalls, repository.actors, repository.caseIDs, declarationReads)
	}
	for i := range repository.actors {
		if repository.actors[i] != reviewerDetailRevocationActor || repository.caseIDs[i] != reviewerDetailRevocationCase {
			t.Fatalf("scoped read %d actor=%q case=%q", i, repository.actors[i], repository.caseIDs[i])
		}
	}
	if len(repository.authorization.ReviewerGrants) != 1 ||
		repository.authorization.ReviewerGrants[0].ReviewerID != reviewerDetailRevocationActor ||
		repository.authorization.ReviewerGrants[0].Capability != dv.CapabilityIdentity ||
		repository.authorization.ReviewerGrants[0].ScopeID != reviewerDetailRevocationOther {
		t.Fatalf("post-revocation grants=%+v, want only unrelated identity grant for another scope", repository.authorization.ReviewerGrants)
	}
	if store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("document reads identity=%d space=%d, want zero", store.identityReads, store.spaceReads)
	}
}
