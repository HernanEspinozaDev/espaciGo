package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type reportAuthStub struct{ err error }

func (a reportAuthStub) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	if a.err != nil {
		return identity.Principal{}, a.err
	}
	return identity.Principal{AccountID: "admin"}, nil
}

func TestAdminLocalHandlerUsesCommonUnauthorizedAndForbiddenEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth error
		code int
		www  string
	}{{"unauthorized", identity.ErrUnauthorized, http.StatusUnauthorized, "Bearer"}, {"forbidden", identity.ErrForbidden, http.StatusForbidden, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(reportAuthStub{err: tc.auth}, nil, nil)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/local/reports/reservations?from=2026-10-01T00:00:00Z&until=2026-10-02T00:00:00Z&timezone=UTC", nil)
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.code || response.Header().Get("WWW-Authenticate") != tc.www {
				t.Fatalf("status/header=%d/%q want %d/%q", response.Code, response.Header().Get("WWW-Authenticate"), tc.code, tc.www)
			}
			var envelope struct {
				Error struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					RequestID string `json:"request_id"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Code == "" || envelope.Error.Message == "" || envelope.Error.RequestID == "" || envelope.Error.RequestID != response.Header().Get("X-Request-ID") {
				t.Fatalf("invalid common error envelope: %+v header=%q", envelope.Error, response.Header().Get("X-Request-ID"))
			}
		})
	}
}

func TestReportPeriodRequiresExplicitIANAAndCalendarBoundaries(t *testing.T) {
	valid := url.Values{"from": {"2026-10-01T00:00:00-03:00"}, "until": {"2026-11-01T00:00:00-03:00"}, "timezone": {"America/Santiago"}}
	period, ok := parsePeriod(valid)
	if !ok || period.TimeZone != "America/Santiago" {
		t.Fatalf("valid 31-calendar-day period rejected: %+v %v", period, ok)
	}
	invalidOffset := url.Values{"from": {"2026-10-01T00:00:00-04:00"}, "until": {"2026-11-01T00:00:00-03:00"}, "timezone": {"America/Santiago"}}
	if _, ok = parsePeriod(invalidOffset); ok {
		t.Fatal("RFC3339 offset inconsistent with IANA zone accepted")
	}
	tooLong := url.Values{"from": {"2026-10-01T00:00:00-03:00"}, "until": {"2026-11-01T00:00:01-03:00"}, "timezone": {"America/Santiago"}}
	if _, ok = parsePeriod(tooLong); ok {
		t.Fatal("period beyond 31 calendar days accepted")
	}
	missing := url.Values{"from": {"2026-10-01T00:00:00-03:00"}, "until": {"2026-10-02T00:00:00-03:00"}}
	if _, ok = parsePeriod(missing); ok {
		t.Fatal("period without IANA zone accepted")
	}
}
