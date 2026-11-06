package main

import (
	"os"
	"path/filepath"
	"testing"
)

func cloudOwnerTestSettings(t *testing.T) (connectionSettings, map[string]string) {
	t.Helper()
	private := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(private, 0700); err != nil {
		t.Fatal(err)
	}
	pass := filepath.Join(private, "test.pgpass")
	if err := os.WriteFile(pass, []byte("synthetic-unused-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return connectionSettings{mode: "direct", databaseURL: "postgresql://postgres@127.0.0.1:15432/espacigo_dms_stage_20261020?sslmode=require"}, map[string]string{"PGPASSFILE": pass}
}

func TestCloudOwnerInvocationRejectsUnreviewedStartupParameters(t *testing.T) {
	for _, parameter := range []string{"role=espacigo_runtime", "session_authorization=espacigo_runtime", "search_path=untrusted"} {
		t.Run(parameter, func(t *testing.T) {
			settings, env := cloudOwnerTestSettings(t)
			settings.databaseURL += "&" + parameter
			if err := validateCloudOwnerInvocation(true, false, false, false, cloudOwnerConfirmation, settings, func(k string) string { return env[k] }); err == nil {
				t.Fatal("accepted unreviewed startup parameter")
			}
		})
	}
}

func TestCloudOwnerInvocationRejectsServiceOverrides(t *testing.T) {
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE"} {
		t.Run(key, func(t *testing.T) {
			settings, env := cloudOwnerTestSettings(t)
			env[key] = "untrusted"
			if err := validateCloudOwnerInvocation(true, false, false, false, cloudOwnerConfirmation, settings, func(k string) string { return env[k] }); err == nil {
				t.Fatal("accepted environment override")
			}
		})
	}
}

func TestCloudOwnerSnapshotRejectsChangedAuthorizedMigration(t *testing.T) {
	source := filepath.Join("..", "..", "db", "migrations")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	mutated := t.TempDir()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() == "V000076__spaces_gallery_capacity_replacement_v2.sql" {
			contents = append(contents, []byte("\n-- unreviewed source mutation\n")...)
		}
		if err := os.WriteFile(filepath.Join(mutated, entry.Name()), contents, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if dir, cleanup, err := makeCloudOwnerSnapshot(mutated); err == nil {
		cleanup()
		t.Fatalf("accepted unreviewed migration into %s", dir)
	}
}
