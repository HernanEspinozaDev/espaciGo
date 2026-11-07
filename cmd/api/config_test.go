package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIsolatedDatasetClockRequiresTheRunScopedManualClock(t *testing.T) {
	lookup := map[string]string{}
	getenv := func(key string) string { return lookup[key] }
	if enabled, err := isolatedDatasetClockFromEnv(getenv); err != nil || enabled {
		t.Fatalf("default isolated clock mode enabled=%v err=%v", enabled, err)
	}
	lookup["LOCAL_DATASET_DEALS_ISOLATED_CLOCK"] = "1"
	if _, err := isolatedDatasetClockFromEnv(getenv); err == nil {
		t.Fatal("isolated dataset clock accepted without a local trial clock")
	}
	lookup["LOCAL_BOOKING_TRIAL"] = "1"
	lookup["LOCAL_DEMO_BUSINESS_CLOCK_FILE"] = "/tmp/shared-clock.json"
	if _, err := isolatedDatasetClockFromEnv(getenv); err == nil {
		t.Fatal("isolated dataset clock accepted a non-run-scoped clock file")
	}
	lookup["LOCAL_DEMO_BUSINESS_CLOCK_FILE"] = "/run/fe14-demo/business-clock.json"
	if enabled, err := isolatedDatasetClockFromEnv(getenv); err != nil || !enabled {
		t.Fatalf("run-scoped isolated clock enabled=%v err=%v", enabled, err)
	}
}

