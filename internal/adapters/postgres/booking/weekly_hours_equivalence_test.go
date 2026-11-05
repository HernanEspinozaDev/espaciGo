package bookingpg

import (
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
