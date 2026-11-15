package publiccatalogpg

import (
	"context"
	"reflect"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/jackc/pgx/v5"
)

type countedDetailPool struct {
	*detailVerificationPool
	queryCalls int
}

func (p *countedDetailPool) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	p.queryCalls++
	return p.detailVerificationPool.Query(ctx, query, args...)
}

func TestDetailDecodesSyntheticWeeklyScheduleProjection(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	row := detailVerificationRow(spaceID)
	row[len(row)-1] = []byte(`{"enabled":true,"time_zone":"America/Santiago","days":[{"weekday":1,"periods":[{"open":"09:00","close":"17:00"}]}]}`)
	pool := &countedDetailPool{detailVerificationPool: &detailVerificationPool{detailRow: row}}
	repo := &Repository{pool: pool}

	space, err := repo.Detail(context.Background(), spaceID)
	if err != nil {
		t.Fatalf("Detail(): %v", err)
	}
	want := publiccatalog.PublicWeeklySchedule{
		Enabled:  true,
		TimeZone: "America/Santiago",
		Days:     []publiccatalog.PublicWeeklyDay{{Weekday: 1, Periods: []publiccatalog.PublicWeeklyPeriod{{Open: "09:00", Close: "17:00"}}}},
	}
	if space.WeeklySchedule == nil || !reflect.DeepEqual(*space.WeeklySchedule, want) {
		t.Fatalf("weekly schedule=%+v, want %+v", space.WeeklySchedule, want)
	}
	if pool.queryCalls != 2 {
		t.Fatalf("detail dependent queries=%d, want features and images", pool.queryCalls)
	}
}

func TestDetailRejectsMalformedWeeklyScheduleBeforeDependentQueries(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	row := detailVerificationRow(spaceID)
	row[len(row)-1] = []byte(`{"enabled":true,`)
	pool := &countedDetailPool{detailVerificationPool: &detailVerificationPool{detailRow: row}}
	repo := &Repository{pool: pool}

	if _, err := repo.Detail(context.Background(), spaceID); err == nil {
		t.Fatal("Detail() accepted malformed weekly schedule JSON")
	}
	if pool.queryCalls != 0 {
		t.Fatalf("malformed detail triggered %d dependent queries", pool.queryCalls)
	}
}
