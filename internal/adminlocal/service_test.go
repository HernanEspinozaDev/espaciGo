package adminlocal

import (
	"testing"
	"time"
)

func TestPeriodUsesCalendarDaysInExplicitZone(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, loc)
	valid := Period{From: start.UTC(), Until: start.AddDate(0, 0, 31).UTC(), TimeZone: loc.String()}
	if err := valid.Validate(); err != nil {
		t.Fatalf("31 calendar days across DST rejected: %v", err)
	}
	tooLong := valid
	tooLong.Until = tooLong.Until.Add(time.Second)
	if err := tooLong.Validate(); err == nil {
		t.Fatal("period beyond 31 calendar days accepted")
	}
	missingZone := valid
	missingZone.TimeZone = ""
	if err := missingZone.Validate(); err == nil {
		t.Fatal("period without explicit IANA zone accepted")
	}
}

func TestReportPeriodNormalizesInstantsToUTCButKeepsIANAZone(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, time.October, 1, 0, 0, 0, 0, loc)
	period := Period{From: from, Until: from.Add(24 * time.Hour), TimeZone: loc.String()}.normalizedUTC()
	if period.TimeZone != "America/Santiago" || period.From.Location() != time.UTC || period.Until.Location() != time.UTC {
		t.Fatalf("period was not normalized while preserving its explicit zone: %+v", period)
	}
}
