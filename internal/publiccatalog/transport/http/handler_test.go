package publiccataloghttp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

type pageSizeRepository struct {
	pageSize         int
	listErr          error
	categoryCalls    int
	filterCalls      int
	listCalls        int
	locationCalls    int
	locationQuery    string
	locationRegion   string
	locationLimit    int
	spaces           []publiccatalog.Space
	filters          publiccatalog.Filters
	attributeFilters []publiccatalog.AttributeFilter
	detail           *publiccatalog.Space
	detailErr        error
	detailCalls      int
	detailID         string
}

func (r *pageSizeRepository) Categories(context.Context) ([]publiccatalog.Category, error) {
	r.categoryCalls++
	return nil, nil
}
func (r *pageSizeRepository) HasActiveCategory(context.Context, string) (bool, error) {
	return true, nil
}
func (r *pageSizeRepository) AttributeFilters(context.Context, string) ([]publiccatalog.AttributeFilter, error) {
	r.filterCalls++
	return append([]publiccatalog.AttributeFilter(nil), r.attributeFilters...), nil
}
func (r *pageSizeRepository) HasRegion(context.Context, string) (bool, error) { return true, nil }
func (r *pageSizeRepository) HasCommune(context.Context, string, string) (bool, error) {
	return true, nil
}
func (r *pageSizeRepository) Locations(_ context.Context, query, region string, limit int) ([]publiccatalog.Location, error) {
	r.locationCalls++
	r.locationQuery, r.locationRegion, r.locationLimit = query, region, limit
	return nil, nil
}
func (r *pageSizeRepository) List(_ context.Context, filters publiccatalog.Filters, _ *publiccatalog.Cursor, _ int) ([]publiccatalog.Space, error) {
	r.listCalls++
	r.pageSize = filters.PageSize
	r.filters = filters
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]publiccatalog.Space(nil), r.spaces...), nil
}
func (r *pageSizeRepository) Detail(_ context.Context, id string) (publiccatalog.Space, error) {
	r.detailCalls++
	r.detailID = id
	if r.detailErr != nil {
		return publiccatalog.Space{}, r.detailErr
	}
	if r.detail != nil {
		return *r.detail, nil
	}
	return publiccatalog.Space{}, publiccatalog.ErrNotFound
}

func TestPublicCatalogHeadMatchesGetHeadersWithoutBody(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces", nil))
	headResponse := httptest.NewRecorder()
	handler.ServeHTTP(headResponse, httptest.NewRequest(http.MethodHead, "/api/v1/public/spaces", nil))

	if headResponse.Code != getResponse.Code || headResponse.Body.Len() != 0 {
		t.Fatalf("HEAD status/body=%d/%q, GET status=%d", headResponse.Code, headResponse.Body.String(), getResponse.Code)
	}
	for _, key := range []string{"Cache-Control", "Content-Type", "X-Content-Type-Options", "Vary"} {
		if got, want := headResponse.Header().Get(key), getResponse.Header().Get(key); got != want {
			t.Errorf("HEAD %s=%q, GET %s=%q", key, got, key, want)
		}
	}
	if got, want := headResponse.Header().Get("Content-Length"), strconv.Itoa(getResponse.Body.Len()); got != want {
		t.Errorf("HEAD Content-Length=%q, want GET representation length %q", got, want)
	}
	if repo.listCalls != 2 {
		t.Fatalf("repository list calls=%d, want GET and HEAD representation reads", repo.listCalls)
	}
}

func TestPublicCatalogMethodNotAllowedAdvertisesAllowedMethods(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/public/spaces", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD, OPTIONS" {
		t.Fatalf("status=%d Allow=%q, want 405 and GET, HEAD, OPTIONS", response.Code, response.Header().Get("Allow"))
	}
	if repo.listCalls != 0 {
		t.Fatalf("repository list calls=%d, want 0 for rejected method", repo.listCalls)
	}
}

func TestPublicCatalogHeadPreservesErrorStatusAndLengthWithoutBody(t *testing.T) {
	cases := []struct {
		name, path string
		wantStatus int
	}{
		{name: "invalid query", path: "/api/v1/public/spaces?page_size=0", wantStatus: http.StatusUnprocessableEntity},
		{name: "missing detail", path: "/api/v1/public/spaces/11111111-1111-4111-8111-111111111111", wantStatus: http.StatusNotFound},
		{name: "repository failure", path: "/api/v1/public/spaces", wantStatus: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			if tc.wantStatus == http.StatusInternalServerError {
				repo.listErr = errors.New("synthetic repository failure")
			}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			handler := New(service)
			getResponse := httptest.NewRecorder()
			handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, tc.path, nil))
			headResponse := httptest.NewRecorder()
			handler.ServeHTTP(headResponse, httptest.NewRequest(http.MethodHead, tc.path, nil))
			if getResponse.Code != tc.wantStatus || headResponse.Code != tc.wantStatus || headResponse.Body.Len() != 0 {
				t.Fatalf("GET/HEAD status=%d/%d body_len=%d, want %d and empty HEAD", getResponse.Code, headResponse.Code, headResponse.Body.Len(), tc.wantStatus)
			}
			if got, want := headResponse.Header().Get("Content-Length"), strconv.Itoa(getResponse.Body.Len()); got != want {
				t.Errorf("HEAD Content-Length=%q, want GET representation length %q", got, want)
			}
			if headResponse.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("HEAD Cache-Control=%q, want no-store", headResponse.Header().Get("Cache-Control"))
			}
			if tc.wantStatus == http.StatusUnprocessableEntity && repo.listCalls != 0 {
				t.Errorf("invalid query dispatched to list %d times", repo.listCalls)
			}
			if tc.wantStatus == http.StatusNotFound && repo.detailCalls != 2 {
				t.Errorf("missing detail repository calls=%d, want GET and HEAD reads", repo.detailCalls)
			}
			if tc.wantStatus == http.StatusInternalServerError && strings.Contains(getResponse.Body.String(), "synthetic repository failure") {
				t.Error("repository error leaked in GET error envelope")
			}
		})
	}
}

