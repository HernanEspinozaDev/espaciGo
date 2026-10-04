package main

import (
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
		"HTTP_ADDR":                 ":8080",
		"DATABASE_HOST":             "database",
		"DATABASE_PORT":             "5432",
		"DATABASE_NAME":             "espacigo_test",
		"DATABASE_USER":             "espacigo_runtime",
		"DATABASE_PASSWORD_FILE":    passwordFile,
		"CORS_ALLOWED_ORIGINS":      "http://localhost:8081, http://127.0.0.1:8081",
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
