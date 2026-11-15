package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var memoryRotationNow = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

const memoryRotationAccountA = "11111111-1111-4111-8111-111111111111"
const memoryRotationAccountB = "22222222-2222-4222-8222-222222222222"
const memoryRotationDeclaration = "33333333-3333-4333-8333-333333333333"

func memoryRotationSet() RutFingerprintSet {
	return RutFingerprintSet{Active: "v2:" + strings.Repeat("b", 64), Candidates: []string{"v1:" + strings.Repeat("a", 64), "v2:" + strings.Repeat("b", 64)}}
}
func memoryRotationSeed(t *testing.T, r *MemoryRutClaimRegistry, account, fp string, verified bool) {
	t.Helper()
	ctx := context.Background()
	if err := r.Claim(ctx, account, memoryRotationDeclaration, fp, memoryRotationNow.Add(time.Minute), memoryRotationNow); err != nil {
		t.Fatal(err)
	}
	if verified {
		if err := r.MarkVerified(ctx, account, memoryRotationDeclaration, fp, memoryRotationNow); err != nil {
			t.Fatal(err)
		}
	}
}
func TestMemoryRotationResolvesPendingCanonicalAndChecksDeclaration(t *testing.T) {
	r := NewMemoryRutClaimRegistry()
	set := memoryRotationSet()
	memoryRotationSeed(t, r, memoryRotationAccountA, set.Candidates[0], false)
	got, err := r.ResolveCandidates(context.Background(), memoryRotationAccountA, memoryRotationDeclaration, set)
	if err != nil || got != set.Candidates[0] {
		t.Fatalf("pending canonical resolution failed: %v", err)
	}
	if _, err := r.ResolveCandidates(context.Background(), memoryRotationAccountA, "44444444-4444-4444-8444-444444444444", set); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong declaration was not rejected: %v", err)
	}
}
func TestMemoryRotationKnownVersionDifferentRutDoesNotBlock(t *testing.T) {
	r := NewMemoryRutClaimRegistry()
	memoryRotationSeed(t, r, memoryRotationAccountB, "v1:"+strings.Repeat("c", 64), true)
	set := memoryRotationSet()
	got, err := r.ClaimCandidates(context.Background(), memoryRotationAccountA, memoryRotationDeclaration, set, memoryRotationNow.Add(time.Hour), memoryRotationNow)
	if err != nil || got != set.Active {
		t.Fatalf("known version of unrelated identity blocked new claim: %v", err)
	}
}
func TestMemoryRotationRevocationCannotHideHistoricalVersion(t *testing.T) {
	r := NewMemoryRutClaimRegistry()
	set := memoryRotationSet()
	memoryRotationSeed(t, r, memoryRotationAccountA, set.Candidates[0], true)
	r.RevokeVerified(context.Background(), memoryRotationAccountA, memoryRotationNow.Add(366*24*time.Hour))
	onlyNew := RutFingerprintSet{Active: set.Active, Candidates: []string{set.Active}}
	if _, err := r.ClaimCandidates(context.Background(), memoryRotationAccountB, memoryRotationDeclaration, onlyNew, memoryRotationNow.Add(400*24*time.Hour), memoryRotationNow.Add(366*24*time.Hour)); !errors.Is(err, ErrRutHMACKeyUnavailable) {
		t.Fatalf("missing historical version bypassed after revocation: %v", err)
	}
}
func TestMemoryRotationAccountCannotAcquireSecondRutAfterVerification(t *testing.T) {
	r := NewMemoryRutClaimRegistry()
	set := memoryRotationSet()
	memoryRotationSeed(t, r, memoryRotationAccountA, "v1:"+strings.Repeat("c", 64), true)
	if err := r.Claim(context.Background(), memoryRotationAccountA, memoryRotationDeclaration, set.Candidates[0], memoryRotationNow.Add(time.Hour), memoryRotationNow); !errors.Is(err, ErrConflict) {
		t.Fatalf("legacy path violated one-claim-per-account invariant: %v", err)
	}
	if _, err := r.ClaimCandidates(context.Background(), memoryRotationAccountA, memoryRotationDeclaration, set, memoryRotationNow.Add(time.Hour), memoryRotationNow); !errors.Is(err, ErrConflict) {
		t.Fatalf("rotating path violated one-claim-per-account invariant: %v", err)
	}
}
func TestMemoryRotationUnknownPendingVersionFailsClosed(t *testing.T) {
	r := NewMemoryRutClaimRegistry()
	memoryRotationSeed(t, r, memoryRotationAccountB, "v0:"+strings.Repeat("c", 64), false)
	if _, err := r.ClaimCandidates(context.Background(), memoryRotationAccountA, memoryRotationDeclaration, memoryRotationSet(), memoryRotationNow.Add(time.Hour), memoryRotationNow); !errors.Is(err, ErrRutHMACKeyUnavailable) {
		t.Fatalf("unknown pending version was ignored: %v", err)
	}
}
func TestMemoryRotationExpiredMatchIsRemovedBeforeActiveInsert(t *testing.T) {
	r := NewMemoryRutClaimRegistry()
	set := memoryRotationSet()
	memoryRotationSeed(t, r, memoryRotationAccountB, set.Candidates[0], false)
	later := memoryRotationNow.Add(2 * time.Minute)
	got, err := r.ClaimCandidates(context.Background(), memoryRotationAccountA, memoryRotationDeclaration, set, later.Add(time.Hour), later)
	if err != nil || got != set.Active {
		t.Fatalf("expired pending replacement failed: %v", err)
	}
	if len(r.byDigest) != 1 {
		t.Fatal("expired matching fingerprint remained beside the new active claim")
	}
}
