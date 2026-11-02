package documentverificationpg

import (
	"testing"
	"time"
)

func TestEarlierRetentionDeadlinePreservesPriorDeadline(t *testing.T) {
	candidate := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.UTC)
	earlier := candidate.Add(-6 * time.Hour)
	later := candidate.Add(6 * time.Hour)

	tests := []struct {
		name     string
		previous *time.Time
		want     time.Time
	}{
		{name: "no previous deadline", want: candidate},
		{name: "earlier previous deadline", previous: &earlier, want: earlier},
		{name: "later previous deadline", previous: &later, want: candidate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := earlierRetentionDeadline(candidate, tt.previous); !got.Equal(tt.want) {
				t.Fatalf("earlierRetentionDeadline() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestScanLeaseIsAtLeastOneMillisecond(t *testing.T) {
	for _, tt := range []struct {
		name  string
		lease time.Duration
		want  time.Duration
	}{
		{name: "sub-millisecond lease", lease: time.Nanosecond, want: time.Millisecond},
		{name: "just below one millisecond", lease: time.Millisecond - time.Nanosecond, want: time.Millisecond},
		{name: "one millisecond lease", lease: time.Millisecond, want: time.Millisecond},
		{name: "longer lease unchanged", lease: 2 * time.Minute, want: 2 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeScanLease(tt.lease); got != tt.want {
				t.Fatalf("normalizeScanLease(%s) = %s, want %s", tt.lease, got, tt.want)
			}
		})
	}
}
