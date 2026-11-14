package publiccataloghttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

func FuzzPublicListQueryRejectsInvalidSyntaxBeforeRepository(f *testing.F) {
	for _, seed := range []string{
		"%",
		"q=%ZZ",
		"page_size=1&page_size=2",
		"start_at=%",
		"attribute=wifi%3Atrue",
		"attribute=wifi%3Atrue&attribute=wifi%3Afalse",
		"category_code=oficina",
		"unexpected=value",
		"min_total_clp=1&max_total_clp=2",
		"%ff=value",
	} {
		f.Add(seed)
	}
	allowed := map[string]bool{
		"category_code": true, "rate_unit": true, "min_unit_price_clp": true, "max_unit_price_clp": true,
		"sort": true, "page_size": true, "cursor": true, "q": true, "region_code": true,
		"comuna_code": true, "min_area_m2": true, "attribute": true, "start_at": true,
		"end_at": true, "min_total_clp": true, "max_total_clp": true,
	}
	f.Fuzz(func(t *testing.T, rawQuery string) {
		repo := &pageSizeRepository{}
		service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces", nil)
		request.URL.RawQuery = rawQuery
		response := httptest.NewRecorder()
		New(service).ServeHTTP(response, request)

		parsed, parseErr := url.ParseQuery(rawQuery)
		mustReject := parseErr != nil
		if parseErr == nil {
			for key, values := range parsed {
				if !allowed[key] || (key != "attribute" && len(values) != 1) || (key == "attribute" && len(values) > 32) {
					mustReject = true
					break
				}
			}
		}
		if mustReject && (response.Code != http.StatusUnprocessableEntity || repo.listCalls != 0) {
			t.Fatalf("invalid query reached catalog: status=%d list_calls=%d", response.Code, repo.listCalls)
		}
		if response.Code == http.StatusUnprocessableEntity && repo.listCalls != 0 {
			t.Fatalf("invalid filter reached repository list: list_calls=%d", repo.listCalls)
		}
		if response.Code >= http.StatusInternalServerError {
			t.Fatalf("unexpected server error for query input: status=%d", response.Code)
		}
	})
}
