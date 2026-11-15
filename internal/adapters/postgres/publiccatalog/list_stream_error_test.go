package publiccatalogpg

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
)

func TestPublicListQueryFailureDoesNotLeakPartialItems(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	const imageID = "22222222-2222-4222-8222-222222222222"
	pool := &coverSourcePool{
		row: []any{
			spaceID, "oficina", "Oficina", "Sala sintética", "Descripción sintética", float64(20), "dia", int64(5000), "CLP", 1, "sala",
			sql.NullString{String: "America/Santiago", Valid: true},
			sql.NullString{String: "13", Valid: true}, sql.NullString{String: "Metropolitana", Valid: true},
			sql.NullString{String: "13101", Valid: true}, sql.NullString{String: "Santiago", Valid: true},
			sql.NullString{String: imageID, Valid: true}, sql.NullInt64{}, float64(0), true, false,
			sql.NullString{String: "media", Valid: true},
		},
		rowsErr: errors.New("synthetic private row stream failure"),
	}
	service, err := publiccatalog.New(&Repository{pool: pool}, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	publiccataloghttp.New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces", nil))
	body := response.Body.String()
	if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), body)
	}
	for _, private := range []string{`"data"`, `"items"`, "Sala sintética", "synthetic private row stream failure"} {
		if strings.Contains(body, private) {
			t.Errorf("error response leaked %q: %s", private, body)
		}
	}
}
