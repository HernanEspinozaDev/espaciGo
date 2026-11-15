package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func TestNativeKYCTailV94AcceptsOnlyReviewedExactTail(t *testing.T) {
	if err := validateNativeKYCTailMode(93, nativeAdminConfirmation, nativeKYCTailReviewedFingerprint(93), nil); err != nil {
		t.Fatalf("V93 rejected: %v", err)
	}
	if err := validateNativeKYCTailMode(94, nativeAdminConfirmation, nativeKYCTailReviewedFingerprint(94), nil); err != nil {
		t.Fatalf("reviewed V94 rejected: %v", err)
	}
	if err := validateNativeKYCTailMode(94, nativeAdminConfirmation, nativeKYCTailReviewedFingerprint(94), []string{"native-kyc-tail-version", "confirm-native-kyc-tail", "native-kyc-tail-source-sha256", "inventory-only"}); err == nil {
		t.Fatal("V94 accepted with an unrelated mode")
	}
}

func TestNativeKYCTailFingerprintRequiresExactContiguousPrefix(t *testing.T) {
	items := make([]migrator.Migration, 88)
	for i := range items {
		items[i].Version = int64(i + 1)
		items[i].Name = "migration.sql"
		items[i].Checksum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	if _, err := nativeKYCTailFingerprint(items, 87); err != nil {
		t.Fatal(err)
	}
	items[20].Version++
	if _, err := nativeKYCTailFingerprint(items, 87); err == nil {
		t.Fatal("expected non-contiguous inventory rejection")
	}
}

func TestNativeKYCTailPlanRequiresImmediateLedgerPredecessor(t *testing.T) {
	plan := migrator.PlanResult{Database: nativeAdminConfirmation, ReadOnly: true, HistoryExists: true,
		ServerVersion: "18.7", Applied: make([]migrator.HistoryEntry, 91), Pending: []migrator.MigrationSummary{{Version: 92, Name: nativeV92Name, Checksum: nativeV92Checksum}}}
	for i := range plan.Applied {
		plan.Applied[i] = migrator.HistoryEntry{Version: int64(i + 1), Name: "m.sql", Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	}
	plan.Applied[86].Version = 87
	if err := validateNativeKYCTailPlan(plan, 92); err != nil {
		t.Fatal(err)
	}
	plan.Applied[86].Version = 85
	if err := validateNativeKYCTailPlan(plan, 92); err == nil {
		t.Fatal("expected ledger gap rejection")
	}
}

func TestNativeKYCTailV93RequiresExactPrior92Prefix(t *testing.T) {
	plan := migrator.PlanResult{Database: nativeAdminConfirmation, ReadOnly: true, HistoryExists: true, ServerVersion: "18.7", Applied: make([]migrator.HistoryEntry, 92), Pending: []migrator.MigrationSummary{{Version: 93, Name: nativeV93Name, Checksum: nativeV93Checksum}}}
	for i := range plan.Applied {
		plan.Applied[i] = migrator.HistoryEntry{Version: int64(i + 1), Name: "m.sql", Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	}
	plan.Applied[91] = migrator.HistoryEntry{Version: 92, Name: nativeV92Name, Checksum: nativeV92Checksum}
	if err := validateNativeKYCTailPlan(plan, 93); err != nil {
		t.Fatalf("exact V92 predecessor rejected: %v", err)
	}
	plan.Applied[91].Checksum = strings.Repeat("0", 64)
	if validateNativeKYCTailPlan(plan, 93) == nil {
		t.Fatal("wrong V92 predecessor checksum accepted")
	}
}

func TestNativeKYCTailV93SnapshotUsesActualPinnedSource(t *testing.T) {
	source := filepath.Join("..", "..", "db", "migrations")
	stage, cleanup, err := makeNativeKYCTailSnapshot(source, 93, nativeKYCTailReviewedFingerprint(93))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	items, err := migrator.DiscoverMigrations(stage)
	if err != nil || len(items) != 93 {
		t.Fatalf("staged inventory count=%d err=%v", len(items), err)
	}
	fp, err := nativeKYCTailFingerprint(items, 93)
	if err != nil || fp != nativeKYCTailReviewedFingerprint(93) {
		t.Fatalf("staged fingerprint=%s err=%v", fp, err)
	}
}
