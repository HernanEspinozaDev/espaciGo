package gcp03origin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

var bootstrapStatements = []string{
	`CREATE EXTENSION postgis`,
	`CREATE EXTENSION btree_gist`,
	`CREATE EXTENSION fuzzystrmatch`,
	`CREATE EXTENSION postgis_tiger_geocoder`,
	`CREATE EXTENSION postgis_topology`,
	`CREATE EXTENSION unaccent`,
	`CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
	`CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
	`CREATE ROLE espacigo_gcp03_observer NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
	`GRANT espacigo_runtime TO postgres WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
	`REVOKE TEMPORARY ON DATABASE espacigo_scratch FROM PUBLIC`,
	`GRANT CONNECT ON DATABASE espacigo_scratch TO espacigo_runtime, espacigo_gcp03_observer`,
	`GRANT USAGE ON SCHEMA public TO espacigo_gcp03_observer`,
}

const bootstrapIdentitySQL = `SELECT current_database(),current_user`

const emptyBaselineSQL = `SELECT
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f'))::bigint,
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema')::bigint,
  (SELECT count(*) FROM pg_catalog.pg_extension)::bigint,
  EXISTS (SELECT 1 FROM pg_catalog.pg_extension WHERE extname='plpgsql' AND extversion='1.0'),
  (SELECT count(*) FROM pg_catalog.pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname NOT IN ('public','information_schema'))::bigint`

const schemaInventorySQL = `WITH schema_inventory AS (
  SELECT n.oid, n.nspname,
    EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_namespace'::regclass AND d.objid=n.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass) AS directly_extension_owned,
    (EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_depend d ON d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass WHERE c.relnamespace=n.oid AND c.relkind IN ('r','p','v','m','S','f'))
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_depend d ON d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass WHERE p.pronamespace=n.oid)) AS has_extension_objects,
    (EXISTS (SELECT 1 FROM pg_catalog.pg_class c WHERE c.relnamespace=n.oid AND c.relkind IN ('r','p','v','m','S','f') AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.pronamespace=n.oid AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))) AS has_application_objects
  FROM pg_catalog.pg_namespace n
  WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname NOT IN ('public','information_schema')
)
SELECT
  count(*) FILTER (WHERE NOT directly_extension_owned AND NOT (has_extension_objects AND NOT has_application_objects)),
  count(*) FILTER (WHERE directly_extension_owned OR (has_extension_objects AND NOT has_application_objects))
FROM schema_inventory`

type BootstrapReport struct {
	Database             string      `json:"database"`
	Login                string      `json:"login"`
	TransactionOutcome   string      `json:"transaction_outcome"`
	Extensions           []Extension `json:"extensions"`
	RolesPrepared        []string    `json:"roles_prepared"`
	RolesReady           bool        `json:"roles_ready"`
	ObserverConnect      bool        `json:"observer_connect"`
	ObserverPublicUsage  bool        `json:"observer_public_usage"`
	ObserverNoWrite      bool        `json:"observer_no_write_privileges"`
	ApplicationSchemas   int64       `json:"application_schemas"`
	ApplicationRelations int64       `json:"application_relations"`
	ApplicationFunctions int64       `json:"application_functions"`
	Passed               bool        `json:"passed"`
}

type Extension struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ObserveReport struct {
	Database                string      `json:"database"`
	Login                   string      `json:"login"`
	TransactionOutcome      string      `json:"transaction_outcome"`
	ServerVersionNum        int         `json:"server_version_num"`
	Extensions              []Extension `json:"extensions"`
	RolesReady              bool        `json:"roles_ready"`
	ObserverMembership      bool        `json:"observer_membership"`
	ObserverPermissions     bool        `json:"observer_permissions"`
	ExtensionOwnedSchemas   int64       `json:"extension_owned_schemas"`
	ApplicationSchemas      int64       `json:"application_schemas"`
	Relations               int64       `json:"relations"`
	ExtensionOwnedRelations int64       `json:"extension_owned_relations"`
	ApplicationRelations    int64       `json:"application_relations"`
	ExtensionOwnedFunctions int64       `json:"extension_owned_functions"`
	ApplicationFunctions    int64       `json:"application_functions"`
	Passed                  bool        `json:"passed"`
}

