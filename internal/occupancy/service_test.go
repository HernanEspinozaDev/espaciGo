package occupancy

import (
	"testing"
	"time"
)

func TestParseIntervalRequiresFiniteRFC3339IncreasingBounds(t *testing.T) {
	start, end, err := parseInterval("2030-01-01T13:00:00Z", "2030-01-01T14:00:00Z")
	if err != nil || !start.Equal(time.Date(2030, 1, 1, 13, 0, 0, 0, time.UTC)) || !end.After(start) {
		t.Fatalf("parsed interval = %s .. %s, err=%v", start, end, err)
	}
	for _, pair := range [][2]string{
		{"2030-01-01T10:00:00Z", "2030-01-01T10:00:00Z"},
		{"2030-01-01T11:00:00Z", "2030-01-01T10:00:00Z"},
		{"2030-01-01", "2030-01-01T10:00:00Z"},
		{"2030-01-01T10:00:00-03:00", "2030-01-01T11:00:00-03:00"},
	} {
		if _, _, err := parseInterval(pair[0], pair[1]); err == nil {
			t.Errorf("accepted invalid interval %q .. %q", pair[0], pair[1])
		}
	}
}

func TestValidTimeZoneUsesIANAZoneDatabase(t *testing.T) {
	for _, zone := range []string{"America/Santiago", "Pacific/Auckland", "UTC"} {
		if !validTimeZone(zone) {
			t.Errorf("rejected IANA zone %q", zone)
		}
	}
	for _, zone := range []string{"", " ../etc/passwd", "America/../UTC", "not-a-zone"} {
		if validTimeZone(zone) {
			t.Errorf("accepted invalid zone %q", zone)
		}
	}
}