func TestLocalProfilesAcceptPrivatePGPassfileWithoutEmbeddingPassword(t *testing.T) {
	for _, profile := range []string{string(runtimeProfileWorkerFree), string(runtimeProfileDocumentScanOnly)} {
		t.Run(profile, func(t *testing.T) {
			dir := privatePassfileDir(t)
			passfile := filepath.Join(dir, "pgpass")
			if err := os.WriteFile(passfile, []byte("localhost:5432:test:login:synthetic\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validatePrivatePGPassfile(passfile); err != nil {
				t.Fatalf("validate private passfile: %v", err)
			}
			values := map[string]string{"HTTP_ADDR": "127.0.0.1:18082", "API_RUNTIME_PROFILE": profile, "DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login", "DATABASE_SET_ROLE": "espacigo_runtime", "PGPASSFILE": passfile}
			cfg, err := loadRuntimeConfig(func(k string) string { return values[k] }, runtimeProfileName(profile))
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			t.Cleanup(cfg.close)
			parsed, err := url.Parse(cfg.databaseURL())
			if err != nil {
				t.Fatal(err)
			}
			if _, hasPassword := parsed.User.Password(); hasPassword {
				t.Fatal("DSN embeds password")
			}
			if cfg.dbSetRole != "espacigo_runtime" {
				t.Fatalf("SET ROLE=%q", cfg.dbSetRole)
			}
		})
	}
}

func TestWizardLocalProfileAcceptsPrivatePGPassfile(t *testing.T) {
	dir := privatePassfileDir(t)
	passfile := filepath.Join(dir, "pgpass")
	if err := os.WriteFile(passfile, []byte("127.0.0.1:15434:test:login:synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1",
		"DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login",
		"DATABASE_SET_ROLE": "espacigo_runtime", "PGPASSFILE": passfile,
	}
	getenv := func(key string) string { return values[key] }

	cfg, err := loadRuntimeConfig(getenv, runtimeProfileWizardLocal)
	if err != nil {
		t.Fatalf("wizard-local profile rejected private PGPASSFILE: %v", err)
	}
	t.Cleanup(cfg.close)
	parsed, err := url.Parse(cfg.databaseURL())
	if err != nil {
		t.Fatal(err)
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		t.Fatal("DSN embeds password")
	}
	if cfg.dbSetRole != "espacigo_runtime" {
		t.Fatalf("SET ROLE=%q", cfg.dbSetRole)
	}

	if _, err := loadRuntimeConfig(getenv, runtimeProfileStandard); err == nil {
		t.Fatal("standard profile accepted PGPASSFILE")
	}
	values["DATABASE_CONNECTION_MODE"] = "cloudsql"
	values["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] = "test-project:test-region:test-instance"
	if _, err := loadRuntimeConfig(getenv, runtimeProfileWizardLocal); err == nil {
		t.Fatal("wizard-local profile accepted PGPASSFILE with Cloud SQL mode")
	}
	delete(values, "DATABASE_CONNECTION_MODE")
	delete(values, "CLOUD_SQL_INSTANCE_CONNECTION_NAME")
	passwordFile := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-password"), 0o600); err != nil {
		t.Fatal(err)
	}
	values["DATABASE_PASSWORD_FILE"] = passwordFile
	if _, err := loadRuntimeConfig(getenv, runtimeProfileWizardLocal); err == nil {
		t.Fatal("wizard-local profile accepted competing password sources")
	}
}

func TestLocalProfilesBindPassfileToValidatedFileDescriptor(t *testing.T) {
	for _, profile := range []runtimeProfileName{runtimeProfileWorkerFree, runtimeProfileDocumentScanOnly} {
		t.Run(string(profile), func(t *testing.T) {
			dir := privatePassfileDir(t)
			passfile := filepath.Join(dir, "pgpass")
			const validatedPassword = "synthetic-original"
			if err := os.WriteFile(passfile, []byte("127.0.0.1:15434:test:login:"+validatedPassword+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PGPASSFILE", passfile)
			values := map[string]string{
				"HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1",
				"DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login",
				"DATABASE_SET_ROLE": "espacigo_runtime", "PGPASSFILE": passfile,
			}
			cfg, err := loadRuntimeConfig(func(key string) string { return values[key] }, profile)
			if err != nil {
				t.Fatalf("load runtime config: %v", err)
			}
			t.Cleanup(cfg.close)
			if err := os.Rename(passfile, filepath.Join(dir, "validated-pgpass")); err != nil {
				t.Fatal("replace validated passfile path")
			}
			if err := os.WriteFile(passfile, []byte("127.0.0.1:15434:test:login:synthetic-replacement\n"), 0o600); err != nil {
				t.Fatal("create replacement passfile")
			}
			pool, closeConnector, err := newDatabasePool(context.Background(), cfg)
			if err != nil {
				t.Fatalf("parse local runtime database config: %v", err)
			}
			defer pool.Close()
			defer closeConnector()
			if pool.Config().ConnConfig.Password != validatedPassword {
				t.Fatal("database config did not consume the validated passfile descriptor")
			}
		})
	}
}

func TestDocumentScanOnlyRejectsSymlinkedPGPassfileParent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "private")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(target, "pgpass")
	if err := os.WriteFile(passfile, []byte("localhost:5432:test:login:synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked-private")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login", "DATABASE_SET_ROLE": "espacigo_runtime", "PGPASSFILE": filepath.Join(link, "pgpass")}
	if _, err := loadRuntimeConfig(func(k string) string { return values[k] }, runtimeProfileDocumentScanOnly); err == nil {
		t.Fatal("document-scan-only profile accepted PGPASSFILE with symlinked parent")
	}
}

func TestDocumentScanOnlyRejectsSymlinkedPGPassfileAncestor(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	privateDir := filepath.Join(target, "private")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(privateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(privateDir, "pgpass")
	if err := os.WriteFile(passfile, []byte("localhost:5432:test:login:synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked-root")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login", "DATABASE_SET_ROLE": "espacigo_runtime", "PGPASSFILE": filepath.Join(link, "private", "pgpass")}
	if _, err := loadRuntimeConfig(func(k string) string { return values[k] }, runtimeProfileDocumentScanOnly); err == nil {
		t.Fatal("document-scan-only profile accepted PGPASSFILE with symlinked ancestor")
	}
}

func TestStandardProfileStillRequiresDatabasePasswordFile(t *testing.T) {
	dir := t.TempDir()
	passfile := filepath.Join(dir, "pgpass")
	if err := os.WriteFile(passfile, []byte("localhost:5432:test:login:synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"DATABASE_HOST": "localhost", "DATABASE_NAME": "test", "DATABASE_USER": "login", "PGPASSFILE": passfile}
	if _, err := loadRuntimeConfig(func(k string) string { return values[k] }, runtimeProfileStandard); err == nil {
		t.Fatal("standard profile accepted PGPASSFILE without DATABASE_PASSWORD_FILE")
	}
}

func TestLocalProfileRejectsCompetingPasswordSources(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(dir, "pgpass")
	if err := os.WriteFile(passfile, []byte("localhost:5432:test:login:synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	passwordFile := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-password"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login", "PGPASSFILE": passfile, "DATABASE_PASSWORD_FILE": passwordFile}
	if _, err := loadRuntimeConfig(func(k string) string { return values[k] }, runtimeProfileWorkerFree); err == nil {
		t.Fatal("local profile accepted both PGPASSFILE and DATABASE_PASSWORD_FILE")
	}
}

func TestWorkerFreeRejectsUnsafePGPassfilePaths(t *testing.T) {
	cases := []struct {
		name string
		path func(*testing.T) string
	}{
		{name: "whitespace only", path: func(*testing.T) string { return " \t " }},
		{name: "missing file", path: func(t *testing.T) string {
			return filepath.Join(privatePassfileDir(t), "missing")
		}},
		{name: "empty file", path: func(t *testing.T) string {
			path := filepath.Join(privatePassfileDir(t), "empty")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "symlinked file", path: func(t *testing.T) string {
			dir := privatePassfileDir(t)
			target := filepath.Join(dir, "target")
			writePrivatePassfile(t, target, 0o600)
			link := filepath.Join(dir, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			return link
		}},
		{name: "non-regular file", path: func(t *testing.T) string {
			dir := privatePassfileDir(t)
			path := filepath.Join(dir, "directory")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "wrong file mode", path: func(t *testing.T) string {
			dir := privatePassfileDir(t)
			path := filepath.Join(dir, "pgpass")
			writePrivatePassfile(t, path, 0o644)
			return path
		}},
		{name: "wrong immediate parent mode", path: func(t *testing.T) string {
			dir := filepath.Join(privatePassfileDir(t), "shared")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "pgpass")
			writePrivatePassfile(t, path, 0o600)
			return path
		}},
		{name: "group/world writable ancestor", path: func(t *testing.T) string {
			unsafe := filepath.Join(privatePassfileDir(t), "unsafe")
			if err := os.Mkdir(unsafe, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(unsafe, 0o777); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(unsafe, "private")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "pgpass")
			writePrivatePassfile(t, path, 0o600)
			return path
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := loadWorkerFreePassfileConfig(tc.path(t)); err == nil {
				t.Fatal("worker-free profile accepted unsafe PGPASSFILE path")
			}
		})
	}
}

func TestStandardAndCloudConnectionModesRejectPGPassfile(t *testing.T) {
	dir := privatePassfileDir(t)
	passfile := filepath.Join(dir, "pgpass")
	writePrivatePassfile(t, passfile, 0o600)
	passwordFile := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, connectionMode := range []string{"direct", "cloudsql"} {
		t.Run(connectionMode, func(t *testing.T) {
			values := map[string]string{
				"DATABASE_CONNECTION_MODE": connectionMode,
				"DATABASE_HOST":            "127.0.0.1",
				"DATABASE_NAME":            "test",
				"DATABASE_USER":            "login",
				"DATABASE_PASSWORD_FILE":   passwordFile,
				"PGPASSFILE":               passfile,
			}
			if connectionMode == "cloudsql" {
				values["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] = "test-project:test-region:test-instance"
			}
			_, err := loadRuntimeConfig(func(key string) string { return values[key] }, runtimeProfileStandard)
			if err == nil {
				t.Fatal("standard runtime profile accepted PGPASSFILE")
			}
			if strings.Contains(err.Error(), passfile) {
				t.Fatal("PGPASSFILE path leaked in configuration error")
			}
		})
	}
}

func TestLocalPassfileProfilesRejectCloudSQLMode(t *testing.T) {
	passfile := filepath.Join(privatePassfileDir(t), "pgpass")
	writePrivatePassfile(t, passfile, 0o600)
	for _, profile := range []runtimeProfileName{runtimeProfileWorkerFree, runtimeProfileDocumentScanOnly} {
		t.Run(string(profile), func(t *testing.T) {
			values := map[string]string{
				"HTTP_ADDR":                          "127.0.0.1:18082",
				"DATABASE_CONNECTION_MODE":           "cloudsql",
				"CLOUD_SQL_INSTANCE_CONNECTION_NAME": "test-project:test-region:test-instance",
				"DATABASE_NAME":                      "test",
				"DATABASE_USER":                      "login",
				"DATABASE_SET_ROLE":                  "espacigo_runtime",
				"PGPASSFILE":                         passfile,
			}
			_, err := loadRuntimeConfig(func(key string) string { return values[key] }, profile)
			if err == nil {
				t.Fatal("local profile accepted PGPASSFILE with Cloud SQL connection mode")
			}
			if strings.Contains(err.Error(), passfile) {
				t.Fatal("PGPASSFILE path leaked in configuration error")
			}
		})
	}
}

func privatePassfileDir(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "pgpass-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove temporary passfile directory: %v", err)
		}
	})
	dir := filepath.Join(root, "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writePrivatePassfile(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte("localhost:5432:test:login:synthetic\n"), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func loadWorkerFreePassfileConfig(path string) error {
	values := map[string]string{
		"HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1",
		"DATABASE_PORT": "15434", "DATABASE_NAME": "test", "DATABASE_USER": "login",
		"PGPASSFILE": path,
	}
	_, err := loadRuntimeConfig(func(key string) string { return values[key] }, runtimeProfileWorkerFree)
	return err
}

func TestLoadConfigReadsLocalPasswordFileAndParsesOrigins(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"HTTP_ADDR":              ":8080",
		"DATABASE_HOST":          "database",
		"DATABASE_PORT":          "5432",
		"DATABASE_NAME":          "espacigo_test",
		"DATABASE_USER":          "espacigo_runtime",
		"DATABASE_PASSWORD_FILE": passwordFile,
		"CORS_ALLOWED_ORIGINS":   "http://localhost:4200, http://127.0.0.1:4200",
		"FRONTEND_PUBLIC_ORIGIN": "http://127.0.0.1:4321",
	}

	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.dbPassword != "synthetic-test-value" {
		t.Fatalf("password file was not trimmed/read correctly")
	}
	wantOrigins := []string{"http://localhost:4200", "http://127.0.0.1:4200"}
	if !reflect.DeepEqual(cfg.allowedOrigins, wantOrigins) {
		t.Fatalf("allowed origins = %#v, want %#v", cfg.allowedOrigins, wantOrigins)
	}
	if cfg.dbSSLMode != "disable" {
		t.Fatalf("default sslmode=%q, want disable", cfg.dbSSLMode)
	}
	if cfg.frontendPublicOrigin != "http://127.0.0.1:4321" {
		t.Fatalf("frontend origin=%q, want configured origin", cfg.frontendPublicOrigin)
	}
}

func TestLoadConfigValidatesDatabaseSetRole(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"HTTP_ADDR": ":8080", "DATABASE_HOST": "database", "DATABASE_NAME": "espacigo_test",
		"DATABASE_USER": "api_login", "DATABASE_PASSWORD_FILE": passwordFile,
		"DATABASE_SET_ROLE": "espacigo_runtime",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil || cfg.dbSetRole != "espacigo_runtime" {
		t.Fatalf("set role = %q, err=%v", cfg.dbSetRole, err)
	}
	for _, role := range []string{"bad role", `x; RESET ROLE`, strings.Repeat("a", 64), "9starts_with_digit"} {
		values["DATABASE_SET_ROLE"] = role
		if _, err := loadConfig(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("accepted invalid set role %q", role)
		}
	}
}

func TestLoadConfigUsesCloudRunPortWhenHTTPAddrIsUnset(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"PORT": "9090", "DATABASE_HOST": "database", "DATABASE_NAME": "espacigo_test",
		"DATABASE_USER": "espacigo_runtime", "DATABASE_PASSWORD_FILE": passwordFile,
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.httpAddr != ":9090" {
		t.Fatalf("HTTP address = %q, want :9090", cfg.httpAddr)
	}
}

func TestLoadConfigCloudSQLUsesConnectorAndBoundedPoolSettings(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_CONNECTION_MODE":           "cloudsql",
		"CLOUD_SQL_INSTANCE_CONNECTION_NAME": "espacigo-testprod:southamerica-west1:espacigo-db",
		"CLOUD_SQL_PRIVATE_IP":               "true",
		"DATABASE_NAME":                      "espacigo_local",
		"DATABASE_USER":                      "espacigo_runtime",
		"DATABASE_PASSWORD_FILE":             passwordFile,
		"DATABASE_POOL_MAX_CONNS":            "24",
		"DATABASE_POOL_MIN_CONNS":            "2",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.dbHost != "localhost" || cfg.cloudSQLInstance != values["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] || !cfg.cloudSQLPrivateIP {
		t.Fatalf("Cloud SQL config = %#v", cfg)
	}
	if cfg.dbPoolMaxConns != 24 || cfg.dbPoolMinConns != 2 {
		t.Fatalf("pool bounds max=%d min=%d", cfg.dbPoolMaxConns, cfg.dbPoolMinConns)
	}
	parsed, err := url.Parse(cfg.databaseURL())
	if err != nil || parsed.Host != "localhost:5432" || parsed.Query().Get("sslmode") != "disable" {
		t.Fatalf("connector DSN = %v, %v", parsed, err)
	}
}

func TestLoadConfigRejectsUnsafeCloudSQLSettings(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := map[string]string{
		"DATABASE_CONNECTION_MODE":           "cloudsql",
		"CLOUD_SQL_INSTANCE_CONNECTION_NAME": "espacigo-testprod:southamerica-west1:espacigo-db",
		"DATABASE_NAME":                      "espacigo_local",
		"DATABASE_USER":                      "espacigo_runtime",
		"DATABASE_PASSWORD_FILE":             passwordFile,
	}
	for _, test := range []struct {
		name   string
		values map[string]string
	}{
		{name: "missing instance", values: map[string]string{"CLOUD_SQL_INSTANCE_CONNECTION_NAME": ""}},
		{name: "direct TLS mixed with connector", values: map[string]string{"DATABASE_SSLMODE": "verify-full"}},
		{name: "invalid IP mode", values: map[string]string{"CLOUD_SQL_PRIVATE_IP": "maybe"}},
		{name: "unbounded pool", values: map[string]string{"DATABASE_POOL_MAX_CONNS": "51"}},
		{name: "min above max", values: map[string]string{"DATABASE_POOL_MAX_CONNS": "2", "DATABASE_POOL_MIN_CONNS": "3"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{}
			for key, value := range base {
				values[key] = value
			}
			for key, value := range test.values {
				values[key] = value
			}
			if _, err := loadConfig(func(key string) string { return values[key] }); err == nil {
				t.Fatal("loadConfig accepted unsafe Cloud SQL settings")
			}
		})
	}
}

func TestLoadConfigRejectsInvalidFrontendPublicOrigin(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_HOST": "database", "DATABASE_NAME": "espacigo_test",
		"DATABASE_USER": "espacigo_runtime", "DATABASE_PASSWORD_FILE": passwordFile,
		"FRONTEND_PUBLIC_ORIGIN": "https://frontend.example.invalid/path",
	}
	if _, err := loadConfig(func(key string) string { return values[key] }); err == nil {
		t.Fatal("loadConfig accepted a frontend URL with a path")
	}
}

func TestLoadConfigParametrizesPostgreSQLTLS(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_HOST":          "db.example.invalid",
		"DATABASE_PORT":          "6543",
		"DATABASE_NAME":          "isolated_test",
		"DATABASE_USER":          "runtime_test",
		"DATABASE_PASSWORD_FILE": passwordFile,
		"DATABASE_SSLMODE":       "verify-full",
		"DATABASE_SSLROOTCERT":   "/run/database-tls/root.pem",
		"DATABASE_SSLCERT":       "/run/database-tls/client.pem",
		"DATABASE_SSLKEY":        "/run/database-tls/client.key",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(cfg.databaseURL())
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"sslmode": "verify-full", "sslrootcert": "/run/database-tls/root.pem",
		"sslcert": "/run/database-tls/client.pem", "sslkey": "/run/database-tls/client.key",
	} {
		if got := parsed.Query().Get(key); got != want {
			t.Fatalf("database URL %s=%q, want %q", key, got, want)
		}
	}
	if password, ok := parsed.User.Password(); !ok || password != "synthetic-test-value" {
		t.Fatal("database URL did not preserve password-file authentication")
	}
}

func TestLoadConfigRejectsInvalidPostgreSQLTLS(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := map[string]string{
		"DATABASE_HOST": "database", "DATABASE_NAME": "espacigo_test",
		"DATABASE_USER": "espacigo_runtime", "DATABASE_PASSWORD_FILE": passwordFile,
	}
	for _, test := range []struct {
		name   string
		values map[string]string
	}{
		{name: "unknown mode", values: map[string]string{"DATABASE_SSLMODE": "accept-anything"}},
		{name: "client certificate without key", values: map[string]string{"DATABASE_SSLCERT": "/run/client.pem"}},
		{name: "client key without certificate", values: map[string]string{"DATABASE_SSLKEY": "/run/client.key"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{}
			for key, value := range base {
				values[key] = value
			}
			for key, value := range test.values {
				values[key] = value
			}
			if _, err := loadConfig(func(key string) string { return values[key] }); err == nil {
				t.Fatal("loadConfig accepted invalid TLS configuration")
			}
		})
	}
}

func TestLoadConfigRejectsMissingDatabaseHost(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_PORT":          "5432",
		"DATABASE_NAME":          "espacigo_test",
		"DATABASE_USER":          "espacigo_runtime",
		"DATABASE_PASSWORD_FILE": passwordFile,
	}

	_, err := loadConfig(func(key string) string { return values[key] })
	if err == nil {
		t.Fatal("loadConfig succeeded without DATABASE_HOST")
	}
	if strings.Contains(err.Error(), "synthetic-test-value") {
		t.Fatalf("configuration error leaked the test value: %q", err)
	}
}

func TestLoadConfigRejectsEmptyPasswordFile(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_HOST":          "database",
		"DATABASE_PORT":          "5432",
		"DATABASE_NAME":          "espacigo_test",
		"DATABASE_USER":          "espacigo_runtime",
		"DATABASE_PASSWORD_FILE": passwordFile,
	}

	_, err := loadConfig(func(key string) string { return values[key] })
	if err == nil {
		t.Fatal("loadConfig succeeded with an empty password file")
	}
}

func TestLoadConfigRejectsInvalidDatabasePort(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "runtime-password")
	if err := os.WriteFile(passwordFile, []byte("synthetic-test-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_HOST":          "database",
		"DATABASE_PORT":          "70000",
		"DATABASE_NAME":          "espacigo_test",
		"DATABASE_USER":          "espacigo_runtime",
		"DATABASE_PASSWORD_FILE": passwordFile,
	}

	_, err := loadConfig(func(key string) string { return values[key] })
	if err == nil {
		t.Fatal("loadConfig succeeded with an out-of-range database port")
	}
}
