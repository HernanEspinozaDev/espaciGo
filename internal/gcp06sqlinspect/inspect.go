package gcp06sqlinspect

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

const (
	RunID              = "20261020-gcp06a"
	ProjectID          = "espacigo-testprod"
	InstanceName       = "espacigo-gcp06-20261020-gcp06a-pg"
	ConnectionName     = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
	DatabaseName       = "espacigo_gcp06_candidate_20261020-gcp06a"
	CredentialFile     = "/var/run/secrets/gcp06-sql-inspector/credentials.json"
	ExpectedSecretName = "espacigo-test-gcp06-sql-inspector-credential"
)

var catalogRoles = []string{
	"postgres",
	"cloudsqlsuperuser",
	"espacigo_schema_owner",
	"espacigo_runtime",
	"espacigo_api_login",
	"espacigo_gadm_bootstrap",
	"espacigo_gcp06_test_access",
}

const (
	queryIdentity = `SELECT session_user, current_user, current_database(), current_setting('server_version_num')::integer,
       current_setting('transaction_read_only')::boolean`
	queryRoles = `SELECT rolname, rolsuper, rolinherit, rolcreaterole, rolcreatedb, rolcanlogin,
       rolreplication, rolbypassrls
FROM pg_catalog.pg_roles
WHERE rolname = ANY($1::text[])
ORDER BY rolname`
	queryMemberships = `SELECT
       CASE WHEN parent.rolname = ANY($1::text[]) THEN parent.rolname ELSE 'other' END,
       CASE WHEN member.rolname = ANY($1::text[]) THEN member.rolname ELSE 'other' END,
       CASE WHEN grantor.rolname = ANY($1::text[]) THEN grantor.rolname ELSE 'other' END,
       membership.admin_option, membership.inherit_option, membership.set_option, count(*)::integer
FROM pg_catalog.pg_auth_members membership
JOIN pg_catalog.pg_roles parent ON parent.oid = membership.roleid
JOIN pg_catalog.pg_roles member ON member.oid = membership.member
JOIN pg_catalog.pg_roles grantor ON grantor.oid = membership.grantor
WHERE parent.rolname = ANY($1::text[]) OR member.rolname = ANY($1::text[])
GROUP BY 1,2,3,4,5,6
ORDER BY 1,2,3,4,5,6`
	queryExtensions = `SELECT extname, extversion
FROM pg_catalog.pg_extension
ORDER BY extname`
	querySchemaState = `SELECT
  pg_catalog.to_regclass('public.schema_migrations') IS NOT NULL,
  pg_catalog.to_regclass('public.usuario') IS NOT NULL,
  pg_catalog.has_schema_privilege(current_user, 'public', 'USAGE'),
  CASE WHEN pg_catalog.to_regclass('public.schema_migrations') IS NULL THEN false
       ELSE pg_catalog.has_table_privilege(current_user, pg_catalog.to_regclass('public.schema_migrations'), 'SELECT') END,
  CASE WHEN pg_catalog.to_regclass('public.usuario') IS NULL THEN false
       ELSE pg_catalog.has_table_privilege(current_user, pg_catalog.to_regclass('public.usuario'), 'SELECT') END`
	queryOwnerAggregates = `SELECT
  CASE WHEN owner.rolname = ANY($1::text[]) THEN owner.rolname ELSE 'other' END,
  count(*)::bigint
FROM pg_catalog.pg_class relation
JOIN pg_catalog.pg_namespace namespace ON namespace.oid = relation.relnamespace
JOIN pg_catalog.pg_roles owner ON owner.oid = relation.relowner
WHERE namespace.nspname = 'public' AND relation.relkind IN ('r','p','S','v','m','f')
GROUP BY 1
ORDER BY 1`
)

type Credentials struct {
	Username string
	Password []byte
}

type Role struct {
	Name        string `json:"name"`
	Superuser   bool   `json:"superuser"`
	Inherit     bool   `json:"inherit"`
	CreateRole  bool   `json:"create_role"`
	CreateDB    bool   `json:"create_db"`
	CanLogin    bool   `json:"can_login"`
	Replication bool   `json:"replication"`
	BypassRLS   bool   `json:"bypass_rls"`
}

type Membership struct {
	Role        string `json:"role"`
	Member      string `json:"member"`
	Grantor     string `json:"grantor"`
	AdminOption bool   `json:"admin_option"`
	Inherit     bool   `json:"inherit"`
	Set         bool   `json:"set"`
	Count       int    `json:"count"`
}

type Extension struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type OwnerAggregate struct {
	OwnerClass string `json:"owner_class"`
	Objects    int64  `json:"objects"`
}

type Report struct {
	RunID                     string           `json:"run_id"`
	Database                  string           `json:"database"`
	ServerMajor               int              `json:"server_major"`
	TransactionReadOnly       bool             `json:"transaction_read_only"`
	InspectedRoles            []Role           `json:"roles"`
	RoleMemberships           []Membership     `json:"role_memberships"`
	Extensions                []Extension      `json:"extensions"`
	MigrationHistoryExists    bool             `json:"migration_history_exists"`
	UsuarioRelationExists     bool             `json:"usuario_relation_exists"`
	InspectorHasPublicUsage   bool             `json:"inspector_has_public_usage"`
	InspectorCanReadHistory   bool             `json:"inspector_can_read_history"`
	InspectorCanReadUserTable bool             `json:"inspector_can_read_user_table"`
	PublicObjectOwnerCounts   []OwnerAggregate `json:"public_object_owner_counts"`
}

