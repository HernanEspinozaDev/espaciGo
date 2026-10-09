package contracthttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/contract"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type corsAuth struct{}

func (corsAuth) Authorize(_ context.Context, raw identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if raw == "" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	return identity.Principal{AccountID: "participant"}, nil
}

type corsContracts struct{ item contract.Contract }

func (r corsContracts) Create(context.Context, string, string, func() time.Time) (contract.Contract, error) {
	return contract.Contract{}, errors.New("unexpected create")
}
func (r corsContracts) Get(context.Context, string, string) (contract.Contract, error) {
	return r.item, nil
}
func (r corsContracts) Sign(context.Context, string, string, func() time.Time) (contract.Contract, error) {
	return contract.Contract{}, errors.New("unexpected sign")
}
func (r corsContracts) Reject(context.Context, string, string, string, func() time.Time) (contract.Contract, error) {
	return contract.Contract{}, errors.New("unexpected reject")
}
func (r corsContracts) EnsureApproved(context.Context, func() time.Time) (int, error) { return 0, nil }
func (r corsContracts) ExpireDue(context.Context, func() time.Time) (int, error)      { return 0, nil }

func TestContractRoutesApplyCORSOnErrorsPreflightAndDocumentDownload(t *testing.T) {
	const origin = "http://localhost:8081"
	service, err := contract.NewService(corsContracts{item: contract.Contract{State: "firmado", Artifact: []byte("%PDF-synthetic")}}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(corsAuth{}, service, []string{origin})

	errorRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/contracts/id", nil)
	errorRequest.Header.Set("Origin", origin)
	errorResponse := httptest.NewRecorder()
	handler.ServeHTTP(errorResponse, errorRequest)
	if errorResponse.Code != http.StatusUnauthorized || errorResponse.Header().Get("Access-Control-Allow-Origin") != origin || errorResponse.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("allowed-origin auth error missing CORS: status=%d headers=%v", errorResponse.Code, errorResponse.Header())
	}

	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/local/booking-trial/contracts/id/document", nil)
	preflight.Header.Set("Origin", origin)
	preflightResponse := httptest.NewRecorder()
	handler.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusNoContent || preflightResponse.Header().Get("Access-Control-Allow-Origin") != origin || preflightResponse.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" || preflightResponse.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("contract preflight CORS=%d headers=%v", preflightResponse.Code, preflightResponse.Header())
	}

	download := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/contracts/id/document", nil)
	download.Header.Set("Origin", origin)
	download.Header.Set("Authorization", "Bearer synthetic")
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, download)
	if downloadResponse.Code != http.StatusOK || downloadResponse.Header().Get("Access-Control-Allow-Origin") != origin || downloadResponse.Header().Get("Content-Type") != "application/pdf" || downloadResponse.Body.String() != "%PDF-synthetic" {
		t.Fatalf("document CORS/download status=%d headers=%v body=%q", downloadResponse.Code, downloadResponse.Header(), downloadResponse.Body.String())
	}

	denied := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/contracts/id", nil)
	denied.Header.Set("Origin", "https://untrusted.example")
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden || deniedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unconfigured origin should remain blocked: status=%d headers=%v", deniedResponse.Code, deniedResponse.Header())
	}
}
