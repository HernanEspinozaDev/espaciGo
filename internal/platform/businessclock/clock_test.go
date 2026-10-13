package businessclock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromEnvRealClockByDefaultAndReadsManualAdvances(t *testing.T) {
	clock, err := FromEnv(func(string) string { return "" })
	if err != nil || time.Since(clock()) > time.Second {
		t.Fatalf("default clock should be real: %v", err)
	}
	path := filepath.Join(t.TempDir(), "clock.json")
	write := func(value string) {
		t.Helper()
		data, _ := json.Marshal(map[string]string{"now": value})
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("2026-10-13T05:00:00-03:00")
	manual, err := newState(path)
	if err != nil {
		t.Fatal(err)
	}
	clock = manual.now
	want := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	if got := clock(); !got.Equal(want) {
		t.Fatalf("initial clock=%s want=%s", got, want)
	}
	write("2026-10-14T02:00:00-03:00")
	if got := clock(); !got.Equal(want.Add(21 * time.Hour)) {
		t.Fatalf("advanced clock=%s", got)
	}
}

func TestManualClockRequiresLocalTrialAndExpectedMount(t *testing.T) {
	for _, env := range []map[string]string{
		{"LOCAL_BOOKING_TRIAL": "0", "LOCAL_DEMO_BUSINESS_CLOCK_FILE": "/run/fe14-demo/clock.json"},
		{"LOCAL_BOOKING_TRIAL": "1", "LOCAL_DEMO_BUSINESS_CLOCK_FILE": "/etc/clock.json"},
		{"LOCAL_BOOKING_TRIAL": "1", "LOCAL_DEMO_BUSINESS_CLOCK_FILE": "/run/fe14-demo/../secrets.json"},
	} {
		if _, err := FromEnv(func(key string) string { return env[key] }); err == nil {
			t.Fatal("expected isolated configuration rejection")
		}
	}
}
