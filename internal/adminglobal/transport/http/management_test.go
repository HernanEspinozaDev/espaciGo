package http

import (
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestRecentAuthenticationUsesServerTimestampAndTenMinuteTTL(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{name: "fresh", at: now.Add(-time.Minute), want: true},
		{name: "ttl boundary", at: now.Add(-recentAuthenticationTTL), want: true},
		{name: "stale", at: now.Add(-recentAuthenticationTTL - time.Nanosecond), want: false},
		{name: "future", at: now.Add(time.Second), want: false},
		{name: "missing", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := recentlyAuthenticated(tc.at, now); got != tc.want {
				t.Fatalf("recentlyAuthenticated(%v)=%v want %v", tc.at, got, tc.want)
			}
		})
	}
	if recentlyAuthenticated(identity.Principal{}.AuthenticatedAt, now) {
		t.Fatal("zero authenticated time accepted")
	}
}
