package migrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRentalSharedHistoryPreservesAppliedIdentityPrefix(t *testing.T) {
	var manifest struct {
		Migrations []struct {
			Version int64  `json:"version"`
			Name    string `json:"name"`
			SHA256  string `json:"sha256"`
		} `json:"migrations"`
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "arrendamiento_espacio_publicado", "evidence", "shared-history-source-manifest-20261115.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Migrations) != 96 {
		t.Fatalf("expected the reviewed applied V1-V96 prefix, got %d", len(manifest.Migrations))
	}
	migrations, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 97 {
		t.Fatalf("shared history must contain applied V1-V96 plus rental V97, got %d migrations", len(migrations))
	}
	history := make([]HistoryEntry, 0, 96)
	for i, expected := range manifest.Migrations {
		if expected.Version != int64(i+1) {
			t.Fatalf("non-contiguous reviewed manifest at position %d", i)
		}
		actual := migrations[i]
		if actual.Version != expected.Version || actual.Name != expected.Name || actual.Checksum != expected.SHA256 {
			t.Fatalf("applied V%d differs from reviewed native history", expected.Version)
		}
		history = append(history, HistoryEntry{Version: expected.Version, Name: expected.Name, Checksum: expected.SHA256})
	}
	if err := verifyHistory(migrations, history); err != nil {
		t.Fatalf("V96 applied history is not compatible with the combined candidate: %v", err)
	}
	quote := migrations[96]
	if quote.Version != 97 || quote.Name != "V000097__commercial_rental_quote_snapshots.sql" || quote.Checksum != "0574033dde36b66efc00f36e20fcedbb424badc20c7109f941f78786fb5d8e14" {
		t.Fatal("rental V97 must preserve the never-applied commercial SQL bytes")
	}
	t.Run("rejects applied identity checksum drift", func(t *testing.T) {
		changed := append([]Migration(nil), migrations...)
		changed[84].Checksum = quote.Checksum
		if err := verifyHistory(changed, history); err == nil {
			t.Fatal("applied V85 identity history accepted the colliding rental checksum")
		}
	})
}
