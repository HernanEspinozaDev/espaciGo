package identityhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type restrictedTestAuth struct{ principal identity.Principal }

func (a restrictedTestAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return a.principal, nil
}

func TestRestrictedAccountMiddlewareAllowsOnlyExistingReservationPaths(t *testing.T) {
	principal := identity.Principal{AccountID: "acct", RestrictedMode: true}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/reservations/") && !identity.HasRestrictedReservationAccess(r.Context(), "acct") {
			t.Errorf("allow-listed reservation request lacked restricted-access context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	const origin = "http://localhost:8081"
	middleware := RestrictedAccountMiddleware(restrictedTestAuth{principal}, next, origin)
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"GET", "/api/v1/local/booking-trial/reservations", 204}, {"POST", "/api/v1/local/booking-trial/reservations", 403}, {"POST", "/api/v1/local/booking-trial/quotes", 403}, {"GET", "/api/v1/admin/local/reports/finance", 403}, {"POST", "/api/v1/local/booking-trial/reservations/id/payment", 204}, {"POST", "/api/v1/local/booking-trial/reservations/id/financial-decision", 403}, {"GET", "/api/v1/local/booking-trial/disputes/id/history", 204}, {"GET", "/api/v1/local/booking-trial/reservations/id/evidence/file", 204}, {"POST", "/api/v1/local/booking-trial/disputes/id/history", 403}, {"GET", "/api/v1/local/booking-trial/reservations/id/operations/evidence", 403}, {"GET", "/api/v1/admin/disputes", 403}, {"POST", "/api/v1/auth/logout", 204}} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", "Bearer local-token")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %s status=%d want=%d", tc.method, tc.path, w.Code, tc.want)
		}
		if w.Code == 403 && w.Header().Get("X-Request-ID") == "" {
			t.Errorf("restricted error omitted correlation id for %s", tc.path)
		}
		if w.Code == 403 && w.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("restricted error omitted allowed-origin CORS for %s: %v", tc.path, w.Header())
		}
		if w.Code == 403 && !strings.Contains(w.Body.String(), "reservas existentes") {
			t.Errorf("restricted error was not legible for mock: %s", w.Body.String())
		}
	}
	deniedOrigin := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/quotes", nil)
	deniedOrigin.Header.Set("Authorization", "Bearer local-token")
	deniedOrigin.Header.Set("Origin", "https://untrusted.invalid")
	deniedResponse := httptest.NewRecorder()
	middleware.ServeHTTP(deniedResponse, deniedOrigin)
	if deniedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("untrusted origin received CORS header: %v", deniedResponse.Header())
	}
}
