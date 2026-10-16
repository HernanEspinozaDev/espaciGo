package main

import "testing"

func TestLoadConfigRequiresExactRunAndLocalProxyTarget(t *testing.T) {
	env := map[string]string{
		"GCP03_PHASE": "restore", "GCP03_PROJECT": "espacigo-testprod", "GCP03_REGION": "southamerica-west1",
		"GCP03_INSTANCE": "espacigo-gcp03-restore-fixture1-pg", "GCP03_RUN_ID": "fixture1",
		"GCP03_BUCKET": "espacigo-gcp03-restore-fixture1", "GCP03_MANIFEST_SHA256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"GCP03_CONFIRM_TARGET": "espacigo-testprod:southamerica-west1:espacigo-gcp03-restore-fixture1-pg/espacigo_scratch",
		"DATABASE_URL":         "postgres://g03-restore-fixture1%40espacigo-testprod.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable",
	}
	get := func(key string) string { return env[key] }
	if _, err := loadConfig(get); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	env["GCP03_INSTANCE"] = "espacigo-gcp03-pilot01-pg"
	if _, err := loadConfig(get); err == nil {
		t.Fatal("accepted unrelated SQL instance")
	}
	env["GCP03_INSTANCE"] = "espacigo-gcp03-restore-fixture1-pg"
	env["DATABASE_URL"] = "postgres://user:password@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
	if _, err := loadConfig(get); err == nil {
		t.Fatal("accepted password-bearing connection URL")
	}
}