func TestPublicPageSizeAbsentAndExplicitBounds(t *testing.T) {
	cases := []struct {
		name, query      string
		status, pageSize int
	}{
		{name: "omitted defaults to 20", status: http.StatusOK, pageSize: 20},
		{name: "zero is rejected", query: "?page_size=0", status: http.StatusUnprocessableEntity},
		{name: "negative is rejected", query: "?page_size=-1", status: http.StatusUnprocessableEntity},
		{name: "one is accepted", query: "?page_size=1", status: http.StatusOK, pageSize: 1},
		{name: "forty eight is accepted", query: "?page_size=48", status: http.StatusOK, pageSize: 48},
		{name: "forty nine is rejected", query: "?page_size=49", status: http.StatusUnprocessableEntity},
		{name: "nonnumeric is rejected", query: "?page_size=large", status: http.StatusUnprocessableEntity},
		{name: "duplicate is rejected", query: "?page_size=1&page_size=48", status: http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces"+tc.query, nil))
			if response.Code != tc.status {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, tc.status, response.Body.String())
			}
			if tc.status == http.StatusOK && repo.pageSize != tc.pageSize {
				t.Fatalf("repository page size=%d, want %d", repo.pageSize, tc.pageSize)
			}
			if tc.status != http.StatusOK && repo.listCalls != 0 {
				t.Fatalf("invalid page size dispatched to repository %d times", repo.listCalls)
			}
		})
	}
}

func TestPublicSpaceDateFiltersNormalizeOffsetsToUTCBeforeRepository(t *testing.T) {
	query := url.Values{}
	query.Set("start_at", "2026-01-15T12:00:00-03:00")
	query.Set("end_at", "2026-01-15T14:30:00-03:00")
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces?"+query.Encode(), nil))
	if response.Code != http.StatusOK || repo.listCalls != 1 {
		t.Fatalf("status=%d list_calls=%d body=%s", response.Code, repo.listCalls, response.Body.String())
	}
	wantStart := time.Date(2026, time.January, 15, 15, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.January, 15, 17, 30, 0, 0, time.UTC)
	if repo.filters.StartAt == nil || !repo.filters.StartAt.Equal(wantStart) || repo.filters.StartAt.Location() != time.UTC {
		t.Errorf("repository start_at=%v, want %v in UTC", repo.filters.StartAt, wantStart)
	}
	if repo.filters.EndAt == nil || !repo.filters.EndAt.Equal(wantEnd) || repo.filters.EndAt.Location() != time.UTC {
		t.Errorf("repository end_at=%v, want %v in UTC", repo.filters.EndAt, wantEnd)
	}
}

func TestPublicSpacePricingBoundsReachRepository(t *testing.T) {
	query := url.Values{}
	query.Set("min_unit_price_clp", "1000")
	query.Set("max_unit_price_clp", "9000")
	query.Set("rate_unit", "hora")
	query.Set("min_total_clp", "2500")
	query.Set("max_total_clp", "24000")
	query.Set("start_at", "2026-01-15T12:00:00Z")
	query.Set("end_at", "2026-01-15T13:00:00Z")
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces?"+query.Encode(), nil))
	if response.Code != http.StatusOK || repo.listCalls != 1 {
		t.Fatalf("status=%d list_calls=%d body=%s", response.Code, repo.listCalls, response.Body.String())
	}
	for name, pair := range map[string]struct {
		got  *int64
		want int64
	}{
		"min_unit_price_clp": {repo.filters.MinUnitPriceCLP, 1000},
		"max_unit_price_clp": {repo.filters.MaxUnitPriceCLP, 9000},
		"min_total_clp":      {repo.filters.MinTotalCLP, 2500},
		"max_total_clp":      {repo.filters.MaxTotalCLP, 24000},
	} {
		if pair.got == nil {
			t.Errorf("%s missing from repository filters", name)
		} else if *pair.got != pair.want {
			t.Errorf("%s=%d, want %d", name, *pair.got, pair.want)
		}
	}
}

func TestPublicSpaceRejectsInvalidDateAndPriceFiltersBeforeRepository(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{name: "invalid timestamp", query: "?start_at=not-a-timestamp"},
		{name: "duplicate timestamp", query: "?start_at=2026-01-15T12%3A00%3A00Z&start_at=2026-01-15T13%3A00%3A00Z"},
		{name: "negative unit price", query: "?min_unit_price_clp=-1"},
		{name: "overflow total", query: "?max_total_clp=9223372036854775808"},
		{name: "malformed attribute expression", query: "?attribute=equipo%3Agt%3A2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces"+tc.query, nil))
			if response.Code != http.StatusUnprocessableEntity || repo.listCalls != 0 {
				t.Fatalf("status=%d list_calls=%d body=%s", response.Code, repo.listCalls, response.Body.String())
			}
		})
	}
}

func TestPublicSpacesAndLocationsRejectMalformedQueryBeforeRepository(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "categories", path: "/api/v1/public/space-categories?unknown=%ZZ"},
		{name: "category filters", path: "/api/v1/public/space-categories/sala/filters?unknown=%ZZ"},
		{name: "spaces", path: "/api/v1/public/spaces?page_size=10&unknown=%ZZ"},
		{name: "locations", path: "/api/v1/public/locations?q=santiago&unknown=%ZZ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != http.StatusUnprocessableEntity || repo.categoryCalls != 0 || repo.filterCalls != 0 || repo.listCalls != 0 || repo.locationCalls != 0 {
				t.Fatalf("status=%d category_calls=%d filter_calls=%d list_calls=%d location_calls=%d body=%s", response.Code, repo.categoryCalls, repo.filterCalls, repo.listCalls, repo.locationCalls, response.Body.String())
			}
		})
	}
}

func TestCategoryFilterRouteRejectsNestedCategoryPathBeforeRepository(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/space-categories/oficina/extra/filters", nil))
	if response.Code != http.StatusNotFound || repo.categoryCalls != 0 || repo.filterCalls != 0 {
		t.Fatalf("status=%d category_calls=%d filter_calls=%d body=%s", response.Code, repo.categoryCalls, repo.filterCalls, response.Body.String())
	}
}

func TestPublicListEndpointsSerializeNilResultsAsArrays(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "categories", path: "/api/v1/public/space-categories"},
		{name: "category filters", path: "/api/v1/public/space-categories/sala/filters"},
		{name: "locations", path: "/api/v1/public/locations"},
		{name: "spaces", path: "/api/v1/public/spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			var envelope struct {
				Items json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid response JSON: %v body=%s", err, response.Body.String())
			}
			if response.Code != http.StatusOK || string(envelope.Items) != "[]" {
				t.Fatalf("status=%d items=%s body=%s", response.Code, envelope.Items, response.Body.String())
			}
		})
	}
}

