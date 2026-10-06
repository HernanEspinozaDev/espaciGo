package bookinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
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
func (r repoStub) Catalog(context.Context, string, booking.CatalogFilter) ([]booking.CatalogItem, error) {
	return []booking.CatalogItem{{SpaceID: r.fixture.SpaceID, CategoryCode: "sala_multiproposito", CategoryName: "Sala o espacio multipropósito", Title: r.fixture.Title, RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "America/Santiago", ProfileVersion: 1, Profile: json.RawMessage(`{"schema_version":1}`), Attributes: json.RawMessage(`{}`)}}, nil
}
func (repoStub) CatalogProfile(ctx context.Context, category string, version int) (spaces.Profile, error) {
	return spaces.Profile{CategoryCode: category, SchemaVersion: version, Attributes: []spaces.AttributeDefinition{{Code: "proyector", Type: "boolean"}}}, nil
}
func (r repoStub) CatalogDetail(context.Context, string, string) (booking.CatalogItem, error) {
	return booking.CatalogItem{SpaceID: r.fixture.SpaceID, CategoryCode: "sala_multiproposito", CategoryName: "Sala o espacio multipropósito", Title: r.fixture.Title, ProfileVersion: 1, Profile: json.RawMessage(`{"schema_version":1}`), Attributes: json.RawMessage(`{}`)}, nil
}
func (repoStub) Quote(_ context.Context, _ string, spaceID, id string, start, end time.Time, clock func() time.Time, ttl time.Duration) (booking.Quote, error) {
	created := clock().UTC()
	return booking.Quote{ID: id, SpaceID: spaceID, RateVersion: 1, RateUnit: "hora", UnitPrice: 8000, Currency: "CLP", Units: 1, Subtotal: 8000, StartAt: start, EndAt: end, TimeZone: "America/Santiago", Conditions: "Reglas sintéticas", CategoryCode: "sala_multiproposito", ProfileVersion: 1, ProfileValues: json.RawMessage(`{}`), CreatedAt: created, ExpiresAt: created.Add(ttl)}, nil
}
func (repoStub) Create(context.Context, string, string, string, []byte, string, string, time.Duration, func() time.Time) (booking.Reservation, error) {
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
	catalogRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?category_code=sala_multiproposito&start_at=2030-01-01T00%3A00%3A00Z&end_at=2030-01-01T01%3A00%3A00Z", nil)
	catalogRequest.Header.Set("Authorization", "Bearer test-session")
	catalogResponse := httptest.NewRecorder()
	h.ServeHTTP(catalogResponse, catalogRequest)
	if catalogResponse.Code != http.StatusOK || !strings.Contains(catalogResponse.Body.String(), `"category_code":"sala_multiproposito"`) || !strings.Contains(catalogResponse.Body.String(), booking.SafetyBanner) {
		t.Fatalf("catalog response=%d %s", catalogResponse.Code, catalogResponse.Body.String())
	}
	invalidFilter := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?start_at=not-a-date&end_at=2030-01-01T01%3A00%3A00Z", nil)
	invalidFilter.Header.Set("Authorization", "Bearer test-session")
	invalidFilterResponse := httptest.NewRecorder()
	h.ServeHTTP(invalidFilterResponse, invalidFilter)
	if invalidFilterResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid availability filter status=%d body=%s", invalidFilterResponse.Code, invalidFilterResponse.Body.String())
	}
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", nil)
	detailRequest.Header.Set("Authorization", "Bearer test-session")
	detailResponse := httptest.NewRecorder()
	h.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), `"profile_version":1`) {
		t.Fatalf("catalog detail=%d %s", detailResponse.Code, detailResponse.Body.String())
	}
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/quotes", strings.NewReader(`{"start_at":"2030-01-01T00:00:00Z","end_at":"2030-01-01T01:00:00Z","unexpected":true}`))
	bad.Header.Set("Authorization", "Bearer test-session")
	bad.Header.Set("Content-Type", "application/json")
	badResponse := httptest.NewRecorder()
	h.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("unknown quote field accepted: %d", badResponse.Code)
	}
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/quotes", strings.NewReader(`{"space_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","start_at":"2030-01-01T00:00:00Z","end_at":"2030-01-01T01:00:00Z"}`))
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
	if quote["safety_notice"] != booking.SafetyBanner || data["conditions"] != "Reglas sintéticas" || data["space_id"] != "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" || data["profile_version"] != float64(1) {
		t.Fatalf("quote missing safety/snapshot conditions: %v", quote)
	}
}

func TestCatalogSearchParsesTypedFiltersAndRequiresIntervalForPrice(t *testing.T) {
	values := url.Values{
		"category_code":   {"sala_multiproposito"},
		"profile_version": {"1"},
		"attributes":      {`{"proyector":false}`},
		"min_total_clp":   {"8000"},
		"start_at":        {"2030-01-01T00:00:00Z"},
		"end_at":          {"2030-01-01T01:00:00Z"},
	}
	filter, ok := catalogFilter(values)
	if !ok || filter.ProfileVersion != 1 || filter.Attributes["proyector"] != false || filter.MinTotalCLP == nil || *filter.MinTotalCLP != 8000 {
		t.Fatalf("catalog filter parse=%+v ok=%v", filter, ok)
	}
	if _, ok := catalogFilter(url.Values{"min_total_clp": {"8000"}}); !ok {
		t.Fatal("syntax parser should leave interval requirement to service validation")
	}
	if _, ok := catalogFilter(url.Values{"attributes": {`[]`}}); ok {
		t.Fatal("non-object attributes filter accepted")
	}
	if _, ok := catalogFilter(url.Values{"attributes": {`{"proyector":false}`, `{"proyector":true}`}}); ok {
		t.Fatal("duplicate attributes query parameter accepted")
	}

	service, err := booking.NewService(repoStub{fixture: booking.Fixture{SpaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Title: "Espacio sintético", OwnerID: renterID, RenterID: renterID, RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "America/Santiago"}}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Catalog(context.Background(), renterID, booking.CatalogFilter{MinTotalCLP: int64Ptr(8000)}); err != booking.ErrInvalid {
		t.Fatalf("price filter without interval error=%v", err)
	}
	if _, err = service.Catalog(context.Background(), renterID, booking.CatalogFilter{MinTotalCLP: int64Ptr(9000), MaxTotalCLP: int64Ptr(8000)}); err != booking.ErrInvalid {
		t.Fatalf("reversed inclusive price range error=%v", err)
	}
	if _, err = service.Catalog(context.Background(), renterID, booking.CatalogFilter{CategoryCode: "sala_multiproposito", ProfileVersion: 1, Attributes: map[string]any{"proyector": "false"}}); err != booking.ErrInvalid {
		t.Fatalf("wrong typed boolean filter error=%v", err)
	}
}

func int64Ptr(value int64) *int64 { return &value }
