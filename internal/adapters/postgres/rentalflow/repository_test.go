package rentalflowpg

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/calendar"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mockRow struct {
	vals []any
	err  error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		if i < len(m.vals) {
			switch target := d.(type) {
			case *string:
				*target = m.vals[i].(string)
			case *int64:
				*target = m.vals[i].(int64)
			case *int:
				*target = m.vals[i].(int)
			case *bool:
				*target = m.vals[i].(bool)
			case *[]byte:
				*target = m.vals[i].([]byte)
			case *[]time.Time:
				*target = m.vals[i].([]time.Time)
			case *time.Time:
				*target = m.vals[i].(time.Time)
			case *sql.NullString:
				*target = m.vals[i].(sql.NullString)
			}
		}
	}
	return nil
}

type mockDB struct {
	calls          int
	lastSQL        string
	lastArgs       []any
	row            mockRow
	queue          []mockRow
	execErr        error
	echoCandidates bool
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.calls++
	m.lastSQL = sql
	m.lastArgs = args
	if len(m.queue) > 0 {
		r := m.queue[0]
		m.queue = m.queue[1:]
		return &r
	}
	if m.echoCandidates && strings.Contains(sql, "WITH candidates AS") {
		return &mockRow{vals: []any{args[1], args[2]}}
	}
	return &m.row
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.lastSQL = sql
	m.lastArgs = args
	return nil, nil
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.lastSQL = sql
	m.lastArgs = args
	return pgconn.NewCommandTag("INSERT 0 1"), m.execErr
}

func TestNewRepositoryValidatesInputs(t *testing.T) {
	now := func() time.Time { return time.Now() }
	m := &mockDB{}
	repo, err := NewRepository(m, nil, now)
	if err != nil || repo == nil {
		t.Fatalf("expected valid repo, got err=%v", err)
	}

	_, err = NewRepository(nil, nil, now)
	if !errors.Is(err, rentalflow.ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil db, got %v", err)
	}

	_, err = NewRepository(m, nil, nil)
	if !errors.Is(err, rentalflow.ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil now, got %v", err)
	}
}

func TestPublicListingSuccessAndNotFound(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }
	m := &mockDB{
		row: mockRow{
			vals: []any{
				"space-1", "owner-1", "activa", "Bodega Central",
				"mes", int64(150000), "CLP", int64(1), "America/Santiago",
			},
		},
	}
	repo, _ := NewRepository(m, nil, now)
	ctx := context.Background()

	l, err := repo.PublicListing(ctx, "space-1")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if l.ID != "space-1" || l.Title != "Bodega Central" || l.UnitPriceCLP != 150000 || l.RateUnit != "mes" {
		t.Fatalf("unexpected listing scanned: %+v", l)
	}

	m.row = mockRow{err: pgx.ErrNoRows}
	_, err = repo.PublicListing(ctx, "space-2")
	if !errors.Is(err, rentalflow.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntervalAvailableQueriesOccupancy(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }
	m := &mockDB{
		row: mockRow{vals: []any{true}},
	}
	repo, _ := NewRepository(m, nil, now)
	ctx := context.Background()

	start := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	avail, err := repo.IntervalAvailable(ctx, rentalflow.Listing{ID: "space-1"}, rentalflow.Interval{StartAt: start, EndAt: end})
	if err != nil || !avail {
		t.Fatalf("expected available=true, got avail=%v err=%v", avail, err)
	}

	m.row = mockRow{vals: []any{false}}
	avail, err = repo.IntervalAvailable(ctx, rentalflow.Listing{ID: "space-1"}, rentalflow.Interval{StartAt: start, EndAt: end})
	if err != nil || avail {
		t.Fatalf("expected available=false, got avail=%v err=%v", avail, err)
	}
}

func TestPublicIntervalsReturnsScheduledUnoccupiedHourlyCandidates(t *testing.T) {
	fixedNow := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	schedule := []byte(`{"enabled":true,"days":[{"weekday":1,"periods":[]},{"weekday":2,"periods":[]},{"weekday":3,"periods":[]},{"weekday":4,"periods":[]},{"weekday":5,"periods":[]},{"weekday":6,"periods":[{"open":"13:00","close":"14:00"},{"open":"15:00","close":"16:00"}]},{"weekday":7,"periods":[]}]}`)
	m := &mockDB{queue: []mockRow{
		{vals: []any{schedule}},
		{vals: []any{[]time.Time{time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)}, []time.Time{time.Date(2026, 11, 14, 16, 0, 0, 0, time.UTC)}}},
	}}
	repo, _ := NewRepository(m, nil, func() time.Time { return fixedNow })
	got, err := repo.PublicIntervals(context.Background(), rentalflow.Listing{ID: "space-1", RateUnit: "hora", TimeZone: "UTC"}, "2026-11-14", 1)
	if err != nil {
		t.Fatalf("PublicIntervals: %v", err)
	}
	if len(got) != 1 || got[0].StartAt.Hour() != 15 || got[0].StartAt.Minute() != 0 {
		t.Fatalf("scheduled unoccupied intervals = %#v, want only 15:00-16:00 candidate", got)
	}
}

