package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to authorized F5 synthetic endpoint")
	}
	defer pool.Close()
	var currentDB string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&currentDB); err != nil || currentDB != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing unexpected integration database")
	}
	for _, signature := range []string{
		"document_verification_private.document_verification_lock_active_actor(uuid)",
		"document_verification_private.document_verification_lock_space_owner(uuid,uuid)",
		"document_verification_private.document_verification_space_owned_by(uuid,uuid)",
		"document_verification_private.document_verification_lock_space_row(uuid)",
	} {
		var owner string
		var superuser, createRole, createDB, replication, bypassRLS, inherit, canLogin, createPublic bool
		var volatile, securityDefiner, safeSearchPath, runtimeExecute, publicExecute, privateCreate bool
		err = pool.QueryRow(ctx, `SELECT r.rolname,r.rolsuper,r.rolcreaterole,r.rolcreatedb,r.rolreplication,r.rolbypassrls,r.rolinherit,r.rolcanlogin,has_schema_privilege(r.rolname,'public','CREATE'),has_schema_privilege(r.rolname,'document_verification_private','CREATE'),p.provolatile='v',p.prosecdef,COALESCE(p.proconfig @> ARRAY['search_path=pg_catalog, pg_temp']::text[],false),has_function_privilege('espacigo_runtime',p.oid,'EXECUTE'),EXISTS(SELECT 1 FROM LATERAL aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, signature).Scan(&owner, &superuser, &createRole, &createDB, &replication, &bypassRLS, &inherit, &canLogin, &createPublic, &privateCreate, &volatile, &securityDefiner, &safeSearchPath, &runtimeExecute, &publicExecute)
		if err != nil {
			t.Fatalf("V81 function metadata unavailable for %s; migration may be unapplied", signature)
		}
		if owner != "espacigo_document_verification_fn" || superuser || createRole || createDB || replication || bypassRLS || inherit || canLogin || createPublic || !privateCreate || !volatile || !securityDefiner || !safeSearchPath || !runtimeExecute || publicExecute {
			t.Fatalf("function owner/ACL boundary is not least-privilege for %s", signature)
		}
	}
	var legacyPublicHelpers int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname IN ('document_verification_lock_active_actor','document_verification_lock_space_owner','document_verification_space_owned_by','document_verification_lock_space_row')`).Scan(&legacyPublicHelpers); err != nil || legacyPublicHelpers != 0 {
		t.Fatalf("V81 scoped helpers must not remain in public: count=%d err=%v", legacyPublicHelpers, err)
	}
	var helperRoleMemberships []string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(array_agg(parent.rolname ORDER BY parent.rolname),ARRAY[]::text[]) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles member ON member.oid=m.member JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid WHERE member.rolname='espacigo_document_verification_fn'`).Scan(&helperRoleMemberships); err != nil || len(helperRoleMemberships) != 0 {
		t.Fatalf("function owner has unintended role memberships: %v err=%v", helperRoleMemberships, err)
	}
	var roleContract bool
	if err := pool.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_migrator_login' AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit)
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login' AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit)
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit)
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit)
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n JOIN pg_catalog.pg_roles r ON r.oid=n.nspowner WHERE n.nspname='document_verification_private' AND r.rolname='espacigo_document_verification_fn')
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE parent.rolname='espacigo_schema_owner' AND member.rolname='espacigo_migrator_login' AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option)
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE parent.rolname='espacigo_document_verification_fn' AND member.rolname='espacigo_schema_owner' AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option)
		AND EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE parent.rolname='espacigo_runtime' AND member.rolname='espacigo_api_login' AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option)
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles member ON member.oid=m.member JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid WHERE member.rolname='espacigo_migrator_login' AND parent.rolname<>'espacigo_schema_owner')
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles member ON member.oid=m.member JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid WHERE member.rolname='espacigo_api_login' AND parent.rolname<>'espacigo_runtime')
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles member ON member.oid=m.member JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid WHERE member.rolname='espacigo_schema_owner' AND parent.rolname<>'espacigo_document_verification_fn')
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid WHERE parent.rolname IN ('espacigo_migrator_login','espacigo_api_login'))
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE parent.rolname='espacigo_schema_owner' AND member.rolname<>'espacigo_migrator_login')
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE member.rolname IN ('espacigo_document_verification_fn','espacigo_runtime'))
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE parent.rolname='espacigo_document_verification_fn' AND member.rolname<>'espacigo_schema_owner')
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND (has_schema_privilege('espacigo_migrator_login',n.oid,'CREATE') OR has_schema_privilege('espacigo_schema_owner',n.oid,'CREATE') OR has_schema_privilege('espacigo_api_login',n.oid,'CREATE') OR has_schema_privilege('espacigo_runtime',n.oid,'CREATE')))
		AND NOT has_database_privilege('espacigo_migrator_login',current_database(),'CREATE')
		AND NOT has_database_privilege('espacigo_api_login',current_database(),'CREATE')
		AND NOT has_database_privilege('espacigo_schema_owner',current_database(),'CREATE')
		AND NOT has_database_privilege('espacigo_runtime',current_database(),'CREATE')
		AND NOT has_database_privilege('espacigo_document_verification_fn',current_database(),'CREATE')
		AND has_table_privilege('espacigo_schema_owner','public.schema_migrations','SELECT')
		AND has_table_privilege('espacigo_schema_owner','public.schema_migrations','INSERT')`).Scan(&roleContract); err != nil || !roleContract {
		t.Fatalf("migrator/helper/runtime role and ledger contract is not least-privilege: valid=%t err=%v", roleContract, err)
	}
	var columnPrivileges []string
	err = pool.QueryRow(ctx, `SELECT COALESCE(array_agg(pg_catalog.format('%s.%s.%s:%s',n.nspname,c.relname,a.attname,p.privilege) ORDER BY n.nspname,c.relname,a.attname,p.privilege),ARRAY[]::text[]) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN (VALUES ('SELECT'),('INSERT'),('UPDATE'),('REFERENCES')) AS p(privilege) WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind IN ('r','p','v','m','f') AND pg_catalog.has_column_privilege($1,pg_catalog.format('%I.%I',n.nspname,c.relname),a.attname::text,p.privilege)`, "espacigo_document_verification_fn").Scan(&columnPrivileges)
	if err != nil {
		t.Fatal("function-owner effective column privileges are unavailable")
	}
	wantColumnPrivileges := []string{
		"public.espacio.id:SELECT",
		"public.espacio.id:UPDATE",
		"public.espacio.propietario_id:SELECT",
		"public.usuario.estado:SELECT",
		"public.usuario.id:SELECT",
		"public.usuario.id:UPDATE",
	}
	if !slices.Equal(columnPrivileges, wantColumnPrivileges) {
		t.Fatalf("function-owner column privileges=%v, want exactly %v", columnPrivileges, wantColumnPrivileges)
	}
	var broadTablePrivileges, sequencePrivileges, schemaCreatePrivileges []string
	err = pool.QueryRow(ctx, `SELECT COALESCE(array_agg(pg_catalog.format('%s.%s:%s',n.nspname,c.relname,p.privilege) ORDER BY n.nspname,c.relname,p.privilege),ARRAY[]::text[]) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN (VALUES ('SELECT'),('INSERT'),('UPDATE'),('DELETE'),('TRUNCATE'),('REFERENCES'),('TRIGGER')) AS p(privilege) WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind IN ('r','p','v','m','f') AND pg_catalog.has_table_privilege('espacigo_document_verification_fn',pg_catalog.format('%I.%I',n.nspname,c.relname),p.privilege)`).Scan(&broadTablePrivileges)
	if err != nil || len(broadTablePrivileges) != 0 {
		t.Fatalf("function-owner has unintended table-level privileges: %v err=%v", broadTablePrivileges, err)
	}
	err = pool.QueryRow(ctx, `SELECT COALESCE(array_agg(pg_catalog.format('%s.%s:%s',n.nspname,c.relname,p.privilege) ORDER BY n.nspname,c.relname,p.privilege),ARRAY[]::text[]) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN (VALUES ('SELECT'),('USAGE'),('UPDATE')) AS p(privilege) WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind='S' AND pg_catalog.has_sequence_privilege('espacigo_document_verification_fn',pg_catalog.format('%I.%I',n.nspname,c.relname),p.privilege)`).Scan(&sequencePrivileges)
	if err != nil || len(sequencePrivileges) != 0 {
		t.Fatalf("function-owner has unintended sequence privileges: %v err=%v", sequencePrivileges, err)
	}
	err = pool.QueryRow(ctx, `SELECT COALESCE(array_agg(n.nspname ORDER BY n.nspname),ARRAY[]::text[]) FROM pg_catalog.pg_namespace n WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND pg_catalog.has_schema_privilege('espacigo_document_verification_fn',n.oid,'CREATE')`).Scan(&schemaCreatePrivileges)
	if err != nil || !slices.Equal(schemaCreatePrivileges, []string{"document_verification_private"}) {
		t.Fatalf("function-owner has schema CREATE privileges: %v err=%v", schemaCreatePrivileges, err)
	}
	for _, signature := range []string{
		"document_verification_private.document_verification_lock_active_actor(uuid)",
		"document_verification_private.document_verification_lock_space_owner(uuid,uuid)",
		"document_verification_private.document_verification_space_owned_by(uuid,uuid)",
		"document_verification_private.document_verification_lock_space_row(uuid)",
	} {
		var grants []string
		err = pool.QueryRow(ctx, `SELECT COALESCE(array_agg(COALESCE(g.rolname,'PUBLIC')||':'||a.privilege_type ORDER BY COALESCE(g.rolname,'PUBLIC'),a.privilege_type),ARRAY[]::text[]) FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) a LEFT JOIN pg_catalog.pg_roles g ON g.oid=a.grantee WHERE p.oid=$1::regprocedure`, signature).Scan(&grants)
		if err != nil || !slices.Equal(grants, []string{"espacigo_document_verification_fn:EXECUTE", "espacigo_runtime:EXECUTE"}) {
			t.Fatalf("unexpected execute ACL for %s: %v err=%v", signature, grants, err)
		}
	}
	var schemaUsage, schemaCreate bool
	var canReadPassword, canReadEmail, canReadSpaceDetails, canUpdateSpaceOwner bool
	err = pool.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_document_verification_fn','public','USAGE'),has_schema_privilege('espacigo_document_verification_fn','public','CREATE'),has_column_privilege('espacigo_document_verification_fn','public.usuario','hash_clave','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.usuario','correo_original','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.usuario','correo_normalizado','SELECT'),has_column_privilege('espacigo_document_verification_fn','public.espacio','titulo','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.espacio','descripcion','SELECT') OR has_column_privilege('espacigo_document_verification_fn','public.espacio','direccion','SELECT'),has_column_privilege('espacigo_document_verification_fn','public.espacio','propietario_id','UPDATE')`).Scan(&schemaUsage, &schemaCreate, &canReadPassword, &canReadEmail, &canReadSpaceDetails, &canUpdateSpaceOwner)
	if err != nil || !schemaUsage || schemaCreate || canReadPassword || canReadEmail || canReadSpaceDetails || canUpdateSpaceOwner {
		t.Fatal("function-owner effective privileges exceed the row-locking checks")
	}
	var runtimeCanAssume, runtimeReadsSharedRows, runtimeCreate, runtimePrivateCreate, runtimePrivateUsage, runtimeDeletesSharedRows bool
	err = pool.QueryRow(ctx, `SELECT pg_catalog.pg_has_role('espacigo_runtime','espacigo_document_verification_fn','MEMBER'),has_table_privilege('espacigo_runtime','public.usuario','SELECT') AND has_table_privilege('espacigo_runtime','public.espacio','SELECT'),has_schema_privilege('espacigo_runtime','public','CREATE'),has_schema_privilege('espacigo_runtime','document_verification_private','CREATE'),has_schema_privilege('espacigo_runtime','document_verification_private','USAGE'),has_table_privilege('espacigo_runtime','public.usuario','DELETE') OR has_table_privilege('espacigo_runtime','public.espacio','DELETE')`).Scan(&runtimeCanAssume, &runtimeReadsSharedRows, &runtimeCreate, &runtimePrivateCreate, &runtimePrivateUsage, &runtimeDeletesSharedRows)
	if err != nil || runtimeCanAssume || !runtimeReadsSharedRows || runtimeCreate || runtimePrivateCreate || !runtimePrivateUsage || runtimeDeletesSharedRows {
		t.Fatal("runtime must retain established shared-table SELECT and private-schema USAGE without assuming the helper owner, creating in either schema, or deleting shared rows")
	}
}

