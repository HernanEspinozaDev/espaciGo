package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/jackc/pgx/v5"
)

const (
	runID                     = "20261020-gcp06a"
	connectionName            = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
	databaseName              = "espacigo_gcp06_candidate_20261020-gcp06a"
	runtimeLogin              = "espacigo_api_login"
	runtimeCredentialMount    = "/var/run/secrets/gcp06-runtime/password"
	activationCredentialMount = "/var/run/secrets/gcp06-admin-bootstrap/password"
	ownerEmail                = "hernan.espinoza.dev@gmail.com"
)

var (
	loginPattern = regexp.MustCompile(`^gadm_activation_[a-f0-9]{12}$`)
)

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "GCP-06 candidate bootstrap failed:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	flags := flag.NewFlagSet("gcp06-admin-bootstrap", flag.ContinueOnError)
	confirmation := flags.String("confirm", "", "exact candidate owner confirmation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if err := validateTarget(
		getenv("GCP06_RUN_ID"), getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"), getenv("DATABASE_NAME"),
		getenv("GCP06_BOOTSTRAP_USER"), *confirmation,
	); err != nil {
		return err
	}
	runtimePassword, err := readCredential(runtimeCredentialMount)
	if err != nil {
		return errors.New("Secret Manager mounted runtime database credential unavailable")
	}
	defer clear(runtimePassword)
	activationPassword, err := readCredential(activationCredentialMount)
	if err != nil {
		return errors.New("Secret Manager mounted activation credential unavailable")
	}
	defer clear(activationPassword)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtimeConn, closeRuntimeConnector, err := cloudsql.Connect(ctx, databaseURL(runtimeLogin), string(runtimePassword), connectionName, true)
	if err != nil {
		return errors.New("private GCP-06 candidate runtime connection failed")
	}
	if err := assumeRuntimeRole(ctx, runtimeConn); err != nil {
		_ = runtimeConn.Close(context.Background())
		closeRuntimeConnector()
		return errors.New("candidate API login cannot assume the least-privilege runtime role")
	}
	accountID, err := verifiedOwnerAccountID(ctx, runtimeConn)
	_ = runtimeConn.Close(context.Background())
	closeRuntimeConnector()
	if err != nil {
		return errors.New("the exact owner account is absent, inactive, blocked, or not verified in the candidate")
	}
	activationConn, closeActivationConnector, err := cloudsql.Connect(ctx, databaseURL(getenv("GCP06_BOOTSTRAP_USER")), string(activationPassword), connectionName, true)
	if err != nil {
		return errors.New("private GCP-06 candidate activation connection failed")
	}
	defer closeActivationConnector()
	defer activationConn.Close(context.Background())
	if err := adminglobal.SetRestrictedBootstrapRole(ctx, activationConn); err != nil {
		return errors.New("candidate activation login does not satisfy the restricted bootstrap contract")
	}
	var actualDatabase string
	if err := activationConn.QueryRow(ctx, "SELECT current_database()").Scan(&actualDatabase); err != nil || actualDatabase != databaseName {
		return errors.New("connected database differs from the pinned GCP-06 candidate")
	}
	correlation, err := (credentials.Generator{}).ID()
	if err != nil {
		return errors.New("could not create bootstrap audit correlation")
	}
	if err := adminglobal.Bootstrap(ctx, activationConn, accountID, "bootstrap", correlation); err != nil {
		return errors.New("candidate bootstrap was rejected; inspect audit by correlation through the approved observer")
	}
	fmt.Println("GCP-06 candidate owner bootstrap completed")
	return nil
}

func validateTarget(targetRun, targetInstance, targetDatabase, login, confirmation string) error {
	if targetRun != runID || targetInstance != connectionName || targetDatabase != databaseName {
		return errors.New("target must be the pinned GCP-06 candidate run, instance, and database")
	}
	if !loginPattern.MatchString(login) {
		return errors.New("DATABASE_USER must be the one-time gadm_activation_<12 hex> login")
	}
	expected := fmt.Sprintf("GCP06 BOOTSTRAP %s %s", runID, ownerEmail)
	if confirmation != expected {
		return errors.New("exact GCP-06 owner confirmation phrase is required")
	}
	return nil
}

func databaseURL(login string) string {
	return (&url.URL{Scheme: "postgres", User: url.User(login), Host: "localhost:5432", Path: "/" + databaseName, RawQuery: "sslmode=disable"}).String()
}

func verifiedOwnerAccountID(ctx context.Context, conn *pgx.Conn) (string, error) {
	if conn == nil {
		return "", errors.New("candidate runtime connection is required")
	}
	var accountID string
	err := conn.QueryRow(ctx, `SELECT u.id::text
		FROM public.usuario u
		WHERE u.correo_normalizado=$1 AND u.estado='activo'
		  AND (u.bloqueado_hasta IS NULL OR u.bloqueado_hasta<=now())
		  AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)`, identity.NormalizeEmail(ownerEmail)).Scan(&accountID)
	if err != nil || accountID == "" {
		return "", errors.New("verified owner account lookup failed")
	}
	return accountID, nil
}

func assumeRuntimeRole(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return errors.New("candidate runtime connection is required")
	}
	if _, err := conn.Exec(ctx, "SET ROLE espacigo_runtime"); err != nil {
		return errors.New("runtime role is unavailable")
	}
	var sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT session_user,current_user").Scan(&sessionUser, &currentUser); err != nil {
		return errors.New("runtime role identity could not be verified")
	}
	if sessionUser != runtimeLogin || currentUser != "espacigo_runtime" {
		return errors.New("runtime role identity differs from the approved contract")
	}
	return nil
}

func readCredential(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0400 || info.Size() == 0 || info.Size() > 4096 {
		return nil, errors.New("credential must be a bounded mode-0400 Secret Manager mount")
	}
	value, err := os.ReadFile(path)
	if err != nil || len(value) == 0 || len(value) > 4096 {
		clear(value)
		return nil, errors.New("credential mount is empty or invalid")
	}
	return value, nil
}
