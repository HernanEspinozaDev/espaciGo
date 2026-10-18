package gcp04credential

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

type fakeStateReader struct {
	data       []byte
	generation int64
	err        error
	calls      int
}

func (f *fakeStateReader) ReadState(_ context.Context, bucket, object string) ([]byte, int64, error) {
	f.calls++
	if bucket != StateBucket || object != StateObject {
		return nil, 0, errors.New("wrong state target")
	}
	return append([]byte(nil), f.data...), f.generation, f.err
}

type fakeInspector struct {
	instance SQLInstance
	secret   string
	versions []string
	readErr  error
	calls    int
}

func (f *fakeInspector) ReadSQLInstance(context.Context, string, string) (SQLInstance, error) {
	f.calls++
	return f.instance, f.readErr
}
func (f *fakeInspector) ReadSecret(context.Context, string, string) (string, error) {
	f.calls++
	return f.secret, f.readErr
}
func (f *fakeInspector) ListSecretVersions(context.Context, string, string) ([]string, error) {
	f.calls++
	return append([]string(nil), f.versions...), f.readErr
}

type fakeSecretWriter struct {
	version string
	err     error
	payload []byte
	calls   int
}

func (f *fakeSecretWriter) AddVersion(_ context.Context, parent string, password []byte) (string, error) {
	f.calls++
	if parent != SecretParent {
		return "", errors.New("wrong secret")
	}
	f.payload = append([]byte(nil), password...)
	return f.version, f.err
}

type fakeSQLWriter struct {
	operation string
	setErr    error
	readErr   error
	done      bool
	failed    bool
	password  []byte
	setCalls  int
	readCalls int
}

func (f *fakeSQLWriter) SetPassword(_ context.Context, project, instance, user string, password []byte) (string, error) {
	f.setCalls++
	if project != ProjectID || instance != InstanceName || user != DatabaseOwner {
		return "", errors.New("wrong SQL target")
	}
	f.password = append([]byte(nil), password...)
	return f.operation, f.setErr
}
func (f *fakeSQLWriter) ReadOperation(_ context.Context, project, operation string) (bool, bool, error) {
	f.readCalls++
	if project != ProjectID || operation != f.operation {
		return false, false, errors.New("wrong operation")
	}
	return f.done, f.failed, f.readErr
}

type fakeJournal struct {
	created   bool
	createErr error
	updateErr error
	records   []JournalRecord
}

func (f *fakeJournal) Create(record JournalRecord) error {
	f.created = true
	if f.createErr != nil {
		return f.createErr
	}
	f.records = append(f.records, record)
	return nil
}
func (f *fakeJournal) Update(record JournalRecord) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.records = append(f.records, record)
	return nil
}

type fakeEvidence struct{ expected string }

func (f fakeEvidence) VerifyPlan(digest string) error {
	if digest != f.expected {
		return errors.New("plan hash mismatch")
	}
	return nil
}

type failOnRead struct{}

func (failOnRead) Read([]byte) (int, error) { return 0, errors.New("random source must not be read") }

func newFixture(t *testing.T) (Release, []byte, []byte, string) {
	t.Helper()
	stateData := fixtureState(t)
	stateSum := sha256.Sum256(stateData)
	release := Release{
		SchemaVersion: 1, Decision: "released", ReleaseID: "review-g04-credential-20261018",
		Reviewer: "HernanEspinozaDev", IssuedAt: fixedNow.Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt: fixedNow.Add(time.Hour).Format(time.RFC3339), Operation: CredentialOp,
		ProjectID: ProjectID, ProjectNumber: ProjectNumber, Region: Region, RunID: RunID,
		Instance: InstanceName, Secret: SecretName,
		State:      StateRelease{Bucket: StateBucket, Object: StateObject, Lineage: "lineage-g04", Serial: 2, Generation: 77, SHA256: hex.EncodeToString(stateSum[:])},
		PlanSHA256: strings.Repeat("a", 64),
	}
	data, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	releaseSum := sha256.Sum256(data)
	return release, data, stateData, hex.EncodeToString(releaseSum[:])
}

