package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerQueueMapsCapabilityErrorToNeutralNotFound(t *testing.T) {
	const (
		caseID    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		subjectID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	snapshot := dv.Snapshot{
		ID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 4,
	}

	for _, tc := range []struct {
		name           string
		listErr        error
		wantStatus     int
		wantCaseInBody bool
	}{
		{name: "authorized queue control", wantStatus: http.StatusOK, wantCaseInBody: true},
		{name: "capability failure is concealed", listErr: dv.ErrCapability, wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &reviewerFake{x: snapshot, err: tc.listErr}
			response := httptest.NewRecorder()
			request := reviewerReq(http.MethodGet, reviewerCasesRoute, "")

			reviewerHandler(service, nil).ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%q, want %d", response.Code, response.Body.String(), tc.wantStatus)
			}
			if response.Header().Get("Cache-Control") != "private, no-store" ||
				response.Header().Get("X-Content-Type-Options") != "nosniff" ||
				response.Header().Get("Vary") != "Authorization" {
				t.Fatalf("queue response headers=%v", response.Header())
			}
			if tc.wantCaseInBody {
				if !strings.Contains(response.Body.String(), caseID) || !strings.Contains(response.Body.String(), `"items"`) {
					t.Fatalf("positive queue control body=%q, want the synthetic case", response.Body.String())
				}
			} else if response.Body.String() != "not found\n" || response.Header().Get("ETag") != "" ||
				strings.Contains(response.Body.String(), caseID) || strings.Contains(response.Body.String(), subjectID) || strings.Contains(response.Body.String(), `"items"`) {
				t.Fatalf("capability failure was not neutral: status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
			}
			if service.actor != reviewerActor || len(service.calls) != 1 || service.calls[0] != "list" {
				t.Fatalf("actor=%q calls=%v, want one list call for the authenticated actor", service.actor, service.calls)
			}
		})
	}
}
