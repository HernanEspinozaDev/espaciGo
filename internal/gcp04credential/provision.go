package gcp04credential

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

type StateObjectReader interface {
	ReadState(context.Context, string, string) ([]byte, int64, error)
}

type SQLInstance struct {
	Name, Project, Region, Version, State, PrivateNetwork, RunID string
	IPv4Enabled                                                  bool
}

type TargetInspector interface {
	ReadSQLInstance(context.Context, string, string) (SQLInstance, error)
	ReadSecret(context.Context, string, string) (string, error)
	ListSecretVersions(context.Context, string, string) ([]string, error)
}

type SecretVersionWriter interface {
	AddVersion(context.Context, string, []byte) (string, error)
}

type SQLPasswordWriter interface {
	SetPassword(context.Context, string, string, string, []byte) (string, error)
	ReadOperation(context.Context, string, string) (done, failed bool, err error)
}

type JournalRecord struct {
	SchemaVersion   int       `json:"schema_version"`
	ReleaseID       string    `json:"release_id"`
	ReleaseSHA256   string    `json:"release_sha256"`
	PlanSHA256      string    `json:"plan_sha256"`
	StateLineage    string    `json:"state_lineage"`
	StateSerial     uint64    `json:"state_serial"`
	StateGeneration int64     `json:"state_generation"`
	ProjectID       string    `json:"project_id"`
	RunID           string    `json:"run_id"`
	Instance        string    `json:"instance_name"`
	Secret          string    `json:"secret_name"`
	Phase           string    `json:"phase"`
	SecretVersion   string    `json:"secret_version,omitempty"`
	SQLOperation    string    `json:"sql_operation,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type JournalStore interface {
	Create(JournalRecord) error
	Update(JournalRecord) error
}

type EvidenceVerifier interface {
	VerifyPlan(string) error
}

type CredentialRunner struct {
	States    StateObjectReader
	Inspector TargetInspector
	Secrets   SecretVersionWriter
	SQL       SQLPasswordWriter
	Journal   JournalStore
	Evidence  EvidenceVerifier
	Random    io.Reader
	Now       func() time.Time
	Wait      func(context.Context, time.Duration) error
	PollEvery time.Duration
	MaxWait   time.Duration
}

type CredentialResult struct {
	ReleaseID     string
	SecretVersion string
	SQLStatus     string
	JournalPhase  string
	Reconcile     bool
}

func (r CredentialRunner) Preview(data []byte, now time.Time) (Preview, error) {
	release, hash, err := ParseRelease(data, now)
	if err != nil {
		return Preview{}, err
	}
	if r.Evidence == nil {
		return Preview{}, errors.New("local saved-plan evidence verifier is unavailable")
	}
	if err := r.Evidence.VerifyPlan(release.PlanSHA256); err != nil {
		return Preview{}, errors.New("local saved-plan evidence does not match the release")
	}
	return BuildPreview(release, hash), nil
}

func (r CredentialRunner) Execute(ctx context.Context, data []byte, confirmation string) (CredentialResult, error) {
	now := r.now()
	release, releaseHash, err := ParseRelease(data, now)
	if err != nil {
		return CredentialResult{}, err
	}
	if err := VerifyReleaseHash(releaseHash, confirmation); err != nil {
		return CredentialResult{}, err
	}
	if r.States == nil || r.Inspector == nil || r.Secrets == nil || r.SQL == nil || r.Journal == nil || r.Evidence == nil {
		return CredentialResult{}, errors.New("credential operation dependencies are unavailable")
	}
	if err := r.Evidence.VerifyPlan(release.PlanSHA256); err != nil {
		return CredentialResult{}, errors.New("local saved-plan evidence does not match the release")
	}
	_, generation, err := r.readReleasedState(ctx, release.State)
	if err != nil {
		return CredentialResult{}, err
	}
	instance, err := r.Inspector.ReadSQLInstance(ctx, ProjectID, InstanceName)
	if err != nil || instance.Name != InstanceName || instance.Project != ProjectID || instance.Region != Region ||
		instance.Version != "POSTGRES_18" || instance.State != "RUNNABLE" || instance.IPv4Enabled ||
		!isExpectedSQLNet(instance.PrivateNetwork) || instance.RunID != RunID {
		return CredentialResult{}, errors.New("Cloud SQL target differs from the exact run release; no credential operation started")
	}
	secretName, err := r.Inspector.ReadSecret(ctx, ProjectID, SecretName)
	if err != nil || !isSecretName(secretName) {
		return CredentialResult{}, errors.New("Secret Manager target differs from the exact run release; no credential operation started")
	}
	versions, err := r.Inspector.ListSecretVersions(ctx, ProjectID, SecretName)
	if err != nil {
		return CredentialResult{}, errors.New("Secret Manager versions could not be checked; no credential operation started")
	}
	if len(versions) != 0 {
		return CredentialResult{}, errors.New("Secret Manager collision: the exact run secret already has versions; reconcile before proceeding")
	}
	// Re-read the pinned state and plan after target preflight. This catches an
	// intervening Terraform change before the non-atomic credential writes.
	_, generation, err = r.readReleasedState(ctx, release.State)
	if err != nil {
		return CredentialResult{}, err
	}
	if generation != release.State.Generation || r.Evidence.VerifyPlan(release.PlanSHA256) != nil {
		return CredentialResult{}, errors.New("released state or saved-plan evidence changed before credential writes")
	}

	started := r.now().UTC()
	journal := JournalRecord{
		SchemaVersion: 1, ReleaseID: release.ReleaseID, ReleaseSHA256: releaseHash,
		PlanSHA256: release.PlanSHA256, StateLineage: release.State.Lineage,
		StateSerial: release.State.Serial, StateGeneration: generation,
		ProjectID: ProjectID, RunID: RunID, Instance: InstanceName,
		Secret: SecretName, Phase: "prepared", StartedAt: started, UpdatedAt: started,
	}
	if err := r.Journal.Create(journal); err != nil {
		return CredentialResult{}, errors.New("private credential journal already exists or could not be created; reconcile before proceeding")
	}
	password, err := r.newPassword()
	if err != nil {
		journal.Phase = "password_generation_failed"
		_ = r.updateJournal(journal)
		return CredentialResult{ReleaseID: release.ReleaseID, JournalPhase: journal.Phase, Reconcile: true}, errors.New("credential preparation failed; inspect the private journal")
	}
	defer clear(password)

	journal.Phase = "secret_version_create_started"
	if err := r.updateJournal(journal); err != nil {
		return CredentialResult{ReleaseID: release.ReleaseID, JournalPhase: "secret_version_create_started", Reconcile: true}, errors.New("journal update failed before Secret Manager call; stop and reconcile")
	}
	version, addErr := r.Secrets.AddVersion(ctx, SecretParent, password)
	if addErr != nil {
		journal.Phase = "secret_version_outcome_uncertain"
		if validVersion(version) {
			journal.SecretVersion = version
		}
		_ = r.updateJournal(journal)
		return resultFromJournal(journal, "not_started", true), errors.New("Secret Manager version creation failed or is uncertain; do not retry; reconcile this journal")
	}
	if !validVersion(version) {
		journal.Phase = "secret_version_identity_uncertain"
		journal.SecretVersion = strings.TrimSpace(version)
		_ = r.updateJournal(journal)
		return resultFromJournal(journal, "not_started", true), errors.New("Secret Manager returned an unexpected version identity; do not retry; reconcile this journal")
	}
	journal.SecretVersion = version
	journal.Phase = "secret_version_created"
	if err := r.updateJournal(journal); err != nil {
		return resultFromJournal(journal, "not_started", true), errors.New("secret version was created but journal update failed; do not call Cloud SQL; reconcile")
	}

	journal.Phase = "sql_password_update_started"
	if err := r.updateJournal(journal); err != nil {
		return resultFromJournal(journal, "not_started", true), errors.New("journal update failed before Cloud SQL call; stop and reconcile")
	}
	operation, updateErr := r.SQL.SetPassword(ctx, ProjectID, InstanceName, DatabaseOwner, password)
	if updateErr != nil || strings.TrimSpace(operation) == "" {
		journal.Phase = "sql_password_update_outcome_uncertain"
		_ = r.updateJournal(journal)
		return resultFromJournal(journal, "uncertain", true), errors.New("Cloud SQL password update outcome is uncertain; retain the exact secret version and reconcile; do not retry or rotate")
	}
	journal.SQLOperation = operation
	journal.Phase = "sql_operation_pending"
	if err := r.updateJournal(journal); err != nil {
		return resultFromJournal(journal, "uncertain", true), errors.New("Cloud SQL operation started but journal update failed; retain the exact version and reconcile")
	}
	if err := r.waitForOperation(ctx, operation); err != nil {
		journal.Phase = "sql_operation_outcome_uncertain"
		_ = r.updateJournal(journal)
		return resultFromJournal(journal, "uncertain", true), errors.New("Cloud SQL operation status is incomplete; retain the exact version and reconcile without retry")
	}
	journal.Phase = "complete"
	if err := r.updateJournal(journal); err != nil {
		return resultFromJournal(journal, "complete", true), errors.New("Cloud SQL operation completed but final journal write failed; reconcile the exact operation")
	}
	return resultFromJournal(journal, "complete", false), nil
}

func isExpectedSQLNet(value string) bool {
	return value == ExpectedSQLNet || value == "projects/espacigo-testprod/global/networks/espacigo-testprod"
}

func validateTerraformState(data []byte, expected StateRelease) error {
	var state struct {
		Lineage   string `json:"lineage"`
		Serial    uint64 `json:"serial"`
		Resources []struct {
			Mode      string `json:"mode"`
			Type      string `json:"type"`
			Name      string `json:"name"`
			Module    string `json:"module"`
			Instances []struct {
				Attributes map[string]any `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(data, &state) != nil || state.Lineage != expected.Lineage || state.Serial != expected.Serial {
		return errors.New("Terraform state lineage or serial differs from the release")
	}
	want := map[string]bool{
		"google_artifact_registry_repository.smoke":                        false,
		"google_artifact_registry_repository_iam_member.cloud_run_reader":  false,
		"google_project_iam_member.runner_cloudsql_client":                 false,
		"google_secret_manager_secret.database_owner_password":             false,
		"google_secret_manager_secret_iam_member.runner_database_password": false,
		"google_service_account.runner":                                    false,
		"google_sql_database_instance.scratch":                             false,
		"google_storage_bucket.private":                                    false,
		"google_storage_bucket_iam_member.runner_objects":                  false,
	}
	allowedData := map[string]bool{
		"data.google_compute_network.existing":    false,
		"data.google_compute_subnetwork.existing": false,
		"data.google_project.current":             false,
	}
	managedCount := 0
	for _, resource := range state.Resources {
		key := resource.Type + "." + resource.Name
		if resource.Mode == "data" {
			key = "data." + key
		}
		if resource.Module != "" || len(resource.Instances) != 1 {
			return errors.New("Terraform state contains an unexpected resource shape")
		}
		if resource.Mode == "data" {
			if _, ok := allowedData[key]; !ok || allowedData[key] {
				return errors.New("Terraform state contains an unexpected data resource")
			}
			allowedData[key] = true
			continue
		}
		if resource.Mode != "managed" {
			return errors.New("Terraform state contains an unexpected resource mode")
		}
		managedCount++
		if _, ok := want[key]; !ok || want[key] {
			return errors.New("Terraform state does not match the exact base resource set")
		}
		want[key] = true
		if key == "google_sql_database_instance.scratch" {
			attrs := resource.Instances[0].Attributes
			if attrs["name"] != InstanceName || attrs["project"] != ProjectID || attrs["region"] != Region || attrs["database_version"] != "POSTGRES_18" {
				return errors.New("Terraform state SQL instance attributes differ from the released target")
			}
		}
		if key == "google_secret_manager_secret.database_owner_password" && resource.Instances[0].Attributes["secret_id"] != SecretName {
			return errors.New("Terraform state secret metadata differs from the released target")
		}
	}
	if managedCount != ExpectedPlanAdd {
		return errors.New("Terraform state does not contain the exact nine-resource base")
	}
	for _, present := range allowedData {
		if !present {
			return errors.New("Terraform state is missing an expected data resource")
		}
	}
	for _, present := range want {
		if !present {
			return errors.New("Terraform state is missing an exact base resource")
		}
	}
	return nil
}