func TestPublicSpaceListSerializesNestedEmptyCollectionsAsArrays(t *testing.T) {
	spaceID := "abcdefab-cdef-4abc-8def-abcdefabcdef"
	regionCode, regionName := "13", "Metropolitana"
	communeCode, communeName := "13101", "Santiago"
	coverImageID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	estimatedTotal := int64(20000)
	days := make([]publiccatalog.PublicWeeklyDay, 7)
	for weekday := range days {
		days[weekday] = publiccatalog.PublicWeeklyDay{Weekday: weekday + 1, Periods: []publiccatalog.PublicWeeklyPeriod{}}
	}
	repo := &pageSizeRepository{spaces: []publiccatalog.Space{{
		SpaceID: spaceID, CategoryCode: "oficina", CategoryName: "Oficina", Title: "Sala",
		Description: "Sala privada", AreaM2: 20, RateUnit: "dia", BasePriceCLP: 5000, Currency: "CLP",
		RegionCode: &regionCode, RegionName: &regionName, CommuneCode: &communeCode, CommuneName: &communeName,
		CoverImageID: &coverImageID, EstimatedTotalCLP: &estimatedTotal,
		UsageRules: "Solo se muestra en el detalle", WeeklySchedule: &publiccatalog.PublicWeeklySchedule{
			Enabled: true, TimeZone: "America/Santiago", Days: days,
		},
	}}}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces", nil))
	var envelope struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid response JSON: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusOK || len(envelope.Items) != 1 || string(envelope.Items[0]["features"]) != "[]" || string(envelope.Items[0]["images"]) != "[]" {
		t.Fatalf("status=%d items=%s body=%s", response.Code, envelope.Items, response.Body.String())
	}
	assertPublicJSONKeys(t, envelope.Items[0], "space_id", "verification", "category_code", "category_name", "title", "description", "area_m2", "rate_unit", "base_price_clp", "currency", "region_code", "region_name", "comuna_code", "comuna_name", "estimated_total_clp", "features", "images", "cover_image_id")
	for key, want := range map[string]string{
		"region_code": `"13"`, "region_name": `"Metropolitana"`, "comuna_code": `"13101"`,
		"comuna_name": `"Santiago"`, "estimated_total_clp": "20000", "cover_image_id": `"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"`,
	} {
		if got := string(envelope.Items[0][key]); got != want {
			t.Errorf("%s=%s, want %s", key, got, want)
		}
	}
}

func TestPublicDetailIncludesRequiredEmptyFieldsAndCanonicalizesID(t *testing.T) {
	spaceID := "abcdefab-cdef-4abc-8def-abcdefabcdef"
	repo := &pageSizeRepository{detail: &publiccatalog.Space{
		SpaceID: spaceID, CategoryCode: "oficina", CategoryName: "Oficina", Title: "Sala",
		Description: "Sala privada", AreaM2: 20, RateUnit: "dia", BasePriceCLP: 5000, Currency: "CLP",
	}}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces/"+strings.ToUpper(spaceID), nil))
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid response JSON: %v body=%s", err, response.Body.String())
	}
	var usageRules string
	if err := json.Unmarshal(envelope.Data["usage_rules"], &usageRules); err != nil {
		t.Fatalf("usage_rules missing or invalid: %v data=%v", err, envelope.Data)
	}
	if response.Code != http.StatusOK || repo.detailCalls != 1 || repo.detailID != spaceID || usageRules != "" || string(envelope.Data["features"]) != "[]" || string(envelope.Data["images"]) != "[]" {
		t.Fatalf("status=%d calls=%d id=%q usage_rules=%q features=%s images=%s body=%s", response.Code, repo.detailCalls, repo.detailID, usageRules, envelope.Data["features"], envelope.Data["images"], response.Body.String())
	}
	assertPublicJSONKeys(t, envelope.Data, "space_id", "verification", "category_code", "category_name", "title", "description", "area_m2", "rate_unit", "base_price_clp", "currency", "estimated_total_clp", "features", "images", "usage_rules")
}

func TestPublicDetailRejectsMalformedIdentifiersAndQueryBeforeRepository(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "invalid UUID", path: "/api/v1/public/spaces/not-a-uuid"},
		{name: "extra path segment", path: "/api/v1/public/spaces/abcdefab-cdef-4abc-8def-abcdefabcdef/extra"},
		{name: "query not allowed", path: "/api/v1/public/spaces/abcdefab-cdef-4abc-8def-abcdefabcdef?owner_id=private"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != http.StatusNotFound || repo.detailCalls != 0 {
				t.Fatalf("status=%d detail_calls=%d body=%s", response.Code, repo.detailCalls, response.Body.String())
			}
		})
	}
}

func assertPublicJSONKeys(t *testing.T, got map[string]json.RawMessage, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("JSON keys=%v, want exactly %v", got, want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("JSON key %q missing from %v", key, got)
		}
	}
}

func TestPublicLocationsPassNormalizedFiltersToRepository(t *testing.T) {
	values := url.Values{}
	values.Set("q", "  Santiago\tCentro  ")
	values.Set("region_code", "13")
	values.Set("limit", "25")
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/locations?"+values.Encode(), nil))
	if response.Code != http.StatusOK || repo.locationCalls != 1 || repo.locationQuery != "Santiago Centro" || repo.locationRegion != "13" || repo.locationLimit != 25 {
		t.Fatalf("status=%d calls=%d query=%q region=%q limit=%d body=%s", response.Code, repo.locationCalls, repo.locationQuery, repo.locationRegion, repo.locationLimit, response.Body.String())
	}
}

func TestPublicLocationsRejectInvalidLimitsBeforeRepository(t *testing.T) {
	for _, query := range []string{
		"limit=0",
		"limit=401",
		"limit=not-a-number",
		"limit=1&limit=2",
	} {
		t.Run(query, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/locations?"+query, nil))
			if response.Code != http.StatusUnprocessableEntity || repo.locationCalls != 0 {
				t.Fatalf("status=%d location_calls=%d body=%s", response.Code, repo.locationCalls, response.Body.String())
			}
		})
	}
}

func TestPublicLocationsAcceptLimitBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "default", wantLimit: 400},
		{name: "minimum", query: "?limit=1", wantLimit: 1},
		{name: "maximum", query: "?limit=400", wantLimit: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pageSizeRepository{}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/locations"+tc.query, nil))
			if response.Code != http.StatusOK || repo.locationCalls != 1 || repo.locationLimit != tc.wantLimit {
				t.Fatalf("status=%d location_calls=%d limit=%d want=%d body=%s", response.Code, repo.locationCalls, repo.locationLimit, tc.wantLimit, response.Body.String())
			}
		})
	}
}

