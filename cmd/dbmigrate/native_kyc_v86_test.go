package main

import (
	"path/filepath"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func TestNativeV86ModeRejectsIncompleteOrMixedInvocation(t *testing.T) {
	if err := validateNativeV86Mode(true, nativeAdminConfirmation, nativeV86SourceFingerprint, []string{"native-kyc-v86", "confirm-native-kyc-v86", "native-kyc-v86-source-sha256", "dir"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                      string
		mode                      bool
		confirmation, fingerprint string
		flags                     []string
	}{
		{"missing mode", false, nativeAdminConfirmation, nativeV86SourceFingerprint, nil},
		{"wrong target", true, "other", nativeV86SourceFingerprint, nil},
		{"wrong fingerprint", true, nativeAdminConfirmation, "wrong", nil},
		{"mixed mode", true, nativeAdminConfirmation, nativeV86SourceFingerprint, []string{"plan-only"}},
		{"foreign confirmation", true, nativeAdminConfirmation, nativeV86SourceFingerprint, []string{"confirm-cloud-owner-pre81"}},
		{"release override", true, nativeAdminConfirmation, nativeV86SourceFingerprint, []string{"release-source-sha"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if validateNativeV86Mode(tc.mode, tc.confirmation, tc.fingerprint, tc.flags) == nil {
				t.Fatal("unsafe invocation accepted")
			}
		})
	}
}

func TestNativeV86SourceSnapshotPinsExactV1ThroughV86(t *testing.T) {
	source := filepath.Join("..", "..", "db", "migrations")
	all, err := migrator.DiscoverMigrations(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", t.TempDir())
	stage, cleanup, err := makeNativeV86Snapshot(source, nativeV86SourceFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	got, err := migrator.DiscoverMigrations(stage)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 86 {
		t.Fatalf("snapshot contains %d migrations", len(got))
	}
	for i := range got {
		if got[i].Name != all[i].Name || got[i].Checksum != all[i].Checksum {
			t.Fatalf("snapshot V%d differs", i+1)
		}
	}
	if _, _, err := makeNativeV86Snapshot(source, "unreviewed"); err == nil {
		t.Fatal("unreviewed fingerprint accepted")
	}
}

func TestNativeV86PlanRequiresOnlyV86Pending(t *testing.T) {
	all, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	plan := migrator.PlanResult{Database: nativeAdminConfirmation, ServerVersion: "18.7", ReadOnly: true, HistoryExists: true}
	for _, m := range all[:85] {
		plan.Applied = append(plan.Applied, migrator.HistoryEntry{Version: m.Version, Name: m.Name, Checksum: m.Checksum})
	}
	plan.Pending = []migrator.MigrationSummary{{Version: 86, Name: nativeV86Name, Checksum: nativeV86Checksum}}
	if err := validateNativeV86Plan(plan); err != nil {
		t.Fatal(err)
	}
	plan.Pending = nil
	if validateNativeV86Plan(plan) == nil {
		t.Fatal("accepted no pending V86")
	}
}
