package migrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMigrationsOrdersFilesAndHashesExactBytes(t *testing.T) {
	dir := t.TempDir()
	second := []byte("CREATE TABLE second_probe (id integer);\n")
	first := []byte("CREATE TABLE first_probe (id integer);\r\n")
	if err := os.WriteFile(filepath.Join(dir, "V000002__second_probe.sql"), second, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "V000001__first_probe.sql"), first, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Version != 1 || got[1].Version != 2 {
		t.Fatalf("migrations not sorted: %#v", got)
	}
	if got[0].Name != "V000001__first_probe.sql" || got[0].Checksum != sha256Hex(first) {
		t.Fatalf("first migration metadata does not preserve exact bytes: %#v", got[0])
	}
}

func TestDiscoverMigrationsRejectsGapBeforeExecution(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"V000001__first.sql", "V000003__third.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := DiscoverMigrations(dir); err == nil {
		t.Fatal("expected a non-contiguous version sequence to fail")
	}
}

func TestDiscoverMigrationsRejectsDuplicateVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"V000001__first.sql", "V000001__renamed.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := DiscoverMigrations(dir); err == nil {
		t.Fatal("expected duplicate version to fail")
	}
}

func TestDiscoverMigrationsRejectsUnrecognizedSQLFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_bad.sql"), []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscoverMigrations(dir); err == nil {
		t.Fatal("expected unrecognized SQL filename to fail closed")
	}
}

func TestVerifyHistoryRejectsChecksumDrift(t *testing.T) {
	migrations := []Migration{{Version: 1, Name: "V000001__probe.sql", Checksum: "expected"}}
	history := []HistoryEntry{{Version: 1, Name: "V000001__probe.sql", Checksum: "changed"}}
	if err := verifyHistory(migrations, history); err == nil {
		t.Fatal("expected applied checksum drift to fail")
	}
}

func TestVerifyHistoryRejectsMissingOrRenamedAppliedFile(t *testing.T) {
	migrations := []Migration{{Version: 2, Name: "V000002__renamed.sql", Checksum: "expected"}}
	history := []HistoryEntry{{Version: 1, Name: "V000001__removed.sql", Checksum: "old"}}
	if err := verifyHistory(migrations, history); err == nil {
		t.Fatal("expected missing or renamed applied migration to fail")
	}
}

func TestVerifyHistoryAcceptsMatchingAppliedPrefix(t *testing.T) {
	migrations := []Migration{
		{Version: 1, Name: "V000001__one.sql", Checksum: "one"},
		{Version: 2, Name: "V000002__two.sql", Checksum: "two"},
	}
	history := []HistoryEntry{{Version: 1, Name: "V000001__one.sql", Checksum: "one"}}
	if err := verifyHistory(migrations, history); err != nil {
		t.Fatalf("matching applied prefix rejected: %v", err)
	}
}
