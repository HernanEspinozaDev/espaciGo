package gcp03origin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestMigrationHistoryHashesVersionsOneTwoTen(t *testing.T) {
	rows := []migrationHistoryEntry{
		{version: "10", checksum: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		{version: "2", checksum: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{version: "1", checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	legacy, canonical, err := migrationHistoryHashes(rows)
	if err != nil {
		t.Fatal(err)
	}
	if legacy != "c2af99dc3536948fd524d57eca7f5d2d38ff34519461983007531ce123aa2ca8" {
		t.Fatalf("legacy v1 hash = %s", legacy)
	}
	if canonical != "b69202fa8709b699b00cd3cc07ada528cc3587b98d368405c5961545f64f682c" {
		t.Fatalf("canonical v2 hash = %s", canonical)
	}
	rows[0], rows[2] = rows[2], rows[0]
	legacyReordered, canonicalReordered, err := migrationHistoryHashes(rows)
	if err != nil || legacyReordered != legacy || canonicalReordered != canonical {
		t.Fatalf("reordered rows changed hashes: v1=%s v2=%s err=%v", legacyReordered, canonicalReordered, err)
	}
}

func TestMigrationHistoryHashesV1ThroughV61SourceChecksums(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "db", "migrations", "V*__*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	versionPattern := regexp.MustCompile(`^V([0-9]+)__`)
	rows := make([]migrationHistoryEntry, 0, 61)
	seen := make(map[string]bool, 61)
	for _, path := range paths {
		match := versionPattern.FindStringSubmatch(filepath.Base(path))
		if match == nil {
			continue
		}
		version := parseMigrationVersion(t, match[1])
		if version > 61 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		rows = append(rows, migrationHistoryEntry{version: fmt.Sprint(version), checksum: fmt.Sprintf("%x", hash)})
		seen[match[1]] = true
	}
	if len(rows) != 61 || len(seen) != 61 {
		t.Fatalf("expected contiguous V1-V61 source migrations, got rows=%d versions=%d", len(rows), len(seen))
	}
	legacy, canonical, err := migrationHistoryHashes(rows)
	if err != nil {
		t.Fatal(err)
	}
	if legacy != "5dd344d805ecdf9c500eabac8e2013b0186a38066d3d17599d59ecef62c54cdd" {
		t.Fatalf("legacy v1 hash = %s", legacy)
	}
	if canonical != canonicalMigrationHistorySHA256V2 {
		t.Fatalf("canonical v2 hash = %s", canonical)
	}
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
	legacyReordered, canonicalReordered, err := migrationHistoryHashes(rows)
	if err != nil || legacyReordered != legacy || canonicalReordered != canonical {
		t.Fatalf("reordered V1-V61 rows changed hashes: v1=%s v2=%s err=%v", legacyReordered, canonicalReordered, err)
	}
}

func parseMigrationVersion(t *testing.T, value string) int {
	t.Helper()
	var version int
	if _, err := fmt.Sscanf(value, "%d", &version); err != nil {
		t.Fatalf("invalid migration version %q: %v", value, err)
	}
	return version
}
