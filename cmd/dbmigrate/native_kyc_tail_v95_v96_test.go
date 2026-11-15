package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func validNativeIdentityTailAuthority() *migrator.IdentityTailDDLAuthority {
	restricted := migrator.V84RoleAttributes{}
	return &migrator.IdentityTailDDLAuthority{
		SessionUser: "espacigo_migrator_login", CurrentUser: "espacigo_schema_owner", CurrentSchema: "public", PublicCreate: true,
		SessionRole:       migrator.V84RoleAttributes{CanLogin: true},
		SchemaOwnerExists: true, SchemaOwnerRole: restricted, RuntimeExists: true, RuntimeRole: restricted,
		PrivacyCleanupWorkerExists: true, PrivacyCleanupWorkerRole: restricted,
		CaseOwner: "espacigo_schema_owner", DeclarationOwner: "espacigo_schema_owner", EventOwner: "espacigo_schema_owner", PurgeOwner: "espacigo_schema_owner",
		PurgeSecurityDefiner: true, PurgeSearchPath: []string{"search_path=pg_catalog"}, PurgeNonOwnerACLCount: 1,
		PurgeWorkerExecute: true, PurgeWorkerGrantorOwner: true,
		RuntimeCaseSelect: true, RuntimeCaseExpiryUpdate: true, RuntimeEventInsert: true,
	}
}

func TestNativeKYCTailV95V96PinsReviewedSourceAndSnapshot(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	source := filepath.Join("..", "..", "db", "migrations")
	cases := []struct {
		version     int
		name        string
		checksum    string
		fingerprint string
	}{
		{95, "V000095__identity_document_case_declaration_binding.sql", "351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275", "67077c1996cc72f2d6177d801fbfbbc445a53ba23b6306fc83ee18adfcbf3554"},
		{96, "V000096__identity_reviewer_observation_audit.sql", "a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5", "c71a06dfac444537bd0b05ef12346c57bcfda686c7761d53b52c24af25d02ab7"},
	}
	for _, tc := range cases {
		t.Run(strings.TrimPrefix(tc.name, "V0000"), func(t *testing.T) {
			if got := nativeKYCTailReviewedFingerprint(tc.version); got != tc.fingerprint {
				t.Fatalf("V%d source fingerprint=%q, want %q", tc.version, got, tc.fingerprint)
			}
			if err := validateNativeKYCTailMode(tc.version, nativeAdminConfirmation, tc.fingerprint, nil); err != nil {
				t.Fatalf("V%d mode rejected: %v", tc.version, err)
			}
			stage, cleanup, err := makeNativeKYCTailSnapshot(source, tc.version, tc.fingerprint)
			if err != nil {
				t.Fatalf("V%d snapshot: %v", tc.version, err)
			}
			t.Cleanup(cleanup)
			items, err := migrator.DiscoverMigrations(stage)
			if err != nil {
				t.Fatalf("V%d staged inventory: %v", tc.version, err)
			}
			if len(items) != tc.version {
				t.Fatalf("V%d staged inventory count=%d", tc.version, len(items))
			}
			if items[tc.version-1].Name != tc.name || items[tc.version-1].Checksum != tc.checksum {
				t.Fatalf("V%d staged final migration=%+v", tc.version, items[tc.version-1])
			}
		})
	}
}

func TestNativeKYCTailV95V96PlanAndPostRequireExactPrefixAndPostgres187(t *testing.T) {
	all, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{95, 96} {
		t.Run(string(rune('0'+version/10))+string(rune('0'+version%10)), func(t *testing.T) {
			pending := all[version-1]
			plan := migrator.PlanResult{
				Database: nativeAdminConfirmation, ReadOnly: true, HistoryExists: true,
				ServerVersion: "18.7", Pending: []migrator.MigrationSummary{{Version: pending.Version, Name: pending.Name, Checksum: pending.Checksum}},
				IdentityDDLAuthority: validNativeIdentityTailAuthority(),
			}
			for _, migration := range all[:version-1] {
				plan.Applied = append(plan.Applied, migrator.HistoryEntry{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum})
			}
			if err := validateNativeKYCTailPlan(plan, version); err != nil {
				t.Fatalf("V%d exact predecessor plan rejected: %v", version, err)
			}
			missingAuthority := plan
			missingAuthority.IdentityDDLAuthority = nil
			if err := validateNativeKYCTailPlan(missingAuthority, version); err == nil || !strings.Contains(err.Error(), "authority preflight") {
				t.Fatalf("V%d plan without authority was not rejected: %v", version, err)
			}
			unsafeAuthority := *plan.IdentityDDLAuthority
			unsafeAuthority.SessionRole.Superuser = true
			unsafePlan := plan
			unsafePlan.IdentityDDLAuthority = &unsafeAuthority
			if err := validateNativeKYCTailPlan(unsafePlan, version); err == nil || !strings.Contains(err.Error(), "migrator login") {
				t.Fatalf("V%d plan with an unsafe migrator login was not rejected: %v", version, err)
			}
			if !nativeKYCTailRequiresCreate(version) {
				t.Fatalf("V%d must require public CREATE", version)
			}
			wrongEarlierApplied := plan
			wrongEarlierApplied.Applied = append([]migrator.HistoryEntry(nil), plan.Applied...)
			wrongEarlierApplied.Applied[10].Checksum = strings.Repeat("0", 64)
			if validateNativeKYCTailPlan(wrongEarlierApplied, version) == nil {
				t.Fatalf("V%d accepted an incorrect checksum earlier in the applied prefix", version)
			}
			plan.Applied[version-2].Checksum = strings.Repeat("0", 64)
			if validateNativeKYCTailPlan(plan, version) == nil {
				t.Fatalf("V%d accepted a wrong exact predecessor checksum", version)
			}

			post := migrator.PlanResult{
				Database: nativeAdminConfirmation, ReadOnly: true, HistoryExists: true, ServerVersion: "18.7",
			}
			for _, migration := range all[:version] {
				post.Applied = append(post.Applied, migrator.HistoryEntry{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum})
			}
			if err := validateNativeKYCTailPost(post, version); err != nil {
				t.Fatalf("V%d exact postcheck rejected: %v", version, err)
			}
			wrongEarlierPost := post
			wrongEarlierPost.Applied = append([]migrator.HistoryEntry(nil), post.Applied...)
			wrongEarlierPost.Applied[10].Checksum = strings.Repeat("0", 64)
			if validateNativeKYCTailPost(wrongEarlierPost, version) == nil {
				t.Fatalf("V%d postcheck accepted an incorrect checksum earlier in the applied prefix", version)
			}
			post.Applied[version-1].Checksum = strings.Repeat("0", 64)
			if validateNativeKYCTailPost(post, version) == nil {
				t.Fatalf("V%d accepted a wrong post-migration checksum", version)
			}
		})
	}
}
