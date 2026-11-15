package rentaldemo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestQuoteDoesNotHoldAndConfirmationIsAtomic(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	space := SpaceIDHour
	sp, err := s.Space(ctx, "host", space, true)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2030, 1, 17, 14, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	q, err := s.CreateQuote(ctx, "renter", QuoteRequest{SpaceID: space, StartAt: start.Format(time.RFC3339), EndAt: end.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	q2, err := s.CreateQuote(ctx, "renter2", QuoteRequest{SpaceID: space, StartAt: start.Format(time.RFC3339), EndAt: end.Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("quote must not hold interval: %v", err)
	}
	var wins int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, pair := range []struct {
		actor string
		quote Quote
	}{{"renter", q}, {"renter2", q2}} {
		wg.Add(1)
		go func(idx int, actor string, quote Quote) {
			defer wg.Done()
			intent := fmt.Sprintf("00000000-0000-4000-8000-%012d", idx+1)
			_, e := s.Confirm(ctx, actor, quote.ID, true, intent)
			if e == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(e, ErrConflict) {
				t.Errorf("Confirm error = %v", e)
			}
		}(i, pair.actor, pair.quote)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("confirm winners = %d, want 1", wins)
	}
	_ = sp
}

func TestRentalDurationValidationAndRejection(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	baseDate := time.Date(2030, 1, 16, 14, 0, 0, 0, time.UTC)

	// Hourly durations: 1, 2, 4 allowed. 3, 5, 90min rejected.
	tests := []struct {
		name    string
		space   string
		start   time.Time
		end     time.Time
		wantErr bool
	}{
		{"hour: 1h valid", SpaceIDHour, baseDate, baseDate.Add(time.Hour), false},
		{"hour: 2h valid", SpaceIDHour, baseDate, baseDate.Add(2 * time.Hour), false},
		{"hour: 4h valid", SpaceIDHour, baseDate, baseDate.Add(4 * time.Hour), false},
		{"hour: 3h invalid", SpaceIDHour, baseDate, baseDate.Add(3 * time.Hour), true},
		{"hour: 90min invalid", SpaceIDHour, baseDate, baseDate.Add(90 * time.Minute), true},
		{"hour: 15min offset invalid", SpaceIDHour, baseDate.Add(15 * time.Minute), baseDate.Add(75 * time.Minute), true},
		{"hour: past date invalid", SpaceIDHour, time.Date(2030, 1, 14, 10, 0, 0, 0, time.UTC), time.Date(2030, 1, 14, 11, 0, 0, 0, time.UTC), true},
		{"hour: >90 days horizon invalid", SpaceIDHour, baseDate.Add(95 * 24 * time.Hour), baseDate.Add(95*24*time.Hour + time.Hour), true},
		{"day: 1d valid", SpaceIDDay, time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC), time.Date(2030, 1, 17, 3, 0, 0, 0, time.UTC), false},
		{"day: 2d valid", SpaceIDDay, time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC), time.Date(2030, 1, 18, 3, 0, 0, 0, time.UTC), false},
		{"day: 3d valid", SpaceIDDay, time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC), time.Date(2030, 1, 19, 3, 0, 0, 0, time.UTC), false},
		{"day: 4d invalid", SpaceIDDay, time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC), time.Date(2030, 1, 20, 3, 0, 0, 0, time.UTC), true},
		{"month: 1m valid", SpaceIDMonth, time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC), time.Date(2030, 2, 16, 3, 0, 0, 0, time.UTC), false},
		{"month: 2m invalid", SpaceIDMonth, time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC), time.Date(2030, 3, 16, 3, 0, 0, 0, time.UTC), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateQuote(ctx, "renter", QuoteRequest{
				SpaceID: tc.space,
				StartAt: tc.start.Format(time.RFC3339),
				EndAt:   tc.end.Format(time.RFC3339),
			})
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			} else if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestActorAuthorizationAndPrivilegeBoundaries(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start := time.Date(2030, 1, 16, 14, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	// Host cannot create quotes or confirm
	_, err = s.CreateQuote(ctx, "host", QuoteRequest{
		SpaceID: SpaceIDHour,
		StartAt: start.Format(time.RFC3339),
		EndAt:   end.Format(time.RFC3339),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("host creating quote got %v, want ErrForbidden", err)
	}

	// Renter cannot create block
	_, err = s.Block(ctx, "renter", SpaceIDHour, "1", start.Format(time.RFC3339), end.Format(time.RFC3339))
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("renter creating block got %v, want ErrForbidden", err)
	}

	// Renter cannot unblock
	err = s.Unblock(ctx, "renter", SpaceIDHour, "block-id", "1")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("renter unblocking got %v, want ErrForbidden", err)
	}

	// Unknown actor rejected
	_, err = s.CreateQuote(ctx, "intruder", QuoteRequest{
		SpaceID: SpaceIDHour,
		StartAt: start.Format(time.RFC3339),
		EndAt:   end.Format(time.RFC3339),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("intruder creating quote got %v, want ErrForbidden", err)
	}
}

func TestQuoteExpirationPreventsConfirmation(t *testing.T) {
	currentTime := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return currentTime })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start := time.Date(2030, 1, 16, 14, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	q, err := s.CreateQuote(ctx, "renter", QuoteRequest{
		SpaceID: SpaceIDHour,
		StartAt: start.Format(time.RFC3339),
		EndAt:   end.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Advance time beyond quote expiration (10 minutes)
	currentTime = currentTime.Add(15 * time.Minute)

	// Confirm should now fail with ErrConflict
	_, err = s.Confirm(ctx, "renter", q.ID, true, "00000000-0000-4000-8000-000000000099")
	if !errors.Is(err, ErrConflict) {
		t.Errorf("confirming expired quote got %v, want ErrConflict", err)
	}
}

func TestAttemptRetrievalIdempotence(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start := time.Date(2030, 1, 16, 14, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	q, err := s.CreateQuote(ctx, "renter", QuoteRequest{
		SpaceID: SpaceIDHour,
		StartAt: start.Format(time.RFC3339),
		EndAt:   end.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}

	intent := "00000000-0000-4000-8000-000000000077"
	c, err := s.Confirm(ctx, "renter", q.ID, true, intent)
	if err != nil {
		t.Fatal(err)
	}

	// Query attempt via Attempt method
	retrieved, err := s.Attempt(ctx, "renter", intent)
	if err != nil {
		t.Fatalf("failed to retrieve attempt: %v", err)
	}
	if retrieved.QuoteID != c.QuoteID || retrieved.IntentID != intent {
		t.Errorf("retrieved attempt mismatch: got %+v, want %+v", retrieved, c)
	}

	// Case insensitive readback
	upperRetrieved, err := s.Attempt(ctx, "renter", strings.ToUpper(intent))
	if err != nil || upperRetrieved.QuoteID != c.QuoteID {
		t.Fatalf("uppercase intent retrieval failed: %v", err)
	}

	// Invalid actor is forbidden
	if _, err := s.Attempt(ctx, "intruder", intent); !errors.Is(err, ErrForbidden) {
		t.Errorf("invalid actor reading attempt got %v, want ErrForbidden", err)
	}

	// Non-existent intent returns ErrNotFound
	if _, err := s.Attempt(ctx, "renter", "00000000-0000-4000-8000-999999999999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-existent intent got %v, want ErrNotFound", err)
	}

	// Another actor cannot read this attempt
	_, err = s.Attempt(ctx, "renter2", intent)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("other actor reading attempt got %v, want ErrNotFound", err)
	}
}

func TestSpaceRetrievalAndVisibilityBoundaries(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// 1. Renter reads active space publicly
	sp, err := s.Space(ctx, ActorRenter, SpaceIDHour, false)
	if err != nil || sp.ID != SpaceIDHour {
		t.Fatalf("renter reading space failed: %v", err)
	}

	// 2. Renter requesting owner=true is forbidden
	if _, err := s.Space(ctx, ActorRenter, SpaceIDHour, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("renter requesting owner mode got %v, want ErrForbidden", err)
	}

	// 3. Host requesting owner=true succeeds
	hostSp, err := s.Space(ctx, ActorHost, SpaceIDHour, true)
	if err != nil || hostSp.ID != SpaceIDHour {
		t.Fatalf("host reading space failed: %v", err)
	}

	// 4. Invalid actor is forbidden
	if _, err := s.Space(ctx, "invalid-actor", SpaceIDHour, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("invalid actor got %v, want ErrForbidden", err)
	}

	// 5. Non-existent space returns ErrNotFound
	if _, err := s.Space(ctx, ActorRenter, "non-existent", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-existent space got %v, want ErrNotFound", err)
	}

	// 6. Host hides space -> public caller gets ErrNotFound, host still reads it
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, hostSp.Revision, "oculta", hostSp.UnitPriceCLP, hostSp.OpenHour, hostSp.CloseHour); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Space(ctx, ActorRenter, SpaceIDHour, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("renter reading hidden space got %v, want ErrNotFound", err)
	}
	if _, err := s.Space(ctx, ActorHost, SpaceIDHour, true); err != nil {
		t.Fatalf("host reading hidden space failed: %v", err)
	}

	// 7. Invalid state returns ErrInvalid
	hostSpUpdated, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, hostSpUpdated.Revision, "archivada", hostSp.UnitPriceCLP, hostSp.OpenHour, hostSp.CloseHour); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid state in UpdateSpace got %v, want ErrInvalid", err)
	}

	// 8. Transition to borrador hides from public reader
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, hostSpUpdated.Revision, "borrador", hostSp.UnitPriceCLP, hostSp.OpenHour, hostSp.CloseHour); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Space(ctx, ActorRenter, SpaceIDHour, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("renter reading draft space got %v, want ErrNotFound", err)
	}

	// 9. UpdateSpace authorization and parameter bounds
	if _, err := s.UpdateSpace(ctx, ActorRenter, SpaceIDHour, "1", "activa", "15000", 8, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("renter updating space got %v, want ErrForbidden", err)
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, "1", "activa", "0", 8, 20); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero price got %v, want ErrInvalid", err)
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, "1", "activa", "15000", 20, 8); !errors.Is(err, ErrInvalid) {
		t.Fatalf("open >= close got %v, want ErrInvalid", err)
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, "stale-rev", "activa", "15000", 8, 20); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision in UpdateSpace got %v, want ErrConflict", err)
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, "unknown-space", "1", "activa", "15000", 8, 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown space in UpdateSpace got %v, want ErrNotFound", err)
	}
}

