package dbbootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

func semanticCopy(t *testing.T, s runtimeACLSemanticSnapshot) runtimeACLSemanticSnapshot {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var out runtimeACLSemanticSnapshot
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestSemanticACLReferenceAcceptsOrderAndRejectsDrift(t *testing.T) {
	reference, err := loadRuntimeACLSemanticReference()
	if err != nil {
		t.Fatal(err)
	}
	reordered := semanticCopy(t, reference)
	for i, j := 0, len(reordered.ACLs)-1; i < j; i, j = i+1, j-1 {
		reordered.ACLs[i], reordered.ACLs[j] = reordered.ACLs[j], reordered.ACLs[i]
	}
	if err = compareRuntimeACLSemanticSnapshots(reference, reordered); err != nil {
		t.Fatal("order affects semantic equality", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*runtimeACLSemanticSnapshot)
	}{
		{"extra privilege", func(s *runtimeACLSemanticSnapshot) {
			s.ACLs = append(s.ACLs, json.RawMessage(`{"class":"pg_class","identity":"table public.categoria_espacio","column":"","grantee":"PUBLIC","grantor":"postgres","privilege":"DELETE","grantable":false,"source":"effective"}`))
		}},
		{"missing privilege", func(s *runtimeACLSemanticSnapshot) { s.ACLs = s.ACLs[1:] }},
		{"wrong owner", func(s *runtimeACLSemanticSnapshot) {
			s.Owners[0] = json.RawMessage(`{"class":"pg_class","identity":"public.categoria_espacio","owner":"unexpected_owner"}`)
		}},
		{"wrong database", func(s *runtimeACLSemanticSnapshot) { s.Database = "other" }},
		{"wrong engine", func(s *runtimeACLSemanticSnapshot) { s.ServerVersion++ }},
		{"inventory changed", func(s *runtimeACLSemanticSnapshot) { s.ACLInventory[0] = "unexpected.acl" }},
		{"empty default added", func(s *runtimeACLSemanticSnapshot) {
			s.DefaultACLObjects = append(s.DefaultACLObjects, json.RawMessage(`{"role":"postgres","schema":"public","object_type":"r","state":"empty"}`))
		}},
		{"null state changed", func(s *runtimeACLSemanticSnapshot) {
			s.ACLStates[0] = json.RawMessage(`{"class":"pg_class","identity":"public.categoria_espacio","column":"","source":"effective","state":"empty"}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := semanticCopy(t, reference)
			tc.change(&observed)
			if compareRuntimeACLSemanticSnapshots(reference, observed) == nil {
				t.Fatal("semantic drift accepted")
			}
		})
	}
}
func TestSemanticSQLMatchesIndependentDiagnosticSource(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "acl_semantic_snapshot.sql"))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	start := strings.Index(raw, "WITH\n")
	end := strings.LastIndex(raw, "ROLLBACK;")
	if start < 0 || end <= start {
		t.Fatal("diagnostic query boundary missing")
	}
	if strings.TrimSuffix(strings.TrimSpace(raw[start:end]), ";") != strings.TrimSuffix(strings.TrimSpace(runtimeACLSemanticQuery), ";") {
		t.Fatal("operational SQL differs from independently exercised diagnostic SQL")
	}
}

// Opt-in executable inside the explicitly selected local Docker target/reference.
// Unix socket stays in that exact container; no credentials are copied. Negative
// probes use real transactional DDL, always rolled back, never target repairs.
func TestLocalSemanticACLPostgresGateAndRollbackNegatives(t *testing.T) {
	mode := os.Getenv("ESPACIGO_LOCAL_SEMANTIC_ACL_TEST")
	if mode == "" {
		t.Skip("requires the selected isolated local target/reference")
	}
	if mode != "reference" && mode != "target" {
		t.Fatal("invalid local ACL test mode")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=espacigo_f5_gate_20261101 sslmode=disable")
	if err != nil {
		t.Fatal("selected container SQL connection unavailable")
	}
	defer conn.Close(ctx)
	var db, user string
	var engine, accounts int
	if err = conn.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int,(SELECT count(*) FROM public.usuario)`).Scan(&db, &user, &engine, &accounts); err != nil {
		t.Fatal(err)
	}
	if db != runtimeACLF5DatabaseName || user != "postgres" || engine != 180006 || mode == "reference" && accounts != 0 || mode == "target" && accounts != 2 {
		t.Fatal("isolated target identity mismatch")
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyRuntimeACLF5PublicBaseline(ctx, tx); err != nil {
		tx.Rollback(ctx)
		t.Fatal("equivalent restored schema denied", err)
	}
	tx.Rollback(ctx)
	t.Log("SEMANTIC_EQUIVALENT_SCHEMA_WITH_DIFFERENT_OIDS_PASS")
	if mode == "reference" {
		for _, probe := range []struct{ name, sql string }{
			{"extra_privilege", `GRANT SELECT ON public.categoria_espacio TO PUBLIC`},
			{"missing_privilege", `REVOKE SELECT ON public.categoria_espacio FROM espacigo_runtime`},
			{"wrong_owner", `ALTER TABLE public.categoria_espacio OWNER TO espacigo_schema_owner`},
		} {
			t.Run(probe.name, func(t *testing.T) {
				negative, e := conn.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				defer negative.Rollback(ctx)
				if _, e = negative.Exec(ctx, probe.sql); e != nil {
					t.Fatal(e)
				}
				if e = verifyRuntimeACLF5PublicBaseline(ctx, negative); e == nil {
					t.Fatal("real PostgreSQL negative accepted")
				}
				if e = negative.Rollback(ctx); e != nil {
					t.Fatal(e)
				}
				clean, e := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
				if e != nil {
					t.Fatal(e)
				}
				defer clean.Rollback(ctx)
				if e = verifyRuntimeACLF5PublicBaseline(ctx, clean); e != nil {
					t.Fatal("rollback did not restore reference", e)
				}
			})
		}
		return
	}
	directory := os.Getenv("ESPACIGO_LOCAL_ACL_MIGRATIONS")
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		t.Fatal("exact copied source directory required")
	}
	migrations, err := migrator.DiscoverMigrations(directory)
	if err != nil {
		t.Fatal(err)
	}
	complete, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer complete.Rollback(ctx)
	rows, err := complete.Query(ctx, `SELECT version,name,checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	sources := []RuntimeACLSource{}
	index := 0
	for rows.Next() {
		var version int64
		var name, checksum string
		if err = rows.Scan(&version, &name, &checksum); err != nil {
			t.Fatal(err)
		}
		if index >= len(migrations) || migrations[index].Version != version || migrations[index].Name != name || migrations[index].Checksum != checksum {
			t.Fatal("ledger/source mismatch")
		}
		sources = append(sources, RuntimeACLSource{Version: int(version), SQL: migrations[index].SQL})
		index++
	}
	rows.Close()
	if err = rows.Err(); err != nil || index != 83 || index != len(migrations) {
		t.Fatal("incomplete source/ledger verification", err)
	}
	if err = VerifyRuntimeACLReadbackF5(ctx, complete, sources); err != nil {
		t.Fatal("actual complete operational ACL gate failed", err)
	}
	t.Log("REAL_FULL_RUNTIME_ACL_GATE_LEDGER_83_DIRECT_EFFECTIVE_OWNERS_MEMBERSHIPS_PASS")
}
