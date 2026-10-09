package bookinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
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

type repoStub struct {
	fixture        booking.Fixture
	distanceMeters float64
	items          []booking.CatalogItem
	decideErr      error
	cancelErr      error
}

type weeklyRepoStub struct {
	repoStub
	hours booking.WeeklyHours
}

type paymentEventRepoStub struct {
	repoStub
	seen map[string]string
}

func (r *paymentEventRepoStub) BeginPayment(context.Context, string, string, string, string, []byte, string, func() time.Time) (booking.PaymentOperation, bool, error) {
	return booking.PaymentOperation{}, false, nil
}
func (r *paymentEventRepoStub) PendingPayments(context.Context, int) ([]booking.PaymentOperation, error) {
	return nil, nil
}
func (r *paymentEventRepoStub) PendingPaymentEventIDs(context.Context, int) ([]string, error) {
	return nil, nil
}
func (r *paymentEventRepoStub) RecordPaymentEvent(_ context.Context, event booking.PaymentEvent, _ []byte, _ time.Time) (bool, error) {
	if r.seen == nil {
		r.seen = map[string]string{}
	}
	if prior, exists := r.seen[event.EventID]; exists {
		if prior != event.OperationID+":"+event.Outcome {
			return false, booking.ErrConflict
		}
		return true, nil
	}
	r.seen[event.EventID] = event.OperationID + ":" + event.Outcome
	return false, nil
}
func (r *paymentEventRepoStub) ApplyPaymentEvent(context.Context, string, func() time.Time, time.Duration) (booking.Reservation, error) {
	return booking.Reservation{}, nil
}

func (r *weeklyRepoStub) WeeklyHoursForSpace(_ context.Context, spaceID string) (booking.WeeklyHours, error) {
	v := r.hours
	v.SpaceID = spaceID
	return v, nil
}
func (r *weeklyRepoStub) WeeklyHoursForHost(ctx context.Context, hostID, spaceID string) (booking.WeeklyHours, error) {
	if hostID != renterID {
		return booking.WeeklyHours{}, booking.ErrNotFound
	}
	return r.WeeklyHoursForSpace(ctx, spaceID)
}
func (r *weeklyRepoStub) SaveWeeklyHours(_ context.Context, hostID, spaceID string, value booking.WeeklyHours) (booking.WeeklyHours, error) {
	if hostID != renterID {
		return booking.WeeklyHours{}, booking.ErrNotFound
	}
	value.SpaceID, value.TimeZone = spaceID, "America/Santiago"
	r.hours = value
	return value, nil
}