func TestStoreContextCancellationAndNilGuards(t *testing.T) {
	// 1. NewStore with nil clock returns ErrInvalid
	if _, err := NewStore(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewStore(nil) got %v, want ErrInvalid", err)
	}

	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	// 2. Cancelled context is rejected by all Store operations
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.Space(ctx, ActorRenter, SpaceIDHour, false); !errors.Is(err, ErrInvalid) {
		t.Errorf("Space with cancelled ctx got %v, want ErrInvalid", err)
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, "1", "activa", "15000", 8, 20); !errors.Is(err, ErrInvalid) {
		t.Errorf("UpdateSpace with cancelled ctx got %v, want ErrInvalid", err)
	}
	if _, err := s.Block(ctx, ActorHost, SpaceIDHour, "1", "2030-01-16T14:00:00Z", "2030-01-16T16:00:00Z"); !errors.Is(err, ErrInvalid) {
		t.Errorf("Block with cancelled ctx got %v, want ErrInvalid", err)
	}
	if err := s.Unblock(ctx, ActorHost, SpaceIDHour, "block-id", "1"); !errors.Is(err, ErrInvalid) {
		t.Errorf("Unblock with cancelled ctx got %v, want ErrInvalid", err)
	}
	if _, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T12:00:00Z", EndAt: "2030-01-16T13:00:00Z"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("CreateQuote with cancelled ctx got %v, want ErrInvalid", err)
	}
	if _, err := s.Confirm(ctx, ActorRenter, "quote-id", true, "00000000-0000-4000-8000-000000000001"); !errors.Is(err, ErrInvalid) {
		t.Errorf("Confirm with cancelled ctx got %v, want ErrInvalid", err)
	}
	if _, err := s.Attempt(ctx, ActorRenter, "00000000-0000-4000-8000-000000000001"); !errors.Is(err, ErrInvalid) {
		t.Errorf("Attempt with cancelled ctx got %v, want ErrInvalid", err)
	}
	if _, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2030-01", 1, false); !errors.Is(err, ErrInvalid) {
		t.Errorf("Month with cancelled ctx got %v, want ErrInvalid", err)
	}
}

