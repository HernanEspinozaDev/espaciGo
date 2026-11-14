package rentalflow

import (
	"testing"
	"time"
)

func TestValidDurationRejectsUnknownRateUnit(t *testing.T) {
	if validDuration("unknown", 1) {
		t.Fatal("unsupported rate unit accepted a duration")
	}
}

func TestIntervalMatchesRequestRejectsInvalidZoneDateAndUnit(t *testing.T) {
	const dateText = "2030-01-02"
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	interval := Interval{
		StartAt: time.Date(2030, time.January, 2, 10, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2030, time.January, 2, 11, 0, 0, 0, time.UTC),
	}
	cases := []struct {
		name     string
		unit     string
		dateText string
		zone     string
	}{
		{name: "invalid zone", unit: "hora", dateText: dateText, zone: "Not/A_Zone"},
		{name: "invalid date", unit: "hora", dateText: "2030-1-02", zone: "UTC"},
		{name: "unsupported unit", unit: "unknown", dateText: dateText, zone: "UTC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if intervalMatchesRequest(interval, tc.unit, tc.dateText, 1, tc.zone, now) {
				t.Fatalf("accepted unit=%q date=%q zone=%q", tc.unit, tc.dateText, tc.zone)
			}
		})
	}
}

func TestDateWithinHorizonRejectsInvalidDate(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	if dateWithinHorizon("2030-1-02", now, time.UTC) {
		t.Fatal("accepted a non-canonical date")
	}
}

func TestExportedRequestValidatorsUseCanonicalDomainRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		valid bool
		check func(string) bool
	}{
		{name: "uuid", value: "11111111-1111-4111-8111-111111111111", valid: true, check: ValidUUID},
		{name: "malformed uuid", value: "not-a-uuid", check: ValidUUID},
		{name: "date", value: "2030-02-28", valid: true, check: ValidDate},
		{name: "impossible date", value: "2030-02-30", check: ValidDate},
		{name: "timestamp", value: "2030-02-28T12:30:00.123Z", valid: true, check: ValidTimestamp},
		{name: "malformed timestamp", value: "2030-02-30T12:30:00Z", check: ValidTimestamp},
		{name: "zero timestamp", value: "0001-01-01T00:00:00Z", check: ValidTimestamp},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.check(tc.value); got != tc.valid {
				t.Fatalf("validator(%q)=%v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}