func TestPublicTypedAttributeSyntaxRejectsAmbiguity(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		ok     bool
	}{
		{"legacy false", []string{"wifi:false"}, true},
		{"malformed attribute code", []string{"bad-code:true"}, false},
		{"invalid boolean token", []string{"wifi:yes"}, false},
		{"duplicate boolean code", []string{"wifi:true", "wifi:false"}, false},
		{"two-sided range", []string{"altura:gte:1.25", "altura:lte:2.50"}, true},
		{"enum list OR", []string{"equipo:in:wifi", "equipo:in:proyector"}, true},
		{"enum list CSV", []string{"equipo:in:wifi,proyector"}, true},
		{"enum list mixed CSV and repeated", []string{"equipo:in:wifi,proyector", "equipo:in:calefaccion"}, true},
		{"duplicate bound", []string{"altura:gte:1", "altura:gte:2"}, false},
		{"mixed bool and range", []string{"wifi:false", "wifi:gte:1"}, false},
		{"mixed range then boolean", []string{"wifi:gte:1", "wifi:false"}, false},
		{"duplicate upper bound", []string{"altura:lte:1", "altura:lte:2"}, false},
		{"exact match then range", []string{"equipo:eq:wifi", "equipo:gte:1"}, false},
		{"range then exact match", []string{"altura:gte:1", "altura:eq:2"}, false},
		{"range then enum list", []string{"altura:gte:1", "altura:in:1"}, false},
		{"unknown operator", []string{"altura:gt:2"}, false},
		{"duplicate option", []string{"equipo:in:wifi", "equipo:in:wifi"}, false},
		{"duplicate CSV option", []string{"equipo:in:wifi,proyector,wifi"}, false},
		{"bad decimal", []string{"altura:gte:1e2"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := parsePublicAttributeFilters(tc.values)
			if ok != tc.ok {
				t.Fatalf("accepted=%v want=%v", ok, tc.ok)
			}
		})
	}
}

func TestPublicTypedAttributeEqualityPreservesExactValue(t *testing.T) {
	booleans, typed, ok := parsePublicAttributeFilters([]string{"wifi:true", "equipo:eq:proyector"})
	if !ok || len(booleans) != 1 || !booleans["wifi"] {
		t.Fatalf("boolean filters=%v typed=%v accepted=%v", booleans, typed, ok)
	}
	predicate, exists := typed["equipo"]
	if !exists || predicate.Value == nil || *predicate.Value != "proyector" || predicate.Minimum != nil || predicate.Maximum != nil || len(predicate.Values) != 0 {
		t.Fatalf("exact-match predicate=%+v exists=%v", predicate, exists)
	}
}

func TestPublicSpaceEnumFilterUsesServerProfileBeforeRepository(t *testing.T) {
	repo := &pageSizeRepository{attributeFilters: []publiccatalog.AttributeFilter{{
		Code: "equipo", Type: "enum", Options: []string{"wifi", "proyector"}, ProfileVersion: 7,
	}}}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces?category_code=oficina&attribute=equipo%3Aeq%3Awifi", nil))
	predicate, exists := repo.filters.TypedAttributes["equipo"]
	if response.Code != http.StatusOK || repo.filterCalls != 1 || repo.listCalls != 1 || !exists || predicate.Type != "enum" || predicate.Value == nil || *predicate.Value != "wifi" || repo.filters.AttributeVersion != 7 {
		t.Fatalf("status=%d profile_calls=%d list_calls=%d predicate=%+v version=%d body=%s", response.Code, repo.filterCalls, repo.listCalls, predicate, repo.filters.AttributeVersion, response.Body.String())
	}
}

func TestPublicSpaceEnumFilterRejectsUnknownOptionBeforeRepository(t *testing.T) {
	repo := &pageSizeRepository{attributeFilters: []publiccatalog.AttributeFilter{{
		Code: "equipo", Type: "enum", Options: []string{"wifi", "proyector"}, ProfileVersion: 7,
	}}}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces?category_code=oficina&attribute=equipo%3Aeq%3Acalefaccion", nil))
	if response.Code != http.StatusUnprocessableEntity || repo.filterCalls != 1 || repo.listCalls != 0 {
		t.Fatalf("status=%d profile_calls=%d list_calls=%d body=%s", response.Code, repo.filterCalls, repo.listCalls, response.Body.String())
	}
}

func TestPublicTypedAttributeParserEnforcesLimits(t *testing.T) {
	withinAttributeLimit := make([]string, 16)
	for i := range withinAttributeLimit {
		withinAttributeLimit[i] = "a" + strconv.Itoa(i) + ":true"
	}
	if booleans, _, ok := parsePublicAttributeFilters(withinAttributeLimit); !ok || len(booleans) != 16 {
		t.Fatalf("16 attributes accepted=%v count=%d", ok, len(booleans))
	}
	tooManyAttributes := append(append([]string(nil), withinAttributeLimit...), "a16:true")
	if _, _, ok := parsePublicAttributeFilters(tooManyAttributes); ok {
		t.Fatal("accepted more than 16 distinct attributes")
	}

	options := make([]string, 32)
	for i := range options {
		options[i] = "o" + strconv.Itoa(i)
	}
	if _, typed, ok := parsePublicAttributeFilters([]string{"equipo:in:" + strings.Join(options, ",")}); !ok || len(typed["equipo"].Values) != 32 {
		t.Fatalf("32 options accepted=%v typed=%v", ok, typed)
	}
	options = append(options, "o32")
	if _, _, ok := parsePublicAttributeFilters([]string{"equipo:in:" + strings.Join(options, ",")}); ok {
		t.Fatal("accepted more than 32 options")
	}
	if _, _, ok := parsePublicAttributeFilters([]string{"equipo:eq:" + strings.Repeat("a", 171)}); ok {
		t.Fatal("accepted attribute expression longer than 180 bytes")
	}
}

func TestPublicAttributeQueryLimit(t *testing.T) {
	values := url.Values{}
	for i := 0; i < 33; i++ {
		values.Add("attribute", "equipo:in:option")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces?"+values.Encode(), nil)
	if _, ok := publicFilters(request); ok {
		t.Fatal("accepted more than 32 repeated attribute filter values")
	}
}

func TestPublicCORSOriginPolicy(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service, "https://catalog.example")
	cases := []struct {
		name, method, origin string
		wantStatus           int
		wantAllowOrigin      string
	}{
		{name: "SSR request without Origin", method: http.MethodGet, wantStatus: http.StatusOK},
		{name: "allowed browser origin", method: http.MethodGet, origin: "https://catalog.example", wantStatus: http.StatusOK, wantAllowOrigin: "https://catalog.example"},
		{name: "allowed preflight", method: http.MethodOptions, origin: "https://catalog.example", wantStatus: http.StatusNoContent, wantAllowOrigin: "https://catalog.example"},
		{name: "untrusted origin", method: http.MethodGet, origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "untrusted preflight", method: http.MethodOptions, origin: "https://attacker.example", wantStatus: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "/api/v1/public/space-categories", nil)
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllowOrigin {
				t.Fatalf("Access-Control-Allow-Origin=%q, want %q", got, tc.wantAllowOrigin)
			}
			if tc.method == http.MethodOptions && tc.wantStatus == http.StatusNoContent && response.Header().Get("Access-Control-Allow-Methods") != "GET, HEAD, OPTIONS" {
				t.Fatalf("preflight methods=%q", response.Header().Get("Access-Control-Allow-Methods"))
			}
		})
	}
}

