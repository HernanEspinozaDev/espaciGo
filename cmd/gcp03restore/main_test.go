package main

import "testing"

func TestLoadConfigContractAcrossAllPhases(t *testing.T) {
	base := map[string]string{
		"GCP03_PROJECT": "espacigo-testprod", "GCP03_REGION": "southamerica-west1",
		"GCP03_INSTANCE": "espacigo-gcp03-restore-fixture1-pg", "GCP03_RUN_ID": "fixture1",
		"GCP03_CONFIRM_TARGET": "espacigo-testprod:southamerica-west1:espacigo-gcp03-restore-fixture1-pg/espacigo_scratch",
	}
	for _, phase := range []struct {
		name, user, bucket, manifest, migration, adminSecret string
	}{
		{name: "bootstrap", user: "postgres", adminSecret: "/secrets/postgres/password"},
		{name: "restore", user: "g03-restore-fixture1@espacigo-testprod.iam", bucket: "espacigo-gcp03-restore-fixture1", manifest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{name: "verify", user: "g03-verify-fixture1@espacigo-testprod.iam", manifest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", migration: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{name: "retire", user: "postgres", adminSecret: "/secrets/postgres/password"},
	} {
		t.Run(phase.name, func(t *testing.T) {
			env := map[string]string{}
			for key, value := range base {
				env[key] = value
			}
			env["GCP03_PHASE"] = phase.name
			env["GCP03_BUCKET"] = phase.bucket
			env["GCP03_MANIFEST_SHA256"] = phase.manifest
			env["GCP03_MIGRATION_SHA256"] = phase.migration
			env["GCP03_ADMIN_PASSWORD_FILE"] = phase.adminSecret
			env["DATABASE_URL"] = "postgres://" + phase.user + "@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
			get := func(key string) string { return env[key] }
			if _, err := loadConfig(get); err != nil {
				t.Fatalf("valid %s configuration rejected: %v", phase.name, err)
			}
		})
	}
}

func TestLoadConfigRejectsBucketOrTargetMismatch(t *testing.T) {
	env := map[string]string{
		"GCP03_PHASE": "restore", "GCP03_PROJECT": "espacigo-testprod", "GCP03_REGION": "southamerica-west1",
		"GCP03_INSTANCE": "espacigo-gcp03-restore-fixture1-pg", "GCP03_RUN_ID": "fixture1",
		"GCP03_BUCKET": "espacigo-gcp03-restore-fixture1-artifacts", "GCP03_MANIFEST_SHA256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"GCP03_CONFIRM_TARGET": "espacigo-testprod:southamerica-west1:espacigo-gcp03-restore-fixture1-pg/espacigo_scratch",
		"DATABASE_URL":         "postgres://g03-restore-fixture1%40espacigo-testprod.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable",
	}
	get := func(key string) string { return env[key] }
	if _, err := loadConfig(get); err == nil {
		t.Fatal("accepted a Terraform-style bucket suffix that differs from the exact executor bucket contract")
	}
	env["GCP03_BUCKET"] = "espacigo-gcp03-restore-fixture1"
	env["GCP03_INSTANCE"] = "espacigo-gcp03-pilot01-pg"
	if _, err := loadConfig(get); err == nil {
		t.Fatal("accepted an unrelated SQL instance")
	}
	env["GCP03_INSTANCE"] = "espacigo-gcp03-restore-fixture1-pg"
	env["DATABASE_URL"] = "postgres://user:password@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
	if _, err := loadConfig(get); err == nil {
		t.Fatal("accepted a password-bearing connection URL")
	}
}
