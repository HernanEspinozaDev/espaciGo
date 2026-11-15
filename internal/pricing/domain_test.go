package pricing

import (
	"errors"
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
			got, err := BilledUnits(tc.unit, s.UTC(), e.UTC(), loc.String())
			if err != nil || got != tc.want {
				t.Fatalf("units=%d err=%v want=%d", got, err, tc.want)
			}
		})
	}
}

func TestDailyUnitsUseSpaceLocalDateWhenUTCDateIsUnchanged(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2030, time.January, 10, 23, 59, 0, 0, loc)
	end := time.Date(2030, time.January, 11, 0, 1, 0, 0, loc)
	startUTC, endUTC := start.UTC(), end.UTC()
	if startUTC.Year() != endUTC.Year() || startUTC.YearDay() != endUTC.YearDay() {
		t.Fatalf("fixture must stay within one UTC date: start=%s end=%s", startUTC, endUTC)
	}
	if start.In(loc).YearDay() == end.In(loc).YearDay() {
		t.Fatalf("fixture must cross a space-local date: start=%s end=%s", start.In(loc), end.In(loc))
	}
	got, err := BilledUnits("dia", startUTC, endUTC, loc.String())
	if err != nil || got != 2 {
		t.Fatalf("local-midnight interval billed %d units, err=%v; want 2 despite remaining within one UTC date", got, err)
	}
}

func TestParseWindowRequiresUTCAndPositiveRange(t *testing.T) {
	for _, in := range []SimulationInput{
		{StartAt: "2030-01-01T00:00:00+00:00", EndAt: "2030-01-01T01:00:00Z"},
		{StartAt: "malformedZ", EndAt: "2030-01-01T01:00:00Z"},
		{StartAt: "2030-01-01T00:00:00Z", EndAt: "malformedZ"},
		{StartAt: "2030-01-01T01:00:00Z", EndAt: "2030-01-01T00:00:00Z"},
	} {
		if _, _, err := parseWindow(in); err == nil {
			t.Fatalf("accepted invalid window: %+v", in)
		}
	}
}

func TestParseWindowPreservesNanosecondsAsUTC(t *testing.T) {
	startText := "2030-01-01T00:00:00.123456789Z"
	endText := "2030-01-01T01:00:00.987654321Z"
	start, end, err := parseWindow(SimulationInput{StartAt: startText, EndAt: endText})
	if err != nil {
		t.Fatalf("parse valid UTC window: %v", err)
	}
	wantStart := time.Date(2030, time.January, 1, 0, 0, 0, 123456789, time.UTC)
	wantEnd := time.Date(2030, time.January, 1, 1, 0, 0, 987654321, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) || start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatalf("parsed window=(%s [%s], %s [%s]), want (%s UTC, %s UTC)", start, start.Location(), end, end.Location(), wantStart, wantEnd)
	}
}

func TestBilledUnitsDailyBoundariesWithSkippedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	shortDayEnd := time.Date(2026, time.September, 7, 0, 0, 0, 0, loc)
	if elapsed := shortDayEnd.Sub(start); elapsed != 23*time.Hour {
		t.Fatalf("synthetic local calendar day elapsed %s; want 23h", elapsed)
	}
	for days, end := range map[int]time.Time{
		1: time.Date(2026, time.September, 7, 0, 0, 0, 0, loc),
		2: time.Date(2026, time.September, 8, 0, 0, 0, 0, loc),
		3: time.Date(2026, time.September, 9, 0, 0, 0, 0, loc),
	} {
		got, e := BilledUnits("dia", start.UTC(), end.UTC(), loc.String())
		if e != nil || got != int64(days) {
			t.Fatalf("interval for %d local dates billed %d, err=%v", days, got, e)
		}
	}
}

func TestDailyUnitsUseLocalCalendarAcross25HourDSTDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.April, 4, 0, 0, 0, 0, loc)
	end := time.Date(2026, time.April, 5, 0, 0, 0, 0, loc)
	if elapsed := end.Sub(start); elapsed != 25*time.Hour {
		t.Fatalf("synthetic local calendar day elapsed %s; want 25h", elapsed)
	}
	got, err := BilledUnits("dia", start.UTC(), end.UTC(), loc.String())
	if err != nil || got != 1 {
		t.Fatalf("25-hour local day billed %d units, err=%v; want one calendar day", got, err)
	}
}