func (r repoStub) Fixture(context.Context, string) (booking.Fixture, error) { return r.fixture, nil }
func (r repoStub) Catalog(context.Context, string, booking.CatalogFilter) ([]booking.CatalogItem, error) {
	if r.items != nil {
		items := append([]booking.CatalogItem(nil), r.items...)
		sort.Slice(items, func(i, j int) bool {
			if items[i].CategoryOrder != items[j].CategoryOrder {
				return items[i].CategoryOrder < items[j].CategoryOrder
			}
			if items[i].Title != items[j].Title {
				return items[i].Title < items[j].Title
			}
			return items[i].SpaceID < items[j].SpaceID
		})
		return items, nil
	}
	return []booking.CatalogItem{{SpaceID: r.fixture.SpaceID, CategoryCode: "sala_multiproposito", CategoryName: "Sala o espacio multipropósito", Title: r.fixture.Title, RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "America/Santiago", ProfileVersion: 1, Profile: json.RawMessage(`{"schema_version":1}`), Attributes: json.RawMessage(`{}`), DistanceMeters: r.distanceMeters}}, nil
}
func (repoStub) CatalogProfile(ctx context.Context, category string, version int) (spaces.Profile, error) {
	return spaces.Profile{CategoryCode: category, SchemaVersion: version, Attributes: []spaces.AttributeDefinition{{Code: "proyector", Type: "boolean"}}}, nil
}
func (r repoStub) CatalogDetail(context.Context, string, string) (booking.CatalogItem, error) {
	return booking.CatalogItem{SpaceID: r.fixture.SpaceID, CategoryCode: "sala_multiproposito", CategoryName: "Sala o espacio multipropósito", Title: r.fixture.Title, RateUnit: r.fixture.RateUnit, TimeZone: r.fixture.TimeZone, ProfileVersion: 1, Profile: json.RawMessage(`{"schema_version":1}`), Attributes: json.RawMessage(`{}`)}, nil
}
func (repoStub) AvailableIntervals(_ context.Context, _ string, _ string, intervals []booking.AvailableInterval) ([]bool, error) {
	free := make([]bool, len(intervals))
	for i := range free {
		free[i] = true
	}
	return free, nil
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
func (repoStub) Pay(context.Context, string, string, string, string, func() time.Time, time.Duration) (booking.Reservation, error) {
	return booking.Reservation{}, nil
}
func (r repoStub) Decide(context.Context, string, string, string, string, func() time.Time) (booking.Reservation, error) {
	return booking.Reservation{}, r.decideErr
}
func (r repoStub) Cancel(context.Context, string, string, string, string, []byte, func() time.Time) (booking.CancellationResult, error) {
	return booking.CancellationResult{}, r.cancelErr
}
func (repoStub) CancellationPreview(context.Context, string, string, time.Time) (booking.CancellationPreview, error) {
	return booking.CancellationPreview{}, nil
}
func (repoStub) RefundOperation(context.Context, string, string) (booking.RefundResult, error) {
	return booking.RefundResult{}, nil
}
func (repoStub) RecordRefund(context.Context, string, string, string, time.Time) (booking.RefundResult, error) {
	return booking.RefundResult{}, nil
}
func (repoStub) NoticeRecipients(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (repoStub) Expire(context.Context, time.Time) error { return nil }

type paymentStub struct{}

func (paymentStub) StartPayment(_ context.Context, operationID, outcome string) (*booking.PaymentEvent, error) {
	if operationID == "" || (outcome != "exito" && outcome != "rechazo" && outcome != "sin_respuesta") {
		return nil, errors.New("invalid")
	}
	return nil, nil
}
func (paymentStub) LookupPayment(context.Context, booking.PaymentOperation) (*booking.PaymentEvent, error) {
	return nil, nil
}
func (paymentStub) VerifyPaymentEvent(booking.PaymentEvent) bool { return false }

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
	selectorZone, _ := time.LoadLocation("America/Santiago")
	selectorDate := time.Now().In(selectorZone).AddDate(0, 0, 1).Format("2006-01-02")
	availabilityRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/availability-options?date="+selectorDate+"&duration=2", nil)
	availabilityRequest.Header.Set("Authorization", "Bearer test-session")
	availabilityResponse := httptest.NewRecorder()
	h.ServeHTTP(availabilityResponse, availabilityRequest)
	if availabilityResponse.Code != http.StatusOK || !strings.Contains(availabilityResponse.Body.String(), `"rate_unit":"hora"`) || !strings.Contains(availabilityResponse.Body.String(), `"time_zone":"America/Santiago"`) || !strings.Contains(availabilityResponse.Body.String(), `"items":[`) {
		t.Fatalf("availability options response=%d %s", availabilityResponse.Code, availabilityResponse.Body.String())
	}
	for _, query := range []string{"date=2030-01-01", "date=2030-01-01&duration=3", "date=2030-01-01&duration=2&rate_unit=dia"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/availability-options?"+query, nil)
		request.Header.Set("Authorization", "Bearer test-session")
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid selector query %q status=%d body=%s", query, response.Code, response.Body.String())
		}
	}
	invalidFilter := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?start_at=not-a-date&end_at=2030-01-01T01%3A00%3A00Z", nil)
	invalidFilter.Header.Set("Authorization", "Bearer test-session")
	invalidFilterResponse := httptest.NewRecorder()
	h.ServeHTTP(invalidFilterResponse, invalidFilter)
	if invalidFilterResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid availability filter status=%d body=%s", invalidFilterResponse.Code, invalidFilterResponse.Body.String())
	}
	invalidPageSize := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?page_size=26", nil)
	invalidPageSize.Header.Set("Authorization", "Bearer test-session")
	invalidPageSizeResponse := httptest.NewRecorder()
	h.ServeHTTP(invalidPageSizeResponse, invalidPageSize)
	if invalidPageSizeResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid page_size status=%d body=%s", invalidPageSizeResponse.Code, invalidPageSizeResponse.Body.String())
	}
	geoFixture := booking.Fixture{SpaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Title: "Espacio sintético", OwnerID: renterID, RenterID: renterID, RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "America/Santiago"}
	geoService, err := booking.NewService(repoStub{fixture: geoFixture, distanceMeters: 1423}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	geoHandler := NewHandler(authStub{}, geoService, []string{"http://localhost:8081"})
	for _, query := range []string{
		"latitude=-33.456", "latitude=&longitude=-70.6693&radius_km=5", "latitude=norte&longitude=-70.6693&radius_km=5",
		"latitude=-33.456&longitude=-70.6693&radius_km=", "latitude=-33.456&longitude=-70.6693&radius_km=cinco",
		"latitude=NaN&longitude=-70.6693&radius_km=5",
		"latitude=90.1&longitude=-70.6693&radius_km=5", "latitude=-33.456&longitude=181&radius_km=5",
		"latitude=-33.456&longitude=-70.6693&radius_km=2",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?"+query, nil)
		request.Header.Set("Authorization", "Bearer test-session")
		response := httptest.NewRecorder()
		geoHandler.ServeHTTP(response, request)
		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("invalid geographic query %q status=%d body=%s", query, response.Code, response.Body.String())
		}
	}
	geoRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?latitude=-33.456&longitude=-70.6693&radius_km=5", nil)
	geoRequest.Header.Set("Authorization", "Bearer test-session")
	geoResponse := httptest.NewRecorder()
	geoHandler.ServeHTTP(geoResponse, geoRequest)
	if geoResponse.Code != http.StatusOK || !strings.Contains(geoResponse.Body.String(), `"distance_km":1.4`) || !strings.Contains(geoResponse.Body.String(), `"distance_kind":"direct"`) || strings.Contains(geoResponse.Body.String(), `"latitude"`) || strings.Contains(geoResponse.Body.String(), `"longitude"`) {
		t.Fatalf("geographic catalog response=%d %s", geoResponse.Code, geoResponse.Body.String())
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

func TestPaymentEventCallbackRejectsUnauthenticatedEventWithoutUserSession(t *testing.T) {
	adapter, err := fakebooking.New([]byte("handler-test-payment-webhook-key-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	repo := &paymentEventRepoStub{}
	service, err := booking.NewService(repo, credentials.Generator{}, time.Now, adapter)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(authStub{}, service, nil)
	event := adapter.SignPaymentEvent("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "exito_simulado")
	body := `{"event_id":"` + event.EventID + `","operation_id":"` + event.OperationID + `","outcome":"` + event.Outcome + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/payment-events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Local-Payment-Signature", "invalid")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated provider event status=%d body=%s", response.Code, response.Body.String())
	}
	if len(repo.seen) != 0 {
		t.Fatal("unauthenticated event was persisted")
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/payment-events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Local-Payment-Signature", event.Signature)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("authenticated callback status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/payment-events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Local-Payment-Signature", event.Signature)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"reused":true`) {
		t.Fatalf("replayed callback status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestWeeklyHoursHTTPReadsAndReplacesOwnerScheduleStrictly(t *testing.T) {
	fixture := booking.Fixture{SpaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Title: "synthetic", RateUnit: "hora", TimeZone: "America/Santiago"}
	repo := &weeklyRepoStub{repoStub: repoStub{fixture: fixture}}
	for i := 1; i <= 7; i++ {
		repo.hours.Days = append(repo.hours.Days, booking.WeeklyDay{Weekday: i, Periods: []booking.WeeklyPeriod{}})
	}
	service, err := booking.NewService(repo, credentials.Generator{}, func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authStub{}, service, []string{"http://mock.local"})
	base := "/api/v1/local/booking-trial/catalog/" + fixture.SpaceID + "/weekly-hours"
	get := httptest.NewRequest(http.MethodGet, base, nil)
	get.Header.Set("Authorization", "Bearer test-session")
	got := httptest.NewRecorder()
	h.ServeHTTP(got, get)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"enabled":false`) {
		t.Fatalf("GET weekly-hours status=%d body=%s", got.Code, got.Body.String())
	}
	days := make([]booking.WeeklyDay, 7)
	for i := range days {
		days[i] = booking.WeeklyDay{Weekday: i + 1, Periods: []booking.WeeklyPeriod{}}
	}
	days[0].Periods = []booking.WeeklyPeriod{{Open: "09:00", Close: "12:00"}, {Open: "13:00", Close: "24:00"}}
	body, _ := json.Marshal(map[string]any{"enabled": true, "days": days})
	put := httptest.NewRequest(http.MethodPut, base, strings.NewReader(string(body)))
	put.Header.Set("Authorization", "Bearer test-session")
	put.Header.Set("Content-Type", "application/json")
	updated := httptest.NewRecorder()
	h.ServeHTTP(updated, put)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"time_zone":"America/Santiago"`) {
		t.Fatalf("PUT weekly-hours status=%d body=%s", updated.Code, updated.Body.String())
	}
	days[0].Periods = []booking.WeeklyPeriod{{Open: "09:00", Close: "13:00"}, {Open: "12:00", Close: "17:00"}}
	body, _ = json.Marshal(map[string]any{"enabled": true, "days": days})
	invalid := httptest.NewRequest(http.MethodPut, base, strings.NewReader(string(body)))
	invalid.Header.Set("Authorization", "Bearer test-session")
	invalid.Header.Set("Content-Type", "application/json")
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, invalid)
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("overlap PUT status=%d body=%s", bad.Code, bad.Body.String())
	}
	unknownField := httptest.NewRequest(http.MethodPut, base, strings.NewReader(`{"enabled":true,"days":[],"owner_id":"x"}`))
	unknownField.Header.Set("Authorization", "Bearer test-session")
	unknownField.Header.Set("Content-Type", "application/json")
	strict := httptest.NewRecorder()
	h.ServeHTTP(strict, unknownField)
	if strict.Code != http.StatusBadRequest {
		t.Fatalf("unknown JSON field status=%d body=%s", strict.Code, strict.Body.String())
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
		"latitude":        {"-33.456"},
		"longitude":       {"-70.6693"},
		"radius_km":       {"5"},
	}
	filter, ok := catalogFilter(values)
	if !ok || filter.ProfileVersion != 1 || filter.Attributes["proyector"] != false || filter.MinTotalCLP == nil || *filter.MinTotalCLP != 8000 || filter.Latitude == nil || *filter.Latitude != -33.456 || filter.Longitude == nil || *filter.Longitude != -70.6693 || filter.RadiusKM == nil || *filter.RadiusKM != 5 {
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

func TestCatalogPaginationKeysetAndCursorBinding(t *testing.T) {
	ids := []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000004", "00000000-0000-4000-8000-000000000005", "00000000-0000-4000-8000-000000000006", "00000000-0000-4000-8000-000000000007", "00000000-0000-4000-8000-000000000008", "00000000-0000-4000-8000-000000000009", "00000000-0000-4000-8000-000000000010", "00000000-0000-4000-8000-000000000011"}
	items := make([]booking.CatalogItem, len(ids))
	for i, id := range ids {
		distance := 1000 + float64(i)/100
		if i == 5 {
			distance = 1000 + float64(i-1)/100
		}
		items[i] = booking.CatalogItem{SpaceID: id, CategoryOrder: 1, Title: "Sala", DistanceMeters: distance, EstimatedTotal: int64PtrTestLocal(9000)}
	}
	service, err := booking.NewService(repoStub{items: items}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	filter := booking.CatalogFilter{Latitude: floatPtrLocal(-33.4), Longitude: floatPtrLocal(-70.6), RadiusKM: intPtrLocal(5)}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	var pageIDs [][]string
	for {
		page, e := service.CatalogPage(context.Background(), renterID, filter, 5, cursor)
		if e != nil {
			t.Fatal(e)
		}
		pages++
		for _, item := range page.Items {
			if seen[item.SpaceID] {
				t.Fatalf("duplicate %s", item.SpaceID)
			}
			seen[item.SpaceID] = true
		}
		idsOnPage := make([]string, 0, len(page.Items))
		for _, item := range page.Items {
			idsOnPage = append(idsOnPage, item.SpaceID)
		}
		pageIDs = append(pageIDs, idsOnPage)
		if cursor == "" && (page.Items[0].DistanceKM == nil || *page.Items[0].DistanceKM != 1.0 || page.Items[1].DistanceKM == nil || *page.Items[1].DistanceKM != 1.0 || page.Items[0].DistanceMeters >= page.Items[1].DistanceMeters) {
			t.Fatalf("raw distances did not order equal-rounded results: %+v", page.Items[:2])
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if pages != 3 || len(seen) != len(items) {
		t.Fatalf("pages=%d seen=%d", pages, len(seen))
	}
	if pageIDs[0][4] != ids[4] || pageIDs[1][0] != ids[5] {
		t.Fatalf("page boundary did not continue through raw-distance/price/ID tie: %v", pageIDs)
	}
	first, err := service.CatalogPage(context.Background(), renterID, filter, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	defaultPage, err := service.CatalogPage(context.Background(), renterID, filter, 0, "")
	if err != nil || len(defaultPage.Items) != 5 || defaultPage.NextCursor == "" {
		t.Fatalf("default page size response=%+v err=%v", defaultPage, err)
	}
	for _, tc := range []struct {
		name, actor string
		size        int
		filter      booking.CatalogFilter
	}{{"account", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", 5, filter}, {"page size", renterID, 4, filter}, {"filters", renterID, 5, booking.CatalogFilter{Latitude: floatPtrLocal(-33.5), Longitude: filter.Longitude, RadiusKM: filter.RadiusKM}}} {
		if _, e := service.CatalogPage(context.Background(), tc.actor, tc.filter, tc.size, first.NextCursor); e != booking.ErrInvalid {
			t.Errorf("%s cursor reuse error=%v", tc.name, e)
		}
	}
	if _, e := service.CatalogPage(context.Background(), renterID, filter, 5, "v2.bad"); e != booking.ErrInvalid {
		t.Fatalf("unknown version error=%v", e)
	}
	mutate := byte('A')
	if first.NextCursor[3] == mutate {
		mutate = 'B'
	}
	corrupt := first.NextCursor[:3] + string(mutate) + first.NextCursor[4:]
	if _, e := service.CatalogPage(context.Background(), renterID, filter, 5, corrupt); e != booking.ErrInvalid {
		t.Fatalf("tampered cursor error=%v", e)
	}
	if _, e := service.CatalogPage(context.Background(), renterID, filter, 26, ""); e != booking.ErrInvalid {
		t.Fatalf("oversized page error=%v", e)
	}
	other, err := booking.NewService(repoStub{items: items}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, e := other.CatalogPage(context.Background(), renterID, filter, 5, first.NextCursor); e != booking.ErrInvalid {
		t.Fatalf("cursor from different process key error=%v", e)
	}
}

func TestCatalogPaginationEmptyResultHasNoCursor(t *testing.T) {
	service, err := booking.NewService(repoStub{items: []booking.CatalogItem{}}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.CatalogPage(context.Background(), renterID, booking.CatalogFilter{}, 5, "")
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("empty page=%+v err=%v", page, err)
	}
}

func TestCatalogPaginationPreservesCategoryAndPriceOrdersAfterFiltering(t *testing.T) {
	items := []booking.CatalogItem{
		{SpaceID: "00000000-0000-4000-8000-000000000001", CategoryOrder: 2, Title: "Z", RateUnit: "hora", Price: 10000, Currency: "CLP", TimeZone: "UTC"},
		{SpaceID: "00000000-0000-4000-8000-000000000002", CategoryOrder: 1, Title: "B", RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "UTC"},
		{SpaceID: "00000000-0000-4000-8000-000000000003", CategoryOrder: 1, Title: "A", RateUnit: "hora", Price: 8000, Currency: "CLP", TimeZone: "UTC"},
	}
	service, err := booking.NewService(repoStub{items: items}, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	readAll := func(filter booking.CatalogFilter) []string {
		t.Helper()
		cursor := ""
		var ids []string
		for {
			page, e := service.CatalogPage(context.Background(), renterID, filter, 1, cursor)
			if e != nil {
				t.Fatal(e)
			}
			for _, item := range page.Items {
				ids = append(ids, item.SpaceID)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		return ids
	}
	categoryOrder := readAll(booking.CatalogFilter{})
	if strings.Join(categoryOrder, ",") != "00000000-0000-4000-8000-000000000003,00000000-0000-4000-8000-000000000002,00000000-0000-4000-8000-000000000001" {
		t.Fatalf("category/title/id order=%v", categoryOrder)
	}
	start := time.Now().UTC().Add(24 * time.Hour)
	end := start.Add(time.Hour)
	min := int64(8000)
	priceOrder := readAll(booking.CatalogFilter{StartAt: &start, EndAt: &end, MinTotalCLP: &min})
	if strings.Join(priceOrder, ",") != "00000000-0000-4000-8000-000000000002,00000000-0000-4000-8000-000000000003,00000000-0000-4000-8000-000000000001" {
		t.Fatalf("filter-before-price-order=%v", priceOrder)
	}
}
func int64PtrTestLocal(v int64) *int64 { return &v }
func floatPtrLocal(v float64) *float64 { return &v }
func intPtrLocal(v int) *int           { return &v }

func TestCatalogPageQueryParameters(t *testing.T) {
	for _, tc := range []struct {
		query url.Values
		want  int
		ok    bool
	}{{url.Values{}, 0, true}, {url.Values{"page_size": {"1"}}, 1, true}, {url.Values{"page_size": {"25"}}, 25, true}, {url.Values{"page_size": {"0"}}, 0, false}, {url.Values{"page_size": {"26"}}, 0, false}, {url.Values{"page_size": {"2", "3"}}, 0, false}, {url.Values{"cursor": {"v1.abc"}}, 0, true}} {
		size, _, ok := catalogPageParams(tc.query)
		if size != tc.want || ok != tc.ok {
			t.Errorf("page params %v => %d,%v want %d,%v", tc.query, size, ok, tc.want, tc.ok)
		}
	}
}

func TestLocalBookingDecisionAndCancellationConflictsMatchHTTPContract(t *testing.T) {
	repo := repoStub{decideErr: booking.ErrConflict, cancelErr: booking.ErrConflict}
	service, err := booking.NewService(repo, credentials.Generator{}, func() time.Time { return time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC) }, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(authStub{}, service, nil)
	base := "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001"
	request := func(path, body, idempotency string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer test-session")
		r.Header.Set("Content-Type", "application/json")
		if idempotency != "" {
			r.Header.Set("Idempotency-Key", idempotency)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response
	}
	missingReason := request(base+"/decision", `{"decision":"rechazar"}`, "")
	if missingReason.Code != http.StatusUnprocessableEntity {
		t.Fatalf("host rejection without reason status=%d body=%s", missingReason.Code, missingReason.Body)
	}
	unknown := request(base+"/decision", `{"decision":"aprobar","extra":true}`, "")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("decision with unknown field status=%d body=%s", unknown.Code, unknown.Body)
	}
	decisionConflict := request(base+"/decision", `{"decision":"aprobar"}`, "")
	if decisionConflict.Code != http.StatusConflict {
		t.Fatalf("decision conflict status=%d body=%s", decisionConflict.Code, decisionConflict.Body)
	}
	cancelConflict := request(base+"/cancel", `{"reason":"ya no lo necesito"}`, "cancel-http-test")
	if cancelConflict.Code != http.StatusConflict {
		t.Fatalf("cancellation conflict status=%d body=%s", cancelConflict.Code, cancelConflict.Body)
	}
}

type adminReadAuthStub struct{ administrator bool }

func (a adminReadAuthStub) Authorize(_ context.Context, raw identity.Secret, required identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if raw != "admin-read-session" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	if required == identity.RoleAdministrator && !a.administrator {
		return identity.Principal{}, identity.ErrForbidden
	}
	return identity.Principal{AccountID: renterID}, nil
}

type adminReadRepoStub struct {
	repoStub
	listCalls, detailCalls int
	actor, correlation     string
	filter                 booking.AdminReservationFilter
	size                   int
	cursor                 string
}

func (r *adminReadRepoStub) ListAdminReservations(_ context.Context, actor, correlation string, filter booking.AdminReservationFilter, size int, cursor string) (booking.AdminReservationPage, error) {
	r.listCalls++
	r.actor = actor
	r.correlation = correlation
	r.filter = filter
	r.size = size
	r.cursor = cursor
	return booking.AdminReservationPage{Items: []booking.AdminReservationSummary{{ID: "00000000-0000-4000-8000-000000000001", State: "pagada", Currency: "CLP"}}, NextCursor: "opaque-next"}, nil
}
func (r *adminReadRepoStub) GetAdminReservation(_ context.Context, actor, id, correlation string) (booking.AdminReservationDetail, error) {
	r.detailCalls++
	r.actor = actor
	r.correlation = correlation
	return booking.AdminReservationDetail{AdminReservationSummary: booking.AdminReservationSummary{ID: id, State: "pagada"}, History: []booking.Transition{}, Payments: []booking.AdminPaymentFact{}, PaymentOperations: []booking.AdminPaymentOperation{}}, nil
}

func TestAdminReservationReadRequiresRoleAndAcceptsScopedFilters(t *testing.T) {
	repo := &adminReadRepoStub{}
	service, err := booking.NewService(repo, credentials.Generator{}, time.Now, paymentStub{})
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/admin/local/reservations"
	call := func(admin bool, target string) *httptest.ResponseRecorder {
		t.Helper()
		handler := NewHandler(adminReadAuthStub{administrator: admin}, service, nil)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Authorization", "Bearer admin-read-session")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	denied := call(false, base)
	if denied.Code != http.StatusForbidden || repo.listCalls != 0 {
		t.Fatalf("non-admin list status=%d calls=%d", denied.Code, repo.listCalls)
	}
	from, to := "2030-01-01T00:00:00Z", "2030-02-01T00:00:00Z"
	got := call(true, base+"?reservation_id=00000000-0000-4000-8000-000000000001&state=pagada&created_from="+url.QueryEscape(from)+"&created_to="+url.QueryEscape(to)+"&page_size=25")
	if got.Code != http.StatusOK || repo.listCalls != 1 || repo.size != 25 || repo.filter.State != "pagada" || repo.filter.ID == "" || repo.filter.CreatedFrom == nil || repo.filter.CreatedTo == nil || repo.correlation == "" {
		t.Fatalf("admin list status=%d calls=%d repo=%+v body=%s", got.Code, repo.listCalls, repo, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), `"next_cursor":"opaque-next"`) {
		t.Fatalf("missing opaque cursor: %s", got.Body.String())
	}
	if bad := call(true, base+"?page_size=101"); bad.Code != http.StatusUnprocessableEntity || repo.listCalls != 1 {
		t.Fatalf("invalid page size status=%d calls=%d", bad.Code, repo.listCalls)
	}
	detail := call(true, base+"/00000000-0000-4000-8000-000000000001")
	if detail.Code != http.StatusOK || repo.detailCalls != 1 || !strings.Contains(detail.Body.String(), `"history":[]`) {
		t.Fatalf("detail status=%d calls=%d body=%s", detail.Code, repo.detailCalls, detail.Body.String())
	}
}

func TestAdminReservationFilterUsesInclusiveFromExclusiveTo(t *testing.T) {
	values := url.Values{"created_from": {"2030-01-01T00:00:00-03:00"}, "created_to": {"2030-02-01T00:00:00Z"}}
	filter, size, cursor, ok := adminReservationParams(values)
	if !ok || size != 25 || cursor != "" || filter.CreatedFrom == nil || filter.CreatedFrom.Location() != time.UTC || filter.CreatedTo == nil || !filter.CreatedTo.After(*filter.CreatedFrom) {
		t.Fatalf("parsed admin filter=%+v size=%d cursor=%q ok=%t", filter, size, cursor, ok)
	}
	for _, query := range []url.Values{{"page_size": {"0"}}, {"state": {"unknown"}}, {"created_from": {"yesterday"}}, {"unexpected": {"x"}}, {"page_size": {"25", "50"}}} {
		if _, _, _, valid := adminReservationParams(query); valid {
			t.Errorf("accepted invalid admin filter %v", query)
		}
	}
}