func TestPublicCatalogRejectsMultipleOriginHeadersBeforeRepository(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/space-categories", nil)
	request.Header.Add("Origin", "https://catalog.example")
	request.Header.Add("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	New(service, "https://catalog.example").ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || repo.categoryCalls != 0 || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("status=%d category_calls=%d allow_origin=%q body=%s", response.Code, repo.categoryCalls, response.Header().Get("Access-Control-Allow-Origin"), response.Body.String())
	}
}

type revocableMediaRepository struct {
	pageSizeRepository
	item              publiccatalog.PublicImage
	visible           bool
	variant           string
	variantCalls      int
	publicImageCalls  int
	mediaVariantCalls int
	err               error
}

func (r *revocableMediaRepository) PublicImage(context.Context, string, string) (publiccatalog.PublicImage, error) {
	r.publicImageCalls++
	return publiccatalog.PublicImage{}, publiccatalog.ErrNotFound
}

func (r *revocableMediaRepository) PublicMediaVariant(_ context.Context, _, _, variant string) (publiccatalog.PublicImage, error) {
	r.variantCalls++
	r.mediaVariantCalls++
	if r.err != nil {
		return publiccatalog.PublicImage{}, r.err
	}
	if !r.visible || variant != r.variant {
		return publiccatalog.PublicImage{}, publiccatalog.ErrNotFound
	}
	return r.item, nil
}

type publicMediaObjectStore struct {
	body       []byte
	key        string
	generation int64
	getCalls   int
	getErr     error
}

func (s *publicMediaObjectStore) Put(context.Context, string, []byte, string) (media.StoredObject, error) {
	return media.StoredObject{}, nil
}

func (s *publicMediaObjectStore) Get(_ context.Context, key string, generation int64) ([]byte, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	if key != s.key || generation != s.generation {
		return nil, media.ErrNotFound
	}
	return append([]byte(nil), s.body...), nil
}

func (s *publicMediaObjectStore) Generation(context.Context, string) (int64, error) {
	return s.generation, nil
}

func (s *publicMediaObjectStore) Delete(context.Context, string, int64) error { return nil }

type publicImageFileStore struct{ getCalls int }

func (s *publicImageFileStore) Get(context.Context, string) ([]byte, error) {
	s.getCalls++
	return nil, publiccatalog.ErrNotFound
}

func TestPublicMediaImageIsNotCachedAndRevocationTakesEffect(t *testing.T) {
	spaceID := "00000000-0000-4000-8000-000000000001"
	imageID := "00000000-0000-4000-8000-000000000002"
	body := []byte("webp-fixture")
	digest := sha256.Sum256(body)
	key := "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000003/1/display.webp"
	repo := &revocableMediaRepository{
		item:    publiccatalog.PublicImage{MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)), ObjectKey: key, Generation: 7},
		visible: true,
		variant: "display",
	}
	store := &publicMediaObjectStore{body: body, key: key, generation: 7}
	service, err := publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), &publicImageFileStore{}, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	var path string
	for _, variant := range []string{"display", "thumbnail"} {
		t.Run(variant, func(t *testing.T) {
			key = "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000003/1/" + variant + ".webp"
			repo.item.ObjectKey = key
			repo.variant = variant
			store.key = key
			path = "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/" + variant

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "image/webp" || response.Body.String() != string(body) {
				t.Fatalf("public %s image response status=%d cache=%q mime=%q body=%q", variant, response.Code, response.Header().Get("Cache-Control"), response.Header().Get("Content-Type"), response.Body.String())
			}
			if response.Header().Get("ETag") != `"`+hex.EncodeToString(digest[:])+`"` || response.Header().Get("Content-Length") != strconv.Itoa(len(body)) || response.Header().Get("Content-Disposition") != "inline; filename=space-image.webp" {
				t.Fatalf("public %s image headers etag=%q length=%q disposition=%q", variant, response.Header().Get("ETag"), response.Header().Get("Content-Length"), response.Header().Get("Content-Disposition"))
			}
		})
	}
	if repo.variantCalls != 2 || store.getCalls != 2 {
		t.Fatalf("valid variant reads repository=%d object-store=%d; want 2 each", repo.variantCalls, store.getCalls)
	}

	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, httptest.NewRequest(http.MethodPost, path, nil))
	if postResponse.Code != http.StatusMethodNotAllowed || postResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("POST status=%d cache=%q; want 405 and no-store", postResponse.Code, postResponse.Header().Get("Cache-Control"))
	}
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, httptest.NewRequest(http.MethodGet, path+"?cache=bust", nil))
	if queryResponse.Code != http.StatusNotFound || queryResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("query status=%d cache=%q; want 404 and no-store", queryResponse.Code, queryResponse.Header().Get("Cache-Control"))
	}
	if repo.variantCalls != 2 || store.getCalls != 2 {
		t.Fatalf("rejected route reached repository=%d object-store=%d; want 2 each", repo.variantCalls, store.getCalls)
	}
	invalidVariantPath := "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/avatar"
	invalidVariantResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidVariantResponse, httptest.NewRequest(http.MethodGet, invalidVariantPath, nil))
	if invalidVariantResponse.Code != http.StatusNotFound || invalidVariantResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("invalid variant status=%d cache=%q; want 404 and no-store", invalidVariantResponse.Code, invalidVariantResponse.Header().Get("Cache-Control"))
	}
	if repo.variantCalls != 2 || store.getCalls != 2 {
		t.Fatalf("invalid variant reached repository=%d object-store=%d; want 2 each", repo.variantCalls, store.getCalls)
	}
	store.generation = 8
	staleGenerationResponse := httptest.NewRecorder()
	handler.ServeHTTP(staleGenerationResponse, httptest.NewRequest(http.MethodGet, path, nil))
	if staleGenerationResponse.Code != http.StatusNotFound || staleGenerationResponse.Header().Get("Cache-Control") != "no-store" || staleGenerationResponse.Body.String() == string(body) {
		t.Fatalf("stale generation status=%d cache=%q body=%q; want hidden 404", staleGenerationResponse.Code, staleGenerationResponse.Header().Get("Cache-Control"), staleGenerationResponse.Body.String())
	}
	if repo.variantCalls != 3 || store.getCalls != 3 {
		t.Fatalf("stale generation repository=%d object-store=%d; want one revalidation and one rejected object read", repo.variantCalls, store.getCalls)
	}

	repo.visible = false
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("revoked public image response status=%d cache=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	if repo.variantCalls != 4 || store.getCalls != 3 {
		t.Fatalf("revoked route repository=%d object-store=%d; want one revalidation and no object read", repo.variantCalls, store.getCalls)
	}
}