func TestPublicIntervalsBoundsDatabaseRoundTripsPerRequest(t *testing.T) {
	m := &mockDB{queue: []mockRow{{vals: []any{[]byte(`{"enabled":false,"days":[]}`)}}}}
	starts := make([]time.Time, 48)
	ends := make([]time.Time, 48)
	for index := range starts {
		starts[index] = time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC).Add(time.Duration(index) * 30 * time.Minute)
		ends[index] = starts[index].Add(time.Hour)
	}
	m.queue = append(m.queue, mockRow{vals: []any{starts, ends}})
	repo, err := NewRepository(m, nil, func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.PublicIntervals(context.Background(), rentalflow.Listing{
		ID: "space-1", RateUnit: "hora", TimeZone: "UTC",
	}, "2026-11-15", 1)
	if err != nil {
		t.Fatalf("PublicIntervals: %v", err)
	}
	if len(got) != 48 {
		t.Fatalf("available hourly intervals=%d, want 48", len(got))
	}
	if m.calls > 2 {
		t.Fatalf("database round trips=%d, want at most 2 per request", m.calls)
	}
}

func TestPublicIntervalsRejectsOutOfContractDurationsBeforeDatabase(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name     string
		unit     string
		duration int
	}{
		{name: "hour overflow", unit: "hora", duration: maxInt},
		{name: "day overflow", unit: "dia", duration: maxInt},
		{name: "unsupported multi-month duration", unit: "mes", duration: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockDB{
				queue:          []mockRow{{vals: []any{[]byte(`{"enabled":false,"days":[]}`)}}},
				echoCandidates: true,
			}
			repo, err := NewRepository(mock, nil, func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) })
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.PublicIntervals(context.Background(), rentalflow.Listing{
				ID: "space-1", RateUnit: tc.unit, TimeZone: "UTC",
			}, "2026-11-15", tc.duration)
			if !errors.Is(err, rentalflow.ErrInvalid) {
				t.Fatalf("PublicIntervals error=%v, want ErrInvalid", err)
			}
			if mock.calls != 0 {
				t.Fatalf("database calls=%d for rejected duration, want 0", mock.calls)
			}
		})
	}
}