// MembershipReport records the single, narrowly scoped runtime membership
// repair. The phase intentionally performs no role, schema, or data DDL.
type MembershipReport struct {
	Database           string `json:"database"`
	Login              string `json:"login"`
	Grantor            string `json:"grantor"`
	TransactionOutcome string `json:"transaction_outcome"`
	RuntimeRoleReady   bool   `json:"runtime_role_ready"`
	MembershipSet      bool   `json:"membership_set"`
	AdminOption        bool   `json:"admin_option"`
	InheritOption      bool   `json:"inherit_option"`
	SetOption          bool   `json:"set_option"`
	Passed             bool   `json:"passed"`
}

const runtimeMembershipGrant = `GRANT espacigo_runtime TO postgres WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`

const runtimeMembershipPostcheckSQL = `SELECT r.rolname='espacigo_runtime' AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls AND NOT r.rolinherit,
 m.admin_option, m.inherit_option, m.set_option
 FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_auth_members m ON m.roleid=r.oid AND m.member=current_user::regrole
 WHERE r.rolname='espacigo_runtime' AND m.grantor=current_user::regrole`

func RunMembership(ctx context.Context, config Config, password string) (MembershipReport, error) {
	if config.Phase != PhaseMembership || config.Login != AdminLogin || config.DatabaseURL != AdminDatabaseURL || password == "" || !config.PrivateIP {
		return MembershipReport{}, errors.New("GCP-03 origin membership configuration is invalid")
	}
	conn, closeConnector, err := cloudsql.Connect(ctx, config.DatabaseURL, password, config.Instance, true)
	if err != nil {
		return MembershipReport{}, err
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	return membershipOnConnection(ctx, conn, func(ctx context.Context, tx pgx.Tx) error { return tx.Commit(ctx) })
}

func membershipOnConnection(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (MembershipReport, error) {
	return membershipOnConnectionWithPostcheck(ctx, conn, runtimeMembershipPostcheckSQL, commit)
}

func membershipOnConnectionWithPostcheck(ctx context.Context, conn *pgx.Conn, postcheckSQL string, commit func(context.Context, pgx.Tx) error) (MembershipReport, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return MembershipReport{TransactionOutcome: "not_started"}, errors.New("GCP-03 origin membership could not begin its transaction")
	}
	report := MembershipReport{}
	if err := tx.QueryRow(ctx, `SELECT current_database(), current_user`).Scan(&report.Database, &report.Login); err != nil || report.Database != DatabaseName || report.Login != AdminLogin {
		return rollbackMembership(tx, report, "GCP-03 origin membership connected to an unexpected identity")
	}
	report.Grantor = report.Login
	var actorSuper, actorExpected bool
	if err := tx.QueryRow(ctx, `SELECT r.rolsuper, r.rolcanlogin AND r.rolcreaterole FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&actorSuper, &actorExpected); err != nil || actorSuper || !actorExpected {
		return rollbackMembership(tx, report, "GCP-03 origin membership requires the fixed non-superuser postgres login")
	}
	var runtimeOK bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit) FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`).Scan(&runtimeOK); err != nil || !runtimeOK {
		return rollbackMembership(tx, report, "GCP-03 origin membership requires the existing restricted runtime role")
	}
	// Exactly one mutating SQL statement is permitted in this transaction.
	if _, err := tx.Exec(ctx, runtimeMembershipGrant); err != nil {
		return rollbackMembership(tx, report, "GCP-03 origin runtime membership grant failed")
	}
	var admin, inherit, set bool
	if postcheckSQL == "" {
		return rollbackMembership(tx, report, "GCP-03 origin runtime membership postcondition is unavailable")
	}
	if err := tx.QueryRow(ctx, postcheckSQL).Scan(&report.RuntimeRoleReady, &admin, &inherit, &set); err != nil {
		return rollbackMembership(tx, report, "GCP-03 origin runtime membership postcondition could not be read")
	}
	report.AdminOption, report.InheritOption, report.SetOption = admin, inherit, set
	report.MembershipSet = !admin && !inherit && set
	report.Passed = report.RuntimeRoleReady && report.MembershipSet
	if !report.Passed {
		return rollbackMembership(tx, report, "GCP-03 origin runtime membership postcondition failed")
	}
	if commit == nil {
		return rollbackMembership(tx, report, "GCP-03 origin membership commit handler is unavailable")
	}
	if err := commit(ctx, tx); err != nil {
		report.Passed = false
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin membership commit outcome is uncertain; reconcile before retry")
	}
	report.TransactionOutcome = "committed"
	return report, nil
}

func rollbackMembership(tx pgx.Tx, report MembershipReport, message string) (MembershipReport, error) {
	report.Passed = false
	if err := tx.Rollback(context.Background()); err != nil {
		report.TransactionOutcome = "outcome_uncertain"
		return report, fmt.Errorf("%s; rollback outcome is unconfirmed, reconcile before retry", message)
	}
	report.TransactionOutcome = "rolled_back"
	return report, fmt.Errorf("%s; transaction rolled back", message)
}

func ReadAdminPassword(path string) (string, error) {
	if path != AdminPasswordFile {
		return "", errors.New("GCP-03 origin admin password path is not the reviewed secret mount")
	}
	value, err := os.ReadFile(path)
	if err != nil || len(value) < 32 || len(value) > 128 || strings.ContainsAny(string(value), "\r\n\x00") {
		return "", errors.New("GCP-03 origin admin password secret is unavailable or malformed")
	}
	return string(value), nil
}

func RunBootstrap(ctx context.Context, config Config, password string) (BootstrapReport, error) {
	if config.Phase != PhaseBootstrap || config.Login != AdminLogin || config.DatabaseURL != AdminDatabaseURL || password == "" {
		return BootstrapReport{}, errors.New("GCP-03 origin bootstrap configuration is invalid")
	}
	conn, closeConnector, err := cloudsql.Connect(ctx, config.DatabaseURL, password, config.Instance, true)
	if err != nil {
		return BootstrapReport{}, err
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	return bootstrapOnConnection(ctx, conn)
}

func bootstrapOnConnection(ctx context.Context, conn *pgx.Conn) (BootstrapReport, error) {
	return bootstrapOnConnectionWithPlan(ctx, conn, bootstrapStatements, func(ctx context.Context, tx pgx.Tx) error {
		return tx.Commit(ctx)
	})
}

func bootstrapOnConnectionWithCommit(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (BootstrapReport, error) {
	return bootstrapOnConnectionWithPlan(ctx, conn, bootstrapStatements, commit)
}

func bootstrapOnConnectionWithStatements(ctx context.Context, conn *pgx.Conn, statements []string) (BootstrapReport, error) {
	return bootstrapOnConnectionWithPlan(ctx, conn, statements, func(ctx context.Context, tx pgx.Tx) error {
		return tx.Commit(ctx)
	})
}

func bootstrapOnConnectionWithPlan(ctx context.Context, conn *pgx.Conn, statements []string, commit func(context.Context, pgx.Tx) error) (BootstrapReport, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return BootstrapReport{TransactionOutcome: "not_started"}, errors.New("GCP-03 origin bootstrap could not begin its transaction")
	}
	var db, login string
	if err := tx.QueryRow(ctx, bootstrapIdentitySQL).Scan(&db, &login); err != nil || db != DatabaseName || login != AdminLogin {
		return rollbackBootstrap(tx, BootstrapReport{Database: db, Login: login}, "GCP-03 origin bootstrap connected to an unexpected target")
	}
	var relations, functions, extensions, schemas int64
	var plpgsql bool
	if err := tx.QueryRow(ctx, emptyBaselineSQL).Scan(&relations, &functions, &extensions, &plpgsql, &schemas); err != nil {
		return rollbackBootstrap(tx, BootstrapReport{Database: db, Login: login}, "GCP-03 origin bootstrap could not verify the empty baseline")
	}
	if relations != 0 || functions != 0 || extensions != 1 || !plpgsql || schemas != 0 {
		return rollbackBootstrap(tx, BootstrapReport{Database: db, Login: login, ApplicationSchemas: schemas, ApplicationRelations: relations, ApplicationFunctions: functions}, "GCP-03 origin bootstrap requires a pristine database; reconcile before retry")
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return rollbackBootstrap(tx, BootstrapReport{Database: db, Login: login}, "GCP-03 origin bootstrap failed inside its transaction")
		}
	}
	report, err := readBootstrapReport(ctx, tx)
	if err != nil {
		return rollbackBootstrap(tx, report, err.Error())
	}
	if !report.Passed {
		return rollbackBootstrap(tx, report, "GCP-03 origin bootstrap checks failed inside its transaction")
	}
	if commit == nil {
		return rollbackBootstrap(tx, report, "GCP-03 origin bootstrap commit handler is unavailable")
	}
	if err := commit(ctx, tx); err != nil {
		report.Passed = false
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin bootstrap commit outcome is uncertain; reconcile before retry")
	}
	report.TransactionOutcome = "committed"
	return report, nil
}

func rollbackBootstrap(tx pgx.Tx, report BootstrapReport, message string) (BootstrapReport, error) {
	report.Passed = false
	if err := tx.Rollback(context.Background()); err != nil {
		report.TransactionOutcome = "outcome_uncertain"
		return report, fmt.Errorf("%s; rollback outcome is unconfirmed, reconcile before retry", message)
	}
	report.TransactionOutcome = "rolled_back"
	return report, fmt.Errorf("%s; transaction rolled back", message)
}

func readBootstrapReport(ctx context.Context, tx pgx.Tx) (BootstrapReport, error) {
	report := BootstrapReport{Database: DatabaseName, Login: AdminLogin, RolesPrepared: []string{"espacigo_gadm_bootstrap", "espacigo_gcp03_observer", "espacigo_runtime"}}
	if err := collectExtensionsTx(ctx, tx, &report.Extensions); err != nil {
		return report, err
	}
	var appRelations, appFunctions int64
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f')
     AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
	     AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))`).Scan(&appRelations, &appFunctions); err != nil {
		return report, errors.New("GCP-03 origin bootstrap could not verify extension and application object ownership")
	}
	var extensionSchemas int64
	if err := tx.QueryRow(ctx, schemaInventorySQL).Scan(&report.ApplicationSchemas, &extensionSchemas); err != nil {
		return report, errors.New("GCP-03 origin bootstrap could not classify extension-generated schemas")
	}
	report.ApplicationRelations = appRelations
	report.ApplicationFunctions = appFunctions
	var runtimeSetMembership bool
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*)=3 FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_runtime','espacigo_gadm_bootstrap','espacigo_gcp03_observer') AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit),
  has_database_privilege('espacigo_gcp03_observer','espacigo_scratch','CONNECT'),
	 has_schema_privilege('espacigo_gcp03_observer','public','USAGE'),
	 NOT has_database_privilege('espacigo_gcp03_observer','espacigo_scratch','CREATE') AND NOT has_database_privilege('espacigo_gcp03_observer','espacigo_scratch','TEMP') AND NOT has_schema_privilege('espacigo_gcp03_observer','public','CREATE'),
	 EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid='espacigo_runtime'::regrole AND m.member=current_user::regrole AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option)`).Scan(&report.RolesReady, &report.ObserverConnect, &report.ObserverPublicUsage, &report.ObserverNoWrite, &runtimeSetMembership); err != nil {
		return report, errors.New("GCP-03 origin bootstrap could not verify the fixed role allowlist and grants")
	}
	report.Passed = exactExtensions(report.Extensions) && report.RolesReady && runtimeSetMembership && report.ObserverConnect && report.ObserverPublicUsage && report.ObserverNoWrite && report.ApplicationSchemas == 0 && appRelations == 0 && appFunctions == 0
	if !report.Passed {
		return report, errors.New("GCP-03 origin bootstrap checks did not pass")
	}
	return report, nil
}

