package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func TestNativeKYCTailCreateAuthorityByExactVersion(t *testing.T) {
	for version, want := range map[int]bool{87: false, 88: true, 89: true, 90: true, 91: false, 92: true, 93: true, 94: true, 95: true, 96: true} {
		if got := nativeKYCTailRequiresCreate(version); got != want {
			t.Fatalf("V%d CREATE prerequisite=%t, want %t", version, got, want)
		}
	}
}

func TestNativeKYCTailSnapshotAndModePinSingleVersion(t *testing.T) {
	source := filepath.Join("..", "..", "db", "migrations")
	t.Setenv("TMPDIR", t.TempDir())
	for _, v := range []int{87, 88, 89, 90, 91, 92, 93, 94} {
		fp := nativeKYCTailReviewedFingerprint(v)
		if err := validateNativeKYCTailMode(v, nativeAdminConfirmation, fp, []string{"native-kyc-tail-version", "confirm-native-kyc-tail", "native-kyc-tail-source-sha256", "dir"}); err != nil {
			t.Fatal(err)
		}
		stage, cleanup, err := makeNativeKYCTailSnapshot(source, v, fp)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(cleanup)
		items, err := migrator.DiscoverMigrations(stage)
		if err != nil || len(items) != v {
			t.Fatalf("snapshot version=%d count=%d err=%v", v, len(items), err)
		}
		if _, _, err := makeNativeKYCTailSnapshot(source, v, strings.Repeat("0", 64)); err == nil {
			t.Fatal("unreviewed source accepted")
		}
		if validateNativeKYCTailMode(v, nativeAdminConfirmation, fp, []string{"native-kyc-v86"}) == nil {
			t.Fatal("mixed modes accepted")
		}
	}
	if validateNativeKYCTailMode(95, nativeAdminConfirmation, nativeKYCTailReviewedFingerprint(94), nil) == nil {
		t.Fatal("out-of-range version accepted")
	}
}

func TestNativeKYCTailPostRejectsWrongFinalChecksum(t *testing.T) {
	all, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []int{87, 88, 89, 90, 91, 92, 93, 94, 95, 96} {
		p := migrator.PlanResult{Database: nativeAdminConfirmation, ReadOnly: true, HistoryExists: true, ServerVersion: "18.7"}
		for _, m := range all[:v] {
			p.Applied = append(p.Applied, migrator.HistoryEntry{Version: m.Version, Name: m.Name, Checksum: m.Checksum})
		}
		if err := validateNativeKYCTailPost(p, v); err != nil {
			t.Fatal(err)
		}
		p.Applied[v-1].Checksum = strings.Repeat("0", 64)
		if validateNativeKYCTailPost(p, v) == nil {
			t.Fatal("wrong final checksum accepted")
		}
	}
}

func TestNativeKYCTailV94RejectsWrongPredecessor(t *testing.T) {
	all, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	p := migrator.PlanResult{Database: nativeAdminConfirmation, ReadOnly: true, HistoryExists: true, ServerVersion: "18.7", Pending: []migrator.MigrationSummary{{Version: 94, Name: nativeV94Name, Checksum: nativeV94Checksum}}}
	for _, m := range all[:93] {
		p.Applied = append(p.Applied, migrator.HistoryEntry{Version: m.Version, Name: m.Name, Checksum: m.Checksum})
	}
	if err := validateNativeKYCTailPlan(p, 94); err != nil {
		t.Fatal(err)
	}
	p.Applied[92].Checksum = strings.Repeat("0", 64)
	if validateNativeKYCTailPlan(p, 94) == nil {
		t.Fatal("V94 accepted wrong V93 predecessor checksum")
	}
}