func TestPublicIntervalsRetainsBothSlotsInDSTRepeatedHour(t *testing.T) {
	schedule := []byte(`{"enabled":true,"days":[{"weekday":1,"periods":[]},{"weekday":2,"periods":[]},{"weekday":3,"periods":[]},{"weekday":4,"periods":[]},{"weekday":5,"periods":[]},{"weekday":6,"periods":[{"open":"23:00","close":"24:00"}]},{"weekday":7,"periods":[]}]}`)
	m := &mockDB{
		queue:          []mockRow{{vals: []any{schedule}}},
		echoCandidates: true,
	}
	repo, err := NewRepository(m, nil, func() time.Time { return time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.PublicIntervals(context.Background(), rentalflow.Listing{
		ID: "space-1", RateUnit: "hora", TimeZone: "America/Santiago",
	}, "2026-04-04", 1)
	if err != nil {
		t.Fatalf("PublicIntervals: %v", err)
	}
	wantStarts := []time.Time{
		time.Date(2026, 4, 5, 2, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 5, 2, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 5, 3, 0, 0, 0, time.UTC),
	}
	if len(got) != len(wantStarts) {
		t.Fatalf("repeated-hour candidates=%d, want %d: %#v", len(got), len(wantStarts), got)
	}
	for i, wantStart := range wantStarts {
		if !got[i].StartAt.Equal(wantStart) || got[i].EndAt.Sub(got[i].StartAt) != time.Hour {
			t.Fatalf("repeated-hour candidate[%d]=[%s,%s), want start %s and one elapsed hour", i, got[i].StartAt, got[i].EndAt, wantStart)
		}
	}
}

func TestPublicIntervalsUsesCivilDayAndClampedMonthBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		unit         string
		dateText     string
		duration     int
		wantEndLocal string
	}{
		{name: "civil day across Santiago DST", unit: "dia", dateText: "2026-09-06", duration: 1, wantEndLocal: "2026-09-07"},
		{name: "month end clamps", unit: "mes", dateText: "2027-01-31", duration: 1, wantEndLocal: "2027-02-28"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.dateText)
			if err != nil {
				t.Fatal(err)
			}
			location, err := time.LoadLocation("America/Santiago")
			if err != nil {
				t.Fatal(err)
			}
			wantStart, ok := calendar.FirstInstantOfDate(date.Year(), date.Month(), date.Day(), location)
			if !ok {
				t.Fatal("first instant of requested date is unavailable")
			}
			var endDate time.Time
			if tc.unit == "dia" {
				endDate = date.AddDate(0, 0, tc.duration)
			} else {
				endDate, ok = calendar.AddMonthsClamped(date, int64(tc.duration))
				if !ok {
					t.Fatal("clamped month anniversary is unavailable")
				}
			}
			wantEnd, ok := calendar.FirstInstantOfDate(endDate.Year(), endDate.Month(), endDate.Day(), location)
			if !ok {
				t.Fatal("first instant of end date is unavailable")
			}
			mock := &mockDB{
				queue:          []mockRow{{vals: []any{[]byte(`{"enabled":false,"days":[]}`)}}},
				echoCandidates: true,
			}
			repo, err := NewRepository(mock, nil, func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) })
			if err != nil {
				t.Fatal(err)
			}
			got, err := repo.PublicIntervals(context.Background(), rentalflow.Listing{
				ID: "space-1", RateUnit: tc.unit, TimeZone: "America/Santiago",
			}, tc.dateText, tc.duration)
			if err != nil {
				t.Fatalf("PublicIntervals: %v", err)
			}
			if len(got) != 1 || !got[0].StartAt.Equal(wantStart) || !got[0].EndAt.Equal(wantEnd) {
				t.Fatalf("calendar candidate = %#v, want [%s,%s)", got, wantStart, wantEnd)
			}
			if localEnd := got[0].EndAt.In(location).Format("2006-01-02"); localEnd != tc.wantEndLocal {
				t.Fatalf("local end date=%s, want %s", localEnd, tc.wantEndLocal)
			}
			if tc.unit == "dia" && got[0].EndAt.Sub(got[0].StartAt) != 23*time.Hour {
				t.Fatalf("DST civil-day elapsed duration=%s, want 23h", got[0].EndAt.Sub(got[0].StartAt))
			}
		})
	}
}

func TestCreateValidatedQuoteRequiresTransactionalRevalidation(t *testing.T) {
	fixedNow := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	now := func() time.Time { return fixedNow }
	m := &mockDB{}
	repo, _ := NewRepository(m, nil, now)
	repo.newID = func() string { return "quote-fixed-uuid" }
	ctx := context.Background()

	listing := rentalflow.Listing{
		ID:           "space-1",
		OwnerID:      "host-1",
		Title:        "Oficina Privada",
		RateUnit:     "mes",
		UnitPriceCLP: 200000,
		Currency:     "CLP",
		RateVersion:  2,
		TimeZone:     "America/Santiago",
	}
	interval := rentalflow.Interval{
		StartAt: fixedNow.Add(24 * time.Hour),
		EndAt:   fixedNow.Add(31 * 24 * time.Hour),
	}
	createdAt := fixedNow

	_, err := repo.CreateValidatedQuote(ctx, "renter-1", listing, interval, 1, 200000, createdAt)
	if !errors.Is(err, rentalflow.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable without a transaction provider, got %v", err)
	}
	if m.calls != 0 {
		t.Fatalf("quote write ran without transactional revalidation: db calls=%d", m.calls)
	}

	// Owner cannot rent own space
	_, err = repo.CreateValidatedQuote(ctx, "host-1", listing, interval, 1, 200000, createdAt)
	if !errors.Is(err, rentalflow.ErrConflict) {
		t.Fatalf("expected ErrConflict for host rental, got %v", err)
	}
}

