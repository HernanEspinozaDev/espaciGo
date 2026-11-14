package rentalflowpg

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

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
	lastSQL  string
	lastArgs []any
	row      mockRow
	queue    []mockRow
	execErr  error
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.lastSQL = sql
	m.lastArgs = args
	if len(m.queue) > 0 {
		r := m.queue[0]
		m.queue = m.queue[1:]
		return &r
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

func TestCreateValidatedQuoteInsertsRow(t *testing.T) {
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
	expiresAt := fixedNow.Add(15 * time.Minute)

	q, err := repo.CreateValidatedQuote(ctx, "renter-1", listing, interval, 1, 200000, expiresAt)
	if err != nil {
		t.Fatalf("expected success, got err=%v", err)
	}
	if q.ID != "quote-fixed-uuid" || q.SpaceID != "space-1" || q.SubtotalCLP != 200000 || q.RenterID != "renter-1" {
		t.Fatalf("unexpected quote struct: %+v", q)
	}

	// Owner cannot rent own space
	_, err = repo.CreateValidatedQuote(ctx, "host-1", listing, interval, 1, 200000, expiresAt)
	if !errors.Is(err, rentalflow.ErrConflict) {
		t.Fatalf("expected ErrConflict for host rental, got %v", err)
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

	// Test 2: Actor participates and Didit verification is approved
	m.queue = []mockRow{
		{vals: []any{1}},
		{vals: []any{sql.NullString{String: "Juan Pérez", Valid: true}, sql.NullString{String: "11.111.111-1", Valid: true}}},
	}
	ident, err := repo.CurrentForRentalContract(ctx, "host-1", "q-1", "renter-1", fixedNow)
	if err != nil {
		t.Fatalf("expected approved Didit identity, got err=%v", err)
	}
	if ident.LegalName != "Juan Pérez" || ident.RUT != "11.111.111-1" || ident.SubjectID != "renter-1" {
		t.Fatalf("unexpected identity result: %+v", ident)
	}

	// Test 3: Didit not present, falls back to manual document case
	m.queue = []mockRow{
		{vals: []any{1}},
		{err: pgx.ErrNoRows}, // Didit attempt missing
		{vals: []any{sql.NullString{String: "María Gonzalez", Valid: true}, sql.NullString{String: "22.222.222-2", Valid: true}}},
	}
	ident, err = repo.CurrentForRentalContract(ctx, "renter-1", "q-1", "host-1", fixedNow)
	if err != nil {
		t.Fatalf("expected fallback manual identity, got err=%v", err)
	}
	if ident.LegalName != "María Gonzalez" || ident.RUT != "22.222.222-2" {
		t.Fatalf("unexpected fallback identity: %+v", ident)
	}

	// Test 4: Neither present returns ErrIdentityUnavailable
	m.queue = []mockRow{
		{vals: []any{1}},
		{err: pgx.ErrNoRows},
		{err: pgx.ErrNoRows},
	}
	_, err = repo.CurrentForRentalContract(ctx, "renter-1", "q-1", "host-1", fixedNow)
	if !errors.Is(err, rentalflow.ErrIdentityUnavailable) {
		t.Fatalf("expected ErrIdentityUnavailable, got %v", err)
	}
}
