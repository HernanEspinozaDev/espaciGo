package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityDetailDoesNotSerializePopulatedValueWithError(t *testing.T) {
	const (
		privateName = "Synthetic-Private-Name-7B2"
		privateLast = "Synthetic-Private-Surname-7B2"
		privateRUT  = "Synthetic-RUT-7B2"
		privateDOB  = "2040-02-03"
	)
	service := &reviewerFake{
		x: dv.Snapshot{
			ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 8,
			IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777",
			DocumentFormatDeclared:        dv.DocumentFormatNew,
		},
		declaration: map[string]string{
			"first_name": privateName, "last_name": privateLast,
			"rut": privateRUT, "date_of_birth": privateDOB,
		},
		err: dv.ErrDocumentStorage,
	}
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodGet, reviewerCasesRoute+"/"+reviewerID, "")
	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d body=%q, want neutral 503", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || len(service.calls) != 1 || service.calls[0] != "load_detail" {
		t.Fatalf("etag=%q service calls=%v, want no ETag and only failed detail load", response.Header().Get("ETag"), service.calls)
	}
	for _, sensitive := range []string{privateName, privateLast, privateRUT, privateDOB} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("partial detail leaked %q: %q", sensitive, response.Body.String())
		}
	}
}
