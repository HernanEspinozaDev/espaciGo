package pricing

import (
	"testing"
	"time"
)

func FuzzDailyBilledUnitsCountSpaceLocalDatesNearMidnight(f *testing.F) {
	zones := []string{"America/Santiago", "America/New_York", "Europe/Berlin", "UTC"}
	baseDate := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)
	spanDays := uint64(time.Date(2040, time.January, 1, 0, 0, 0, 0, time.UTC).Sub(baseDate) / (24 * time.Hour))
	seedDay := func(value time.Time) int64 {
		return int64(time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC).Sub(baseDate) / (24 * time.Hour))
	}

	f.Add(seedDay(time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)), uint8(0), uint8(59), uint16(120))
	f.Add(seedDay(time.Date(2026, time.April, 4, 0, 0, 0, 0, time.UTC)), uint8(0), uint8(58), uint16(300))
	f.Add(seedDay(time.Date(2026, time.March, 7, 0, 0, 0, 0, time.UTC)), uint8(1), uint8(59), uint16(120))
	f.Add(seedDay(time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)), uint8(2), uint8(59), uint16(120))

	f.Fuzz(func(t *testing.T, daySeed int64, zoneIndex, minuteSeed uint8, durationSeed uint16) {
		day := baseDate.AddDate(0, 0, int(uint64(daySeed)%spanDays))
		zone := zones[int(zoneIndex)%len(zones)]
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatalf("load timezone %q: %v", zone, err)
		}
		year, month, date := day.Date()
		start := time.Date(year, month, date, 23, int(minuteSeed%60), 0, 0, loc)
		elapsed := time.Duration(uint64(durationSeed)%7200+1)*time.Second + time.Nanosecond
		end := start.Add(elapsed)
		startLocal, endLocal := start.In(loc), end.In(loc)
		want := civilDateOrdinal(endLocal) - civilDateOrdinal(startLocal) + 1

		got, err := BilledUnits("dia", start.UTC(), end.UTC(), zone)
		if err != nil || got != want {
			t.Fatalf("zone=%q start=%s end=%s billed=%d err=%v, want %d inclusive local-date units", zone, startLocal, endLocal, got, err, want)
		}
	})
}

func civilDateOrdinal(value time.Time) int64 {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / int64((24 * time.Hour).Seconds())
}
