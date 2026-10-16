package main

import (
	"net/url"
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
}

func TestDatabaseURLWithPasswordFileInjectsSecretWithoutExposingItOnErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("private-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := connectionSettings{
		databaseURL:  "postgres://migrator@localhost:5432/app?sslmode=disable",
		passwordFile: path,
	}
	got, err := settings.databaseURLWithPasswordFile()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal("constructed URL is invalid")
	}
	password, ok := parsed.User.Password()
	if !ok || password != "private-pass" {
		t.Fatal("password file value was not applied to connection URL")
	}
	settings.passwordFile = filepath.Join(t.TempDir(), "missing")
	_, err = settings.databaseURLWithPasswordFile()
	if err == nil || strings.Contains(err.Error(), "private-pass") || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("password file error was not sanitized: %v", err)
	}
	settings.databaseURL = "postgres://migrator:inline-secret@localhost:5432/app?sslmode=disable"
	_, err = settings.databaseURLWithPasswordFile()
	if err == nil || strings.Contains(err.Error(), "inline-secret") {
		t.Fatalf("inline password rejection was not sanitized: %v", err)
	}
}

func TestLoadConnectionSettingsRejectsInvalidCloudSQLSettingsWithoutLeakingURL(t *testing.T) {
	for _, values := range []map[string]string{
		{"DATABASE_URL": "postgres://u:secret@localhost/db", "DATABASE_CONNECTION_MODE": "other"},
		{"DATABASE_URL": "postgres://u:secret@localhost/db", "DATABASE_CONNECTION_MODE": "cloudsql"},
		{"DATABASE_URL": "postgres://u:secret@localhost/db", "DATABASE_CONNECTION_MODE": "cloudsql", "CLOUD_SQL_INSTANCE_CONNECTION_NAME": "project-one:region-one:instance-one", "DATABASE_PASSWORD_FILE": "/secret", "CLOUD_SQL_PRIVATE_IP": "perhaps"},
		{"DATABASE_URL": "postgres://u:secret@localhost/db", "DBMIGRATE_TIMEOUT": "5h"},
	} {
		_, err := loadConnectionSettings(func(key string) string { return values[key] })
		if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "postgres://") {
			t.Fatalf("expected sanitized configuration error, got %v", err)
		}
	}
}
