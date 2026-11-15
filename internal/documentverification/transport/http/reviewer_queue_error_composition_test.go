package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerQueuePartialRepository struct {
	cases     []dv.Snapshot
	err       error
	actor     string
	listCalls int
	loadCalls int
	readCalls int
}

func (r *reviewerQueuePartialRepository) ListReviewerCases(_ context.Context, actor string) ([]dv.Snapshot, error) {
	r.listCalls++
	r.actor = actor
	return r.cases, r.err
}

func (r *reviewerQueuePartialRepository) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	r.loadCalls++
	return dv.Snapshot{}, dv.ErrNotFound
}

func (r *reviewerQueuePartialRepository) WithReviewerCaseForRead(context.Context, string, string, func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	r.readCalls++
	return dv.ErrNotFound
}

type reviewerQueuePartialStore struct {
	identityReads int
	spaceReads    int
}

func (s *reviewerQueuePartialStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.identityReads++
	return nil, dv.ErrNotFound
}

func (s *reviewerQueuePartialStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.spaceReads++
	return nil, dv.ErrNotFound
}

func TestReviewerQueueDiscardsPartialRepositoryResultsThroughRegisteredRoute(t *testing.T) {
	const (
		partialCaseID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab"
		subjectID     = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbc"
		errorDetail   = "synthetic reviewer query diagnostic E31"
	)
	repository := &reviewerQueuePartialRepository{
		cases: []dv.Snapshot{{
			ID: partialCaseID, SubjectID: subjectID, Purpose: dv.PurposeIdentity,
			State: dv.CaseInReview, Revision: 6, DocumentFormatDeclared: dv.DocumentFormatNew,
		}},
		err: errors.New(errorDetail),
	}
	cases, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerQueuePartialStore{}
	access, err := dv.NewReviewerAccessService(repository, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerQueueProjectionAuth{}, access))
	request := httptest.NewRequest(http.MethodGet, reviewerCasesRoute, nil)
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d body=%q, want neutral 503", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Cache-Control") != "private, no-store" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("failed queue headers=%v", response.Header())
	}
	for _, sensitive := range []string{partialCaseID, subjectID, errorDetail, `"items"`} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("partial reviewer queue leaked %q: %q", sensitive, response.Body.String())
		}
	}
	if repository.listCalls != 1 || repository.actor != reviewerQueueProjectionActor || repository.loadCalls != 0 || repository.readCalls != 0 {
		t.Fatalf("repository calls/list actor=%d/%q load=%d scoped-read=%d", repository.listCalls, repository.actor, repository.loadCalls, repository.readCalls)
	}
	if store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("partial queue error triggered document reads: identity=%d space=%d", store.identityReads, store.spaceReads)
	}
}
