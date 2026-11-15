package rentaldemo

import (
	"context"
	"testing"
	"time"
)

func TestMonthReturnsTomorrowHourlyCandidates(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	s, err := NewStore(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Month(context.Background(), ActorRenter, SpaceIDHour, "2026-11", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got.Days {
		if d.Date == "2026-11-15" {
			if len(d.Intervals) == 0 {
				t.Fatalf("tomorrow has no hourly candidates: %+v", d)
			}
			return
		}
	}
	t.Fatal("tomorrow missing")
}

func TestMonthHostBlockRemovesPublicSlotAndUnblockRestores(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	before, _ := s.Month(ctx, ActorRenter, SpaceIDHour, "2026-11", 1, false)
	start := time.Date(2026, 11, 15, 11, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	b, err := s.Block(ctx, ActorHost, SpaceIDHour, "1", start.Format(time.RFC3339), end.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2026-11", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Days[14].Intervals) <= len(blocked.Days[14].Intervals) {
		t.Fatalf("block didn't remove public slot: %d => %d", len(before.Days[14].Intervals), len(blocked.Days[14].Intervals))
	}
	if blocked.Blocks != nil {
		t.Fatal("public month exposed blocks")
	}
	if err = s.Unblock(ctx, ActorHost, SpaceIDHour, b.ID, "2"); err != nil {
		t.Fatal(err)
	}
	restored, _ := s.Month(ctx, ActorRenter, SpaceIDHour, "2026-11", 1, false)
	if len(restored.Days[14].Intervals) != len(before.Days[14].Intervals) {
		t.Fatal("slot not restored")
	}
}

func TestQuoteRejectsNonCandidateIntervals(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	cases := []struct{ name, start, end string }{{"90m", "2026-11-15T10:00:00Z", "2026-11-15T11:30:00Z"}, {"quarter", "2026-11-15T10:15:00Z", "2026-11-15T11:15:00Z"}, {"outside-hours", "2026-11-15T06:00:00Z", "2026-11-15T07:00:00Z"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceID: SpaceIDHour, StartAt: tc.start, EndAt: tc.end})
			if err == nil {
				t.Fatal("accepted non-candidate interval")
			}
		})
	}
}

func TestMonthOwnerPrivilegeAndBlockIsolation(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	// Non-host caller requesting owner=true must be rejected
	if _, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2026-11", 1, true); err != ErrForbidden {
		t.Fatalf("non-host owner month accepted: %v", err)
	}
	// Host caller requesting owner=true receives revision and initialized blocks slice
	hostMonth, err := s.Month(ctx, ActorHost, SpaceIDHour, "2026-11", 1, true)
	if err != nil {
		t.Fatalf("host owner month failed: %v", err)
	}
	if hostMonth.Revision == "" || hostMonth.Blocks == nil {
		t.Fatalf("host month missing revision or blocks: %+v", hostMonth)
	}
	// Public caller receives empty revision and nil blocks
	publicMonth, err := s.Month(ctx, ActorRenter, SpaceIDHour, "2026-11", 1, false)
	if err != nil {
		t.Fatalf("public month failed: %v", err)
	}
	if publicMonth.Revision != "" || publicMonth.Blocks != nil {
		t.Fatalf("public month leaked revision or blocks: %+v", publicMonth)
	}
}
