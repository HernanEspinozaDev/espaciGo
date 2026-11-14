package publiccatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type discoveryRepository struct {
	memoryRepository

	categories        []Category
	categoriesErr     error
	categoriesCalls   int
	categoryActive    bool
	categoryErr       error
	categoryCalls     int
	filters           []AttributeFilter
	filtersErr        error
	filterCalls       int
	regionActive      bool
	regionErr         error
	regionCalls       int
	communeActive     bool
	communeErr        error
	communeCalls      int
	listCalls         int
	lastListFilters   Filters
	locations         []Location
	locationsErr      error
	locationCalls     int
	lastQuery         string
	lastRegion        string
	lastLocationLimit int
}

func (r *discoveryRepository) Categories(context.Context) ([]Category, error) {
	r.categoriesCalls++
	return r.categories, r.categoriesErr
}

func (r *discoveryRepository) HasActiveCategory(context.Context, string) (bool, error) {
	r.categoryCalls++
	return r.categoryActive, r.categoryErr
}

func (r *discoveryRepository) AttributeFilters(context.Context, string) ([]AttributeFilter, error) {
	r.filterCalls++
	return r.filters, r.filtersErr
}

func (r *discoveryRepository) HasRegion(context.Context, string) (bool, error) {
	r.regionCalls++
	return r.regionActive, r.regionErr
}

func (r *discoveryRepository) HasCommune(context.Context, string, string) (bool, error) {
	r.communeCalls++
	return r.communeActive, r.communeErr
}

func (r *discoveryRepository) List(ctx context.Context, filters Filters, after *Cursor, limit int) ([]Space, error) {
	r.listCalls++
	r.lastListFilters = filters
	return r.memoryRepository.List(ctx, filters, after, limit)
}

func (r *discoveryRepository) Locations(_ context.Context, query, region string, limit int) ([]Location, error) {
	r.locationCalls++
	r.lastQuery, r.lastRegion, r.lastLocationLimit = query, region, limit
	return r.locations, r.locationsErr
}

func newDiscoveryService(t *testing.T, repo *discoveryRepository) *Service {
	t.Helper()
	service, err := New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return service
}

func TestCategoriesReturnsRepositoryResultAndError(t *testing.T) {
	want := []Category{{Code: "oficina", Name: "Oficina", Order: 1}}
	repo := &discoveryRepository{categories: want}
	service := newDiscoveryService(t, repo)
	got, err := service.Categories(context.Background())
	if err != nil || len(got) != 1 || got[0] != want[0] || repo.categoriesCalls != 1 {
		t.Fatalf("Categories() = %#v, %v; calls=%d", got, err, repo.categoriesCalls)
	}

	failure := errors.New("synthetic repository failure")
	repo.categoriesErr = failure
	if _, err := service.Categories(context.Background()); !errors.Is(err, failure) || repo.categoriesCalls != 2 {
		t.Fatalf("Categories() error=%v calls=%d, want repository error", err, repo.categoriesCalls)
	}
}

func TestAttributeFiltersRequiresActiveCategoryBeforeReadingProfile(t *testing.T) {
	repo := &discoveryRepository{filters: []AttributeFilter{{Code: "wifi", Type: "boolean"}}}
	service := newDiscoveryService(t, repo)
	if _, err := service.AttributeFilters(context.Background(), "oficina"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inactive category error=%v, want ErrNotFound", err)
	}
	if repo.categoryCalls != 1 || repo.filterCalls != 0 {
		t.Fatalf("inactive category reads: category=%d filters=%d", repo.categoryCalls, repo.filterCalls)
	}

	repo.categoryActive = true
	got, err := service.AttributeFilters(context.Background(), "oficina")
	if err != nil || len(got) != 1 || got[0].Code != "wifi" || repo.filterCalls != 1 {
		t.Fatalf("active category filters=%#v err=%v calls=%d", got, err, repo.filterCalls)
	}

	failure := errors.New("synthetic attribute profile read failure")
	repo.filtersErr = failure
	if _, err := service.AttributeFilters(context.Background(), "oficina"); !errors.Is(err, failure) {
		t.Fatalf("AttributeFilters() error=%v, want repository error", err)
	}
	if repo.categoryCalls != 3 || repo.filterCalls != 2 {
		t.Fatalf("profile failure calls category=%d filters=%d, want 3/2", repo.categoryCalls, repo.filterCalls)
	}

	categoryFailure := errors.New("synthetic category lookup failure")
	repo.categoryErr = categoryFailure
	if _, err := service.AttributeFilters(context.Background(), "oficina"); !errors.Is(err, categoryFailure) {
		t.Fatalf("AttributeFilters() category error=%v, want lookup error", err)
	}
	if repo.categoryCalls != 4 || repo.filterCalls != 2 {
		t.Fatalf("category lookup failure calls category=%d filters=%d, want 4/2", repo.categoryCalls, repo.filterCalls)
	}
}

