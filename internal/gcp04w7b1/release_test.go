package gcp04w7b1

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testAPI struct {
	inv                         Inventory
	users                       []UserMetadata
	versions                    []VersionMetadata
	secret                      SecretMetadata
	calls                       []string
	role                        []string
	secretPayload, userPassword []byte
	failInsert                  bool
	insertConflict              bool
	failWait                    bool
	failDelete                  bool
	failNextSecretDelete        bool
	failOperationProof          bool
	slowCreateSecret            time.Duration
	ignoreCreateSecretDeadline  bool
	slowOperationRead           time.Duration
	destroyedETag               string
}

func fixtureInventory() Inventory                                         { return Inventory{Instance: expectedInstance()} }
func (f *testAPI) ReadInstance(context.Context) (InstanceMetadata, error) { return f.inv.Instance, nil }
func (f *testAPI) ListUsers(context.Context) ([]UserMetadata, error) {
	f.calls = append(f.calls, "list-users")
	return append([]UserMetadata(nil), f.users...), nil
}
func (f *testAPI) ReadSecret(context.Context) (SecretMetadata, error) { return f.secret, nil }
func (f *testAPI) ListVersions(context.Context) ([]VersionMetadata, error) {
	return append([]VersionMetadata(nil), f.versions...), nil
}
func (f *testAPI) CreateSecret(ctx context.Context, run string) (SecretMetadata, error) {
	f.calls = append(f.calls, "create-secret")
	if f.slowCreateSecret > 0 {
		timer := time.NewTimer(f.slowCreateSecret)
		defer timer.Stop()
		if f.ignoreCreateSecretDeadline {
			<-timer.C
		} else {
			select {
			case <-ctx.Done():
				return SecretMetadata{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	f.secret = SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": run, "purpose": "gcp04-w7-b1-bootstrap"}}
	return f.secret, nil
}
func (f *testAPI) AddVersion(_ context.Context, name string, p []byte) (VersionMetadata, error) {
	f.calls = append(f.calls, "add-version:"+name)
	f.secretPayload = append([]byte(nil), p...)
	v := VersionMetadata{Name: name + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}
	f.versions = []VersionMetadata{v}
	return v, nil
}
func (f *testAPI) InsertUser(_ context.Context, name, typ string, roles []string, p []byte) (string, error) {
	f.calls = append(f.calls, "insert-user")
	f.role = append([]string(nil), roles...)
	f.userPassword = append([]byte(nil), p...)
	if f.failInsert {
		if f.insertConflict {
			f.users = append(f.users, UserMetadata{Name: name, Type: typ, Roles: append([]string(nil), roles...)})
		}
		return "", errors.New("transport uncertain")
	}
	f.users = append(f.users, UserMetadata{Name: name, Type: typ, Roles: roles})
	return "operation-1", nil
}
func (f *testAPI) WaitOperation(context.Context, string) error {
	f.calls = append(f.calls, "wait")
	if f.failWait {
		return errors.New("deadline")
	}
	return nil
}
func (f *testAPI) ReadOperation(_ context.Context, id string) (OperationMetadata, error) {
	f.calls = append(f.calls, "read-operation:"+id)
	if f.slowOperationRead > 0 {
		time.Sleep(f.slowOperationRead)
	}
	if f.failOperationProof {
		return OperationMetadata{}, errors.New("not a CREATE_USER operation")
	}
	typ := "CREATE_USER"
	if id == "delete-op" {
		typ = "DELETE_USER"
	}
	return OperationMetadata{ID: id, Type: typ, TargetID: Instance, TargetProject: ProjectID, TargetLink: "https://sqladmin.googleapis.com/sql/v1beta4/projects/" + ProjectID + "/instances/" + Instance, Status: "DONE"}, nil
}
func (f *testAPI) DeleteUser(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, "delete-user:"+name)
	if f.failDelete {
		return "", errors.New("delete outcome unknown")
	}
	f.users = nil
	return "delete-op", nil
}
func (f *testAPI) DestroyVersion(_ context.Context, name, etag string) error {
	f.calls = append(f.calls, "destroy-version:"+name+":"+etag)
	for i := range f.versions {
		f.versions[i].State = "DESTROYED"
		if f.destroyedETag != "" {
			f.versions[i].ETag = f.destroyedETag
		}
	}
	return nil
}
func (f *testAPI) DeleteSecret(_ context.Context, name, release, etag string) error {
	f.calls = append(f.calls, "delete-secret:"+name+":"+release+":"+etag)
	if f.failNextSecretDelete {
		f.failNextSecretDelete = false
		return errors.New("secret delete outcome uncertain")
	}
	f.secret = SecretMetadata{}
	return nil
}

type testJournal struct{ records []JournalRecord }

func (j *testJournal) Save(r JournalRecord) error { j.records = append(j.records, r); return nil }
func (j *testJournal) ReadRecords(resource string) ([]JournalRecord, error) {
	var out []JournalRecord
	for _, r := range j.records {
		id := r.ResourceReleaseID
		if id == "" {
			id = r.ReleaseID
		}
		if id == resource {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	return out, nil
}
func approved(stage string) Release {
	id := "b1-run-1"
	if stage == "b1_cleanup" {
		id = "b1-cleanup-1"
	}
	return Release{Decision: "approved", Stage: stage, ReleaseID: id, ResourceReleaseID: "b1-run-1", CreateOperationID: "create-op-1", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(10 * time.Minute), Target: ProjectID + "/" + Instance, Login: Bootstrap, Role: AdminRole, SecretID: SecretID, QuerySHA256: QuerySHA256, Runs: 1, MaxDurationSeconds: 30, SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag", CreatedVersionETag: "version-etag"}
}

func seedCreateProof(j *testJournal) {
	j.records = append(j.records, JournalRecord{ReleaseID: "b1-run-1", ResourceReleaseID: "b1-run-1", Stage: "b1_create", Phase: "create_complete", Target: ProjectID + "/" + Instance, Login: Bootstrap, Secret: secretParent(), SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag", Operation: "create-op-1", OperationType: "CREATE_USER", OperationTarget: Instance, OperationUser: Bootstrap})
}

func TestDefaultPlanIsOfflineAndExplicitAdminRole(t *testing.T) {
	p := StaticPlan()
	if p.Mode != "default_off_preview" || p.Mutations != "none" || !reflect.DeepEqual(p.UserRequest.Roles, []string{AdminRole}) {
		t.Fatalf("unsafe default plan: %#v", p)
	}
}

func TestCreateB1SingleInsertAndMemoryOnlyCredential(t *testing.T) {
	f := &testAPI{inv: fixtureInventory()}
	j := &testJournal{}
	dir := t.TempDir()
	version, err := CreateB1(context.Background(), f.inv, approved("b1_create"), f, f, j, dir, time.Now(), strings.NewReader(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatal(err)
	}
	if version != "7" {
		t.Fatalf("version=%q", version)
	}
	operationID, err := CreateOperationIDFromJournal(j.records, "b1-run-1")
	if err != nil || operationID != "operation-1" {
		t.Fatalf("create operation proof id=%q err=%v", operationID, err)
	}
	if !reflect.DeepEqual(f.role, []string{AdminRole}) {
		t.Fatalf("role fallback/omission: %v", f.role)
	}
	if !reflect.DeepEqual(f.secretPayload, f.userPassword) || len(f.userPassword) == 0 {
		t.Fatal("RAM credential payload mismatch")
	}
	if got := count(f.calls, "insert-user"); got != 1 {
		t.Fatalf("insert calls=%d", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "b1-run-1-create.lease")); err != nil {
		t.Fatalf("successful single-run lease was not retained: %v", err)
	}
	for _, r := range j.records {
		if strings.Contains(string(mustJSON(r)), string(f.userPassword)) {
			t.Fatal("password leaked to journal")
		}
	}
	if _, err := CreateB1(context.Background(), f.inv, approved("b1_create"), f, f, j, dir, time.Now(), nil); !errors.Is(err, ErrLease) {
		t.Fatalf("same release rerun not blocked by lease: %v", err)
	}
}

func TestCreateUncertainOutcomeDoesNotRetryOrUnlockLease(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), failInsert: true}
	_, err := CreateB1(context.Background(), f.inv, approved("b1_create"), f, f, &testJournal{}, t.TempDir(), time.Now(), strings.NewReader(strings.Repeat("y", 32)))
	if !errors.Is(err, ErrUncertain) {
		t.Fatalf("expected uncertain result: %v", err)
	}
	if count(f.calls, "insert-user") != 1 {
		t.Fatalf("insert retried: %v", f.calls)
	}
	if len(f.versions) != 1 || f.versions[0].State != "ENABLED" {
		t.Fatal("uncertain user insert triggered automatic secret compensation")
	}
}

func TestInsertConflictNeverCreatesOwnUserProofOrAllowsCleanupDeletion(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), failInsert: true, insertConflict: true}
	j := &testJournal{}
	_, err := CreateB1(context.Background(), f.inv, approved("b1_create"), f, f, j, t.TempDir(), time.Now(), strings.NewReader(strings.Repeat("c", 32)))
	if !errors.Is(err, ErrUncertain) || count(f.calls, "insert-user") != 1 {
		t.Fatalf("expected one uncertain insert attempt: err=%v calls=%v", err, f.calls)
	}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	err = CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now())
	if err == nil || count(f.calls, "delete-user:"+Bootstrap) != 0 {
		t.Fatalf("409-like same-name user was adopted/deleted without own operation proof: err=%v calls=%v", err, f.calls)
	}
}

func TestCreateStopsBeforeNextMutationWhenReleaseExpiresDuringFirstMutation(t *testing.T) {
	now := time.Now()
	r := approved("b1_create")
	r.ExpiresAt = now.Add(25 * time.Millisecond)
	r.MaxDurationSeconds = 30
	f := &testAPI{inv: fixtureInventory(), slowCreateSecret: 40 * time.Millisecond, ignoreCreateSecretDeadline: true}
	_, err := CreateB1(context.Background(), f.inv, r, f, f, &testJournal{}, t.TempDir(), now, strings.NewReader(strings.Repeat("z", 32)))
	if !errors.Is(err, ErrUncertain) || count(f.calls, "add-version:"+secretParent()) != 0 || count(f.calls, "insert-user") != 0 {
		t.Fatalf("expired stage advanced to another mutation: err=%v calls=%v", err, f.calls)
	}
}

func TestCreateCollisionAndExpiredReleaseFailBeforeWrites(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Inventory, *Release)
	}{{"user collision", func(i *Inventory, _ *Release) { i.Users = []UserMetadata{{Name: Bootstrap}} }}, {"secret collision", func(i *Inventory, _ *Release) { i.Secret = SecretMetadata{Name: secretParent()} }}, {"expired", func(_ *Inventory, r *Release) { r.ExpiresAt = time.Now().Add(-time.Second) }}, {"empty role", func(_ *Inventory, r *Release) { r.Role = "" }}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &testAPI{inv: fixtureInventory()}
			r := approved("b1_create")
			tc.mutate(&f.inv, &r)
			_, err := CreateB1(context.Background(), f.inv, r, f, f, &testJournal{}, t.TempDir(), time.Now(), nil)
			if err == nil || len(f.calls) != 0 {
				t.Fatalf("unsafe gate: err=%v calls=%v", err, f.calls)
			}
		})
	}
}

