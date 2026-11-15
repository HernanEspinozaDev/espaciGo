package migrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	v97AuthorityDatabasePrefix = "migrator_test_"
	v97AuthorityUser           = "postgres"
	v97AuthorityMigration      = "V000097__commercial_rental_quote_snapshots.sql"
	v97AuthorityChecksum       = "0574033dde36b66efc00f36e20fcedbb424badc20c7109f941f78786fb5d8e14"
	v97AuthorityInventory      = "134a3f0825eb1895acc564f2b05472ae83c625648791d2a8905c3500412d42a4"

	v97GrantCreateSQL             = `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`
	v97GrantUsuarioReferencesSQL  = `GRANT REFERENCES (id) ON TABLE public.usuario TO espacigo_schema_owner`
	v97GrantEspacioReferencesSQL  = `GRANT REFERENCES (id) ON TABLE public.espacio TO espacigo_schema_owner`
	v97RevokeCreateSQL            = `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`
	v97RevokeUsuarioReferencesSQL = `REVOKE REFERENCES (id) ON TABLE public.usuario FROM espacigo_schema_owner`
	v97RevokeEspacioReferencesSQL = `REVOKE REFERENCES (id) ON TABLE public.espacio FROM espacigo_schema_owner`
)

type v97AuthorityReport struct {
	expectedDatabase string
	Format           string `json:"format"`
	Mode             string `json:"mode"`
	Database         string `json:"database"`
	SessionUser      string `json:"session_user"`
	CurrentUser      string `json:"current_user"`
	AppliedCount     int    `json:"applied_count"`
	BaselineSHA256   string `json:"baseline_sha256"`
	AuthorityClosed  bool   `json:"authority_closed"`
}

type v97AuthorityQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func validateV97AuthorityMode(mode, baseline string) error {
	if mode != "inspect" && mode != "prepare" && mode != "finalize" {
		return errors.New("invalid V97 authority mode")
	}
	if mode == "finalize" {
		if !validHex(baseline, 64) {
			return errors.New("finalize requires a lowercase SHA-256 baseline")
		}
	} else if baseline != "" {
		return errors.New("baseline SHA-256 is valid only in finalize mode")
	}
	return nil
}

func validateV97MigrationInventory(migrations []Migration) error {
	if len(migrations) != 97 || inventoryFingerprint(migrations) != v97AuthorityInventory {
		return errors.New("V97 authority migration inventory fingerprint mismatch")
	}
	for i, migration := range migrations {
		if migration.Version != int64(i+1) {
			return errors.New("V97 authority migration sequence is not contiguous V1..V97")
		}
	}
	last := migrations[96]
	if last.Version != 97 || last.Name != v97AuthorityMigration || last.Checksum != v97AuthorityChecksum {
		return errors.New("V97 migration name or checksum mismatch")
	}
	return nil
}

