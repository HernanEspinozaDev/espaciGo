package gcp04w9release

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fake struct {
	inv     Inventory
	calls   []string
	userErr error
	waitErr error
}

func (f *fake) ReadInstance(context.Context) (InstanceMetadata, error) {
	f.calls = append(f.calls, "instance")
	return f.inv.Instance, nil
}
func (f *fake) ListUsers(context.Context) ([]UserMetadata, error) {
	f.calls = append(f.calls, "users")
	return f.inv.Users, nil
}
func (f *fake) ReadSecret(context.Context) (SecretMetadata, error) {
	f.calls = append(f.calls, "secret")
	return f.inv.Secret, nil
}
func (f *fake) ListVersions(context.Context) ([]VersionMetadata, error) {
	f.calls = append(f.calls, "versions")
	return f.inv.Versions, nil
}
func (f *fake) CreateSecret(context.Context, string) error {
	f.calls = append(f.calls, "create-secret")
	return nil
}
func (f *fake) AddVersion(_ context.Context, _ string, p []byte) (string, error) {
	f.calls = append(f.calls, "add-version")
	if len(p) == 0 {
		return "", errors.New("empty")
	}
	return "7", nil
}
func (f *fake) CreateUser(_ context.Context, n string, r []string, p []byte) (string, error) {
	f.calls = append(f.calls, "create-user")
	if n != Login || !reflect.DeepEqual(r, []string{Role}) || len(p) == 0 {
		return "", errors.New("unsafe request")
	}
	return "op-1", f.userErr
}
func (f *fake) WaitOperation(context.Context, string) error {
	f.calls = append(f.calls, "wait")
	return f.waitErr
}
func (f *fake) DestroyVersion(context.Context, string) error {
	f.calls = append(f.calls, "destroy-version")
	return nil
}
func (f *fake) DeleteSecretIfOwned(context.Context, string, string) error {
	f.calls = append(f.calls, "delete-secret-owned")
	return nil
}

type journal struct{ records []JournalRecord }

func (j *journal) Save(x JournalRecord) error {
	if strings.Contains(strings.ToLower(x.Operation), "password") {
		return errors.New("secret in journal")
	}
	j.records = append(j.records, x)
	return nil
}
func fixture() Inventory {
	return Inventory{Instance: InstanceMetadata{Project: ProjectID, Name: Instance, Region: Region, Version: "POSTGRES_18", State: "RUNNABLE", Connection: Connection, IP: PrivateIP, Tier: "db-custom-N4-2-4096", DiskType: "HYPERDISK_BALANCED", DiskSizeGB: 20, Availability: "ZONAL", PrivateNetwork: "projects/espacigo-testprod/global/networks/espacigo-testprod", IPv4Enabled: false}, Secret: SecretMetadata{Name: SecretParent, Replication: "USER_MANAGED", Replicas: []string{Region}}}
}
func TestBuildPlanOnlyMetadataNoPayload(t *testing.T) {
	x := fixture()
	p, e := BuildPlan(x)
	if e != nil {
		t.Fatal(e)
	}
	if p.Mode != "dry_run" || p.Mutations != "none" || !reflect.DeepEqual(p.ProposedUser.DatabaseRoles, []string{Role}) {
		t.Fatalf("unexpected plan: %#v", p)
	}
	if strings.Contains(p.PayloadPolicy, "password=") {
		t.Fatal("payload value leaked")
	}
}
func TestBuildPlanFailsClosedOnCollisionOrDrift(t *testing.T) {
	x := fixture()
	x.Users = []UserMetadata{{Name: Login}}
	if _, e := BuildPlan(x); !errors.Is(e, ErrCollision) {
		t.Fatalf("collision: %v", e)
	}
	x = fixture()
	x.Versions = []VersionMetadata{{Name: "projects/p/secrets/s/versions/1", State: "ENABLED"}}
	if _, e := BuildPlan(x); e == nil {
		t.Fatal("version collision accepted")
	}
	x = fixture()
	x.Instance.IP = "10.0.0.1"
	if _, e := BuildPlan(x); e == nil {
		t.Fatal("drift accepted")
	}
}
func TestExecuteExactOrderAndNoCredentialJournal(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{}
	f := &fake{inv: inv}
	j := &journal{}
	r := Release{Decision: "approved", Stage: "experiment", ReleaseID: "release-1", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}
	v, e := Execute(context.Background(), f.inv, r, f, j, time.Now(), strings.NewReader(strings.Repeat("a", 32)))
	if e != nil {
		t.Fatal(e)
	}
	if v != "7" {
		t.Fatal(v)
	}
	want := []string{"create-secret", "add-version", "create-user", "wait"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls %#v", f.calls)
	}
	if len(j.records) != 7 || j.records[0].Phase != "secret_shell_create_started" || j.records[6].Phase != "complete" {
		t.Fatalf("journal %#v", j.records)
	}
}

