package documentverificationpg

import (
	"testing"
	"time"
)

func TestNewRetentionProcessorValidatesDurationBounds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lease     time.Duration
		retry     time.Duration
		wantValid bool
	}{
		{name: "minimum positive durations", lease: time.Nanosecond, retry: time.Nanosecond, wantValid: true},
		{name: "maximum accepted durations", lease: 5 * time.Minute, retry: 24 * time.Hour, wantValid: true},
		{name: "zero lease", lease: 0, retry: time.Second},
		{name: "negative lease", lease: -time.Nanosecond, retry: time.Second},
		{name: "lease over maximum", lease: 5*time.Minute + time.Nanosecond, retry: time.Second},
		{name: "zero retry", lease: time.Second, retry: 0},
		{name: "negative retry", lease: time.Second, retry: -time.Nanosecond},
		{name: "retry over maximum", lease: time.Second, retry: 24*time.Hour + time.Nanosecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			processor := NewRetentionProcessor(&retentionFake{}, &retentionStoreFake{}, tc.lease, tc.retry)
			if (processor != nil) != tc.wantValid {
				t.Fatalf("NewRetentionProcessor(lease=%s,retry=%s) nonnil=%t; want %t", tc.lease, tc.retry, processor != nil, tc.wantValid)
			}
		})
	}
}
