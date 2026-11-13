package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func main() {
	migrationDir := flag.String("dir", "db/migrations", "directory containing V<version>__<description>.sql files")
	inventoryOnly := flag.Bool("inventory-only", false, "print the offline migration inventory without reading connection settings or contacting a database")
	schemaOnly := flag.Bool("schema-evidence-only", false, "print the packaged IMG-01 catalog evidence in a read-only transaction")
	planOnly := flag.Bool("plan-only", false, "print the read-only applied/pending migration inventory without executing SQL")
	releaseMode := flag.Bool("release", false, "apply a pinned private Cloud SQL release and print its checked result")
	releaseSHA := flag.String("release-source-sha", "", "full source SHA embedded in the release image")
	releaseInventory := flag.String("release-inventory-sha256", "", "reviewed migration inventory fingerprint")
	releaseExpected := flag.Int("release-expected-applied", 0, "exact reviewed ledger prefix before this execution")
	applyRuntimeACL := flag.Bool("apply-runtime-acl", false, "apply the reviewed runtime ACL as administrator after migrations")
	recoverRuntimeACL := flag.Bool("recover-runtime-acl", false, "read back the exact F5 ACL contract first, then forward-reconcile only if needed")
	confirmRuntimeACL := flag.String("confirm-runtime-acl", "", "must exactly equal the isolated F5 target name")
	confirmF5AdminMigration := flag.String("confirm-f5-admin-migration", "", "allow the one-time versioned migration only on the isolated F5 target")
	cloudOwner := flag.Bool("cloud-owner-migration", false, "run confirmed CloudSQL staging repair V76..V80")
	confirmCloudOwner := flag.String("confirm-cloud-owner-pre81", "", "exact CloudSQL staging confirmation")
	wizardAdminVersion := flag.Int64("wizard-admin-version", 0, "apply one authorized wizard migration version (60, 64, or 66) in the isolated ensayo")
	confirmWizardAdmin := flag.String("confirm-wizard-admin", "", "must exactly equal espacigo_wizard_20261106")
	recoveryInspectionOnly := flag.Bool("recovery-inspection-only", false, "print bounded read-only recovery inspection report")
	v84AuthorityMode := flag.String("v84-authority-mode", "", "bounded V84 authority operation: inspect, prepare, or finalize")
	v84AuthorityBaseline := flag.String("v84-authority-baseline", "", "expected V84 authority baseline SHA-256 for finalize")
	confirmV84Authority := flag.String("confirm-v84-authority", "", "must exactly equal espacigo_dms_stage_20261020")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "positional arguments are not supported")
		os.Exit(2)
	}
	var authorityFlagSeen bool
	var authorityConflict string
	flag.Visit(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "v84-authority-") || f.Name == "confirm-v84-authority" {
			authorityFlagSeen = true
		}
		if *v84AuthorityMode != "" && f.Name != "dir" && f.Name != "v84-authority-mode" && f.Name != "v84-authority-baseline" && f.Name != "confirm-v84-authority" {
			authorityConflict = f.Name
		}
	})
	if (authorityFlagSeen && *v84AuthorityMode == "") || authorityConflict != "" {
		fmt.Fprintln(os.Stderr, "V84 authority options require their explicit exclusive mode")
		os.Exit(2)
	}
	if *recoveryInspectionOnly {
		if *inventoryOnly || *schemaOnly || *planOnly || *releaseMode || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *confirmCloudOwner != "" || *releaseSHA != "" || *releaseInventory != "" || *releaseExpected != 0 || *wizardAdminVersion != 0 || *confirmWizardAdmin != "" {
			fmt.Fprintln(os.Stderr, "-recovery-inspection-only cannot combine with offline, schema, plan, apply, or administrative modes")
			os.Exit(2)
		}
	}
	if *v84AuthorityMode != "" {
		if *inventoryOnly || *schemaOnly || *planOnly || *releaseMode || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *recoveryInspectionOnly || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *confirmCloudOwner != "" || *releaseSHA != "" || *releaseInventory != "" || *releaseExpected != 0 || *wizardAdminVersion != 0 || *confirmWizardAdmin != "" || *confirmV84Authority != v84AuthorityDatabase {
			fmt.Fprintln(os.Stderr, "V84 authority mode cannot combine with other modes or administrative options")
			os.Exit(2)
		}
		if err := validateV84AuthorityFlags(*v84AuthorityMode, *v84AuthorityBaseline, *confirmV84Authority, releaseSourceSHA); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		settings, err := loadConnectionSettings(os.Getenv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := validateV84AuthorityInvocation(*v84AuthorityMode, *v84AuthorityBaseline, *confirmV84Authority, settings, releaseSourceSHA, os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
		defer cancel()
		if err := runV84Authority(ctx, os.Stdout, settings, *migrationDir, *v84AuthorityMode, *v84AuthorityBaseline, os.Getenv); err != nil {
			fmt.Fprintf(os.Stderr, "V84 authority failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := validateWizardModeConflicts(*wizardAdminVersion, *confirmWizardAdmin, *inventoryOnly, *schemaOnly, *releaseMode, *releaseSHA, *releaseInventory, *releaseExpected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *schemaOnly && (*inventoryOnly || *planOnly || *releaseMode || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *confirmCloudOwner != "" || *releaseSHA != "" || *releaseInventory != "" || *releaseExpected != 0 || flag.NArg() != 0) {
		fmt.Fprintln(os.Stderr, "-schema-evidence-only cannot combine with other modes or release options")
		os.Exit(2)
	}
	if err := validateReleaseInvocation(*releaseMode, *inventoryOnly || *planOnly || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *confirmCloudOwner != "" || flag.NArg() != 0, *releaseSHA, *releaseInventory, *releaseExpected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *inventoryOnly {
		if err := printOfflineInventory(os.Stdout, *migrationDir, *planOnly || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *confirmCloudOwner != "" || flag.NArg() != 0); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	if err := validateWizardAdminInvocation(*wizardAdminVersion, *confirmWizardAdmin, *planOnly, *applyRuntimeACL, *recoverRuntimeACL, *confirmRuntimeACL, *confirmF5AdminMigration, *cloudOwner, *confirmCloudOwner); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
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
	if *recoveryInspectionOnly {
		if err := validateRecoveryInspectionInvocation(true, *inventoryOnly, *schemaOnly, *planOnly, *releaseMode, *applyRuntimeACL, *recoverRuntimeACL, *cloudOwner, settings, os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
		defer cancel()
		if err := runRecoveryInspection(ctx, os.Stdout, settings, *migrationDir, os.Getenv); err != nil {
			fmt.Fprintf(os.Stderr, "recovery inspection failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *wizardAdminVersion != 0 {
		if err := validateWizardAdminIdentity(settings.databaseURL, settings.mode, settings.setRole, os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		expected, err := reviewedWizardAdminInventory(*migrationDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
		defer cancel()
		result, err := migrator.RunWizardAdministrativeMigration(ctx, settings.databaseURL, *migrationDir, *wizardAdminVersion, expected)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wizard administrative migration failed (do not retry until commit outcome is read back): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wizard administrative migration applied V%06d\n", result.Applied[0])
		return
	}
	if err := validateCloudOwnerInvocation(*cloudOwner, *planOnly, *applyRuntimeACL, *recoverRuntimeACL, *confirmCloudOwner, settings, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
	defer cancel()
	if *schemaOnly {
		if err := printSchemaEvidence(ctx, os.Stdout, settings, *migrationDir); err != nil {
			fmt.Fprintf(os.Stderr, "schema evidence failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *releaseMode {
		if err := runCloudRelease(ctx, os.Stdout, settings, *migrationDir, *releaseSHA, *releaseInventory, *releaseExpected); err != nil {
			fmt.Fprintf(os.Stderr, "release failed; inspect ledger before retry: %v\n", err)
			os.Exit(1)
		}
		return
	}
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
	if *cloudOwner {
		if *confirmF5AdminMigration != "" {
			fmt.Fprintln(os.Stderr, "cloud owner mode cannot combine with F5 migration")
			os.Exit(2)
		}
		dir, cleanup, e := makeCloudOwnerSnapshot(*migrationDir)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(2)
		}
		defer cleanup()
		plan, e := migrator.Plan(ctx, settings.databaseURL, dir, "")
		if e == nil {
			e = validateCloudOwnerPending(plan)
		}
		if e != nil {
			fmt.Fprintf(os.Stderr, "cloud owner preflight failed: %v\n", e)
			os.Exit(1)
		}
		result, e := migrator.RunWithRoleInRange(ctx, settings.databaseURL, dir, "", 75, 80)
		if e != nil {
			fmt.Fprintf(os.Stderr, "cloud owner migration failed: %v\n", e)
			os.Exit(1)
		}
		fmt.Printf("applied migrations: %v\n", result.Applied)
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
