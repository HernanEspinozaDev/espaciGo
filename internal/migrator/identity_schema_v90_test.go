package migrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type v90ReleasePolicy struct {
	ReviewRequired map[string]struct {
		Checksum  string `json:"checksum"`
		Strategy  string `json:"strategy"`
		Automatic bool   `json:"automatic"`
	} `json:"review_required_migrations"`
}

func TestIdentityV90MigrationRegisteredAndAppendOnly(t *testing.T) {
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 90 || all[89].Version != 90 || all[89].Name != "V000090__identity_verified_snapshot_purge.sql" {
		t.Fatalf("migration inventory must preserve V90; got %d entries", len(all))
	}
	policyPath := filepath.Join("..", "..", "db", "release-policy.json")
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	var policy v90ReleasePolicy
	if err := json.Unmarshal(policyBytes, &policy); err != nil {
		t.Fatal(err)
	}
	name := "V000090__identity_verified_snapshot_purge.sql"
	entry, ok := policy.ReviewRequired[name]
	if !ok || entry.Strategy != "security-tightening" || entry.Automatic {
		t.Fatalf("V90 must be explicitly review-required and non-automatic: %+v", entry)
	}
	sql, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(sql)
	if entry.Checksum != hex.EncodeToString(digest[:]) {
		t.Fatalf("V90 policy checksum %q does not match migration", entry.Checksum)
	}
}