func TestPurgeExpiredCommercialQuotesIsBounded(t *testing.T) {
	at := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("test", -3*60*60))
	m := &mockDB{}
	repo, err := NewRepository(m, nil, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, limit := range []int{0, 501} {
		if _, err := repo.PurgeExpiredCommercialQuotes(ctx, at, limit); !errors.Is(err, rentalflow.ErrInvalid) {
			t.Fatalf("limit %d: expected ErrInvalid, got %v", limit, err)
		}
	}
	if m.lastSQL != "" {
		t.Fatalf("invalid purge limit executed SQL: %s", m.lastSQL)
	}
	purged, err := repo.PurgeExpiredCommercialQuotes(ctx, at, 100)
	if err != nil || purged != 1 {
		t.Fatalf("bounded purge returned purged=%d err=%v", purged, err)
	}
	if !strings.Contains(m.lastSQL, "ORDER BY vence_en,id") || !strings.Contains(m.lastSQL, "LIMIT $2") || !strings.Contains(m.lastSQL, "DELETE FROM public.cotizacion_arriendo_comercial") || len(m.lastArgs) != 2 {
		t.Fatalf("purge query is not deterministically bounded: SQL=%s args=%v", m.lastSQL, m.lastArgs)
	}
	storedAt, ok := m.lastArgs[0].(time.Time)
	if !ok || !storedAt.Equal(at.UTC()) || m.lastArgs[1] != 100 {
		t.Fatalf("purge args=%v, want UTC timestamp and limit 100", m.lastArgs)
	}
}

func TestGetForParticipantScansQuote(t *testing.T) {
	fixedNow := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	now := func() time.Time { return fixedNow }
	m := &mockDB{
		row: mockRow{
			vals: []any{
				"q-1", "space-1", "host-1", "renter-1", "Oficina Privada",
				int64(2), "mes", int64(200000), "CLP", int64(1), int64(200000),
				fixedNow.Add(24 * time.Hour), fixedNow.Add(31 * 24 * time.Hour),
				"America/Santiago", fixedNow, fixedNow.Add(15 * time.Minute),
			},
		},
	}
	repo, _ := NewRepository(m, nil, now)
	ctx := context.Background()

	q, err := repo.GetForParticipant(ctx, "renter-1", "q-1")
	if err != nil {
		t.Fatalf("expected quote, got err=%v", err)
	}
	if q.ID != "q-1" || q.OwnerID != "host-1" || q.SpaceTitle != "Oficina Privada" {
		t.Fatalf("unexpected quote: %+v", q)
	}
	if !strings.Contains(m.lastSQL, "public.cotizacion_arriendo_comercial") || !strings.Contains(m.lastSQL, "q.titulo_espacio") || strings.Contains(m.lastSQL, "JOIN public.espacio") {
		t.Fatalf("quote read does not use the persisted title snapshot: %s", m.lastSQL)
	}

	m.row = mockRow{err: pgx.ErrNoRows}
	_, err = repo.GetForParticipant(ctx, "unauthorized-actor", "q-1")
	if !errors.Is(err, rentalflow.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCurrentForRentalContractDiditAndFallback(t *testing.T) {
	fixedNow := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	now := func() time.Time { return fixedNow }

	// Test 1: Actor not in quote returns ErrNotFound
	m := &mockDB{
		row: mockRow{err: pgx.ErrNoRows},
	}
	repo, _ := NewRepository(m, nil, now)
	ctx := context.Background()

	_, err := repo.CurrentForRentalContract(ctx, "stranger", "q-1", "host-1", fixedNow)
	if !errors.Is(err, rentalflow.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for stranger, got %v", err)
	}
	if !strings.Contains(m.lastSQL, "public.cotizacion_arriendo_comercial") {
		t.Fatalf("contract identity scope does not read the commercial quote: %s", m.lastSQL)
	}

	// Legacy provider/profile identity must never be disclosed for contracts.
	m.queue = []mockRow{
		{vals: []any{1}},
		{vals: []any{sql.NullString{String: "Legacy Name", Valid: true}, sql.NullString{String: "11.111.111-1", Valid: true}}},
	}
	_, err = repo.CurrentForRentalContract(ctx, "host-1", "q-1", "renter-1", fixedNow)
	if !errors.Is(err, rentalflow.ErrIdentityUnavailable) {
		t.Fatalf("legacy Didit identity disclosed: err=%v", err)
	}

	// Existing provider and manual-case fixture data is never exposed until
	// the purpose-limited encrypted declaration reader is safely integrated.
	m.queue = []mockRow{
		{vals: []any{1}},
		{vals: []any{sql.NullString{String: "Juan Pérez", Valid: true}, sql.NullString{String: "11.111.111-1", Valid: true}}},
	}
	ident, err := repo.CurrentForRentalContract(ctx, "host-1", "q-1", "renter-1", fixedNow)
	if !errors.Is(err, rentalflow.ErrIdentityUnavailable) || ident != (rentalflow.ContractIdentity{}) {
		t.Fatalf("provider identity was not withheld: identity=%+v err=%v", ident, err)
	}
	if len(m.queue) != 1 {
		t.Fatalf("identity query ran after participant scope: remaining queued rows=%d", len(m.queue))
	}
}
