package documentverificationpg

import (
	"testing"
	"time"
)

func TestRetentionRetryDelayAppliesExponentialBackoffAndCap(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    time.Duration
		attempt int16
		want    time.Duration
	}{
		{name: "first attempt uses base", base: 3 * time.Second, attempt: 1, want: 3 * time.Second},
		{name: "second attempt doubles", base: 3 * time.Second, attempt: 2, want: 6 * time.Second},
		{name: "third attempt quadruples", base: 3 * time.Second, attempt: 3, want: 12 * time.Second},
		{name: "near cap saturates without overflow", base: 13 * time.Hour, attempt: 2, want: 24 * time.Hour},
		{name: "maximum int16 attempt remains capped", base: time.Second, attempt: 32767, want: 24 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retentionRetryDelay(tc.base, tc.attempt); got != tc.want {
				t.Fatalf("retentionRetryDelay(%s, %d)=%s; want %s", tc.base, tc.attempt, got, tc.want)
			}
		})
	}
}
