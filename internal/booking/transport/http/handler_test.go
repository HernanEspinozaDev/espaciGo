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