func TestPublicMediaImageHidesRepositoryError(t *testing.T) {
	spaceID := "00000000-0000-4000-8000-000000000001"
	imageID := "00000000-0000-4000-8000-000000000002"
	body := []byte("webp-fixture")
	digest := sha256.Sum256(body)
	key := "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000003/1/display.webp"
	repo := &revocableMediaRepository{
		item:    publiccatalog.PublicImage{MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)), ObjectKey: key, Generation: 7},
		visible: true,
		variant: "display",
		err:     errors.New("private database sentinel"),
	}
	store := &publicMediaObjectStore{body: body, key: key, generation: 7}
	service, err := publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), &publicImageFileStore{}, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	path := "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), "internal_error") || strings.Contains(response.Body.String(), "private database sentinel") {
		t.Fatalf("repository error response status=%d cache=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	if repo.variantCalls != 1 || store.getCalls != 0 {
		t.Fatalf("repository calls=%d object-store reads=%d; want 1 and 0", repo.variantCalls, store.getCalls)
	}
}

func TestPublicMediaVariantCORSOriginPolicy(t *testing.T) {
	spaceID := "00000000-0000-4000-8000-000000000001"
	imageID := "00000000-0000-4000-8000-000000000002"
	body := []byte("webp-fixture")
	digest := sha256.Sum256(body)
	key := "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000003/1/display.webp"
	repo := &revocableMediaRepository{
		item:    publiccatalog.PublicImage{MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)), ObjectKey: key, Generation: 7},
		visible: true,
		variant: "display",
	}
	store := &publicMediaObjectStore{body: body, key: key, generation: 7}
	service, err := publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), &publicImageFileStore{}, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service, "https://catalog.example")
	path := "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display"

	untrusted := httptest.NewRequest(http.MethodGet, path, nil)
	untrusted.Header.Set("Origin", "https://attacker.example")
	untrustedResponse := httptest.NewRecorder()
	handler.ServeHTTP(untrustedResponse, untrusted)
	if untrustedResponse.Code != http.StatusForbidden || untrustedResponse.Header().Get("Access-Control-Allow-Origin") != "" || untrustedResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("untrusted CORS status=%d allow-origin=%q cache=%q", untrustedResponse.Code, untrustedResponse.Header().Get("Access-Control-Allow-Origin"), untrustedResponse.Header().Get("Cache-Control"))
	}
	if repo.variantCalls != 0 || store.getCalls != 0 {
		t.Fatalf("untrusted origin reached repository=%d object-store=%d", repo.variantCalls, store.getCalls)
	}

	preflight := httptest.NewRequest(http.MethodOptions, path, nil)
	preflight.Header.Set("Origin", "https://catalog.example")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodGet)
	preflightResponse := httptest.NewRecorder()
	handler.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusNoContent || preflightResponse.Header().Get("Access-Control-Allow-Origin") != "https://catalog.example" || preflightResponse.Header().Get("Access-Control-Allow-Methods") != "GET, HEAD, OPTIONS" {
		t.Fatalf("media preflight status=%d allow-origin=%q methods=%q", preflightResponse.Code, preflightResponse.Header().Get("Access-Control-Allow-Origin"), preflightResponse.Header().Get("Access-Control-Allow-Methods"))
	}
	if repo.variantCalls != 0 || store.getCalls != 0 {
		t.Fatalf("preflight reached repository=%d object-store=%d", repo.variantCalls, store.getCalls)
	}

	allowed := httptest.NewRequest(http.MethodGet, path, nil)
	allowed.Header.Set("Origin", "https://catalog.example")
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusOK || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://catalog.example" || allowedResponse.Header().Get("Vary") != "Origin" || allowedResponse.Header().Get("Access-Control-Allow-Credentials") != "" || allowedResponse.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("allowed media CORS status=%d allow-origin=%q vary=%q credentials=%q mime=%q", allowedResponse.Code, allowedResponse.Header().Get("Access-Control-Allow-Origin"), allowedResponse.Header().Get("Vary"), allowedResponse.Header().Get("Access-Control-Allow-Credentials"), allowedResponse.Header().Get("Content-Type"))
	}
	if repo.variantCalls != 1 || store.getCalls != 1 {
		t.Fatalf("allowed origin repository=%d object-store=%d; want 1 each", repo.variantCalls, store.getCalls)
	}
}

func TestPublicMediaImageHidesObjectStoreError(t *testing.T) {
	spaceID := "00000000-0000-4000-8000-000000000001"
	imageID := "00000000-0000-4000-8000-000000000002"
	body := []byte("webp-fixture")
	digest := sha256.Sum256(body)
	key := "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000003/1/display.webp"
	repo := &revocableMediaRepository{
		item:    publiccatalog.PublicImage{MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)), ObjectKey: key, Generation: 7},
		visible: true,
		variant: "display",
	}
	store := &publicMediaObjectStore{body: body, key: key, generation: 7, getErr: errors.New("private object-store sentinel")}
	service, err := publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), &publicImageFileStore{}, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	path := "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "private object-store sentinel") || strings.Contains(response.Body.String(), string(body)) {
		t.Fatalf("object-store error response status=%d cache=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	if repo.variantCalls != 1 || store.getCalls != 1 {
		t.Fatalf("repository calls=%d object-store reads=%d; want 1 each", repo.variantCalls, store.getCalls)
	}
}

