package main

import (
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"os"
	"path/filepath"
	"testing"
)

func TestCloudOwnerSnapshotCopiesOnlyVerifiedHistoryThrough80(t *testing.T) {
	src := filepath.Join("..", "..", "db", "migrations")
	dir, cleanup, err := makeCloudOwnerSnapshot(src)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	got, err := migrator.DiscoverMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 80 || got[0].Version != 1 || got[79].Version != 80 {
		t.Fatalf("snapshot range/count %d", len(got))
	}
	orig, err := migrator.DiscoverMigrations(src)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		if got[i].Checksum != orig[i].Checksum {
			t.Fatalf("checksum changed at %d", i+1)
		}
	}
}
func TestCloudOwnerSnapshotRejectsGap(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "V000001__one.sql"), []byte("select 1"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, e := makeCloudOwnerSnapshot(d); e == nil {
		t.Fatal("accepted incomplete history")
	}
}
func TestCloudOwnerPendingOnlyV76To80(t *testing.T) {
	for _, p := range []migrator.PlanResult{
		{Database: "espacigo_dms_stage_20261020", Pending: []migrator.MigrationSummary{{Version: 76}, {Version: 77}}},
		{Database: "espacigo_dms_stage_20261020", Pending: []migrator.MigrationSummary{{Version: 80}}},
		{Database: "wrong", Pending: []migrator.MigrationSummary{{Version: 76}}},
	} {
		if validateCloudOwnerPending(p) == nil {
			t.Fatalf("accepted invalid plan %#v", p)
		}
	}
}

func TestCloudOwnerRecoveryAcceptsEveryCommittedBoundary(t *testing.T) {
	for last := int64(75); last <= 80; last++ {
		p := migrator.PlanResult{Database: cloudOwnerConfirmation, HistoryExists: true, ReadOnly: true}
		for v := int64(1); v <= last; v++ {
			p.Applied = append(p.Applied, migrator.HistoryEntry{Version: v})
		}
		for v := last + 1; v <= 80; v++ {
			p.Pending = append(p.Pending, migrator.MigrationSummary{Version: v})
		}
		if err := validateCloudOwnerPending(p); err != nil {
			t.Fatalf("recovery after V%d: %v", last, err)
		}
	}
}

func TestCloudOwnerRecoveryRejectsEmptyOrTruncatedInventory(t *testing.T) {
	for _, p := range []migrator.PlanResult{
		{Database: cloudOwnerConfirmation},
		{Database: cloudOwnerConfirmation, ReadOnly: true, HistoryExists: true, Applied: []migrator.HistoryEntry{{Version: 75}}},
	} {
		if validateCloudOwnerPending(p) == nil {
			t.Fatal("accepted incomplete plan")
		}
	}
}