func (r CredentialRunner) readReleasedState(ctx context.Context, expected StateRelease) ([]byte, int64, error) {
	data, generation, err := r.States.ReadState(ctx, expected.Bucket, expected.Object)
	if err != nil {
		return nil, 0, errors.New("exact Terraform state object could not be read; no credential operation started")
	}
	sum := sha256.Sum256(data)
	if generation != expected.Generation || hex.EncodeToString(sum[:]) != expected.SHA256 {
		return nil, 0, errors.New("Terraform state generation or SHA-256 differs from the release; no credential operation started")
	}
	if err := validateTerraformState(data, expected); err != nil {
		return nil, 0, err
	}
	return data, generation, nil
}

func isSecretName(name string) bool {
	return name == SecretParent || name == "projects/"+ProjectNumber+"/secrets/"+SecretName
}

func validVersion(name string) bool {
	prefixes := []string{SecretParent + "/versions/", "projects/" + ProjectNumber + "/secrets/" + SecretName + "/versions/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			n := strings.TrimPrefix(name, prefix)
			if n == "" || strings.Trim(n, "0123456789") != "" || n[0] == '0' {
				return false
			}
			return true
		}
	}
	return false
}

func (r CredentialRunner) newPassword() ([]byte, error) {
	source := r.Random
	if source == nil {
		source = rand.Reader
	}
	secret := make([]byte, 32)
	if _, err := io.ReadFull(source, secret); err != nil {
		clear(secret)
		return nil, errors.New("private credential generation failed")
	}
	password := []byte(base64.RawURLEncoding.EncodeToString(secret))
	clear(secret)
	return password, nil
}

