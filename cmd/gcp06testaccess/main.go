package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06testaccess"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "GCP-06 test-account operation failed:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	flags := flag.NewFlagSet("gcp06-test-access", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	runID, instance, database := getenv("GCP06_RUN_ID"), getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"), getenv("DATABASE_NAME")
	login := getenv("GCP06_TEST_ACCESS_USER")
	if err := gcp06testaccess.ValidateTarget(runID, instance, database, login); err != nil {
		return err
	}
	mode := getenv("GCP06_TEST_ACCESS_MODE")
	if mode != "preview" && mode != "apply" {
		return errors.New("GCP06_TEST_ACCESS_MODE must be preview or apply")
	}
	if err := cloudsql.ValidatePGEnvironment(getenv); err != nil {
		return errors.New("PostgreSQL environment overrides are not allowed")
	}
	dbPassword, err := gcp06testaccess.ReadSecret(gcp06testaccess.SQLPasswordFile)
	if err != nil {
		return errors.New("candidate test-access SQL secret is unavailable")
	}
	defer func() { b := []byte(dbPassword); clear(b) }()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	conn, closeConnector, err := cloudsql.Connect(ctx, databaseURL(login), string(dbPassword), instance, true)
	if err != nil {
		return errors.New("private GCP-06 candidate connection failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	if err := gcp06testaccess.ValidateRestrictedConnection(ctx, conn, login); err != nil {
		return err
	}
	if mode == "preview" {
		result, err := gcp06testaccess.Preview(ctx, conn)
		if err != nil {
			return err
		}
		fmt.Printf("GCP-06 candidate reset preview: count=%d target_set_sha256=%s\n", result.TargetCount, result.TargetSHA256)
		return nil
	}
	count, err := strconv.Atoi(getenv("GCP06_TEST_ACCESS_EXPECTED_COUNT"))
	if err != nil || count < 1 || count > 10000 {
		return errors.New("reviewed target count is invalid")
	}
	digest := getenv("GCP06_TEST_ACCESS_EXPECTED_SHA256")
	if !sha256Pattern.MatchString(digest) {
		return errors.New("reviewed target digest is invalid")
	}
	operationID := getenv("GCP06_TEST_ACCESS_OPERATION_ID")
	expectedConfirmation := fmt.Sprintf("GCP06 APPLY %s %d %s %s", gcp06testaccess.RunID, count, digest, operationID)
	if getenv("GCP06_TEST_ACCESS_CONFIRMATION") != expectedConfirmation {
		return errors.New("exact reviewed candidate reset confirmation is required")
	}
	appPassword, err := gcp06testaccess.ReadSecret(gcp06testaccess.AppPasswordFile)
	if err != nil {
		return errors.New("candidate shared test-account Secret Manager value is unavailable")
	}
	defer func() { b := []byte(appPassword); clear(b) }()
	result, err := gcp06testaccess.Apply(ctx, conn, operationID, count, digest, identity.Secret(appPassword))
	if err != nil {
		return err
	}
	fmt.Printf("GCP-06 candidate reset committed: count=%d target_set_sha256=%s\n", result.TargetCount, result.TargetSHA256)
	return nil
}

func databaseURL(login string) string {
	return (&url.URL{Scheme: "postgres", User: url.User(login), Host: "localhost:5432", Path: "/" + gcp06testaccess.Database, RawQuery: "sslmode=disable"}).String()
}
