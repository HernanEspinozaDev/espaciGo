package main

import (
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
