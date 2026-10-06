package bookinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const renterID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type authStub struct{}

func (authStub) Authorize(_ context.Context, raw identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if raw != "test-session" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	return identity.Principal{AccountID: renterID}, nil
}

type repoStub struct{ fixture booking.Fixture }

func (r repoStub) Fixture(context.Context, string) (booking.Fixture, error) { return r.fixture, nil }
func (repoStub) Quote(_ context.Context, _ string, id string, start, end, created, expires time.Time) (booking.Quote, error) {
	return booking.Quote{ID: id, SpaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", RateVersion: 1, RateUnit: "hora", UnitPrice: 8000, Currency: "CLP", Units: 1, Subtotal: 8000, StartAt: start, EndAt: end, TimeZone: "America/Santiago", Conditions: "Reglas sintéticas", CreatedAt: created, ExpiresAt: expires}, nil
}
func (repoStub) Create(context.Context, string, string, string, []byte, string, string, time.Time, time.Time) (booking.Reservation, error) {
	return booking.Reservation{}, nil
}
func (repoStub) Get(context.Context, string, string) (booking.Detail, error) {
	return booking.Detail{}, nil
}
func (repoStub) List(context.Context, string) ([]booking.Reservation, error) { return nil, nil }
func (repoStub) Pay(context.Context, string, string, string, string, time.Time, time.Time) (booking.Reservation, error) {
	return booking.Reservation{}, nil
}
func (repoStub) Decide(context.Context, string, string, string, time.Time) (booking.Reservation, error) {
	return booking.Reservation{}, nil
}
func (repoStub) Cancel(context.Context, string, string, time.Time) (booking.Reservation, error) {
	return booking.Reservation{}, nil
}
func (repoStub) Expire(context.Context, time.Time) error { return nil }

type paymentStub struct{}

func (paymentStub) Process(_ context.Context, outcome string) (string, error) {
	if outcome != "exito" && outcome != "rechazo" && outcome != "sin_respuesta" {
		return "", errors.New("invalid")
	}
	return outcome, nil
}

func TestLocalBookingFixtureAndQuoteRequireSessionAndCarrySafetyNotice(t *testing.T) {
	service, err := booking.NewService(repoStub{fixture: booking.Fixture{SpaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Title: "Espacio sintético", OwnerID: renterID, RenterID: renterID, RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "America/Santiago"}}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authStub{}, service, []string{"http://localhost:8081"})
	unauth := httptest.NewRecorder()
	h.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/fixture", nil))
	if unauth.Code != http.StatusUnauthorized || unauth.Header().Get("X-Prototype-Safety") != booking.SafetyBanner {
		t.Fatalf("unauthenticated response status/header=%d/%q", unauth.Code, unauth.Header().Get("X-Prototype-Safety"))
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/fixture", nil)
	request.Header.Set("Authorization", "Bearer test-session")
	fixtureResponse := httptest.NewRecorder()
	h.ServeHTTP(fixtureResponse, request)
	if fixtureResponse.Code != http.StatusOK {
		t.Fatalf("fixture status=%d body=%s", fixtureResponse.Code, fixtureResponse.Body.String())
	}
	var fixture map[string]any
	if err = json.Unmarshal(fixtureResponse.Body.Bytes(), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture["safety_notice"] != booking.SafetyBanner {
		t.Fatalf("safety notice missing: %v", fixture)
	}
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/quotes", strings.NewReader(`{"start_at":"2030-01-01T00:00:00Z","end_at":"2030-01-01T01:00:00Z","unexpected":true}`))
	bad.Header.Set("Authorization", "Bearer test-session")
	bad.Header.Set("Content-Type", "application/json")
	badResponse := httptest.NewRecorder()
	h.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("unknown quote field accepted: %d", badResponse.Code)
	}
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/quotes", strings.NewReader(`{"start_at":"2030-01-01T00:00:00Z","end_at":"2030-01-01T01:00:00Z"}`))
	quoteRequest.Header.Set("Authorization", "Bearer test-session")
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteResponse := httptest.NewRecorder()
	h.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusOK {
		t.Fatalf("quote status=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
	}
	var quote map[string]any
	if err = json.Unmarshal(quoteResponse.Body.Bytes(), &quote); err != nil {
		t.Fatal(err)
	}
	data := quote["data"].(map[string]any)
	if quote["safety_notice"] != booking.SafetyBanner || data["conditions"] != "Reglas sintéticas" {
		t.Fatalf("quote missing safety/snapshot conditions: %v", quote)
	}
}