// manageV97AuthorityWithConn is available only to migration tests. The helper
// source lives in a _test.go file so it cannot be used by production callers.
func manageV97AuthorityWithConn(ctx context.Context, conn *pgx.Conn, migrationDir, mode, baseline string) (v97AuthorityReport, error) {
	var report v97AuthorityReport
	if err := validateV97AuthorityMode(mode, baseline); err != nil {
		return report, err
	}
	if conn == nil {
		return report, errors.New("V97 authority connection is required")
	}
	expectedDatabase, err := validateV97FixtureTarget(conn)
	if err != nil {
		return report, err
	}
	report.expectedDatabase = expectedDatabase
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return report, err
	}
	if err := validateV97MigrationInventory(migrations); err != nil {
		return report, err
	}
	unlock, err := AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return report, err
	}
	defer func() { _ = unlock(context.Background()) }()

	if mode == "finalize" {
		if err := inspectV97Target(ctx, conn, &report); err != nil {
			return report, err
		}
		if err := revokeV97Authority(ctx, conn); err != nil {
			return report, err
		}
		return verifyV97FinalizedState(ctx, conn, migrations, baseline, report)
	}

	access := pgx.ReadOnly
	if mode == "prepare" {
		access = pgx.ReadWrite
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: access})
	if err != nil {
		return report, errors.New("begin V97 authority inspection")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := inspectV97Target(ctx, tx, &report); err != nil {
		return report, err
	}
	if err := validateV97RoleAuthority(ctx, tx); err != nil {
		return report, err
	}
	if err := inspectV97History(ctx, tx, migrations, &report); err != nil {
		return report, err
	}
	currentBaseline, closed, err := v97AuthorityBaseline(ctx, tx)
	if err != nil {
		return report, err
	}
	report.Mode, report.BaselineSHA256, report.AuthorityClosed = mode, currentBaseline, closed
	if mode == "inspect" {
		return report, nil
	}
	if report.AppliedCount != 96 || !closed {
		return report, errors.New("V97 prepare requires exact V96 ledger and no temporary DDL privileges")
	}
	for _, statement := range []string{v97GrantCreateSQL, v97GrantUsuarioReferencesSQL, v97GrantEspacioReferencesSQL} {
		if _, err := tx.Exec(ctx, statement); err != nil {
			_ = tx.Rollback(context.Background())
			return report, errors.New("grant exact V97 temporary DDL privilege")
		}
	}
	if err := verifyV97TemporaryPrivileges(ctx, tx, true); err != nil {
		_ = tx.Rollback(context.Background())
		return report, err
	}
	if err := tx.Commit(ctx); err != nil {
		return report, errors.New("commit V97 temporary DDL privileges")
	}
	readback, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return report, errors.New("begin committed V97 grant readback; finalize required")
	}
	defer func() { _ = readback.Rollback(context.Background()) }()
	if err := inspectV97Target(ctx, readback, &report); err != nil {
		return report, err
	}
	if err := verifyV97TemporaryPrivileges(ctx, readback, true); err != nil {
		return report, err
	}
	report.AuthorityClosed = false
	return report, nil
}

func inspectV97Target(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, report *v97AuthorityReport) error {
	report.Format = "espacigo.v97-authority.v1"
	if err := q.QueryRow(ctx, `SELECT current_database(),session_user,current_user`).Scan(&report.Database, &report.SessionUser, &report.CurrentUser); err != nil {
		return errors.New("read V97 authority target identity")
	}
	if report.expectedDatabase == "" || report.Database != report.expectedDatabase || report.SessionUser != v97AuthorityUser || report.CurrentUser != v97AuthorityUser {
		return errors.New("V97 authority target identity mismatch")
	}
	return nil
}