func TestLocationsValidatesBeforeReadsAndNormalizesAcceptedQuery(t *testing.T) {
	invalid := []struct {
		name, query, region string
		limit               int
	}{
		{name: "query exceeds rune limit", query: strings.Repeat("é", 81), region: "13", limit: 10},
		{name: "malformed region", query: "Santiago", region: "1x", limit: 10},
		{name: "zero limit", query: "Santiago", region: "13", limit: 0},
		{name: "excessive limit", query: "Santiago", region: "13", limit: 401},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{regionActive: true}
			service := newDiscoveryService(t, repo)
			if _, err := service.Locations(context.Background(), tc.query, tc.region, tc.limit); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Locations() error=%v, want ErrInvalid", err)
			}
			if repo.regionCalls != 0 || repo.locationCalls != 0 {
				t.Fatalf("invalid request reached repository: region=%d locations=%d", repo.regionCalls, repo.locationCalls)
			}
		})
	}

	repo := &discoveryRepository{regionActive: true, locations: []Location{{RegionCode: "13", CommuneCode: "13101", CommuneName: "Santiago"}}}
	service := newDiscoveryService(t, repo)
	got, err := service.Locations(context.Background(), "  Cafe\u0301\t Centro  ", "13", 25)
	if err != nil || len(got) != 1 || got[0].CommuneCode != "13101" {
		t.Fatalf("Locations() = %#v, %v", got, err)
	}
	if repo.regionCalls != 1 || repo.locationCalls != 1 || repo.lastQuery != "Café Centro" || repo.lastRegion != "13" || repo.lastLocationLimit != 25 {
		t.Fatalf("repository args/calls: query=%q region=%q limit=%d regionCalls=%d locationCalls=%d", repo.lastQuery, repo.lastRegion, repo.lastLocationLimit, repo.regionCalls, repo.locationCalls)
	}
}

func TestLocationsRejectsInactiveRegionAndStopsOnLookupError(t *testing.T) {
	repo := &discoveryRepository{}
	service := newDiscoveryService(t, repo)
	if _, err := service.Locations(context.Background(), "Santiago", "13", 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("inactive region error=%v, want ErrInvalid", err)
	}
	if repo.regionCalls != 1 || repo.locationCalls != 0 {
		t.Fatalf("inactive region reads: region=%d locations=%d", repo.regionCalls, repo.locationCalls)
	}

	failure := errors.New("synthetic region lookup failure")
	repo.regionErr = failure
	if _, err := service.Locations(context.Background(), "Santiago", "13", 10); !errors.Is(err, failure) {
		t.Fatalf("region lookup error=%v, want wrapped repository error", err)
	}
	if repo.regionCalls != 2 || repo.locationCalls != 0 {
		t.Fatalf("failed region lookup reads: region=%d locations=%d", repo.regionCalls, repo.locationCalls)
	}
}

func TestListRejectsInactiveOrUnverifiableCatalogFiltersBeforeListing(t *testing.T) {
	categoryFailure := errors.New("synthetic category lookup failure")
	regionFailure := errors.New("synthetic region lookup failure")
	communeFailure := errors.New("synthetic commune lookup failure")
	cases := []struct {
		name                         string
		filters                      Filters
		categoryActive, regionActive bool
		communeActive                bool
		categoryErr, regionErr       error
		communeErr                   error
		wantErr                      error
		wantCategory, wantRegion     int
		wantCommune                  int
	}{
		{name: "inactive category", filters: Filters{CategoryCode: "oficina"}, wantErr: ErrInvalid, wantCategory: 1},
		{name: "category lookup error", filters: Filters{CategoryCode: "oficina"}, categoryErr: categoryFailure, wantErr: categoryFailure, wantCategory: 1},
		{name: "inactive region", filters: Filters{RegionCode: "13"}, wantErr: ErrInvalid, wantRegion: 1},
		{name: "region lookup error", filters: Filters{RegionCode: "13"}, regionErr: regionFailure, wantErr: regionFailure, wantRegion: 1},
		{name: "inactive commune", filters: Filters{RegionCode: "13", CommuneCode: "13101"}, regionActive: true, wantErr: ErrInvalid, wantRegion: 1, wantCommune: 1},
		{name: "commune lookup error", filters: Filters{RegionCode: "13", CommuneCode: "13101"}, regionActive: true, communeErr: communeFailure, wantErr: communeFailure, wantRegion: 1, wantCommune: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{
				categoryActive: tc.categoryActive, categoryErr: tc.categoryErr,
				regionActive: tc.regionActive, regionErr: tc.regionErr,
				communeActive: tc.communeActive, communeErr: tc.communeErr,
			}
			service := newDiscoveryService(t, repo)
			if _, err := service.List(context.Background(), tc.filters); !errors.Is(err, tc.wantErr) {
				t.Fatalf("List() error=%v, want %v", err, tc.wantErr)
			}
			if repo.categoryCalls != tc.wantCategory || repo.regionCalls != tc.wantRegion || repo.communeCalls != tc.wantCommune || repo.listCalls != 0 {
				t.Fatalf("calls category=%d region=%d commune=%d list=%d; want %d/%d/%d/0", repo.categoryCalls, repo.regionCalls, repo.communeCalls, repo.listCalls, tc.wantCategory, tc.wantRegion, tc.wantCommune)
			}
		})
	}
}