func TestDailyUnitsRoundAroundExclusiveLocalMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2030, time.January, 10, 9, 0, 0, 0, loc)
	midnight := time.Date(2030, time.January, 11, 0, 0, 0, 0, loc)
	for _, tc := range []struct {
		name     string
		overage  time.Duration
		wantUnit int64
	}{
		{name: "one nanosecond before", overage: -time.Nanosecond, wantUnit: 1},
		{name: "exact midnight is exclusive", overage: 0, wantUnit: 1},
		{name: "one nanosecond after", overage: time.Nanosecond, wantUnit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := midnight.Add(tc.overage)
			got, err := BilledUnits("dia", start.UTC(), end.UTC(), loc.String())
			if err != nil || got != tc.wantUnit {
				t.Fatalf("daily end %s billed %d units, err=%v; want %d", end.In(loc), got, err, tc.wantUnit)
			}
		})
	}
}

func TestDailyUnitsRoundAroundSkippedLocalMidnightTransition(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.September, 5, 9, 0, 0, 0, loc)
	firstInstant := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	for _, tc := range []struct {
		name     string
		overage  time.Duration
		wantUnit int64
	}{
		{name: "one nanosecond before", overage: -time.Nanosecond, wantUnit: 1},
		{name: "exact first instant is exclusive", overage: 0, wantUnit: 1},
		{name: "one nanosecond after", overage: time.Nanosecond, wantUnit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := firstInstant.Add(tc.overage)
			got, err := BilledUnits("dia", start.UTC(), end.UTC(), loc.String())
			if err != nil || got != tc.wantUnit {
				t.Fatalf("daily end %s billed %d units, err=%v; want %d", end.In(loc), got, err, tc.wantUnit)
			}
		})
	}
}

func TestBilledUnitsMonthlyAnniversaryAtSkippedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.August, 6, 0, 0, 0, 0, loc)
	end := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	got, err := BilledUnits("mes", start.UTC(), end.UTC(), loc.String())
	if err != nil || got != 1 {
		t.Fatalf("monthly interval billed %d months, err=%v", got, err)
	}
}

func TestMonthlyUnitsUseSpaceLocalAnniversaryAcrossSkippedMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.August, 6, 0, 30, 0, 0, loc)
	anniversary, exists := addMonthsClamped(start, 1)
	if !exists {
		t.Fatal("local monthly anniversary did not resolve")
	}
	wantAnniversary := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	if !anniversary.Equal(wantAnniversary) {
		t.Fatalf("resolved local anniversary=%s, want first valid instant %s", anniversary, wantAnniversary)
	}
	end := time.Date(2026, time.September, 6, 1, 1, 0, 0, loc)
	if got := end.Sub(wantAnniversary); got != time.Minute {
		t.Fatalf("fixture ends %s after resolved local anniversary; want 1m", got)
	}
	utcAnniversary := time.Date(2026, time.September, 6, 4, 30, 0, 0, time.UTC)
	if !end.UTC().Before(utcAnniversary) {
		t.Fatalf("fixture must end before UTC one-month anniversary: end=%s anniversary=%s", end.UTC(), utcAnniversary)
	}
	got, err := BilledUnits("mes", start.UTC(), end.UTC(), loc.String())
	if err != nil || got != 2 {
		t.Fatalf("interval one minute past the space-local anniversary billed %d units, err=%v; want 2", got, err)
	}
}

func TestBilledUnitsRejectsMonthlyAnniversaryOnSkippedDate(t *testing.T) {
	loc, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2011, time.November, 30, 9, 0, 0, 0, loc)
	end := time.Date(2011, time.December, 31, 9, 0, 0, 0, loc)
	got, err := BilledUnits("mes", start.UTC(), end.UTC(), loc.String())
	if got != 0 || !errors.Is(err, ErrInvalid) {
		t.Fatalf("monthly interval crossing a skipped anniversary billed %d months, err=%v; want 0 and ErrInvalid", got, err)
	}
}

func TestBilledUnitsChargesOneMonthForPartialFirstMonth(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2030, time.January, 5, 9, 0, 0, 0, loc)
	end := time.Date(2030, time.January, 20, 9, 0, 0, 0, loc)
	got, err := BilledUnits("mes", start.UTC(), end.UTC(), loc.String())
	if err != nil || got != 1 {
		t.Fatalf("partial first month billed %d months, err=%v; want 1", got, err)
	}
}