func TestPrepareSecretShellOnlyCreatesLabeledShellAndNeverAddsVersionOrUser(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{}
	f := &fake{inv: inv}
	j := &journal{}
	r := approvedShellRelease("shell-1")
	if err := PrepareSecretShell(context.Background(), inv, r, f, j, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"create-secret"}) {
		t.Fatalf("shell stage performed unrelated mutations: %#v", f.calls)
	}
	if len(j.records) != 2 || j.records[0].Phase != "secret_shell_create_started" || j.records[1].Phase != "secret_shell_created" {
		t.Fatalf("unexpected shell journal %#v", j.records)
	}
}

func TestReleaseStagesCannotAuthorizeEachOther(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{}
	mutator := &fake{inv: inv}
	if _, err := Execute(context.Background(), inv, approvedShellRelease("stage-1"), mutator, &journal{}, time.Now(), nil); err == nil {
		t.Fatal("secret-shell approval authorized experiment execution")
	}
	mutator = &fake{inv: inv}
	if err := PrepareSecretShell(context.Background(), inv, approvedRelease("stage-2"), mutator, &journal{}, time.Now()); err == nil {
		t.Fatal("experiment approval authorized shell preparation")
	}
	if len(mutator.calls) != 0 {
		t.Fatalf("cross-stage approval mutated a resource: %#v", mutator.calls)
	}
}

