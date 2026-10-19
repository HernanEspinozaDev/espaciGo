package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConnectionSettingsPreservesLocalDatabaseURLMode(t *testing.T) {
	values := map[string]string{"DATABASE_URL": "postgres://local:pass@127.0.0.1:5432/localdb"}
	settings, err := loadConnectionSettings(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if settings.mode != "direct" || settings.databaseURL != values["DATABASE_URL"] {
		t.Fatalf("local settings not preserved: %#v", settings)
	}
	if settings.timeout != 10*time.Minute {
		t.Fatalf("default timeout = %s, want 10m", settings.timeout)
	}
}

func TestLoadConnectionSettingsCloudSQLPrivateMode(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":                       "postgres://migrator@localhost:5432/app?sslmode=disable",
		"DATABASE_CONNECTION_MODE":           "cloudsql",
		"CLOUD_SQL_INSTANCE_CONNECTION_NAME": "project-one:southamerica-west1:db-one",
		"CLOUD_SQL_PRIVATE_IP":               "true",
		"DATABASE_PASSWORD_FILE":             "/run/secrets/database-migrator-password/password",
		"DATABASE_SET_ROLE":                  "espacigo_schema_owner",
		"DBMIGRATE_TIMEOUT":                  "4h",
	}
	settings, err := loadConnectionSettings(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if settings.mode != "cloudsql" || settings.instance != values["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] || !settings.privateIP {
		t.Fatalf("Cloud SQL settings not parsed: %#v", settings)
	}
	if settings.timeout != 4*time.Hour {
		t.Fatalf("timeout = %s, want 4h", settings.timeout)
	}
	if settings.passwordFile != values["DATABASE_PASSWORD_FILE"] {
		t.Fatal("password file path not preserved")
	}
	if settings.setRole != "espacigo_schema_owner" {
		t.Fatalf("set role = %q", settings.setRole)
	}
}

func TestLoadConnectionSettingsRejectsUnsafeSetRole(t *testing.T) {
	for _, value := range []string{"role name", `x; RESET ROLE`, "9bad", strings.Repeat("a", 64)} {
		values := map[string]string{
			"DATABASE_URL": "postgres://migrator@localhost/app", "DATABASE_SET_ROLE": value,
		}
		if _, err := loadConnectionSettings(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("accepted unsafe DATABASE_SET_ROLE %q", value)
		}
	}
}

func TestDatabaseURLAndPasswordFileKeepsSecretOutOfURLAndErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("private-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := connectionSettings{
		databaseURL:  "postgres://migrator@localhost:5432/app?sslmode=disable",
		passwordFile: path,
	}
	gotURL, gotPassword, err := settings.databaseURLAndPasswordFile()
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != settings.databaseURL || gotPassword != "private-pass" {
		t.Fatal("password should be returned separately from the connection URL")
	}
	settings.passwordFile = filepath.Join(t.TempDir(), "missing")
	_, _, err = settings.databaseURLAndPasswordFile()
	if err == nil || strings.Contains(err.Error(), "private-pass") || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("password file error was not sanitized: %v", err)
	}
}

func TestLoadConnectionSettingsRejectsInvalidCloudSQLSettingsWithoutLeakingURL(t *testing.T) {
	for _, values := range []map[string]string{
		{"DATABASE_URL": "postgres://u:secret@localhost/db", "DATABASE_CONNECTION_MODE": "other"},
		{"DATABASE_URL": "postgres://u@localhost/app?sslmode=disable", "DATABASE_CONNECTION_MODE": "cloudsql"},
		{"DATABASE_URL": "postgres://u@localhost/app?sslmode=disable", "DATABASE_CONNECTION_MODE": "cloudsql", "CLOUD_SQL_INSTANCE_CONNECTION_NAME": "project-one:region-one:instance-one", "DATABASE_PASSWORD_FILE": "/secret", "CLOUD_SQL_PRIVATE_IP": "perhaps"},
		{"DATABASE_URL": "postgres://u@localhost/app?sslmode=disable", "DATABASE_CONNECTION_MODE": "cloudsql", "CLOUD_SQL_INSTANCE_CONNECTION_NAME": "project-one:region-one:instance-one", "DATABASE_PASSWORD_FILE": "/secret", "PGSERVICE": "override"},
		{"DATABASE_URL": "postgres://u@localhost/app?sslmode=disable&password=secret", "DATABASE_CONNECTION_MODE": "cloudsql", "CLOUD_SQL_INSTANCE_CONNECTION_NAME": "project-one:region-one:instance-one", "DATABASE_PASSWORD_FILE": "/secret"},
		{"DATABASE_URL": "postgres://u@localhost/app?sslmode=disable", "DBMIGRATE_TIMEOUT": "5h"},
	} {
		_, err := loadConnectionSettings(func(key string) string { return values[key] })
		if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "postgres://") {
			t.Fatalf("expected sanitized configuration error, got %v", err)
		}
	}
}