func Observe(ctx context.Context, config Config) (ObserveReport, error) {
	if config.Phase != PhaseObserve || config.Login != ObserverLogin || config.DatabaseURL != ObserverDatabaseURL || config.PasswordFile != "" || !config.PrivateIP {
		return ObserveReport{}, errors.New("GCP-03 origin observer configuration is invalid")
	}
	conn, closeConnector, err := cloudsql.ConnectIAM(ctx, config.DatabaseURL, config.Instance, true)
	if err != nil {
		return ObserveReport{}, err
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	return observeOnConnection(ctx, conn)
}

func observeOnConnection(ctx context.Context, conn *pgx.Conn) (ObserveReport, error) {
	return observeOnConnectionWithCommit(ctx, conn, func(ctx context.Context, tx pgx.Tx) error {
		return tx.Commit(ctx)
	})
}

func observeOnConnectionWithCommit(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (ObserveReport, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return ObserveReport{}, errors.New("GCP-03 origin observer could not begin its read-only transaction")
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SET TRANSACTION READ ONLY`); err != nil {
		return ObserveReport{}, errors.New("GCP-03 origin observer could not enforce read-only mode")
	}
	report := ObserveReport{}
	if err := tx.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::integer`).Scan(&report.Database, &report.Login, &report.ServerVersionNum); err != nil {
		return report, errors.New("GCP-03 origin observer could not identify the target session")
	}
	if report.Database != DatabaseName || report.Login != ObserverLogin {
		return report, errors.New("GCP-03 origin observer connected with an unexpected database identity")
	}
	if err := collectExtensionsTx(ctx, tx, &report.Extensions); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, schemaInventorySQL).Scan(&report.ApplicationSchemas, &report.ExtensionOwnedSchemas); err != nil {
		return report, errors.New("GCP-03 origin observer could not classify extension-generated schemas")
	}
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f')),
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f') AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f') AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
	  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))`).Scan(&report.Relations, &report.ExtensionOwnedRelations, &report.ApplicationRelations, &report.ExtensionOwnedFunctions, &report.ApplicationFunctions); err != nil {
		return report, errors.New("GCP-03 origin observer could not verify the extension-owned empty baseline")
	}
	var rolesReady, observerMembership, observerPermissions bool
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*)=3 FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_runtime','espacigo_gadm_bootstrap','espacigo_gcp03_observer') AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit),
	 pg_catalog.pg_has_role(current_user,'espacigo_gcp03_observer','member'),
	 has_database_privilege(current_user,'espacigo_scratch','CONNECT') AND has_schema_privilege(current_user,'public','USAGE')
	 AND NOT has_database_privilege(current_user,'espacigo_scratch','CREATE') AND NOT has_database_privilege(current_user,'espacigo_scratch','TEMP')
	 AND NOT has_schema_privilege(current_user,'public','CREATE')`).Scan(&rolesReady, &observerMembership, &observerPermissions); err != nil {
		return report, errors.New("GCP-03 origin observer could not verify the bootstrap role allowlist")
	}
	report.RolesReady, report.ObserverMembership, report.ObserverPermissions = rolesReady, observerMembership, observerPermissions
	report.Passed = report.ServerVersionNum >= 180000 && exactExtensions(report.Extensions) && rolesReady && observerMembership && observerPermissions && report.ApplicationSchemas == 0 && report.ApplicationRelations == 0 && report.ApplicationFunctions == 0 && report.Relations == report.ExtensionOwnedRelations
	if commit == nil {
		report.Passed = false
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin observer commit handler is unavailable")
	}
	if err := commit(ctx, tx); err != nil {
		report.Passed = false
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin observer read-only transaction outcome is uncertain")
	}
	report.TransactionOutcome = "committed"
	return report, nil
}

