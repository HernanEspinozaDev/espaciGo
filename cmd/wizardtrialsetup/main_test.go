package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialAssignmentQueryTypesFormatArguments(t *testing.T) {
	if !strings.Contains(credentialAssignmentQuery, "$1::text") || !strings.Contains(credentialAssignmentQuery, "$2::text") {
		t.Fatal("credential format arguments must have explicit text types")
	}
}

func TestRequirePrivatePassfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pgpass")
	if err := os.WriteFile(path, []byte("secret must not be read or printed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := requirePrivatePassfile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := requirePrivatePassfile(path); err == nil {
		t.Fatal("accepted permissive file mode")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := requirePrivatePassfile(link); err == nil {
		t.Fatal("accepted symlink passfile")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := requirePrivatePassfile(path); err == nil {
		t.Fatal("accepted non-private parent directory")
	}
}

func TestCommitOutcomeUncertainMessageRetainsCredentials(t *testing.T) {
	err := commitOutcomeError()
	if err == nil || err.Error() != "commit outcome uncertain; credentials retained; readback required" {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "password") {
		t.Fatal("error exposes credential details")
	}
}

func TestParseConfigExactTarget(t *testing.T) {
	raw := "postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable"
	_, got, err := parseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.host != expectedHost || got.port != expectedPort || got.user != expectedUser || got.database != expectedTarget {
		t.Fatalf("unexpected target: %+v", got)
	}
}

func TestParseConfigRejectsBeforeConnect(t *testing.T) {
	cases := []string{
		"postgres://postgres:secret@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable",
		"postgres://postgres@localhost:15436/espacigo_wizard_20261106?sslmode=disable",
		"postgres://postgres@127.0.0.1:5432/espacigo_wizard_20261106?sslmode=disable",
		"postgres://other@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable",
		"postgres://postgres@127.0.0.1:15436/other?sslmode=disable",
		"postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable&application_name=x",
		"postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable&options=x",
	}
	for _, raw := range cases {
		if _, _, err := parseConfig(raw); err == nil {
			t.Errorf("accepted unsafe URL")
		}
	}
}

func TestParseConfigRejectsEnvironmentOverrides(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE"} {
		t.Setenv(name, "not-empty")
		_, _, err := parseConfig("postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable")
		if err == nil {
			t.Errorf("accepted %s override", name)
		}
		_ = os.Unsetenv(name)
	}
}

func TestMembershipContractPinsExpectedEdgesAndFlags(t *testing.T) {
	want := map[string][2]bool{
		"espacigo_schema_owner:espacigo_migrator_login": {false, false},
		"espacigo_runtime:espacigo_api_login":           {false, false},
	}
	if len(expectedMemberships) != len(want) {
		t.Fatalf("expected %d memberships, got %d", len(want), len(expectedMemberships))
	}
	for _, edge := range expectedMemberships {
		key := edge.role + ":" + edge.member
		flags, ok := want[key]
		if !ok || edge.adminOption != flags[0] || edge.inheritOption != flags[1] || !edge.setOption {
			t.Errorf("unexpected membership edge or flags: %+v", edge)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing memberships: %v", want)
	}
}

func TestStageMembershipContractsAreDistinct(t *testing.T) {
	helper := membershipEdge{"espacigo_document_verification_fn", "espacigo_schema_owner", false, false, true}
	if sameMemberships(append(append([]membershipEdge{}, expectedMemberships...), helper), expectedMemberships) {
		t.Fatal("initial role contract must reject the post-V81 helper membership")
	}
	wantACL := append(append([]membershipEdge{}, expectedMemberships...), helper)
	if !sameMemberships(wantACL, aclExpectedMemberships) {
		t.Fatalf("ACL contract should accept exactly three expected edges: %+v", aclExpectedMemberships)
	}
	for _, bad := range []membershipEdge{
		{"espacigo_document_verification_fn", "espacigo_schema_owner", true, false, true},
		{"espacigo_document_verification_fn", "espacigo_schema_owner", false, true, true},
		{"espacigo_document_verification_fn", "espacigo_schema_owner", false, false, false},
		{"unexpected", "espacigo_schema_owner", false, false, true},
	} {
		if sameMemberships(append(append([]membershipEdge{}, wantACL...), bad), aclExpectedMemberships) {
			t.Errorf("ACL contract accepted unsafe or extra edge: %+v", bad)
		}
	}
	if sameMemberships(append(append([]membershipEdge{}, wantACL...), helper), aclExpectedMemberships) {
		t.Fatal("ACL contract accepted duplicate observed edge")
	}
	duplicatedWant := append(append([]membershipEdge{}, wantACL...), helper)
	if sameMemberships(wantACL, duplicatedWant) {
		t.Fatal("membership comparison accepted duplicate expected edge")
	}
}

func TestRoleSourceIncludesMigrationPrecreatedRoles(t *testing.T) {
	want := map[string]bool{"espacigo_gadm_bootstrap": true, "espacigo_privacy_cleanup_worker": true, "espacigo_gcp06_test_access": true}
	for _, r := range roleFlags {
		if want[r.name] && r.login {
			t.Errorf("%s must be NOLOGIN", r.name)
		}
		delete(want, r.name)
	}
	if len(want) > 0 {
		t.Fatalf("missing roles: %v", want)
	}
}
