package rentaldemo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestConfirmationCarriesImmutableSnapshotAndCanBeReadWithoutReplay(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T12:00:00Z", "2030-01-16T13:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	const intent = "00000000-0000-4000-8000-000000000011"
	c, err := s.Confirm(ctx, ActorRenter, q.ID, true, intent)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(c)
	var got map[string]any
	_ = json.Unmarshal(wire, &got)
	for key, want := range map[string]any{"id": q.ID, "space_id": q.SpaceID, "space_title": q.SpaceTitle, "start_at": q.StartAt, "end_at": q.EndAt, "subtotal_clp": q.SubtotalCLP, "quote_id": q.ID, "payment_status": "paid_simulated"} {
		if got[key] != want {
			t.Errorf("snapshot %s=%v want %v", key, got[key], want)
		}
	}
	before := len(s.occupied)
	again, err := s.Confirm(ctx, ActorRenter, q.ID, true, intent)
	if err != nil || again.QuoteID != c.QuoteID || len(s.occupied) != before {
		t.Fatal("same intent duplicated or changed")
	}
	read, err := s.Attempt(ctx, ActorRenter, intent)
	if err != nil || read.QuoteID != c.QuoteID || len(s.occupied) != before {
		t.Fatal("readback missing or mutated")
	}
	if _, err := s.Attempt(ctx, ActorRenter2, intent); !errors.Is(err, ErrNotFound) {
		t.Fatal("attempt leaked to another actor")
	}
	sp, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)
	if _, err := s.UpdateSpace(ctx, ActorHost, sp.ID, sp.Revision, sp.State, "24000", sp.OpenHour, sp.CloseHour); err != nil {
		t.Fatal(err)
	}
	read, err = s.Attempt(ctx, ActorRenter, intent)
	if err != nil || read.Quote.SubtotalCLP != q.SubtotalCLP {
		t.Fatal("historical snapshot recalculated")
	}
}

func TestConfirmationRejectsExactExpiryAndUnapprovedIntent(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T12:00:00Z", "2030-01-16T13:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	intent := "00000000-0000-4000-8000-000000000012"
	if _, err := s.Confirm(ctx, ActorRenter, q.ID, false, intent); !errors.Is(err, ErrInvalid) || len(s.occupied) != 0 {
		t.Fatal("unchecked approval mutated demo")
	}
	now, _ = time.Parse(time.RFC3339, q.ExpiresAt)
	if _, err := s.Confirm(ctx, ActorRenter, q.ID, true, intent); !errors.Is(err, ErrConflict) || len(s.occupied) != 0 {
		t.Fatalf("expired quote accepted: %v occupancy=%d", err, len(s.occupied))
	}
}

func TestConfirmationConflictOnDifferentIntentOrOverlappingOccupancy(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	q1, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T14:00:00Z", "2030-01-16T16:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	q2, err := s.CreateQuote(ctx, ActorRenter2, QuoteRequest{SpaceIDHour, "2030-01-16T15:00:00Z", "2030-01-16T17:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	intent1 := "00000000-0000-4000-8000-000000000021"
	if _, err := s.Confirm(ctx, ActorRenter, q1.ID, true, intent1); err != nil {
		t.Fatalf("q1 confirm failed: %v", err)
	}
	// Overlapping quote confirmation must fail with ErrConflict
	intent2 := "00000000-0000-4000-8000-000000000022"
	if _, err := s.Confirm(ctx, ActorRenter2, q2.ID, true, intent2); !errors.Is(err, ErrConflict) {
		t.Fatalf("overlapping q2 confirmed: got %v, want ErrConflict", err)
	}
	// Re-confirming q1 with a different intent must fail with ErrConflict
	intent1Alt := "00000000-0000-4000-8000-000000000023"
	if _, err := s.Confirm(ctx, ActorRenter, q1.ID, true, intent1Alt); !errors.Is(err, ErrConflict) {
		t.Fatalf("re-confirming q1 with new intent: got %v, want ErrConflict", err)
	}
}

func TestConfirmationIntentUUIDCasingAndFormatValidation(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T16:00:00Z", "2030-01-16T17:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}

	// Invalid UUID formats rejected
	for _, badIntent := range []string{
		"00000000-0000-4000-8000-0000000000",   // too short
		"00000000-0000-4000-8000-0000000000000",  // too long
		"000000000000400080000000000000000000",   // no hyphens
		"00000000-0000-4000-8000-00000000000Z",  // non-hex
		"",
	} {
		if _, err := s.Confirm(ctx, ActorRenter, q.ID, true, badIntent); !errors.Is(err, ErrInvalid) {
			t.Errorf("bad intent %q accepted: %v", badIntent, err)
		}
	}

	// Uppercase intent is normalized and readable in lowercase
	upperIntent := "00000000-0000-4000-8000-AAAAAAAAAAAA"
	c, err := s.Confirm(ctx, ActorRenter, q.ID, true, upperIntent)
	if err != nil {
		t.Fatalf("uppercase confirm failed: %v", err)
	}
	if c.IntentID != "00000000-0000-4000-8000-aaaaaaaaaaaa" {
		t.Errorf("confirmation intent not normalized: %q", c.IntentID)
	}
	read, err := s.Attempt(ctx, ActorRenter, "00000000-0000-4000-8000-aaaaaaaaaaaa")
	if err != nil || read.QuoteID != q.ID {
		t.Fatalf("lowercase attempt readback failed: %v", err)
	}
}

func TestConfirmationQuoteOwnershipAndNotFound(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	const validIntent = "00000000-0000-4000-8000-000000000099"

	// 1. Non-existent quote returns ErrNotFound
	if _, err := s.Confirm(ctx, ActorRenter, "non-existent-quote-id", true, validIntent); !errors.Is(err, ErrNotFound) {
		t.Fatalf("confirm non-existent quote got %v, want ErrNotFound", err)
	}

	// 2. Quote created by Renter cannot be confirmed by Renter2
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T18:00:00Z", "2030-01-16T19:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(ctx, ActorRenter2, q.ID, true, validIntent); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other renter confirming quote got %v, want ErrForbidden", err)
	}

	// 3. Host cannot confirm quote
	if _, err := s.Confirm(ctx, ActorHost, q.ID, true, validIntent); !errors.Is(err, ErrForbidden) {
		t.Fatalf("host confirming quote got %v, want ErrForbidden", err)
	}
}