func TestPublicMediaRouteRejectsInvalidPathValuesBeforeRepository(t *testing.T) {
	spaceID := "00000000-0000-4000-8000-000000000001"
	imageID := "00000000-0000-4000-8000-000000000002"
	repo := &revocableMediaRepository{visible: true}
	store := &publicMediaObjectStore{}
	fileStore := &publicImageFileStore{}
	service, err := publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), fileStore, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	cases := []struct {
		name string
		path string
	}{
		{name: "invalid space UUID", path: "/api/v1/public/spaces/not-a-uuid/images/" + imageID + "/variants/display"},
		{name: "invalid image UUID", path: "/api/v1/public/spaces/" + spaceID + "/images/not-a-uuid/variants/display"},
		{name: "nested space path", path: "/api/v1/public/spaces/" + spaceID + "/extra/images/" + imageID + "/variants/display"},
		{name: "invalid legacy space UUID", path: "/api/v1/public/spaces/not-a-uuid/images/" + imageID},
		{name: "invalid legacy image UUID", path: "/api/v1/public/spaces/" + spaceID + "/images/not-a-uuid"},
		{name: "unknown variant", path: "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/unknown"},
		{name: "extra variant segment", path: "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display/extra"},
		{name: "extra legacy image segment", path: "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/extra"},
		{name: "unexpected query", path: "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display?key=value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getResponse := httptest.NewRecorder()
			handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, tc.path, nil))
			headResponse := httptest.NewRecorder()
			handler.ServeHTTP(headResponse, httptest.NewRequest(http.MethodHead, tc.path, nil))
			if getResponse.Code != http.StatusNotFound || headResponse.Code != getResponse.Code || headResponse.Body.Len() != 0 || headResponse.Header().Get("Cache-Control") != "no-store" || headResponse.Header().Get("Content-Length") != strconv.Itoa(getResponse.Body.Len()) {
				t.Fatalf("GET/HEAD status=%d/%d HEAD body_len=%d length=%q GET length=%d cache=%q", getResponse.Code, headResponse.Code, headResponse.Body.Len(), headResponse.Header().Get("Content-Length"), getResponse.Body.Len(), headResponse.Header().Get("Cache-Control"))
			}
		})
	}
	if repo.publicImageCalls != 0 || repo.mediaVariantCalls != 0 || fileStore.getCalls != 0 || store.getCalls != 0 {
		t.Fatalf("invalid routes reached repository/store: image_calls=%d variant_calls=%d file_get_calls=%d media_get_calls=%d", repo.publicImageCalls, repo.mediaVariantCalls, fileStore.getCalls, store.getCalls)
	}
}

type imageLookupFailureRepository struct {
	pageSizeRepository
	err          error
	imageCalls   int
	variantCalls int
}

func (r *imageLookupFailureRepository) PublicImage(context.Context, string, string) (publiccatalog.PublicImage, error) {
	r.imageCalls++
	return publiccatalog.PublicImage{}, r.err
}

func (r *imageLookupFailureRepository) PublicMediaVariant(context.Context, string, string, string) (publiccatalog.PublicImage, error) {
	r.variantCalls++
	return publiccatalog.PublicImage{}, r.err
}

func TestPublicImageMetadataRepositoryFailuresAreHiddenBeforeStorageRead(t *testing.T) {
	const (
		spaceID = "00000000-0000-4000-8000-000000000001"
		imageID = "00000000-0000-4000-8000-000000000002"
	)
	cases := []struct {
		name        string
		path        string
		media       bool
		wantImage   int
		wantVariant int
	}{
		{name: "legacy PNG metadata", path: "/api/v1/public/spaces/" + spaceID + "/images/" + imageID, wantImage: 1},
		{name: "normalized WebP metadata", path: "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display", media: true, wantVariant: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := errors.New("synthetic private image metadata failure")
			repo := &imageLookupFailureRepository{err: failure}
			files := &publicImageFileStore{}
			objects := &publicMediaObjectStore{}
			var service *publiccatalog.Service
			var err error
			if tc.media {
				service, err = publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), files, objects)
			} else {
				service, err = publiccatalog.NewWithImageStore(repo, []byte("01234567890123456789012345678901"), files)
			}
			if err != nil {
				t.Fatal(err)
			}

			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), failure.Error()) {
				t.Fatalf("status=%d cache=%q leaked=%t body=%s", response.Code, response.Header().Get("Cache-Control"), strings.Contains(response.Body.String(), failure.Error()), response.Body.String())
			}
			if repo.imageCalls != tc.wantImage || repo.variantCalls != tc.wantVariant || files.getCalls != 0 || objects.getCalls != 0 {
				t.Fatalf("repository calls=%d/%d storage calls=%d/%d; want %d/%d and zero storage reads", repo.imageCalls, repo.variantCalls, files.getCalls, objects.getCalls, tc.wantImage, tc.wantVariant)
			}
		})
	}
}

type legacyPublicImageRepository struct {
	pageSizeRepository
	item  publiccatalog.PublicImage
	calls int
}

func (r *legacyPublicImageRepository) PublicImage(context.Context, string, string) (publiccatalog.PublicImage, error) {
	r.calls++
	return r.item, nil
}

func (r *legacyPublicImageRepository) PublicMediaVariant(context.Context, string, string, string) (publiccatalog.PublicImage, error) {
	return publiccatalog.PublicImage{}, publiccatalog.ErrNotFound
}

type legacyPublicImageStore struct {
	fileID string
	body   []byte
	calls  int
}

func (s *legacyPublicImageStore) Get(_ context.Context, fileID string) ([]byte, error) {
	s.calls++
	if fileID != s.fileID {
		return nil, media.ErrNotFound
	}
	return append([]byte(nil), s.body...), nil
}

func TestLegacyPublicPNGRoutePreservesHeadersForGETAndHEAD(t *testing.T) {
	const (
		spaceID = "00000000-0000-4000-8000-000000000001"
		imageID = "00000000-0000-4000-8000-000000000002"
		fileID  = "synthetic-public-image-file"
	)
	body := []byte("\x89PNG\r\n\x1a\nsynthetic-png")
	digest := sha256.Sum256(body)
	repo := &legacyPublicImageRepository{item: publiccatalog.PublicImage{
		MIME: "image/png", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)), FileID: fileID,
	}}
	store := &legacyPublicImageStore{fileID: fileID, body: body}
	service, err := publiccatalog.NewWithImageStore(repo, []byte("01234567890123456789012345678901"), store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	path := "/api/v1/public/spaces/" + spaceID + "/images/" + imageID
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || response.Header().Get("Content-Length") != strconv.Itoa(len(body)) ||
				response.Header().Get("Content-Disposition") != "inline; filename=espacigo-public-image.png" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d type=%q length=%q disposition=%q cache=%q", response.Code, response.Header().Get("Content-Type"), response.Header().Get("Content-Length"), response.Header().Get("Content-Disposition"), response.Header().Get("Cache-Control"))
			}
			if method == http.MethodHead && response.Body.Len() != 0 {
				t.Fatalf("HEAD body length=%d, want 0", response.Body.Len())
			}
			if method == http.MethodGet && response.Body.String() != string(body) {
				t.Fatalf("GET body=%q, want synthetic PNG bytes", response.Body.String())
			}
		})
	}
	if repo.calls != 2 || store.calls != 2 {
		t.Fatalf("repository/store calls=%d/%d, want 2/2", repo.calls, store.calls)
	}
}

