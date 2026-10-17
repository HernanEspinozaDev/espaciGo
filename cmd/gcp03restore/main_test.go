package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFailureEvidenceKeepsTypedArtifactSubsteps(t *testing.T) {
	env := map[string]string{"GCP03_PHASE": "restore", "GCP03_RUN_ID": "local01"}
	getenv := func(key string) string { return env[key] }
	for code, substep := range restoreFailureCodes {
		var output strings.Builder
		writeFailureEvidence(&output, getenv, restoreFailure{code: code, substep: substep})
		var got failureEvidence
		if err := json.Unmarshal([]byte(output.String()), &got); err != nil {
			t.Fatalf("code %s did not serialize: %v", code, err)
		}
		if got.Stage != "artifact_validation" || got.ErrorCode != code || got.Substep != substep {
			t.Fatalf("code %s produced stage=%q code=%q substep=%q", code, got.Stage, got.ErrorCode, got.Substep)
		}
	}
}

func TestGeneratedTOCValidationDistinguishesHeaderAndEntryDifferences(t *testing.T) {
	sealed := []byte("; Archive created at 2026-10-16 17:53:07 -03\n1; 0 10 TABLE public sample owner\n2; 0 11 TABLE DATA public sample owner\n")
	generated := []byte("; Archive created at 2026-10-16 16:53:07 UTC\n1; 0 10 TABLE public sample owner\n2; 0 11 TABLE DATA public sample owner\n")
	allowlist := []byte("2; 0 11 TABLE DATA public sample owner\n")
	if err := validateGeneratedTOC(generated, sealed, allowlist); err != nil {
		t.Fatalf("identical entries with an environment-dependent header rejected: %v", err)
	}
	changed := []byte("; Archive created at 2026-10-16 16:53:07 UTC\n1; 0 10 TABLE public altered owner\n2; 0 11 TABLE DATA public sample owner\n")
	err := validateGeneratedTOC(changed, sealed, allowlist)
	var failure restoreFailure
	if !errors.As(err, &failure) || failure.code != "ARTIFACT_GENERATED_TOC_MISMATCH" {
		t.Fatalf("changed entry did not return the generated TOC substep code: %v", err)
	}
}

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
		{name: "diagnose", user: "g03-verify-fixture1@espacigo-testprod.iam", bucket: "espacigo-gcp03-restore-fixture1", manifest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
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

func TestDiagnosticFailureEvidenceIsAllowlistedAndSanitized(t *testing.T) {
	env := map[string]string{"GCP03_PHASE": "diagnose", "GCP03_RUN_ID": "local01"}
	var output strings.Builder
	writeFailureEvidence(&output, func(key string) string { return env[key] }, diagnosticFailure{code: "DIAGNOSTIC_DB_METADATA_CHECK_FAILED", substep: "schema_metadata"})
	var got failureEvidence
	if err := json.Unmarshal([]byte(output.String()), &got); err != nil {
		t.Fatal("diagnostic failure evidence is invalid JSON")
	}
	if got.Stage != "read_only_diagnostic" || got.ErrorCode != "DIAGNOSTIC_DB_METADATA_CHECK_FAILED" || got.Substep != "schema_metadata" || strings.Contains(output.String(), "SELECT") {
		t.Fatalf("diagnostic failure leaked SQL or lost its safe code: %s", output.String())
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