func TestCleanupDeletesExactUserThenConfirmsAbsenceBeforeSecret(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	if err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	want := []string{"read-operation:create-op-1", "delete-user:" + Bootstrap, "wait", "read-operation:delete-op", "list-users", "destroy-version:" + secretParent() + "/versions/7:version-etag", "delete-secret:" + secretParent() + ":b1-run-1:secret-etag"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("cleanup order %v want %v", f.calls, want)
	}
}

func TestCleanupCollisionNeverDeletesUser(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole, "other"}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now())
	if err == nil || count(f.calls, "delete-user:"+Bootstrap) != 0 {
		t.Fatalf("collision cleanup proceeded: %v %v", err, f.calls)
	}
}

func TestCleanupDoesNotAdoptSameNameAndRoleWithoutItsOwnCreateOperation(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, &testJournal{}, t.TempDir(), time.Now())
	if err == nil || count(f.calls, "delete-user:"+Bootstrap) != 0 {
		t.Fatalf("same-name collision without a completed own CREATE_USER proof was deleted: %v %v", err, f.calls)
	}
}

func TestCleanupRequiresFreshExactCreateUserOperationMetadata(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}, failOperationProof: true}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now())
	if err == nil || count(f.calls, "delete-user:"+Bootstrap) != 0 {
		t.Fatalf("cleanup accepted a mismatched API operation proof: %v %v", err, f.calls)
	}
}

