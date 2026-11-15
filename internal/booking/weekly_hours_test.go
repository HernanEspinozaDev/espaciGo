package booking

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testWeek(periods ...WeeklyPeriod) WeeklyHours {
	days := make([]WeeklyDay, 7)
	for i := range days {
		days[i] = WeeklyDay{Weekday: i + 1, Periods: []WeeklyPeriod{}}
	}
	days[3].Periods = periods // Thursday.
	return WeeklyHours{Enabled: true, TimeZone: "America/Santiago", Days: days}
}

func TestWeeklyHoursValidationAndHalfOpenIntervals(t *testing.T) {
	valid := testWeek(WeeklyPeriod{Open: "09:00", Close: "12:00"}, WeeklyPeriod{Open: "13:00", Close: "24:00"})
	if err := ValidateWeeklyHours(valid); err != nil {
		t.Fatalf("valid split shift rejected: %v", err)
	}
	day := time.Date(2030, time.January, 3, 0, 0, 0, 0, time.UTC) // Thursday.
	zone := "UTC"
	interval := func(h1, m1, h2, m2 int) (time.Time, time.Time) {
		return time.Date(day.Year(), day.Month(), day.Day(), h1, m1, 0, 0, time.UTC), time.Date(day.Year(), day.Month(), day.Day(), h2, m2, 0, 0, time.UTC)
	}
	start, end := interval(9, 30, 11, 30)
	if !IntervalFitsWeeklyHours(start, end, zone, valid) {
		t.Fatal("interval inside morning period was rejected")
	}
	start, end = interval(11, 30, 13, 30)
	if IntervalFitsWeeklyHours(start, end, zone, valid) {
		t.Fatal("interval crossing the pause was accepted")
	}
	start, end = interval(12, 0, 13, 0)
	if IntervalFitsWeeklyHours(start, end, zone, valid) {
		t.Fatal("interval in the pause was accepted")
	}
	start, end = interval(23, 0, 24, 0)
	if !IntervalFitsWeeklyHours(start, end, zone, valid) {
		t.Fatal("24:00 closing boundary was not treated as exclusive end")
	}
	start, end = interval(23, 0, 24, 1)
	if IntervalFitsWeeklyHours(start, end, zone, valid) {
		t.Fatal("interval past 24:00 was accepted")
	}
	closed := valid
	closed.Days[3].Periods = nil
	if err := ValidateWeeklyHours(closed); err != nil {
		t.Fatal(err)
	}
	start, end = interval(9, 0, 10, 0)
	if IntervalFitsWeeklyHours(start, end, zone, closed) {
		t.Fatal("active schedule with empty weekday must be closed")
	}
	invalid := testWeek(WeeklyPeriod{Open: "09:00", Close: "13:00"}, WeeklyPeriod{Open: "12:00", Close: "17:00"})
	if ValidateWeeklyHours(invalid) == nil {
		t.Fatal("overlapping periods accepted")
	}
	invalid = testWeek(WeeklyPeriod{Open: "22:00", Close: "02:00"})
	if ValidateWeeklyHours(invalid) == nil {
		t.Fatal("cross-midnight period accepted")
	}
	invalid = testWeek(WeeklyPeriod{Open: "24:00", Close: "24:00"})
	if ValidateWeeklyHours(invalid) == nil {
		t.Fatal("non-positive 24:00 period accepted")
	}
}

func TestIntervalFitsWeeklyHoursUsesLocalWeekdayForIANAZone(t *testing.T) {
	zone := "America/Santiago"
	schedule := testWeek()
	schedule.Days[3].Periods = nil                                             // Thursday is closed.
	schedule.Days[2].Periods = []WeeklyPeriod{{Open: "21:00", Close: "22:00"}} // Wednesday.
	start := time.Date(2030, time.January, 3, 0, 0, 0, 0, time.UTC)            // Thursday UTC, Wednesday locally.
	end := start.Add(30 * time.Minute)
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	if got := start.In(loc).Format("2006-01-02 15:04 -07:00"); got != "2030-01-02 21:00 -03:00" {
		t.Fatalf("local interval start = %q", got)
	}
	if !IntervalFitsWeeklyHours(start, end, zone, schedule) {
		t.Fatal("interval within the local Wednesday period was rejected")
	}
	if IntervalFitsWeeklyHours(start, end, "UTC", schedule) {
		t.Fatal("interval was accepted using the UTC weekday instead of the configured local weekday")
	}
}

