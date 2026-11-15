package migrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

const (
	v84AuthorityInstance   = "espacigo-testprod:southamerica-west1:espacigo-testprod-db"
	v84AuthorityDatabase   = "espacigo_dms_stage_20261020"
	v84AuthorityUser       = "postgres"
	v84AuthorityRole       = "espacigo_schema_owner"
	v84AuthorityMigration  = "V000084__didit_identity_verification_schema.sql"
	v84AuthorityChecksum   = "d8cc68f3ea73c6e351a31bd8f88fd0606e8217c028777d5ebca7b406bb852b8f"
	v84AuthorityInventory  = "3921ff747e4a7e7f49de443de7333dad03535f0687078b93b8a3cac6042a209a"
	v84GrantCreateSQL      = `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`
	v84GrantReferencesSQL  = `GRANT REFERENCES (id) ON TABLE public.usuario TO espacigo_schema_owner`
	v84RevokeCreateSQL     = `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`
	v84RevokeReferencesSQL = `REVOKE REFERENCES (id) ON TABLE public.usuario FROM espacigo_schema_owner`
)

// V84AuthorityReport describes a narrowly scoped, temporary V84 privilege recovery.
type V84AuthorityReport struct {
	Format          string `json:"format"`
	Mode            string `json:"mode"`
	Database        string `json:"database"`
	SessionUser     string `json:"session_user"`
	CurrentUser     string `json:"current_user"`
	AppliedCount    int    `json:"applied_count"`
	BaselineSHA256  string `json:"baseline_sha256"`
	AuthorityClosed bool   `json:"authority_closed"`
}

// ManageV84AuthorityCloudSQL manages only the two temporary grants required by V84.
// It never assumes the schema-owner role: the dedicated migration Job must do that.
func ManageV84AuthorityCloudSQL(ctx context.Context, databaseURL, password, migrationDir, instance string, privateIP bool, mode, expectedBaseline string) (V84AuthorityReport, error) {
	var report V84AuthorityReport
	if mode != "inspect" && mode != "prepare" && mode != "finalize" {
		return report, errors.New("invalid V84 authority mode")
	}
	if instance != v84AuthorityInstance {
		return report, errors.New("V84 authority instance does not match fixed target")
	}
	if !privateIP {
		return report, errors.New("V84 authority requires the private Cloud SQL IP")
	}
	if mode != "finalize" && expectedBaseline != "" {
		return report, errors.New("baseline SHA-256 is valid only in finalize mode")
	}
	u, err := url.Parse(databaseURL)
	if err != nil || u.User == nil || u.User.Username() != v84AuthorityUser || strings.TrimPrefix(u.Path, "/") != v84AuthorityDatabase {
		return report, errors.New("V84 authority requires the fixed database and postgres login")
	}
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return report, err
	}
	if len(migrations) != 84 || inventoryFingerprint(migrations) != v84AuthorityInventory {
		return report, errors.New("V84 authority migration inventory fingerprint mismatch")
	}
	m := migrations[83]
	if m.Version != 84 || m.Name != v84AuthorityMigration || m.Checksum != v84AuthorityChecksum {
		return report, errors.New("V84 migration name or checksum mismatch")
	}
	if mode == "finalize" && !validHex(expectedBaseline, 64) {
		return report, errors.New("finalize requires a lowercase SHA-256 baseline")
	}
	config, err := cloudsql.NewPGXConfig(databaseURL, password)
	if err != nil {
		return report, err
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return report, err
	}
	defer closeConnector()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return report, errors.New("connect V84 authority database")
	}
	defer conn.Close(context.Background())
	return manageV84AuthorityWithConn(ctx, conn, migrationDir, mode, expectedBaseline)
}

