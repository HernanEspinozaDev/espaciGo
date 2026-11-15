package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityDetailDiscardsPartialDeclarationThroughRegisteredRoute(t *testing.T) {
	const (
		privateName = "SyntheticPartialIdentityNameF91"
		privateLast = "SyntheticPartialIdentitySurnameF91"
		privateRUT  = "SyntheticPartialIdentityRUTF91"
		privateDOB  = "2041-06-07"
		errorDetail = "synthetic declaration backend diagnostic F91"
	)
	now := time.Date(2030, 2, 3, 12, 0, 0, 0, time.UTC)
	repository := &reviewerDetailRouteRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: reviewerDetailRouteSubject, Purpose: dv.PurposeIdentity,
			State: dv.CaseInReview, Revision: 5,
			IdentityDeclarationRevisionID: reviewerDetailRouteRevision,
			DocumentFormatDeclared:        dv.DocumentFormatNew,
		},
		authorization: dv.Authorization{
			ActorID: reviewerQueueProjectionActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerQueueProjectionActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerDetailRouteSubject,
			}},
		},
	}
	declarationReads := 0
	declarationAccount, declarationRevision := "", ""
	declarations := dv.ReviewerIdentityDeclarationReaderFunc(func(ctx context.Context, accountID, revisionID string) (dv.ReviewerIdentityDeclaration, error) {
		declarationReads++
		declarationAccount, declarationRevision = accountID, revisionID
		if err := ctx.Err(); err != nil {
			return dv.ReviewerIdentityDeclaration{}, err
		}
		return dv.ReviewerIdentityDeclaration{
			FirstName: privateName, LastName: privateLast, RUT: privateRUT, DateOfBirth: privateDOB,
		}, errors.New(errorDetail)
	})
	cases, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerDetailRouteStore{}
	access, err := dv.NewReviewerAccessServiceWithIdentityDeclarationReader(repository, cases, store, declarations)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return now })
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerQueueProjectionAuth{}, access))
	request := httptest.NewRequest(http.MethodGet, reviewerCasesRoute+"/"+reviewerID, nil)
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d body=%q, want neutral 503", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Content-Disposition") != "" ||
		response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("partial detail response headers=%v", response.Header())
	}
	for _, sensitive := range []string{privateName, privateLast, privateRUT, privateDOB, errorDetail, "identity_declaration"} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("partial declaration leaked %q: %q", sensitive, response.Body.String())
		}
	}
	if declarationReads != 1 || declarationAccount != reviewerDetailRouteSubject || declarationRevision != reviewerDetailRouteRevision {
		t.Fatalf("declaration reads=%d account=%q revision=%q", declarationReads, declarationAccount, declarationRevision)
	}
	if len(repository.actors) == 0 || len(repository.actors) != len(repository.caseIDs) {
		t.Fatalf("repository authorization/read counts actors=%v caseIDs=%v", repository.actors, repository.caseIDs)
	}
	for i := range repository.actors {
		if repository.actors[i] != reviewerQueueProjectionActor || repository.caseIDs[i] != reviewerID {
			t.Fatalf("repository read %d actor=%q case=%q", i, repository.actors[i], repository.caseIDs[i])
		}
	}
	if store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("partial declaration error triggered evidence reads: identity=%d space=%d", store.identityReads, store.spaceReads)
	}
}
