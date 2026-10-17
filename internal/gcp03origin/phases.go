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
	"github.com/jackc/pgx/v5/pgconn"
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
	Database           string                   `json:"database"`
	Login              string                   `json:"login"`
	Grantor            string                   `json:"grantor"`
	FailureStage       string                   `json:"failure_stage,omitempty"`
	SQLState           string                   `json:"sqlstate,omitempty"`
	TransactionOutcome string                   `json:"transaction_outcome"`
	RuntimeRoleReady   bool                     `json:"runtime_role_ready"`
	MembershipRowCount int                      `json:"membership_row_count"`
	Memberships        []RuntimeMembershipGrant `json:"memberships"`
	MembershipSet      bool                     `json:"membership_set"`
	EffectiveSet       bool                     `json:"effective_set"`
	AdminOption        bool                     `json:"admin_option"`
	InheritOption      bool                     `json:"inherit_option"`
	SetOption          bool                     `json:"set_option"`
	Passed             bool                     `json:"passed"`
}

// RuntimeMembershipGrant records one exact grantor row for the fixed runtime
// role and postgres member. Multiple rows are reported in full and rejected;
// the report never selects an arbitrary grantor.
type RuntimeMembershipGrant struct {
	Grantor       string `json:"grantor"`
	AdminOption   bool   `json:"admin_option"`
	InheritOption bool   `json:"inherit_option"`
	SetOption     bool   `json:"set_option"`
}

const runtimeMembershipGrant = `GRANT espacigo_runtime TO postgres WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`

const runtimeMembershipRolePostcheckSQL = `SELECT count(*)=1,
 COALESCE(bool_and(NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit), false),
 COALESCE(bool_or(pg_catalog.pg_has_role(current_user, oid, 'SET')), false)
 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`

const runtimeMembershipRowsPostcheckSQL = `SELECT grantor.rolname, m.admin_option, m.inherit_option, m.set_option
 FROM pg_catalog.pg_auth_members m
 JOIN pg_catalog.pg_roles granted_role ON granted_role.oid=m.roleid
 JOIN pg_catalog.pg_roles member ON member.oid=m.member
 JOIN pg_catalog.pg_roles grantor ON grantor.oid=m.grantor
 WHERE granted_role.rolname='espacigo_runtime' AND member.rolname=current_user
 ORDER BY grantor.rolname`

