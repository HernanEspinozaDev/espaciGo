package migrator

import (
	"context"
	"strings"
	"testing"
)

func TestRunWithRoleInRangeRejectsInvalidBoundsBeforeConnecting(t *testing.T) {
	for _, bounds := range [][2]int64{{0, 80}, {81, 80}} {
		_, err := RunWithRoleInRange(context.Background(), "not-a-database-url", t.TempDir(), "", bounds[0], bounds[1])
		if err == nil || !strings.Contains(err.Error(), "range") {
			t.Fatalf("RunWithRoleInRange(%d, %d) error = %v, want range validation error", bounds[0], bounds[1], err)
		}
	}
}

func TestVerifyRangeInventoryRequiresExactContiguousRange(t *testing.T) {
	valid := []Migration{{Version: 1}, {Version: 2}, {Version: 3}}
	if err := verifyRangeInventory(valid, migrationRange{minApplied: 2, maxVersion: 3}); err != nil {
		t.Fatalf("valid inventory rejected: %v", err)
	}
	for _, tc := range []struct {
		name       string
		migrations []Migration
		bounds     migrationRange
	}{{"gap", []Migration{{Version: 1}, {Version: 3}}, migrationRange{1, 3}}, {"out of range", []Migration{{Version: 1}, {Version: 2}, {Version: 4}}, migrationRange{1, 3}}, {"too few", []Migration{{Version: 1}, {Version: 2}}, migrationRange{1, 3}}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyRangeInventory(tc.migrations, tc.bounds); err == nil {
				t.Fatal("expected inventory rejection")
			}
		})
	}
}
