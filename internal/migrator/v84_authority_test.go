package migrator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestV84AuthorityManifestConstants(t *testing.T) {
	if v84AuthorityInstance != "espacigo-testprod:southamerica-west1:espacigo-testprod-db" || v84AuthorityDatabase != "espacigo_dms_stage_20261020" || v84AuthorityUser != "postgres" {
		t.Fatal("V84 authority target constants changed")
	}
	if v84AuthorityMigration != "V000084__didit_identity_verification_schema.sql" || len(v84AuthorityChecksum) != 64 || len(v84AuthorityInventory) != 64 {
		t.Fatal("V84 authority manifest constants are malformed")
	}
}

func TestV84AuthorityHelperRejectsUnknownModeAfterManifestCheck(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manageV84AuthorityWithConn(context.Background(), nil, dir, "unknown", "")
	if err == nil || !strings.Contains(err.Error(), "invalid V84 authority mode") {
		t.Fatalf("helper error = %v, want invalid mode", err)
	}
}

func TestV84AuthorityExpectedBaselineFormat(t *testing.T) {
	if !validHex(strings.Repeat("a", 64), 64) || validHex(strings.Repeat("A", 64), 64) || validHex(strings.Repeat("a", 63), 64) {
		t.Fatal("baseline SHA-256 format validation changed")
	}
}

func TestV84AuthorityUsesColumnScopedReferences(t *testing.T) {
	if v84GrantReferencesSQL != `GRANT REFERENCES (id) ON TABLE public.usuario TO espacigo_schema_owner` {
		t.Fatalf("grant SQL is not limited to usuario.id: %q", v84GrantReferencesSQL)
	}
	if v84RevokeReferencesSQL != `REVOKE REFERENCES (id) ON TABLE public.usuario FROM espacigo_schema_owner` {
		t.Fatalf("revoke SQL is not limited to usuario.id: %q", v84RevokeReferencesSQL)
	}
}