func ValidateTarget(runID, connectionName, databaseName, username string) error {
	if runID != RunID || connectionName != ConnectionName || databaseName != DatabaseName {
		return errors.New("SQL inspector target must match the pinned GCP-06 candidate")
	}
	if !validInspectorLogin(username) {
		return errors.New("SQL inspector credential must contain the one-time GCP-06 login")
	}
	return nil
}

func validInspectorLogin(username string) bool {
	if len(username) != len("gcp06_inspect_")+12 || username[:len("gcp06_inspect_")] != "gcp06_inspect_" {
		return false
	}
	for _, r := range username[len("gcp06_inspect_"):] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// AllowedStatements exposes query text for audit/tests; callers cannot supply SQL.
func AllowedStatements() []string {
	return []string{queryIdentity, queryRoles, queryMemberships, queryExtensions, querySchemaState, queryOwnerAggregates}
}

func Inspect(ctx context.Context, conn *pgx.Conn, username string) (Report, error) {
	if conn == nil || !validInspectorLogin(username) {
		return Report{}, errors.New("candidate SQL connection and one-time inspector login are required")
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Report{}, errors.New("could not begin read-only catalog inspection")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var report Report
	var sessionUser, currentUser string
	var serverVersion int
	if err := tx.QueryRow(ctx, queryIdentity).Scan(&sessionUser, &currentUser, &report.Database, &serverVersion, &report.TransactionReadOnly); err != nil {
		return Report{}, errors.New("candidate catalog identity query failed")
	}
	if sessionUser != username || currentUser != username || report.Database != DatabaseName || serverVersion < 180000 || !report.TransactionReadOnly {
		return Report{}, errors.New("catalog connection is not the pinned PostgreSQL 18 candidate in a read-only transaction")
	}
	report.RunID = RunID
	report.ServerMajor = serverVersion / 10000

	rolesToInspect := append(append([]string(nil), catalogRoles...), username)
	rows, err := tx.Query(ctx, queryRoles, rolesToInspect)
	if err != nil {
		return Report{}, errors.New("allowlisted role catalog query failed")
	}
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.Name, &role.Superuser, &role.Inherit, &role.CreateRole, &role.CreateDB, &role.CanLogin, &role.Replication, &role.BypassRLS); err != nil {
			rows.Close()
			return Report{}, errors.New("allowlisted role catalog result was invalid")
		}
		report.InspectedRoles = append(report.InspectedRoles, role)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, errors.New("allowlisted role catalog query failed")
	}
	rows.Close()

	rows, err = tx.Query(ctx, queryMemberships, rolesToInspect)
	if err != nil {
		return Report{}, errors.New("allowlisted role membership query failed")
	}
	for rows.Next() {
		var membership Membership
		if err := rows.Scan(&membership.Role, &membership.Member, &membership.Grantor, &membership.AdminOption, &membership.Inherit, &membership.Set, &membership.Count); err != nil {
			rows.Close()
			return Report{}, errors.New("allowlisted role membership result was invalid")
		}
		report.RoleMemberships = append(report.RoleMemberships, membership)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, errors.New("allowlisted role membership query failed")
	}
	rows.Close()

	rows, err = tx.Query(ctx, queryExtensions)
	if err != nil {
		return Report{}, errors.New("extension catalog query failed")
	}
	for rows.Next() {
		var extension Extension
		if err := rows.Scan(&extension.Name, &extension.Version); err != nil {
			rows.Close()
			return Report{}, errors.New("extension catalog result was invalid")
		}
		report.Extensions = append(report.Extensions, extension)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, errors.New("extension catalog query failed")
	}
	rows.Close()

	if err := tx.QueryRow(ctx, querySchemaState).Scan(&report.MigrationHistoryExists, &report.UsuarioRelationExists, &report.InspectorHasPublicUsage, &report.InspectorCanReadHistory, &report.InspectorCanReadUserTable); err != nil {
		return Report{}, errors.New("allowlisted relation and effective privilege query failed")
	}
	rows, err = tx.Query(ctx, queryOwnerAggregates, rolesToInspect)
	if err != nil {
		return Report{}, errors.New("public object owner aggregate query failed")
	}
	for rows.Next() {
		var owner OwnerAggregate
		if err := rows.Scan(&owner.OwnerClass, &owner.Objects); err != nil {
			rows.Close()
			return Report{}, errors.New("public object owner aggregate was invalid")
		}
		report.PublicObjectOwnerCounts = append(report.PublicObjectOwnerCounts, owner)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, errors.New("public object owner aggregate query failed")
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return Report{}, errors.New("read-only catalog inspection did not complete")
	}
	if len(report.InspectedRoles) == 0 || len(report.Extensions) == 0 {
		return Report{}, errors.New("candidate catalog response was incomplete")
	}
	sort.Slice(report.InspectedRoles, func(i, j int) bool { return report.InspectedRoles[i].Name < report.InspectedRoles[j].Name })
	return report, nil
}
