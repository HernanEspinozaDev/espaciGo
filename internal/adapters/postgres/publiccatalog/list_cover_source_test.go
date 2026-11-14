package publiccatalogpg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
	"github.com/jackc/pgx/v5"
)

type coverSourcePool struct {
	query    string
	args     []any
	row      []any
	queryErr error
	rowsErr  error
}

func (p *coverSourcePool) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	p.query = query
	p.args = append([]any(nil), args...)
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	return &coverSourceRows{row: p.row, err: p.rowsErr}, nil
}

func (*coverSourcePool) QueryRow(context.Context, string, ...any) pgx.Row {
	return coverSourceRow{}
}

type coverSourceRow struct{}

func (coverSourceRow) Scan(...any) error { return errors.New("unexpected QueryRow") }

type coverSourceRows struct {
	pgx.Rows
	row  []any
	err  error
	next bool
}

func (r *coverSourceRows) Close()     {}
func (r *coverSourceRows) Err() error { return r.err }
func (r *coverSourceRows) Next() bool {
	if r.next {
		return false
	}
	r.next = true
	return true
}
func (r *coverSourceRows) Scan(dest ...any) error {
	if len(dest) > len(r.row) {
		return fmt.Errorf("scan destinations=%d, fixture columns=%d", len(dest), len(r.row))
	}
	for i, destination := range dest {
		value := r.row[i]
		switch out := destination.(type) {
		case *string:
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("column %d: expected string fixture, got %T", i, value)
			}
			*out = v
		case *int:
			v, ok := value.(int)
			if !ok {
				return fmt.Errorf("column %d: expected int fixture, got %T", i, value)
			}
			*out = v
		case *int64:
			v, ok := value.(int64)
			if !ok {
				return fmt.Errorf("column %d: expected int64 fixture, got %T", i, value)
			}
			*out = v
		case *float64:
			v, ok := value.(float64)
			if !ok {
				return fmt.Errorf("column %d: expected float64 fixture, got %T", i, value)
			}
			*out = v
		case *bool:
			v, ok := value.(bool)
			if !ok {
				return fmt.Errorf("column %d: expected bool fixture, got %T", i, value)
			}
			*out = v
		case *[]byte:
			v, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("column %d: expected []byte fixture, got %T", i, value)
			}
			*out = append([]byte(nil), v...)
		case *sql.NullString:
			v, ok := value.(sql.NullString)
			if !ok {
				return fmt.Errorf("column %d: expected NullString fixture, got %T", i, value)
			}
			*out = v
		case *sql.NullInt64:
			v, ok := value.(sql.NullInt64)
			if !ok {
				return fmt.Errorf("column %d: expected NullInt64 fixture, got %T", i, value)
			}
			*out = v
		default:
			return fmt.Errorf("column %d: unsupported scan destination %T", i, destination)
		}
	}
	return nil
}

type coverSourceCase struct {
	name   string
	source string
}

