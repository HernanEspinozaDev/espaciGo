package publiccatalogpg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
	"github.com/jackc/pgx/v5"
)

type malformedDetailScanPool struct {
	*detailVerificationPool
	failPrefix string
}

func (p *malformedDetailScanPool) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	trimmed := strings.TrimSpace(query)
	if strings.HasPrefix(trimmed, p.failPrefix) {
		var row []any
		switch p.failPrefix {
		case "SELECT f.codigo,f.etiqueta":
			row = []any{17, "Wi-Fi", true}
		case "SELECT image_id,is_cover,source FROM (":
			row = []any{"22222222-2222-4222-8222-222222222222", "true", "media"}
		}
		return &detailVerificationRows{row: row}, nil
	}
	return p.detailVerificationPool.Query(ctx, query, args...)
}

func TestDetailScanFailuresDoNotLeakPartialPublicDTO(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name       string
		failPrefix string
	}{
		{name: "feature scan", failPrefix: "SELECT f.codigo,f.etiqueta"},
		{name: "image scan", failPrefix: "SELECT image_id,is_cover,source FROM ("},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := &malformedDetailScanPool{
				detailVerificationPool: &detailVerificationPool{detailRow: detailVerificationRow(spaceID)},
				failPrefix:             tc.failPrefix,
			}
			service, err := publiccatalog.New(&Repository{pool: pool}, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces/"+spaceID, nil)
			publiccataloghttp.New(service).ServeHTTP(response, request)

			body := response.Body.String()
			if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), body)
			}
			for _, privateOrPartial := range []string{"data", "Sala sintética", "Wi-Fi", "22222222-2222-4222-8222-222222222222", "column 0"} {
				if strings.Contains(body, privateOrPartial) {
					t.Fatalf("response leaked %q: %s", privateOrPartial, body)
				}
			}
		})
	}
}
