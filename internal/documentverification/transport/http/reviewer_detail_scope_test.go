package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerDetailRejectsOutOfScopeIdentityDeclarationBeforeSerialization(t *testing.T) {
	const (
		privateName = "SyntheticPrivateFirstName"
		privateLast = "SyntheticPrivateLastName"
		privateRUT  = "11.111.111-1"
		privateDOB  = "1980-01-02"
	)
	base := dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 8,
		IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777",
		DocumentFormatDeclared:        dv.DocumentFormatNew,
	}
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		mutate     func(*dv.Snapshot)
		wantStatus int
		wantETag   bool
	}{
		{name: "terminal state", mutate: func(x *dv.Snapshot) { x.State = dv.CaseApproved }, wantStatus: http.StatusNotFound},
		{name: "self review", mutate: func(x *dv.Snapshot) { x.SubjectID = reviewerActor }, wantStatus: http.StatusNotFound},
		{name: "wrong purpose", mutate: func(x *dv.Snapshot) { x.Purpose = dv.PurposeSpace }, wantStatus: http.StatusOK, wantETag: true},
		{name: "revoked", mutate: func(x *dv.Snapshot) { x.RevokedAt = &now }, wantStatus: http.StatusNotFound},
		{name: "superseded", mutate: func(x *dv.Snapshot) { x.SupersededAt = &now }, wantStatus: http.StatusNotFound},
		{name: "missing declaration revision", mutate: func(x *dv.Snapshot) { x.IdentityDeclarationRevisionID = "" }, wantStatus: http.StatusNotFound},
		{name: "undetermined is not a declared format", mutate: func(x *dv.Snapshot) { x.DocumentFormatDeclared = dv.DocumentFormatUndetermined }, wantStatus: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := base
			tc.mutate(&snapshot)
			service := &reviewerFake{
				x: snapshot,
				declaration: map[string]string{
					"first_name": privateName, "last_name": privateLast,
					"rut": privateRUT, "date_of_birth": privateDOB,
				},
			}
			response := httptest.NewRecorder()
			path := reviewerCasesRoute + "/" + reviewerID
			reviewerHandler(service, nil).ServeHTTP(response, reviewerReq(http.MethodGet, path, ""))

			if response.Code != tc.wantStatus || (response.Header().Get("ETag") != "") != tc.wantETag {
				t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
			}
			for _, sensitive := range []string{privateName, privateLast, privateRUT, privateDOB} {
				if strings.Contains(response.Body.String(), sensitive) {
					t.Fatalf("out-of-scope detail leaked %q: %q", sensitive, response.Body.String())
				}
			}
			if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
				t.Fatalf("out-of-scope response security headers=%v", response.Header())
			}
		})
	}
}

func TestReviewerDetailAllowsLegacyAndUnsureDeclaredFormats(t *testing.T) {
	for _, format := range []dv.DocumentFormat{dv.DocumentFormatLegacy, dv.DocumentFormatUnsure} {
		t.Run(string(format), func(t *testing.T) {
			service := &reviewerFake{
				x: dv.Snapshot{
					ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
					Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 8,
					IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777",
					DocumentFormatDeclared:        format,
				},
				declaration: map[string]string{
					"first_name": "Synthetic", "last_name": "Reviewer",
					"rut": "11.111.111-1", "date_of_birth": "1980-01-02",
				},
			}
			response := httptest.NewRecorder()
			path := reviewerCasesRoute + "/" + reviewerID
			reviewerHandler(service, nil).ServeHTTP(response, reviewerReq(http.MethodGet, path, ""))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"identity_declaration"`) {
				t.Fatalf("format=%q status=%d body=%q", format, response.Code, response.Body.String())
			}
		})
	}
}
