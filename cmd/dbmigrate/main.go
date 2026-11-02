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
	confirmRuntimeACL := flag.String("confirm-runtime-acl", "", "must exactly equal the isolated F5 target name")
	flag.Parse()

	settings, err := loadConnectionSettings(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
	defer cancel()
	if *applyRuntimeACL {
		if *planOnly || *confirmRuntimeACL != runtimeACLConfirmation {
			fmt.Fprintln(os.Stderr, "runtime ACL phase requires a separate invocation and exact target confirmation")
			os.Exit(2)
		}
		if err := validateRuntimeACLAdminIdentity(settings.databaseURL, settings.mode, settings.setRole); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := applyRuntimeACLPhase(ctx, settings.databaseURL, *migrationDir); err != nil {
			fmt.Fprintf(os.Stderr, "runtime ACL phase failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("runtime ACL phase applied; core readback passed")
		return
	}
	if *confirmRuntimeACL != "" {
		fmt.Fprintln(os.Stderr, "-confirm-runtime-acl is valid only with -apply-runtime-acl")
		os.Exit(2)
	}
	if *planOnly {
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
	if err := validateMigrationExecutionIdentity(settings.databaseURL, settings.setRole); err != nil {
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
