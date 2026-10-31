package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentVerificationProfileRouteAllowlist(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := restrictDocumentVerificationRoutes(next)
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/health/live", http.StatusNoContent},
		{"GET", "/health/ready", http.StatusNoContent},
		{"GET", "/health/other", http.StatusNotFound},
		{"OPTIONS", "/health/ready", http.StatusNoContent},
		{"OPTIONS", "/health/live", http.StatusNotFound},
		{"GET", "/openapi.yaml", http.StatusNoContent},
		{"GET", "/api/v1/auth/terms", http.StatusNoContent},
		{"POST", "/api/v1/auth/login", http.StatusNoContent},
		{"GET", "/api/v1/auth/session", http.StatusNoContent},
		{"POST", "/api/v1/auth/logout", http.StatusNoContent},
		{"GET", "/api/v1/document-verification/cases", http.StatusNoContent},
		{"POST", "/api/v1/document-verification/cases", http.StatusNoContent},
		{"GET", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111", http.StatusNoContent},
		{"GET", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111/", http.StatusNotFound},
		{"POST", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111/submit", http.StatusNoContent},
		{"POST", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111/submit/extra", http.StatusNotFound},
		{"POST", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111/files", http.StatusNoContent},
		{"DELETE", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111", http.StatusNotFound},
		{"GET", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111/unknown", http.StatusNotFound},
		{"POST", "/api/v1/document-verification/cases/11111111-1111-4111-8111-111111111111/approve", http.StatusNotFound},
		{"GET", "/api/v1/document-verification/cases/123", http.StatusNotFound},
		{"POST", "/api/v1/auth/register", http.StatusNotFound},
		{"POST", "/api/v1/auth/password/reset", http.StatusNotFound},
		{"GET", "/api/v1/profile", http.StatusNotFound},
		{"GET", "/api/v1/admin/users", http.StatusNotFound},
		{"GET", "/api/v1/spaces", http.StatusNotFound},
		{"GET", "/api/v1/gallery", http.StatusNotFound},
		{"GET", "/api/v1/bookings", http.StatusNotFound},
		{"GET", "/api/v1/document-verification/cases-evil", http.StatusNotFound},
		{"PUT", "/api/v1/auth/login", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status=%d, want %d", res.Code, tc.want)
			}
		})
	}
}

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

func TestRuntimeProfilePinsDocumentScanOnlyToAuthorizedF5Runtime(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	keyDir := t.TempDir()
	if err := os.Chmod(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(keyDir, "document.key")
	if err := os.WriteFile(keyPath, []byte("synthetic-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := map[string]string{
		"API_RUNTIME_PROFILE":                            "local-document-scan-only",
		"LOCAL_AUTH_PROTOTYPE":                           "1",
		"DATABASE_CONNECTION_MODE":                       "direct",
		"DATABASE_HOST":                                  "127.0.0.1",
		"DATABASE_PORT":                                  "15433",
		"DATABASE_NAME":                                  "espacigo_f5_synth_20261029",
		"DATABASE_USER":                                  "espacigo_api_login",
		"DATABASE_SET_ROLE":                              "espacigo_runtime",
		"HTTP_ADDR":                                      "127.0.0.1:18084",
		"M03_EVIDENCE_DIR":                               root,
		"DOCUMENT_VERIFICATION_STORAGE_ROOT":             "/var/lib/espacigo-private-documents",
		"DOCUMENT_VERIFICATION_ENCRYPTION_KEY_FILE":      keyPath,
		"DOCUMENT_VERIFICATION_CLAMAV_ADDRESS":           "127.0.0.1:3310",
		"DOCUMENT_VERIFICATION_CLAMAV_MAX_SIGNATURE_AGE": "48h",
	}
	profile, err := runtimeProfile(func(key string) string { return base[key] })
	if err != nil || profile != runtimeProfileDocumentScanOnly {
		t.Fatalf("document scan profile=%q err=%v", profile, err)
	}
	for name, change := range map[string]func(map[string]string){
		"wrong database port": func(v map[string]string) { v["DATABASE_PORT"] = "5432" },
		"wrong database":      func(v map[string]string) { v["DATABASE_NAME"] = "espacigo_dms_stage_20261020" },
		"wrong runtime role":  func(v map[string]string) { v["DATABASE_SET_ROLE"] = "postgres" },
		"missing scanner":     func(v map[string]string) { delete(v, "DOCUMENT_VERIFICATION_CLAMAV_ADDRESS") },
		"missing storage key": func(v map[string]string) { delete(v, "DOCUMENT_VERIFICATION_ENCRYPTION_KEY_FILE") },
		"overlapping roots":   func(v map[string]string) { v["DOCUMENT_VERIFICATION_STORAGE_ROOT"] = root + "/private-documents" },
	} {
		t.Run(name, func(t *testing.T) {
			values := map[string]string{}
			for key, value := range base {
				values[key] = value
			}
			change(values)
			if _, err := runtimeProfile(func(key string) string { return values[key] }); err == nil {
				t.Fatal("expected fail-closed profile rejection")
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

func TestLaunchDocumentScanOnlyProfileLaunchesScannerAndRetentionOnly(t *testing.T) {
	launched := []string{}
	workers := []namedWorker{
		{"document-verification-scan", func() { launched = append(launched, "document-verification-scan") }},
		{"document-verification-retention", func() { launched = append(launched, "document-verification-retention") }},
		{"gallery-retention", func() { launched = append(launched, "gallery-retention") }},
	}
	if err := launchWorkers(runtimeProfileName("local-document-scan-only"), workers); err != nil {
		t.Fatal(err)
	}
	if len(launched) != 2 || launched[0] != "document-verification-scan" || launched[1] != "document-verification-retention" {
		t.Fatalf("launched workers=%v, want document-verification-scan and document-verification-retention only", launched)
	}
}

func TestValidateWorkerRegistryKeepsDocumentWorkersIsolatedAndUnique(t *testing.T) {
	scan := namedWorker{"document-verification-scan", func() {}}
	retention := namedWorker{"document-verification-retention", func() {}}
	for _, tc := range []struct {
		name    string
		profile runtimeProfileName
		workers []namedWorker
		wantErr bool
	}{
		{name: "valid document profile", profile: runtimeProfileDocumentScanOnly, workers: []namedWorker{scan, retention}},
		{name: "missing scan worker", profile: runtimeProfileDocumentScanOnly, workers: []namedWorker{retention}, wantErr: true},
		{name: "missing retention worker", profile: runtimeProfileDocumentScanOnly, workers: []namedWorker{scan}, wantErr: true},
		{name: "duplicate scan worker", profile: runtimeProfileDocumentScanOnly, workers: []namedWorker{scan, scan, retention}, wantErr: true},
		{name: "duplicate retention worker", profile: runtimeProfileDocumentScanOnly, workers: []namedWorker{scan, retention, retention}, wantErr: true},
		{name: "standard profile", profile: runtimeProfileStandard, workers: []namedWorker{scan, retention}, wantErr: true},
		{name: "worker-free profile", profile: runtimeProfileWorkerFree, workers: []namedWorker{scan, retention}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWorkerRegistry(tc.profile, tc.workers)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateWorkerRegistry() error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
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