func manageV84AuthorityWithConn(ctx context.Context, conn *pgx.Conn, migrationDir, mode, expectedBaseline string) (V84AuthorityReport, error) {
	var r V84AuthorityReport
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return r, err
	}
	if len(migrations) != 84 || inventoryFingerprint(migrations) != v84AuthorityInventory || migrations[83].Name != v84AuthorityMigration || migrations[83].Checksum != v84AuthorityChecksum {
		return r, errors.New("V84 migration inventory or exact migration mismatch")
	}
	if mode != "inspect" && mode != "prepare" && mode != "finalize" {
		return r, errors.New("invalid V84 authority mode")
	}
	if mode == "finalize" && !validHex(expectedBaseline, 64) {
		return r, errors.New("finalize requires a lowercase SHA-256 baseline")
	}
	if mode != "finalize" && expectedBaseline != "" {
		return r, errors.New("baseline SHA-256 is valid only in finalize mode")
	}
	unlock, err := AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return r, err
	}
	defer func() { _ = unlock(context.Background()) }()
	access := pgx.ReadOnly
	if mode == "prepare" {
		access = pgx.ReadWrite
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: access})
	if err != nil {
		return r, errors.New("begin V84 authority inspection")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := inspectV84Target(ctx, tx, &r); err != nil {
		return r, err
	}
	if mode != "finalize" {
		if err := validateV84RoleAuthority(ctx, tx); err != nil {
			return r, err
		}
	}
	if mode != "finalize" {
		if err := inspectV84History(ctx, tx, migrations, &r); err != nil {
			return r, err
		}
	}
	r.Mode = mode
	var baseline string
	var missing bool
	if mode != "finalize" {
		baseline, missing, err = v84Baseline(ctx, tx)
		if err != nil {
			return r, err
		}
		r.BaselineSHA256 = baseline
	}
	if mode == "inspect" {
		r.AuthorityClosed = missing
		return r, nil
	}
	if mode == "prepare" {
		tablesExist, err := v84TablesExist(ctx, tx)
		if err != nil {
			return r, errors.New("inspect V84 table state")
		}
		if r.AppliedCount != 83 || tablesExist || !missing {
			return r, errors.New("prepare requires exact V83 ledger, absent V84 tables, and both privileges missing")
		}
		// Recheck all grant preconditions in the same write transaction immediately
		// before granting, so no read/write gap exists.
		if _, err := tx.Exec(ctx, v84GrantCreateSQL); err != nil {
			_ = tx.Rollback(context.Background())
			return r, errors.New("grant V84 schema privilege")
		}
		if _, err := tx.Exec(ctx, v84GrantReferencesSQL); err != nil {
			_ = tx.Rollback(context.Background())
			return r, errors.New("grant V84 column privilege")
		}
		if err := verifyV84EffectiveGrants(ctx, tx); err != nil {
			_ = tx.Rollback(context.Background())
			return r, err
		}
		if err := tx.Commit(ctx); err != nil {
			return r, errors.New("commit V84 temporary grants")
		}
		// A precommit probe is not proof that the grant committed. Observe the
		// exact target again in a separate read-only transaction before reporting.
		readback, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return r, errors.New("begin committed V84 grant readback; finalize required")
		}
		defer func() { _ = readback.Rollback(context.Background()) }()
		if err := verifyV84EffectiveGrants(ctx, readback); err != nil {
			return r, err
		}
		r.AuthorityClosed = false
		return r, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return r, errors.New("finish V84 finalize preflight")
	}
	// Compare the saved baseline only after unconditional revocation.
	// Revoke independently of ledger and postcheck: cleanup is unconditional once
	// target and identity are established, including after partial migration failure.
	revokeTx, err := conn.Begin(ctx)
	if err != nil {
		return r, errors.New("begin V84 privilege revocation")
	}
	_, err = revokeTx.Exec(ctx, v84RevokeCreateSQL)
	if err == nil {
		_, err = revokeTx.Exec(ctx, v84RevokeReferencesSQL)
	}
	if err != nil {
		_ = revokeTx.Rollback(context.Background())
		return r, errors.New("revoke exact V84 temporary privileges")
	}
	if err := revokeTx.Commit(ctx); err != nil {
		return r, errors.New("commit V84 privilege revocation")
	}
	post, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return r, errors.New("begin V84 finalize verification")
	}
	defer func() { _ = post.Rollback(context.Background()) }()
	var after V84AuthorityReport
	if err := inspectV84Target(ctx, post, &after); err != nil {
		return r, err
	}
	if err := inspectV84History(ctx, post, migrations, &after); err != nil {
		return r, err
	}
	afterBaseline, stillMissing, err := v84Baseline(ctx, post)
	if err != nil {
		return r, err
	}
	r.AppliedCount = after.AppliedCount
	r.BaselineSHA256 = afterBaseline
	r.AuthorityClosed = stillMissing
	if afterBaseline != expectedBaseline || !stillMissing || (after.AppliedCount != 83 && after.AppliedCount != 84) {
		return r, errors.New("V84 finalize verification found baseline, authority, or ledger drift; temporary grants remain revoked")
	}
	return r, nil
}

func inspectV84Target(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, r *V84AuthorityReport) error {
	r.Format, r.Mode = "espacigo.v84-authority.v1", ""
	if err := q.QueryRow(ctx, `SELECT current_database(),session_user,current_user`).Scan(&r.Database, &r.SessionUser, &r.CurrentUser); err != nil {
		return errors.New("read V84 authority target identity")
	}
	if r.Database != v84AuthorityDatabase || r.SessionUser != v84AuthorityUser || r.CurrentUser != v84AuthorityUser {
		return errors.New("V84 authority target identity mismatch")
	}
	return nil
}

