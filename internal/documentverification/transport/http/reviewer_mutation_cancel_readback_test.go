package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerCancelAfterMutationService struct {
	ReviewerCasesService
	snapshot            dv.Snapshot
	cancel              context.CancelFunc
	cancelOnLoad        bool
	cancelOnLoadCall    int
	cancelAfterMutation bool
	loadCalls           int
	detailCalls         int
	reviewIdentityCalls int
	revokeCalls         int
}

func (s *reviewerCancelAfterMutationService) Load(context.Context, string, string) (dv.Snapshot, error) {
	s.loadCalls++
	if s.cancelOnLoad || s.cancelOnLoadCall == s.loadCalls {
		s.cancel()
	}
	return s.snapshot, nil
}

func (s *reviewerCancelAfterMutationService) LoadDetail(context.Context, string, string) (dv.ReviewerCaseDetail, error) {
	s.detailCalls++
	return dv.ReviewerCaseDetail{Snapshot: s.snapshot}, nil
}

func (s *reviewerCancelAfterMutationService) ReviewIdentity(context.Context, string, string, dv.Decision, dv.ReasonCode, dv.DocumentFormat, time.Time, *dv.IdentityReviewChecklist, time.Time, uint64) error {
	s.reviewIdentityCalls++
	s.snapshot.State = dv.CaseApproved
	s.snapshot.Revision++
	if s.cancelAfterMutation {
		s.cancel()
	}
	return nil
}

func (s *reviewerCancelAfterMutationService) Revoke(context.Context, string, string, dv.Purpose, dv.ReasonCode, uint64) error {
	s.revokeCalls++
	s.snapshot.State = dv.CaseRevoked
	s.snapshot.Revision++
	if s.cancelAfterMutation {
		s.cancel()
	}
	return nil
}

func TestReviewerMutationCancellationSkipsReadbackAndRequiresExplicitGet(t *testing.T) {
	const (
		ownerID               = "88888888-8888-4888-8888-888888888888"
		declarationRevisionID = "77777777-7777-4777-8777-777777777777"
	)
	cases := []struct {
		name                 string
		path                 string
		body                 string
		initialState         dv.CaseState
		wantState            dv.CaseState
		wantInitialLoadCalls int
		wantReviewIdentity   int
		wantRevoke           int
	}{
		{
			name: "identity decision", path: reviewerCasesRoute + "/" + reviewerID + "/review",
			body:         `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`,
			initialState: dv.CaseSubmitted, wantState: dv.CaseApproved, wantInitialLoadCalls: 1, wantReviewIdentity: 1,
		},
		{
			name: "identity revocation", path: reviewerCasesRoute + "/" + reviewerID + "/revoke",
			body:         `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`,
			initialState: dv.CaseApproved, wantState: dv.CaseRevoked, wantRevoke: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			service := &reviewerCancelAfterMutationService{
				snapshot: dv.Snapshot{
					ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
					State: tc.initialState, Revision: 7,
					IdentityDeclarationRevisionID: declarationRevisionID,
					DocumentFormatDeclared:        dv.DocumentFormatNew,
				},
				cancel: cancel, cancelAfterMutation: true,
			}
			handler := NewReviewerCasesHandler(reviewerAuth{}, service)
			response := httptest.NewRecorder()
			request := reviewerReq(http.MethodPost, tc.path, tc.body).WithContext(ctx)
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" || response.Header().Get("ETag") != "" {
				t.Fatalf("POST status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
			}
			if service.loadCalls != tc.wantInitialLoadCalls || service.reviewIdentityCalls != tc.wantReviewIdentity || service.revokeCalls != tc.wantRevoke || service.snapshot.State != tc.wantState || service.snapshot.Revision != 8 {
				t.Fatalf("mutation/readback state: loads=%d review=%d revoke=%d snapshot=%+v", service.loadCalls, service.reviewIdentityCalls, service.revokeCalls, service.snapshot)
			}

			getResponse := httptest.NewRecorder()
			getRequest := reviewerReq(http.MethodGet, reviewerCasesRoute+"/"+reviewerID, "")
			handler.ServeHTTP(getResponse, getRequest)
			if getResponse.Code != http.StatusOK || getResponse.Header().Get("ETag") != `"8"` || service.detailCalls != 1 {
				t.Fatalf("explicit GET status=%d etag=%q detail calls=%d body=%q", getResponse.Code, getResponse.Header().Get("ETag"), service.detailCalls, getResponse.Body.String())
			}
			var reconciled reviewerCaseDTO
			if err := json.Unmarshal(getResponse.Body.Bytes(), &reconciled); err != nil {
				t.Fatal(err)
			}
			if reconciled.State != tc.wantState || reconciled.Revision != 8 {
				t.Fatalf("explicit GET reconciled state=%q revision=%d, want %q/8", reconciled.State, reconciled.Revision, tc.wantState)
			}
			if service.loadCalls != tc.wantInitialLoadCalls {
				t.Fatalf("explicit GET performed an implicit load: calls=%d want=%d", service.loadCalls, tc.wantInitialLoadCalls)
			}
		})
	}
}

