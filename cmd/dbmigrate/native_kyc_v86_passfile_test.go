package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestNativeV86PGXConsumesExplicitPrivatePassfile(t *testing.T) {
	passfile := filepath.Join(t.TempDir(), "pgpass")
	const synthetic = "synthetic-unit-passphrase-not-a-real-credential"
	if err := os.WriteFile(passfile, []byte("127.0.0.1:5432:espacigo_dms_stage_20261020:espacigo_migrator_login:"+synthetic+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER"} {
		t.Setenv(key, "")
	}
	t.Setenv("PGPASSFILE", passfile)
	cfg, err := pgx.ParseConfig("postgresql://espacigo_migrator_login@127.0.0.1:5432/espacigo_dms_stage_20261020?sslmode=disable")
	if err != nil {
		t.Fatal("parse synthetic native configuration failed")
	}
	if cfg.Password != synthetic {
		t.Fatal("pgx did not consume the explicitly selected synthetic passfile")
	}
}