func TestCleanupStopsBeforeDeleteWhenReleaseExpiresDuringProofRead(t *testing.T) {
	now := time.Now()
	r := approved("b1_cleanup")
	r.ExpiresAt = now.Add(20 * time.Millisecond)
	r.MaxDurationSeconds = 30
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}, slowOperationRead: 35 * time.Millisecond}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	err := CleanupB1(context.Background(), f.inv, r, f, f, f, j, t.TempDir(), now)
	if err == nil || count(f.calls, "delete-user:"+Bootstrap) != 0 || count(f.calls, "destroy-version:") != 0 {
		t.Fatalf("expired cleanup stage performed a mutation: err=%v calls=%v", err, f.calls)
	}
}

func TestExpiredLeaseRequiresReconciliationAndIsNeverStolen(t *testing.T) {
	dir := t.TempDir()
	if err := AcquireLease(dir, "lease-run-create", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := AcquireLease(dir, "lease-run-create", time.Now()); !errors.Is(err, ErrLease) {
		t.Fatalf("expired lease was automatically stolen: %v", err)
	}
}

func TestAppendJournalReadsExactPrivateResourceProof(t *testing.T) {
	dir := t.TempDir()
	j := AppendJournal{Dir: filepath.Join(dir, "journal")}
	r := JournalRecord{ReleaseID: "b1-run-1", ResourceReleaseID: "b1-run-1", Stage: "b1_create", Phase: "create_complete", Target: ProjectID + "/" + Instance, Login: Bootstrap, Secret: secretParent(), SecretETag: "s-etag", Version: "7", VersionETag: "v-etag", Operation: "create-op-1", OperationType: "CREATE_USER", OperationTarget: Instance, OperationUser: Bootstrap}
	if err := j.Save(r); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "journal", "b1-run-1.jsonl"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("journal file is not private: info=%v err=%v", info, err)
	}
	if info, err = os.Stat(filepath.Join(dir, "journal")); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("journal directory is not private: info=%v err=%v", info, err)
	}
	got, err := j.ReadRecords("b1-run-1")
	if err != nil || len(got) != 1 || got[0].Operation != "create-op-1" || got[0].OperationType != "CREATE_USER" || got[0].OperationTarget != Instance {
		t.Fatalf("journal proof %#v err=%v", got, err)
	}
}

