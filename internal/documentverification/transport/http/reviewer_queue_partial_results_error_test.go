package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerQueueDoesNotSerializePartialResultsWhenListReturnsError(t *testing.T) {
	const (
		partialCaseID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		subjectID     = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	service := &reviewerFake{
		x: dv.Snapshot{
			ID: partialCaseID, SubjectID: subjectID,
			Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 4,
			DocumentFormatDeclared: dv.DocumentFormatNew,
		},
		err: dv.ErrDocumentStorage,
	}
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodGet, reviewerCasesRoute, "")

	reviewerHandler(service, nil).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d body=%q, want neutral 503", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Cache-Control") != "private, no-store" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("failed queue headers=%v", response.Header())
	}
	if strings.Contains(response.Body.String(), partialCaseID) || strings.Contains(response.Body.String(), subjectID) || strings.Contains(response.Body.String(), `"items"`) {
		t.Fatalf("partial reviewer queue leaked in error response: %q", response.Body.String())
	}
	if service.actor != reviewerActor || len(service.calls) != 1 || service.calls[0] != "list" {
		t.Fatalf("actor=%q calls=%v, want exactly one authenticated list", service.actor, service.calls)
	}
}