func TestPublicSpaceDetailRepositoryErrorIsHidden(t *testing.T) {
	const spaceID = "00000000-0000-4000-8000-000000000001"
	failure := errors.New("synthetic private listing repository detail")
	repo := &pageSizeRepository{detailErr: failure}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces/"+spaceID, nil))
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid public error envelope: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), failure.Error()) || envelope["error"] == nil || envelope["data"] != nil || repo.detailCalls != 1 || repo.detailID != spaceID {
		t.Fatalf("status=%d cache=%q leaked=%t has_error=%t has_data=%t detail_calls=%d id=%q body=%s", response.Code, response.Header().Get("Cache-Control"), strings.Contains(response.Body.String(), failure.Error()), envelope["error"] != nil, envelope["data"] != nil, repo.detailCalls, repo.detailID, response.Body.String())
	}
}

type discoveryEndpointFailureRepository struct {
	pageSizeRepository
	categoriesErr       error
	filtersErr          error
	locationsErr        error
	categoryActive      bool
	categoriesCalls     int
	categoryChecks      int
	attributeFilterCall int
	locationReads       int
}

func (r *discoveryEndpointFailureRepository) Categories(context.Context) ([]publiccatalog.Category, error) {
	r.categoriesCalls++
	return nil, r.categoriesErr
}

func (r *discoveryEndpointFailureRepository) HasActiveCategory(context.Context, string) (bool, error) {
	r.categoryChecks++
	return r.categoryActive, nil
}

func (r *discoveryEndpointFailureRepository) AttributeFilters(context.Context, string) ([]publiccatalog.AttributeFilter, error) {
	r.attributeFilterCall++
	return nil, r.filtersErr
}

func (r *discoveryEndpointFailureRepository) Locations(context.Context, string, string, int) ([]publiccatalog.Location, error) {
	r.locationReads++
	return nil, r.locationsErr
}

func TestPublicDiscoveryErrorsAreMappedWithoutLeakingRepositoryDetails(t *testing.T) {
	cases := []struct {
		name                 string
		path                 string
		status               int
		categoryActive       bool
		categoriesErr        bool
		filtersErr           bool
		locationsErr         bool
		wantCategories       int
		wantCategoryChecks   int
		wantAttributeFilters int
		wantLocationReads    int
	}{
		{name: "category listing repository failure", path: "/api/v1/public/space-categories", status: http.StatusInternalServerError, categoriesErr: true, wantCategories: 1},
		{name: "inactive category filter profile", path: "/api/v1/public/space-categories/oficina/filters", status: http.StatusNotFound, wantCategoryChecks: 1},
		{name: "filter profile repository failure", path: "/api/v1/public/space-categories/oficina/filters", status: http.StatusInternalServerError, categoryActive: true, filtersErr: true, wantCategoryChecks: 1, wantAttributeFilters: 1},
		{name: "location repository failure", path: "/api/v1/public/locations?q=Santiago", status: http.StatusInternalServerError, locationsErr: true, wantLocationReads: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := errors.New("synthetic public discovery repository detail")
			repo := &discoveryEndpointFailureRepository{categoryActive: tc.categoryActive}
			if tc.categoriesErr {
				repo.categoriesErr = failure
			}
			if tc.filtersErr {
				repo.filtersErr = failure
			}
			if tc.locationsErr {
				repo.locationsErr = failure
			}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid public error envelope: %v body=%s", err, response.Body.String())
			}
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" || envelope["error"] == nil || envelope["items"] != nil || strings.Contains(response.Body.String(), failure.Error()) {
				t.Fatalf("status=%d want=%d cache=%q has_error=%t has_items=%t leaked=%t body=%s", response.Code, tc.status, response.Header().Get("Cache-Control"), envelope["error"] != nil, envelope["items"] != nil, strings.Contains(response.Body.String(), failure.Error()), response.Body.String())
			}
			if repo.categoriesCalls != tc.wantCategories || repo.categoryChecks != tc.wantCategoryChecks || repo.attributeFilterCall != tc.wantAttributeFilters || repo.locationReads != tc.wantLocationReads {
				t.Fatalf("repository calls categories=%d category=%d filters=%d locations=%d; want %d/%d/%d/%d", repo.categoriesCalls, repo.categoryChecks, repo.attributeFilterCall, repo.locationReads, tc.wantCategories, tc.wantCategoryChecks, tc.wantAttributeFilters, tc.wantLocationReads)
			}
		})
	}
}

func TestPublicUnknownRouteReturnsGenericNoStore404ForGETAndHEAD(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	path := "/api/v1/public/unregistered"
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, path, nil))
	headResponse := httptest.NewRecorder()
	handler.ServeHTTP(headResponse, httptest.NewRequest(http.MethodHead, path, nil))
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(getResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid public error envelope: %v body=%s", err, getResponse.Body.String())
	}
	if getResponse.Code != http.StatusNotFound || headResponse.Code != getResponse.Code || getResponse.Header().Get("Cache-Control") != "no-store" || headResponse.Header().Get("Cache-Control") != "no-store" || envelope["error"] == nil || headResponse.Body.Len() != 0 || headResponse.Header().Get("Content-Length") != strconv.Itoa(getResponse.Body.Len()) {
		t.Fatalf("GET/HEAD status=%d/%d cache=%q/%q has_error=%t HEAD_body=%d length=%q GET_length=%d", getResponse.Code, headResponse.Code, getResponse.Header().Get("Cache-Control"), headResponse.Header().Get("Cache-Control"), envelope["error"] != nil, headResponse.Body.Len(), headResponse.Header().Get("Content-Length"), getResponse.Body.Len())
	}
	if repo.categoryCalls != 0 || repo.filterCalls != 0 || repo.listCalls != 0 || repo.locationCalls != 0 || repo.detailCalls != 0 {
		t.Fatalf("unknown route reached repository: categories=%d filters=%d list=%d locations=%d detail=%d", repo.categoryCalls, repo.filterCalls, repo.listCalls, repo.locationCalls, repo.detailCalls)
	}
}

type failingEntropyReader struct{}

func (failingEntropyReader) Read([]byte) (int, error) {
	return 0, errors.New("synthetic entropy failure")
}

func TestRequestIDEntropyFailureStillUsesPrivateErrorHeaders(t *testing.T) {
	repo := &pageSizeRepository{}
	service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	originalReader := rand.Reader
	rand.Reader = failingEntropyReader{}
	defer func() { rand.Reader = originalReader }()

	handler := New(service)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/public/spaces", nil))
			if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" ||
				response.Header().Get("Content-Type") != "application/json; charset=utf-8" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("status=%d cache=%q content_type=%q nosniff=%q", response.Code, response.Header().Get("Cache-Control"), response.Header().Get("Content-Type"), response.Header().Get("X-Content-Type-Options"))
			}
			if method == http.MethodHead && response.Body.Len() != 0 {
				t.Fatalf("HEAD body length=%d, want 0", response.Body.Len())
			}
		})
	}
	if repo.listCalls != 0 {
		t.Fatalf("request ID entropy failure reached repository %d times", repo.listCalls)
	}
}
