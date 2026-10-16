package gcp03restorecredential

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

type testAPIError struct{ definitive bool }

func (e testAPIError) Error() string    { return "sanitized test API error" }
func (e testAPIError) Definitive() bool { return e.definitive }

type fakeSecrets struct {
	versions   []string
	addName    string
	addErr     error
	destroyErr error
	added      []byte
	listed     string
	destroyed  []string
}

func (f *fakeSecrets) ListVersions(_ context.Context, parent string) ([]string, error) {
	f.listed = parent
	return append([]string(nil), f.versions...), nil
}
func (f *fakeSecrets) AddVersion(_ context.Context, parent string, value []byte) (string, error) {
	f.listed = parent
	f.added = append([]byte(nil), value...)
	if f.addErr != nil {
		return "", f.addErr
	}
	if f.addName != "" {
		return f.addName, nil
	}
	return "projects/447373903153/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions/9", nil
}
func (f *fakeSecrets) DestroyVersion(_ context.Context, name string) error {
	f.destroyed = append(f.destroyed, name)
	return f.destroyErr
}

type fakeSQL struct {
	instanceName  string
	instanceState string
	databaseName  string
	userName      string
	userType      string
	updateErr     error
	operation     string
	operationSeq  []struct {
		done, failed bool
		err          error
	}
	updated       []byte
	updateCalls   int
	operationCall int
}

func (f *fakeSQL) GetInstance(_ context.Context, project, instance string) (string, string, error) {
	if project != ProjectID || instance != InstanceName {
		return "", "", errors.New("wrong fixed instance request")
	}
	return f.instanceName, f.instanceState, nil
}
func (f *fakeSQL) GetDatabase(_ context.Context, project, instance, database string) (string, error) {
	if project != ProjectID || instance != InstanceName || database != DatabaseName {
		return "", errors.New("wrong fixed database request")
	}
	return f.databaseName, nil
}
func (f *fakeSQL) GetUser(_ context.Context, project, instance, user string) (string, string, error) {
	if project != ProjectID || instance != InstanceName || user != UserName {
		return "", "", errors.New("wrong fixed user request")
	}
	return f.userName, f.userType, nil
}
func (f *fakeSQL) UpdatePassword(_ context.Context, project, instance, user string, password []byte) (string, error) {
	f.updateCalls++
	if project != ProjectID || instance != InstanceName || user != UserName {
		return "", errors.New("wrong fixed password update request")
	}
	f.updated = append([]byte(nil), password...)
	return f.operation, f.updateErr
}
func (f *fakeSQL) GetOperation(_ context.Context, project, name string) (bool, bool, error) {
	if project != ProjectID || name != f.operation {
		return false, false, errors.New("wrong fixed operation request")
	}
	index := f.operationCall
	f.operationCall++
	if index >= len(f.operationSeq) {
		return false, false, nil
	}
	state := f.operationSeq[index]
	return state.done, state.failed, state.err
}

func configuredFakes() (*fakeSecrets, *fakeSQL) {
	return &fakeSecrets{}, &fakeSQL{
		instanceName: InstanceName, instanceState: "RUNNABLE", databaseName: DatabaseName,
		userName: UserName, userType: "BUILT_IN", operation: "operation-123",
		operationSeq: []struct {
			done, failed bool
			err          error
		}{{done: true}},
	}
}

func runProvisioner(s *fakeSecrets, sql *fakeSQL, random []byte, maxPolls int) (Result, error) {
	return (Provisioner{
		Secrets: s, SQL: sql, Random: bytes.NewReader(random),
		PollInterval: time.Nanosecond, MaxPolls: maxPolls,
	}).Provision(context.Background(), true, Confirmation)
}

func TestProvisionerStoresAndSetsSameURLSafe32BytePassword(t *testing.T) {
	secrets, sql := configuredFakes()
	random := make([]byte, 32)
	for i := range random {
		random[i] = byte(i)
	}
	result, err := runProvisioner(secrets, sql, random, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets.added) != 43 || len(sql.updated) != 43 || !bytes.Equal(secrets.added, sql.updated) {
		t.Fatal("Secret Manager and Cloud SQL did not receive the same 43-character password")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(secrets.added))
	if err != nil || len(decoded) != 32 || !bytes.Equal(decoded, random) {
		t.Fatal("generated password was not URL-safe Base64 of exactly 32 random bytes")
	}
	if result.SecretVersion != "projects/447373903153/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions/9" || result.Operation != "operation-123" || result.Reconcile {
		t.Fatalf("unexpected successful result: %+v", result)
	}
	if secrets.listed != SecretParent || len(secrets.destroyed) != 0 || sql.updateCalls != 1 || sql.operationCall != 1 {
		t.Fatal("provisioner did not use the single exact target and complete operation")
	}
}

func TestProvisionerRequiresExactApplyConfirmationAndFreshSecret(t *testing.T) {
	for _, tc := range []struct {
		name         string
		apply        bool
		confirmation string
		versions     []string
	}{
		{name: "no apply", apply: false, confirmation: Confirmation},
		{name: "wrong run", apply: true, confirmation: "SET-POSTGRES-PASSWORD:pilot01"},
		{name: "secret already has a version", apply: true, confirmation: Confirmation, versions: []string{"version-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secrets, sql := configuredFakes()
			secrets.versions = tc.versions
			_, err := (Provisioner{Secrets: secrets, SQL: sql}).Provision(context.Background(), tc.apply, tc.confirmation)
			if err == nil || len(secrets.added) != 0 || sql.updateCalls != 0 {
				t.Fatal("invalid or previously-used target was not rejected before mutation")
			}
		})
	}
}

