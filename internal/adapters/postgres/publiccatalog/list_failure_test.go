package publiccatalogpg

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
)

func TestPublicListQueryAndScanFailuresReturnNoPartialItems(t *testing.T) {
	queryFailure := errors.New("synthetic private query failure")
	cases := []struct {
		name string
		pool *coverSourcePool
	}{
		{name: "query failure", pool: &coverSourcePool{queryErr: queryFailure}},
		{name: "scan failure", pool: &coverSourcePool{row: []any{17}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := publiccatalog.New(&Repository{pool: tc.pool}, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces", nil)
			publiccataloghttp.New(service).ServeHTTP(response, request)

			body := response.Body.String()
			if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), body)
			}
			for _, forbidden := range []string{`"data"`, `"items"`, "Sala sintética", "synthetic private query failure", "fixture columns"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("error response leaked %q: %s", forbidden, body)
				}
			}
		})
	}
}

func TestListRejectsUnknownTypedAttributeBeforeQuery(t *testing.T) {
	pool := &coverSourcePool{}
	repo := &Repository{pool: pool}
	filters := publiccatalog.Filters{
		TypedAttributes: map[string]publiccatalog.AttributePredicate{
			"custom": {Type: "untrusted-sql-type"},
		},
	}
	if _, err := repo.List(context.Background(), filters, nil, 10); !errors.Is(err, publiccatalog.ErrInvalid) {
		t.Fatalf("List() error=%v, want ErrInvalid", err)
	}
	if pool.query != "" {
		t.Fatalf("invalid predicate reached query adapter: %q", pool.query)
	}
}
