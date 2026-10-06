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
