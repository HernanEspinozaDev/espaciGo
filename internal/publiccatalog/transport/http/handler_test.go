package publiccataloghttp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

type pageSizeRepository struct{ pageSize int }

func (r *pageSizeRepository) Categories(context.Context) ([]publiccatalog.Category, error) {
	return nil, nil
}
func (r *pageSizeRepository) HasActiveCategory(context.Context, string) (bool, error) {
	return true, nil
}
func (r *pageSizeRepository) AttributeFilters(context.Context, string) ([]publiccatalog.AttributeFilter, error) {
	return []publiccatalog.AttributeFilter{{Code: "wifi", Type: "boolean", ProfileVersion: 1}}, nil
}
func (r *pageSizeRepository) HasRegion(context.Context, string) (bool, error) { return true, nil }
func (r *pageSizeRepository) HasCommune(context.Context, string, string) (bool, error) {
	return true, nil
}
func (r *pageSizeRepository) Locations(context.Context, string, string, int) ([]publiccatalog.Location, error) {
	return nil, nil
}
func (r *pageSizeRepository) List(_ context.Context, filters publiccatalog.Filters, _ *publiccatalog.Cursor, _ int) ([]publiccatalog.Space, error) {
	r.pageSize = filters.PageSize
	return []publiccatalog.Space{}, nil
}
func (r *pageSizeRepository) Detail(context.Context, string) (publiccatalog.Space, error) {
	return publiccatalog.Space{}, publiccatalog.ErrNotFound
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
		{"two-sided range", []string{"altura:gte:1.25", "altura:lte:2.50"}, true},
		{"enum list OR", []string{"equipo:in:wifi", "equipo:in:proyector"}, true},
		{"enum list CSV", []string{"equipo:in:wifi,proyector"}, true},
		{"enum list mixed CSV and repeated", []string{"equipo:in:wifi,proyector", "equipo:in:calefaccion"}, true},
		{"duplicate bound", []string{"altura:gte:1", "altura:gte:2"}, false},
		{"mixed bool and range", []string{"wifi:false", "wifi:gte:1"}, false},
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
			if tc.method == http.MethodOptions && tc.wantStatus == http.StatusNoContent && response.Header().Get("Access-Control-Allow-Methods") != "GET, OPTIONS" {
				t.Fatalf("preflight methods=%q", response.Header().Get("Access-Control-Allow-Methods"))
			}
		})
	}
}

type revocableMediaRepository struct {
	pageSizeRepository
	item    publiccatalog.PublicImage
	visible bool
}

func (r *revocableMediaRepository) PublicImage(context.Context, string, string) (publiccatalog.PublicImage, error) {
	return publiccatalog.PublicImage{}, publiccatalog.ErrNotFound
}

func (r *revocableMediaRepository) PublicMediaVariant(context.Context, string, string, string) (publiccatalog.PublicImage, error) {
	if !r.visible {
		return publiccatalog.PublicImage{}, publiccatalog.ErrNotFound
	}
	return r.item, nil
}

type publicMediaObjectStore struct {
	body       []byte
	key        string
	generation int64
}

func (s *publicMediaObjectStore) Put(context.Context, string, []byte, string) (media.StoredObject, error) {
	return media.StoredObject{}, nil
}

func (s *publicMediaObjectStore) Get(_ context.Context, key string, generation int64) ([]byte, error) {
	if key != s.key || generation != s.generation {
		return nil, media.ErrNotFound
	}
	return append([]byte(nil), s.body...), nil
}

func (s *publicMediaObjectStore) Generation(context.Context, string) (int64, error) {
	return s.generation, nil
}

func (s *publicMediaObjectStore) Delete(context.Context, string, int64) error { return nil }

type publicImageFileStore struct{}

func (publicImageFileStore) Get(context.Context, string) ([]byte, error) {
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
	}
	store := &publicMediaObjectStore{body: body, key: key, generation: 7}
	service, err := publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), publicImageFileStore{}, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service)
	path := "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display"

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "image/webp" || response.Body.String() != string(body) {
		t.Fatalf("first public image response status=%d cache=%q mime=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Header().Get("Content-Type"), response.Body.String())
	}

	repo.visible = false
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("revoked public image response status=%d cache=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
}
