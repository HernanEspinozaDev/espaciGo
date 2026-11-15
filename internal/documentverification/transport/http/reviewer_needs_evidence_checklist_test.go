package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityNeedsEvidenceRejectsMissingOrIncompleteChecklist(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{
			name: "omitted checklist",
			body: `{"decision":"needs_evidence","reason_code":"antecedentes_incompletos","expected_revision":7,"document_format_observed":"new"}`,
		},
		{
			name: "null checklist",
			body: `{"decision":"needs_evidence","reason_code":"antecedentes_incompletos","expected_revision":7,"document_format_observed":"new","identity_review_checklist":null}`,
		},
		{
			name: "empty checklist",
			body: `{"decision":"needs_evidence","reason_code":"antecedentes_incompletos","expected_revision":7,"document_format_observed":"new","identity_review_checklist":{}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &reviewerFake{x: dv.Snapshot{
				ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
				Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted, Revision: 7,
				DocumentFormatDeclared: dv.DocumentFormatNew,
			}}
			response := httptest.NewRecorder()
			request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", test.body)
			NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest || response.Body.String() != "invalid request\n" {
				t.Fatalf("status=%d body=%q, want invalid request without panic", response.Code, response.Body.String())
			}
			if len(service.calls) != 1 || service.calls[0] != "load" {
				t.Fatalf("service calls=%v, want only the initial case load and no decision/readback", service.calls)
			}
		})
	}
}
