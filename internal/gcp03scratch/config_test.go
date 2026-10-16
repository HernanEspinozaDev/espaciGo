package gcp03scratch

import (
	"strings"
	"testing"
)

func validEnvironment(phase Phase) map[string]string {
	return map[string]string{
		"GCP03_PHASE":                        string(phase),
		"GCP03_CONFIRM_TARGET":               TargetConfirmation,
		"CLOUD_SQL_INSTANCE_CONNECTION_NAME": InstanceConnection,
		"DATABASE_URL":                       DatabaseURL,
		"DATABASE_PASSWORD_FILE":             PasswordFile,
		"CLOUD_SQL_PRIVATE_IP":               "true",
	}
}

func TestLoadConfigAcceptsOnlyNamedPhasesForExactScratchTarget(t *testing.T) {
	for _, phase := range []Phase{PhaseBootstrap, PhaseMigrate, PhaseFinalize, PhaseRetire} {
		t.Run(string(phase), func(t *testing.T) {
			values := validEnvironment(phase)
			config, err := LoadConfig(func(key string) string { return values[key] })
			if err != nil {
				t.Fatal(err)
			}
			if config.Phase != phase || config.Instance != InstanceConnection || config.DatabaseURL != DatabaseURL || !config.PrivateIP {
				t.Fatalf("unexpected loaded config: %#v", config)
			}
		})
	}
}

func TestLoadConfigRejectsTargetOverridesAndSecretFlags(t *testing.T) {
	for name, change := range map[string]func(map[string]string){
		"other phase":  func(e map[string]string) { e["GCP03_PHASE"] = "sql" },
		"other target": func(e map[string]string) { e["GCP03_CONFIRM_TARGET"] = "other-instance/other-database" },
		"other Cloud SQL": func(e map[string]string) {
			e["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] = "other-project:southamerica-west1:other-db"
		},
		"other database URL": func(e map[string]string) {
			e["DATABASE_URL"] = "postgres://someone@localhost:5432/other?sslmode=disable"
		},
		"inline password URL": func(e map[string]string) {
			e["DATABASE_URL"] = "postgres://user:secret@localhost:5432/espacigo_scratch?sslmode=disable"
		},
		"other secret mount":          func(e map[string]string) { e["DATABASE_PASSWORD_FILE"] = "/tmp/password" },
		"public IP connector":         func(e map[string]string) { e["CLOUD_SQL_PRIVATE_IP"] = "false" },
		"libpq environment":           func(e map[string]string) { e["PGHOST"] = "other-host" },
		"arbitrary SQL variable":      func(e map[string]string) { e["GCP03_SQL"] = "DROP DATABASE espacigo_scratch" },
		"arbitrary SQL file variable": func(e map[string]string) { e["GCP03_SQL_FILE"] = "/tmp/anything.sql" },
	} {
		t.Run(name, func(t *testing.T) {
			values := validEnvironment(PhaseBootstrap)
			change(values)
			if _, err := LoadConfig(func(key string) string { return values[key] }); err == nil {
				t.Fatal("unsafe GCP-03 configuration was accepted")
			}
		})
	}
}

func TestParsePasswordAcceptsOnlyGeneratedBase64URLShape(t *testing.T) {
	password := strings.Repeat("x", 43)
	got, err := parsePassword(strings.NewReader(password))
	if err != nil || got != password {
		t.Fatalf("valid password rejected: %v", err)
	}
	for _, invalid := range []string{"", password + "\n", strings.Repeat("x", 42), strings.Repeat("x", 257), "password-with-space " + strings.Repeat("x", 20)} {
		if got, err := parsePassword(strings.NewReader(invalid)); err == nil || (invalid != "" && (strings.Contains(err.Error(), invalid) || strings.Contains(got, invalid))) {
			t.Fatalf("invalid password was accepted or leaked: %q %v", got, err)
		}
	}
}
