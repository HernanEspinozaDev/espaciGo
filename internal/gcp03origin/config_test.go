package gcp03origin

import (
	"strings"
	"testing"
)

func env(phase Phase) map[string]string {
	values := map[string]string{
		"GCP03_ORIGIN_PHASE":                 string(phase),
		"GCP03_ORIGIN_CONFIRM_TARGET":        TargetConfirmation,
		"CLOUD_SQL_INSTANCE_CONNECTION_NAME": InstanceConnection,
		"CLOUD_SQL_PRIVATE_IP":               "true",
		"DATABASE_CONNECTION_MODE":           "cloudsql",
	}
	if phase == PhaseBootstrap {
		values["DATABASE_URL"] = AdminDatabaseURL
		values["DATABASE_PASSWORD_FILE"] = AdminPasswordFile
	} else {
		values["DATABASE_URL"] = ObserverDatabaseURL
	}
	return values
}

func TestLoadConfigAcceptsOnlyFixedBootstrapAndObserverProfiles(t *testing.T) {
	for _, phase := range []Phase{PhaseBootstrap, PhaseObserve} {
		t.Run(string(phase), func(t *testing.T) {
			values := env(phase)
			config, err := LoadConfig(func(key string) string { return values[key] })
			if err != nil {
				t.Fatal(err)
			}
			if config.Phase != phase || config.Instance != InstanceConnection || !config.PrivateIP {
				t.Fatalf("unexpected fixed profile: %#v", config)
			}
			if phase == PhaseObserve && (config.PasswordFile != "" || config.Login != ObserverLogin) {
				t.Fatalf("observer received a password profile: %#v", config)
			}
		})
	}
}

func TestLoadConfigRejectsTargetAndSecretOverrides(t *testing.T) {
	for _, test := range []struct {
		name, key, value string
		phase            Phase
	}{
		{"other target", "GCP03_ORIGIN_CONFIRM_TARGET", "other/db", PhaseBootstrap},
		{"other instance", "CLOUD_SQL_INSTANCE_CONNECTION_NAME", "other:region:db", PhaseBootstrap},
		{"public ip", "CLOUD_SQL_PRIVATE_IP", "false", PhaseBootstrap},
		{"bootstrap url", "DATABASE_URL", "postgres://attacker@localhost:5432/db?sslmode=disable", PhaseBootstrap},
		{"observer secret", "DATABASE_PASSWORD_FILE", AdminPasswordFile, PhaseObserve},
		{"arbitrary sql", "GCP03_SQL", "DROP DATABASE", PhaseBootstrap},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := env(test.phase)
			values[test.key] = test.value
			if _, err := LoadConfig(func(key string) string { return values[key] }); err == nil || strings.Contains(err.Error(), test.value) {
				t.Fatalf("expected sanitized rejection, got %v", err)
			}
		})
	}
}

func TestExactExtensionsRejectsAdditionalOrMissingExtensions(t *testing.T) {
	good := []Extension{{Name: "btree_gist", Version: "1.8"}, {Name: "fuzzystrmatch", Version: "1.2"}, {Name: "plpgsql", Version: "1.0"}, {Name: "postgis", Version: "3.6.4"}, {Name: "postgis_tiger_geocoder", Version: "3.6.4"}, {Name: "postgis_topology", Version: "3.6.4"}, {Name: "unaccent", Version: "1.1"}}
	if !exactExtensions(good) {
		t.Fatal("expected exact source-compatible Cloud SQL extension versions")
	}
	if exactExtensions(good[:2]) {
		t.Fatal("missing extension must fail")
	}
	if exactExtensions(append(good, Extension{Name: "hstore", Version: "1.8"})) {
		t.Fatal("unexpected extension must fail")
	}
	wrongVersion := append([]Extension(nil), good...)
	wrongVersion[3].Version = "3.5.2"
	if exactExtensions(wrongVersion) {
		t.Fatal("unsupported PostGIS minor must fail")
	}
}

func TestObserverLoginMatchesTruncatedServiceAccountName(t *testing.T) {
	if got := strings.TrimSuffix(ObserverServiceAcct, ".gserviceaccount.com"); got != ObserverLogin {
		t.Fatalf("observer IAM database login = %q, want %q", ObserverLogin, got)
	}
}
