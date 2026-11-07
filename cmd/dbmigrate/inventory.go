package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

type offlineInventory struct {
	Format          string                      `json:"format"`
	InventorySHA256 string                      `json:"inventory_sha256"`
	Migrations      []migrator.MigrationSummary `json:"migrations"`
}

// This path deliberately runs before connection settings are loaded. It is
// safe for inspecting the scratch runtime image with no network or secrets.
func printOfflineInventory(output io.Writer, directory string, conflictingMode bool) error {
	if conflictingMode {
		return errors.New("-inventory-only cannot combine with execution, planning, administrative options, or positional arguments")
	}
	migrations, err := migrator.DiscoverMigrations(directory)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		return errors.New("migration inventory is empty")
	}
	report := offlineInventory{Format: "espacigo.migrations.inventory.v1"}
	fingerprint := sha256.New()
	for _, migration := range migrations {
		report.Migrations = append(report.Migrations, migrator.MigrationSummary{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum})
		fmt.Fprintf(fingerprint, "%s  %s\n", migration.Checksum, migration.Name)
	}
	report.InventorySHA256 = hex.EncodeToString(fingerprint.Sum(nil))
	return json.NewEncoder(output).Encode(report)
}
