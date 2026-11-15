package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerCancelAfterQueueService struct {
	ReviewerCasesService
	cancel context.CancelFunc
	cases  []dv.Snapshot
}

func (s reviewerCancelAfterQueueService) List(context.Context, string) ([]dv.Snapshot, error) {
	s.cancel()
	return s.cases, nil
}

func TestReviewerQueueHandlerDoesNotSerializeAfterCancellation(t *testing.T) {
	const privateCaseID = "99999999-9999-4999-8999-999999999999"
	ctx, cancel := context.WithCancel(context.Background())
	service := reviewerCancelAfterQueueService{
		cancel: cancel,
		cases: []dv.Snapshot{{
			ID: privateCaseID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 4,
		}},
	}
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodGet, reviewerCasesRoute, "").WithContext(ctx)

	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), privateCaseID) || strings.Contains(response.Body.String(), `"items"`) {
		t.Fatalf("status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
	if response.Header().Get("Content-Type") != "text/plain; charset=utf-8" || response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("canceled queue security headers=%v", response.Header())
	}
}
