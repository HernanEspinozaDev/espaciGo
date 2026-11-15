package main

import "testing"

func TestRetiredManualDiditRuntimeProfileIsRejected(t *testing.T) {
	values := manualProfileEnv()
	values["API_RUNTIME_PROFILE"] = "local-kyc-didit-sandbox-manual"
	if profile, err := runtimeProfile(func(key string) string { return values[key] }); err == nil {
		t.Fatalf("retired profile resolved to %q", profile)
	}
}

func TestRetiredManualDiditProfileCannotStartRuntimeOrWorkers(t *testing.T) {
	profile := runtimeProfileName("local-kyc-didit-sandbox-manual")
	if usesDedicatedStartup(profile) {
		t.Fatal("retired profile still selects dedicated KYC startup")
	}
	if err := validateWorkerRegistry(profile, nil); err == nil {
		t.Fatal("retired profile still has a valid worker registry")
	}
	if err := runLocalKYCOnly(config{}, profile); err == nil || err.Error() != "invalid local KYC runtime profile" {
		t.Fatalf("runLocalKYCOnly error = %v, want rejection before database startup", err)
	}
}
