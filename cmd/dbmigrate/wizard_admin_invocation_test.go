package main

import (
	"path/filepath"
	"testing"
)

func TestReviewedWizardAdminInventoryMatchesSource(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := reviewedWizardAdminInventory(filepath.Join(root, "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 83 || got[59].Version != 60 || got[63].Version != 64 || got[65].Version != 66 || got[82].Version != 83 {
		t.Fatalf("unexpected reviewed inventory shape: %d entries", len(got))
	}
}

func TestWizardAdminInvocationIsSeparateAndExact(t *testing.T) {
	valid := func(version int64, confirm string, plan, apply, recover bool, acl, f5 string, cloud bool, cloudConfirm string) error {
		return validateWizardAdminInvocation(version, confirm, plan, apply, recover, acl, f5, cloud, cloudConfirm)
	}
	if err := valid(60, wizardAdminConfirmation, false, false, false, "", "", false, ""); err != nil {
		t.Fatal(err)
	}
	for _, version := range []int64{0, 59, 61, 64, 66, 83} {
		if version != 64 && version != 66 && version != 0 {
			if err := valid(version, wizardAdminConfirmation, false, false, false, "", "", false, ""); err == nil {
				t.Fatalf("accepted version %d", version)
			}
		}
	}
	for _, version := range []int64{60, 64, 66} {
		if err := valid(version, "wrong", false, false, false, "", "", false, ""); err == nil {
			t.Fatalf("accepted wrong confirmation at %d", version)
		}
	}
	cases := []struct {
		name                 string
		plan, apply, recover bool
		acl, f5              string
		cloud                bool
		cloudConfirm         string
	}{
		{"plan", true, false, false, "", "", false, ""}, {"apply acl", false, true, false, "", "", false, ""}, {"recover acl", false, false, true, "", "", false, ""},
		{"acl confirmation", false, false, false, "other", "", false, ""}, {"f5", false, false, false, "", "f5", false, ""}, {"cloud owner", false, false, false, "", "", true, ""}, {"cloud confirmation", false, false, false, "", "", false, "cloud"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := valid(60, wizardAdminConfirmation, tc.plan, tc.apply, tc.recover, tc.acl, tc.f5, tc.cloud, tc.cloudConfirm); err == nil {
				t.Fatal("accepted mixed invocation")
			}
		})
	}
	if err := valid(0, wizardAdminConfirmation, false, false, false, "", "", false, ""); err == nil {
		t.Fatal("accepted confirmation without version")
	}
}

func TestWizardAdminIdentityAcceptsOnlyFixedLocalConfiguration(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(root, filepath.FromSlash(wizardAdminPassfile))
	err = validateWizardAdminIdentity(wizardAdminDatabaseURL, "direct", "", func(key string) string {
		if key == "PGPASSFILE" {
			return passfile
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWizardAdminIdentityRejectsOverridesAndWrongTargets(t *testing.T) {
	url := "postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable"
	base := map[string]string{"PGPASSFILE": ""}
	for key, value := range map[string]string{"PGPASSWORD": "x", "PGOPTIONS": "-c role=x", "PGSERVICE": "x", "PGSERVICEFILE": "x", "PGHOST": "remote"} {
		t.Run(key, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range base {
				env[k] = v
			}
			env[key] = value
			if err := validateWizardAdminIdentity(url, "direct", "", func(k string) string { return env[k] }); err == nil {
				t.Fatal("accepted override")
			}
		})
	}
	for _, tc := range []struct{ url, mode, role string }{{"postgres://postgres@localhost:15436/espacigo_wizard_20261106?sslmode=disable", "direct", ""}, {"postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=require", "direct", ""}, {url, "cloudsql", ""}, {url, "direct", "espacigo_schema_owner"}} {
		if err := validateWizardAdminIdentity(tc.url, tc.mode, tc.role, func(string) string { return "" }); err == nil {
			t.Fatal("accepted invalid target or role")
		}
	}
}