func RunMembership(ctx context.Context, config Config, password string) (MembershipReport, error) {
	if config.Phase != PhaseMembership || config.Login != AdminLogin || config.DatabaseURL != AdminDatabaseURL || password == "" || !config.PrivateIP {
		return MembershipReport{FailureStage: "configuration", TransactionOutcome: "not_started"}, errors.New("GCP-03 origin membership configuration is invalid")
	}
	conn, closeConnector, err := cloudsql.Connect(ctx, config.DatabaseURL, password, config.Instance, true)
	if err != nil {
		return MembershipReport{FailureStage: "connect", SQLState: pgSQLState(err), TransactionOutcome: "not_started"}, errors.New("GCP-03 origin membership connection failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	return membershipOnConnection(ctx, conn, func(ctx context.Context, tx pgx.Tx) error { return tx.Commit(ctx) })
}

func membershipOnConnection(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (MembershipReport, error) {
	return membershipOnConnectionWithPostcheck(ctx, conn, commit)
}

func membershipOnConnectionWithPostcheck(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (MembershipReport, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return MembershipReport{FailureStage: "begin_transaction", SQLState: pgSQLState(err), TransactionOutcome: "not_started"}, errors.New("GCP-03 origin membership could not begin its transaction")
	}
	report := MembershipReport{Memberships: []RuntimeMembershipGrant{}}
	if err := tx.QueryRow(ctx, `SELECT current_database(), current_user`).Scan(&report.Database, &report.Login); err != nil || report.Database != DatabaseName || report.Login != AdminLogin {
		return rollbackMembership(tx, report, "identity_precheck", err, "GCP-03 origin membership connected to an unexpected identity")
	}
	report.Grantor = report.Login
	var actorSuper, actorExpected bool
	if err := tx.QueryRow(ctx, `SELECT r.rolsuper, r.rolcanlogin AND r.rolcreaterole FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&actorSuper, &actorExpected); err != nil || actorSuper || !actorExpected {
		return rollbackMembership(tx, report, "actor_profile_precheck", err, "GCP-03 origin membership requires the fixed non-superuser postgres login")
	}
	var runtimeOK bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit) FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`).Scan(&runtimeOK); err != nil || !runtimeOK {
		return rollbackMembership(tx, report, "runtime_role_precheck", err, "GCP-03 origin membership requires the existing restricted runtime role")
	}
	// Exactly one mutating SQL statement is permitted in this transaction.
	if _, err := tx.Exec(ctx, runtimeMembershipGrant); err != nil {
		return rollbackMembership(tx, report, "grant", err, "GCP-03 origin runtime membership grant failed")
	}
	var roleExists, roleSafe bool
	if err := tx.QueryRow(ctx, runtimeMembershipRolePostcheckSQL).Scan(&roleExists, &roleSafe, &report.EffectiveSet); err != nil {
		return rollbackMembership(tx, report, "postcheck_role_query", err, "GCP-03 origin runtime role postcondition could not be read")
	}
	report.RuntimeRoleReady = roleExists && roleSafe
	rows, err := tx.Query(ctx, runtimeMembershipRowsPostcheckSQL)
	if err != nil {
		return rollbackMembership(tx, report, "postcheck_membership_query", err, "GCP-03 origin runtime membership rows could not be read")
	}
	for rows.Next() {
		var membership RuntimeMembershipGrant
		if err := rows.Scan(&membership.Grantor, &membership.AdminOption, &membership.InheritOption, &membership.SetOption); err != nil {
			rows.Close()
			return rollbackMembership(tx, report, "postcheck_membership_query", err, "GCP-03 origin runtime membership rows could not be read")
		}
		report.Memberships = append(report.Memberships, membership)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return rollbackMembership(tx, report, "postcheck_membership_query", rowsErr, "GCP-03 origin runtime membership rows could not be read")
	}
	stage := evaluateRuntimeMembershipPostcheck(&report)
	if stage != "" {
		return rollbackMembership(tx, report, stage, nil, "GCP-03 origin runtime membership postcondition failed")
	}
	if commit == nil {
		return rollbackMembership(tx, report, "commit_configuration", nil, "GCP-03 origin membership commit handler is unavailable")
	}
	if err := commit(ctx, tx); err != nil {
		report.Passed = false
		report.FailureStage = "commit"
		report.SQLState = pgSQLState(err)
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin membership commit outcome is uncertain; reconcile before retry")
	}
	report.TransactionOutcome = "committed"
	return report, nil
}

func evaluateRuntimeMembershipPostcheck(report *MembershipReport) string {
	report.MembershipRowCount = len(report.Memberships)
	report.MembershipSet = false
	report.Passed = false
	report.Grantor = ""
	report.AdminOption = false
	report.InheritOption = false
	report.SetOption = false
	switch report.MembershipRowCount {
	case 0:
		return "postcheck_no_membership"
	case 1:
		membership := report.Memberships[0]
		report.Grantor = membership.Grantor
		report.AdminOption = membership.AdminOption
		report.InheritOption = membership.InheritOption
		report.SetOption = membership.SetOption
		report.MembershipSet = !membership.AdminOption && !membership.InheritOption && membership.SetOption
	default:
		return "postcheck_multiple_memberships"
	}
	if !report.RuntimeRoleReady || !report.MembershipSet || !report.EffectiveSet {
		return "postcondition"
	}
	report.Passed = true
	return ""
}

func rollbackMembership(tx pgx.Tx, report MembershipReport, stage string, cause error, message string) (MembershipReport, error) {
	report.Passed = false
	report.FailureStage = stage
	report.SQLState = pgSQLState(cause)
	if err := tx.Rollback(context.Background()); err != nil {
		report.TransactionOutcome = "outcome_uncertain"
		return report, fmt.Errorf("%s; rollback outcome is unconfirmed, reconcile before retry", message)
	}
	report.TransactionOutcome = "rolled_back"
	return report, fmt.Errorf("%s; transaction rolled back", message)
}

func pgSQLState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// MembershipDiagnosticReport contains only fixed identity and catalog facts.
// It deliberately excludes application rows, SQL text, and raw database errors.
type MembershipDiagnosticReport struct {
	Database                       string                           `json:"database"`
	Login                          string                           `json:"login"`
	ReadOnly                       bool                             `json:"read_only"`
	TransactionOutcome             string                           `json:"transaction_outcome"`
	FailureStage                   string                           `json:"failure_stage,omitempty"`
	SQLState                       string                           `json:"sqlstate,omitempty"`
	ActorExists                    bool                             `json:"actor_exists"`
	ActorCanLogin                  bool                             `json:"actor_can_login"`
	ActorCanCreateRole             bool                             `json:"actor_can_create_role"`
	ActorCanCreateDB               bool                             `json:"actor_can_create_db"`
	ActorSuperuser                 bool                             `json:"actor_superuser"`
	RuntimeRoleExists              bool                             `json:"runtime_role_exists"`
	RuntimeRoleCanLogin            bool                             `json:"runtime_role_can_login"`
	RuntimeRoleSuperuser           bool                             `json:"runtime_role_superuser"`
	RuntimeRoleCanCreateDB         bool                             `json:"runtime_role_can_create_db"`
	RuntimeRoleCanCreateRole       bool                             `json:"runtime_role_can_create_role"`
	RuntimeRoleReplication         bool                             `json:"runtime_role_replication"`
	RuntimeRoleBypassRLS           bool                             `json:"runtime_role_bypass_rls"`
	RuntimeRoleInherit             bool                             `json:"runtime_role_inherit"`
	RuntimeRoleReady               bool                             `json:"runtime_role_ready"`
	Memberships                    []MembershipDiagnosticMembership `json:"memberships"`
	MembershipSet                  bool                             `json:"membership_set"`
	EffectiveSet                   bool                             `json:"effective_set"`
	CurrentCloudSQLSuperuserMember bool                             `json:"current_cloudsql_superuser_member"`
	ReportComplete                 bool                             `json:"report_complete"`
}

type MembershipDiagnosticMembership struct {
	Grantor       string `json:"grantor"`
	AdminOption   bool   `json:"admin_option"`
	InheritOption bool   `json:"inherit_option"`
	SetOption     bool   `json:"set_option"`
}

const membershipDiagnosticActorSQL = `SELECT count(*)=1,
 COALESCE(bool_or(rolcanlogin), false), COALESCE(bool_or(rolcreaterole), false),
 COALESCE(bool_or(rolcreatedb), false), COALESCE(bool_or(rolsuper), false)
 FROM pg_catalog.pg_roles WHERE rolname='postgres'`

const membershipDiagnosticRuntimeSQL = `SELECT count(*)=1,
 COALESCE(bool_or(rolcanlogin), false), COALESCE(bool_or(rolsuper), false),
 COALESCE(bool_or(rolcreatedb), false), COALESCE(bool_or(rolcreaterole), false),
 COALESCE(bool_or(rolreplication), false), COALESCE(bool_or(rolbypassrls), false),
 COALESCE(bool_or(rolinherit), false)
 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`

const membershipDiagnosticRowsSQL = `SELECT grantor.rolname, m.admin_option, m.inherit_option, m.set_option
 FROM pg_catalog.pg_auth_members m
 JOIN pg_catalog.pg_roles role ON role.oid=m.roleid
 JOIN pg_catalog.pg_roles member ON member.oid=m.member
 JOIN pg_catalog.pg_roles grantor ON grantor.oid=m.grantor
 WHERE role.rolname='espacigo_runtime' AND member.rolname='postgres'
 ORDER BY grantor.rolname`

const membershipDiagnosticEffectiveSQL = `SELECT
 COALESCE((SELECT pg_catalog.pg_has_role(current_user, r.oid, 'SET') FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_runtime'), false),
 COALESCE((SELECT pg_catalog.pg_has_role(current_user, r.oid, 'MEMBER') FROM pg_catalog.pg_roles r WHERE r.rolname='cloudsqlsuperuser'), false)`

func RunMembershipDiagnostic(ctx context.Context, config Config, password string) (MembershipDiagnosticReport, error) {
	report := MembershipDiagnosticReport{ReadOnly: true, TransactionOutcome: "not_started"}
	if config.Phase != PhaseMembershipCheck || config.Login != AdminLogin || config.DatabaseURL != AdminDatabaseURL || password == "" || !config.PrivateIP {
		report.FailureStage = "configuration"
		return report, errors.New("GCP-03 origin membership diagnostic configuration is invalid")
	}
	conn, closeConnector, err := cloudsql.Connect(ctx, config.DatabaseURL, password, config.Instance, true)
	if err != nil {
		report.FailureStage, report.SQLState = "connect", pgSQLState(err)
		return report, errors.New("GCP-03 origin membership diagnostic connection failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	return membershipDiagnosticOnConnection(ctx, conn, func(ctx context.Context, tx pgx.Tx) error { return tx.Commit(ctx) })
}

func beginMembershipDiagnosticTx(ctx context.Context, conn *pgx.Conn) (pgx.Tx, error) {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, membershipDiagnosticBeginError{cause: err, outcome: "not_started"}
	}
	if _, err := tx.Exec(ctx, `SET TRANSACTION READ ONLY`); err != nil {
		outcome := "rolled_back"
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			outcome = "outcome_uncertain"
		}
		return nil, membershipDiagnosticBeginError{cause: err, outcome: outcome}
	}
	return tx, nil
}

type membershipDiagnosticBeginError struct {
	cause   error
	outcome string
}

func (e membershipDiagnosticBeginError) Error() string {
	return "read-only diagnostic transaction could not be started"
}
func (e membershipDiagnosticBeginError) Unwrap() error { return e.cause }

func membershipDiagnosticOnConnection(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (MembershipDiagnosticReport, error) {
	report := MembershipDiagnosticReport{ReadOnly: true, TransactionOutcome: "not_started", Memberships: []MembershipDiagnosticMembership{}}
	tx, err := beginMembershipDiagnosticTx(ctx, conn)
	if err != nil {
		var beginErr membershipDiagnosticBeginError
		if errors.As(err, &beginErr) {
			report.TransactionOutcome = beginErr.outcome
			report.SQLState = pgSQLState(beginErr.cause)
		}
		report.FailureStage = "begin_read_only_transaction"
		return report, errors.New("GCP-03 origin membership diagnostic could not begin a read-only transaction")
	}
	report.TransactionOutcome = "active"
	if err := tx.QueryRow(ctx, `SELECT current_database(), current_user`).Scan(&report.Database, &report.Login); err != nil || report.Database != DatabaseName || report.Login != AdminLogin {
		return rollbackMembershipDiagnostic(ctx, tx, report, "identity", err)
	}
	if err := tx.QueryRow(ctx, membershipDiagnosticActorSQL).Scan(&report.ActorExists, &report.ActorCanLogin, &report.ActorCanCreateRole, &report.ActorCanCreateDB, &report.ActorSuperuser); err != nil {
		return rollbackMembershipDiagnostic(ctx, tx, report, "actor_profile", err)
	}
	if err := tx.QueryRow(ctx, membershipDiagnosticRuntimeSQL).Scan(&report.RuntimeRoleExists, &report.RuntimeRoleCanLogin, &report.RuntimeRoleSuperuser, &report.RuntimeRoleCanCreateDB, &report.RuntimeRoleCanCreateRole, &report.RuntimeRoleReplication, &report.RuntimeRoleBypassRLS, &report.RuntimeRoleInherit); err != nil {
		return rollbackMembershipDiagnostic(ctx, tx, report, "runtime_role_profile", err)
	}
	report.RuntimeRoleReady = report.RuntimeRoleExists && !report.RuntimeRoleCanLogin && !report.RuntimeRoleSuperuser && !report.RuntimeRoleCanCreateDB && !report.RuntimeRoleCanCreateRole && !report.RuntimeRoleReplication && !report.RuntimeRoleBypassRLS && !report.RuntimeRoleInherit
	rows, err := tx.Query(ctx, membershipDiagnosticRowsSQL)
	if err != nil {
		return rollbackMembershipDiagnostic(ctx, tx, report, "membership_rows", err)
	}
	for rows.Next() {
		var membership MembershipDiagnosticMembership
		if err := rows.Scan(&membership.Grantor, &membership.AdminOption, &membership.InheritOption, &membership.SetOption); err != nil {
			rows.Close()
			return rollbackMembershipDiagnostic(ctx, tx, report, "membership_rows", err)
		}
		report.Memberships = append(report.Memberships, membership)
		if membership.Grantor == AdminLogin && !membership.AdminOption && !membership.InheritOption && membership.SetOption {
			report.MembershipSet = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return rollbackMembershipDiagnostic(ctx, tx, report, "membership_rows", err)
	}
	rows.Close()
	if err := tx.QueryRow(ctx, membershipDiagnosticEffectiveSQL).Scan(&report.EffectiveSet, &report.CurrentCloudSQLSuperuserMember); err != nil {
		return rollbackMembershipDiagnostic(ctx, tx, report, "effective_membership", err)
	}
	if commit == nil {
		return rollbackMembershipDiagnostic(ctx, tx, report, "commit_configuration", nil)
	}
	if err := commit(ctx, tx); err != nil {
		report.FailureStage, report.SQLState = "commit", pgSQLState(err)
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin membership diagnostic commit outcome is uncertain; reconcile before retry")
	}
	report.TransactionOutcome = "committed"
	report.ReportComplete = true
	return report, nil
}

func rollbackMembershipDiagnostic(ctx context.Context, tx pgx.Tx, report MembershipDiagnosticReport, stage string, cause error) (MembershipDiagnosticReport, error) {
	report.FailureStage, report.SQLState, report.ReportComplete = stage, pgSQLState(cause), false
	if err := tx.Rollback(ctx); err != nil {
		report.TransactionOutcome = "outcome_uncertain"
		return report, errors.New("GCP-03 origin membership diagnostic rollback outcome is uncertain; reconcile before retry")
	}
	report.TransactionOutcome = "rolled_back"
	return report, errors.New("GCP-03 origin membership diagnostic failed at a reported stage; no database changes were requested")
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
