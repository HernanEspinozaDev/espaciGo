package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

const (
	nativeV86Name     = "V000086__identity_declaration_retention_purge.sql"
	nativeV86Checksum = "af864b793d8f4cbc9cf28dbfe65b8ed6d2388a0ade9a739fc8b480470bd3dc8e"
)

// nativeV86InventoryFingerprint is SHA-256 over `version name checksum\n` rows V1..V86.
func nativeV86InventoryFingerprint(items []migrator.Migration) (string, error) {
	if len(items) < 86 {
		return "", fmt.Errorf("source inventory has %d migrations, need V1..V86", len(items))
	}
	items = items[:86]
	h := sha256.New()
	for i, m := range items {
		if m.Version != int64(i+1) || m.Name == "" || len(m.Checksum) != 64 {
			return "", fmt.Errorf("source inventory is not a valid contiguous V1..V86 at %d", i+1)
		}
		fmt.Fprintf(h, "%d %s %s\n", m.Version, m.Name, m.Checksum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func makeNativeV86Snapshot(source, expectedFingerprint string) (string, func(), error) {
	items, err := migrator.DiscoverMigrations(source)
	if err != nil {
		return "", nil, err
	}
	fingerprint, err := nativeV86InventoryFingerprint(items)
	if err != nil {
		return "", nil, err
	}
	if expectedFingerprint == "" || expectedFingerprint != fingerprint {
		return "", nil, errors.New("native V86 source fingerprint does not match the explicitly reviewed fingerprint")
	}
	items = items[:86]
	if items[85].Name != nativeV86Name || items[85].Checksum != nativeV86Checksum {
		return "", nil, errors.New("native V86 migration name or pinned checksum differs")
	}
	root, err := os.MkdirTemp("", "native-kyc-v86-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for _, item := range items {
		if err := os.WriteFile(filepath.Join(root, filepath.Base(item.Path)), item.SQL, 0600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	staged, err := migrator.DiscoverMigrations(root)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	stagedFingerprint, err := nativeV86InventoryFingerprint(staged)
	if err != nil || stagedFingerprint != fingerprint {
		cleanup()
		return "", nil, errors.New("staged native V86 source differs from reviewed inventory")
	}
	return root, cleanup, nil
}

func validateNativeV86Plan(plan migrator.PlanResult) error {
	if plan.Database != nativeAdminConfirmation || !plan.ReadOnly || !plan.HistoryExists || !strings.HasPrefix(strings.TrimSpace(plan.ServerVersion), "18.7") {
		return errors.New("native V86 preflight requires read-only ledger on the exact PostgreSQL 18.7 target")
	}
	if len(plan.Applied) != 85 || len(plan.Pending) != 1 {
		return errors.New("native V86 preflight requires exactly V1..V85 applied and only V86 pending")
	}
	for i, entry := range plan.Applied {
		if entry.Version != int64(i+1) || entry.Name == "" || len(entry.Checksum) != 64 {
			return fmt.Errorf("native V86 applied ledger is not verified and contiguous at V%d", i+1)
		}
	}
	pending := plan.Pending[0]
	if pending.Version != 86 || pending.Name != nativeV86Name || pending.Checksum != nativeV86Checksum {
		return errors.New("native V86 pending ledger source differs from pinned V86")
	}
	return nil
}

func validateNativeV86PostPlan(plan migrator.PlanResult) error {
	if plan.Database != nativeAdminConfirmation || !plan.ReadOnly || !plan.HistoryExists || !strings.HasPrefix(strings.TrimSpace(plan.ServerVersion), "18.7") {
		return errors.New("native V86 postcheck requires read-only exact target ledger on PostgreSQL 18.7")
	}
	if len(plan.Applied) != 86 || len(plan.Pending) != 0 {
		return errors.New("native V86 postcheck requires V1..V86 applied with no pending migrations")
	}
	for i, entry := range plan.Applied {
		if entry.Version != int64(i+1) || entry.Name == "" || len(entry.Checksum) != 64 {
			return fmt.Errorf("native V86 post-ledger is not contiguous at V%d", i+1)
		}
	}
	if plan.Applied[85].Name != nativeV86Name || plan.Applied[85].Checksum != nativeV86Checksum {
		return errors.New("native V86 post-ledger does not contain the pinned V86")
	}
	return nil
}
