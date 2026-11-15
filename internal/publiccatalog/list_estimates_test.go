package publiccatalog

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestListEstimatesOnlySafeRentalTotals(t *testing.T) {
	start := time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	rows := []Space{
		{SpaceID: "00000000-0000-4000-8000-000000000001", RateUnit: "hora", BasePriceCLP: 12000, TimeZone: "America/Santiago"},
		{SpaceID: "00000000-0000-4000-8000-000000000002", RateUnit: "unsupported", BasePriceCLP: 12000, TimeZone: "America/Santiago"},
		{SpaceID: "00000000-0000-4000-8000-000000000003", RateUnit: "hora", BasePriceCLP: 12000, TimeZone: "Invalid/Zone"},
		{SpaceID: "00000000-0000-4000-8000-000000000004", RateUnit: "hora", BasePriceCLP: math.MaxInt64, TimeZone: "America/Santiago"},
	}
	service, err := New(memoryRepository{rows: rows}, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}

	page, err := service.List(context.Background(), Filters{StartAt: &start, EndAt: &end})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != len(rows) {
		t.Fatalf("items=%d, want %d", len(page.Items), len(rows))
	}
	if got := page.Items[0].EstimatedTotalCLP; got == nil || *got != 24000 {
		t.Fatalf("valid 90-minute hourly estimate=%v, want 24000", got)
	}
	for _, index := range []int{1, 2, 3} {
		if got := page.Items[index].EstimatedTotalCLP; got != nil {
			t.Fatalf("unsafe estimate for item %d = %d, want nil", index, *got)
		}
	}
	if rows[0].EstimatedTotalCLP != nil {
		t.Fatal("List mutated the repository fixture")
	}
}

func TestListEstimatedTotalsUseCalendarUnitsAndSpaceTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		start     time.Time
		end       time.Time
		unit      string
		price     int64
		wantTotal int64
	}{
		{
			name:      "hourly rate by elapsed time through repeated wall hour",
			start:     time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC),
			end:       time.Date(2026, time.November, 1, 7, 30, 0, 0, time.UTC),
			unit:      "hora",
			price:     8000,
			wantTotal: 16000,
		},
		{
			name:      "daily rate across local midnight after spring DST transition",
			start:     time.Date(2024, time.March, 10, 20, 0, 0, 0, loc),
			end:       time.Date(2024, time.March, 11, 1, 0, 0, 0, loc),
			unit:      "dia",
			price:     10000,
			wantTotal: 20000,
		},
		{
			name:      "monthly anniversary clamped to leap day",
			start:     time.Date(2024, time.January, 31, 9, 0, 0, 0, loc),
			end:       time.Date(2024, time.February, 29, 9, 0, 0, 0, loc),
			unit:      "mes",
			price:     50000,
			wantTotal: 50000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := New(memoryRepository{rows: []Space{{
				SpaceID:  "00000000-0000-4000-8000-000000000001",
				RateUnit: tc.unit, BasePriceCLP: tc.price, TimeZone: loc.String(),
			}}}, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			start, end := tc.start.UTC(), tc.end.UTC()
			page, err := service.List(context.Background(), Filters{StartAt: &start, EndAt: &end})
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			gotTotal := int64(-1)
			if len(page.Items) == 1 && page.Items[0].EstimatedTotalCLP != nil {
				gotTotal = *page.Items[0].EstimatedTotalCLP
			}
			if len(page.Items) != 1 || page.Items[0].EstimatedTotalCLP == nil || gotTotal != tc.wantTotal {
				t.Fatalf("estimated total=%d, want %d", gotTotal, tc.wantTotal)
			}
		})
	}
}
