package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	osuser "os/user"
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
	nativeAdminPreV81 := flag.Bool("native-admin-pre-v81", false, "apply native PostgreSQL V77..V80 as the local postgres OS account")
	confirmNativeAdminPreV81 := flag.String("confirm-native-admin-pre-v81", "", "must exactly equal espacigo_dms_stage_20261020")
	nativeRoleTransition := flag.Bool("native-role-transition-v80", false, "apply the reviewed V80 native role transition on the exact local PostgreSQL target")
	confirmNativeRoleTransition := flag.String("confirm-native-role-transition-v80", "", "must exactly equal espacigo_dms_stage_20261020")
	nativeMigratorV81 := flag.Bool("native-migrator-v81", false, "apply V81 through the dedicated native migrator login and private passfile")
	confirmNativeMigratorV81 := flag.String("confirm-native-migrator-v81", "", "must exactly equal espacigo_dms_stage_20261020")
	nativeKYCV86 := flag.Bool("native-kyc-v86", false, "apply only the reviewed native-local V86 migration")
	confirmNativeKYCV86 := flag.String("confirm-native-kyc-v86", "", "must exactly equal espacigo_dms_stage_20261020")
	nativeKYCSourceFingerprint := flag.String("native-kyc-v86-source-sha256", "", "reviewed SHA-256 inventory fingerprint for V1..V86")
	nativeTailVersion := flag.Int("native-kyc-tail-version", 0, "apply exactly one reviewed native-local migration: 87 through 96")
	confirmNativeTail := flag.String("confirm-native-kyc-tail", "", "must exactly equal espacigo_dms_stage_20261020")
	nativeTailFingerprint := flag.String("native-kyc-tail-source-sha256", "", "pinned reviewed source fingerprint for the selected native KYC prefix")
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
	var nativeModeSelected, recoveryModeSelected bool
	flag.Visit(func(f *flag.Flag) {
		nativeModeSelected = nativeModeSelected || strings.Contains(f.Name, "native-")
		recoveryModeSelected = recoveryModeSelected || f.Name == "recovery-inspection-only" || strings.HasPrefix(f.Name, "v84-authority-") || f.Name == "confirm-v84-authority"
	})
	if nativeModeSelected && recoveryModeSelected {
		fmt.Fprintln(os.Stderr, "native and cloud recovery authority modes are mutually exclusive")
		os.Exit(2)
	}
	var selectedNativeFlags []string
	var tailMentioned, v86Mentioned bool
	flag.Visit(func(f *flag.Flag) {
		selectedNativeFlags = append(selectedNativeFlags, f.Name)
		switch f.Name {
		case "native-kyc-tail-version", "confirm-native-kyc-tail", "native-kyc-tail-source-sha256":
			tailMentioned = true
		case "native-kyc-v86", "confirm-native-kyc-v86", "native-kyc-v86-source-sha256":
			v86Mentioned = true
		}
	})
	if tailMentioned {
		if err := validateNativeKYCTailMode(*nativeTailVersion, *confirmNativeTail, *nativeTailFingerprint, selectedNativeFlags); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := runNativeKYCTail(*migrationDir, *nativeTailVersion, *nativeTailFingerprint, os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
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
	if v86Mentioned {
		var selected []string
		flag.Visit(func(f *flag.Flag) { selected = append(selected, f.Name) })
		if err := validateNativeV86Mode(*nativeKYCV86, *confirmNativeKYCV86, *nativeKYCSourceFingerprint, selected); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := runNativeKYCV86(*migrationDir, os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *nativeAdminPreV81 {
		if *nativeRoleTransition || *nativeMigratorV81 || *inventoryOnly || *schemaOnly || *planOnly || *releaseMode || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmCloudOwner != "" || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *wizardAdminVersion != 0 || *confirmWizardAdmin != "" || *releaseSHA != "" || *releaseInventory != "" || *releaseExpected != 0 || *confirmNativeRoleTransition != "" || *confirmNativeMigratorV81 != "" {
			fmt.Fprintln(os.Stderr, "-native-admin-pre-v81 cannot be combined with another migration mode")
			os.Exit(2)
		}
	} else if *confirmNativeAdminPreV81 != "" {
		fmt.Fprintln(os.Stderr, "-confirm-native-admin-pre-v81 requires -native-admin-pre-v81")
		os.Exit(2)
	}
	if *nativeRoleTransition {
		if *nativeAdminPreV81 || *nativeMigratorV81 || *inventoryOnly || *schemaOnly || *planOnly || *releaseMode || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmCloudOwner != "" || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *wizardAdminVersion != 0 || *confirmWizardAdmin != "" || *releaseSHA != "" || *releaseInventory != "" || *releaseExpected != 0 || *confirmNativeAdminPreV81 != "" || *confirmNativeMigratorV81 != "" {
			fmt.Fprintln(os.Stderr, "-native-role-transition-v80 cannot be combined with another migration mode")
			os.Exit(2)
		}
	} else if *confirmNativeRoleTransition != "" {
		fmt.Fprintln(os.Stderr, "-confirm-native-role-transition-v80 requires -native-role-transition-v80")
		os.Exit(2)
	}
	if *nativeMigratorV81 {
		if *nativeAdminPreV81 || *nativeRoleTransition || *inventoryOnly || *schemaOnly || *planOnly || *releaseMode || *applyRuntimeACL || *recoverRuntimeACL || *cloudOwner || *confirmCloudOwner != "" || *confirmRuntimeACL != "" || *confirmF5AdminMigration != "" || *wizardAdminVersion != 0 || *confirmWizardAdmin != "" || *releaseSHA != "" || *releaseInventory != "" || *releaseExpected != 0 || *confirmNativeAdminPreV81 != "" || *confirmNativeRoleTransition != "" {
			fmt.Fprintln(os.Stderr, "-native-migrator-v81 cannot be combined with another migration mode")
			os.Exit(2)
		}
	} else if *confirmNativeMigratorV81 != "" {
		fmt.Fprintln(os.Stderr, "-confirm-native-migrator-v81 requires -native-migrator-v81")
		os.Exit(2)
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
	if *nativeMigratorV81 {
		currentUser, userErr := osuser.Current()
		if userErr != nil {
			fmt.Fprintln(os.Stderr, "could not verify private passfile owner identity")
			os.Exit(2)
		}
		if err := validateNativeMigratorV81Invocation(*confirmNativeMigratorV81, settings, os.Getenv, currentUser.Username, nativeMigratorPassfilePath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
		defer cancel()
		plan, planErr := migrator.Plan(ctx, settings.databaseURL, *migrationDir, settings.setRole)
		if planErr == nil {
			planErr = validateNativeV81Plan(plan)
		}
		if planErr != nil {
			fmt.Fprintf(os.Stderr, "native V81 read-only gate failed: %v\n", planErr)
			os.Exit(1)
		}
		snapshot, cleanup, snapshotErr := makeNativeV81Snapshot(*migrationDir)
		if snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "native V81 source staging failed: %v\n", snapshotErr)
			os.Exit(1)
		}
		defer cleanup()
		result, runErr := migrator.RunWithRoleInRange(ctx, settings.databaseURL, snapshot, settings.setRole, 80, 81)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "native V81 migration failed; read back ledger before retry: %v\n", runErr)
			os.Exit(1)
		}
		postPlan, postErr := migrator.Plan(ctx, settings.databaseURL, *migrationDir, settings.setRole)
		if postErr == nil {
			postErr = validateNativeV81PostPlan(postPlan)
		}
		if postErr != nil {
			fmt.Fprintf(os.Stderr, "native V81 postcheck failed; read back the ledger before retry: %v\n", postErr)
			os.Exit(1)
		}
		fmt.Printf("native V81 applied migrations: %v; postcheck confirmed V1..V81 and pending V82..V85\n", result.Applied)
		return
	}
	if *nativeRoleTransition {
		currentUser, userErr := osuser.Current()
		if userErr != nil {
			fmt.Fprintln(os.Stderr, "could not verify local postgres OS identity")
			os.Exit(2)
		}
		if err := validateNativeAdminRangeInvocation(*confirmNativeRoleTransition, settings, os.Getenv, currentUser.Username); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
		defer cancel()
		plan, planErr := migrator.Plan(ctx, settings.databaseURL, *migrationDir, "")
		if planErr == nil {
			planErr = validateNativeV80RoleTransitionPlan(plan)
		}
		if planErr != nil {
			fmt.Fprintf(os.Stderr, "native V80 role-transition read-only gate failed: %v\n", planErr)
			os.Exit(1)
		}
		if err := migrator.ApplyNativeV80RoleTransition(ctx, settings.databaseURL, *migrationDir, nativeAdminConfirmation, 16388); err != nil {
			fmt.Fprintf(os.Stderr, "native V80 role transition failed; read back roles and memberships before retry: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("native V80 role transition applied; runtime OID preserved and V81 role contract provisioned")
		return
	}
	if *nativeAdminPreV81 {
		currentUser, userErr := osuser.Current()
		if userErr != nil {
			fmt.Fprintln(os.Stderr, "could not verify local postgres OS identity")
			os.Exit(2)
		}
		if err := validateNativeAdminRangeInvocation(*confirmNativeAdminPreV81, settings, os.Getenv, currentUser.Username); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
		defer cancel()
		plan, planErr := migrator.Plan(ctx, settings.databaseURL, *migrationDir, "")
		if planErr == nil {
			planErr = validateNativePreV81Plan(plan)
		}
		if planErr != nil {
			fmt.Fprintf(os.Stderr, "native pre-V81 read-only gate failed: %v\n", planErr)
			os.Exit(1)
		}
		snapshot, cleanup, snapshotErr := makeNativePreV81Snapshot(*migrationDir)
		if snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "native pre-V81 source staging failed: %v\n", snapshotErr)
			os.Exit(1)
		}
		defer cleanup()
		result, runErr := migrator.RunWithRoleInRange(ctx, settings.databaseURL, snapshot, "", 76, 80)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "native pre-V81 migration failed; read back ledger before retry: %v\n", runErr)
			os.Exit(1)
		}
		fmt.Printf("native pre-V81 applied migrations: %v\n", result.Applied)
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
