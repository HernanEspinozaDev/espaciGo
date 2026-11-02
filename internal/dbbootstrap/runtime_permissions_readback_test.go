package dbbootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestParseRuntimeACLStatements(t *testing.T) {
	sql := `GRANT UPDATE (ACTOR_ID, ESTADO) ON public.disputa_ensayo_historial TO espacigo_runtime WITH GRANT OPTION;
REVOKE ALL PRIVILEGES ON public.media_asset, public.media_variant FROM espacigo_runtime CASCADE;
GRANT EXECUTE ON FUNCTION public.purge_expired_local_contracts_and_publication_history(timestamptz,integer) TO espacigo_runtime;
REVOKE DELETE ON public.tarifa_espacio FROM espacigo_runtime;
GRANT EXECUTE ON FUNCTION public.purge_unreferenced_local_publication_rates(uuid) TO espacigo_runtime;`
	effects, err := ParseRuntimeACLStatements(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 7 {
		t.Fatalf("got %d effects, want 7: %#v", len(effects), effects)
	}
	want := []RuntimeACLEffect{
		{Action: "GRANT", ObjectKind: "TABLE", Object: "public.disputa_ensayo_historial", Privilege: "UPDATE", Column: "actor_id", GrantOption: true},
		{Action: "GRANT", ObjectKind: "TABLE", Object: "public.disputa_ensayo_historial", Privilege: "UPDATE", Column: "estado", GrantOption: true},
		{Action: "REVOKE", ObjectKind: "TABLE", Object: "public.media_asset", Privilege: "ALL PRIVILEGES"},
	}
	for i, w := range want {
		if effects[i] != w {
			t.Errorf("effect[%d]=%+v want %+v", i, effects[i], w)
		}
	}
	if effects[4].Object != "public.purge_expired_local_contracts_and_publication_history(timestamptz,integer)" {
		t.Fatalf("function identity was split at its argument comma: %+v", effects[4])
	}
	if effects[6].Action != "GRANT" || effects[6].Privilege != "EXECUTE" {
		t.Fatalf("override grant was not retained: %+v", effects[6])
	}
}

func TestParseACLStatementsForGranteePublic(t *testing.T) {
	effects, err := ParseACLStatementsForGrantee(`REVOKE ALL ON FUNCTION public.purge_expired(integer) FROM PUBLIC;`, "PUBLIC")
	if err != nil {
		t.Fatal(err)
	}
	want := RuntimeACLEffect{
		Action: "REVOKE", Grantee: "PUBLIC", ObjectKind: "FUNCTION",
		Object: "public.purge_expired(integer)", Privilege: "ALL PRIVILEGES",
	}
	if len(effects) != 1 || effects[0] != want {
		t.Fatalf("got %+v, want [%+v]", effects, want)
	}
}

func TestRuntimeACLSourceParses(t *testing.T) {
	effects, err := ParseRuntimeACLStatements(runtimeACLSQL + ";" + runtimeACLReplaySQL + ";" + runtimeACLOverridesSQL)
	if err != nil {
		t.Fatalf("runtime source unsupported: %v", err)
	}
	if len(effects) == 0 {
		t.Fatal("runtime source produced no effects")
	}
}

func TestRuntimeACLParserFailsClosed(t *testing.T) {
	for _, sql := range []string{
		"GRANT SELECT ON public.x TO role; SELECT 1",
		"GRANT SELECT ON public.x TO role WITH ADMIN OPTION;",
		"GRANT SELECT ON public.x TO espacigo_runtime CASCADE;",
		"REVOKE SELECT ON public.x FROM espacigo_runtime WITH GRANT OPTION;",
		"GRANT SELECT ON public.x TO espacigo_runtime; GRANT UPDATE (bad,,list) ON public.x TO espacigo_runtime;",
		"",
	} {
		if _, err := ParseRuntimeACLStatements(sql); err == nil {
			t.Errorf("accepted unsupported SQL %q", sql)
		}
	}
}

func TestRuntimeACLParserRejectsInvalidPrivilegeScopes(t *testing.T) {
	for _, sql := range []string{
		"GRANT USAGE ON TABLE public.items TO espacigo_runtime;",
		"GRANT EXECUTE ON SEQUENCE public.items_id_seq TO espacigo_runtime;",
		"GRANT SELECT ON FUNCTION public.purge(uuid) TO espacigo_runtime;",
		"GRANT SELECT ON SCHEMA public TO espacigo_runtime;",
		"GRANT DELETE (id) ON TABLE public.items TO espacigo_runtime;",
		"GRANT ALL PRIVILEGES (id) ON TABLE public.items TO espacigo_runtime;",
		"GRANT SELECT ON TABLE public.items TO espacigo_runtime RESTRICT;",
		"GRANT EXECUTE ON FUNCTION public.purge(integer nonsense) TO espacigo_runtime;",
	} {
		if _, err := ParseRuntimeACLStatements(sql); err == nil {
			t.Errorf("accepted invalid ACL syntax %q", sql)
		}
	}
}

func TestRuntimeACLParserIgnoresCommentSemicolons(t *testing.T) {
	sql := `GRANT SELECT ON TABLE public.items TO espacigo_runtime; -- an inert ; comment
GRANT INSERT ON TABLE public.items TO espacigo_runtime;`
	effects, err := ParseRuntimeACLStatements(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 2 {
		t.Fatalf("got %d effects, want 2: %+v", len(effects), effects)
	}
}

func TestSplitACLStatementsPreservesQuotedSemicolons(t *testing.T) {
	statements, err := splitACLStatements(`SELECT 'left;right'; SELECT "left;right";`)
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want 2: %#v", len(statements), statements)
	}
}

func TestSplitACLStatementsHonorsStandardConformingStrings(t *testing.T) {
	statements, err := splitACLStatements(`SELECT 'a\'; GRANT SELECT ON TABLE public.items TO espacigo_runtime;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want 2: %#v", len(statements), statements)
	}
}

type runtimeACLExecDB struct {
	observerExists bool
	replayExists   bool
	execs          []string
}

type runtimeACLBoolRow bool

func (runtimeACLExecDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (db *runtimeACLExecDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	db.execs = append(db.execs, sql)
	return pgconn.NewCommandTag("GRANT"), nil
}

func (db *runtimeACLExecDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "rolname = 'espacigo_db_observer'"):
		return runtimeACLBoolRow(db.observerExists)
	case strings.Contains(sql, "to_regclass('public.registro_replay_baja_local')"):
		return runtimeACLBoolRow(db.replayExists)
	default:
		return runtimeACLBoolRow(false)
	}
}

func (row runtimeACLBoolRow) Scan(destinations ...any) error {
	if len(destinations) != 1 {
		return errors.New("unexpected scan column count")
	}
	value, ok := destinations[0].(*bool)
	if !ok {
		return errors.New("unexpected scan destination")
	}
	*value = bool(row)
	return nil
}

func TestGrantRuntimePermissionsExecutesSharedSQLSources(t *testing.T) {
	db := &runtimeACLExecDB{observerExists: true, replayExists: true}
	if err := GrantRuntimePermissions(context.Background(), db); err != nil {
		t.Fatalf("GrantRuntimePermissions: %v", err)
	}
	if len(db.execs) != 4 {
		t.Fatalf("executed %d ACL batches, want observer, core, replay, override", len(db.execs))
	}
	if db.execs[0] != runtimeACLObserverSQL {
		t.Fatalf("observer SQL differs from shared source: %q", db.execs[0])
	}
	if db.execs[1] != runtimeACLSQL {
		t.Fatalf("runtime SQL differs from shared source: %q", db.execs[1])
	}
	if db.execs[2] != runtimeACLReplaySQL {
		t.Fatalf("replay SQL differs from shared source: %q", db.execs[2])
	}
	if db.execs[3] != runtimeACLOverridesSQL {
		t.Fatalf("override SQL differs from shared source: %q", db.execs[3])
	}
}

func TestGrantRuntimePermissionsSkipsOptionalGrantsWhenAbsent(t *testing.T) {
	db := &runtimeACLExecDB{}
	if err := GrantRuntimePermissions(context.Background(), db); err != nil {
		t.Fatalf("GrantRuntimePermissions: %v", err)
	}
	if len(db.execs) != 2 || db.execs[0] != runtimeACLSQL || db.execs[1] != runtimeACLOverridesSQL {
		t.Fatalf("unexpected SQL batches when optional targets are absent: %#v", db.execs)
	}
}