type v84Queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func inspectV84History(ctx context.Context, q v84Queryer, migrations []Migration, r *V84AuthorityReport) error {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind IN ('r','p'))`).Scan(&exists); err != nil || !exists {
		return errors.New("V84 authority requires existing migration ledger")
	}
	h, err := readHistory(ctx, q)
	if err != nil {
		return err
	}
	if err := verifyHistory(migrations, h); err != nil {
		return err
	}
	if len(h) != 83 && len(h) != 84 {
		return errors.New("V84 authority requires ledger prefix 83 or 84")
	}
	r.AppliedCount = len(h)
	tablesExist, err := v84TablesExist(ctx, q)
	if err != nil {
		return errors.New("inspect V84 relation state")
	}
	if (len(h) == 83 && tablesExist) || (len(h) == 84 && !tablesExist) {
		return errors.New("V84 ledger and schema objects do not form a valid 83/84 state")
	}
	if len(h) == 84 {
		var owned bool
		if err := q.QueryRow(ctx, `SELECT count(*)=2 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('identity_verification_attempts','didit_webhook_inbox') AND pg_catalog.pg_get_userbyid(c.relowner)='espacigo_schema_owner'`).Scan(&owned); err != nil || !owned {
			return errors.New("V84 relations are not both owned by schema owner")
		}
	}
	return nil
}

func v84TablesExist(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) (bool, error) {
	var first, second bool
	err := q.QueryRow(ctx, `SELECT to_regclass('public.identity_verification_attempts') IS NOT NULL, to_regclass('public.didit_webhook_inbox') IS NOT NULL`).Scan(&first, &second)
	if err == nil && first != second {
		return false, errors.New("partial V84 table state")
	}
	return first && second, err
}

// validateV84RoleAuthority checks the observed least-privilege role graph and
// ownership contract; a matching hash alone is not an authorization guard.
func validateV84RoleAuthority(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	const query = `WITH expected(name,super,createrole,createdb,inherit,login,bypassrls,replication) AS (VALUES
 ('espacigo_schema_owner',false,false,false,false,false,false,false),
 ('espacigo_runtime',false,false,false,false,false,false,false),
 ('espacigo_document_verification_fn',false,false,false,false,false,false,false),
 ('espacigo_migrator_login',false,false,false,false,true,false,false)),
 actual AS (SELECT r.rolname,r.rolsuper,r.rolcreaterole,r.rolcreatedb,r.rolinherit,r.rolcanlogin,r.rolbypassrls,r.rolreplication FROM pg_catalog.pg_roles r)
 SELECT (SELECT count(*)=4 FROM expected e JOIN actual a ON a.rolname=e.name AND (a.rolsuper,a.rolcreaterole,a.rolcreatedb,a.rolinherit,a.rolcanlogin,a.rolbypassrls,a.rolreplication)=(e.super,e.createrole,e.createdb,e.inherit,e.login,e.bypassrls,e.replication)),
 (SELECT count(*)=3 FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles g ON g.oid=am.roleid JOIN pg_catalog.pg_roles m ON m.oid=am.member WHERE (g.rolname,m.rolname,am.admin_option,am.inherit_option,am.set_option) IN (('espacigo_document_verification_fn','espacigo_schema_owner',false,false,true),('espacigo_runtime','espacigo_api_login',false,false,true),('espacigo_schema_owner','espacigo_migrator_login',false,false,true)) AND (g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn') OR m.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn'))),
 (SELECT count(*)=0 FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles g ON g.oid=am.roleid JOIN pg_catalog.pg_roles m ON m.oid=am.member WHERE (g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn') OR m.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn')) AND NOT ((g.rolname,m.rolname,am.admin_option,am.inherit_option,am.set_option) IN (('espacigo_document_verification_fn','espacigo_schema_owner',false,false,true),('espacigo_runtime','espacigo_api_login',false,false,true),('espacigo_schema_owner','espacigo_migrator_login',false,false,true)))),
 (SELECT pg_catalog.pg_get_userbyid(n.nspowner)='pg_database_owner' FROM pg_catalog.pg_namespace n WHERE n.nspname='public'),
 (SELECT pg_catalog.pg_get_userbyid(c.relowner)='postgres' FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='usuario'),
 (SELECT pg_catalog.pg_get_userbyid(c.relowner)='postgres' FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations')`
	var flags, edges, noExtra, schemaOwner, tableOwner, ledgerOwner bool
	if err := q.QueryRow(ctx, query).Scan(&flags, &edges, &noExtra, &schemaOwner, &tableOwner, &ledgerOwner); err != nil {
		return errors.New("read V84 role authority guard")
	}
	if !flags || !edges || !noExtra || !schemaOwner || !tableOwner || !ledgerOwner {
		return errors.New("V84 role flags, exact memberships, or owners do not match authority contract")
	}
	return nil
}

