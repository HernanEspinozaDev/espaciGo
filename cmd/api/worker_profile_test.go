package main

import (
	"os"
	"testing"
)

func TestRunRejectsWorkerFreeProfileBeforeDatabaseConfig(t *testing.T) {
	t.Setenv("API_RUNTIME_PROFILE", "local-http-worker-free")
	t.Setenv("LOCAL_AUTH_PROTOTYPE", "")
	if err := run(); err == nil || err.Error() != "invalid API runtime profile" {
		t.Fatalf("run() error = %v, want invalid API runtime profile before database configuration", err)
	}
}

func TestRuntimeProfileDefaultsAndValidatesLocalWorkerFreeRequirements(t *testing.T) {
	profile, err := runtimeProfile(func(string) string { return "" })
	if err != nil || profile != runtimeProfileStandard {
		t.Fatalf("default profile = %q, %v", profile, err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	publicRoot := t.TempDir()
	if err := os.Chmod(publicRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	base := map[string]string{"API_RUNTIME_PROFILE": "local-http-worker-free", "LOCAL_AUTH_PROTOTYPE": "1", "DATABASE_CONNECTION_MODE": "direct", "DATABASE_HOST": "127.0.0.1", "HTTP_ADDR": "127.0.0.1:18082", "M03_EVIDENCE_DIR": root}
	for name, change := range map[string]func(map[string]string){
		"valid":                    func(map[string]string) {},
		"auth required":            func(v map[string]string) { delete(v, "LOCAL_AUTH_PROTOTYPE") },
		"no cloud app profile":     func(v map[string]string) { v["APP_PROFILE"] = "cloud-test" },
		"loopback HTTP only":       func(v map[string]string) { v["HTTP_ADDR"] = "0.0.0.0:8080" },
		"HTTP address required":    func(v map[string]string) { delete(v, "HTTP_ADDR") },
		"direct database required": func(v map[string]string) { v["DATABASE_CONNECTION_MODE"] = "cloudsql" },
		"loopback database only":   func(v map[string]string) { v["DATABASE_HOST"] = "192.0.2.8" },
		"filesystem only":          func(v map[string]string) { v["BLOB_STORE_BACKEND"] = "gcs" },
		"filesystem replay only":   func(v map[string]string) { v["LOCAL_PRIVACY_REPLAY_BACKEND"] = "gcs" },
		"absolute evidence root":   func(v map[string]string) { v["M03_EVIDENCE_DIR"] = "relative/path" },
		"private evidence root":    func(v map[string]string) { v["M03_EVIDENCE_DIR"] = publicRoot },
		"existing evidence root":   func(v map[string]string) { v["M03_EVIDENCE_DIR"] = "/path/that/does/not/exist" },
	} {
		t.Run(name, func(t *testing.T) {
			v := map[string]string{}
			for k, x := range base {
				v[k] = x
			}
			change(v)
			_, err := runtimeProfile(func(k string) string { return v[k] })
			if name == "valid" && err != nil {
				t.Fatal(err)
			}
			if name != "valid" && err == nil {
				t.Fatal("expected fail-closed rejection")
			}
		})
	}
}

func TestLaunchWorkersSkipsAllRegisteredCallbacksForWorkerFreeProfile(t *testing.T) {
	launched := 0
	workers := []namedWorker{{"alpha", func() { launched++ }}, {"beta", func() { launched++ }}}
	if err := launchWorkers(runtimeProfileWorkerFree, workers); err != nil {
		t.Fatal(err)
	}
	if launched != 0 {
		t.Fatalf("launched %d workers", launched)
	}
}
func TestLaunchWorkersRunsAllRegisteredCallbacksWhenEnabled(t *testing.T) {
	launched := 0
	workers := []namedWorker{{"alpha", func() { launched++ }}, {"beta", func() { launched++ }}}
	if err := launchWorkers(runtimeProfileStandard, workers); err != nil {
		t.Fatal(err)
	}
	if launched != len(workers) {
		t.Fatalf("launched %d of %d", launched, len(workers))
	}
}
func TestLaunchWorkersRejectsUnknownProfileBeforeLaunching(t *testing.T) {
	launched := 0
	workers := []namedWorker{{"alpha", func() { launched++ }}}
	if err := launchWorkers(runtimeProfileName("unknown"), workers); err == nil {
		t.Fatal("expected unknown profile rejection")
	}
	if launched != 0 {
		t.Fatalf("launched %d workers for invalid profile", launched)
	}
}
