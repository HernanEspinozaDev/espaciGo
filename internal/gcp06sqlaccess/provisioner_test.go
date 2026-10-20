package gcp06sqlaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeAPIError struct{ definitive bool }

func (e fakeAPIError) Error() string    { return "sanitized fake Cloud API error" }
func (e fakeAPIError) Definitive() bool { return e.definitive }

type fakeSecrets struct {
	exists       bool
	destroyTTL   string
	versions     []Version
	addName      string
	addErr       error
	getErr       error
	destroyErr   error
	addedPayload []byte
	destroyed    []string
	getVersion   string
	getCalls     []string
	payloadLogin string
}

func (f *fakeSecrets) GetSecret(_ context.Context, parent string) (bool, string, error) {
	if parent != SecretParent {
		return false, "", errors.New("wrong secret parent")
	}
	return f.exists, f.destroyTTL, nil
}
func (f *fakeSecrets) ListVersions(_ context.Context, parent string) ([]Version, error) {
	if parent != SecretParent {
		return nil, errors.New("wrong secret parent")
	}
	return append([]Version(nil), f.versions...), nil
}
func (f *fakeSecrets) AddVersion(_ context.Context, parent string, payload []byte) (string, error) {
	if parent != SecretParent {
		return "", errors.New("wrong secret parent")
	}
	f.addedPayload = append([]byte(nil), payload...)
	var record struct {
		Username string `json:"username"`
	}
	_ = json.Unmarshal(payload, &record)
	f.payloadLogin = record.Username
	if f.addErr != nil {
		return "", f.addErr
	}
	name := f.addName
	if name == "" {
		name = SecretParent + "/versions/1"
	}
	f.versions = append(f.versions, Version{Name: name, State: "ENABLED"})
	return name, nil
}
func (f *fakeSecrets) GetVersion(_ context.Context, name string) (string, error) {
	f.getCalls = append(f.getCalls, name)
	if f.getErr != nil {
		return "", f.getErr
	}
	if name == SecretParent+"/versions/1" && f.getVersion != "" {
		return f.getVersion, nil
	}
	for _, version := range f.versions {
		if version.Name == name {
			return version.State, nil
		}
	}
	return "", errors.New("version absent")
}
func (f *fakeSecrets) ReadCredentialLogin(_ context.Context, name string) (string, error) {
	if !validVersionName(name) || f.payloadLogin == "" {
		return "", errors.New("credential unavailable")
	}
	return f.payloadLogin, nil
}
func (f *fakeSecrets) DestroyVersion(_ context.Context, name string) error {
	f.destroyed = append(f.destroyed, name)
	if f.destroyErr != nil {
		return f.destroyErr
	}
	for i := range f.versions {
		if f.versions[i].Name == name {
			f.versions[i].State = "DESTROYED"
		}
	}
	return nil
}

type fakeSQL struct {
	instanceName, instanceState, dbVersion, region string
	privateIP                                      bool
	databaseName                                   string
	users                                          map[string]string
	userGetCalls, insertCalls, deleteCalls         []string
	insertErr, deleteErr, operationErr             error
	operationID                                    string
	done, failed                                   bool
	operationCalls                                 int
}

func (f *fakeSQL) GetInstance(_ context.Context, project, instance string) (string, string, string, string, bool, error) {
	if project != ProjectID || instance != InstanceName {
		return "", "", "", "", false, errors.New("wrong instance")
	}
	return f.instanceName, f.instanceState, f.dbVersion, f.region, f.privateIP, nil
}
func (f *fakeSQL) GetDatabase(_ context.Context, project, instance, database string) (string, error) {
	if project != ProjectID || instance != InstanceName || database != DatabaseName {
		return "", errors.New("wrong database")
	}
	return f.databaseName, nil
}
func (f *fakeSQL) GetUser(_ context.Context, project, instance, username string) (bool, string, error) {
	if project != ProjectID || instance != InstanceName || !inspectorLoginPattern.MatchString(username) {
		return false, "", errors.New("wrong user target")
	}
	f.userGetCalls = append(f.userGetCalls, username)
	typ, ok := f.users[username]
	return ok, typ, nil
}
func (f *fakeSQL) InsertUser(_ context.Context, project, instance, username string, password []byte) (string, error) {
	if project != ProjectID || instance != InstanceName || !inspectorLoginPattern.MatchString(username) || len(password) != 43 {
		return "", errors.New("wrong insert target")
	}
	f.insertCalls = append(f.insertCalls, username)
	if f.insertErr != nil {
		return "", f.insertErr
	}
	f.users[username] = "BUILT_IN"
	return f.operationID, nil
}
func (f *fakeSQL) DeleteUser(_ context.Context, project, instance, username string) (string, error) {
	if project != ProjectID || instance != InstanceName || !inspectorLoginPattern.MatchString(username) {
		return "", errors.New("wrong delete target")
	}
	f.deleteCalls = append(f.deleteCalls, username)
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	delete(f.users, username)
	return f.operationID, nil
}
func (f *fakeSQL) GetOperation(_ context.Context, project, operation string) (bool, bool, error) {
	if project != ProjectID || operation != f.operationID {
		return false, false, errors.New("wrong operation")
	}
	f.operationCalls++
	return f.done, f.failed, f.operationErr
}

