package gcp04w8release

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type mockAdmin struct {
	instance Instance
	users    []User
	secret   bool
	versions []string
	err      error
	reads    []string
}

func (m *mockAdmin) ReadInstance(_ context.Context, project, name string) (Instance, error) {
	m.reads = append(m.reads, "instance:"+project+":"+name)
	return m.instance, m.err
}
func (m *mockAdmin) ListUsers(_ context.Context, project, name string) ([]User, error) {
	m.reads = append(m.reads, "users:"+project+":"+name)
	return append([]User(nil), m.users...), m.err
}
func (m *mockAdmin) ReadRegionalSecret(_ context.Context, project, region, name string) (bool, error) {
	m.reads = append(m.reads, "secret:"+project+":"+region+":"+name)
	return m.secret, m.err
}
func (m *mockAdmin) ListRegionalSecretVersions(_ context.Context, project, region, name string) ([]string, error) {
	m.reads = append(m.reads, "versions:"+project+":"+region+":"+name)
	return append([]string(nil), m.versions...), m.err
}

func fixtureAdmin() *mockAdmin {
	return &mockAdmin{instance: expectedInstance(), secret: true}
}

func TestBuildIsDryRunWithExactTargetAndExplicitRole(t *testing.T) {
	api := fixtureAdmin()
	plan, err := Build(context.Background(), api)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "dry_run_only" || plan.CloudMutations != "none; this output is a proposal" {
		t.Fatalf("unexpected execution mode: %#v", plan)
	}
	if plan.UserRequest.Name != Login || plan.UserRequest.Type != "BUILT_IN" || !reflect.DeepEqual(plan.UserRequest.DatabaseRoles, []string{CandidateRole}) {
		t.Fatalf("databaseRoles must be explicit and nonempty: %#v", plan.UserRequest)
	}
	if plan.SecretPayload != "not_generated_or_serialized" || strings.Contains(strings.Join(plan.Sequence, " "), "latest") {
		t.Fatalf("secret payload/version safety violated: %#v", plan)
	}
	serialized, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	text := string(serialized)
	if strings.Contains(text, "password=") || strings.Contains(text, "secret_data") {
		t.Fatalf("dry-run manifest serialized credential material: %s", text)
	}
	if len(api.reads) != 4 {
		t.Fatalf("expected metadata-only API reads, got %#v", api.reads)
	}
}

func TestBuildAbortsOnLoginCollisionBeforeSecretRead(t *testing.T) {
	api := fixtureAdmin()
	api.users = []User{{Name: "postgres"}, {Name: Login}}
	_, err := Build(context.Background(), api)
	if !errors.Is(err, ErrCollision) {
		t.Fatalf("collision should fail closed: %v", err)
	}
	if len(api.reads) != 2 || strings.HasPrefix(api.reads[len(api.reads)-1], "secret:") {
		t.Fatalf("collision proceeded beyond user preflight: %#v", api.reads)
	}
}

func TestBuildRejectsDriftAndSecretCollision(t *testing.T) {
	api := fixtureAdmin()
	api.instance.PrivateIP = "10.189.0.4"
	if _, err := Build(context.Background(), api); err == nil {
		t.Fatal("target drift passed")
	}
	api = fixtureAdmin()
	api.secret = false
	if _, err := Build(context.Background(), api); err == nil {
		t.Fatal("missing exact regional secret passed")
	}
	api = fixtureAdmin()
	api.versions = []string{"3"}
	if _, err := Build(context.Background(), api); err == nil || !strings.Contains(err.Error(), "version collision") {
		t.Fatalf("preexisting version collision did not abort: %v", err)
	}
}

func TestRoleListRejectsEmptyAndFallback(t *testing.T) {
	for _, roles := range [][]string{nil, {}, {"cloudsqlsuperuser"}, {CandidateRole, "pg_read_all_data"}} {
		if err := validateRoles(roles); err == nil {
			t.Fatalf("unsafe role list accepted: %#v", roles)
		}
	}
	if err := validateRoles([]string{CandidateRole}); err != nil {
		t.Fatalf("explicit candidate rejected: %v", err)
	}
}

func TestReadErrorsAreRedacted(t *testing.T) {
	api := fixtureAdmin()
	api.err = errors.New("sensitive payload: password=do-not-print")
	_, err := Build(context.Background(), api)
	if err == nil || strings.Contains(err.Error(), "do-not-print") || strings.Contains(err.Error(), "password=") {
		t.Fatalf("API error leaked untrusted details: %v", err)
	}
}