func TestMonthlyUnitsRoundAroundAnniversaryBoundary(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2030, time.January, 5, 9, 0, 0, 0, loc)
	anniversary := time.Date(2030, time.February, 5, 9, 0, 0, 0, loc)
	for _, tc := range []struct {
		name     string
		overage  time.Duration
		wantUnit int64
	}{
		{name: "one nanosecond before", overage: -time.Nanosecond, wantUnit: 1},
		{name: "exact anniversary", overage: 0, wantUnit: 1},
		{name: "one nanosecond after", overage: time.Nanosecond, wantUnit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := anniversary.Add(tc.overage)
			got, err := BilledUnits("mes", start.UTC(), end.UTC(), loc.String())
			if err != nil || got != tc.wantUnit {
				t.Fatalf("monthly end %s billed %d units, err=%v; want %d", end.In(loc), got, err, tc.wantUnit)
			}
		})
	}
}

func TestBilledUnitsRejectsUnsupportedUnitAndTimezone(t *testing.T) {
	start := time.Date(2030, time.January, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for _, tc := range []struct {
		name string
		unit string
		zone string
	}{
		{name: "unsupported unit", unit: "week", zone: "UTC"},
		{name: "invalid timezone", unit: "hora", zone: "Not/A_Zone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BilledUnits(tc.unit, start, end, tc.zone)
			if got != 0 || !errors.Is(err, ErrInvalid) {
				t.Fatalf("units=%d err=%v, want 0 and ErrInvalid", got, err)
			}
		})
	}
}

func TestBilledUnitsRejectsNonPositiveInterval(t *testing.T) {
	start := time.Date(2030, time.January, 1, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		end  time.Time
	}{
		{name: "empty", end: start},
		{name: "reversed", end: start.Add(-time.Nanosecond)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BilledUnits("hora", start, tc.end, "UTC")
			if got != 0 || !errors.Is(err, ErrInvalid) {
				t.Fatalf("units=%d err=%v, want 0 and ErrInvalid", got, err)
			}
		})
	}
}

func TestHourlyUnitsUseElapsedTimeAcrossDSTTransitions(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		start string
		end   string
	}{
		{name: "fall repeated hour", start: "2026-11-01T01:30:00-04:00", end: "2026-11-01T01:30:00-05:00"},
		{name: "spring skipped hour", start: "2026-03-08T01:30:00-05:00", end: "2026-03-08T03:30:00-04:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tc.start)
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339, tc.end)
			if err != nil {
				t.Fatal(err)
			}
			got, err := BilledUnits("hora", start.UTC(), end.UTC(), loc.String())
			if err != nil || got != 1 {
				t.Fatalf("elapsed hour billed %d units, err=%v", got, err)
			}
		})
	}
}

func TestHourlyUnitsUseElapsedTimeAcrossHalfHourDSTTransitions(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Lord_Howe")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		start     string
		end       string
		elapsed   time.Duration
		wantUnits int64
	}{
		{
			name:      "fall back adds thirty elapsed minutes",
			start:     "2026-04-05T01:30:00+11:00",
			end:       "2026-04-05T02:30:00+10:30",
			elapsed:   90 * time.Minute,
			wantUnits: 2,
		},
		{
			name:      "spring forward removes thirty elapsed minutes",
			start:     "2026-10-04T01:30:00+10:30",
			end:       "2026-10-04T03:00:00+11:00",
			elapsed:   time.Hour,
			wantUnits: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tc.start)
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339, tc.end)
			if err != nil {
				t.Fatal(err)
			}
			if got := start.In(loc).Format(time.RFC3339); got != tc.start {
				t.Fatalf("start fixture resolves in %s as %s", loc, got)
			}
			if got := end.In(loc).Format(time.RFC3339); got != tc.end {
				t.Fatalf("end fixture resolves in %s as %s", loc, got)
			}
			if got := end.Sub(start); got != tc.elapsed {
				t.Fatalf("fixture elapsed %s, want %s", got, tc.elapsed)
			}
			got, err := BilledUnits("hora", start.UTC(), end.UTC(), loc.String())
			if err != nil || got != tc.wantUnits {
				t.Fatalf("elapsed %s billed %d units, err=%v; want %d", end.Sub(start), got, err, tc.wantUnits)
			}
		})
	}
}

func TestHourlyUnitsRoundAroundExactBoundary(t *testing.T) {
	start := time.Date(2030, time.January, 1, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		overage  time.Duration
		wantUnit int64
	}{
		{name: "one nanosecond before", overage: -time.Nanosecond, wantUnit: 1},
		{name: "exact hour", overage: 0, wantUnit: 1},
		{name: "one nanosecond after", overage: time.Nanosecond, wantUnit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := start.Add(time.Hour + tc.overage)
			got, err := BilledUnits("hora", start, end, "UTC")
			if err != nil || got != tc.wantUnit {
				t.Fatalf("duration=%s billed %d units, err=%v; want %d", end.Sub(start), got, err, tc.wantUnit)
			}
		})
	}
}
