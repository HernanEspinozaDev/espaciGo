package migrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaEvidenceAcceptsOnlyReviewedFraming(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "db", "query", "img01_schema_release_evidence.sql"))
	if err != nil {
		t.Fatal(err)
	}
	statement, err := schemaEvidenceStatement(source)
	if err != nil || !strings.HasPrefix(statement, "SELECT jsonb_build_object(") {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(source), "BEGIN READ ONLY;", "BEGIN;", 1), strings.Replace(string(source), "\\pset format unaligned", "\\include other.sql", 1), string(source) + "SELECT 1;", strings.Replace(string(source), "SELECT jsonb_build_object(", "DELETE FROM x RETURNING jsonb_build_object(", 1)} {
		if _, err := schemaEvidenceStatement([]byte(bad)); err == nil {
			t.Fatal("unreviewed SQL framing accepted")
		}
	}
}

func TestRunnerSchemaEvidenceReadonlyBlocksWritingSelect(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		if _, err := conn.Exec(ctx, `CREATE TABLE public.catalog_write_probe(id integer); CREATE FUNCTION public.catalog_write_attempt() RETURNS integer LANGUAGE plpgsql AS $$ BEGIN INSERT INTO public.catalog_write_probe VALUES(1); RETURN 1; END $$;`); err != nil {
			t.Fatal(err)
		}
		query := []byte("BEGIN READ ONLY;\nSELECT jsonb_build_object('format','espacigo.img01.schema-evidence.v1','attempt',public.catalog_write_attempt())::text;\nCOMMIT;\n")
		if report, err := collectSchemaEvidence(ctx, conn, query); err == nil || len(report) != 0 {
			t.Fatal("catalog query wrote in read-only transaction")
		}
		var count int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.catalog_write_probe").Scan(&count); err != nil || count != 0 {
			t.Fatalf("unexpected writes: %d %v", count, err)
		}
	})
}

func assertPackagedCatalogRead(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	conn := connectTestDB(t, ctx, databaseURL)
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "SET ROLE espacigo_schema_owner"); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "db", "query", "img01_schema_release_evidence.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture has a random isolated DB name. Adapt only the destination
	// predicate for this local test; cloud execution uses the unchanged file.
	source = []byte(strings.Replace(string(source), "current_database() = 'espacigo_dms_stage_20261020'", "current_database() = '"+conn.Config().Database+"'", 1))
	report, err := collectSchemaEvidence(ctx, conn, source)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Query  string `json:"query_sha256"`
		Target struct {
			Readonly string `json:"transaction_read_only"`
			User     string `json:"current_user"`
		} `json:"target"`
		Ledger struct {
			Count int `json:"count"`
		} `json:"migration_ledger"`
	}
	if err := json.Unmarshal(report, &d); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(source)
	if d.Query != hex.EncodeToString(digest[:]) || d.Target.Readonly != "on" || d.Target.User != "espacigo_schema_owner" || d.Ledger.Count != 83 {
		t.Fatalf("incomplete restricted catalog: %+v", d)
	}
}
