package pricing

import (
	"testing"
	"time"
)

func FuzzHourlyBilledUnitsUseElapsedTimeAcrossZones(f *testing.F) {
	zones := []string{"UTC", "America/Santiago", "America/New_York", "Pacific/Apia"}
	spanSeconds := uint64(5 * 365 * 24 * 60 * 60)
	maxDuration := uint64(7 * 24 * time.Hour)
	baseUnix := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

	f.Add(int64(time.Date(2026, time.March, 8, 6, 30, 0, 0, time.UTC).Unix()-baseUnix), int64(2*time.Hour), uint8(2))
	f.Add(int64(time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC).Unix()-baseUnix), int64(time.Hour+time.Nanosecond), uint8(2))
	f.Add(int64(0), int64(1), uint8(0))

	f.Fuzz(func(t *testing.T, startOffset, durationSeed int64, zoneIndex uint8) {
		startUnix := baseUnix + int64(uint64(startOffset)%spanSeconds)
		start := time.Unix(startUnix, 0).UTC()
		duration := time.Duration(uint64(durationSeed)%maxDuration + 1)
		end := start.Add(duration)
		zone := zones[int(zoneIndex)%len(zones)]

		got, err := BilledUnits("hora", start, end, zone)
		if err != nil {
			t.Fatalf("BilledUnits(hora, %s, %s, %q): %v", start, end, zone, err)
		}
		want := (int64(duration) + int64(time.Hour) - 1) / int64(time.Hour)
		if got != want {
			t.Fatalf("BilledUnits(hora, duration=%s, zone=%q)=%d, want %d elapsed-time units", duration, zone, got, want)
		}
	})
}
