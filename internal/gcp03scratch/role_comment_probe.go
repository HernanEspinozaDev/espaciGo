package gcp03scratch

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	roleCommentProbeRole    = GADMRole
	roleCommentProbeComment = "NOLOGIN GADM local bootstrap group; temporary isolated login membership only."
)

// RoleCommentProbeResult contains only fixed target identity, role attributes,
// membership counts, and the outcome of the single reviewed COMMENT statement.
type RoleCommentProbeResult struct {
	Phase                   Phase
	ServerVersionNum        int
	CurrentRoleSuperuser    bool
	CurrentRoleCreateRole   bool
	ProbeRole               string
	ProbeRoleLogin          bool
	ProbeRoleSuperuser      bool
	ProbeRoleCreateDB       bool
	ProbeRoleCreateRole     bool
	ProbeRoleReplication    bool
	ProbeRoleBypassRLS      bool
	ProbeRoleInherit        bool
	IncomingMemberships     int
	OutgoingMemberships     int
	CreatorAdminMemberships int
	CommentOutcome          string
	SQLState                string
}

// RunRoleCommentProbe runs one of two fixed, non-migration phases against the
// exact GCP-03 scratch instance/database. It accepts no SQL override and never
// touches schema_migrations or any V1-V61 migration file.
func RunRoleCommentProbe(ctx context.Context, config Config, password string) (RoleCommentProbeResult, error) {
	if !IsRoleCommentProbePhase(config.Phase) {
		return RoleCommentProbeResult{}, errors.New("unsupported fixed GCP-03 role-comment probe phase")
	}
	result := RoleCommentProbeResult{Phase: config.Phase, ProbeRole: roleCommentProbeRole}
	err := withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		if err := verifyRoleCommentProbeTarget(ctx, conn, &result); err != nil {
			return err
		}
		if config.Phase == PhaseRoleCommentPrepare {
			var exists bool
			if err := conn.QueryRow(ctx, `SELECT EXISTS (
				SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_gadm_bootstrap'
			)`).Scan(&exists); err != nil {
				return errors.New("could not inspect the fixed GCP-03 probe role")
			}
			if exists {
				return errors.New("fixed GCP-03 probe role already exists; reconcile its state before retrying")
			}
			if _, err := conn.Exec(ctx, `CREATE ROLE espacigo_gadm_bootstrap
				NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`); err != nil {
				return errors.New("could not create the fixed NOLOGIN GCP-03 probe role")
			}
		} else {
			if err := verifyFixedRole(ctx, conn, &result); err != nil {
				return err
			}
			if result.IncomingMemberships+result.OutgoingMemberships != 0 {
				result.CommentOutcome = "not_run_membership_reconciliation_incomplete"
				return errors.New("fixed GCP-03 probe role still has memberships; COMMENT was not attempted")
			}
		}

		if config.Phase == PhaseRoleCommentPrepare {
			if err := verifyFixedRole(ctx, conn, &result); err != nil {
				return err
			}
		}
		if err := executeFixedRoleComment(ctx, conn); err != nil {
			outcome, sqlState, expectedPermissionDenial := classifyRoleCommentError(err)
			result.SQLState = sqlState
			result.CommentOutcome = outcome
			if expectedPermissionDenial {
				return nil
			}
			return errors.New("fixed GCP-03 role comment failed unexpectedly")
		}
		result.CommentOutcome = "succeeded"
		return nil
	})
	return result, err
}

func classifyRoleCommentError(err error) (outcome, sqlState string, expectedPermissionDenial bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "non_postgresql_error", "", false
	}
	if pgErr.Code == "42501" {
		return "permission_denied", pgErr.Code, true
	}
	return "unexpected_sql_error", pgErr.Code, false
}

func verifyRoleCommentProbeTarget(ctx context.Context, conn *pgx.Conn, result *RoleCommentProbeResult) error {
	var database, login string
	var applicationSchemaExists bool
	if err := conn.QueryRow(ctx, `SELECT current_database(), current_user,
		current_setting('server_version_num')::integer,
		(SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname = current_user),
		(SELECT rolcreaterole FROM pg_catalog.pg_roles WHERE rolname = current_user),
		(to_regclass('public.schema_migrations') IS NOT NULL OR EXISTS (
			SELECT 1
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			LEFT JOIN pg_catalog.pg_depend d ON d.classid = 'pg_catalog.pg_class'::regclass
				AND d.objid = c.oid AND d.deptype = 'e'
			WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
				AND d.objid IS NULL
		))`).
		Scan(&database, &login, &result.ServerVersionNum, &result.CurrentRoleSuperuser,
			&result.CurrentRoleCreateRole, &applicationSchemaExists); err != nil {
		return errors.New("could not inspect the fixed GCP-03 probe session")
	}
	if database != DatabaseName || login != MigratorLogin {
		return errors.New("fixed GCP-03 probe connected to an unexpected database login")
	}
	if result.ServerVersionNum < 160000 || result.CurrentRoleSuperuser || !result.CurrentRoleCreateRole {
		return errors.New("fixed GCP-03 probe requires PostgreSQL 16+, a non-superuser CREATEROLE login")
	}
	if applicationSchemaExists {
		return errors.New("fixed GCP-03 probe requires an empty application schema; no COMMENT was attempted")
	}
	return nil
}

func verifyFixedRole(ctx context.Context, conn *pgx.Conn, result *RoleCommentProbeResult) error {
	err := conn.QueryRow(ctx, `SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole,
		rolreplication, rolbypassrls, rolinherit
		FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_gadm_bootstrap'`).Scan(
		&result.ProbeRoleLogin, &result.ProbeRoleSuperuser, &result.ProbeRoleCreateDB,
		&result.ProbeRoleCreateRole, &result.ProbeRoleReplication, &result.ProbeRoleBypassRLS,
		&result.ProbeRoleInherit)
	if err != nil {
		return errors.New("fixed GCP-03 probe role is absent or could not be inspected")
	}
	if result.ProbeRoleLogin || result.ProbeRoleSuperuser || result.ProbeRoleCreateDB || result.ProbeRoleCreateRole ||
		result.ProbeRoleReplication || result.ProbeRoleBypassRLS || result.ProbeRoleInherit {
		return errors.New("fixed GCP-03 probe role is absent or has unexpected attributes")
	}
	err = conn.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE m.roleid = r.oid)::integer,
		count(*) FILTER (WHERE m.member = r.oid)::integer,
		count(*) FILTER (WHERE m.roleid = r.oid AND m.member = current_user::regrole AND m.admin_option)::integer
		FROM pg_catalog.pg_roles r
		LEFT JOIN pg_catalog.pg_auth_members m ON m.roleid = r.oid OR m.member = r.oid
		WHERE r.rolname = 'espacigo_gadm_bootstrap'
		GROUP BY r.oid`).Scan(&result.IncomingMemberships, &result.OutgoingMemberships, &result.CreatorAdminMemberships)
	if err != nil {
		return errors.New("could not inspect memberships of the fixed GCP-03 probe role")
	}
	return nil
}

func executeFixedRoleComment(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `COMMENT ON ROLE espacigo_gadm_bootstrap IS
		'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.'`)
	return err
}