func (r CredentialRunner) waitForOperation(ctx context.Context, operation string) error {
	every := r.PollEvery
	if every <= 0 {
		every = 2 * time.Second
	}
	maxWait := r.MaxWait
	if maxWait <= 0 {
		maxWait = 10 * time.Minute
	}
	wait := r.Wait
	if wait == nil {
		wait = func(ctx context.Context, duration time.Duration) error {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	deadline := r.now().Add(maxWait)
	for r.now().Before(deadline) {
		done, failed, err := r.SQL.ReadOperation(ctx, ProjectID, operation)
		if err != nil {
			return errors.New("Cloud SQL operation status could not be read")
		}
		if done {
			if failed {
				return errors.New("Cloud SQL operation completed with an error")
			}
			return nil
		}
		if err := wait(ctx, every); err != nil {
			return errors.New("Cloud SQL operation wait was interrupted")
		}
	}
	return errors.New("Cloud SQL operation exceeded the bounded wait")
}

func (r CredentialRunner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r CredentialRunner) updateJournal(record JournalRecord) error {
	record.UpdatedAt = r.now()
	return r.Journal.Update(record)
}

func resultFromJournal(j JournalRecord, sqlStatus string, reconcile bool) CredentialResult {
	return CredentialResult{ReleaseID: j.ReleaseID, SecretVersion: j.SecretVersion, SQLStatus: sqlStatus, JournalPhase: j.Phase, Reconcile: reconcile}
}