func TestListFailsClosedWhenAttributeProfileIsUnavailableOrAmbiguous(t *testing.T) {
	failure := errors.New("synthetic attribute profile failure")
	cases := []struct {
		name       string
		filters    []AttributeFilter
		filtersErr error
		wantErr    error
	}{
		{name: "profile read failure", filtersErr: failure, wantErr: failure},
		{
			name: "duplicate profile code",
			filters: []AttributeFilter{
				{Code: "wifi", Type: "boolean", ProfileVersion: 1},
				{Code: "wifi", Type: "boolean", ProfileVersion: 1},
			},
			wantErr: ErrInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{categoryActive: true, filters: tc.filters, filtersErr: tc.filtersErr}
			service := newDiscoveryService(t, repo)
			request := Filters{CategoryCode: "oficina", Attributes: map[string]bool{"wifi": true}}
			if _, err := service.List(context.Background(), request); !errors.Is(err, tc.wantErr) {
				t.Fatalf("List() error=%v, want %v", err, tc.wantErr)
			}
			if repo.categoryCalls != 1 || repo.filterCalls != 1 || repo.listCalls != 0 {
				t.Fatalf("calls category=%d profile=%d list=%d; want 1/1/0", repo.categoryCalls, repo.filterCalls, repo.listCalls)
			}
		})
	}
}

func TestListValidatesLegacyBooleanFiltersAgainstCurrentAttributeProfile(t *testing.T) {
	profile := []AttributeFilter{
		{Code: "wifi", Type: "boolean", ProfileVersion: 7},
		{Code: "altura", Type: "number", ProfileVersion: 7},
	}
	cases := []struct {
		name             string
		attributes       map[string]bool
		attributeVersion int
		wantErr          error
		wantCalls        int
		wantVersion      int
	}{
		{
			name:        "valid filter receives current profile version",
			attributes:  map[string]bool{"wifi": true},
			wantCalls:   1,
			wantVersion: 7,
		},
		{
			name:       "unknown filter is rejected",
			attributes: map[string]bool{"unknown": true},
			wantErr:    ErrInvalid,
		},
		{
			name:       "nonboolean filter is rejected",
			attributes: map[string]bool{"altura": true},
			wantErr:    ErrInvalid,
		},
		{
			name:             "stale profile version is rejected",
			attributes:       map[string]bool{"wifi": true},
			attributeVersion: 6,
			wantErr:          ErrInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{categoryActive: true, filters: profile}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), Filters{
				CategoryCode:     "oficina",
				PageSize:         20,
				Attributes:       tc.attributes,
				AttributeVersion: tc.attributeVersion,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("List() error=%v, want %v", err, tc.wantErr)
			}
			if repo.listCalls != tc.wantCalls {
				t.Fatalf("repository List calls=%d, want %d", repo.listCalls, tc.wantCalls)
			}
			if tc.wantCalls > 0 && repo.lastListFilters.AttributeVersion != tc.wantVersion {
				t.Fatalf("repository received profile version=%d, want %d", repo.lastListFilters.AttributeVersion, tc.wantVersion)
			}
		})
	}
}

