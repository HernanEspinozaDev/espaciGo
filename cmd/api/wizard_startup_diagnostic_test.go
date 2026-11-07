package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func safeRuntimeConfigGuard(err error) string {
	if err == nil {
		return "unknown"
	}
	// These are fixed messages from openPrivatePGPassfile; never emit the path.
	switch err.Error() {
	case "invalid local PostgreSQL passfile path":
		return "passfile-path-invalid"
	case "local PostgreSQL passfile must be inside a private directory":
		return "passfile-parent-required"
	case "could not open local PostgreSQL passfile root":
		return "passfile-root-unavailable"
	case "local PostgreSQL passfile ancestors must be real directories":
		return "passfile-ancestor-not-directory"
	case "could not inspect local PostgreSQL passfile directory":
		return "passfile-ancestor-stat-failed"
	case "local PostgreSQL passfile ancestors must not be group/world writable":
		return "passfile-ancestor-writable"
	case "local PostgreSQL passfile directory must be private and owned by the API user":
		return "passfile-parent-not-private-owned"
	case "local PostgreSQL passfile must be a regular file":
		return "passfile-not-openable-regular-file"
	case "local PostgreSQL passfile must be a private owned regular file":
		return "passfile-not-private-owned-regular-file"
	default:
		if strings.HasPrefix(err.Error(), "invalid CORS origin ") {
			return "cors-origin-invalid"
		}
		return "runtime-config-guard-unclassified"
	}
}

// Opt-in, read-only diagnosis for the isolated wizard API launch environment.
// It reports only allowlisted guard identifiers; error values may contain secrets.
func TestWizardStartupDiagnostic(t *testing.T) {
	if os.Getenv("ESPACIGO_WIZARD_STARTUP_DIAG") != "1" {
		t.Skip("set ESPACIGO_WIZARD_STARTUP_DIAG=1 to run read-only startup diagnostics")
	}

	profile, err := runtimeProfile(os.Getenv)
	if err != nil {
		t.Fatal("FAIL stage=runtime-profile")
	}
	t.Logf("PASS stage=runtime-profile profile=%s", profile)

	if _, err = localRepublicationEnabled(os.Getenv, profile); err != nil {
		t.Fatal("FAIL stage=local-republication-policy")
	}
	if _, err = localDocumentReviewEnabled(os.Getenv, profile); err != nil {
		t.Fatal("FAIL stage=local-document-review-policy")
	}
	if _, err = localPublicCatalogEnabled(os.Getenv, profile); err != nil {
		t.Fatal("FAIL stage=local-public-catalog-policy")
	}
	t.Log("PASS stage=local-feature-policies")

	passfilePath := strings.TrimSpace(os.Getenv("PGPASSFILE"))
	if passfilePath != "" {
		if err := validatePrivatePGPassfile(passfilePath); err != nil {
			t.Fatalf("FAIL stage=runtime-config guard=%s", safeRuntimeConfigGuard(err))
		}
	}

	cfg, err := loadRuntimeConfig(os.Getenv, profile)
	if err != nil {
		t.Fatalf("FAIL stage=runtime-config guard=%s", safeRuntimeConfigGuard(err))
	}
	defer cfg.close()
	t.Log("PASS stage=runtime-config")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	pool, closeConnector, err := newDatabasePool(ctx, cfg)
	if err != nil {
		t.Fatal("FAIL stage=database-pool-construction")
	}
	defer closeConnector()
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("FAIL stage=database-ping-or-configured-role")
	}
	t.Log("PASS stage=database-ping-or-configured-role")

	var database, sessionUser, currentUser string
	if err := pool.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil {
		t.Fatal("FAIL stage=read-only-database-identity-query")
	}
	// Report identifiers for confirming target and SET ROLE; these are not credentials.
	t.Logf("PASS stage=read-only-database-identity-query database=%s session_user=%s current_user=%s", database, sessionUser, currentUser)
	t.Log("LIMIT stage=after-database-identity-query: API handler initialization and listener were not run")
}
