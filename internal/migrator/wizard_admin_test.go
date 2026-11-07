package migrator

import (
	"context"
	"strings"
	"testing"
)

func TestWizardAdminRejectsUnsafeInputsBeforeConnect(t *testing.T) {
	valid := make([]MigrationSummary, 83)
	for i := range valid {
		valid[i] = MigrationSummary{Version: int64(i + 1), Name: "V", Checksum: strings.Repeat("a", 64)}
	}
	for _, tc := range []struct {
		name, dsn string
		version   int64
		manifest  []MigrationSummary
	}{
		{"version", wizardTrialDatabaseURL, 61, valid},
		{"target", "postgres://postgres@127.0.0.1:15436/other?sslmode=disable", 60, valid},
		{"extra-param", wizardTrialDatabaseURL + "&options=-csearch_path=public", 60, valid},
		{"manifest", wizardTrialDatabaseURL, 60, valid[:82]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunWizardAdministrativeMigration(context.Background(), tc.dsn, t.TempDir(), tc.version, tc.manifest)
			if err == nil {
				t.Fatal("expected input guard rejection")
			}
		})
	}
}

func TestVerifyWizardManifestRequiresExactOrderedInventory(t *testing.T) {
	m := make([]MigrationSummary, 83)
	for i := range m {
		m[i] = MigrationSummary{Version: int64(i + 1), Name: "V", Checksum: strings.Repeat("a", 64)}
	}
	if err := verifyWizardManifest(m); err != nil {
		t.Fatal(err)
	}
	m[41].Version++
	if err := verifyWizardManifest(m); err == nil {
		t.Fatal("accepted unordered manifest")
	}
}

func TestWizardExpectedOwnerMetadata(t *testing.T) {
	want := map[int64]map[string]bool{
		60: {"public.reject_gadm_bootstrap_identity_change()": false},
		64: {"public.finalize_pending_suppression_cleanup(uuid,timestamptz)": true},
		66: {
			"public.gcp06_test_access_batch_audit":            false,
			"public.gcp06_test_access_target_ids()":           true,
			"public.gcp06_test_access_owner_ready()":          true,
			"public.gcp06_test_access_operation_exists(uuid)": true,
		},
	}
	for version, objects := range want {
		for _, obj := range wizardOwnerObjects(version) {
			got, ok := objects[obj.identity]
			if !ok || wizardExpectedSecurityDefiner(version, obj) != got {
				t.Errorf("V%d %s security-definer=%v want %v", version, obj.identity, wizardExpectedSecurityDefiner(version, obj), got)
			}
		}
		if len(objects) != len(wizardOwnerObjects(version)) {
			t.Errorf("V%d metadata count=%d want %d", version, len(wizardOwnerObjects(version)), len(objects))
		}
	}
}

func TestWizardOwnerAllowlist(t *testing.T) {
	want := map[int64]int{60: 1, 64: 1, 66: 4}
	for v, n := range want {
		if got := len(wizardOwnerObjects(v)); got != n {
			t.Errorf("V%d owner allowlist count=%d want %d", v, got, n)
		}
	}
	if len(wizardOwnerObjects(61)) != 0 {
		t.Fatal("unexpected owner allowlist for V61")
	}
}