func TestListRejectsLegacyBooleanFiltersFromDifferentProfileVersions(t *testing.T) {
	repo := &discoveryRepository{
		categoryActive: true,
		filters: []AttributeFilter{
			{Code: "wifi", Type: "boolean", ProfileVersion: 7},
			{Code: "proyector", Type: "boolean", ProfileVersion: 8},
		},
	}
	service := newDiscoveryService(t, repo)
	_, err := service.List(context.Background(), Filters{
		CategoryCode: "oficina",
		PageSize:     20,
		Attributes:   map[string]bool{"wifi": true, "proyector": true},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("mixed attribute profile versions error=%v, want ErrInvalid", err)
	}
	if repo.listCalls != 0 {
		t.Fatalf("mixed profile versions reached listing repository %d times", repo.listCalls)
	}
}

func TestListValidatesNumericPredicatesAgainstProfileBoundsAndStep(t *testing.T) {
	number := func(value string) *json.Number {
		parsed := json.Number(value)
		return &parsed
	}
	profile := func(minimum, maximum, step string) []AttributeFilter {
		return []AttributeFilter{{Code: "units", Type: "integer", Minimum: number(minimum), Maximum: number(maximum), Step: number(step), ProfileVersion: 3}}
	}
	cases := []struct {
		name      string
		profile   []AttributeFilter
		predicate AttributePredicate
		wantErr   error
		wantCalls int
	}{
		{name: "aligned range accepted", profile: profile("1", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer("3"), Maximum: stringPointer("5")}, wantCalls: 1},
		{name: "fractional integer rejected", profile: profile("1", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer("2.5")}, wantErr: ErrInvalid},
		{name: "noncanonical leading zero rejected", profile: profile("1", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer("03")}, wantErr: ErrInvalid},
		{name: "oversized decimal rejected", profile: profile("1", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer(strings.Repeat("1", 65))}, wantErr: ErrInvalid},
		{name: "step mismatch rejected", profile: profile("1", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer("4")}, wantErr: ErrInvalid},
		{name: "reversed range rejected", profile: profile("1", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer("7"), Maximum: stringPointer("3")}, wantErr: ErrInvalid},
		{name: "malformed profile step rejected", profile: profile("1", "9", "0"), predicate: AttributePredicate{Minimum: stringPointer("3")}, wantErr: ErrInvalid},
		{name: "malformed profile minimum rejected", profile: profile("invalid", "9", "2"), predicate: AttributePredicate{Minimum: stringPointer("3")}, wantErr: ErrInvalid},
		{name: "malformed profile maximum rejected", profile: profile("1", "invalid", "2"), predicate: AttributePredicate{Maximum: stringPointer("3")}, wantErr: ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{categoryActive: true, filters: tc.profile}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), Filters{
				CategoryCode:    "oficina",
				TypedAttributes: map[string]AttributePredicate{"units": tc.predicate},
			})
			if !errors.Is(err, tc.wantErr) || repo.listCalls != tc.wantCalls {
				t.Fatalf("List() error=%v list_calls=%d, want error=%v calls=%d", err, repo.listCalls, tc.wantErr, tc.wantCalls)
			}
		})
	}
}

func TestListValidatesPricingConstraintsBeforeRepositoryListing(t *testing.T) {
	start := time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	amount := func(value int64) *int64 { return &value }
	cases := []struct {
		name      string
		filters   Filters
		wantErr   error
		wantCalls int
	}{
		{name: "estimated total sort requires interval", filters: Filters{Sort: "estimated_total_asc"}, wantErr: ErrInvalid},
		{name: "negative minimum total rejected", filters: Filters{StartAt: &start, EndAt: &end, MinTotalCLP: amount(-1)}, wantErr: ErrInvalid},
		{name: "inverted total range rejected", filters: Filters{StartAt: &start, EndAt: &end, MinTotalCLP: amount(200), MaxTotalCLP: amount(100)}, wantErr: ErrInvalid},
		{name: "negative unit price rejected", filters: Filters{RateUnit: "hora", MinUnitPriceCLP: amount(-1)}, wantErr: ErrInvalid},
		{name: "inverted unit price range rejected", filters: Filters{RateUnit: "dia", MinUnitPriceCLP: amount(200), MaxUnitPriceCLP: amount(100)}, wantErr: ErrInvalid},
		{name: "zero unit price boundary accepted", filters: Filters{RateUnit: "hora", MinUnitPriceCLP: amount(0), MaxUnitPriceCLP: amount(0)}, wantCalls: 1},
		{name: "zero total boundary accepted", filters: Filters{StartAt: &start, EndAt: &end, MinTotalCLP: amount(0), MaxTotalCLP: amount(0)}, wantCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), tc.filters)
			if !errors.Is(err, tc.wantErr) || repo.listCalls != tc.wantCalls {
				t.Fatalf("List() error=%v list_calls=%d, want error=%v calls=%d", err, repo.listCalls, tc.wantErr, tc.wantCalls)
			}
		})
	}
}
