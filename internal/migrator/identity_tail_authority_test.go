package migrator

import (
	"strings"
	"testing"
)

func validIdentityTailDDLAuthority() IdentityTailDDLAuthority {
	return IdentityTailDDLAuthority{
		SessionUser: "espacigo_migrator_login", CurrentUser: "espacigo_schema_owner", CurrentSchema: "public", PublicCreate: true,
		SessionRole:       V84RoleAttributes{CanLogin: true},
		SchemaOwnerExists: true, RuntimeExists: true, PrivacyCleanupWorkerExists: true,
		CaseOwner: "espacigo_schema_owner", DeclarationOwner: "espacigo_schema_owner", EventOwner: "espacigo_schema_owner", PurgeOwner: "espacigo_schema_owner",
		PurgeSecurityDefiner: true, PurgeSearchPath: []string{"search_path=pg_catalog"}, PurgeNonOwnerACLCount: 1,
		PurgeWorkerExecute: true, PurgeWorkerGrantorOwner: true,
		RuntimeCaseSelect: true, RuntimeCaseExpiryUpdate: true, RuntimeEventInsert: true,
	}
}

func TestIdentityTailDDLAuthoritySelectionAndGuards(t *testing.T) {
	for _, tc := range []struct {
		version int64
		name    string
		want    bool
	}{
		{95, v95IdentityMigration, true},
		{96, v96IdentityMigration, true},
		{95, v96IdentityMigration, false},
		{96, v95IdentityMigration, false},
		{94, v95IdentityMigration, false},
	} {
		if got := needsIdentityTailDDLAuthority(tc.version, tc.name); got != tc.want {
			t.Errorf("needsIdentityTailDDLAuthority(%d, %q)=%v, want %v", tc.version, tc.name, got, tc.want)
		}
	}

	for _, version := range []int64{95, 96} {
		if err := ValidateIdentityTailDDLAuthority(validIdentityTailDDLAuthority(), version); err != nil {
			t.Errorf("valid V%d authority rejected: %v", version, err)
		}
	}

	cases := []struct {
		name    string
		version int64
		mutate  func(*IdentityTailDDLAuthority)
		want    string
	}{
		{"wrong actor", 95, func(a *IdentityTailDDLAuthority) { a.CurrentUser = "postgres" }, "pinned"},
		{"unsafe migrator login", 95, func(a *IdentityTailDDLAuthority) { a.SessionRole.Superuser = true }, "migrator login"},
		{"no public create", 95, func(a *IdentityTailDDLAuthority) { a.PublicCreate = false }, "CREATE"},
		{"schema owner login", 95, func(a *IdentityTailDDLAuthority) { a.SchemaOwnerExists = false }, "schema-owner"},
		{"runtime inherit", 95, func(a *IdentityTailDDLAuthority) { a.RuntimeRole.Inherit = true }, "runtime role"},
		{"missing privacy worker", 95, func(a *IdentityTailDDLAuthority) { a.PrivacyCleanupWorkerExists = false }, "privacy cleanup worker"},
		{"case owner mismatch v95", 95, func(a *IdentityTailDDLAuthority) { a.CaseOwner = "postgres" }, "schema-owned"},
		{"declaration owner mismatch", 95, func(a *IdentityTailDDLAuthority) { a.DeclarationOwner = "postgres" }, "schema-owned"},
		{"purge owner mismatch", 95, func(a *IdentityTailDDLAuthority) { a.PurgeOwner = "postgres" }, "schema-owned"},
		{"purge not definer", 95, func(a *IdentityTailDDLAuthority) { a.PurgeSecurityDefiner = false }, "definer/search-path"},
		{"purge search path drift", 95, func(a *IdentityTailDDLAuthority) { a.PurgeSearchPath = []string{"public"} }, "definer/search-path"},
		{"extra purge ACL", 95, func(a *IdentityTailDDLAuthority) { a.PurgeNonOwnerACLCount++ }, "worker-only"},
		{"worker execute missing", 95, func(a *IdentityTailDDLAuthority) { a.PurgeWorkerExecute = false }, "worker-only"},
		{"worker grantor mismatch", 95, func(a *IdentityTailDDLAuthority) { a.PurgeWorkerGrantorOwner = false }, "worker-only"},
		{"worker grant option", 95, func(a *IdentityTailDDLAuthority) { a.PurgeWorkerGrantable = true }, "worker-only"},
		{"runtime purge execute", 95, func(a *IdentityTailDDLAuthority) { a.RuntimePurgeExecute = true }, "worker-only"},
		{"event owner mismatch v96", 96, func(a *IdentityTailDDLAuthority) { a.EventOwner = "postgres" }, "schema-owned"},
		{"runtime select missing", 96, func(a *IdentityTailDDLAuthority) { a.RuntimeCaseSelect = false }, "runtime read/write"},
		{"runtime column update missing", 96, func(a *IdentityTailDDLAuthority) { a.RuntimeCaseExpiryUpdate = false }, "runtime read/write"},
		{"runtime event insert missing", 96, func(a *IdentityTailDDLAuthority) { a.RuntimeEventInsert = false }, "runtime read/write"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authority := validIdentityTailDDLAuthority()
			tc.mutate(&authority)
			err := ValidateIdentityTailDDLAuthority(authority, tc.version)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("V%d authority error=%v, want message containing %q", tc.version, err, tc.want)
			}
		})
	}
}

func TestIdentityTailDDLAuthorityV96DoesNotRequireV95Worker(t *testing.T) {
	authority := validIdentityTailDDLAuthority()
	authority.PrivacyCleanupWorkerExists = false
	authority.PrivacyCleanupWorkerRole = V84RoleAttributes{}
	if err := ValidateIdentityTailDDLAuthority(authority, 96); err != nil {
		t.Fatalf("V96 incorrectly depends on the V95 worker role: %v", err)
	}
}
