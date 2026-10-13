package publiccataloghttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
		{name: "twenty five is accepted", query: "?page_size=25", status: http.StatusOK, pageSize: 25},
		{name: "twenty six is rejected", query: "?page_size=26", status: http.StatusUnprocessableEntity},
		{name: "duplicate is rejected", query: "?page_size=1&page_size=25", status: http.StatusUnprocessableEntity},
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
