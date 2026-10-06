package booking

import (
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
	if !IntervalFitsWeeklyHours(firstValid, firstValid.Add(time.Hour), "America/Santiago", schedule) {
		t.Fatal("valid post-gap interval rejected")
	}
	fold := time.Date(2026, time.April, 4, 0, 0, 0, 0, time.UTC)
	repeated := localWallInstants(fold, 23, 0, loc)
	if len(repeated) != 2 || !repeated[0].Before(repeated[1]) {
		t.Fatalf("expected two ordered local 23:00 instants, got %v", repeated)
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
