package rentaldemo

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCivilCalendarAcrossMonthLeapAndDST(t *testing.T) {
	cases := []struct {
		name, now, space, month, date, start, end string
		duration                                  int
	}{
		{"daily month boundary", "2030-01-15T12:00:00Z", SpaceIDDay, "2030-01", "2030-01-31", "2030-01-31T03:00:00Z", "2030-02-01T03:00:00Z", 1},
		{"leap day", "2028-02-01T12:00:00Z", SpaceIDDay, "2028-02", "2028-02-29", "2028-02-29T03:00:00Z", "2028-03-01T03:00:00Z", 1},
		{"Santiago 23h", "2026-08-20T12:00:00Z", SpaceIDDay, "2026-09", "2026-09-06", "2026-09-06T04:00:00Z", "2026-09-07T03:00:00Z", 1},
		{"Santiago 25h", "2026-03-20T12:00:00Z", SpaceIDDay, "2026-04", "2026-04-04", "2026-04-04T03:00:00Z", "2026-04-05T04:00:00Z", 1},
		{"monthly clamped", "2030-01-15T12:00:00Z", SpaceIDMonth, "2030-01", "2030-01-31", "2030-01-31T03:00:00Z", "2030-02-28T03:00:00Z", 1},
		{"monthly skipped midnight", "2026-08-20T12:00:00Z", SpaceIDMonth, "2026-09", "2026-09-06", "2026-09-06T04:00:00Z", "2026-10-06T03:00:00Z", 1},
		{"daily 2-day across month boundary", "2030-01-15T12:00:00Z", SpaceIDDay, "2030-01", "2030-01-30", "2030-01-30T03:00:00Z", "2030-02-01T03:00:00Z", 2},
		{"daily 3-day across month boundary", "2030-01-15T12:00:00Z", SpaceIDDay, "2030-01", "2030-01-30", "2030-01-30T03:00:00Z", "2030-02-02T03:00:00Z", 3},
		{"monthly leap anniversary", "2028-02-01T12:00:00Z", SpaceIDMonth, "2028-02", "2028-02-29", "2028-02-29T03:00:00Z", "2028-03-29T03:00:00Z", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tc.now)
			s, _ := NewStore(func() time.Time { return now })
			m, err := s.Month(context.Background(), "", tc.space, tc.month, tc.duration, false)
			if err != nil {
				t.Fatal(err)
			}
			var slots []Interval
			for _, d := range m.Days {
				if d.Date == tc.date {
					slots = d.Intervals
				}
			}
			if len(slots) != 1 || slots[0].StartAt != tc.start || slots[0].EndAt != tc.end {
				t.Fatalf("civil slots=%+v want %s..%s", slots, tc.start, tc.end)
			}
			q, err := s.CreateQuote(context.Background(), ActorRenter, QuoteRequest{tc.space, tc.start, tc.end})
			if err != nil || q.Units != tc.duration {
				t.Fatalf("quote mismatch %+v %v", q, err)
			}
		})
	}
}
func TestHourlyPartialStatusHorizonAndRepeatedWallTimes(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	m, _ := s.Month(context.Background(), "", SpaceIDHour, "2030-01", 1, false)
	if m.Days[15].Status != "partial" {
		t.Fatalf("partial working day called %s", m.Days[15].Status)
	}
	// Synthetic internal fixture zone, not a mutation of any real listing.
	s.spaces[SpaceIDHour].TimeZone = "America/New_York"
	s.spaces[SpaceIDHour].OpenHour = 0
	s.spaces[SpaceIDHour].CloseHour = 24
	now = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	fold, _ := s.Month(context.Background(), "", SpaceIDHour, "2026-11", 1, false)
	found := map[string]bool{}
	for _, i := range fold.Days[0].Intervals {
		found[i.StartAt] = true
	}
	if !found["2026-11-01T05:30:00Z"] || !found["2026-11-01T06:30:00Z"] {
		t.Fatal("lost repeated 01:30 wall time")
	}
}
func TestQuoteExpiringAfterRentalStartCannotBeConfirmedInPast(t *testing.T) {
	now := time.Date(2030, 1, 15, 11, 55, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	q, err := s.CreateQuote(context.Background(), ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-15T12:00:00Z", "2030-01-15T13:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Minute)
	if _, err := s.Confirm(context.Background(), ActorRenter, q.ID, true, "00000000-0000-4000-8000-000000000013"); !errors.Is(err, ErrConflict) {
		t.Fatalf("past rental accepted: %v", err)
	}
}

func Test90DayHorizonStrictBoundary(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Day 90: 2030-01-15 + 90 days = 2030-04-15
	day90Start := time.Date(2030, 4, 15, 14, 0, 0, 0, time.UTC)
	day90End := day90Start.Add(time.Hour)
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{
		SpaceID: SpaceIDHour,
		StartAt: day90Start.Format(time.RFC3339),
		EndAt:   day90End.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("quote on exact 90-day horizon rejected: %v", err)
	}

	// Day 91: 2030-04-16 (> 90 days)
	day91Start := time.Date(2030, 4, 16, 14, 0, 0, 0, time.UTC)
	day91End := day91Start.Add(time.Hour)
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{
		SpaceID: SpaceIDHour,
		StartAt: day91Start.Format(time.RFC3339),
		EndAt:   day91End.Format(time.RFC3339),
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("quote beyond 90-day horizon got %v, want ErrInvalid", err)
	}

	// Month readback for April 2030
	m, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2030-04", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	// Day 15 (delta = 90) must be available
	day15 := m.Days[14]
	if day15.Status == "unavailable" || len(day15.Intervals) == 0 {
		t.Fatalf("day 90 unexpectedly unavailable: status=%s intervals=%d", day15.Status, len(day15.Intervals))
	}
	// Day 16 (delta = 91) must be outside_horizon
	day16 := m.Days[15]
	if day16.Status != "outside_horizon" || len(day16.Intervals) != 0 {
		t.Fatalf("day 91 expected outside_horizon: status=%s intervals=%d", day16.Status, len(day16.Intervals))
	}
}
