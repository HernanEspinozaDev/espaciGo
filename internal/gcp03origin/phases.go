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

const bootstrapSQL = `BEGIN;
DO $guard$
BEGIN
  IF current_database() <> 'espacigo_scratch' OR current_user <> 'postgres' THEN
    RAISE EXCEPTION 'GCP-03 origin bootstrap refused unexpected database or login';
  END IF;
END
$guard$;
CREATE EXTENSION postgis;
CREATE EXTENSION btree_gist;
CREATE EXTENSION fuzzystrmatch;
CREATE EXTENSION postgis_tiger_geocoder;
CREATE EXTENSION postgis_topology;
CREATE EXTENSION unaccent;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_gcp03_observer NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT CONNECT ON DATABASE espacigo_scratch TO espacigo_runtime, espacigo_gcp03_observer;
GRANT USAGE ON SCHEMA public TO espacigo_gcp03_observer;
COMMIT;`

const emptyBaselineSQL = `SELECT
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f'))::bigint,
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema')::bigint,
  (SELECT count(*) FROM pg_catalog.pg_extension)::bigint,
  EXISTS (SELECT 1 FROM pg_catalog.pg_extension WHERE extname='plpgsql' AND extversion='1.0'),
  (SELECT count(*) FROM pg_catalog.pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname NOT IN ('public','information_schema'))::bigint`

type BootstrapReport struct {
	Database             string      `json:"database"`
	Login                string      `json:"login"`
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
	var db, login string
	if err := conn.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&db, &login); err != nil || db != DatabaseName || login != AdminLogin {
		return BootstrapReport{}, errors.New("GCP-03 origin bootstrap connected to an unexpected target")
	}
	var relations, functions, extensions, schemas int64
	var plpgsql bool
	if err := conn.QueryRow(ctx, emptyBaselineSQL).Scan(&relations, &functions, &extensions, &plpgsql, &schemas); err != nil {
		return BootstrapReport{}, errors.New("GCP-03 origin bootstrap could not verify the empty baseline")
	}
	if relations != 0 || functions != 0 || extensions != 1 || !plpgsql || schemas != 0 {
		return BootstrapReport{Database: db, Login: login, ApplicationSchemas: schemas, ApplicationRelations: relations, ApplicationFunctions: functions}, errors.New("GCP-03 origin bootstrap requires a pristine database; do not retry without reconciliation")
	}
	if _, err := conn.PgConn().Exec(ctx, bootstrapSQL).ReadAll(); err != nil {
		return BootstrapReport{Database: db, Login: login}, errors.New("GCP-03 origin bootstrap failed; outcome requires reconciliation before any retry")
	}
	report, err := readBootstrapReport(ctx, conn)
	if err != nil {
		return report, err
	}
	return report, nil
}

func readBootstrapReport(ctx context.Context, conn *pgx.Conn) (BootstrapReport, error) {
	report := BootstrapReport{Database: DatabaseName, Login: AdminLogin, RolesPrepared: []string{"espacigo_gadm_bootstrap", "espacigo_gcp03_observer", "espacigo_runtime"}}
	if err := collectExtensions(ctx, conn, &report.Extensions); err != nil {
		return report, err
	}
	var appRelations, appFunctions int64
	if err := conn.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM pg_catalog.pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname NOT IN ('public','information_schema')
   AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_namespace'::regclass AND d.objid=n.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f')
     AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
     AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))`).Scan(&report.ApplicationSchemas, &appRelations, &appFunctions); err != nil {
		return report, errors.New("GCP-03 origin bootstrap could not verify extension and application object ownership")
	}
	report.ApplicationRelations = appRelations
	report.ApplicationFunctions = appFunctions
	if err := conn.QueryRow(ctx, `SELECT
  (SELECT count(*)=3 FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_runtime','espacigo_gadm_bootstrap','espacigo_gcp03_observer') AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit),
  has_database_privilege('espacigo_gcp03_observer','espacigo_scratch','CONNECT'),
	 has_schema_privilege('espacigo_gcp03_observer','public','USAGE'),
	 NOT has_database_privilege('espacigo_gcp03_observer','espacigo_scratch','CREATE') AND NOT has_database_privilege('espacigo_gcp03_observer','espacigo_scratch','TEMP') AND NOT has_schema_privilege('espacigo_gcp03_observer','public','CREATE')`).Scan(&report.RolesReady, &report.ObserverConnect, &report.ObserverPublicUsage, &report.ObserverNoWrite); err != nil {
		return report, errors.New("GCP-03 origin bootstrap could not verify the fixed role allowlist and grants")
	}
	report.Passed = exactExtensions(report.Extensions) && report.RolesReady && report.ObserverConnect && report.ObserverPublicUsage && report.ObserverNoWrite && report.ApplicationSchemas == 0 && appRelations == 0 && appFunctions == 0
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
	if err := tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM pg_catalog.pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname NOT IN ('public','information_schema')
   AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_namespace'::regclass AND d.objid=n.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname NOT IN ('public','information_schema')
   AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_namespace'::regclass AND d.objid=n.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f')),
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f') AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m','S','f') AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass)),
  (SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))`).Scan(&report.ApplicationSchemas, &report.ExtensionOwnedSchemas, &report.Relations, &report.ExtensionOwnedRelations, &report.ApplicationRelations, &report.ExtensionOwnedFunctions, &report.ApplicationFunctions); err != nil {
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
	if err := tx.Commit(ctx); err != nil {
		return report, errors.New("GCP-03 origin observer read-only transaction did not complete")
	}
	return report, nil
}

func collectExtensions(ctx context.Context, conn *pgx.Conn, out *[]Extension) error {
	rows, err := conn.Query(ctx, `SELECT extname,extversion FROM pg_catalog.pg_extension ORDER BY extname`)
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
