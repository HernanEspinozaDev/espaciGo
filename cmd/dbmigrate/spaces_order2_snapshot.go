package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

// The independently reviewed pin hashes ordered `version filename sha256(contents)\n` lines.
const spacesOrder2InventorySHA256 = "4b4757ae2975e8a72c817ecc02fbcb89b38b6d6d8c8bc0688c4d6bed7ba87907"

// stageSpacesOrder2Snapshot validates the complete reviewed source inventory before staging V1..V61.
func stageSpacesOrder2Snapshot(source string) (string, func(), error) {
	all, err := migrator.DiscoverMigrations(source)
	if err != nil {
		return "", nil, err
	}
	if len(all) != 84 {
		return "", nil, errors.New("source migration inventory differs from reviewed V1..V84 inventory")
	}
	fingerprint := sha256.New()
	for i, migration := range all {
		if migration.Version != int64(i+1) {
			return "", nil, errors.New("source migration sequence is not contiguous V1..V84")
		}
		fmt.Fprintf(fingerprint, "%d %s %s\n", migration.Version, migration.Name, migration.Checksum)
	}
	if hex.EncodeToString(fingerprint.Sum(nil)) != spacesOrder2InventorySHA256 {
		return "", nil, errors.New("source migration inventory fingerprint differs from reviewed inventory")
	}
	root, err := os.MkdirTemp("", "dbmigrate-spaces-order2-v61-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	if err := os.Chmod(root, 0700); err != nil {
		cleanup()
		return "", nil, err
	}
	for _, migration := range all[:61] {
		if err := os.WriteFile(filepath.Join(root, filepath.Base(migration.Path)), migration.SQL, 0600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	staged, err := migrator.DiscoverMigrations(root)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if len(staged) != 61 {
		cleanup()
		return "", nil, errors.New("staged migration snapshot does not contain V1..V61")
	}
	for i := range staged {
		if staged[i].Version != all[i].Version || staged[i].Name != all[i].Name || staged[i].Checksum != all[i].Checksum {
			cleanup()
			return "", nil, errors.New("staged migration snapshot differs from original history")
		}
	}
	return root, cleanup, nil
}