func configuredFakes() (*fakeSecrets, *fakeSQL) {
	return &fakeSecrets{exists: true}, &fakeSQL{
		instanceName: InstanceName, instanceState: "RUNNABLE", dbVersion: "POSTGRES_18", region: ExpectedRegion,
		privateIP: true, databaseName: DatabaseName, users: map[string]string{}, operationID: "operation-123", done: true,
	}
}

func runPrepare(secrets *fakeSecrets, sql *fakeSQL) (Result, error) {
	random := make([]byte, 38)
	for i := range random {
		random[i] = byte(i)
	}
	return (Provisioner{Secrets: secrets, SQL: sql, Random: bytes.NewReader(random), PollInterval: time.Nanosecond, MaxPolls: 1}).Prepare(context.Background(), PrepareConfirm)
}

func TestPrepareCreatesRandomOneRunCredentialAtPinnedCandidate(t *testing.T) {
	secrets, sql := configuredFakes()
	result, err := runPrepare(secrets, sql)
	if err != nil {
		t.Fatal(err)
	}
	if result.Login != "gcp06_inspect_000102030405" || result.SecretVersion != SecretParent+"/versions/1" || result.Reconcile || len(sql.insertCalls) != 1 || sql.operationCalls != 1 {
		t.Fatalf("unexpected one-run preparation result: %+v", result)
	}
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(secrets.addedPayload, &payload); err != nil || payload.Username != result.Login || len(payload.Password) != 43 {
		t.Fatal("Secret Manager payload did not contain the generated one-run credential")
	}
	if !strings.Contains(string(secrets.addedPayload), `"username"`) {
		t.Fatal("credential bundle is not a JSON login/password object")
	}
}

func TestPrepareRejectsAnyNonCandidateOrPreexistingSecretBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*fakeSecrets, *fakeSQL)
	}{
		{name: "secret absent", edit: func(s *fakeSecrets, _ *fakeSQL) { s.exists = false }},
		{name: "delayed destroy", edit: func(s *fakeSecrets, _ *fakeSQL) { s.destroyTTL = "86400s" }},
		{name: "secret version exists", edit: func(s *fakeSecrets, _ *fakeSQL) {
			s.versions = []Version{{Name: SecretParent + "/versions/1", State: "DISABLED"}}
		}},
		{name: "wrong database", edit: func(_ *fakeSecrets, s *fakeSQL) { s.databaseName = "postgres" }},
		{name: "public candidate", edit: func(_ *fakeSecrets, s *fakeSQL) { s.privateIP = false }},
		{name: "wrong postgres version", edit: func(_ *fakeSecrets, s *fakeSQL) { s.dbVersion = "POSTGRES_17" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secrets, sql := configuredFakes()
			tc.edit(secrets, sql)
			_, err := runPrepare(secrets, sql)
			if err == nil || len(secrets.addedPayload) != 0 || len(sql.insertCalls) != 0 {
				t.Fatal("invalid candidate did not stop before mutation")
			}
		})
	}
}

func TestPrepareDoesNotRetryUncertainUserCreate(t *testing.T) {
	secrets, sql := configuredFakes()
	sql.insertErr = fakeAPIError{definitive: false}
	result, err := runPrepare(secrets, sql)
	if err == nil || !result.Reconcile || result.Login == "" || result.SecretVersion != SecretParent+"/versions/1" {
		t.Fatalf("uncertain SQL create did not return exact reconciliation identifiers: %+v", result)
	}
	if len(sql.insertCalls) != 1 || sql.operationCalls != 0 {
		t.Fatal("uncertain SQL users.insert was retried or polled without a confirmed operation")
	}
}

