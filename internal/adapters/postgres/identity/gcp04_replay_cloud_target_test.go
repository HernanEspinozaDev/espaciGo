package postgres_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestGCP04ReplayCloudTargetMatchesTerraformConnectionName(t *testing.T) {
	mainTF, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "terraform", "environments", "testprod", "gcp04-replay-smoke", "main.tf"))
	if err != nil {
		t.Fatalf("read GCP-04 Terraform config: %v", err)
	}
	connectionNameBinding := regexp.MustCompile(`(?s)env\s*\{\s*name\s*=\s*"GCP04_REPLAY_CLOUD_SQL_INSTANCE"\s*value\s*=\s*google_sql_database_instance\.scratch\[0\]\.connection_name\s*\}`)
	if !connectionNameBinding.Match(mainTF) {
		t.Fatal("Terraform must pass Cloud SQL connection_name to the Go Connector")
	}

	// This is the evaluated connection_name for the fixed GCP-04 testprod run.
	terraformValue := "espacigo-testprod:southamerica-west1:espacigo-gcp04-g04replay18-pg"
	if err := validateGCP04ReplayCloudTarget(terraformValue, "true"); err != nil {
		t.Fatalf("Terraform connection_name was rejected: %v", err)
	}
	for _, target := range []string{
		"espacigo-gcp04-g04replay18-pg",
		"other-project:southamerica-west1:espacigo-gcp04-g04replay18-pg",
		"espacigo-testprod:us-central1:espacigo-gcp04-g04replay18-pg",
		"espacigo-testprod:southamerica-west1:other-instance",
	} {
		if err := validateGCP04ReplayCloudTarget(target, "true"); err == nil {
			t.Errorf("foreign or short Connector target %q was accepted", target)
		}
	}
	if err := validateGCP04ReplayCloudTarget(terraformValue, "false"); err == nil {
		t.Fatal("public-IP connector setting was accepted")
	}
}
