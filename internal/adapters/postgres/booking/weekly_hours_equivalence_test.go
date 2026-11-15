package bookingpg

import (
	"reflect"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

func TestWeeklyScheduleEquivalentIgnoresMetadataAndOrdering(t *testing.T) {
	persisted := booking.WeeklyHours{Enabled: true, TimeZone: "America/Santiago", Days: []booking.WeeklyDay{
		{Weekday: 1, Periods: []booking.WeeklyPeriod{{Open: "09:00", Close: "12:00"}}},
		{Weekday: 2},
	}}
	incoming := booking.WeeklyHours{Enabled: true, TimeZone: "UTC", Days: []booking.WeeklyDay{
		{Weekday: 2},
		{Weekday: 1, Periods: []booking.WeeklyPeriod{{Open: "09:00", Close: "12:00"}}},
	}}
	if !weeklyScheduleEquivalent(persisted, incoming) {
		t.Fatal("same enabled schedule should be equivalent regardless of metadata/order")
	}
	incoming.Days[1].Periods[0].Close = "13:00"
	if weeklyScheduleEquivalent(persisted, incoming) {
		t.Fatal("different periods must not be equivalent")
	}
}

func TestWeeklyScheduleEquivalentNormalizesOrderingWithoutMutatingInputs(t *testing.T) {
	current := booking.WeeklyHours{Enabled: true, TimeZone: "America/Santiago", Days: []booking.WeeklyDay{
		{Weekday: 2, Periods: []booking.WeeklyPeriod{{Open: "14:00", Close: "16:00"}, {Open: "09:00", Close: "12:00"}}},
		{Weekday: 1, Periods: []booking.WeeklyPeriod{{Open: "10:00", Close: "12:00"}, {Open: "08:00", Close: "09:00"}}},
	}}
	requested := booking.WeeklyHours{Enabled: true, TimeZone: "UTC", Days: []booking.WeeklyDay{
		{Weekday: 1, Periods: []booking.WeeklyPeriod{{Open: "08:00", Close: "09:00"}, {Open: "10:00", Close: "12:00"}}},
		{Weekday: 2, Periods: []booking.WeeklyPeriod{{Open: "09:00", Close: "12:00"}, {Open: "14:00", Close: "16:00"}}},
	}}
	currentBefore := cloneWeeklyHoursForTest(current)
	requestedBefore := cloneWeeklyHoursForTest(requested)

	if !weeklyScheduleEquivalent(current, requested) {
		t.Fatal("the same periods should be equivalent after day and period ordering")
	}
	if !reflect.DeepEqual(current, currentBefore) {
		t.Fatalf("comparison mutated persisted schedule: got %#v, want %#v", current, currentBefore)
	}
	if !reflect.DeepEqual(requested, requestedBefore) {
		t.Fatalf("comparison mutated requested schedule: got %#v, want %#v", requested, requestedBefore)
	}
}

func cloneWeeklyHoursForTest(value booking.WeeklyHours) booking.WeeklyHours {
	clone := value
	clone.Days = make([]booking.WeeklyDay, len(value.Days))
	for i, day := range value.Days {
		clone.Days[i] = day
		clone.Days[i].Periods = append([]booking.WeeklyPeriod(nil), day.Periods...)
	}
	return clone
}