func TestPrepareRejectsUncertainSecretAddWithoutCreatingSQLUser(t *testing.T) {
	secrets, sql := configuredFakes()
	secrets.addErr = fakeAPIError{definitive: false}
	result, err := runPrepare(secrets, sql)
	if err == nil || !result.Reconcile || result.Login == "" || len(sql.insertCalls) != 0 || !strings.Contains(err.Error(), "do not retry") {
		t.Fatal("uncertain secret add was retried or followed by SQL user creation")
	}
}

func TestCleanupDeletesOnlyExactCandidateLoginAndVersion(t *testing.T) {
	secrets, sql := configuredFakes()
	login := "gcp06_inspect_012345abcdef"
	version := SecretParent + "/versions/7"
	sql.users[login] = "BUILT_IN"
	secrets.versions = []Version{{Name: version, State: "ENABLED"}}
	secrets.payloadLogin = login
	success := (Provisioner{Secrets: secrets, SQL: sql, PollInterval: time.Nanosecond, MaxPolls: 1})

	for _, tc := range []struct {
		name, user, version, confirmation string
	}{
		{"wrong candidate login", "gcp06_inspect_012345abcdee", version, CleanupConfirmation("gcp06_inspect_012345abcdee", version)},
		{"wrong secret", login, "projects/espacigo-testprod/secrets/another-secret/versions/7", "GCP06 SQL INSPECT CLEANUP " + RunID + " " + login + " 7"},
		{"wrong confirmation", login, version, "wrong"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := success.Cleanup(context.Background(), tc.user, tc.version, tc.confirmation); err == nil {
				t.Fatal("non-exact cleanup request was accepted")
			}
		})
	}
	if len(sql.deleteCalls) != 0 || len(secrets.destroyed) != 0 {
		t.Fatal("rejected cleanup request mutated a Cloud resource")
	}

	result, err := success.Cleanup(context.Background(), login, version, CleanupConfirmation(login, version))
	if err != nil {
		t.Fatal(err)
	}
	if !result.UserDeleted || !result.SecretDeleted || result.Reconcile || len(sql.deleteCalls) != 1 || sql.deleteCalls[0] != login || len(secrets.destroyed) != 1 || secrets.destroyed[0] != version {
		t.Fatalf("exact cleanup or deletion verification failed: %+v", result)
	}
}

func TestCleanupDoesNotRetryUncertainDeleteOrDestroyWrongVersion(t *testing.T) {
	secrets, sql := configuredFakes()
	login := "gcp06_inspect_012345abcdef"
	version := SecretParent + "/versions/3"
	sql.users[login] = "BUILT_IN"
	secrets.versions = []Version{{Name: version, State: "ENABLED"}}
	secrets.payloadLogin = login
	sql.deleteErr = fakeAPIError{definitive: false}
	provisioner := Provisioner{Secrets: secrets, SQL: sql}
	result, err := provisioner.Cleanup(context.Background(), login, version, CleanupConfirmation(login, version))
	if err == nil || !result.Reconcile || len(sql.deleteCalls) != 1 || len(secrets.destroyed) != 0 {
		t.Fatal("uncertain user delete was retried or secret was destroyed before user deletion was verified")
	}
}

func TestCleanupVerifiesAlreadyDeletedUserBeforeDestroyingExactSecret(t *testing.T) {
	secrets, sql := configuredFakes()
	login := "gcp06_inspect_012345abcdef"
	version := SecretParent + "/versions/2"
	secrets.versions = []Version{{Name: version, State: "ENABLED"}}
	secrets.payloadLogin = login
	result, err := (Provisioner{Secrets: secrets, SQL: sql}).Cleanup(context.Background(), login, version, CleanupConfirmation(login, version))
	if err != nil || !result.UserDeleted || !result.SecretDeleted || len(sql.deleteCalls) != 0 || len(secrets.destroyed) != 1 || secrets.destroyed[0] != version {
		t.Fatal("cleanup did not verify absent login before destroying only the exact secret version")
	}
}

func TestCanonicalSecretManagerProjectNumberNames(t *testing.T) {
	canonical := SecretCanonicalParent + "/versions/123"
	if !validVersionName(canonical) || VersionNumber(canonical) != "123" {
		t.Fatal("canonical Secret Manager project-number resource name was rejected")
	}
	if got := CanonicalVersion("123"); got != canonical {
		t.Fatalf("numeric version did not canonicalize to the project-number resource name: %q", got)
	}
	if validVersionName("projects/999999999999/secrets/espacigo-test-gcp06-sql-inspector-credential/versions/123") {
		t.Fatal("version from a different project was accepted")
	}
}