func verifyV84EffectiveGrants(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	var schemaCreate, columnRefs, grantOption, tableRefs bool
	if err := q.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE'), has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES'), has_schema_privilege('espacigo_schema_owner','public','CREATE WITH GRANT OPTION') OR has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES WITH GRANT OPTION'), has_table_privilege('espacigo_schema_owner','public.usuario','REFERENCES')`).Scan(&schemaCreate, &columnRefs, &grantOption, &tableRefs); err != nil {
		return errors.New("verify V84 effective grants")
	}
	if !schemaCreate || !columnRefs || grantOption || tableRefs {
		return errors.New("V84 grant verification found missing or excessive privilege")
	}
	var directCreate, directReferences bool
	const direct = `SELECT
 EXISTS(SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(n.nspacl)>0 THEN n.nspacl ELSE NULL::aclitem[] END) a
 WHERE n.nspname='public' AND a.grantee='espacigo_schema_owner'::regrole AND a.privilege_type='CREATE' AND NOT a.is_grantable
 AND a.grantor IN (n.nspowner, 'postgres'::regrole)),
 EXISTS(SELECT 1 FROM pg_catalog.pg_attribute c JOIN pg_catalog.pg_class t ON t.oid=c.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(c.attacl)>0 THEN c.attacl ELSE NULL::aclitem[] END) a
 WHERE n.nspname='public' AND t.relname='usuario' AND c.attname='id' AND c.attnum>0 AND NOT c.attisdropped
 AND a.grantee='espacigo_schema_owner'::regrole AND a.grantor='postgres'::regrole AND a.privilege_type='REFERENCES' AND NOT a.is_grantable)`
	if err := q.QueryRow(ctx, direct).Scan(&directCreate, &directReferences); err != nil {
		return errors.New("read exact direct V84 privilege tuples")
	}
	if !directCreate || !directReferences {
		return errors.New("V84 temporary grants lack the exact grantee, grantor, privilege, or grant-option tuple")
	}
	return nil
}

// v84Baseline covers both GRANT/REVOKE targets, all their grantees, protected
// role flags/memberships and original owners. aclitem text preserves grantee,
// privilege bitmap (including grant options) and grantor; sorting only removes
// array ordering differences. New V84 tables are deliberately not a baseline.
func v84Baseline(ctx context.Context, q v84Queryer) (string, bool, error) {
	const query = `
WITH tuples(v) AS (
 SELECT 'schema_acl|'||COALESCE(x::text,'') FROM pg_catalog.pg_namespace n LEFT JOIN LATERAL unnest(n.nspacl) x ON true WHERE n.nspname='public'
 UNION ALL SELECT 'usuario_acl|'||COALESCE(x::text,'') FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace LEFT JOIN LATERAL unnest(c.relacl) x ON true WHERE n.nspname='public' AND c.relname='usuario'
 UNION ALL SELECT 'usuario_column_acl|'||a.attname||'|'||COALESCE(x::text,'') FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace LEFT JOIN LATERAL unnest(a.attacl) x ON true WHERE n.nspname='public' AND c.relname='usuario' AND a.attnum>0 AND NOT a.attisdropped
 UNION ALL SELECT 'role|'||rolname||'|'||rolsuper||'|'||rolcreaterole||'|'||rolcreatedb||'|'||rolinherit||'|'||rolcanlogin||'|'||rolbypassrls||'|'||rolreplication FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn','espacigo_migrator_login','espacigo_api_login')
 UNION ALL SELECT 'member|'||m.rolname||'|'||u.rolname||'|'||am.admin_option||'|'||am.inherit_option||'|'||am.set_option FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles m ON m.oid=am.roleid JOIN pg_catalog.pg_roles u ON u.oid=am.member WHERE m.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn') OR u.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_document_verification_fn')
 UNION ALL SELECT 'owner|'||n.nspname||'|'||c.relname||'|'||pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname IN ('usuario','schema_migrations')
 UNION ALL SELECT 'schema_owner|'||n.nspname||'|'||pg_catalog.pg_get_userbyid(n.nspowner) FROM pg_catalog.pg_namespace n WHERE n.nspname='public'
)
SELECT v, pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE'), pg_catalog.has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES') FROM tuples ORDER BY v`
	rows, err := q.Query(ctx, query)
	if err != nil {
		return "", false, errors.New("read V84 authority baseline")
	}
	defer rows.Close()
	var tuples []string
	createPresent, refsPresent := true, true
	for rows.Next() {
		var v string
		var create, refs bool
		if err := rows.Scan(&v, &create, &refs); err != nil {
			return "", false, err
		}
		tuples = append(tuples, v)
		createPresent = createPresent && create
		refsPresent = refsPresent && refs
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	sort.Strings(tuples)
	sum := sha256.Sum256([]byte(strings.Join(tuples, "\n") + "\n"))
	return hex.EncodeToString(sum[:]), !createPresent && !refsPresent, nil
}
