package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func ownerIdentitySnapshot() documentverification.Snapshot {
	snapshot := casesSnapshot()
	snapshot.Purpose = documentverification.PurposeIdentity
	snapshot.SpaceID = ""
	snapshot.DocumentExpiresAt = time.Time{}
	snapshot.IdentityDeclarationRevisionID = "77777777-7777-4777-8777-777777777777"
	snapshot.CaptureConsentVersion = documentverification.IdentityCaptureConsentVersion
	snapshot.CaptureConsentedAt = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	snapshot.DocumentFormatDeclared = documentverification.DocumentFormatNew
	snapshot.Files = []documentverification.File{
		{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", EvidenceRole: documentverification.EvidenceRoleIdentityFront, State: documentverification.FileClean, Size: 12, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", EvidenceRole: documentverification.EvidenceRoleIdentityBack, State: documentverification.FileClean, Size: 13, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", EvidenceRole: documentverification.EvidenceRoleIdentitySelfie, State: documentverification.FileClean, Size: 14, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{3}},
	}
	return snapshot
}

type cancelAfterOwnerCaseReadService struct {
	OwnerCasesService
	cancel    context.CancelFunc
	snapshot  documentverification.Snapshot
	listCalls int
	loadCalls int
}

func (s *cancelAfterOwnerCaseReadService) ListOwnCases(context.Context, string) ([]documentverification.Snapshot, error) {
	s.listCalls++
	s.cancel()
	return []documentverification.Snapshot{s.snapshot}, nil
}

func (s *cancelAfterOwnerCaseReadService) LoadOwnCase(context.Context, string, string) (documentverification.Snapshot, error) {
	s.loadCalls++
	s.cancel()
	return s.snapshot, nil
}

func TestOwnerCaseReadHandlersDoNotSerializeAfterCancellation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		wantList   int
		wantDetail int
	}{
		{name: "list", path: ownerCasesRoute, wantList: 1},
		{name: "detail", path: ownerCasesRoute + "/" + ownerCasesID, wantDetail: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			service := &cancelAfterOwnerCaseReadService{cancel: cancel, snapshot: ownerIdentitySnapshot()}
			handler := NewOwnerCasesHandler(casesAuthFake{principal: identity.Principal{AccountID: casesActor}}, service)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer synthetic-owner-token")
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
			if response.Header().Get("ETag") != "" || strings.Contains(response.Body.String(), ownerCasesID) {
				t.Fatalf("canceled owner data was serialized: etag=%q body=%q", response.Header().Get("ETag"), response.Body.String())
			}
			if service.listCalls != tc.wantList || service.loadCalls != tc.wantDetail {
				t.Fatalf("service calls: list=%d detail=%d", service.listCalls, service.loadCalls)
			}
		})
	}
}

type cancelingOwnerSubmitService struct {
	OwnerCasesService
	cancel           context.CancelFunc
	snapshot         documentverification.Snapshot
	cancelOnLoadCall int
	cancelOnSubmit   bool
	loadCalls        int
	submitCalls      int
}

func (s *cancelingOwnerSubmitService) LoadOwnCase(context.Context, string, string) (documentverification.Snapshot, error) {
	s.loadCalls++
	if s.cancelOnLoadCall == s.loadCalls {
		s.cancel()
	}
	return s.snapshot, nil
}

func (s *cancelingOwnerSubmitService) Submit(context.Context, string, string, documentverification.Purpose, uint64) error {
	s.submitCalls++
	s.snapshot.State = documentverification.CaseSubmitted
	s.snapshot.Revision++
	if s.cancelOnSubmit {
		s.cancel()
	}
	return nil
}

func TestOwnerSubmitDoesNotReturnCaseAfterCancellation(t *testing.T) {
	for _, tc := range []struct {
		name             string
		initialState     documentverification.CaseState
		initialRevision  uint64
		ifMatch          string
		cancelOnLoadCall int
		cancelOnSubmit   bool
		wantLoadCalls    int
		wantSubmitCalls  int
	}{
		{
			name: "canceled stale-success replay read", initialState: documentverification.CaseSubmitted,
			initialRevision: 8, ifMatch: `"7"`, cancelOnLoadCall: 1, wantLoadCalls: 1,
		},
		{
			name: "mutation cancels before readback", initialState: documentverification.CaseDraft,
			initialRevision: 7, ifMatch: `"7"`, cancelOnSubmit: true, wantLoadCalls: 1, wantSubmitCalls: 1,
		},
		{
			name: "readback cancels before response", initialState: documentverification.CaseDraft,
			initialRevision: 7, ifMatch: `"7"`, cancelOnLoadCall: 2, wantLoadCalls: 2, wantSubmitCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			snapshot := ownerIdentitySnapshot()
			snapshot.State, snapshot.Revision = tc.initialState, tc.initialRevision
			service := &cancelingOwnerSubmitService{
				cancel: cancel, snapshot: snapshot,
				cancelOnLoadCall: tc.cancelOnLoadCall, cancelOnSubmit: tc.cancelOnSubmit,
			}
			handler := NewOwnerCasesHandler(casesAuthFake{principal: identity.Principal{AccountID: casesActor}}, service)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, ownerCasesRoute+"/"+ownerCasesID+"/submit", nil).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer synthetic-owner-token")
			request.Header.Set("If-Match", tc.ifMatch)
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" || response.Header().Get("ETag") != "" || strings.Contains(response.Body.String(), ownerCasesID) {
				t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
			}
			if ctx.Err() != context.Canceled || service.loadCalls != tc.wantLoadCalls || service.submitCalls != tc.wantSubmitCalls {
				t.Fatalf("canceled submit path: canceled=%v loads=%d submits=%d", ctx.Err() != nil, service.loadCalls, service.submitCalls)
			}
		})
	}
}
