package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func main() {
	migrationDir := flag.String("dir", "db/migrations", "directory containing V<version>__<description>.sql files")
	planOnly := flag.Bool("plan-only", false, "print the read-only applied/pending migration inventory without executing SQL")
	applyRuntimeACL := flag.Bool("apply-runtime-acl", false, "apply the reviewed runtime ACL as administrator after migrations")
	recoverRuntimeACL := flag.Bool("recover-runtime-acl", false, "read back the exact F5 ACL contract first, then forward-reconcile only if needed")
	confirmRuntimeACL := flag.String("confirm-runtime-acl", "", "must exactly equal the isolated F5 target name")
	confirmF5AdminMigration := flag.String("confirm-f5-admin-migration", "", "allow the one-time versioned migration only on the isolated F5 target")
	flag.Parse()

	runtimeACLMode, err := validateRuntimeACLInvocation(*applyRuntimeACL, *recoverRuntimeACL, *planOnly, *confirmRuntimeACL, *confirmF5AdminMigration)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := validateRuntimeACLReviewGate(runtimeACLMode, runtimeACLPhaseReviewed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	settings, err := loadConnectionSettings(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
	defer cancel()
	if runtimeACLMode != runtimeACLInvocationNone {
		if err := validateRuntimeACLAdminIdentity(settings.databaseURL, settings.mode, settings.setRole); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if runtimeACLMode == runtimeACLInvocationApply {
			if err := applyRuntimeACLPhase(ctx, settings.databaseURL, *migrationDir); err != nil {
				fmt.Fprintf(os.Stderr, "runtime ACL phase failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("runtime ACL phase applied; complete pre- and post-commit readback passed")
			return
		}
		result, err := recoverRuntimeACLPhase(ctx, settings.databaseURL, *migrationDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "runtime ACL recovery failed: %v\n", err)
			os.Exit(1)
		}
		if result == runtimeACLRecoveryAlreadyValid {
			fmt.Println("runtime ACL readback already passed; no DDL replayed")
		} else {
			fmt.Println("runtime ACL forward recovery applied; complete pre- and post-commit readback passed")
		}
		return
	}
	if *planOnly {
		if *confirmF5AdminMigration != "" {
			fmt.Fprintln(os.Stderr, "-confirm-f5-admin-migration is valid only for migration execution")
			os.Exit(2)
		}
		var plan migrator.PlanResult
		if settings.mode == "cloudsql" {
			databaseURL, password, urlErr := settings.databaseURLAndPasswordFile()
			if urlErr != nil {
				fmt.Fprintln(os.Stderr, urlErr)
				os.Exit(2)
			}
			plan, err = migrator.PlanCloudSQL(ctx, databaseURL, password, *migrationDir, settings.instance, settings.privateIP, settings.setRole)
		} else {
			plan, err = migrator.Plan(ctx, settings.databaseURL, *migrationDir, settings.setRole)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "migration plan failed: %v\n", err)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			fmt.Fprintln(os.Stderr, "could not encode migration plan")
			os.Exit(1)
		}
		return
	}
	if *confirmF5AdminMigration == "" {
		err = validateMigrationExecutionIdentity(settings.databaseURL, settings.setRole)
	} else {
		err = validateF5AdminMigrationIdentity(settings.databaseURL, settings.mode, settings.setRole, *confirmF5AdminMigration)
		if err == nil {
			settings.databaseURL, err = withF5AdminMigrationMarker(settings.databaseURL, *confirmF5AdminMigration)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var result migrator.Result
	if settings.mode == "cloudsql" {
		databaseURL, password, urlErr := settings.databaseURLAndPasswordFile()
		if urlErr != nil {
			fmt.Fprintln(os.Stderr, urlErr)
			os.Exit(2)
		}
		result, err = migrator.RunCloudSQLWithRole(ctx, databaseURL, password, *migrationDir, settings.instance, settings.privateIP, settings.setRole)
	} else {
		result, err = migrator.RunWithRole(ctx, settings.databaseURL, *migrationDir, settings.setRole)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
	if len(result.Applied) == 0 {
		fmt.Println("sin cambios")
		return
	}
	fmt.Printf("applied migrations: %v\n", result.Applied)
}