func TestStoreMonthValidationAndProjectionBoundaries(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()

	// 1. Renter requesting owner=true -> ErrForbidden
	if _, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2030-01", 1, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("renter requesting owner month got %v, want ErrForbidden", err)
	}

	// 2. Malformed month string -> ErrInvalid
	for _, badMonth := range []string{"invalid", "2030-1", "2030-13", "2030/01", ""} {
		if _, err := s.Month(ctx, ActorRenter, SpaceIDHour, badMonth, 1, false); !errors.Is(err, ErrInvalid) {
			t.Fatalf("bad month %q got %v, want ErrInvalid", badMonth, err)
		}
	}

	// 3. Durations outside rate unit specifications
	// Hourly only allows 1, 2, 4
	if _, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2030-01", 3, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hourly 3-hour duration got %v, want ErrInvalid", err)
	}
	// Daily only allows 1..3
	if _, err := s.Month(ctx, ActorRenter, SpaceIDDay, "2030-01", 4, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("daily 4-day duration got %v, want ErrInvalid", err)
	}
	// Monthly only allows 1
	if _, err := s.Month(ctx, ActorRenter, SpaceIDMonth, "2030-01", 2, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("monthly 2-month duration got %v, want ErrInvalid", err)
	}

	// 4. Non-existent space -> ErrNotFound
	if _, err := s.Month(ctx, ActorRenter, "unknown-space", "2030-01", 1, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown space got %v, want ErrNotFound", err)
	}

	// 5. Host adds block, public month hides private block metadata
	sp, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)
	_, err := s.Block(ctx, ActorHost, SpaceIDHour, sp.Revision, "2030-01-16T10:00:00Z", "2030-01-16T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	publicM, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2030-01", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if publicM.Revision != "" || len(publicM.Blocks) != 0 {
		t.Fatalf("public month leaked revision or blocks: rev=%q blocks=%v", publicM.Revision, publicM.Blocks)
	}

	ownerM, err := s.Month(ctx, ActorHost, SpaceIDHour, "2030-01", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if ownerM.Revision == "" || len(ownerM.Blocks) == 0 {
		t.Fatalf("owner month missing revision or blocks: rev=%q blocks=%v", ownerM.Revision, ownerM.Blocks)
	}
}

func TestStoreUnblockLifecycleAndCAS(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()

	sp, err := s.Space(ctx, ActorHost, SpaceIDHour, true)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Create a block
	blk, err := s.Block(ctx, ActorHost, SpaceIDHour, sp.Revision, "2030-01-16T14:00:00Z", "2030-01-16T16:00:00Z")
	if err != nil {
		t.Fatalf("block failed: %v", err)
	}
	spAfterBlock, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)

	// 2. Unblock with stale revision returns ErrConflict
	if err := s.Unblock(ctx, ActorHost, SpaceIDHour, blk.ID, sp.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("unblock with stale revision got %v, want ErrConflict", err)
	}

	// 3. Unblock with non-existent block ID returns ErrNotFound
	if err := s.Unblock(ctx, ActorHost, SpaceIDHour, "00000000-0000-0000-0000-000000000000", spAfterBlock.Revision); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unblock non-existent block got %v, want ErrNotFound", err)
	}

	// 4. Unblock on non-existent space returns ErrNotFound
	if err := s.Unblock(ctx, ActorHost, "unknown-space", blk.ID, spAfterBlock.Revision); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unblock unknown space got %v, want ErrNotFound", err)
	}

	// 5. Successful unblock advances revision and removes the block
	if err := s.Unblock(ctx, ActorHost, SpaceIDHour, blk.ID, spAfterBlock.Revision); err != nil {
		t.Fatalf("valid unblock failed: %v", err)
	}
	spAfterUnblock, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)
	if spAfterUnblock.Revision == spAfterBlock.Revision {
		t.Fatal("revision did not advance after unblock")
	}
	if len(s.blocks[SpaceIDHour]) != 0 {
		t.Fatalf("expected 0 blocks, got %d", len(s.blocks[SpaceIDHour]))
	}
}