func TestCleanupUncertainDeleteNeverDestroysCredential(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}, failDelete: true}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now())
	if !errors.Is(err, ErrUncertain) || count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag") != 0 {
		t.Fatalf("uncertain user delete proceeded to secret destruction: %v %v", err, f.calls)
	}
}

func TestCleanupResumesAlreadyDestroyedVersionWithoutDestroyingAgain(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "DESTROYED", ETag: "version-etag"}}}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	j.records = append(j.records, JournalRecord{ReleaseID: "b1-cleanup-old", ResourceReleaseID: "b1-run-1", Phase: "version_destroyed", SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag"})
	if err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag") != 0 {
		t.Fatalf("DESTROYED version was destroyed again: %v", f.calls)
	}
}

func TestCleanupResumesDisabledVersionWithSingleDestroy(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "DISABLED", ETag: "version-etag"}}}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	if err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag") != 1 {
		t.Fatalf("DISABLED version was not destroyed exactly once: %v", f.calls)
	}
}

func TestCleanupResumesAfterDestroyETagChangesWithoutRepeatingDestroy(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}, destroyedETag: "version-etag-destroyed", failNextSecretDelete: true}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	j := &testJournal{}
	seedCreateProof(j)
	first := approved("b1_cleanup")
	if err := CleanupB1(context.Background(), f.inv, first, f, f, f, j, t.TempDir(), time.Now()); !errors.Is(err, ErrUncertain) {
		t.Fatalf("expected shell delete uncertainty after destroy: %v", err)
	}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	second := approved("b1_cleanup")
	second.ReleaseID = "b1-cleanup-2"
	second.VersionETag = "version-etag-destroyed"
	if err := CleanupB1(context.Background(), f.inv, second, f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag") != 1 || count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag-destroyed") != 0 {
		t.Fatalf("resume repeated destroy or missed first destroy: %v", f.calls)
	}
}

func TestCleanupResumesEmptyShellWithPriorVersionProof(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}}
	f.inv.Secret = f.secret
	j := &testJournal{}
	seedCreateProof(j)
	j.records = append(j.records, JournalRecord{ReleaseID: "b1-cleanup-old", ResourceReleaseID: "b1-run-1", Phase: "version_destroyed", SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag"})
	if err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag") != 0 {
		t.Fatalf("empty shell caused repeated destroy: %v", f.calls)
	}
}