func TestListCoverImageSourceMatchesPublicContract(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	const imageID = "22222222-2222-4222-8222-222222222222"
	cases := []coverSourceCase{{name: "synthetic gallery", source: "synthetic"}, {name: "media pipeline", source: "media"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := &coverSourcePool{row: []any{
				spaceID, "oficina", "Oficina", "Sala", "Sala sintética", float64(20), "dia", int64(5000), "CLP", 1, "sala",
				sql.NullString{String: "America/Santiago", Valid: true},
				sql.NullString{String: "13", Valid: true}, sql.NullString{String: "Metropolitana", Valid: true},
				sql.NullString{String: "13101", Valid: true}, sql.NullString{String: "Santiago", Valid: true},
				sql.NullString{String: imageID, Valid: true}, sql.NullString{String: tc.source, Valid: true}, sql.NullInt64{}, float64(0), true, false,
			}}
			repo := &Repository{pool: pool}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatalf("construct catalog service: %v", err)
			}
			response := httptest.NewRecorder()
			publiccataloghttp.New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces", nil))
			if response.Code != http.StatusOK {
				t.Fatalf("HTTP status=%d body=%s", response.Code, response.Body.String())
			}
			var wire struct {
				Items []struct {
					Images       []publiccatalog.PublicImageRef    `json:"images"`
					Verification publiccatalog.VerificationSummary `json:"verification"`
				} `json:"items"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
				t.Fatalf("decode public catalog response: %v", err)
			}
			if len(wire.Items) != 1 || len(wire.Items[0].Images) != 1 {
				t.Fatalf("public response items=%+v, want one space with one image", wire.Items)
			}
			if wire.Items[0].Images[0].Source != tc.source {
				t.Errorf("cover source=%q, want %q", wire.Items[0].Images[0].Source, tc.source)
			}
			if wire.Items[0].Images[0].ImageID != imageID || !wire.Items[0].Images[0].IsCover {
				t.Errorf("cover ref=%+v, want image %s marked as cover", wire.Items[0].Images[0], imageID)
			}
			if !wire.Items[0].Verification.IdentityVerified || wire.Items[0].Verification.SpaceVerified {
				t.Errorf("verification=%+v, want current identity verified and space unverified", wire.Items[0].Verification)
			}
			if !strings.Contains(pool.query, "cover.source AS cover_source") {
				t.Errorf("list query does not select the cover source column")
			}
		})
	}
}

func TestPublicCatalogEstimatedTotalFlowsFromRepositoryThroughHTTP(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	cases := []struct {
		name, unit, start, end, zone string
		price, wantTotal             int64
		wantNull                     bool
	}{
		{name: "hourly repeated wall time", unit: "hora", price: 8000, start: "2026-11-01T05:30:00Z", end: "2026-11-01T07:30:00Z", wantTotal: 16000},
		{name: "daily local date boundary", unit: "dia", price: 10000, start: "2024-03-11T00:00:00Z", end: "2024-03-11T05:00:00Z", wantTotal: 20000},
		{name: "monthly leap-day anniversary", unit: "mes", price: 50000, start: "2024-01-31T14:00:00Z", end: "2024-02-29T14:00:00Z", wantTotal: 50000},
		{name: "monthly one nanosecond after anniversary", unit: "mes", price: 50000, start: "2024-01-31T14:00:00Z", end: "2024-02-29T14:00:00.000000001Z", wantTotal: 100000},
		{name: "invalid listing timezone", unit: "dia", price: 10000, start: "2024-03-11T00:00:00Z", end: "2024-03-11T05:00:00Z", zone: "Invalid/Zone", wantNull: true},
		{name: "unsupported synthetic rate unit", unit: "semana", price: 10000, start: "2024-03-11T00:00:00Z", end: "2024-03-11T05:00:00Z", wantNull: true},
		{name: "subtotal exact safe boundary", unit: "hora", price: math.MaxInt64 / 2, start: "2026-11-01T05:30:00Z", end: "2026-11-01T07:30:00Z", wantTotal: math.MaxInt64 - 1},
		{name: "subtotal first unsafe value", unit: "hora", price: math.MaxInt64/2 + 1, start: "2026-11-01T05:30:00Z", end: "2026-11-01T07:30:00Z", wantNull: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			zone := tc.zone
			if zone == "" {
				zone = "America/New_York"
			}
			pool := &coverSourcePool{row: []any{
				spaceID, "oficina", "Oficina", "Sala", "Sala sintética", float64(20), tc.unit, tc.price, "CLP", 1, "sala",
				sql.NullString{String: zone, Valid: true},
				sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{},
				sql.NullString{}, sql.NullString{}, sql.NullInt64{}, float64(0), false, false,
			}}
			repo := &Repository{pool: pool}
			service, err := publiccatalog.New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatalf("construct catalog service: %v", err)
			}
			request := httptest.NewRequest(http.MethodGet,
				"/api/v1/public/spaces?start_at="+tc.start+"&end_at="+tc.end, nil)
			response := httptest.NewRecorder()
			publiccataloghttp.New(service).ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("HTTP status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
			}
			var wire struct {
				Items []struct {
					SpaceID           string `json:"space_id"`
					EstimatedTotalCLP *int64 `json:"estimated_total_clp"`
				} `json:"items"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
				t.Fatalf("decode public catalog response: %v", err)
			}
			if tc.wantNull {
				if len(wire.Items) != 1 || wire.Items[0].SpaceID != spaceID || wire.Items[0].EstimatedTotalCLP != nil {
					t.Fatalf("public item=%+v, want one item with null estimated total", wire.Items)
				}
				return
			}
			gotTotal := int64(-1)
			if len(wire.Items) == 1 && wire.Items[0].EstimatedTotalCLP != nil {
				gotTotal = *wire.Items[0].EstimatedTotalCLP
			}
			if len(wire.Items) != 1 || wire.Items[0].SpaceID != spaceID || wire.Items[0].EstimatedTotalCLP == nil || gotTotal != tc.wantTotal {
				t.Fatalf("public item=%+v, estimated total=%d, want space %s with %d CLP", wire.Items, gotTotal, spaceID, tc.wantTotal)
			}
		})
	}
}
