package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWizardModeConflictValidation(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		version                    int64
		confirm                    string
		inventory, schema, release bool
		sha, inventorySHA          string
		expected                   int
	}{
		{"inventory", 60, wizardAdminConfirmation, true, false, false, "", "", 0},
		{"schema", 60, wizardAdminConfirmation, false, true, false, "", "", 0},
		{"release", 60, wizardAdminConfirmation, false, false, true, "", "", 0},
		{"release sha without mode", 60, wizardAdminConfirmation, false, false, false, "sha", "", 0},
		{"release inventory without mode", 60, wizardAdminConfirmation, false, false, false, "", "fingerprint", 0},
		{"release expected without mode", 60, wizardAdminConfirmation, false, false, false, "", "", 83},
		{"confirm only plus inventory", 0, wizardAdminConfirmation, true, false, false, "", "", 0},
		{"unsupported plus schema", 61, wizardAdminConfirmation, false, true, false, "", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateWizardModeConflicts(tc.version, tc.confirm, tc.inventory, tc.schema, tc.release, tc.sha, tc.inventorySHA, tc.expected); err == nil {
				t.Fatal("accepted conflicting wizard/mode invocation")
			}
		})
	}
	if err := validateWizardModeConflicts(0, "", false, false, false, "", "", 0); err != nil {
		t.Fatalf("legacy defaults rejected: %v", err)
	}
}

func TestWizardModeConflictsRejectBeforeConnectionAndInventoryRemainsOffline(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "dbmigrate")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(root, "cmd", "dbmigrate")
	build.Env = cleanDBEnvironment(os.Environ())
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	cases := [][]string{
		{"unexpected", "-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation},
		{"--", "-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation},
		{"-plan-only", "unexpected", "-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation},
		{"unexpected"},
		{"-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-inventory-only"},
		{"-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-schema-evidence-only"},
		{"-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-release"},
		{"-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-release-source-sha=sha"},
		{"-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-release-inventory-sha256=fingerprint"},
		{"-wizard-admin-version=60", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-release-expected-applied=83"},
		{"-confirm-wizard-admin=" + wizardAdminConfirmation, "-inventory-only"},
		{"-wizard-admin-version=61", "-confirm-wizard-admin=" + wizardAdminConfirmation, "-schema-evidence-only"},
	}
	for _, args := range cases {
		cmd := exec.Command(binary, args...)
		cmd.Env = cleanDBEnvironment(os.Environ())
		out, err := cmd.CombinedOutput()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
			t.Errorf("%v: expected exit 2; got %v: %s", args, err, out)
			continue
		}
		if strings.Contains(strings.Join(args, " "), "unexpected") || args[0] == "--" {
			if !strings.Contains(string(out), "positional arguments are not supported") {
				t.Errorf("%v: expected fixed positional-argument error; got: %s", args, out)
			}
		}
		if bytes.Contains(out, []byte("DATABASE_URL")) || bytes.Contains(out, []byte("connection settings")) || bytes.Contains(out, []byte("ping")) || bytes.Contains(out, []byte("SQL")) {
			t.Errorf("%v: reached connection/config path: %s", args, out)
		}
	}
	invDir := filepath.Join(tmp, "migrations")
	if err := os.Mkdir(invDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invDir, "V000001__offline.sql"), []byte("SELECT 1;\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-inventory-only", "-dir", invDir)
	cmd.Env = cleanDBEnvironment(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("legacy offline inventory: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "V000001") {
		t.Fatalf("inventory missing migration: %s", out)
	}
}

func cleanDBEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key := strings.SplitN(entry, "=", 2)[0]
		if strings.HasPrefix(key, "PG") || strings.HasPrefix(key, "DATABASE_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
