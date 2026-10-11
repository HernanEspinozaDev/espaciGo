package main

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
		"CORS_ALLOWED_ORIGINS":   "http://localhost:8081, http://127.0.0.1:8081",
	}

	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.dbPassword != "synthetic-test-value" {
		t.Fatalf("password file was not trimmed/read correctly")
	}
	wantOrigins := []string{"http://localhost:8081", "http://127.0.0.1:8081"}
	if !reflect.DeepEqual(cfg.allowedOrigins, wantOrigins) {
		t.Fatalf("allowed origins = %#v, want %#v", cfg.allowedOrigins, wantOrigins)
	}
	if cfg.dbSSLMode != "disable" {
		t.Fatalf("default sslmode=%q, want disable", cfg.dbSSLMode)
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
