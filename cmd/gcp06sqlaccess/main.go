// Command gcp06-sql-access provisions or removes one candidate-scoped SQL
// inspector login and its Secret Manager credential. It is never run by CI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06sqlaccess"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "GCP-06 SQL access operation failed:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: gcp06-sql-access prepare|cleanup [exact flags]")
	}
	action := args[0]
	flags := flag.NewFlagSet("gcp06-sql-access "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	apply := flags.Bool("apply", false, "perform the exact reviewed candidate operation")
	confirm := flags.String("confirm-target", "", "exact run/login/version confirmation")
	login := flags.String("login", "", "one-run GCP-06 inspector login to clean up")
	version := flags.String("secret-version", "", "exact numeric Secret Manager version to clean up")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*apply {
		return errors.New("operation requires --apply, exact target confirmation, and no positional arguments")
	}
	var cleanupVersion string
	switch action {
	case "prepare":
		if *confirm != gcp06sqlaccess.PrepareConfirm || *login != "" || *version != "" {
			return errors.New("prepare requires only the exact GCP-06 candidate confirmation")
		}
	case "cleanup":
		cleanupVersion = gcp06sqlaccess.CanonicalVersion(*version)
		if cleanupVersion == "" || !gcp06sqlaccess.IsInspectorLogin(*login) || *confirm != gcp06sqlaccess.CleanupConfirmation(*login, cleanupVersion) {
			return errors.New("cleanup requires the exact candidate login, numeric secret version, and confirmation")
		}
	default:
		return errors.New("supported operations are prepare and cleanup")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	sql, secrets, err := gcp06sqlaccess.NewGoogleAPIClients(ctx)
	if err != nil {
		return err
	}
	provisioner := gcp06sqlaccess.Provisioner{SQL: sql, Secrets: secrets}
	switch action {
	case "prepare":
		result, err := provisioner.Prepare(ctx, *confirm)
		if err != nil {
			if result.Login != "" {
				fmt.Fprintf(stderr, "reconciliation_required=true login=%s secret=%s", result.Login, gcp06sqlaccess.SecretParent)
				if number := gcp06sqlaccess.VersionNumber(result.SecretVersion); number != "" {
					fmt.Fprintf(stderr, " secret_version=%s", number)
				}
				fmt.Fprintln(stderr)
			}
			return err
		}
		fmt.Fprintf(stdout, "GCP-06 inspector credential prepared: login=%s secret_version=%s operation_id=%s\n", result.Login, gcp06sqlaccess.VersionNumber(result.SecretVersion), result.Operation)
		return nil
	case "cleanup":
		result, err := provisioner.Cleanup(ctx, *login, cleanupVersion, *confirm)
		if err != nil {
			if result.Reconcile {
				fmt.Fprintf(stderr, "reconciliation_required=true login=%s secret_version=%s\n", result.Login, gcp06sqlaccess.VersionNumber(result.SecretVersion))
			}
			return err
		}
		fmt.Fprintf(stdout, "GCP-06 inspector cleanup verified: login_deleted=%t secret_version_destroyed=%t\n", result.UserDeleted, result.SecretDeleted)
		return nil
	}
	return nil
}
