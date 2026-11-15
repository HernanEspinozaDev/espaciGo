package main

import "testing"

func TestManualSandboxStatusInspectionCannotStartAfterRetirement(t *testing.T) {
	values := manualProfileEnv()
	values["API_RUNTIME_PROFILE"] = "local-kyc-didit-sandbox-manual"
	if profile, err := runtimeProfile(func(key string) string { return values[key] }); err == nil {
		t.Fatalf("status inspection profile resolved to %q", profile)
	}
}