func TestProvisionerRejectsMismatchedLiveTargetBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*fakeSQL)
	}{
		{name: "wrong instance", edit: func(s *fakeSQL) { s.instanceName = "espacigo-gcp03-pilot01-pg" }},
		{name: "wrong state", edit: func(s *fakeSQL) { s.instanceState = "PENDING_CREATE" }},
		{name: "wrong database", edit: func(s *fakeSQL) { s.databaseName = "postgres" }},
		{name: "wrong user", edit: func(s *fakeSQL) { s.userName = "migrator" }},
		{name: "wrong user type", edit: func(s *fakeSQL) { s.userType = "CLOUD_IAM_USER" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secrets, sql := configuredFakes()
			tc.edit(sql)
			_, err := runProvisioner(secrets, sql, make([]byte, 32), 3)
			if err == nil || len(secrets.added) != 0 || sql.updateCalls != 0 {
				t.Fatal("mismatched live resource target was not stopped before secret creation")
			}
		})
	}
}

func TestProvisionerSecretVersionFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name       string
		definitive bool
		reconcile  bool
	}{
		{name: "definitive rejection", definitive: true},
		{name: "uncertain response", reconcile: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secrets, sql := configuredFakes()
			secrets.addErr = testAPIError{definitive: tc.definitive}
			result, err := runProvisioner(secrets, sql, make([]byte, 32), 3)
			if err == nil || result.Reconcile != tc.reconcile || sql.updateCalls != 0 || len(secrets.destroyed) != 0 {
				t.Fatal("Secret Manager failure was retried, misclassified, or followed by a SQL update")
			}
		})
	}
}

func TestProvisionerSQLDefinitiveFailureDestroysOnlyCreatedVersion(t *testing.T) {
	secrets, sql := configuredFakes()
	sql.updateErr = testAPIError{definitive: true}
	result, err := runProvisioner(secrets, sql, make([]byte, 32), 3)
	if err == nil || result.Reconcile || len(secrets.destroyed) != 1 || secrets.destroyed[0] != resultVersionName {
		t.Fatal("definitive SQL rejection did not clean up only the version created by this attempt")
	}
}

func TestProvisionerUncertainSQLAndPollingFailuresRetainVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*fakeSecrets, *fakeSQL)
	}{
		{name: "uncertain update response", edit: func(_ *fakeSecrets, s *fakeSQL) { s.updateErr = testAPIError{} }},
		{name: "poll status uncertain", edit: func(_ *fakeSecrets, s *fakeSQL) {
			s.operationSeq = []struct {
				done, failed bool
				err          error
			}{{err: errors.New("private marker")}}
		}},
		{name: "finite poll exhausted", edit: func(_ *fakeSecrets, s *fakeSQL) { s.operationSeq = nil }},
		{name: "operation ID missing", edit: func(_ *fakeSecrets, s *fakeSQL) { s.operation = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secrets, sql := configuredFakes()
			tc.edit(secrets, sql)
			result, err := runProvisioner(secrets, sql, make([]byte, 32), 1)
			if err == nil || !result.Reconcile || result.SecretVersion == "" || len(secrets.destroyed) != 0 {
				t.Fatal("uncertain or partial operation did not retain its exact version for reconciliation")
			}
			if strings.Contains(err.Error(), "private marker") {
				t.Fatal("private API error detail escaped into the reconciliation result")
			}
		})
	}
}

func TestProvisionerCompletedFailedOperationRetainsVersionForPartialStateReconciliation(t *testing.T) {
	secrets, sql := configuredFakes()
	sql.operationSeq = []struct {
		done, failed bool
		err          error
	}{{done: true, failed: true}}
	result, err := runProvisioner(secrets, sql, make([]byte, 32), 3)
	if err == nil || !result.Reconcile || result.SecretVersion != resultVersionName || len(secrets.destroyed) != 0 {
		t.Fatal("completed failed SQL operation did not retain its version for uncertain partial-state reconciliation")
	}
}

func TestProvisionerCleanupFailureRetainsExactVersionForReconciliation(t *testing.T) {
	secrets, sql := configuredFakes()
	sql.updateErr = testAPIError{definitive: true}
	secrets.destroyErr = errors.New("private cleanup marker")
	result, err := runProvisioner(secrets, sql, make([]byte, 32), 3)
	if err == nil || !result.Reconcile || result.SecretVersion != resultVersionName || len(secrets.destroyed) != 1 {
		t.Fatal("failed exact-version cleanup did not return the new version for reconciliation")
	}
	if strings.Contains(err.Error(), "private cleanup marker") {
		t.Fatal("private cleanup error escaped the sanitized result")
	}
}

func TestProvisionerRejectsUnexpectedSecretVersionWithoutSQLMutation(t *testing.T) {
	secrets, sql := configuredFakes()
	secrets.addName = "projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password/versions/1"
	result, err := runProvisioner(secrets, sql, make([]byte, 32), 3)
	if err == nil || !result.Reconcile || sql.updateCalls != 0 || len(secrets.destroyed) != 0 {
		t.Fatal("unexpected secret response did not fail closed without touching another secret or SQL user")
	}
}

func TestSecretVersionNumberOnlyAcceptsTheFixedRunSecret(t *testing.T) {
	if got := SecretVersionNumber(resultVersionName); got != "9" {
		t.Fatalf("numeric secret version = %q, want 9", got)
	}
	for _, wrong := range []string{
		"projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password/versions/9",
		"projects/espacigo-testprod/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions/latest",
	} {
		if got := SecretVersionNumber(wrong); got != "" {
			t.Fatalf("foreign or mutable version %q produced numeric output %q", wrong, got)
		}
	}
}

const resultVersionName = "projects/447373903153/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions/9"
