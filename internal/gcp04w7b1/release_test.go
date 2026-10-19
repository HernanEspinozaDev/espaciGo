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
	failWait                    bool
	failDelete                  bool
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
func (f *testAPI) CreateSecret(_ context.Context, run string) (SecretMetadata, error) {
	f.calls = append(f.calls, "create-secret")
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
	}
	return nil
}
func (f *testAPI) DeleteSecret(_ context.Context, name, release, etag string) error {
	f.calls = append(f.calls, "delete-secret:"+name+":"+release+":"+etag)
	f.secret = SecretMetadata{}
	return nil
}

type testJournal struct{ records []JournalRecord }

func (j *testJournal) Save(r JournalRecord) error { j.records = append(j.records, r); return nil }
func approved(stage string) Release {
	return Release{Decision: "approved", Stage: stage, ReleaseID: "b1-run-1", ResourceReleaseID: "b1-run-1", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(10 * time.Minute), Target: ProjectID + "/" + Instance, Login: Bootstrap, Role: AdminRole, SecretID: SecretID, QuerySHA256: QuerySHA256, Runs: 1, MaxDurationSeconds: 30, SecretETag: "secret-etag", Version: "7", VersionETag: "version-etag"}
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
	if err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, j, t.TempDir(), time.Now()); err != nil {
		t.Fatal(err)
	}
	want := []string{"delete-user:" + Bootstrap, "wait", "list-users", "destroy-version:" + secretParent() + "/versions/7:version-etag", "delete-secret:" + secretParent() + ":b1-run-1:secret-etag"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("cleanup order %v want %v", f.calls, want)
	}
}

func TestCleanupCollisionNeverDeletesUser(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole, "other"}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, &testJournal{}, t.TempDir(), time.Now())
	if err == nil || count(f.calls, "delete-user:"+Bootstrap) != 0 {
		t.Fatalf("collision cleanup proceeded: %v %v", err, f.calls)
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

func TestCleanupUncertainDeleteNeverDestroysCredential(t *testing.T) {
	f := &testAPI{inv: fixtureInventory(), users: []UserMetadata{{Name: Bootstrap, Type: "BUILT_IN", Roles: []string{AdminRole}}}, secret: SecretMetadata{Name: secretParent(), ETag: "secret-etag", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "b1-run-1", "purpose": "gcp04-w7-b1-bootstrap"}}, versions: []VersionMetadata{{Name: secretParent() + "/versions/7", Number: "7", State: "ENABLED", ETag: "version-etag"}}, failDelete: true}
	f.inv.Users, f.inv.Secret, f.inv.Versions = f.users, f.secret, f.versions
	err := CleanupB1(context.Background(), f.inv, approved("b1_cleanup"), f, f, f, &testJournal{}, t.TempDir(), time.Now())
	if !errors.Is(err, ErrUncertain) || count(f.calls, "destroy-version:"+secretParent()+"/versions/7:version-etag") != 0 {
		t.Fatalf("uncertain user delete proceeded to secret destruction: %v %v", err, f.calls)
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