func TestNormalizeWeeklyDaysSerializesClosedDaysAsEmptyArrays(t *testing.T) {
	days := make([]WeeklyDay, 7)
	for i := range days {
		days[i] = WeeklyDay{Weekday: i + 1}
	}

	encoded, err := json.Marshal(normalizeWeeklyDays(days))
	if err != nil {
		t.Fatalf("marshal normalized schedule: %v", err)
	}
	if strings.Contains(string(encoded), `"periods":null`) {
		t.Fatalf("closed-day periods must serialize as arrays, got %s", encoded)
	}
}

func TestValidateWeeklyHoursRejectsIncompleteShapeAndMalformedClock(t *testing.T) {
	incomplete := testWeek()
	incomplete.Days = incomplete.Days[:6]
	tooManyPeriods := testWeek(
		WeeklyPeriod{Open: "01:00", Close: "02:00"},
		WeeklyPeriod{Open: "03:00", Close: "04:00"},
		WeeklyPeriod{Open: "05:00", Close: "06:00"},
	)
	malformedClock := testWeek(WeeklyPeriod{Open: "9:00", Close: "10:00"})

	for _, tc := range []struct {
		name  string
		value WeeklyHours
	}{
		{name: "incomplete seven-day shape", value: incomplete},
		{name: "more than two periods", value: tooManyPeriods},
		{name: "non-canonical clock", value: malformedClock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWeeklyHours(tc.value); err != ErrInvalid {
				t.Fatalf("ValidateWeeklyHours() error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}

func TestIntervalFitsWeeklyHoursFailsClosedForInvalidZoneAndPeriod(t *testing.T) {
	start := time.Date(2030, time.January, 3, 9, 0, 0, 0, time.UTC) // Thursday.
	end := start.Add(time.Hour)
	valid := testWeek(WeeklyPeriod{Open: "09:00", Close: "10:00"})
	for _, zone := range []string{"Unknown/Zone", "../UTC"} {
		if IntervalFitsWeeklyHours(start, end, zone, valid) {
			t.Errorf("IntervalFitsWeeklyHours() accepted invalid zone %q", zone)
		}
	}

	invalidPeriod := testWeek(WeeklyPeriod{Open: "09:00", Close: "09:00"})
	if IntervalFitsWeeklyHours(start, end, "UTC", invalidPeriod) {
		t.Fatal("IntervalFitsWeeklyHours() accepted a non-positive enabled period")
	}
}

func TestIntervalFitsWeeklyHoursIgnoresDisabledSchedule(t *testing.T) {
	schedule := testWeek(WeeklyPeriod{Open: "09:00", Close: "10:00"})
	schedule.Enabled = false
	start := time.Date(2030, time.January, 3, 15, 0, 0, 0, time.UTC) // Thursday.
	end := start.Add(time.Hour)
	if !IntervalFitsWeeklyHours(start, end, "UTC", schedule) {
		t.Fatal("disabled weekly schedule must not restrict a positive interval")
	}
	if IntervalFitsWeeklyHours(start, start, "UTC", schedule) {
		t.Fatal("disabled weekly schedule must still reject a zero-length interval")
	}
}

func TestIntervalFitsWeeklyHoursAcrossAdjacentPeriods(t *testing.T) {
	schedule := testWeek(
		WeeklyPeriod{Open: "13:00", Close: "17:00"},
		WeeklyPeriod{Open: "09:00", Close: "13:00"},
	)
	if err := ValidateWeeklyHours(schedule); err != nil {
		t.Fatalf("adjacent periods should be a valid continuous schedule: %v", err)
	}
	start := time.Date(2030, time.January, 3, 12, 0, 0, 0, time.UTC) // Thursday.
	if !IntervalFitsWeeklyHours(start, start.Add(2*time.Hour), "UTC", schedule) {
		t.Fatal("interval spanning adjacent periods should fit their continuous availability")
	}
}

func TestNormalizeWeeklyDaysSortsPeriodsWithoutMutatingInput(t *testing.T) {
	input := []WeeklyDay{{
		Weekday: 4,
		Periods: []WeeklyPeriod{
			{Open: "13:00", Close: "14:00"},
			{Open: "09:00", Close: "10:00"},
		},
	}}

	got := normalizeWeeklyDays(input)
	if got[0].Periods[0].Open != "09:00" || got[0].Periods[1].Open != "13:00" {
		t.Fatalf("normalized periods = %+v, want sorted by opening time", got[0].Periods)
	}
	if input[0].Periods[0].Open != "13:00" {
		t.Fatalf("normalizeWeeklyDays mutated input: %+v", input[0].Periods)
	}
}

func TestWeeklyHoursDSTGapsAndRepeatedTimes(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	spring := time.Date(2026, time.September, 6, 0, 0, 0, 0, time.UTC)
	if got := localWallInstants(spring, 0, 30, loc); len(got) != 0 {
		t.Fatalf("nonexistent spring time candidates=%v", got)
	}
	schedule := testWeek()
	schedule.Days[6].Periods = []WeeklyPeriod{{Open: "01:00", Close: "04:00"}} // Sunday.
	firstValid, ok := firstInstantOfLocalDate(spring, loc)
	if !ok || firstValid.In(loc).Format("2006-01-02 15:04") != "2026-09-06 01:00" {
		t.Fatalf("first valid spring instant=%v ok=%v", firstValid, ok)
	}
	if got := firstValid.UTC().Format(time.RFC3339); got != "2026-09-06T04:00:00Z" {
		t.Fatalf("first valid spring UTC instant=%q, want %q", got, "2026-09-06T04:00:00Z")
	}
	if !IntervalFitsWeeklyHours(firstValid, firstValid.Add(time.Hour), "America/Santiago", schedule) {
		t.Fatal("valid post-gap interval rejected")
	}
	fold := time.Date(2026, time.April, 4, 0, 0, 0, 0, time.UTC)
	repeated := localWallInstants(fold, 23, 0, loc)
	if len(repeated) != 2 || !repeated[0].Before(repeated[1]) {
		t.Fatalf("expected two ordered local 23:00 instants, got %v", repeated)
	}
	firstRepeatedUTC := repeated[0].UTC().Format(time.RFC3339)
	secondRepeatedUTC := repeated[1].UTC().Format(time.RFC3339)
	if firstRepeatedUTC != "2026-04-05T02:00:00Z" || secondRepeatedUTC != "2026-04-05T03:00:00Z" {
		t.Fatalf("repeated local 23:00 UTC instants=%q and %q", firstRepeatedUTC, secondRepeatedUTC)
	}
	schedule.Days[5].Periods = []WeeklyPeriod{{Open: "22:00", Close: "24:00"}} // Saturday.
	for _, start := range repeated {
		if !IntervalFitsWeeklyHours(start, start.Add(time.Hour), "America/Santiago", schedule) {
			t.Fatalf("repeated-hour interval %s rejected", start)
		}
	}
	schedule.Days[6].Periods = []WeeklyPeriod{{Open: "00:00", Close: "02:00"}} // Sunday.
	crossMidnightStart := time.Date(2026, time.April, 5, 1, 30, 0, 0, time.UTC).In(loc)
	crossMidnightEnd := time.Date(2026, time.April, 5, 4, 30, 0, 0, time.UTC).In(loc)
	if IntervalFitsWeeklyHours(crossMidnightStart, crossMidnightEnd, "America/Santiago", schedule) {
		t.Fatal("interval crossing the local date boundary fit adjacent daily periods")
	}
	if IntervalFitsWeeklyHours(repeated[0], repeated[0], "America/Santiago", schedule) {
		t.Fatal("zero-length interval accepted")
	}
}

func TestWeeklyHoursAdjacentPeriodsAcrossDSTTransition(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	schedule := testWeek()
	schedule.Days[3].Periods = []WeeklyPeriod{}
	schedule.Days[6].Periods = []WeeklyPeriod{
		{Open: "02:00", Close: "04:00"},
		{Open: "01:00", Close: "02:00"},
	}
	if err := ValidateWeeklyHours(schedule); err != nil {
		t.Fatalf("adjacent DST-day schedule rejected: %v", err)
	}
	start := time.Date(2026, time.March, 8, 6, 30, 0, 0, time.UTC)
	end := time.Date(2026, time.March, 8, 7, 30, 0, 0, time.UTC)
	if got := start.In(loc).Format("2006-01-02 15:04 MST"); got != "2026-03-08 01:30 EST" {
		t.Fatalf("local interval start = %q", got)
	}
	if got := end.In(loc).Format("2006-01-02 15:04 MST"); got != "2026-03-08 03:30 EDT" {
		t.Fatalf("local interval end = %q", got)
	}
	if !IntervalFitsWeeklyHours(start, end, loc.String(), schedule) {
		t.Fatal("interval spanning adjacent periods across the spring DST gap was rejected")
	}
}

func TestWeeklyHoursIntervalAcrossFallBackTransition(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	schedule := testWeek()
	schedule.Days[3].Periods = []WeeklyPeriod{}
	schedule.Days[6].Periods = []WeeklyPeriod{{Open: "01:00", Close: "02:00"}} // Sunday.
	if err := ValidateWeeklyHours(schedule); err != nil {
		t.Fatalf("fall-back schedule rejected: %v", err)
	}

	start := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC)
	end := time.Date(2026, time.November, 1, 6, 30, 0, 0, time.UTC)
	if got := start.In(loc).Format("2006-01-02 15:04 MST"); got != "2026-11-01 01:30 EDT" {
		t.Fatalf("local interval start = %q", got)
	}
	if got := end.In(loc).Format("2006-01-02 15:04 MST"); got != "2026-11-01 01:30 EST" {
		t.Fatalf("local interval end = %q", got)
	}
	if !IntervalFitsWeeklyHours(start, end, loc.String(), schedule) {
		t.Fatal("interval fully inside Sunday hours across the repeated fall-back hour was rejected")
	}
}

func TestWeeklyHoursFallBackDoesNotCollapseRepeatedWallClockSpan(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	schedule := testWeek()
	schedule.Days[3].Periods = []WeeklyPeriod{}
	schedule.Days[6].Periods = []WeeklyPeriod{{Open: "01:30", Close: "01:45"}} // Sunday.
	if err := ValidateWeeklyHours(schedule); err != nil {
		t.Fatalf("fall-back schedule rejected: %v", err)
	}
	start := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC) // 01:30 EDT.
	end := time.Date(2026, time.November, 1, 6, 45, 0, 0, time.UTC)   // 01:45 EST.
	if IntervalFitsWeeklyHours(start, end, loc.String(), schedule) {
		t.Fatal("interval spanning out-of-hours parts of the repeated wall-clock hour was accepted")
	}
}

func TestWeeklyHoursLordHoweHalfHourDSTTransitions(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Lord_Howe")
	if err != nil {
		t.Fatal(err)
	}

	spring := testWeek()
	spring.Days[3].Periods = []WeeklyPeriod{}
	spring.Days[6].Periods = []WeeklyPeriod{
		{Open: "01:30", Close: "02:00"},
		{Open: "02:30", Close: "03:00"},
	}
	if err := ValidateWeeklyHours(spring); err != nil {
		t.Fatalf("valid adjacent periods around half-hour DST gap rejected: %v", err)
	}
	springStart := time.Date(2026, time.October, 3, 15, 0, 0, 0, time.UTC)
	springEnd := time.Date(2026, time.October, 3, 15, 45, 0, 0, time.UTC)
	if got := springStart.In(loc).Format("2006-01-02 15:04 -07:00"); got != "2026-10-04 01:30 +10:30" {
		t.Fatalf("local spring interval start = %q", got)
	}
	if got := springEnd.In(loc).Format("2006-01-02 15:04 -07:00"); got != "2026-10-04 02:45 +11:00" {
		t.Fatalf("local spring interval end = %q", got)
	}
	if !IntervalFitsWeeklyHours(springStart, springEnd, loc.String(), spring) {
		t.Fatal("interval covered by adjacent periods across the half-hour spring gap was rejected")
	}
	springGap := testWeek()
	springGap.Days[3].Periods = []WeeklyPeriod{}
	springGap.Days[6].Periods = []WeeklyPeriod{
		{Open: "01:30", Close: "02:00"},
		{Open: "02:45", Close: "03:00"},
	}
	if IntervalFitsWeeklyHours(springStart, springEnd, loc.String(), springGap) {
		t.Fatal("interval containing an uncovered post-gap half hour was accepted")
	}

	fall := testWeek()
	fall.Days[3].Periods = []WeeklyPeriod{}
	fall.Days[6].Periods = []WeeklyPeriod{{Open: "01:30", Close: "02:00"}}
	fallStart := time.Date(2026, time.April, 4, 14, 45, 0, 0, time.UTC)
	fallEnd := time.Date(2026, time.April, 4, 15, 15, 0, 0, time.UTC)
	if got := fallStart.In(loc).Format("2006-01-02 15:04 -07:00"); got != "2026-04-05 01:45 +11:00" {
		t.Fatalf("local fall interval start = %q", got)
	}
	if got := fallEnd.In(loc).Format("2006-01-02 15:04 -07:00"); got != "2026-04-05 01:45 +10:30" {
		t.Fatalf("local fall interval end = %q", got)
	}
	if !IntervalFitsWeeklyHours(fallStart, fallEnd, loc.String(), fall) {
		t.Fatal("interval fully covered across the half-hour fall overlap was rejected")
	}
	narrowFall := testWeek()
	narrowFall.Days[3].Periods = []WeeklyPeriod{}
	narrowFall.Days[6].Periods = []WeeklyPeriod{{Open: "01:40", Close: "01:50"}}
	if IntervalFitsWeeklyHours(fallStart, fallEnd, loc.String(), narrowFall) {
		t.Fatal("repeated wall-clock interval outside half-hour schedule boundaries was accepted")
	}
}
