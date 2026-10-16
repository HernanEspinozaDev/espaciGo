package gcp03scratch

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

// RetiredVerificationReport contains only fixed catalog metadata and pass/fail
// results. It never includes row data, SQL, passwords, or connection details.
type RetiredVerificationReport struct {
	Observation       dbbootstrap.ScratchObservation `json:"observation"`
	SchemaHistoryACL  []dbbootstrap.ACLObservation   `json:"schema_migrations_acl"`
	MigratorCanTemp   bool                           `json:"migrator_can_temp"`
	MigratorCanCreate bool                           `json:"migrator_can_create_public"`
	DatabaseOwner     string                         `json:"database_owner"`
	PublicSchemaOwner string                         `json:"public_schema_owner"`
	HistoryOwner      string                         `json:"schema_migrations_owner"`
	MigrationHistory  bool                           `json:"migration_history_v1_v61_pass"`
	MigratorRetired   bool                           `json:"migrator_retired_pass"`
	NoRoleMemberships bool                           `json:"migrator_no_memberships_pass"`
	OwnerHistoryACL   bool                           `json:"owner_history_acl_pass"`
	Passed            bool                           `json:"passed"`
}

func (r RetiredVerificationReport) IntegrityPassed() bool { return r.Passed }

// ObserveRetiredScratch collects every fixed check before deciding whether the
// retirement is complete. Callers must run it inside a read-only transaction.
func ObserveRetiredScratch(ctx context.Context, conn *pgx.Conn, migrationDir string) (RetiredVerificationReport, error) {
	var report RetiredVerificationReport
	if conn == nil || migrationDir == "" {
		return report, errors.New("GCP-03 retirement verification configuration is invalid")
	}
	if err := conn.QueryRow(ctx, `SELECT current_database() = 'espacigo_scratch'
		AND current_user = 'postgres'`).Scan(&report.NoRoleMemberships); err != nil {
		return report, errors.New("GCP-03 retirement verification session is unavailable")
	}
	sessionOK := report.NoRoleMemberships
	var err error
	report.Observation, err = dbbootstrap.ObserveScratch(ctx, conn, dbbootstrap.RoleMembershipRetired)
	if err != nil {
		return report, errors.New("GCP-03 could not collect fixed retirement catalog observations")
	}
	report.NoRoleMemberships = sessionOK
	if err := migrator.VerifyAllApplied(ctx, conn, migrationDir, MigrationCount); err == nil {
		report.MigrationHistory = true
	}
	if err := dbbootstrap.VerifyScratchMigratorRetired(ctx, conn); err == nil {
		report.MigratorRetired = true
	}
	var memberships bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_catalog.pg_auth_members m
		JOIN pg_catalog.pg_roles granted ON granted.oid = m.roleid
		JOIN pg_catalog.pg_roles member ON member.oid = m.member
		WHERE granted.rolname = 'espacigo_scratch_migrator'
		   OR member.rolname = 'espacigo_scratch_migrator'
	)`).Scan(&memberships); err != nil {
		return report, errors.New("GCP-03 could not observe scratch migrator memberships")
	}
	report.NoRoleMemberships = report.NoRoleMemberships && !memberships
	var ownerACLQueryPass bool
	if err := conn.QueryRow(ctx, `SELECT
		has_database_privilege('espacigo_scratch_migrator', 'espacigo_scratch', 'TEMP'),
		has_schema_privilege('espacigo_scratch_migrator', 'public', 'CREATE'),
		(SELECT r.rolname FROM pg_catalog.pg_database d JOIN pg_catalog.pg_roles r ON r.oid = d.datdba WHERE d.datname = 'espacigo_scratch'),
		(SELECT r.rolname FROM pg_catalog.pg_namespace n JOIN pg_catalog.pg_roles r ON r.oid = n.nspowner WHERE n.nspname = 'public'),
		pg_catalog.pg_get_userbyid(c.relowner)
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'schema_migrations' AND c.relkind = 'r'`).Scan(
		&report.MigratorCanTemp, &report.MigratorCanCreate, &report.DatabaseOwner, &report.PublicSchemaOwner, &report.HistoryOwner); err != nil {
		return report, errors.New("GCP-03 could not observe migrator effective privileges or history owner")
	}
	report.SchemaHistoryACL = make([]dbbootstrap.ACLObservation, 0)
	rows, err := conn.Query(ctx, `SELECT CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END,
		grantor.rolname, acl.privilege_type, acl.is_grantable
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace,
		LATERAL pg_catalog.aclexplode(coalesce(c.relacl, pg_catalog.acldefault('r', c.relowner))) acl
		LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
		LEFT JOIN pg_catalog.pg_roles grantor ON grantor.oid = acl.grantor
		WHERE n.nspname = 'public' AND c.relname = 'schema_migrations' AND c.relkind = 'r'
		ORDER BY 1, 3, 2`)
	if err != nil {
		return report, errors.New("GCP-03 could not observe migration history ACL grantors")
	}
	for rows.Next() {
		var item dbbootstrap.ACLObservation
		if err := rows.Scan(&item.Grantee, &item.Grantor, &item.Privilege, &item.Grantable); err != nil {
			rows.Close()
			return report, errors.New("GCP-03 could not decode migration history ACL grantors")
		}
		report.SchemaHistoryACL = append(report.SchemaHistoryACL, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, errors.New("GCP-03 could not finish migration history ACL observation")
	}
	rows.Close()
	if err := conn.QueryRow(ctx, `SELECT
		pg_catalog.pg_get_userbyid(c.relowner) = 'espacigo_scratch_migrator'
		AND has_table_privilege('postgres', c.oid, 'SELECT')
		AND NOT has_table_privilege('espacigo_runtime', c.oid, 'SELECT')
		AND NOT EXISTS (
			SELECT 1 FROM pg_catalog.aclexplode(coalesce(c.relacl, pg_catalog.acldefault('r', c.relowner))) acl
			WHERE acl.grantee = 0 AND acl.privilege_type = 'SELECT'
		)
		AND NOT EXISTS (
			SELECT 1 FROM pg_catalog.aclexplode(coalesce(c.relacl, pg_catalog.acldefault('r', c.relowner))) acl
			JOIN pg_catalog.pg_roles r ON r.oid = acl.grantee
			WHERE r.rolname = 'postgres' AND acl.privilege_type = 'SELECT' AND acl.is_grantable
		)
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'schema_migrations' AND c.relkind = 'r'`).Scan(&ownerACLQueryPass); err != nil {
		return report, errors.New("GCP-03 could not observe migration history ownership and ACL")
	}
	report.OwnerHistoryACL = ownerACLQueryPass && exactOwnerHistoryACL(report.SchemaHistoryACL)
	report.Passed = sessionOK && report.Observation.Passed() && report.MigrationHistory &&
		report.MigratorRetired && report.NoRoleMemberships && report.OwnerHistoryACL &&
		!report.MigratorCanTemp && !report.MigratorCanCreate && report.DatabaseOwner != "" &&
		report.PublicSchemaOwner != "" && report.HistoryOwner == "espacigo_scratch_migrator"
	return report, nil
}

func exactOwnerHistoryACL(items []dbbootstrap.ACLObservation) bool {
	foundPostgresSelect := false
	for _, item := range items {
		if item.Grantee == "espacigo_scratch_migrator" {
			continue
		}
		if item.Grantee == "postgres" && item.Grantor == "espacigo_scratch_migrator" &&
			item.Privilege == "SELECT" && !item.Grantable {
			if foundPostgresSelect {
				return false
			}
			foundPostgresSelect = true
			continue
		}
		return false
	}
	return foundPostgresSelect
}
