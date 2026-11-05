package media

import (
	"errors"
	"testing"
	"time"
)

func TestExportedNormalizeReservationOptions(t *testing.T) {
	got, err := NormalizeReservationOptions(ReservationOptions{ReplacementTargetID: "00000000-0000-4000-8000-000000000001", ExpectedRevision: "2026-10-27T22:18:16Z"})
	if err != nil || got.ReplacementTargetID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("NormalizeReservationOptions() = %#v, %v", got, err)
	}
	_, err = NormalizeReservationOptions(ReservationOptions{ReplacementTargetID: "bad", ExpectedRevision: "2026-10-27T22:18:16Z"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid options error = %v", err)
	}
}

func TestNormalizeReservationOptionsCanonicalizesReplacementTarget(t *testing.T) {
	got, err := normalizeReservationOptions(ReservationOptions{
		ReplacementTargetID: " 00000000-0000-4000-8000-0000000000AB ",
		ExpectedRevision:    "2026-10-27T22:18:16Z",
	})
	if err != nil {
		t.Fatalf("normalizeReservationOptions() error = %v", err)
	}
	if got.ReplacementTargetID != "00000000-0000-4000-8000-0000000000ab" {
		t.Fatalf("canonical target = %q", got.ReplacementTargetID)
	}
	if got.ExpectedRevision != "2026-10-27T22:18:16Z" {
		t.Fatalf("revision changed unexpectedly: %q", got.ExpectedRevision)
	}
}

func TestNormalizeReservationOptionsRejectsInvalidCombination(t *testing.T) {
	cases := []struct {
		name    string
		options ReservationOptions
	}{
		{"target without revision", ReservationOptions{ReplacementTargetID: "00000000-0000-4000-8000-000000000001"}},
		{"revision without target", ReservationOptions{ExpectedRevision: "2026-10-27T22:18:16Z"}},
		{"malformed target", ReservationOptions{ReplacementTargetID: "not-a-uuid", ExpectedRevision: "2026-10-27T22:18:16Z"}},
		{"noncanonical revision", ReservationOptions{ReplacementTargetID: "00000000-0000-4000-8000-000000000001", ExpectedRevision: "2026-10-27 22:18:16Z"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := normalizeReservationOptions(tc.options); !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestCheckExpectedRevision(t *testing.T) {
	current := time.Date(2026, 10, 27, 22, 18, 16, 0, time.UTC)
	if err := checkExpectedRevision(current, "2026-10-27T22:18:16Z"); err != nil {
		t.Fatalf("matching revision error = %v", err)
	}
	if err := checkExpectedRevision(current, "2026-10-27T22:18:15Z"); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("stale revision error = %v, want ErrPrecondition", err)
	}
	if err := checkExpectedRevision(current, "2026-10-27 22:18:16Z"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed revision error = %v, want ErrInvalid", err)
	}
}