func TestReviewerIdentityDecisionDoesNotMutateWhenPreflightLoadCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service := &reviewerCancelAfterMutationService{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted, Revision: 7,
			IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777",
			DocumentFormatDeclared:        dv.DocumentFormatNew,
		},
		cancel: cancel, cancelOnLoad: true,
	}
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body).WithContext(ctx)
	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("ETag") != "" || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	if service.loadCalls != 1 || service.reviewIdentityCalls != 0 || service.snapshot.State != dv.CaseSubmitted || service.snapshot.Revision != 7 {
		t.Fatalf("canceled preflight performed mutation: loads=%d reviews=%d snapshot=%+v", service.loadCalls, service.reviewIdentityCalls, service.snapshot)
	}
}

func TestReviewerRevocationDoesNotMutateWhenRequestAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := &reviewerCancelAfterMutationService{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeIdentity, State: dv.CaseApproved, Revision: 7,
		},
		cancel: cancel,
	}
	body := `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", body).WithContext(ctx)
	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("ETag") != "" || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	if service.revokeCalls != 0 || service.snapshot.State != dv.CaseApproved || service.snapshot.Revision != 7 {
		t.Fatalf("pre-canceled request performed revocation: revokes=%d snapshot=%+v", service.revokeCalls, service.snapshot)
	}
}

func TestReviewerMutationReadbackDoesNotSerializeWhenLoadCancels(t *testing.T) {
	cases := []struct {
		name             string
		path             string
		body             string
		initialState     dv.CaseState
		wantState        dv.CaseState
		cancelOnLoadCall int
		wantLoadCalls    int
		wantReviewCalls  int
		wantRevokeCalls  int
	}{
		{
			name: "identity decision", path: reviewerCasesRoute + "/" + reviewerID + "/review",
			body:         `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`,
			initialState: dv.CaseSubmitted, wantState: dv.CaseApproved, cancelOnLoadCall: 2,
			wantLoadCalls: 2, wantReviewCalls: 1,
		},
		{
			name: "identity revocation", path: reviewerCasesRoute + "/" + reviewerID + "/revoke",
			body:         `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`,
			initialState: dv.CaseApproved, wantState: dv.CaseRevoked, cancelOnLoadCall: 1,
			wantLoadCalls: 1, wantRevokeCalls: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			service := &reviewerCancelAfterMutationService{
				snapshot: dv.Snapshot{
					ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
					Purpose: dv.PurposeIdentity, State: tc.initialState, Revision: 7,
					IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777",
					DocumentFormatDeclared:        dv.DocumentFormatNew,
				},
				cancel: cancel, cancelOnLoadCall: tc.cancelOnLoadCall,
			}
			response := httptest.NewRecorder()
			request := reviewerReq(http.MethodPost, tc.path, tc.body).WithContext(ctx)
			NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Header().Get("ETag") != "" || response.Body.String() != "temporarily unavailable\n" {
				t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
			}
			if ctx.Err() != context.Canceled || service.loadCalls != tc.wantLoadCalls || service.reviewIdentityCalls != tc.wantReviewCalls || service.revokeCalls != tc.wantRevokeCalls || service.snapshot.State != tc.wantState || service.snapshot.Revision != 8 {
				t.Fatalf("mutation/readback state: canceled=%v loads=%d review=%d revoke=%d snapshot=%+v", ctx.Err() != nil, service.loadCalls, service.reviewIdentityCalls, service.revokeCalls, service.snapshot)
			}
		})
	}
}
