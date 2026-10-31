package documentverificationpg

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Opt-in coverage for the V81 function boundary. This test never applies DDL.
func TestPostgresV81FunctionOwnerAndLeastPrivilege(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to authorized F5 synthetic endpoint")
	}
	defer pool.Close()
	var currentDB string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&currentDB); err != nil || currentDB != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing unexpected integration database")
	}
	for _, signature := range []string{
		"public.document_verification_lock_active_actor(uuid)",
		"public.document_verification_lock_space_owner(uuid,uuid)",
		"public.document_verification_space_owned_by(uuid,uuid)",
		"public.document_verification_lock_space_row(uuid)",
	} {
		var owner string
		var superuser, createRole, createDB, replication, bypassRLS, inherit, canLogin, createPublic bool
		var volatile, securityDefiner, safeSearchPath, runtimeExecute, publicExecute bool
		err = pool.QueryRow(ctx, `SELECT r.rolname,r.rolsuper,r.rolcreaterole,r.rolcreatedb,r.rolreplication,r.rolbypassrls,r.rolinherit,r.rolcanlogin,has_schema_privilege(r.rolname,'public','CREATE'),p.provolatile='v',p.prosecdef,COALESCE(p.proconfig @> ARRAY['search_path=pg_catalog, pg_temp']::text[],false),has_function_privilege('espacigo_runtime',p.oid,'EXECUTE'),EXISTS(SELECT 1 FROM LATERAL aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, signature).Scan(&owner, &superuser, &createRole, &createDB, &replication, &bypassRLS, &inherit, &canLogin, &createPublic, &volatile, &securityDefiner, &safeSearchPath, &runtimeExecute, &publicExecute)
		if err != nil {
			t.Fatalf("V81 function metadata unavailable for %s; migration may be unapplied", signature)
		}
		if owner != "espacigo_document_verification_fn" || superuser || createRole || createDB || replication || bypassRLS || inherit || canLogin || createPublic || !volatile || !securityDefiner || !safeSearchPath || !runtimeExecute || publicExecute {
			t.Fatalf("function owner/ACL boundary is not least-privilege for %s", signature)
		}
	}
	var columnPrivileges []string
	err = pool.QueryRow(ctx, `SELECT COALESCE(array_agg(pg_catalog.format('%s.%s:%s',c.relname,a.attname,p.privilege) ORDER BY c.relname,a.attname,p.privilege),ARRAY[]::text[]) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN (VALUES ('SELECT'),('UPDATE')) AS p(privilege) WHERE n.nspname='public' AND c.relname IN ('usuario','espacio') AND pg_catalog.has_column_privilege($1,pg_catalog.format('%I.%I',n.nspname,c.relname),a.attname::text,p.privilege)`, "espacigo_document_verification_fn").Scan(&columnPrivileges)
	if err != nil {
		t.Fatal("function-owner effective column privileges are unavailable")
	}
	wantColumnPrivileges := []string{
		"espacio.id:SELECT",
		"espacio.id:UPDATE",
		"espacio.propietario_id:SELECT",
		"usuario.estado:SELECT",
		"usuario.id:SELECT",
		"usuario.id:UPDATE",
	}
	if !slices.Equal(columnPrivileges, wantColumnPrivileges) {
		t.Fatalf("function-owner column privileges=%v, want exactly %v", columnPrivileges, wantColumnPrivileges)
	}
	var schemaUsage, schemaCreate bool
	var canReadPassword, canReadEmail, canReadSpaceDetails, canUpdateSpaceOwner bool
	err = pool.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_document_verification_fn','public','USAGE'),has_schema_privilege('espacigo_document_verification_fn','public','CREATE'),has_column_privilege('espacigo_document_verification_fn','public.usuario','hash_clave','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.usuario','correo_original','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.usuario','correo_normalizado','SELECT'),has_column_privilege('espacigo_document_verification_fn','public.espacio','titulo','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.espacio','descripcion','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.espacio','direccion','SELECT'),has_column_privilege('espacigo_document_verification_fn','public.espacio','propietario_id','UPDATE')`).Scan(&schemaUsage, &schemaCreate, &canReadPassword, &canReadEmail, &canReadSpaceDetails, &canUpdateSpaceOwner)
	if err != nil || !schemaUsage || schemaCreate || canReadPassword || canReadEmail || canReadSpaceDetails || canUpdateSpaceOwner {
		t.Fatal("function-owner effective privileges exceed the row-locking checks")
	}
	var runtimeCanAssume, runtimeReadsSharedRows, runtimeCreate, runtimeDeletesSharedRows bool
	err = pool.QueryRow(ctx, `SELECT pg_catalog.pg_has_role('espacigo_runtime','espacigo_document_verification_fn','MEMBER'),has_table_privilege('espacigo_runtime','public.usuario','SELECT') AND has_table_privilege('espacigo_runtime','public.espacio','SELECT'),has_schema_privilege('espacigo_runtime','public','CREATE'),has_table_privilege('espacigo_runtime','public.usuario','DELETE') OR has_table_privilege('espacigo_runtime','public.espacio','DELETE')`).Scan(&runtimeCanAssume, &runtimeReadsSharedRows, &runtimeCreate, &runtimeDeletesSharedRows)
	if err != nil || runtimeCanAssume || !runtimeReadsSharedRows || runtimeCreate || runtimeDeletesSharedRows {
		t.Fatal("runtime must retain established shared-table SELECT without assuming the helper owner, creating in public, or deleting shared rows")
	}
}