func TestPostgresV81ActorLockSerializesAccountStateChange(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to authorized F5 synthetic endpoint")
	}
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_gate_20261101" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	actorID := uuid.NewString()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM public.usuario WHERE id=$1`, actorID); err != nil {
			t.Errorf("delete V81 actor-state fixture: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM public.usuario WHERE id=$1`, actorID).Scan(&remaining); err != nil {
			t.Errorf("verify V81 actor-state fixture cleanup: %v", err)
		} else if remaining != 0 {
			t.Errorf("V81 actor-state fixture cleanup left %d rows", remaining)
		}
	})
	if _, err := pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actorID, actorID+"@example.invalid"); err != nil {
		t.Fatal("cannot seed V81 actor-state fixture")
	}
	lockTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal("cannot begin V81 actor-lock transaction")
	}
	t.Cleanup(func() {
		if err := lockTx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback V81 actor-lock transaction: %v", err)
		}
	})
	if _, err := lockTx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("cannot assume restricted runtime role for actor lock")
	}
	var active bool
	if err := lockTx.QueryRow(ctx, `SELECT document_verification_private.document_verification_lock_active_actor($1)`, actorID).Scan(&active); err != nil || !active {
		t.Fatal("runtime did not acquire the active-actor row lock")
	}
	updateTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal("cannot begin competing account-state update")
	}
	if _, err := updateTx.Exec(ctx, `SET LOCAL lock_timeout='250ms'`); err != nil {
		t.Fatal("cannot set bounded lock wait for actor-state race")
	}
	_, updateErr := updateTx.Exec(ctx, `UPDATE public.usuario SET estado='bloqueado' WHERE id=$1`, actorID)
	var lockError *pgconn.PgError
	if !errors.As(updateErr, &lockError) || lockError.Code != "55P03" {
		_ = updateTx.Rollback(context.Background())
		t.Fatalf("actor state update was not blocked by the helper lock: %v", updateErr)
	}
	if err := updateTx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("rollback blocked account-state update")
	}
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatal("release V81 actor lock")
	}
	tag, err := pool.Exec(ctx, `UPDATE public.usuario SET estado='bloqueado' WHERE id=$1`, actorID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("account-state update did not proceed after actor lock release: rows=%d err=%v", tag.RowsAffected(), err)
	}
}