func TestCreateQuoteRejectsBlockedOrOccupiedIntervals(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()

	sp, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)

	// 1. Host blocks 14:00 to 16:00
	_, err := s.Block(ctx, ActorHost, SpaceIDHour, sp.Revision, "2030-01-16T14:00:00Z", "2030-01-16T16:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Renter attempts to create quote overlapping the block -> ErrConflict
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T14:00:00Z", EndAt: "2030-01-16T16:00:00Z"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateQuote on blocked slot got %v, want ErrConflict", err)
	}

	// 3. Confirm quote on 16:00 to 18:00
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T16:00:00Z", EndAt: "2030-01-16T18:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	const intent = "00000000-0000-4000-8000-000000000088"
	_, err = s.Confirm(ctx, ActorRenter, q.ID, true, intent)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Renter2 attempts to create quote overlapping the confirmed occupancy -> ErrConflict
	_, err = s.CreateQuote(ctx, ActorRenter2, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T16:00:00Z", EndAt: "2030-01-16T18:00:00Z"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateQuote on occupied slot got %v, want ErrConflict", err)
	}
}

func TestStoreOverlapBoundariesAndEdgeCases(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()

	sp, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)

	// Host blocks 14:00 to 16:00 on 2030-01-16
	_, err := s.Block(ctx, ActorHost, SpaceIDHour, sp.Revision, "2030-01-16T14:00:00Z", "2030-01-16T16:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Adjacent before: 13:00 to 14:00 is allowed (touching end-to-start)
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T13:00:00Z", EndAt: "2030-01-16T14:00:00Z"})
	if err != nil {
		t.Errorf("Adjacent slot before block should be allowed, got error: %v", err)
	}

	// Adjacent after: 16:00 to 17:00 is allowed (touching start-to-end)
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T16:00:00Z", EndAt: "2030-01-16T17:00:00Z"})
	if err != nil {
		t.Errorf("Adjacent slot after block should be allowed, got error: %v", err)
	}

	// Overlapping start: 13:00 to 15:00 -> ErrConflict
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T13:00:00Z", EndAt: "2030-01-16T15:00:00Z"})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("Overlapping start got %v, want ErrConflict", err)
	}

	// Overlapping end: 15:00 to 17:00 -> ErrConflict
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T15:00:00Z", EndAt: "2030-01-16T17:00:00Z"})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("Overlapping end got %v, want ErrConflict", err)
	}

	// Strictly inside: 14:00 to 15:00 -> ErrConflict
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T14:00:00Z", EndAt: "2030-01-16T15:00:00Z"})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("Strictly inside block got %v, want ErrConflict", err)
	}

	// Strictly enclosing: 13:00 to 17:00 -> ErrConflict
	_, err = s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: "2030-01-16T13:00:00Z", EndAt: "2030-01-16T17:00:00Z"})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("Strictly enclosing block got %v, want ErrConflict", err)
	}
}