func TestExecuteUsesOnlyReviewerPinnedStagedShell(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{Name: SecretParent, ETag: "etag-shell", Replication: "USER_MANAGED", Replicas: []string{Region}, Labels: map[string]string{"release_id": "staged-1", "purpose": "gcp04-w9-catalog-probe"}}
	f := &fake{inv: inv}
	r := approvedRelease("staged-1")
	r.SecretShellETag = "etag-shell"
	v, err := Execute(context.Background(), inv, r, f, &journal{}, time.Now(), strings.NewReader(strings.Repeat("e", 32)))
	if err != nil || v != "7" {
		t.Fatalf("staged shell execution result %q %v", v, err)
	}
	want := []string{"add-version", "create-user", "wait"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("unexpected staged-shell execution calls %#v", f.calls)
	}
	for _, tc := range []struct {
		name   string
		change func(*Inventory, *Release)
	}{
		{name: "etag", change: func(_ *Inventory, r *Release) { r.SecretShellETag = "stale-etag" }},
		{name: "release-label", change: func(i *Inventory, _ *Release) { i.Secret.Labels["release_id"] = "other-release" }},
		{name: "purpose-label", change: func(i *Inventory, _ *Release) { i.Secret.Labels["purpose"] = "other-purpose" }},
		{name: "version-collision", change: func(i *Inventory, _ *Release) {
			i.Versions = []VersionMetadata{{Name: SecretParent + "/versions/9", State: "ENABLED"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := inv
			i.Secret.Labels = map[string]string{"release_id": "staged-1", "purpose": "gcp04-w9-catalog-probe"}
			release := approvedRelease("staged-1")
			release.SecretShellETag = "etag-shell"
			tc.change(&i, &release)
			mutator := &fake{inv: i}
			if _, err := Execute(context.Background(), i, release, mutator, &journal{}, time.Now(), nil); err == nil {
				t.Fatal("unowned or changed staged shell accepted")
			}
			if len(mutator.calls) != 0 {
				t.Fatalf("staged-shell validation mutated resources: %#v", mutator.calls)
			}
		})
	}
}
func TestExecuteDefinitiveFailureCompensatesOnlyCreatedVersion(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{}
	f := &fake{inv: inv, userErr: ErrDefinitive}
	j := &journal{}
	r := Release{Decision: "approved", Stage: "experiment", ReleaseID: "r2", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}
	_, e := Execute(context.Background(), f.inv, r, f, j, time.Now(), strings.NewReader(strings.Repeat("b", 32)))
	if e == nil {
		t.Fatal("expected rejection")
	}
	if !reflect.DeepEqual(f.calls, []string{"create-secret", "add-version", "create-user", "destroy-version", "delete-secret-owned"}) {
		t.Fatalf("unsafe compensation sequence %#v", f.calls)
	}
}
func TestExecuteUncertainUserDoesNotCompensateOrRetry(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{}
	f := &fake{inv: inv, userErr: errors.New("uncertain")}
	j := &journal{}
	r := Release{Decision: "approved", Stage: "experiment", ReleaseID: "r3", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}
	_, e := Execute(context.Background(), f.inv, r, f, j, time.Now(), strings.NewReader(strings.Repeat("c", 32)))
	if e == nil {
		t.Fatal("expected uncertain")
	}
	if f.calls[len(f.calls)-1] != "create-user" {
		t.Fatalf("unexpected action after uncertain outcome %#v", f.calls)
	}
}
func TestExecuteRejectsExpiredOrUnapprovedRelease(t *testing.T) {
	f := &fake{inv: fixture()}
	for _, r := range []Release{{Decision: "requested", ReleaseID: "x", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}, {Decision: "approved", Stage: "experiment", ReleaseID: "x", Reviewer: "someone", ExpiresAt: time.Now().Add(time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}, {Decision: "approved", Stage: "experiment", ReleaseID: "x", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(-time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}} {
		if _, e := Execute(context.Background(), f.inv, r, f, &journal{}, time.Now(), nil); e == nil {
			t.Fatal("invalid release accepted")
		}
	}
}

func TestExecuteNeverTouchesPreexistingExactSecret(t *testing.T) {
	f := &fake{inv: fixture()}
	j := &journal{}
	r := approvedRelease("existing-secret")
	if _, err := Execute(context.Background(), f.inv, r, f, j, time.Now(), nil); !errors.Is(err, ErrCollision) {
		t.Fatalf("pre-existing shell must be treated as ownership collision: %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("pre-existing resource was touched: %#v", f.calls)
	}
}

func TestExecuteDefinitiveOperationFailureCompensatesSecret(t *testing.T) {
	inv := fixture()
	inv.Secret = SecretMetadata{}
	f := &fake{inv: inv, waitErr: ErrDefinitive}
	j := &journal{}
	_, err := Execute(context.Background(), f.inv, approvedRelease("op-failure"), f, j, time.Now(), strings.NewReader(strings.Repeat("d", 32)))
	if err == nil {
		t.Fatal("expected definitive operation error")
	}
	want := []string{"create-secret", "add-version", "create-user", "wait", "destroy-version", "delete-secret-owned"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("unexpected compensation path %#v", f.calls)
	}
}

func approvedRelease(id string) Release {
	return Release{Decision: "approved", Stage: "experiment", ReleaseID: id, Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(time.Minute), Target: ProjectID + "/" + Instance, Roles: []string{Role}, Experiment: "one_temporary_cloudsql_role_and_one_w8_probe", MaxDurationSeconds: 1800, Runs: 1, QuerySHA256: QuerySHA256}
}
func approvedShellRelease(id string) Release {
	r := approvedRelease(id)
	r.Stage = "secret_shell"
	return r
}
