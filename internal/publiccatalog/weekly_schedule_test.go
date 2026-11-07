package publiccatalog

import (
	"encoding/json"
	"testing"
)

func TestWeeklyScheduleWireUsesExactAllowlistedKeys(t *testing.T) {
	space := Space{WeeklySchedule: &PublicWeeklySchedule{Enabled: true, TimeZone: "America/Santiago", Days: []PublicWeeklyDay{{Weekday: 1, Periods: []PublicWeeklyPeriod{{Open: "09:00", Close: "17:00"}}}}}}
	data, err := json.Marshal(space)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	var schedule map[string]json.RawMessage
	if err := json.Unmarshal(wire["weekly_schedule"], &schedule); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, schedule, []string{"enabled", "time_zone", "days"})
	var days []map[string]json.RawMessage
	if err := json.Unmarshal(schedule["days"], &days); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, days[0], []string{"weekday", "periods"})
	var periods []map[string]json.RawMessage
	if err := json.Unmarshal(days[0]["periods"], &periods); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, periods[0], []string{"open", "close"})
}

func TestNilWeeklyScheduleIsOmittedFromWire(t *testing.T) {
	data, err := json.Marshal(Space{})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if _, exists := wire["weekly_schedule"]; exists {
		t.Fatal("nil schedule must be omitted")
	}
}

func assertKeys(t *testing.T, got map[string]json.RawMessage, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("keys=%v want %v", got, want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
}
