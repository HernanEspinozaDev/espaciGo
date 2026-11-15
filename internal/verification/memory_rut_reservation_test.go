package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMemoryRutHistoricalReservationSurvivesRevocation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	a, b, d := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"
	fp := "v1:" + strings.Repeat("a", 64)
	r := NewMemoryRutClaimRegistry()
	if err := r.Claim(ctx, a, d, fp, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkVerified(ctx, a, d, fp, now); err != nil {
		t.Fatal(err)
	}
	later := now.Add(366 * 24 * time.Hour)
	r.RevokeVerified(ctx, a, later)
	if err := r.Claim(ctx, b, d, fp, later.Add(time.Hour), later); !errors.Is(err, ErrConflict) {
		t.Fatalf("historical verified reservation released after revocation: %v", err)
	}
}

func TestMemoryRutLegacyFingerprintIncludesKeyVersion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRutClaimRegistry()
	digest := strings.Repeat("b", 64)
	if err := r.Claim(ctx, "11111111-1111-4111-8111-111111111111", "33333333-3333-4333-8333-333333333333", "v1:"+digest, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := r.Claim(ctx, "22222222-2222-4222-8222-222222222222", "44444444-4444-4444-8444-444444444444", "v2:"+digest, now.Add(time.Hour), now); err != nil {
		t.Fatalf("different version was collapsed into digest identity: %v", err)
	}
}

func TestMemoryRutVerifiedReservationHasNoExpiry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRutClaimRegistry()
	a, d := "11111111-1111-4111-8111-111111111111", "33333333-3333-4333-8333-333333333333"
	fp := "v1:" + strings.Repeat("c", 64)
	if err := r.Claim(ctx, a, d, fp, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkVerified(ctx, a, d, fp, now); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, claim := range r.byDigest {
		if claim.AccountID == a && (!claim.Verified || !claim.ExpiresAt.IsZero()) {
			t.Fatal("verified reservation must be permanent without expiry")
		}
	}
}
