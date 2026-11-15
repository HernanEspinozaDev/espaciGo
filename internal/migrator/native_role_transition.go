package migrator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const nativeV80RoleTransitionTarget = "espacigo_dms_stage_20261020"

// ApplyNativeV80RoleTransition provisions the minimum role graph required by
// V81 while preserving the existing runtime role OID and object grants. The
// caller pins the exact target; expectedDatabaseOID=0 is reserved for isolated
// integration tests, while the operational command supplies the reviewed OID.
func ApplyNativeV80RoleTransition(ctx context.Context, databaseURL, migrationDir, expectedDatabase string, expectedDatabaseOID uint32) (returnErr error) {
	if expectedDatabase == "" {
		return errors.New("native role transition requires an explicit target database")
	}
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return fmt.Errorf("discover native role-transition source: %w", err)
	}
	if len(migrations) < 85 {
		return errors.New("native role transition requires the reviewed V1..V85 source inventory")
	}
	for i, migration := range migrations[:85] {
		if migration.Version != int64(i+1) {
			return errors.New("native role-transition source inventory is not contiguous")
		}
	}

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to native role-transition target: %w", err)
	}
	defer conn.Close(context.Background())
	release, err := AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := release(context.Background()); returnErr == nil && unlockErr != nil {
			returnErr = unlockErr
		}
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin native role transition: %w", err)
	}
	defer tx.Rollback(context.Background())

	var database, serverVersion, sessionUser, currentUser, databaseOwner string
	var databaseOID int64
	var superuser bool
	if err := tx.QueryRow(ctx, `
		SELECT current_database(), current_setting('server_version'), session_user, current_user,
		       current_user_role.rolsuper, db.oid::bigint, pg_catalog.pg_get_userbyid(db.datdba)
		FROM pg_catalog.pg_database db
		JOIN pg_catalog.pg_roles current_user_role ON current_user_role.rolname=current_user
		WHERE db.datname=current_database()`).Scan(&database, &serverVersion, &sessionUser, &currentUser, &superuser, &databaseOID, &databaseOwner); err != nil {
		return fmt.Errorf("read native role-transition identity: %w", err)
	}
	if database != expectedDatabase || sessionUser != "postgres" || currentUser != "postgres" || !superuser || databaseOwner != "postgres" || !strings.HasPrefix(strings.TrimSpace(serverVersion), "18.6") {
		return errors.New("native role transition requires the exact PostgreSQL 18.6 database, postgres owner, and peer administrator identity")
	}
	if expectedDatabaseOID != 0 && uint32(databaseOID) != expectedDatabaseOID {
		return errors.New("native role transition database OID differs from the reviewed target")
	}

	history, err := readHistory(ctx, tx)
	if err != nil {
		return fmt.Errorf("read native migration ledger under lock: %w", err)
	}
	if len(history) != 80 {
		return fmt.Errorf("native role transition requires the exact V1..V80 ledger prefix; found %d rows", len(history))
	}
	for i := range history {
		if history[i].Version != int64(i+1) || history[i].Version != migrations[i].Version || history[i].Name != migrations[i].Name || history[i].Checksum != migrations[i].Checksum {
			return fmt.Errorf("native role transition ledger/source mismatch at V%06d", i+1)
		}
	}

	var runtimeOID uint32
	var runtimeLogin, runtimeInherit, runtimeSuper, runtimeCreateRole, runtimeCreateDB, runtimeReplication, runtimeBypassRLS bool
	if err := tx.QueryRow(ctx, `SELECT oid, rolcanlogin, rolinherit, rolsuper, rolcreaterole, rolcreatedb, rolreplication, rolbypassrls
		FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`).Scan(&runtimeOID, &runtimeLogin, &runtimeInherit, &runtimeSuper, &runtimeCreateRole, &runtimeCreateDB, &runtimeReplication, &runtimeBypassRLS); err != nil {
		return fmt.Errorf("read existing runtime role: %w", err)
	}
	if !runtimeLogin || runtimeInherit || runtimeSuper || runtimeCreateRole || runtimeCreateDB || runtimeReplication || runtimeBypassRLS {
		return errors.New("native role transition runtime attributes differ from the reviewed LOGIN/NOINHERIT predecessor")
	}
	var runtimeMemberships, runtimeSessions, existingNewRoles int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_auth_members WHERE roleid=$1 OR member=$1`, runtimeOID).Scan(&runtimeMemberships); err != nil {
		return fmt.Errorf("read runtime memberships: %w", err)
	}
	if runtimeMemberships != 0 {
		return errors.New("native role transition refuses unreviewed runtime memberships")
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE datname=current_database() AND usesysid=$1 AND pid<>pg_backend_pid()`, runtimeOID).Scan(&runtimeSessions); err != nil {
		return fmt.Errorf("read runtime sessions: %w", err)
	}
	if runtimeSessions != 0 {
		return errors.New("native role transition refuses to disable runtime while any runtime session is active")
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname=ANY($1::text[])`, []string{"espacigo_api_login", "espacigo_migrator_login", "espacigo_schema_owner", "espacigo_document_verification_fn"}).Scan(&existingNewRoles); err != nil {
		return fmt.Errorf("check native role names: %w", err)
	}
	if existingNewRoles != 0 {
		return errors.New("native role transition refuses pre-existing API, migrator, schema-owner, or helper roles")
	}
	var targetObjectsReady, privateSchemaAbsent bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('public.usuario') IS NOT NULL
		AND to_regclass('public.espacio') IS NOT NULL
		AND to_regclass('public.schema_migrations') IS NOT NULL,
		to_regnamespace('document_verification_private') IS NULL`).Scan(&targetObjectsReady, &privateSchemaAbsent); err != nil {
		return fmt.Errorf("read V81 role-transition object preconditions: %w", err)
	}
	if !targetObjectsReady || !privateSchemaAbsent {
		return errors.New("native role transition requires the V80 tables and an unused private schema name")
	}
	var nonPostgresOwners int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM (
			SELECT c.relname, pg_catalog.pg_get_userbyid(c.relowner) AS owner
			FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
			WHERE n.nspname='public' AND c.relname IN ('usuario','espacio','schema_migrations')
		) objects WHERE owner<>'postgres'`).Scan(&nonPostgresOwners); err != nil {
		return fmt.Errorf("read target object owners: %w", err)
	}
	if nonPostgresOwners != 0 {
		return errors.New("native role transition requires postgres ownership of the V80 role-contract tables")
	}

	for _, statement := range []string{
		`CREATE ROLE espacigo_api_login LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
		`CREATE ROLE espacigo_migrator_login LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
		`CREATE ROLE espacigo_schema_owner NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
		`CREATE ROLE espacigo_document_verification_fn NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
		`CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn`,
		`GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn, espacigo_schema_owner`,
		`GRANT SELECT(id,estado), UPDATE(id) ON public.usuario TO espacigo_document_verification_fn`,
		`GRANT SELECT(id,propietario_id), UPDATE(id) ON public.espacio TO espacigo_document_verification_fn`,
		`GRANT SELECT, INSERT ON public.schema_migrations TO espacigo_schema_owner`,
		`GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
		`GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
		`GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
		"GRANT CONNECT ON DATABASE " + pgx.Identifier{expectedDatabase}.Sanitize() + " TO espacigo_api_login, espacigo_migrator_login",
		`ALTER ROLE espacigo_runtime NOLOGIN`,
	} {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("apply guarded native role transition: %w", err)
		}
	}

	var afterOID uint32
	var afterLogin, apiLogin, migratorLogin, schemaOwnerLogin, helperLogin bool
	if err := tx.QueryRow(ctx, `
		SELECT runtime.oid, runtime.rolcanlogin, api.rolcanlogin, migrator.rolcanlogin, schema_owner.rolcanlogin, helper.rolcanlogin
		FROM pg_catalog.pg_roles runtime
		JOIN pg_catalog.pg_roles api ON api.rolname='espacigo_api_login'
		JOIN pg_catalog.pg_roles migrator ON migrator.rolname='espacigo_migrator_login'
		JOIN pg_catalog.pg_roles schema_owner ON schema_owner.rolname='espacigo_schema_owner'
		JOIN pg_catalog.pg_roles helper ON helper.rolname='espacigo_document_verification_fn'
		WHERE runtime.rolname='espacigo_runtime'`).Scan(&afterOID, &afterLogin, &apiLogin, &migratorLogin, &schemaOwnerLogin, &helperLogin); err != nil {
		return fmt.Errorf("read back native role transition: %w", err)
	}
	if afterOID != runtimeOID || afterLogin || !apiLogin || !migratorLogin || schemaOwnerLogin || helperLogin {
		return errors.New("native role transition postcondition failed; transaction rolled back")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit native role transition; read back before retry: %w", err)
	}
	return nil
}