func inspectV97History(ctx context.Context, q v97AuthorityQueryer, migrations []Migration, report *v97AuthorityReport) error {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind IN ('r','p'))`).Scan(&exists); err != nil || !exists {
		return errors.New("V97 authority requires an existing migration ledger")
	}
	history, err := readHistory(ctx, q)
	if err != nil {
		return err
	}
	if err := verifyHistory(migrations, history); err != nil {
		return err
	}
	if len(history) != 96 && len(history) != 97 {
		return errors.New("V97 authority requires the exact V96 or V97 ledger prefix")
	}
	report.AppliedCount = len(history)
	var existsQuote bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('public.cotizacion_arriendo_comercial') IS NOT NULL`).Scan(&existsQuote); err != nil {
		return errors.New("inspect V97 commercial quote table")
	}
	if existsQuote != (len(history) == 97) {
		return errors.New("V97 ledger and commercial quote table state disagree")
	}
	if existsQuote {
		var owned bool
		if err := q.QueryRow(ctx, `SELECT pg_catalog.pg_get_userbyid(c.relowner)='espacigo_schema_owner' FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='cotizacion_arriendo_comercial'`).Scan(&owned); err != nil || !owned {
			return errors.New("V97 commercial quote table is not owned by schema owner")
		}
		if err := verifyV97RuntimePrivileges(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func validateV97RoleAuthority(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	const query = `WITH expected(name,super,createrole,createdb,inherit,login,bypassrls,replication) AS (VALUES
 ('espacigo_schema_owner',false,false,false,false,false,false,false),
 ('espacigo_runtime',false,false,false,false,false,false,false),
 ('espacigo_document_verification_fn',false,false,false,false,false,false,false),
 ('espacigo_migrator_login',false,false,false,false,true,false,false),
 ('espacigo_api_login',false,false,false,false,true,false,false)),
 actual AS (SELECT r.rolname,r.rolsuper,r.rolcreaterole,r.rolcreatedb,r.rolinherit,r.rolcanlogin,r.rolbypassrls,r.rolreplication FROM pg_catalog.pg_roles r)
 SELECT (SELECT count(*)=5 FROM expected e JOIN actual a ON a.rolname=e.name AND (a.rolsuper,a.rolcreaterole,a.rolcreatedb,a.rolinherit,a.rolcanlogin,a.rolbypassrls,a.rolreplication)=(e.super,e.createrole,e.createdb,e.inherit,e.login,e.bypassrls,e.replication)),
 (SELECT count(*)=3 FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles g ON g.oid=am.roleid JOIN pg_catalog.pg_roles m ON m.oid=am.member WHERE (g.rolname,m.rolname,am.admin_option,am.inherit_option,am.set_option) IN (('espacigo_document_verification_fn','espacigo_schema_owner',false,false,true),('espacigo_runtime','espacigo_api_login',false,false,true),('espacigo_schema_owner','espacigo_migrator_login',false,false,true)) AND (g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn') OR m.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn'))),
 (SELECT count(*)=0 FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles g ON g.oid=am.roleid JOIN pg_catalog.pg_roles m ON m.oid=am.member WHERE (g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn') OR m.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn')) AND NOT ((g.rolname,m.rolname,am.admin_option,am.inherit_option,am.set_option) IN (('espacigo_document_verification_fn','espacigo_schema_owner',false,false,true),('espacigo_runtime','espacigo_api_login',false,false,true),('espacigo_schema_owner','espacigo_migrator_login',false,false,true)))),
 (SELECT pg_catalog.pg_get_userbyid(n.nspowner)='pg_database_owner' FROM pg_catalog.pg_namespace n WHERE n.nspname='public'),
 (SELECT pg_catalog.pg_get_userbyid(c.relowner)='postgres' FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='usuario'),
 (SELECT pg_catalog.pg_get_userbyid(c.relowner)='postgres' FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='espacio'),
 (SELECT pg_catalog.pg_get_userbyid(c.relowner)='postgres' FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations')`
	var roles, edges, noExtra, schemaOwner, userOwner, spaceOwner, ledgerOwner bool
	if err := q.QueryRow(ctx, query).Scan(&roles, &edges, &noExtra, &schemaOwner, &userOwner, &spaceOwner, &ledgerOwner); err != nil {
		return errors.New("read V97 role authority contract")
	}
	if !roles || !edges || !noExtra || !schemaOwner || !userOwner || !spaceOwner || !ledgerOwner {
		return errors.New("V97 role flags, memberships, or existing object owners differ from the fixed contract")
	}
	return nil
}

func v97AuthorityBaseline(ctx context.Context, q v97AuthorityQueryer) (string, bool, error) {
	const query = `WITH tuples(v) AS (
 SELECT 'schema_acl|'||CASE WHEN n.nspacl IS NULL THEN '<NULL>' ELSE COALESCE((SELECT string_agg(x::text, ',' ORDER BY x::text) FROM unnest(n.nspacl) x),'') END FROM pg_catalog.pg_namespace n WHERE n.nspname='public'
 UNION ALL SELECT 'table_acl|'||c.relname||'|'||CASE WHEN c.relacl IS NULL THEN '<NULL>' ELSE COALESCE((SELECT string_agg(x::text, ',' ORDER BY x::text) FROM unnest(c.relacl) x),'') END FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('usuario','espacio')
 UNION ALL SELECT 'column_acl|'||c.relname||'|'||a.attname||'|'||CASE WHEN a.attacl IS NULL THEN '<NULL>' ELSE COALESCE((SELECT string_agg(x::text, ',' ORDER BY x::text) FROM unnest(a.attacl) x),'') END FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('usuario','espacio') AND a.attnum>0 AND NOT a.attisdropped
 UNION ALL SELECT 'role|'||rolname||'|'||rolsuper||'|'||rolcreaterole||'|'||rolcreatedb||'|'||rolinherit||'|'||rolcanlogin||'|'||rolbypassrls||'|'||rolreplication FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn','espacigo_migrator_login','espacigo_api_login')
 UNION ALL SELECT 'member|'||g.rolname||'|'||m.rolname||'|'||am.admin_option||'|'||am.inherit_option||'|'||am.set_option FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles g ON g.oid=am.roleid JOIN pg_catalog.pg_roles m ON m.oid=am.member WHERE g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn') OR m.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn')
 UNION ALL SELECT 'owner|'||n.nspname||'|'||c.relname||'|'||pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('usuario','espacio','schema_migrations')
 UNION ALL SELECT 'schema_owner|'||n.nspname||'|'||pg_catalog.pg_get_userbyid(n.nspowner) FROM pg_catalog.pg_namespace n WHERE n.nspname='public'
)
SELECT v,
 pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE'),
 pg_catalog.has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES'),
 pg_catalog.has_column_privilege('espacigo_schema_owner','public.espacio','id','REFERENCES'),
 pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE WITH GRANT OPTION'),
 pg_catalog.has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES WITH GRANT OPTION') OR pg_catalog.has_column_privilege('espacigo_schema_owner','public.espacio','id','REFERENCES WITH GRANT OPTION'),
 pg_catalog.has_table_privilege('espacigo_schema_owner','public.usuario','REFERENCES') OR pg_catalog.has_table_privilege('espacigo_schema_owner','public.espacio','REFERENCES'),
 EXISTS(SELECT 1 FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('usuario','espacio') AND a.attnum>0 AND NOT a.attisdropped AND a.attname<>'id' AND pg_catalog.has_column_privilege('espacigo_schema_owner',a.attrelid,a.attname,'REFERENCES'))
FROM tuples ORDER BY v`
	rows, err := q.Query(ctx, query)
	if err != nil {
		return "", false, errors.New("read V97 authority baseline")
	}
	defer rows.Close()
	var tuples []string
	closed := true
	for rows.Next() {
		var tuple string
		var create, userRef, spaceRef, createOption, refOption, tableRefs, otherRefs bool
		if err := rows.Scan(&tuple, &create, &userRef, &spaceRef, &createOption, &refOption, &tableRefs, &otherRefs); err != nil {
			return "", false, err
		}
		tuples = append(tuples, tuple)
		closed = closed && !create && !userRef && !spaceRef && !createOption && !refOption && !tableRefs && !otherRefs
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	if len(tuples) == 0 {
		return "", false, errors.New("V97 authority baseline is empty")
	}
	sort.Strings(tuples)
	sum := sha256.Sum256([]byte(strings.Join(tuples, "\n") + "\n"))
	return hex.EncodeToString(sum[:]), closed, nil
}

func verifyV97TemporaryPrivileges(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, expected bool) error {
	var create, userRef, spaceRef, createOption, refOption, tableRefs, otherRefs bool
	const effective = `SELECT
 pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE'),
 pg_catalog.has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES'),
 pg_catalog.has_column_privilege('espacigo_schema_owner','public.espacio','id','REFERENCES'),
 pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE WITH GRANT OPTION'),
 pg_catalog.has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES WITH GRANT OPTION') OR pg_catalog.has_column_privilege('espacigo_schema_owner','public.espacio','id','REFERENCES WITH GRANT OPTION'),
 pg_catalog.has_table_privilege('espacigo_schema_owner','public.usuario','REFERENCES') OR pg_catalog.has_table_privilege('espacigo_schema_owner','public.espacio','REFERENCES'),
 EXISTS(SELECT 1 FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('usuario','espacio') AND a.attnum>0 AND NOT a.attisdropped AND a.attname<>'id' AND pg_catalog.has_column_privilege('espacigo_schema_owner',a.attrelid,a.attname,'REFERENCES'))`
	if err := q.QueryRow(ctx, effective).Scan(&create, &userRef, &spaceRef, &createOption, &refOption, &tableRefs, &otherRefs); err != nil {
		return errors.New("read V97 effective temporary DDL privileges")
	}
	if create != expected || userRef != expected || spaceRef != expected || createOption || refOption || tableRefs || otherRefs {
		return errors.New("V97 effective temporary privileges are missing, broad, or grantable")
	}
	var count int
	var exactCreate, exactUsuario, exactEspacio bool
	const direct = `WITH grants(kind,grantor,privilege_type,is_grantable) AS (
 SELECT 'schema',a.grantor,a.privilege_type,a.is_grantable FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(n.nspacl)>0 THEN n.nspacl ELSE NULL::aclitem[] END) a WHERE n.nspname='public' AND a.grantee='espacigo_schema_owner'::regrole AND a.privilege_type='CREATE'
 UNION ALL SELECT 'usuario:'||c.attname,a.grantor,a.privilege_type,a.is_grantable FROM pg_catalog.pg_attribute c JOIN pg_catalog.pg_class t ON t.oid=c.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(c.attacl)>0 THEN c.attacl ELSE NULL::aclitem[] END) a WHERE n.nspname='public' AND t.relname='usuario' AND c.attnum>0 AND NOT c.attisdropped AND a.grantee='espacigo_schema_owner'::regrole AND a.privilege_type='REFERENCES'
 UNION ALL SELECT 'espacio:'||c.attname,a.grantor,a.privilege_type,a.is_grantable FROM pg_catalog.pg_attribute c JOIN pg_catalog.pg_class t ON t.oid=c.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(c.attacl)>0 THEN c.attacl ELSE NULL::aclitem[] END) a WHERE n.nspname='public' AND t.relname='espacio' AND c.attnum>0 AND NOT c.attisdropped AND a.grantee='espacigo_schema_owner'::regrole AND a.privilege_type='REFERENCES'
)
SELECT count(*),
 count(*) FILTER (WHERE kind='schema' AND privilege_type='CREATE' AND NOT is_grantable)=1,
 count(*) FILTER (WHERE kind='usuario:id' AND privilege_type='REFERENCES' AND NOT is_grantable)=1,
 count(*) FILTER (WHERE kind='espacio:id' AND privilege_type='REFERENCES' AND NOT is_grantable)=1
FROM grants`
	if err := q.QueryRow(ctx, direct).Scan(&count, &exactCreate, &exactUsuario, &exactEspacio); err != nil {
		return errors.New("read exact V97 direct privilege tuples")
	}
	wantCount := 0
	if expected {
		wantCount = 3
	}
	if count != wantCount || exactCreate != expected || exactUsuario != expected || exactEspacio != expected {
		return fmt.Errorf("V97 direct privilege tuples differ from expected state (expected=%t count=%d create=%t usuario=%t espacio=%t)", expected, count, exactCreate, exactUsuario, exactEspacio)
	}
	return nil
}

func verifyV97RuntimePrivileges(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	var selectInsertDelete, update, truncate, references, grantOption bool
	const effective = `SELECT
 pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','SELECT,INSERT,DELETE'),
 pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','UPDATE'),
 pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','TRUNCATE'),
 pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','REFERENCES'),
 pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','SELECT WITH GRANT OPTION') OR pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','INSERT WITH GRANT OPTION') OR pg_catalog.has_table_privilege('espacigo_runtime','public.cotizacion_arriendo_comercial','DELETE WITH GRANT OPTION')`
	if err := q.QueryRow(ctx, effective).Scan(&selectInsertDelete, &update, &truncate, &references, &grantOption); err != nil {
		return errors.New("read V97 runtime table privileges")
	}
	if !selectInsertDelete || update || truncate || references || grantOption {
		return errors.New("V97 runtime table privileges are not least-privilege")
	}
	var count int
	var ownerMatches, privilegeSet, onlyOwnerAndRuntime, noRuntimeColumnGrants bool
	const direct = `WITH table_acl AS (
 SELECT 'table'::text AS scope,c.relowner,a.grantee,a.grantor,a.privilege_type,a.is_grantable
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(c.relacl)>0 THEN c.relacl ELSE NULL::aclitem[] END) a
 WHERE n.nspname='public' AND c.relname='cotizacion_arriendo_comercial'
 UNION ALL
 SELECT 'column',t.relowner,a.grantee,a.grantor,a.privilege_type,a.is_grantable
 FROM pg_catalog.pg_attribute c JOIN pg_catalog.pg_class t ON t.oid=c.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(c.attacl)>0 THEN c.attacl ELSE NULL::aclitem[] END) a
 WHERE n.nspname='public' AND t.relname='cotizacion_arriendo_comercial' AND c.attnum>0 AND NOT c.attisdropped
)
SELECT count(*) FILTER (WHERE scope='table' AND grantee='espacigo_runtime'::regrole),
 COALESCE(bool_and(grantor='espacigo_schema_owner'::regrole AND NOT is_grantable) FILTER (WHERE scope='table' AND grantee='espacigo_runtime'::regrole),false),
 count(*) FILTER (WHERE scope='table' AND grantee='espacigo_runtime'::regrole AND privilege_type IN ('SELECT','INSERT','DELETE'))=3,
 COALESCE(bool_and(grantee IN (relowner,'espacigo_runtime'::regrole)),true),
 count(*) FILTER (WHERE scope='column' AND grantee='espacigo_runtime'::regrole)=0
FROM table_acl`
	if err := q.QueryRow(ctx, direct).Scan(&count, &ownerMatches, &privilegeSet, &onlyOwnerAndRuntime, &noRuntimeColumnGrants); err != nil {
		return fmt.Errorf("read exact V97 runtime direct grants: %w", err)
	}
	if count != 3 || !ownerMatches || !privilegeSet || !onlyOwnerAndRuntime || !noRuntimeColumnGrants {
		return errors.New("V97 table/column ACL differs from owner and runtime SELECT/INSERT/DELETE only")
	}
	return nil
}

func revokeV97Authority(ctx context.Context, conn *pgx.Conn) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("begin V97 privilege revocation")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, statement := range []string{v97RevokeCreateSQL, v97RevokeUsuarioReferencesSQL, v97RevokeEspacioReferencesSQL} {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return errors.New("revoke exact V97 temporary privileges")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("commit V97 temporary privilege revocation")
	}
	return nil
}

func verifyV97FinalizedState(ctx context.Context, conn *pgx.Conn, migrations []Migration, expectedBaseline string, report v97AuthorityReport) (v97AuthorityReport, error) {
	post, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return report, errors.New("begin V97 finalization readback")
	}
	defer func() { _ = post.Rollback(context.Background()) }()
	if err := inspectV97Target(ctx, post, &report); err != nil {
		return report, err
	}
	if err := inspectV97History(ctx, post, migrations, &report); err != nil {
		return report, err
	}
	currentBaseline, closed, err := v97AuthorityBaseline(ctx, post)
	if err != nil {
		return report, err
	}
	report.Mode, report.BaselineSHA256, report.AuthorityClosed = "finalize", currentBaseline, closed
	if currentBaseline != expectedBaseline || !closed || (report.AppliedCount != 96 && report.AppliedCount != 97) {
		return report, errors.New("V97 finalization found baseline, authority, or ledger drift; temporary grants remain revoked")
	}
	return report, nil
}

// This test-only manager cannot operate on the persistent native/cloud target.
// The exact database is captured once; all SQL identity readbacks must match it.
func validateV97FixtureTarget(conn *pgx.Conn) (string, error) {
	if conn == nil || os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		return "", errors.New("V97 authority requires the isolated fixture")
	}
	cfg := conn.Config()
	selected, err := pgx.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || cfg == nil || !filepath.IsAbs(cfg.Host) || cfg.Host != selected.Host || cfg.Port != 5432 || cfg.Port != selected.Port || cfg.User != v97AuthorityUser || selected.User != v97AuthorityUser || !strings.HasPrefix(cfg.Database, v97AuthorityDatabasePrefix) {
		return "", errors.New("V97 authority target differs from selected marked fixture")
	}
	marker, err := os.Lstat(filepath.Join(cfg.Host, rentalCombinedFixtureMarker))
	if err != nil || !marker.Mode().IsRegular() || marker.Mode().Perm() != 0600 {
		return "", errors.New("V97 authority fixture marker missing or unsafe")
	}
	return cfg.Database, nil
}
