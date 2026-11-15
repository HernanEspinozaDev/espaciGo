package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerQueueRoutesRegisterCollectionAndTrailingSlash(t *testing.T) {
	const (
		caseID    = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
		subjectID = "11111111-1111-4111-8111-111111111111"
	)
	repo := &reviewerQueueProjectionRepo{
		listed: []dv.Snapshot{{
			ID: caseID, SubjectID: subjectID, Purpose: dv.PurposeIdentity,
			State: dv.CaseSubmitted, Revision: 3,
		}},
	}
	cases, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, cases, reviewerQueueProjectionStore{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerQueueProjectionAuth{}, access))

	for i, path := range []string{reviewerCasesRoute, reviewerCasesRoute + "/"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
		mux.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("path=%q status=%d body=%q", path, response.Code, response.Body.String())
		}
		if repo.listCalls != i+1 || repo.listActor != reviewerQueueProjectionActor {
			t.Fatalf("path=%q list calls=%d actor=%q, want %d for authenticated reviewer", path, repo.listCalls, repo.listActor, i+1)
		}
		if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Vary") != "Authorization" {
			t.Fatalf("path=%q response headers=%v", path, response.Header())
		}
		var result struct {
			Items []reviewerCaseDTO `json:"items"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Items) != 1 || result.Items[0].ID != caseID {
			t.Fatalf("path=%q items=%+v, want case %q", path, result.Items, caseID)
		}
	}
}
