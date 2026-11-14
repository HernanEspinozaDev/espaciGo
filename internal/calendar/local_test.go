package calendar

import (
	"testing"
	"time"
)

func TestResolveCalendarBoundariesAcrossSantiagoMidnightGap(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start, ok := FirstInstantOfDate(2026, time.September, 6, loc)
	if !ok || start.In(loc).Format(time.RFC3339) != "2026-09-06T01:00:00-03:00" {
		t.Fatalf("first instant of Sep 6 = %s, %v", start.In(loc), ok)
	}

	august := time.Date(2026, time.August, 6, 0, 0, 0, 0, loc)
	anniversary, ok := AddMonthsClamped(august, 1)
	if !ok || !anniversary.Equal(start) {
		t.Fatalf("monthly anniversary = %s, %v; want %s", anniversary.In(loc), ok, start.In(loc))
	}
}

func TestResolveMonthlyWallTimeToFirstInstantAfterDSTGap(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.February, 8, 2, 30, 0, 0, loc)
	anniversary, ok := AddMonthsClamped(start, 1)
	if !ok || anniversary.In(loc).Format(time.RFC3339) != "2026-03-08T03:00:00-04:00" {
		t.Fatalf("monthly anniversary within spring gap = %s, %v", anniversary.In(loc), ok)
	}
}

func TestDateOrdinalCountsLeapDayAsOneCalendarDay(t *testing.T) {
	cases := []struct {
		name      string
		year      int
		month     time.Month
		day       int
		nextYear  int
		nextMonth time.Month
		nextDay   int
	}{
		{name: "leap year", year: 2024, month: time.February, day: 28, nextYear: 2024, nextMonth: time.February, nextDay: 29},
		{name: "after leap day", year: 2024, month: time.February, day: 29, nextYear: 2024, nextMonth: time.March, nextDay: 1},
		{name: "century non-leap year", year: 1900, month: time.February, day: 28, nextYear: 1900, nextMonth: time.March, nextDay: 1},
		{name: "400-year leap rule", year: 2000, month: time.February, day: 28, nextYear: 2000, nextMonth: time.February, nextDay: 29},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DateOrdinal(tc.nextYear, tc.nextMonth, tc.nextDay) - DateOrdinal(tc.year, tc.month, tc.day)
			if got != 1 {
				t.Fatalf("next calendar date is %d ordinals later, want 1", got)
			}
		})
	}
}

func TestMonthlyAnniversaryUsesFirstInstantInRepeatedWallTime(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.October, 1, 1, 30, 0, 0, loc)
	anniversary, ok := AddMonthsClamped(start, 1)
	want := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC)
	if !ok || !anniversary.Equal(want) {
		t.Fatalf("monthly anniversary = %s, %v; want first 01:30 occurrence at %s", anniversary.UTC(), ok, want)
	}
}

func TestMonthlyAnniversaryClampsJanuaryEndToFebruaryEnd(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		year    int
		wantDay int
	}{
		{name: "common year", year: 2026, wantDay: 28},
		{name: "leap year", year: 2028, wantDay: 29},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Date(tc.year, time.January, 31, 9, 0, 0, 0, loc)
			got, ok := AddMonthsClamped(start, 1)
			want := time.Date(tc.year, time.February, tc.wantDay, 9, 0, 0, 0, loc)
			if !ok || !got.Equal(want) {
				t.Fatalf("monthly anniversary = %s, %v; want %s", got.In(loc), ok, want)
			}
		})
	}
}

func TestFirstInstantOfEntirelySkippedDateReturnsFalse(t *testing.T) {
	loc, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := FirstInstantOfDate(2011, time.December, 30, loc)
	if ok || !got.IsZero() {
		t.Fatalf("skipped local date resolved to %s, %v; want zero time and false", got.In(loc), ok)
	}
}

func TestFirstInstantOfDateRejectsInvalidCalendarLabels(t *testing.T) {
	cases := []struct {
		name  string
		year  int
		month time.Month
		day   int
	}{
		{name: "day zero", year: 2026, month: time.January, day: 0},
		{name: "day beyond month", year: 2026, month: time.February, day: 30},
		{name: "month zero", year: 2026, month: 0, day: 1},
		{name: "month beyond year", year: 2026, month: 13, day: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FirstInstantOfDate(tc.year, tc.month, tc.day, time.UTC)
			if ok || !got.IsZero() {
				t.Fatalf("invalid date %d-%02d-%02d resolved to %s, %v", tc.year, tc.month, tc.day, got, ok)
			}
		})
	}
}

func TestAddMonthsClampedSupportsNegativeMonthIndexAcrossYearZero(t *testing.T) {
	start := time.Date(0, time.January, 15, 9, 30, 0, 0, time.UTC)
	got, ok := AddMonthsClamped(start, -1)
	want := time.Date(-1, time.December, 15, 9, 30, 0, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("one-month subtraction from year zero = %s, %v; want %s", got, ok, want)
	}
}