func TestCleanupResumesAbsentShellOnlyWithExactPriorETagProof(t *testing.T) {
	f := &testAPI{inv: fixtureInventory()}
	j := &testJournal{}
	seedCreateProof(j)
	j.records = append(j.records,
		JournalRecord{ReleaseID: "b1-cleanup-old", ResourceReleaseID: "b1-run-1", Phase: "version_destroyed", Secret: secretParent(), SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag"},
		JournalRecord{ReleaseID: "b1-cleanup-old", ResourceReleaseID: "b1-run-1", Phase: "secret_shell_delete_started", Secret: secretParent(), SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag"},
	)
	if err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if count(f.calls, "delete-secret:") != 0 || count(f.calls, "destroy-version:") != 0 || count(f.calls, "delete-user:") != 0 {
		t.Fatalf("absent resource cleanup issued a delete: %v", f.calls)
	}
}

func TestCleanupAbsentShellWithoutOwnershipETagProofFails(t *testing.T) {
	f := &testAPI{inv: fixtureInventory()}
	j := &testJournal{}
	seedCreateProof(j)
	j.records = append(j.records, JournalRecord{ReleaseID: "b1-cleanup-old", ResourceReleaseID: "b1-run-1", Phase: "version_destroyed", Secret: secretParent(), SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag"})
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now())
	if err == nil || count(f.calls, "delete-secret:") != 0 {
		t.Fatalf("absent shell without prior exact delete attempt was adopted: %v %v", err, f.calls)
	}
}

func TestStageDeadlineIsBoundedByReleaseExpiry(t *testing.T) {
	now := time.Now()
	r := approved("b1_create")
	r.ExpiresAt = now.Add(25 * time.Millisecond)
	r.MaxDurationSeconds = 30
	ctx, cancel := stageContext(context.Background(), r, now)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || deadline.After(r.ExpiresAt) || deadline.Sub(now) > 25*time.Millisecond {
		t.Fatalf("stage context exceeded release expiry: deadline=%v release=%v", deadline, r.ExpiresAt)
	}
}

func TestOperationProofPinsIDTypeProjectAndInstance(t *testing.T) {
	op := OperationMetadata{ID: "create-op-1", Type: "CREATE_USER", TargetID: Instance, TargetProject: ProjectID, TargetLink: "https://sqladmin.googleapis.com/sql/v1beta4/projects/" + ProjectID + "/instances/" + Instance, Status: "DONE"}
	if !validOperation(op, "create-op-1", "CREATE_USER") {
		t.Fatal("valid exact CREATE_USER operation rejected")
	}
	for name, mutate := range map[string]func(*OperationMetadata){
		"wrong operation ID":    func(o *OperationMetadata) { o.ID = "other" },
		"wrong operation type":  func(o *OperationMetadata) { o.Type = "UPDATE_USER" },
		"wrong target instance": func(o *OperationMetadata) { o.TargetID = "other-instance" },
		"wrong project":         func(o *OperationMetadata) { o.TargetProject = "other-project" },
		"wrong target link": func(o *OperationMetadata) {
			o.TargetLink = "https://sqladmin.googleapis.com/sql/v1beta4/projects/other/instances/other"
		},
		"not done":   func(o *OperationMetadata) { o.Status = "RUNNING" },
		"has errors": func(o *OperationMetadata) { o.HasErrors = true },
	} {
		t.Run(name, func(t *testing.T) {
			bad := op
			mutate(&bad)
			if validOperation(bad, "create-op-1", "CREATE_USER") {
				t.Fatalf("invalid operation metadata accepted: %#v", bad)
			}
		})
	}
}

func TestPlanDoesNotAdoptPostgresOrObserver(t *testing.T) {
	x := fixtureInventory()
	x.Users = []UserMetadata{{Name: "postgres", Type: "BUILT_IN"}, {Name: Observer, Type: "BUILT_IN", Roles: []string{ObserverRole}}}
	p, err := BuildPlan(x)
	if err != nil {
		t.Fatalf("unrelated identities should not be adopted or reset: %v", err)
	}
	if p.Login != Bootstrap || len(p.UserRequest.Roles) != 1 || p.UserRequest.Roles[0] != AdminRole {
		t.Fatalf("plan did not retain exact new identity and explicit role: %#v", p)
	}
}

func TestReconcileRejectsUnexpectedAdditionalSecretVersion(t *testing.T) {
	x := fixtureInventory()
	x.Users = []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}
	x.Secret = SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}
	x.Versions = []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}, {Name: secretParent() + "/versions/8", Number: "8", State: "ENABLED", ETag: "version-etag-2"}}
	got, err := ReconcileInventory(x, "b1-run-1", "7")
	if err != nil {
		t.Fatal(err)
	}
	if got["status"] != "incomplete_or_mixed_state" {
		t.Fatalf("unexpected extra version was accepted: %#v", got)
	}
}

func count(xs []string, want string) int {
	n := 0
	for _, x := range xs {
		if x == want {
			n++
		}
	}
	return n
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
