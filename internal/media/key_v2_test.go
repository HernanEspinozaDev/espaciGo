package media

import (
	"strings"
	"testing"
	"time"
)

func TestSpaceObjectKeyUsesV2ReservationIdentity(t *testing.T) {
	at := time.Date(2026, 10, 27, 12, 34, 56, 123456789, time.FixedZone("x", -3*3600))
	got := objectKey(PurposeSpace, "00000000-0000-4000-8000-000000000010", "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", 42, VariantDisplay, at)
	want := "users/00000000-0000-4000-8000-000000000010/spaces/00000000-0000-4000-8000-000000000001/images/v2/00000000-0000-4000-8000-000000000002/20261027T153456.123456789Z_42_display.webp"
	if got != want {
		t.Fatalf("objectKey() = %q, want %q", got, want)
	}
	profile := objectKey(PurposeProfile, "00000000-0000-4000-8000-000000000010", "", "00000000-0000-4000-8000-000000000002", 42, VariantAvatar, at)
	if !strings.HasPrefix(profile, "media/v1/profiles/") {
		t.Fatalf("profile key changed: %q", profile)
	}
}
