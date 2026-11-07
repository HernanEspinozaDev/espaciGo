package migrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

// ReleaseContract pins the reviewed ledger prefix and all packaged SQL bytes.
// It deliberately does not authorize SQL or replace cloud backup/provenance gates.
type ReleaseContract struct {
	SourceSHA       string `json:"source_sha"`
	InventorySHA256 string `json:"inventory_sha256"`
	ExpectedApplied int    `json:"expected_applied"`
	Database        string `json:"database"`
}

type ReleaseResult struct {
	Format         string          `json:"format"`
	Contract       ReleaseContract `json:"contract"`
	Before         PlanResult      `json:"before"`
	After          PlanResult      `json:"after"`
	Applied        []int64         `json:"applied"`
	SchemaEvidence json.RawMessage `json:"schema_evidence,omitempty"`
}

func inventoryFingerprint(migrations []Migration) string {
	digest := sha256.New()
	for _, migration := range migrations {
		fmt.Fprintf(digest, "%s  %s\n", migration.Checksum, migration.Name)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func validHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	if err != nil {
		return false
	}
	for _, r := range value {
		if r >= 'A' && r <= 'F' {
			return false
		}
	}
	return true
}

func validateReleaseContract(contract ReleaseContract, migrations []Migration) error {
	if !validHex(contract.SourceSHA, 40) || !validHex(contract.InventorySHA256, 64) || contract.Database == "" || contract.ExpectedApplied < 1 || contract.ExpectedApplied > len(migrations) {
		return errors.New("release requires full SHA, exact inventory fingerprint, existing database and reviewed ledger prefix")
	}
	if err := verifyRangeInventory(migrations, migrationRange{int64(contract.ExpectedApplied), int64(len(migrations))}); err != nil {
		return err
	}
	if inventoryFingerprint(migrations) != contract.InventorySHA256 {
		return errors.New("release inventory fingerprint differs from packaged migrations")
	}
	return nil
}

// RunReleaseCloudSQL keeps preflight, application and postcheck on one session,
// under the same advisory lock. No administrative repair mode is supported.
func RunReleaseCloudSQL(ctx context.Context, databaseURL, password, migrationDir, instance string, privateIP bool, setRole string, contract ReleaseContract, schemaQuery []byte) (ReleaseResult, error) {
	if _, err := schemaEvidenceStatement(schemaQuery); err != nil {
		return ReleaseResult{}, err
	}
	config, err := cloudsql.NewPGXConfig(databaseURL, password)
	if err != nil {
		return ReleaseResult{}, err
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return ReleaseResult{}, err
	}
	defer closeConnector()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return ReleaseResult{}, errors.New("connect release database")
	}
	defer conn.Close(context.Background())
	return runReleaseWithConn(ctx, conn, migrationDir, setRole, contract, schemaQuery)
}

func runReleaseWithConn(ctx context.Context, conn *pgx.Conn, migrationDir, setRole string, contract ReleaseContract, schemaQuery ...[]byte) (ReleaseResult, error) {
	// Discover once: the exact in-memory SQL checked here is what the runner uses.
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return ReleaseResult{}, err
	}
	if err := validateReleaseContract(contract, migrations); err != nil {
		return ReleaseResult{}, err
	}
	if len(schemaQuery) > 1 {
		return ReleaseResult{}, errors.New("one schema evidence query required")
	}
	if len(schemaQuery) == 1 {
		if _, err := schemaEvidenceStatement(schemaQuery[0]); err != nil {
			return ReleaseResult{}, err
		}
	}
	if setRole != "" {
		if !validDatabaseRoleName(setRole) {
			return ReleaseResult{}, errors.New("invalid release role")
		}
		if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{setRole}.Sanitize()); err != nil {
			return ReleaseResult{}, errors.New("could not assume release role")
		}
	}
	release, err := AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return ReleaseResult{}, err
	}
	defer func() { _ = release(context.Background()) }()
	before, err := planWithConn(ctx, conn, migrations, "")
	if err != nil {
		return ReleaseResult{}, err
	}
	if before.Database != contract.Database || !before.HistoryExists || len(before.Applied) != contract.ExpectedApplied {
		return ReleaseResult{}, errors.New("release database or applied prefix changed since review; inspect before retrying")
	}
	// The nested advisory lock is reentrant on this same PostgreSQL session;
	// releasing the runner's acquisition leaves the outer lock held for postcheck.
	result, err := runDiscoveredWithConnOptions(ctx, conn, migrations, nil)
	if err != nil {
		return ReleaseResult{}, err
	}
	after, err := planWithConn(ctx, conn, migrations, "")
	if err != nil {
		return ReleaseResult{}, err
	}
	if after.Database != contract.Database || !after.HistoryExists || len(after.Applied) != len(migrations) || len(after.Pending) != 0 {
		return ReleaseResult{}, errors.New("release postcheck is incomplete")
	}
	if result.Applied == nil {
		result.Applied = []int64{}
	}
	var evidence json.RawMessage
	if len(schemaQuery) == 1 {
		evidence, err = collectSchemaEvidence(ctx, conn, schemaQuery[0])
		if err != nil {
			return ReleaseResult{}, err
		}
	}
	return ReleaseResult{Format: "espacigo.migration-execution.v2", Contract: contract, Before: before, After: after, Applied: result.Applied, SchemaEvidence: evidence}, nil
}