func fixtureState(t *testing.T) []byte {
	t.Helper()
	types := []struct{ typ, name string }{
		{"google_artifact_registry_repository", "smoke"},
		{"google_artifact_registry_repository_iam_member", "cloud_run_reader"},
		{"google_project_iam_member", "runner_cloudsql_client"},
		{"google_secret_manager_secret", "database_owner_password"},
		{"google_secret_manager_secret_iam_member", "runner_database_password"},
		{"google_service_account", "runner"},
		{"google_sql_database_instance", "scratch"},
		{"google_storage_bucket", "private"},
		{"google_storage_bucket_iam_member", "runner_objects"},
	}
	resources := make([]map[string]any, 0, len(types))
	for _, item := range types {
		attrs := map[string]any{}
		if item.typ == "google_sql_database_instance" {
			attrs = map[string]any{"name": InstanceName, "project": ProjectID, "region": Region, "database_version": "POSTGRES_18"}
		}
		if item.typ == "google_secret_manager_secret" {
			attrs = map[string]any{"secret_id": SecretName}
		}
		resources = append(resources, map[string]any{"mode": "managed", "type": item.typ, "name": item.name, "instances": []any{map[string]any{"attributes": attrs}}})
	}
	resources = append(resources,
		map[string]any{"mode": "data", "type": "google_compute_network", "name": "existing", "instances": []any{map[string]any{"attributes": map[string]any{}}}},
		map[string]any{"mode": "data", "type": "google_compute_subnetwork", "name": "existing", "instances": []any{map[string]any{"attributes": map[string]any{}}}},
		map[string]any{"mode": "data", "type": "google_project", "name": "current", "instances": []any{map[string]any{"attributes": map[string]any{}}}},
	)
	data, err := json.Marshal(map[string]any{"lineage": "lineage-g04", "serial": 2, "resources": resources})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newRunner(data []byte, generation int64, versions []string) (*CredentialRunner, *fakeSecretWriter, *fakeSQLWriter, *fakeJournal, *fakeInspector, *bytes.Reader) {
	secret := &fakeSecretWriter{version: "projects/447373903153/secrets/" + SecretName + "/versions/3"}
	sql := &fakeSQLWriter{operation: "op-g04-test", done: true}
	journal := &fakeJournal{}
	inspector := &fakeInspector{
		instance: SQLInstance{Name: InstanceName, Project: ProjectID, Region: Region, Version: "POSTGRES_18", State: "RUNNABLE", PrivateNetwork: "projects/espacigo-testprod/global/networks/espacigo-testprod", RunID: RunID},
		secret:   "projects/447373903153/secrets/" + SecretName,
		versions: versions,
	}
	random := bytes.NewReader(bytes.Repeat([]byte{0x5a}, 32))
	release, _, _, _ := newFixtureForRunner(data)
	return &CredentialRunner{
		States: &fakeStateReader{data: data, generation: generation}, Inspector: inspector,
		Secrets: secret, SQL: sql, Journal: journal, Evidence: fakeEvidence{expected: release.PlanSHA256},
		Random: random, Now: func() time.Time { return fixedNow }, Wait: func(context.Context, time.Duration) error { return nil },
	}, secret, sql, journal, inspector, random
}

func newFixtureForRunner(stateData []byte) (Release, []byte, []byte, string) {
	release := Release{
		SchemaVersion: 1, Decision: "released", ReleaseID: "review-g04-credential-20261018",
		Reviewer: "HernanEspinozaDev", IssuedAt: fixedNow.Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt: fixedNow.Add(time.Hour).Format(time.RFC3339), Operation: CredentialOp,
		ProjectID: ProjectID, ProjectNumber: ProjectNumber, Region: Region, RunID: RunID,
		Instance: InstanceName, Secret: SecretName,
		State:      StateRelease{Bucket: StateBucket, Object: StateObject, Lineage: "lineage-g04", Serial: 2, Generation: 77},
		PlanSHA256: strings.Repeat("a", 64),
	}
	stateSum := sha256.Sum256(stateData)
	release.State.SHA256 = hex.EncodeToString(stateSum[:])
	data, _ := json.Marshal(release)
	releaseSum := sha256.Sum256(data)
	return release, data, stateData, hex.EncodeToString(releaseSum[:])
}

func TestPreviewDoesNotCallCloudOrGenerateSecret(t *testing.T) {
	_, data, stateData, _ := newFixture(t)
	runner, secret, sql, journal, inspector, _ := newRunner(stateData, 77, nil)
	runner.Random = failOnRead{}
	preview, err := runner.Preview(data, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CloudCalls != "none" || preview.SecretCreated != "no" || preview.ReleaseSHA256 == "" {
		t.Fatalf("unsafe preview: %#v", preview)
	}
	if secret.calls != 0 || sql.setCalls != 0 || inspector.calls != 0 || journal.created {
		t.Fatal("preview called a Cloud client or created a journal")
	}
}

func TestExecutionRequiresExactReleaseHashBeforeCloudCalls(t *testing.T) {
	_, data, stateData, _ := newFixture(t)
	runner, secret, sql, journal, inspector, _ := newRunner(stateData, 77, nil)
	if _, err := runner.Execute(context.Background(), data, strings.Repeat("0", 64)); err == nil {
		t.Fatal("mismatched exact-release SHA was accepted")
	}
	if secret.calls != 0 || sql.setCalls != 0 || inspector.calls != 0 || journal.created {
		t.Fatal("mismatched release reached Cloud clients")
	}
}

func TestExecutionStopsOnExistingSecretVersionBeforeGeneratingOrWriting(t *testing.T) {
	_, data, stateData, digest := newFixture(t)
	runner, secret, sql, journal, inspector, _ := newRunner(stateData, 77, []string{SecretParent + "/versions/1"})
	runner.Random = failOnRead{}
	if _, err := runner.Execute(context.Background(), data, digest); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("existing version did not fail closed: %v", err)
	}
	if secret.calls != 0 || sql.setCalls != 0 || journal.created || inspector.calls != 3 {
		t.Fatal("collision performed a write or skipped exact target reads")
	}
}

func TestSecretVersionAmbiguityIsJournaledAndStopsBeforeSQL(t *testing.T) {
	_, data, stateData, digest := newFixture(t)
	runner, secret, sql, journal, _, _ := newRunner(stateData, 77, nil)
	secret.err = errors.New("ambiguous API response")
	result, err := runner.Execute(context.Background(), data, digest)
	if err == nil || !result.Reconcile || result.JournalPhase != "secret_version_outcome_uncertain" {
		t.Fatalf("unexpected partial result %#v, %v", result, err)
	}
	if secret.calls != 1 || sql.setCalls != 0 || len(journal.records) < 2 || journal.records[len(journal.records)-1].Phase != "secret_version_outcome_uncertain" {
		t.Fatal("ambiguous Secret Manager result was not journaled and stopped")
	}
}

func TestSQLUpdatePartialFailureRetainsVersionAndNeverRetriesOrCompensates(t *testing.T) {
	_, data, stateData, digest := newFixture(t)
	runner, secret, sql, journal, _, _ := newRunner(stateData, 77, nil)
	sql.setErr = errors.New("ambiguous SQL API response")
	result, err := runner.Execute(context.Background(), data, digest)
	if err == nil || !result.Reconcile || result.JournalPhase != "sql_password_update_outcome_uncertain" {
		t.Fatalf("unexpected partial result %#v, %v", result, err)
	}
	if secret.calls != 1 || sql.setCalls != 1 || result.SecretVersion != secret.version || len(journal.records) == 0 || journal.records[len(journal.records)-1].Phase != result.JournalPhase {
		t.Fatal("partial SQL failure did not preserve one version and a reconciliation journal")
	}
	journalJSON, marshalErr := json.Marshal(journal.records)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if bytes.Contains(journalJSON, secret.payload) || bytes.Contains(journalJSON, sql.password) {
		t.Fatal("credential value was written to the journal")
	}
}

func TestSuccessfulCredentialOperationUsesSameInMemoryValueAndEmitsMetadataOnly(t *testing.T) {
	_, data, stateData, digest := newFixture(t)
	runner, secret, sql, journal, inspector, _ := newRunner(stateData, 77, nil)
	result, err := runner.Execute(context.Background(), data, digest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secret.payload, sql.password) || result.SecretVersion != secret.version || result.SQLStatus != "complete" || result.Reconcile {
		t.Fatalf("unexpected success result %#v", result)
	}
	if inspector.calls != 3 || secret.calls != 1 || sql.setCalls != 1 || sql.readCalls != 1 {
		t.Fatal("success did not use one preflight, one version write, one SQL update and one operation read")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, secret.payload) || bytes.Contains(encoded, sql.password) {
		t.Fatal("credential value appeared in operation result")
	}
	if journal.records[len(journal.records)-1].Phase != "complete" || !bytes.Equal(sql.password, secret.payload) {
		t.Fatal("journal did not record terminal metadata or same secret value")
	}
}

func TestTerraformStateMismatchBlocksBeforeTargetWrites(t *testing.T) {
	_, data, stateData, digest := newFixture(t)
	stateData = append(stateData, []byte(" ")...)
	runner, secret, sql, journal, _, _ := newRunner(stateData, 77, nil)
	if _, err := runner.Execute(context.Background(), data, digest); err == nil {
		t.Fatal("state hash mismatch was accepted")
	}
	if secret.calls != 0 || sql.setCalls != 0 || journal.created {
		t.Fatal("state mismatch reached credential writes")
	}
}

func TestTerraformStateAllowsExactDataSourcesAndRejectsExtraManagedResource(t *testing.T) {
	data := fixtureState(t)
	fixture, _, _, _ := newFixtureForRunner(data)
	if err := validateTerraformState(data, fixture.State); err != nil {
		t.Fatalf("real base state shape was rejected: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	resources := state["resources"].([]any)
	resources = append(resources, map[string]any{"mode": "managed", "type": "google_storage_bucket", "name": "unexpected", "instances": []any{map[string]any{"attributes": map[string]any{}}}})
	state["resources"] = resources
	extra, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTerraformState(extra, fixture.State); err == nil {
		t.Fatal("extra managed resource was accepted")
	}
}

func TestExpectedSQLNetworkAcceptsCanonicalFormsOnly(t *testing.T) {
	for _, value := range []string{ExpectedSQLNet, "projects/espacigo-testprod/global/networks/espacigo-testprod"} {
		if !isExpectedSQLNet(value) {
			t.Errorf("expected canonical network form to pass: %q", value)
		}
	}
	for _, value := range []string{"projects/another-project/global/networks/espacigo-testprod", "projects/espacigo-testprod/global/networks/other", "https://www.googleapis.com/compute/v1/projects/another-project/global/networks/espacigo-testprod"} {
		if isExpectedSQLNet(value) {
			t.Errorf("unexpected network form passed: %q", value)
		}
	}
}

func TestParseReleaseRejectsExpiredOrWrongRunAndUnknownFields(t *testing.T) {
	_, data, _, _ := newFixture(t)
	var release Release
	if err := json.Unmarshal(data, &release); err != nil {
		t.Fatal(err)
	}
	release.RunID = "another-run"
	wrongRun, _ := json.Marshal(release)
	if _, _, err := ParseRelease(wrongRun, fixedNow); err == nil {
		t.Fatal("wrong run ID was accepted")
	}
	_, data, _, _ = newFixture(t)
	if _, _, err := ParseRelease(append(data[:len(data)-1], []byte(`,"unexpected":"field"}`)...), fixedNow); err == nil {
		t.Fatal("unknown release field was accepted")
	}
}

func TestReleaseFileReaderRequiresPrivateFixedModeAndPath(t *testing.T) {
	if _, _, _, err := ReadPrivateRelease("/tmp/other-release.json", fixedNow); err == nil {
		t.Fatal("arbitrary release path was accepted")
	}
}

func TestMissingReleaseFileBlocksPreviewWithoutCloudOrCredentialGeneration(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	temporary := t.TempDir()
	if err := os.Chmod(temporary, 0700); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(temporary, filepath.Dir(ReleasePath))
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(temporary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if _, _, _, err := ReadPrivateRelease(ReleasePath, fixedNow); err == nil {
		t.Fatal("preview accepted a missing exact release")
	}
}
