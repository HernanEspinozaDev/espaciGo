package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestExecutorPreservesPhaseFailureAsNonzeroExitAfterProxyShutdown(t *testing.T) {
	var stdout, stderr strings.Builder
	shutdownCalls := 0
	code := executePhase(context.Background(), func(context.Context) (evidence, error) {
		return evidence{}, retirementFailure("fixture_counts", nil)
	}, func(context.Context) error { shutdownCalls++; return nil }, &stdout, &stderr, func(key string) string {
		if key == "GCP03_PHASE" {
			return "retire"
		}
		return ""
	})
	if code != 1 || shutdownCalls != 1 || stdout.Len() != 0 {
		t.Fatalf("phase failure was lost: exit=%d shutdownCalls=%d stdout=%q", code, shutdownCalls, stdout.String())
	}
	var got failureEvidence
	if err := json.Unmarshal([]byte(stderr.String()), &got); err != nil || got.Stage != "retire" || got.Substep != "fixture_counts" {
		t.Fatalf("failure evidence was not emitted: evidence=%+v err=%v", got, err)
	}
}

func TestExecutorReportsSuccessOnlyAfterProxyShutdown(t *testing.T) {
	var stdout, stderr strings.Builder
	code := executePhase(context.Background(), func(context.Context) (evidence, error) {
		return evidence{Phase: "retire"}, nil
	}, func(context.Context) error { return nil }, &stdout, &stderr, func(string) string { return "" })
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("successful phase returned failure: exit=%d stderr=%q", code, stderr.String())
	}
	var got evidence
	if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil || got.Phase != "retire" {
		t.Fatalf("successful evidence missing: evidence=%+v err=%v", got, err)
	}
}

func TestExecutorFailureEvidenceWriterFailureStillReturnsNonzero(t *testing.T) {
	code := executePhase(context.Background(), func(context.Context) (evidence, error) {
		return evidence{}, errors.New("phase failure")
	}, func(context.Context) error { return nil }, io.Discard, io.Discard, func(string) string { return "" })
	if code != 1 {
		t.Fatalf("failure path returned %d, want 1", code)
	}
}

func TestRetirementFailureEvidenceIsTypedSanitizedAndDoesNotClaimRollback(t *testing.T) {
	env := map[string]string{"GCP03_PHASE": "retire", "GCP03_RUN_ID": "local01"}
	unsafe := &pgconn.PgError{Code: "42501", Message: "permission denied for secret_table", Detail: "private@example.test"}
	var output strings.Builder
	writeFailureEvidence(&output, func(key string) string { return env[key] }, runFailure{
		cause: retirementFailure("revoke_schema_create", unsafe), proxyShutdownConfirmed: true,
	})
	var got failureEvidence
	if err := json.Unmarshal([]byte(output.String()), &got); err != nil {
		t.Fatal("retirement failure was not valid JSON")
	}
	if got.Format != "espacigo-gcp03-managed-synthetic-restore-failure-v2" || got.Stage != "retire" || got.ErrorCode != "RETIRE_STEP_FAILED" || got.Substep != "revoke_schema_create" || got.SQLState != "42501" || got.CommitOutcome != "" || !got.ProxyShutdownConfirmed || got.ApplicationExitCode != 1 {
		t.Fatalf("unexpected typed retirement evidence: %+v", got)
	}
	if strings.Contains(output.String(), "permission denied") || strings.Contains(output.String(), "private@example.test") || strings.Contains(output.String(), "secret_table") {
		t.Fatal("retirement failure evidence leaked raw PostgreSQL diagnostics")
	}
}

func TestRetirementCommitFailureExplicitlyMarksOutcomeUncertain(t *testing.T) {
	commit := retirementFailure("commit", &pgconn.PgError{Code: "08006", Message: "connection dropped"}).(retireFailure)
	commit.code, commit.commitOutcome = "COMMIT_OUTCOME_UNCERTAIN", "uncertain"
	var output strings.Builder
	writeFailureEvidence(&output, func(key string) string {
		if key == "GCP03_PHASE" {
			return "retire"
		}
		return ""
	}, commit)
	var got failureEvidence
	if err := json.Unmarshal([]byte(output.String()), &got); err != nil {
		t.Fatal("commit failure was not valid JSON")
	}
	if got.Stage != "retire" || got.Substep != "commit" || got.SQLState != "08006" || got.CommitOutcome != "uncertain" {
		t.Fatalf("commit uncertainty was not explicit: %+v", got)
	}
}

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
		{name: "retire-diagnose", user: "postgres", adminSecret: "/secrets/postgres/password"},
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
