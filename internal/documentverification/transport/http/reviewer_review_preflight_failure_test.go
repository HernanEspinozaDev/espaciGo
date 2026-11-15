package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerPreflightLoadFailureService struct {
	*reviewerFake
	loadErr             error
	reviewIdentityCalls int
}

func (s *reviewerPreflightLoadFailureService) Load(_ context.Context, actor, _ string) (dv.Snapshot, error) {
	s.calls = append(s.calls, "load")
	s.actor = actor
	return s.x, s.loadErr
}

func (s *reviewerPreflightLoadFailureService) ReviewIdentity(ctx context.Context, actor, id string, decision dv.Decision, reason dv.ReasonCode, format dv.DocumentFormat, expiry time.Time, checklist *dv.IdentityReviewChecklist, until time.Time, revision uint64) error {
	s.reviewIdentityCalls++
	return s.reviewerFake.ReviewIdentity(ctx, actor, id, decision, reason, format, expiry, checklist, until, revision)
}

func TestReviewerIdentityDecisionStopsWhenPreflightLoadIsOutOfScope(t *testing.T) {
	service := &reviewerPreflightLoadFailureService{
		reviewerFake: &reviewerFake{
			x: dv.Snapshot{
				ID: reviewerID, SubjectID: "synthetic-private-subject",
				Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
				DocumentFormatDeclared: dv.DocumentFormatNew,
			},
		},
		loadErr: dv.ErrNotFound,
	}
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body)

	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || response.Body.String() != "not found\n" {
		t.Fatalf("status=%d body=%q, want neutral 404", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("preflight denial headers=%v, want no ETag and private no-store", response.Header())
	}
	if service.actor != reviewerActor || len(service.calls) != 1 || service.calls[0] != "load" || service.reviewIdentityCalls != 0 || len(service.reviewArgs) != 0 {
		t.Fatalf("actor=%q calls=%v reviewIdentityCalls=%d reviewArgs=%v; preflight denial must not mutate", service.actor, service.calls, service.reviewIdentityCalls, service.reviewArgs)
	}
	if strings.Contains(response.Body.String(), "synthetic-private-subject") {
		t.Fatalf("preflight response disclosed synthetic private subject: %q", response.Body.String())
	}
}
