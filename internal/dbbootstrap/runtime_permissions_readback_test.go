package dbbootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
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
		{Action: "REVOKE", ObjectKind: "TABLE", Object: "public.media_asset", Privilege: "ALL PRIVILEGES", RevokeBehavior: "CASCADE"},
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
		"REVOKE GRANT OPTION FOR SELECT ON public.x FROM espacigo_runtime CASCADE;",
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

func TestFoldRuntimeACLSourceEffectsPreservesDirectPrivilegeState(t *testing.T) {
	effects := []RuntimeACLEffect{
		{Action: "GRANT", Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.items", Privilege: "ALL PRIVILEGES"},
		{Action: "GRANT", Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.items", Privilege: "UPDATE", Column: "owner_id"},
		{Action: "REVOKE", Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.items", Privilege: "SELECT"},
		{Action: "GRANT", Grantee: "PUBLIC", ObjectKind: "FUNCTION", Object: "public.purge(uuid)", Privilege: "EXECUTE"},
		{Action: "REVOKE", Grantee: "PUBLIC", ObjectKind: "FUNCTION", Object: "public.purge(uuid)", Privilege: "ALL PRIVILEGES"},
	}
	got, err := foldRuntimeACLEffects(effects)
	if err != nil {
		t.Fatal(err)
	}
	key := func(grantee, kind, object, privilege, column string) runtimeACLGrantKey {
		return runtimeACLGrantKey{Grantee: grantee, ObjectKind: kind, Object: object, Privilege: privilege, Column: column}
	}
	for _, privilege := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
		if grant := got[key("espacigo_runtime", "TABLE", "public.items", privilege, "")]; !grant.Granted {
			t.Errorf("missing folded table privilege %s", privilege)
		}
	}
	if grant := got[key("espacigo_runtime", "TABLE", "public.items", "SELECT", "")]; grant.Granted {
		t.Error("REVOKE SELECT did not remove the direct table grant")
	}
	if grant := got[key("espacigo_runtime", "TABLE", "public.items", "UPDATE", "owner_id")]; !grant.Granted {
		t.Error("table-level REVOKE/GRANT folding lost the column-scoped UPDATE")
	}
	if grant := got[key("PUBLIC", "FUNCTION", "public.purge(uuid)", "EXECUTE", "")]; grant.Granted {
		t.Error("REVOKE ALL did not remove the direct PUBLIC function grant")
	}
}

func TestFoldRuntimeACLSourceEffectsTracksGrantOptionsAndRejectsUnknowns(t *testing.T) {
	grantable := []RuntimeACLEffect{{
		Action: "GRANT", Grantee: "espacigo_runtime", ObjectKind: "TABLE",
		Object: "public.items", Privilege: "SELECT", GrantOption: true,
	}}
	got, err := foldRuntimeACLEffects(grantable)
	if err != nil {
		t.Fatal(err)
	}
	key := runtimeACLGrantKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.items", Privilege: "SELECT"}
	if !got[key].GrantOption || !got[key].Granted {
		t.Fatal("grant option was not preserved in the folded contract")
	}
	if _, err := foldRuntimeACLEffects([]RuntimeACLEffect{{
		Action: "GRANT", Grantee: "espacigo_runtime", ObjectKind: "TABLE",
		Object: "public.items", Privilege: "FUTURE_PRIVILEGE",
	}}); err == nil {
		t.Fatal("unsupported ACL effect was accepted")
	}
}

func TestBuildRuntimeACLContractCombinesAppliedMigrationsAndAdministrativeSource(t *testing.T) {
	sources := []RuntimeACLSource{
		{Version: 74, SQL: []byte(`GRANT INSERT ON TABLE public.rol_usuario_evento TO espacigo_runtime;`)},
		{Version: 79, SQL: []byte(`DO $g$ BEGIN GRANT UPDATE (reviewer_id) ON TABLE public.document_verification_reviewer_grant TO espacigo_runtime; END $g$;`)},
	}
	got, err := BuildRuntimeACLContract(sources)
	if err != nil {
		t.Fatal(err)
	}
	contains := func(grantee, kind, object, privilege, column string) bool {
		grant, ok := got[runtimeACLGrantKey{Grantee: grantee, ObjectKind: kind, Object: object, Privilege: privilege, Column: column}]
		return ok && grant.Granted
	}
	for _, expected := range []runtimeACLGrantKey{
		{Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.rol_usuario_evento", Privilege: "INSERT"},
		{Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.document_verification_reviewer_grant", Privilege: "UPDATE", Column: "reviewer_id"},
		{Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.usuario", Privilege: "SELECT"},
		{Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.registro_replay_baja_local", Privilege: "INSERT"},
	} {
		if !contains(expected.Grantee, expected.ObjectKind, expected.Object, expected.Privilege, expected.Column) {
			t.Errorf("source-derived contract is missing %+v", expected)
		}
	}
	if contains("espacigo_runtime", "TABLE", "public.tarifa_espacio", "DELETE", "") {
		t.Error("administrative override did not remove DELETE from the final contract")
	}
}

func TestBuildRuntimeACLContractAcceptsCheckedInMigrationHistory(t *testing.T) {
	migrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	sources := make([]RuntimeACLSource, len(migrations))
	for i, migration := range migrations {
		sources[i] = RuntimeACLSource{Version: int(migration.Version), SQL: migration.SQL}
	}
	contract, err := BuildRuntimeACLContract(sources)
	if err != nil {
		t.Fatalf("derive ACL contract from checked-in migration history: %v", err)
	}
	if len(contract) == 0 {
		t.Fatal("checked-in migration history produced an empty runtime ACL contract")
	}
	var dynamicConditionRecorded, sourceGrantorRecorded bool
	for key, grant := range contract {
		if key.Grantee == "espacigo_runtime" && grant.GrantOption {
			t.Fatalf("checked-in runtime ACL unexpectedly delegates grants on %s %s %s", key.ObjectKind, key.Object, key.Privilege)
		}
		if key.Grantee == "espacigo_runtime" && key.ObjectKind == "FUNCTION" &&
			key.Object == "public.purge_unreferenced_local_publication_rates(uuid)" && grant.Granted {
			_, dynamicConditionRecorded = grant.Conditions["role-exists:espacigo_runtime"]
			dynamicConditionRecorded = dynamicConditionRecorded && grant.DynamicSQL
			_, sourceGrantorRecorded = grant.Grantors[runtimeACLGrantorObjectOwner]
		}
	}
	if !dynamicConditionRecorded || !sourceGrantorRecorded {
		t.Fatalf("source-derived ACL contract omitted dynamic/condition/grantor metadata: dynamic_condition=%t grantor=%t", dynamicConditionRecorded, sourceGrantorRecorded)
	}
}

func TestBuildRuntimeACLContractRejectsUnorderedSourceHistory(t *testing.T) {
	sources := []RuntimeACLSource{
		{Version: 2, SQL: []byte("GRANT SELECT ON TABLE public.second_item TO espacigo_runtime;")},
		{Version: 1, SQL: []byte("GRANT SELECT ON TABLE public.first_item TO espacigo_runtime;")},
	}
	if _, err := BuildRuntimeACLContract(sources); err == nil {
		t.Fatal("unordered migration sources were silently reordered")
	}
}

func TestFormatRuntimeACLEffectProducesValidatedTargetedSQL(t *testing.T) {
	cases := []struct {
		name   string
		effect RuntimeACLEffect
		want   string
	}{
		{
			name:   "column grant",
			effect: RuntimeACLEffect{Action: "GRANT", Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.document_verification_case", Privilege: "UPDATE", Column: "reviewer_id"},
			want:   "GRANT UPDATE (reviewer_id) ON TABLE public.document_verification_case TO espacigo_runtime",
		},
		{
			name:   "public function revoke",
			effect: RuntimeACLEffect{Action: "REVOKE", Grantee: "PUBLIC", ObjectKind: "FUNCTION", Object: "public.document_verification_lock_space_row(uuid)", Privilege: "ALL PRIVILEGES"},
			want:   "REVOKE ALL PRIVILEGES ON FUNCTION public.document_verification_lock_space_row(uuid) FROM PUBLIC",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := formatRuntimeACLEffect(test.effect)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
	if _, err := formatRuntimeACLEffect(RuntimeACLEffect{Action: "GRANT", Grantee: "runtime; DROP ROLE postgres", ObjectKind: "TABLE", Object: "public.items", Privilege: "SELECT"}); err == nil {
		t.Fatal("unsafe grantee was accepted")
	}
}

func TestRuntimeACLRevokeBehaviorSurvivesParseAndReplay(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{
			source: "REVOKE SELECT ON TABLE public.items FROM espacigo_runtime CASCADE;",
			want:   "REVOKE SELECT ON TABLE public.items FROM espacigo_runtime CASCADE",
		},
		{
			source: "REVOKE UPDATE ON TABLE public.items FROM espacigo_runtime RESTRICT;",
			want:   "REVOKE UPDATE ON TABLE public.items FROM espacigo_runtime RESTRICT",
		},
	}
	for _, test := range cases {
		effects, err := extractRuntimeACLSourceEffects([]RuntimeACLSource{{Version: 79, SQL: []byte(test.source)}})
		if err != nil {
			t.Fatalf("extract %q: %v", test.source, err)
		}
		if len(effects) != 1 {
			t.Fatalf("parsed %d effects for %q, want one", len(effects), test.source)
		}
		got, err := formatRuntimeACLEffect(effects[0])
		if err != nil {
			t.Fatalf("format %q: %v", test.source, err)
		}
		if got != test.want {
			t.Errorf("replay %q = %q, want %q", test.source, got, test.want)
		}
	}
}

func TestRuntimeACLExtractionRecordsRoleConditionAndDynamicSQL(t *testing.T) {
	source := []byte(`DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        EXECUTE 'REVOKE SELECT ON TABLE public.items FROM espacigo_runtime CASCADE';
    END IF;
END
$$;`)
	effects, err := ExtractMigrationRuntimeACL(52, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 1 {
		t.Fatalf("extracted %d effects, want one: %+v", len(effects), effects)
	}
	if effects[0].RevokeBehavior != "CASCADE" {
		t.Errorf("revoke behavior = %q, want CASCADE", effects[0].RevokeBehavior)
	}
	if effects[0].Condition != "role-exists:espacigo_runtime" {
		t.Errorf("condition = %q, want role-exists:espacigo_runtime", effects[0].Condition)
	}
	if !effects[0].DynamicSQL {
		t.Error("effect extracted from EXECUTE literal was not marked dynamic")
	}
}

func TestRuntimeACLGrantorContractMatchesReviewedExecutionOwner(t *testing.T) {
	ownerGrant := runtimeACLObservedGrant{Grantor: "espacigo_document_verification_fn", Owner: "espacigo_document_verification_fn"}
	if !runtimeACLGrantorMatches(runtimeACLGrantorObjectOwner, ownerGrant) {
		t.Fatal("owner-derived grantor should match the actual object owner")
	}
	if runtimeACLGrantorMatches(runtimeACLGrantorObjectOwner, runtimeACLObservedGrant{Grantor: "postgres", Owner: ownerGrant.Owner}) {
		t.Fatal("a superuser name must not substitute for the effective object-owner grantor")
	}
	if !runtimeACLGrantorMatches("postgres", runtimeACLObservedGrant{Grantor: "postgres", Owner: "postgres"}) {
		t.Fatal("explicit grantor contract should match its exact role")
	}
	if runtimeACLGrantorMatches("postgres", ownerGrant) {
		t.Fatal("explicit grantor contract must not accept an alternate role")
	}
}

func TestRuntimeACLAllowsOnlyImplicitDefaultPublicFunctionExecute(t *testing.T) {
	key := runtimeACLResolvedKey{Grantee: "PUBLIC", ObjectKind: "FUNCTION", Privilege: "EXECUTE"}
	defaultExecute := runtimeACLObservedGrant{Grantor: "function_owner", Owner: "function_owner", Default: true}
	if !allowImplicitDefaultPublicFunctionExecute(key, defaultExecute, false) {
		t.Fatal("unmodified PostgreSQL default PUBLIC EXECUTE should be explicitly recognized")
	}
	cases := map[string]struct {
		key     runtimeACLResolvedKey
		grant   runtimeACLObservedGrant
		modeled bool
	}{
		"explicit grant":  {key, runtimeACLObservedGrant{Grantor: "function_owner", Owner: "function_owner"}, false},
		"grantor drift":   {key, runtimeACLObservedGrant{Grantor: "postgres", Owner: "function_owner", Default: true}, false},
		"grant option":    {key, runtimeACLObservedGrant{Grantor: "function_owner", Owner: "function_owner", Default: true, GrantOption: true}, false},
		"modeled effect":  {key, defaultExecute, true},
		"runtime grantee": {runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "FUNCTION", Privilege: "EXECUTE"}, defaultExecute, false},
		"other privilege": {runtimeACLResolvedKey{Grantee: "PUBLIC", ObjectKind: "FUNCTION", Privilege: "USAGE"}, defaultExecute, false},
	}
	for name, test := range cases {
		if allowImplicitDefaultPublicFunctionExecute(test.key, test.grant, test.modeled) {
			t.Errorf("case %q must not be accepted as an implicit default", name)
		}
	}
}

type runtimeACLObjectSourceRow struct{}

func (runtimeACLObjectSourceRow) Scan(destinations ...any) error {
	if len(destinations) != 1 {
		return errors.New("unexpected object lookup scan count")
	}
	id, ok := destinations[0].(*uint32)
	if !ok {
		return errors.New("unexpected object lookup destination")
	}
	*id = 42
	return nil
}

type runtimeACLSourceExecDB struct{ execs []string }

func (runtimeACLSourceExecDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected rows query")
}

func (db *runtimeACLSourceExecDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return runtimeACLObjectSourceRow{}
}

func (db *runtimeACLSourceExecDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	db.execs = append(db.execs, sql)
	return pgconn.NewCommandTag("GRANT"), nil
}

func TestApplyRuntimeACLFromMigrationsExecutesParsedSourceEffects(t *testing.T) {
	source := RuntimeACLSource{Version: 79, SQL: []byte(`GRANT SELECT ON TABLE public.items TO espacigo_runtime; REVOKE DELETE ON TABLE public.items FROM espacigo_runtime;`)}
	db := &runtimeACLSourceExecDB{}
	if err := ApplyRuntimeACLFromMigrations(context.Background(), db, []RuntimeACLSource{source}); err != nil {
		t.Fatalf("ApplyRuntimeACLFromMigrations: %v", err)
	}
	want := []string{
		"GRANT SELECT ON TABLE public.items TO espacigo_runtime",
		"REVOKE DELETE ON TABLE public.items FROM espacigo_runtime",
	}
	if len(db.execs) != len(want) {
		t.Fatalf("executed %d statements, want %d: %#v", len(db.execs), len(want), db.execs)
	}
	for i := range want {
		if db.execs[i] != want[i] {
			t.Errorf("executed source effect %d = %q, want %q", i, db.execs[i], want[i])
		}
	}
}

func TestBuildF5RuntimeACLContractPreservesLegacyPublicBaseline(t *testing.T) {
	sources := []RuntimeACLSource{
		{Version: 64, SQL: []byte(`REVOKE ALL ON FUNCTION public.legacy_cleanup() FROM PUBLIC;
GRANT SELECT ON TABLE public.items TO espacigo_runtime;`)},
		{Version: 81, SQL: []byte(`REVOKE ALL ON FUNCTION public.document_helper() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.document_helper() TO espacigo_runtime;`)},
	}
	got, err := buildF5RuntimeACLContract(sources)
	if err != nil {
		t.Fatal(err)
	}
	legacy := runtimeACLGrantKey{Grantee: "PUBLIC", ObjectKind: "FUNCTION", Object: "public.legacy_cleanup()", Privilege: "EXECUTE"}
	if _, exists := got[legacy]; exists {
		t.Fatal("F5 contract must preserve the pre-V81 PUBLIC baseline instead of replaying historical revokes")
	}
	newFunction := runtimeACLGrantKey{Grantee: "PUBLIC", ObjectKind: "FUNCTION", Object: "public.document_helper()", Privilege: "EXECUTE"}
	if grant, exists := got[newFunction]; !exists || grant.Granted {
		t.Fatalf("F5 contract must retain the V81 PUBLIC revoke, got %+v, exists=%v", grant, exists)
	}
	runtimeTable := runtimeACLGrantKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", Object: "public.items", Privilege: "SELECT"}
	if grant, exists := got[runtimeTable]; !exists || !grant.Granted {
		t.Fatalf("F5 contract lost the runtime grant: %+v, exists=%v", grant, exists)
	}
}

func TestRuntimeACLF5BaselineMatchesExactTargetAndFingerprints(t *testing.T) {
	if !runtimeACLF5BaselineMatches("espacigo_f5_gate_20261101", 180006, 5762, "527e02ed8bf413337ae39caf6e4edaa7", 0, "d41d8cd98f00b204e9800998ecf8427e") {
		t.Fatal("approved F5 PUBLIC/default baseline was rejected")
	}
	for _, changed := range []struct {
		database string
		version  int
		rows     int64
		public   string
		defaults int64
		defHash  string
	}{
		{"other_database", 180006, 5762, "527e02ed8bf413337ae39caf6e4edaa7", 0, "d41d8cd98f00b204e9800998ecf8427e"},
		{"espacigo_f5_gate_20261101", 180006, 5761, "527e02ed8bf413337ae39caf6e4edaa7", 0, "d41d8cd98f00b204e9800998ecf8427e"},
		{"espacigo_f5_gate_20261101", 180006, 5762, "different", 0, "d41d8cd98f00b204e9800998ecf8427e"},
		{"espacigo_f5_gate_20261101", 180006, 5762, "527e02ed8bf413337ae39caf6e4edaa7", 1, "d41d8cd98f00b204e9800998ecf8427e"},
		{"espacigo_f5_gate_20261101", 180006, 5762, "527e02ed8bf413337ae39caf6e4edaa7", 0, "different"},
		{"espacigo_f5_gate_20261101", 180007, 5762, "527e02ed8bf413337ae39caf6e4edaa7", 0, "d41d8cd98f00b204e9800998ecf8427e"},
	} {
		if runtimeACLF5BaselineMatches(changed.database, changed.version, changed.rows, changed.public, changed.defaults, changed.defHash) {
			t.Errorf("accepted altered F5 ACL baseline: %+v", changed)
		}
	}
}

func TestRuntimeACLF5RestoredBaselineRetainsAllOtherGuards(t *testing.T) {
	const restored = "37f89420b0338cdd11a978e57a04204c"
	if !runtimeACLF5BaselineMatches("espacigo_f5_gate_20261101", 180006, 5762, restored, 0, "d41d8cd98f00b204e9800998ecf8427e") {
		t.Fatal("schema-equivalent restored F5 baseline rejected")
	}
	if runtimeACLF5BaselineMatches("other", 180006, 5762, restored, 0, "d41d8cd98f00b204e9800998ecf8427e") ||
		runtimeACLF5BaselineMatches("espacigo_f5_gate_20261101", 180006, 5763, restored, 0, "d41d8cd98f00b204e9800998ecf8427e") ||
		runtimeACLF5BaselineMatches("espacigo_f5_gate_20261101", 180007, 5762, restored, 0, "d41d8cd98f00b204e9800998ecf8427e") ||
		runtimeACLF5BaselineMatches("espacigo_f5_gate_20261101", 180006, 5762, restored, 1, "d41d8cd98f00b204e9800998ecf8427e") ||
		runtimeACLF5BaselineMatches("espacigo_f5_gate_20261101", 180006, 5762, restored, 0, "different") {
		t.Fatal("restored baseline bypassed database/engine/count/default guards")
	}
}

type runtimeACLF5BaselineRow struct {
	database string
	version  int
	rows     int64
	public   string
	defaults int64
	defHash  string
}

func (r runtimeACLF5BaselineRow) Scan(destinations ...any) error {
	if len(destinations) != 6 {
		return errors.New("unexpected F5 baseline scan count")
	}
	values := []any{&r.database, &r.version, &r.rows, &r.public, &r.defaults, &r.defHash}
	for i, destination := range destinations {
		switch out := destination.(type) {
		case *string:
			*out = *values[i].(*string)
		case *int:
			*out = *values[i].(*int)
		case *int64:
			*out = *values[i].(*int64)
		default:
			return errors.New("unexpected F5 baseline scan destination")
		}
	}
	return nil
}

type runtimeACLF5BaselineDB struct {
	row   pgx.Row
	query string
	execs []string
}

func (db *runtimeACLF5BaselineDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected F5 baseline rows query")
}

func (db *runtimeACLF5BaselineDB) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	db.query = query
	if query == runtimeACLF5PublicBaselineQuery {
		return db.row
	}
	return runtimeACLObjectSourceRow{}
}

func (db *runtimeACLF5BaselineDB) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	db.execs = append(db.execs, query)
	return pgconn.NewCommandTag("GRANT"), nil
}

func TestVerifyRuntimeACLF5PublicBaselineReadsPinnedTargetSnapshot(t *testing.T) {
	db := &runtimeACLF5BaselineDB{row: runtimeACLF5BaselineRow{
		database: runtimeACLF5DatabaseName, version: runtimeACLF5BaselineServer,
		rows: runtimeACLF5PublicACLRows, public: runtimeACLF5PublicACLHash,
		defaults: runtimeACLF5DefaultACLRows, defHash: runtimeACLF5DefaultACLHash,
	}}
	if err := verifyRuntimeACLF5PublicBaseline(context.Background(), db); err != nil {
		t.Fatalf("approved F5 baseline rejected: %v", err)
	}
	if db.query != runtimeACLF5PublicBaselineQuery {
		t.Fatal("F5 baseline verifier did not use the complete catalog fingerprint query")
	}
	if strings.Contains(strings.ToUpper(db.query), "CREATE TEMP") {
		t.Fatal("F5 baseline readback must not create temporary objects")
	}

	db.row = runtimeACLF5BaselineRow{database: "other_database", version: runtimeACLF5BaselineServer,
		rows: runtimeACLF5PublicACLRows, public: runtimeACLF5PublicACLHash,
		defaults: runtimeACLF5DefaultACLRows, defHash: runtimeACLF5DefaultACLHash}
	if err := verifyRuntimeACLF5PublicBaseline(context.Background(), db); err == nil {
		t.Fatal("F5 baseline verifier accepted another database")
	}
}

func TestApplyRuntimeACLFromMigrationsF5PreservesLegacyPublicACL(t *testing.T) {
	db := &runtimeACLF5BaselineDB{row: runtimeACLF5BaselineRow{
		database: runtimeACLF5DatabaseName, version: runtimeACLF5BaselineServer,
		rows: runtimeACLF5PublicACLRows, public: runtimeACLF5PublicACLHash,
		defaults: runtimeACLF5DefaultACLRows, defHash: runtimeACLF5DefaultACLHash,
	}}
	sources := []RuntimeACLSource{
		{Version: 64, SQL: []byte(`REVOKE ALL ON FUNCTION public.legacy_cleanup() FROM PUBLIC;
GRANT SELECT ON TABLE public.items TO espacigo_runtime;`)},
		{Version: 81, SQL: []byte(`REVOKE ALL ON FUNCTION public.document_helper() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.document_helper() TO espacigo_runtime;`)},
	}
	if err := ApplyRuntimeACLFromMigrationsF5(context.Background(), db, sources); err != nil {
		t.Fatalf("ApplyRuntimeACLFromMigrationsF5: %v", err)
	}
	want := []string{
		"GRANT SELECT ON TABLE public.items TO espacigo_runtime",
		"GRANT EXECUTE ON FUNCTION public.document_helper() TO espacigo_runtime",
		"REVOKE ALL PRIVILEGES ON FUNCTION public.document_helper() FROM PUBLIC",
	}
	if len(db.execs) != len(want) {
		t.Fatalf("executed %d ACL effects, want %d: %#v", len(db.execs), len(want), db.execs)
	}
	for i := range want {
		if db.execs[i] != want[i] {
			t.Errorf("F5 effect %d = %q, want %q", i, db.execs[i], want[i])
		}
	}
}

func TestF5ReadbackUsesBaselinePlusDirectRuntimeAndRejectsExtraRuntimeACLs(t *testing.T) {
	if !runtimeACLEffectivePrivilegeExpected(false, true) {
		t.Fatal("effective access must include a privilege present in the pinned PUBLIC baseline")
	}
	if !runtimeACLEffectivePrivilegeExpected(true, false) {
		t.Fatal("effective access must include an exact direct runtime grant")
	}
	if runtimeACLEffectivePrivilegeExpected(false, false) {
		t.Fatal("effective access must not be inferred without PUBLIC or runtime grant")
	}
	publicKey := runtimeACLResolvedKey{Grantee: "PUBLIC", ObjectKind: "TABLE", ObjectID: "1", Privilege: "SELECT"}
	runtimeKey := runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", ObjectID: "1", Privilege: "SELECT"}
	if !ignoreUnmodeledF5PublicACL(publicKey, true) {
		t.Fatal("an unmodeled PUBLIC row covered by the verified full baseline must not be compared to historical source revokes")
	}
	if ignoreUnmodeledF5PublicACL(runtimeKey, true) {
		t.Fatal("an unmodeled direct runtime grant must never be ignored")
	}
	if ignoreUnmodeledF5PublicACL(publicKey, false) {
		t.Fatal("generic targets must retain the source-derived PUBLIC contract")
	}
}

func TestRuntimeACLDirectDriftOnlyExplainsMatchingMissingGrantEffectiveMismatch(t *testing.T) {
	key := runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", ObjectID: "42", Privilege: "INSERT", Column: "case_id"}
	drifts := map[runtimeACLResolvedKey]struct{}{key: {}}
	if !runtimeACLDirectDriftExplainsEffectiveMismatch(key, true, false, drifts) {
		t.Fatal("a missing source-required direct grant must explain its matching absent effective privilege")
	}
	if runtimeACLDirectDriftExplainsEffectiveMismatch(key, true, true, drifts) {
		t.Fatal("a missing grant must not explain an effective privilege that is present")
	}
	unrelated := key
	unrelated.Privilege = "SELECT"
	if runtimeACLDirectDriftExplainsEffectiveMismatch(unrelated, true, false, drifts) {
		t.Fatal("a direct INSERT drift must not mask an unrelated effective SELECT mismatch")
	}
}

func TestRuntimeACLDirectDifferenceRepairableOnlyForMissingRequiredGrant(t *testing.T) {
	tests := []struct {
		name           string
		expectedGrant  bool
		observedGrant  bool
		wantRepairable bool
	}{
		{name: "required grant missing", expectedGrant: true, wantRepairable: true},
		{name: "required grant present", expectedGrant: true, observedGrant: true},
		{name: "revoked grant remains", observedGrant: true},
		{name: "revoked grant absent"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := runtimeACLDirectDifferenceRepairable(test.expectedGrant, test.observedGrant); got != test.wantRepairable {
				t.Fatalf("repairable=%v, want %v", got, test.wantRepairable)
			}
		})
	}
}

func TestRuntimeACLRepairableDriftSummaryBoundsDiagnostics(t *testing.T) {
	details := []string{"one", "two", "three", "four", "five", "six", "seven", "eight"}
	count, summary := summarizeRuntimeACLRepairableDrifts(details)
	if count != len(details) {
		t.Fatalf("drift count=%d, want %d", count, len(details))
	}
	if !strings.Contains(summary, "one") || !strings.Contains(summary, "five") || strings.Contains(summary, "six") || !strings.Contains(summary, "and 3 more") {
		t.Fatalf("drift summary should retain five examples and bound the rest: %q", summary)
	}
}

type runtimeACLEffectivePrivilegeTestRow struct {
	value bool
	err   error
}

func (r runtimeACLEffectivePrivilegeTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	value, ok := dest[0].(*bool)
	if !ok {
		return errors.New("unexpected effective privilege scan destination")
	}
	*value = r.value
	return nil
}

type runtimeACLEffectivePrivilegeTestDB struct {
	values map[runtimeACLResolvedKey]bool
}

func (runtimeACLEffectivePrivilegeTestDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected rows query")
}

func (db runtimeACLEffectivePrivilegeTestDB) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	if len(args) == 0 && strings.Contains(query, "pg_proc p") {
		return runtimeACLEffectivePrivilegeTestRow{value: true}
	}
	if len(args) < 2 {
		return runtimeACLEffectivePrivilegeTestRow{err: errors.New("unexpected effective privilege query")}
	}
	key := runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", ObjectID: args[0].(string), Privilege: args[len(args)-1].(string)}
	if len(args) == 3 {
		key.Column = args[1].(string)
	}
	value, ok := db.values[key]
	if !ok {
		return runtimeACLEffectivePrivilegeTestRow{err: errors.New("missing fake effective privilege result")}
	}
	return runtimeACLEffectivePrivilegeTestRow{value: value}
}

func (runtimeACLEffectivePrivilegeTestDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func TestVerifyRuntimeACLEffectivePrivilegesDoesNotMaskUnrelatedMismatch(t *testing.T) {
	missing := runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", ObjectID: "42", Privilege: "INSERT", Column: "case_id"}
	unrelated := runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", ObjectID: "42", Privilege: "SELECT", Column: "id"}
	expected := map[runtimeACLResolvedKey]runtimeACLExpectedGrant{
		missing:   {Granted: true},
		unrelated: {Granted: true},
	}
	drifts := map[runtimeACLResolvedKey]struct{}{missing: {}}
	db := runtimeACLEffectivePrivilegeTestDB{values: map[runtimeACLResolvedKey]bool{missing: false, unrelated: false}}
	err := verifyRuntimeACLEffectivePrivileges(context.Background(), db, expected, nil, drifts)
	var repairable *RuntimeACLRepairableDriftError
	if err == nil || errors.As(err, &repairable) {
		t.Fatalf("unrelated effective SELECT mismatch must remain non-repairable, got %v", err)
	}
}

func TestVerifyRuntimeACLEffectivePrivilegesAllowsMatchingMissingGrantMismatch(t *testing.T) {
	missing := runtimeACLResolvedKey{Grantee: "espacigo_runtime", ObjectKind: "TABLE", ObjectID: "42", Privilege: "INSERT", Column: "case_id"}
	expected := map[runtimeACLResolvedKey]runtimeACLExpectedGrant{missing: {Granted: true}}
	drifts := map[runtimeACLResolvedKey]struct{}{missing: {}}
	db := runtimeACLEffectivePrivilegeTestDB{values: map[runtimeACLResolvedKey]bool{missing: false}}
	if err := verifyRuntimeACLEffectivePrivileges(context.Background(), db, expected, nil, drifts); err != nil {
		t.Fatalf("matching missing direct grant should be deferred until all other checks pass: %v", err)
	}
}

func TestGenericRuntimeACLContractStillReplaysHistoricalPublicRevokes(t *testing.T) {
	source := RuntimeACLSource{Version: 64, SQL: []byte(`REVOKE ALL ON FUNCTION public.legacy_cleanup() FROM PUBLIC;
GRANT SELECT ON TABLE public.items TO espacigo_runtime;`)}
	contract, err := BuildRuntimeACLContract([]RuntimeACLSource{source})
	if err != nil {
		t.Fatal(err)
	}
	publicKey := runtimeACLGrantKey{Grantee: "PUBLIC", ObjectKind: "FUNCTION", Object: "public.legacy_cleanup()", Privilege: "EXECUTE"}
	if grant, exists := contract[publicKey]; !exists || grant.Granted {
		t.Fatalf("generic contract no longer models the migration PUBLIC revoke: %+v, exists=%v", grant, exists)
	}
	db := &runtimeACLSourceExecDB{}
	if err := ApplyRuntimeACLFromMigrations(context.Background(), db, []RuntimeACLSource{source}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GRANT SELECT ON TABLE public.items TO espacigo_runtime",
		"REVOKE ALL PRIVILEGES ON FUNCTION public.legacy_cleanup() FROM PUBLIC",
	}
	if len(db.execs) != len(want) {
		t.Fatalf("generic target executed %d ACL effects, want %d: %#v", len(db.execs), len(want), db.execs)
	}
	for i := range want {
		if db.execs[i] != want[i] {
			t.Errorf("generic target effect %d = %q, want %q", i, db.execs[i], want[i])
		}
	}
}
