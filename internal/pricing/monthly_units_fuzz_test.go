package pricing

import (
	"testing"
	"time"
)

func FuzzMonthlyBilledUnitsRoundAtLocalAnniversary(f *testing.F) {
	zones := []string{"America/Santiago", "America/New_York", "Europe/Berlin", "UTC"}
	baseUnix := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
	spanSeconds := uint64(time.Date(2040, time.January, 1, 0, 0, 0, 0, time.UTC).Unix() - baseUnix)

	f.Add(time.Date(2026, time.August, 6, 4, 30, 0, 0, time.UTC).Unix()-baseUnix, uint8(0))
	f.Add(time.Date(2026, time.February, 8, 7, 30, 0, 0, time.UTC).Unix()-baseUnix, uint8(1))
	f.Add(time.Date(2026, time.October, 25, 0, 30, 0, 0, time.UTC).Unix()-baseUnix, uint8(2))
	f.Add(int64(0), uint8(3))

	f.Fuzz(func(t *testing.T, startOffset int64, zoneIndex uint8) {
		startUnix := baseUnix + int64(uint64(startOffset)%spanSeconds)
		startUTC := time.Unix(startUnix, 0).UTC()
		zone := zones[int(zoneIndex)%len(zones)]
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatalf("load timezone %q: %v", zone, err)
		}
		anniversary, exists := addMonthsClamped(startUTC.In(loc), 1)
		if !exists || !anniversary.After(startUTC) {
			t.Fatalf("one-month anniversary failed to resolve: start=%s zone=%q anniversary=%s", startUTC.In(loc), zone, anniversary)
		}

		for _, tc := range []struct {
			name string
			end  time.Time
			want int64
		}{
			{name: "before", end: anniversary.Add(-time.Nanosecond), want: 1},
			{name: "exact", end: anniversary, want: 1},
			{name: "after", end: anniversary.Add(time.Nanosecond), want: 2},
		} {
			got, err := BilledUnits("mes", startUTC, tc.end.UTC(), zone)
			if err != nil || got != tc.want {
				t.Fatalf("zone=%q start=%s end=%s billed=%d err=%v, want %d at %s anniversary boundary", zone, startUTC.In(loc), tc.end.In(loc), got, err, tc.want, tc.name)
			}
		}
	})
}
