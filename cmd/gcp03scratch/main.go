package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03scratch"
)

const migrationDirectory = "/app/db/migrations"

func main() {
	config, err := gcp03scratch.LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	password, err := gcp03scratch.ReadPassword(config.PasswordFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	if config.Phase == gcp03scratch.PhaseObserve {
		report, err := gcp03scratch.ObserveScratch(ctx, config, password)
		if err != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 fixed compatibility observer failed")
			os.Exit(1)
		}
		if err := dbbootstrap.WriteObserverNDJSON(os.Stdout, report); err != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 fixed compatibility observer report could not be encoded")
			os.Exit(1)
		}
		if !report.Passed() {
			fmt.Fprintln(os.Stderr, "GCP-03 fixed compatibility observer found failed checks")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "GCP-03 scratch phase observe completed")
		return
	}
	if config.Phase == gcp03scratch.PhaseVerifyRetired {
		report, err := gcp03scratch.ObserveRetiredPhase(ctx, config, password, migrationDirectory)
		if err != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 fixed retirement observer could not complete its read-only collection")
			os.Exit(1)
		}
		if err := dbbootstrap.WriteIntegrityNDJSON(os.Stdout, report, report.Passed); err != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 retirement observer report could not be encoded")
			os.Exit(1)
		}
		fmt.Printf("GCP03_RETIRED_VERIFY migration_history_v1_v61=%t migrator_retired=%t migrator_no_memberships=%t migrator_can_temp=%t migrator_can_create_public=%t database_owner=%s public_schema_owner=%s schema_migrations_owner=%s owner_history_acl=%t passed=%t\n",
			report.MigrationHistory, report.MigratorRetired, report.NoRoleMemberships, report.MigratorCanTemp,
			report.MigratorCanCreate, report.DatabaseOwner, report.PublicSchemaOwner, report.HistoryOwner,
			report.OwnerHistoryACL, report.Passed)
		if !report.Passed {
			fmt.Fprintln(os.Stderr, "GCP-03 fixed retirement observer found failed checks")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "GCP-03 scratch phase verify_retired completed")
		return
	}
	if gcp03scratch.IsRoleCommentProbePhase(config.Phase) {
		result, probeErr := gcp03scratch.RunRoleCommentProbe(ctx, config, password)
		outcome := result.CommentOutcome
		if outcome == "" {
			outcome = "not_attempted"
		}
		fmt.Printf("GCP03_ROLE_COMMENT_PROBE phase=%s server_version_num=%d current_role_superuser=%t current_role_createrole=%t probe_role=%s probe_role_login=%t probe_role_superuser=%t probe_role_createdb=%t probe_role_createrole=%t probe_role_replication=%t probe_role_bypassrls=%t probe_role_inherit=%t incoming_memberships=%d outgoing_memberships=%d creator_admin_memberships=%d comment_outcome=%s sqlstate=%s\n",
			result.Phase, result.ServerVersionNum, result.CurrentRoleSuperuser, result.CurrentRoleCreateRole,
			result.ProbeRole, result.ProbeRoleLogin, result.ProbeRoleSuperuser, result.ProbeRoleCreateDB,
			result.ProbeRoleCreateRole, result.ProbeRoleReplication, result.ProbeRoleBypassRLS, result.ProbeRoleInherit,
			result.IncomingMemberships, result.OutgoingMemberships, result.CreatorAdminMemberships,
			outcome, result.SQLState)
		if probeErr != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 fixed role-comment probe stopped before successful completion")
			os.Exit(1)
		}
		fmt.Printf("GCP-03 scratch phase %s completed\n", config.Phase)
		return
	}
	if err := gcp03scratch.RunPhase(ctx, config, password, migrationDirectory); err != nil {
		fmt.Fprintf(os.Stderr, "GCP-03 scratch phase %s failed: %v\n", config.Phase, err)
		os.Exit(1)
	}
	if config.Phase == gcp03scratch.PhaseBootstrap {
		versions, err := gcp03scratch.ReadScratchExtensionVersions(ctx, config, password)
		if err != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 scratch extension version verification failed")
			os.Exit(1)
		}
		fmt.Printf("GCP-03 scratch extensions: btree_gist=%s postgis=%s unaccent=%s\n", versions["btree_gist"], versions["postgis"], versions["unaccent"])
	}
	fmt.Printf("GCP-03 scratch phase %s completed\n", config.Phase)
}