func TestOverlappingQuotesConfirmationConflictAndAdjacentSuccess(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	space := SpaceIDHour

	// Quote 1: 14:00 to 16:00
	q1, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{
		SpaceID: space,
		StartAt: "2030-01-18T14:00:00Z",
		EndAt:   "2030-01-18T16:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Quote 2: 15:00 to 17:00 (overlaps q1 from 15:00 to 16:00; created before q1 confirms)
	q2, err := s.CreateQuote(ctx, ActorRenter2, QuoteRequest{
		SpaceID: space,
		StartAt: "2030-01-18T15:00:00Z",
		EndAt:   "2030-01-18T17:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Quote 3: 16:00 to 18:00 (adjacent to q1, touches end-to-start; no overlap with q1)
	q3, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{
		SpaceID: space,
		StartAt: "2030-01-18T16:00:00Z",
		EndAt:   "2030-01-18T18:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Confirm q1 successfully
	intent1 := "00000000-0000-4000-8000-000000000101"
	c1, err := s.Confirm(ctx, ActorRenter, q1.ID, true, intent1)
	if err != nil {
		t.Fatalf("q1 confirmation failed: %v", err)
	}
	if c1.IntentID != intent1 {
		t.Errorf("q1 intentId = %q, want %q", c1.IntentID, intent1)
	}

	// 2. Confirm q2 fails with ErrConflict because q1 occupies 14:00-16:00
	intent2 := "00000000-0000-4000-8000-000000000102"
	_, err = s.Confirm(ctx, ActorRenter2, q2.ID, true, intent2)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("q2 confirm on overlapping slot got %v, want ErrConflict", err)
	}

	// 3. Confirm q3 (created before q1 confirmed) fails with ErrConflict because q1's confirmation bumped the space revision
	intent3 := "00000000-0000-4000-8000-000000000103"
	_, err = s.Confirm(ctx, ActorRenter, q3.ID, true, intent3)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("q3 confirm with stale revision got %v, want ErrConflict", err)
	}

	// 4. A fresh quote q4 created after q1 confirmation captures the current revision and succeeds on the adjacent slot
	q4, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{
		SpaceID: space,
		StartAt: "2030-01-18T16:00:00Z",
		EndAt:   "2030-01-18T18:00:00Z",
	})
	if err != nil {
		t.Fatalf("fresh quote q4 failed: %v", err)
	}
	intent4 := "00000000-0000-4000-8000-000000000104"
	c4, err := s.Confirm(ctx, ActorRenter, q4.ID, true, intent4)
	if err != nil {
		t.Fatalf("q4 confirmation on adjacent non-overlapping slot failed: %v", err)
	}
	if c4.IntentID != intent4 {
		t.Errorf("q4 intentId = %q, want %q", c4.IntentID, intent4)
	}
}
