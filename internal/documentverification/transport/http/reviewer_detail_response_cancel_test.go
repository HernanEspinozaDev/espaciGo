package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerCancelAfterDetailService struct {
	ReviewerCasesService
	cancel context.CancelFunc
	detail dv.ReviewerCaseDetail
}

func (s reviewerCancelAfterDetailService) LoadDetail(context.Context, string, string) (dv.ReviewerCaseDetail, error) {
	s.cancel()
	return s.detail, nil
}

func TestReviewerIdentityDetailHandlerDoesNotSerializeAfterCancellation(t *testing.T) {
	const (
		privateName = "SyntheticPrivateFirstName"
		privateLast = "SyntheticPrivateLastName"
		privateRUT  = "11.111.111-1"
		privateDOB  = "1980-01-02"
	)
	ctx, cancel := context.WithCancel(context.Background())
	service := reviewerCancelAfterDetailService{
		cancel: cancel,
		detail: dv.ReviewerCaseDetail{
			Snapshot: dv.Snapshot{
				ID: reviewerID, SubjectID: "77777777-7777-4777-8777-777777777777",
				Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 8,
			},
			IdentityDeclaration: &dv.ReviewerIdentityDeclaration{
				FirstName: privateName, LastName: privateLast, RUT: privateRUT, DateOfBirth: privateDOB,
			},
		},
	}
	path := reviewerCasesRoute + "/" + reviewerID
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodGet, path, "").WithContext(ctx)

	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	for _, sensitive := range []string{privateName, privateLast, privateRUT, privateDOB} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("canceled detail leaked %q: headers=%v body=%q", sensitive, response.Header(), response.Body.String())
		}
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("canceled detail response headers=%v", response.Header())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("canceled detail security headers=%v", response.Header())
	}
}
