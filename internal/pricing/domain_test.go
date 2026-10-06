package pricing

import (
	"testing"
	"time"
)

func TestUnitsUseConfirmedLocalCalendarAndStartedUnits(t *testing.T) {
	loc, _ := time.LoadLocation("America/Santiago")
	cases := []struct {
		name, unit, start, end string
		want                   int64
	}{
		{"partial hour rounds up", "hora", "2030-01-10T09:00:00-03:00", "2030-01-10T10:30:00-03:00", 2},
		{"exact hour", "hora", "2030-01-10T09:00:00-03:00", "2030-01-10T10:00:00-03:00", 1},
		{"touches two local dates", "dia", "2030-01-10T09:00:00-03:00", "2030-01-11T11:00:00-03:00", 2},
		{"midnight end is exclusive", "dia", "2030-01-10T09:00:00-03:00", "2030-01-11T00:00:00-03:00", 1},
		{"started month", "mes", "2030-01-05T09:00:00-03:00", "2030-02-08T09:00:00-03:00", 2},
		{"anniversary exact", "mes", "2030-01-05T09:00:00-03:00", "2030-02-05T09:00:00-03:00", 1},
		{"clamped anniversary", "mes", "2030-01-31T09:00:00-03:00", "2030-02-28T09:00:00-03:00", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := time.Parse(time.RFC3339, tc.start)
			e, _ := time.Parse(time.RFC3339, tc.end)
			got, err := units(tc.unit, s.UTC(), e.UTC(), loc.String())
			if err != nil || got != tc.want {
				t.Fatalf("units=%d err=%v want=%d", got, err, tc.want)
			}
		})
	}
}

func TestParseWindowRequiresUTCAndPositiveRange(t *testing.T) {
	for _, in := range []SimulationInput{{StartAt: "2030-01-01T00:00:00+00:00", EndAt: "2030-01-01T01:00:00Z"}, {StartAt: "2030-01-01T01:00:00Z", EndAt: "2030-01-01T00:00:00Z"}} {
		if _, _, err := parseWindow(in); err == nil {
			t.Fatalf("accepted invalid window: %+v", in)
		}
	}
}

func TestBilledUnitsDailyBoundariesWithSkippedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	for days, end := range map[int]time.Time{
		1: time.Date(2026, time.September, 7, 0, 0, 0, 0, loc),
		2: time.Date(2026, time.September, 8, 0, 0, 0, 0, loc),
		3: time.Date(2026, time.September, 9, 0, 0, 0, 0, loc),
	} {
		got, e := units("dia", start.UTC(), end.UTC(), loc.String())
		if e != nil || got != int64(days) {
			t.Fatalf("interval for %d local dates billed %d, err=%v", days, got, e)
		}
	}
}

func TestBilledUnitsMonthlyAnniversaryAtSkippedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.August, 6, 0, 0, 0, 0, loc)
	end := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	got, err := units("mes", start.UTC(), end.UTC(), loc.String())
	if err != nil || got != 1 {
		t.Fatalf("monthly interval billed %d months, err=%v", got, err)
	}
}
