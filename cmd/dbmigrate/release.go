package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

// Set only at image build time. An unlabelled/local binary cannot apply a
// cloud release by accepting an arbitrary source SHA at runtime.
var releaseSourceSHA string

func validateReleaseInvocation(enabled, conflicting bool, sha, fingerprint string, expected int) error {
	if !enabled {
		if sha != "" || fingerprint != "" || expected != 0 {
			return errors.New("release options require -release")
		}
		return nil
	}
	if conflicting || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(sha) || sha != releaseSourceSHA || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(fingerprint) || expected < 83 {
		return errors.New("release requires the embedded source SHA, exact inventory fingerprint, reviewed prefix at least V83 and no other modes")
	}
	return nil
}

func runCloudRelease(ctx context.Context, output io.Writer, settings connectionSettings, directory, sha, fingerprint string, expected int) error {
	if err := validateCloudReleaseTarget(settings); err != nil {
		return err
	}
	query, err := os.ReadFile(filepath.Join(directory, "..", "query", "img01_schema_release_evidence.sql"))
	if err != nil {
		return errors.New("packaged schema evidence query missing")
	}
	config, _ := pgx.ParseConfig(settings.databaseURL)
	databaseURL, password, err := settings.databaseURLAndPasswordFile()
	if err != nil {
		return err
	}
	result, err := migrator.RunReleaseCloudSQL(ctx, databaseURL, password, directory, settings.instance, settings.privateIP, settings.setRole, migrator.ReleaseContract{
		SourceSHA: sha, InventorySHA256: fingerprint, ExpectedApplied: expected, Database: config.Database,
	}, query)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func validateCloudReleaseTarget(settings connectionSettings) error {
	if settings.mode != "cloudsql" || !settings.privateIP || settings.instance != "espacigo-testprod:southamerica-west1:espacigo-testprod-db" {
		return errors.New("release requires the reviewed private Cloud SQL target")
	}
	if err := validateMigrationExecutionIdentity(settings.databaseURL, settings.setRole); err != nil {
		return err
	}
	config, err := pgx.ParseConfig(settings.databaseURL)
	if err != nil || config.Database != "espacigo_dms_stage_20261020" {
		return errors.New("release database differs from reviewed destination")
	}
	return nil
}

func printSchemaEvidence(ctx context.Context, output io.Writer, settings connectionSettings, directory string) error {
	if err := validateCloudReleaseTarget(settings); err != nil {
		return err
	}
	query, err := os.ReadFile(filepath.Join(directory, "..", "query", "img01_schema_release_evidence.sql"))
	if err != nil {
		return errors.New("packaged schema evidence query missing")
	}
	databaseURL, password, err := settings.databaseURLAndPasswordFile()
	if err != nil {
		return err
	}
	evidence, err := migrator.SchemaEvidenceCloudSQL(ctx, databaseURL, password, settings.instance, settings.privateIP, settings.setRole, query)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(evidence)
}