func collectExtensionsTx(ctx context.Context, tx pgx.Tx, out *[]Extension) error {
	rows, err := tx.Query(ctx, `SELECT extname,extversion FROM pg_catalog.pg_extension ORDER BY extname`)
	if err != nil {
		return errors.New("GCP-03 origin could not observe installed extensions")
	}
	defer rows.Close()
	for rows.Next() {
		var item Extension
		if err := rows.Scan(&item.Name, &item.Version); err != nil {
			return errors.New("GCP-03 origin extension observation failed")
		}
		*out = append(*out, item)
	}
	if rows.Err() != nil {
		return errors.New("GCP-03 origin extension observation failed")
	}
	return nil
}

func exactExtensions(items []Extension) bool {
	want := map[string]bool{"btree_gist": false, "fuzzystrmatch": false, "plpgsql": false, "postgis": false, "postgis_tiger_geocoder": false, "postgis_topology": false, "unaccent": false}
	for _, item := range items {
		if _, ok := want[item.Name]; ok && expectedExtensionVersion(item.Name, item.Version) {
			want[item.Name] = true
		} else {
			return false
		}
	}
	for _, present := range want {
		if !present {
			return false
		}
	}
	return len(items) == len(want)
}

func expectedExtensionVersion(name, version string) bool {
	switch name {
	case "btree_gist":
		return version == "1.8"
	case "fuzzystrmatch":
		return version == "1.2"
	case "plpgsql":
		return version == "1.0"
	case "postgis", "postgis_tiger_geocoder", "postgis_topology":
		return strings.HasPrefix(version, "3.6.")
	case "unaccent":
		return version == "1.1"
	default:
		return false
	}
}

func WriteJSON(w interface{ Write([]byte) (int, error) }, report any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode GCP-03 origin report: %w", err)
	}
	return nil
}
