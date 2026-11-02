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

func TestValidateMigrationExecutionIdentityRequiresDedicatedLoginAndSchemaOwner(t *testing.T) {
	validURL := "postgres://espacigo_migrator_login@127.0.0.1:5432/espacigo"
	if err := validateMigrationExecutionIdentity(validURL, "espacigo_schema_owner"); err != nil {
		t.Fatalf("accepted documented non-superuser migration identity: %v", err)
	}
	for _, test := range []struct {
		name, databaseURL, setRole string
	}{
		{name: "missing role", databaseURL: validURL},
		{name: "runtime role", databaseURL: validURL, setRole: "espacigo_runtime"},
		{name: "superuser login", databaseURL: "postgres://postgres@127.0.0.1:5432/espacigo", setRole: "espacigo_schema_owner"},
		{name: "other login", databaseURL: "postgres://other@127.0.0.1:5432/espacigo", setRole: "espacigo_schema_owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateMigrationExecutionIdentity(test.databaseURL, test.setRole); err == nil {
				t.Fatalf("accepted database URL user and SET ROLE: %q / %q", test.databaseURL, test.setRole)
			}
		})
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

func TestValidateRuntimeACLAdminIdentityRequiresExactLocalTargetAndPrivatePassfile(t *testing.T) {
	passfile := filepath.Join(t.TempDir(), "pgpass")
	if err := os.WriteFile(passfile, []byte("synthetic-test-line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PGPASSFILE", passfile)
	t.Setenv("PGPASSWORD", "")
	t.Setenv("PGSERVICE", "")
	t.Setenv("PGSERVICEFILE", "")
	validURL := "postgres://postgres@127.0.0.1:15434/espacigo_f5_gate_20261101?sslmode=disable"
	if err := validateRuntimeACLAdminIdentity(validURL, "direct", ""); err != nil {
		t.Fatalf("accepted exact local administrative target: %v", err)
	}
	for _, test := range []struct {
		name, databaseURL, mode, setRole, passfile string
	}{
		{name: "wrong port", databaseURL: "postgres://postgres@127.0.0.1:15433/espacigo_f5_gate_20261101", mode: "direct", passfile: passfile},
		{name: "wrong database", databaseURL: "postgres://postgres@127.0.0.1:15434/espacigo_f5_synth_20261101", mode: "direct", passfile: passfile},
		{name: "wrong user", databaseURL: "postgres://espacigo_runtime@127.0.0.1:15434/espacigo_f5_gate_20261101", mode: "direct", passfile: passfile},
		{name: "embedded password", databaseURL: "postgres://postgres:synthetic-test-password@127.0.0.1:15434/espacigo_f5_gate_20261101", mode: "direct", passfile: passfile},
		{name: "query password", databaseURL: "postgres://postgres@127.0.0.1:15434/espacigo_f5_gate_20261101?password=synthetic-test-password", mode: "direct", passfile: passfile},
		{name: "cloud mode", databaseURL: validURL, mode: "cloudsql", passfile: passfile},
		{name: "set role", databaseURL: validURL, mode: "direct", setRole: "espacigo_schema_owner", passfile: passfile},
		{name: "disabled passfile", databaseURL: validURL, mode: "direct", passfile: "/dev/null"},
		{name: "relative passfile", databaseURL: validURL, mode: "direct", passfile: "pgpass"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("PGPASSFILE", test.passfile)
			if err := validateRuntimeACLAdminIdentity(test.databaseURL, test.mode, test.setRole); err == nil {
				t.Fatal("accepted unsafe runtime ACL administrative identity")
			}
		})
	}
	t.Setenv("PGPASSWORD", "synthetic-test-password")
	if err := validateRuntimeACLAdminIdentity(validURL, "direct", ""); err == nil {
		t.Fatal("accepted PGPASSWORD as an alternate credential source")
	}
	t.Setenv("PGPASSWORD", "")
	worldReadable := filepath.Join(t.TempDir(), "pgpass")
	if err := os.WriteFile(worldReadable, []byte("synthetic-test-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PGPASSFILE", worldReadable)
	if err := validateRuntimeACLAdminIdentity(validURL, "direct", ""); err == nil {
		t.Fatal("accepted a passfile with group/world permissions")
	}
}
