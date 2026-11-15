package main

import "testing"

func TestManualSandboxRuntimePreflightRejectsRetiredProfile(t *testing.T) {
	t.Setenv("API_RUNTIME_PROFILE", "local-kyc-didit-sandbox-manual")
	if err := run(); err == nil || err.Error() != "invalid API runtime profile" {
		t.Fatalf("run() error = %v, want retired-profile rejection before database startup", err)
	}
}
