package main

import (
	"testing"
	"time"
)

func TestDemoListenerRejectsNonLoopbackAndAmbiguousAddresses(t *testing.T) {
	invalid := []string{
		"0.0.0.0:18117",
		":18117",
		"localhost:18117",
		"[::]:18117",
		"[::1]:18117",
		"127.0.0.1:0",
		"127.0.0.1:-1",
		"127.0.0.1:65536",
		"127.0.0.2:18117",
		"127.0.1.1:18117",
		"0177.0.0.1:18117",
		"127.0.0.1",
		"127.0.0.1:port",
		"https://127.0.0.1:18117",
		"http://127.0.0.1:18117",
		"",
	}
	for _, addr := range invalid {
		if err := validateAddress(addr); err == nil {
			t.Errorf("unsafe address accepted: %q", addr)
		}
	}
	valid := []string{
		"127.0.0.1:18117",
		"127.0.0.1:1",
		"127.0.0.1:65535",
		"127.0.0.1:8080",
	}
	for _, addr := range valid {
		if err := validateAddress(addr); err != nil {
			t.Errorf("valid address rejected %q: %v", addr, err)
		}
	}
}

func TestSyntheticClockParsingValidation(t *testing.T) {
	validClocks := []string{
		"2030-01-15T12:00:00Z",
		"2026-11-14T10:30:00-03:00",
		"2026-03-01T00:00:00.123456789Z",
	}
	for _, s := range validClocks {
		val, err := time.Parse(time.RFC3339Nano, s)
		if err != nil || val.IsZero() {
			t.Errorf("expected valid synthetic clock %q: %v", s, err)
		}
	}

	invalidClocks := []string{
		"not-a-date",
		"2026-13-01T00:00:00Z",
		"0000-00-00T00:00:00Z",
		"2026-02-30T00:00:00Z",
	}
	for _, s := range invalidClocks {
		val, err := time.Parse(time.RFC3339Nano, s)
		if err == nil && !val.IsZero() {
			t.Errorf("expected invalid synthetic clock for %q", s)
		}
	}
}

func TestRunFlagAndArgumentValidation(t *testing.T) {
	// 1. Extra positional argument rejected
	if err := run([]string{"extra-arg"}); err == nil {
		t.Error("expected error for extra positional arguments, got nil")
	}

	// 2. Unsafe listening address rejected
	if err := run([]string{"--addr", "0.0.0.0:18117"}); err == nil {
		t.Error("expected error for 0.0.0.0 address, got nil")
	}

	// 3. Invalid synthetic clock rejected
	if err := run([]string{"--now", "not-rfc3339"}); err == nil {
		t.Error("expected error for invalid --now timestamp, got nil")
	}
}
