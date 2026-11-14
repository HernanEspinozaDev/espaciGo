package rentalflow

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testSpaceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testOwnerID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

type catalogStub struct {
	listing Listing
	err     error
	calls   int
	onRead  func()
}

func (s *catalogStub) PublicListing(context.Context, string) (Listing, error) {
	s.calls++
	if s.onRead != nil {
		s.onRead()
	}
	return s.listing, s.err
}

type calendarStub struct {
	intervals           []Interval
	available           bool
	listErr             error
	checkErr            error
	onList              func()
	onCheck             func()
	listCalls           int
	checkCalls          int
	lastCheckedInterval Interval
}

func (s *calendarStub) PublicIntervals(context.Context, Listing, string, int) ([]Interval, error) {
	s.listCalls++
	if s.onList != nil {
		s.onList()
	}
	return s.intervals, s.listErr
}

func (s *calendarStub) IntervalAvailable(_ context.Context, _ Listing, interval Interval) (bool, error) {
	s.checkCalls++
	s.lastCheckedInterval = interval
	if s.onCheck != nil {
		s.onCheck()
	}
	return s.available, s.checkErr
}

func TestPublicAvailabilityDoesNotStartWorkForCanceledContext(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	catalog := &catalogStub{}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = service.ListPublic(ctx, testSpaceID, "2030-05-11", 1)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled request error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 0 || calendar.listCalls != 0 {
		t.Fatalf("canceled request started work: listing=%d calendar=%d", catalog.calls, calendar.listCalls)
	}
}

func TestPublicAvailabilityStopsBeforeCalendarWhenContextIsCanceledByListingRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}, onRead: cancel}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.ListPublic(ctx, testSpaceID, "2030-05-11", 1)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled request error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 1 || calendar.listCalls != 0 {
		t.Fatalf("canceled request continued after listing read: listing=%d calendar=%d", catalog.calls, calendar.listCalls)
	}
}

func TestPublicAvailabilityMapsCanceledListingNotFoundAsUnavailable(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	catalog := &catalogStub{err: ErrNotFound, onRead: cancel}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.ListPublic(ctx, testSpaceID, "2030-05-11", 1)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled listing read error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 1 || calendar.listCalls != 0 {
		t.Fatalf("canceled listing read continued: listing=%d calendar=%d", catalog.calls, calendar.listCalls)
	}
}

func TestPublicAvailabilityDoesNotReturnIntervalsWhenContextIsCanceledByCalendarRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}}
	calendar := &calendarStub{intervals: []Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}, onList: cancel}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(ctx, testSpaceID, "2030-05-11", 1)
	cancellationMapped := errors.Is(err, ErrUnavailable)
	if !cancellationMapped {
		t.Errorf("canceled availability error=%v, want ErrUnavailable", err)
	}
	responseDiscarded := got.SpaceID == "" && got.TimeZone == "" && got.RateUnit == "" && got.CheckedAt.IsZero() && len(got.Items) == 0
	if !responseDiscarded {
		t.Errorf("cancellation returned partial public availability: %+v", got)
	}
	readsMatch := catalog.calls == 1 && calendar.listCalls == 1
	if !readsMatch {
		t.Errorf("reads before cancellation: listing=%d calendar=%d, want one each", catalog.calls, calendar.listCalls)
	}
	if !cancellationMapped || !responseDiscarded || !readsMatch {
		return
	}
}

func TestPublicAvailabilityRejectsInactivePublicationBeforeCalendarRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "borrador", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("inactive publication error=%v, want ErrNotFound", err)
	}
	if calendar.listCalls != 0 {
		t.Fatalf("calendar read calls=%d, want 0 for inactive publication", calendar.listCalls)
	}
}

func TestPublicAvailabilityRejectsUnsafeListingTitleBeforeCalendarRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala\nprivada", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsafe listing title error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 1 || calendar.listCalls != 0 {
		t.Fatalf("unsafe listing title continued downstream: listing=%d calendar=%d", catalog.calls, calendar.listCalls)
	}
}

func TestPublicAvailabilityHidesMissingPublicationBeforeCalendarRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	catalog := &catalogStub{
		listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "No divulgar publicación", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
		err:     ErrNotFound,
	}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	missingPublicationMapped := errors.Is(err, ErrNotFound)
	if !missingPublicationMapped {
		t.Errorf("missing publication error=%v, want ErrNotFound", err)
	}
	responseEmpty := got.SpaceID == "" && got.TimeZone == "" && got.RateUnit == "" && got.CheckedAt.IsZero() && len(got.Items) == 0
	if !responseEmpty {
		t.Errorf("missing publication returned partial metadata: %+v", got)
	}
	downstreamStopped := catalog.calls == 1 && calendar.listCalls == 0
	if !downstreamStopped {
		t.Errorf("missing publication reached later reads: listing=%d calendar=%d", catalog.calls, calendar.listCalls)
	}
	if !missingPublicationMapped || !responseEmpty || !downstreamStopped {
		return
	}
}

func TestPublicAvailabilityReturnsOnlyIntervalsForAnActiveListing(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	calendar := &calendarStub{intervals: []Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}}
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"}}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.SpaceID != testSpaceID || got.TimeZone != "America/Santiago" || got.RateUnit != "hora" || !got.CheckedAt.Equal(now) || len(got.Items) != 1 || !got.Items[0].StartAt.Equal(start) {
		t.Fatalf("public availability=%+v", got)
	}
}

func TestPublicAvailabilityFailsClosedOnMalformedListingSnapshot(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		mutate func(*Listing)
	}{
		{name: "invalid owner UUID", mutate: func(v *Listing) { v.OwnerID = "not-a-uuid" }},
		{name: "blank title", mutate: func(v *Listing) { v.Title = "" }},
		{name: "trimmed title", mutate: func(v *Listing) { v.Title = " Sala " }},
		{name: "oversized title", mutate: func(v *Listing) { v.Title = strings.Repeat("ñ", 71) }},
		{name: "unsupported rate unit", mutate: func(v *Listing) { v.RateUnit = "semana" }},
		{name: "price at minimum", mutate: func(v *Listing) { v.UnitPriceCLP = 5000 }},
		{name: "non-CLP currency", mutate: func(v *Listing) { v.Currency = "USD" }},
		{name: "missing rate version", mutate: func(v *Listing) { v.RateVersion = 0 }},
		{name: "missing timezone", mutate: func(v *Listing) { v.TimeZone = "" }},
		{name: "local timezone", mutate: func(v *Listing) { v.TimeZone = "Local" }},
		{name: "unknown timezone", mutate: func(v *Listing) { v.TimeZone = "Mars/Olympus" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"}
			tc.mutate(&listing)
			catalog := &catalogStub{listing: listing}
			calendar := &calendarStub{}
			service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
			if !errors.Is(err, ErrUnavailable) || len(got.Items) != 0 {
				t.Fatalf("malformed listing returned availability=%+v error=%v, want empty and ErrUnavailable", got, err)
			}
			if catalog.calls != 1 || calendar.listCalls != 0 {
				t.Fatalf("malformed listing calls: catalog=%d calendar=%d, want 1 and 0", catalog.calls, calendar.listCalls)
			}
		})
	}
}

func TestPublicAvailabilityRejectsMalformedSpaceAndDateBeforeReadingDependencies(t *testing.T) {
	nowCalls := 0
	service, err := NewAvailabilityService(&catalogStub{}, &calendarStub{}, func() time.Time {
		nowCalls++
		return time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		spaceID string
		date    string
	}{
		{name: "malformed space ID", spaceID: "not-a-uuid", date: "2030-05-11"},
		{name: "noncanonical date", spaceID: testSpaceID, date: "2030-5-11"},
		{name: "impossible date", spaceID: testSpaceID, date: "2030-02-30"},
		{name: "timestamp instead of date", spaceID: testSpaceID, date: "2030-05-11T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.ListPublic(context.Background(), tc.spaceID, tc.date, 1)
			if !errors.Is(err, ErrInvalid) || len(got.Items) != 0 {
				t.Fatalf("invalid public availability input=%+v error=%v, want empty and ErrInvalid", got, err)
			}
		})
	}
	if nowCalls != 0 {
		t.Fatalf("clock calls=%d, want 0 for malformed inputs", nowCalls)
	}
}

func TestPublicAvailabilitySortsServerIntervalsByStartTime(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	later := time.Date(2030, time.May, 11, 16, 0, 0, 0, time.UTC)
	earlier := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	calendar := &calendarStub{intervals: []Interval{{StartAt: later, EndAt: later.Add(time.Hour)}, {StartAt: earlier, EndAt: earlier.Add(time.Hour)}}}
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"}}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || !got.Items[0].StartAt.Equal(earlier) || !got.Items[1].StartAt.Equal(later) {
		t.Fatalf("public intervals not sorted by start: %+v", got.Items)
	}
}

func TestPublicAvailabilityPreservesRepeatedWallTimeSlotsByInstant(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	firstStart, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-04:00")
	if err != nil {
		t.Fatal(err)
	}
	secondStart, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-05:00")
	if err != nil {
		t.Fatal(err)
	}
	calendar := &calendarStub{intervals: []Interval{
		{StartAt: secondStart, EndAt: secondStart.Add(time.Hour)},
		{StartAt: firstStart, EndAt: firstStart.Add(time.Hour)},
	}}
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2, TimeZone: loc.String()}
	service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2026-11-01", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || !got.Items[0].StartAt.Equal(firstStart.UTC()) || !got.Items[1].StartAt.Equal(secondStart.UTC()) ||
		got.Items[0].StartAt.In(loc).Format(time.RFC3339) != "2026-11-01T01:30:00-04:00" ||
		got.Items[1].StartAt.In(loc).Format(time.RFC3339) != "2026-11-01T01:30:00-05:00" {
		t.Fatalf("repeated local slots were not both preserved in instant order: %+v", got.Items)
	}
}

func TestPublicAvailabilityFailsClosedOnDuplicateOrNonCanonicalIntervals(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	valid := Interval{StartAt: start, EndAt: start.Add(time.Hour)}
	alternateOffset := time.FixedZone("synthetic-plus-two", 2*60*60)
	sameInstantDifferentOffset := Interval{StartAt: valid.StartAt.In(alternateOffset), EndAt: valid.EndAt.In(alternateOffset)}
	for _, tc := range []struct {
		name      string
		intervals []Interval
	}{
		{name: "duplicate interval", intervals: []Interval{valid, valid}},
		{name: "duplicate instant with alternate offset", intervals: []Interval{valid, sameInstantDifferentOffset}},
		{name: "wrong duration", intervals: []Interval{{StartAt: start, EndAt: start.Add(90 * time.Minute)}}},
		{name: "off-grid start", intervals: []Interval{{StartAt: start.Add(15 * time.Minute), EndAt: start.Add(75 * time.Minute)}}},
		{name: "wrong local date", intervals: []Interval{{StartAt: start.AddDate(0, 0, 1), EndAt: start.AddDate(0, 0, 1).Add(time.Hour)}}},
		{name: "subsecond start", intervals: []Interval{{StartAt: start.Add(time.Nanosecond), EndAt: start.Add(time.Hour + time.Nanosecond)}}},
		{name: "zero start", intervals: []Interval{{StartAt: time.Time{}, EndAt: start}}},
		{name: "zero end", intervals: []Interval{{StartAt: start, EndAt: time.Time{}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"}
			calendar := &calendarStub{intervals: tc.intervals}
			service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
			if !errors.Is(err, ErrUnavailable) || len(got.Items) != 0 {
				t.Fatalf("invalid calendar response=%+v error=%v, want empty response and ErrUnavailable", got, err)
			}
		})
	}
}

func TestPublicAvailabilityUsesSantiagoCalendarDaysAndMonthsAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		rateUnit string
		now      time.Time
		date     string
		duration int
		start    time.Time
		end      time.Time
		elapsed  time.Duration
	}{
		{
			name: "day crossing skipped midnight", rateUnit: "dia",
			now: time.Date(2026, time.September, 5, 12, 0, 0, 0, loc), date: "2026-09-06", duration: 1,
			start: time.Date(2026, time.September, 6, 1, 0, 0, 0, loc),
			end:   time.Date(2026, time.September, 7, 0, 0, 0, 0, loc), elapsed: 23 * time.Hour,
		},
		{
			name: "day crossing repeated hour", rateUnit: "dia",
			now: time.Date(2026, time.March, 10, 12, 0, 0, 0, loc), date: "2026-04-04", duration: 1,
			start: time.Date(2026, time.April, 4, 0, 0, 0, 0, loc),
			end:   time.Date(2026, time.April, 5, 0, 0, 0, 0, loc), elapsed: 25 * time.Hour,
		},
		{
			name: "month clamped from January 31", rateUnit: "mes",
			now: time.Date(2026, time.January, 30, 12, 0, 0, 0, loc), date: "2026-01-31", duration: 1,
			start: time.Date(2026, time.January, 31, 0, 0, 0, 0, loc),
			end:   time.Date(2026, time.February, 28, 0, 0, 0, 0, loc),
		},
		{
			name: "month clamped to leap day", rateUnit: "mes",
			now: time.Date(2028, time.January, 30, 12, 0, 0, 0, loc), date: "2028-01-31", duration: 1,
			start: time.Date(2028, time.January, 31, 0, 0, 0, 0, loc),
			end:   time.Date(2028, time.February, 29, 0, 0, 0, 0, loc),
		},
		{
			name: "month ending at first instant after skipped midnight", rateUnit: "mes",
			now: time.Date(2026, time.August, 1, 12, 0, 0, 0, loc), date: "2026-08-06", duration: 1,
			start: time.Date(2026, time.August, 6, 0, 0, 0, 0, loc),
			end:   time.Date(2026, time.September, 6, 1, 0, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: tc.rateUnit, UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 3, TimeZone: loc.String()}
			interval := Interval{StartAt: tc.start.UTC(), EndAt: tc.end.UTC()}
			calendar := &calendarStub{intervals: []Interval{interval}}
			service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return tc.now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.ListPublic(context.Background(), testSpaceID, tc.date, tc.duration)
			if err != nil {
				t.Fatalf("calendar-unit availability rejected: %v", err)
			}
			if len(got.Items) != 1 || !got.Items[0].StartAt.Equal(interval.StartAt) || !got.Items[0].EndAt.Equal(interval.EndAt) {
				t.Fatalf("calendar-unit availability=%+v", got)
			}
			if tc.elapsed != 0 && tc.end.Sub(tc.start) != tc.elapsed {
				t.Fatalf("elapsed duration=%v, want %v", tc.end.Sub(tc.start), tc.elapsed)
			}
		})
	}
}

func TestPublicAvailabilityHorizonUsesCivilDaysAcrossDSTAndRejectsDay91(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.June, 8, 12, 0, 0, 0, loc)
	if horizonDate := now.AddDate(0, 0, 90).Format("2006-01-02"); horizonDate != "2026-09-06" {
		t.Fatalf("synthetic 90-day horizon date=%s, want date crossing Santiago DST transition", horizonDate)
	}
	for _, tc := range []struct {
		days      int
		wantErr   error
		wantReads int
	}{
		{days: 0, wantReads: 1},
		{days: 90, wantReads: 1},
		{days: -1, wantErr: ErrInvalid, wantReads: 0},
		{days: 91, wantErr: ErrInvalid, wantReads: 0},
	} {
		t.Run(strconv.Itoa(tc.days), func(t *testing.T) {
			date := now.AddDate(0, 0, tc.days)
			dateText := date.Format("2006-01-02")
			start := time.Date(date.Year(), date.Month(), date.Day(), 15, 0, 0, 0, loc)
			calendar := &calendarStub{intervals: []Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}}
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: loc.String()}
			service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.ListPublic(context.Background(), testSpaceID, dateText, 1)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(got.Items) != 0 {
					t.Fatalf("day-%d availability=%+v error=%v, want %v", tc.days, got, err, tc.wantErr)
				}
			} else if err != nil || len(got.Items) != 1 {
				t.Fatalf("day-%d availability=%+v error=%v, want one interval", tc.days, got, err)
			}
			if calendar.listCalls != tc.wantReads {
				t.Fatalf("day-%d calendar reads=%d, want %d", tc.days, calendar.listCalls, tc.wantReads)
			}
		})
	}
}

func TestPublicAvailabilityUsesListingLocalDateAcrossUTCDateBoundary(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, time.May, 10, 0, 30, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 9, 18, 0, 0, 0, loc)
	calendar := &calendarStub{intervals: []Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}}
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: loc.String()}
	service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-09", 1)
	if err != nil {
		t.Fatalf("local date still in progress after UTC midnight rejected: %v", err)
	}
	if len(got.Items) != 1 || !got.Items[0].StartAt.Equal(start.UTC()) || calendar.listCalls != 1 {
		t.Fatalf("availability=%+v calendar reads=%d; want the future local-day interval", got, calendar.listCalls)
	}
}

func TestPublicAvailabilityAcceptsOnlyCanonicalUnitDurations(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		unit     string
		duration int
		valid    bool
	}{
		{unit: "hora", duration: 1, valid: true},
		{unit: "hora", duration: 2, valid: true},
		{unit: "hora", duration: 3},
		{unit: "hora", duration: 4, valid: true},
		{unit: "dia", duration: 1, valid: true},
		{unit: "dia", duration: 2, valid: true},
		{unit: "dia", duration: 3, valid: true},
		{unit: "dia", duration: 4},
		{unit: "mes", duration: 1, valid: true},
		{unit: "mes", duration: 2},
	}
	for _, tc := range cases {
		t.Run(tc.unit+"-"+strconv.Itoa(tc.duration), func(t *testing.T) {
			calendar := &calendarStub{}
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: tc.unit, UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
			service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", tc.duration)
			if tc.valid {
				if err != nil || len(got.Items) != 0 {
					t.Fatalf("canonical %s duration %d rejected: result=%+v error=%v", tc.unit, tc.duration, got, err)
				}
				if calendar.listCalls != 1 {
					t.Fatalf("canonical duration reached calendar %d times, want 1", calendar.listCalls)
				}
			} else if !errors.Is(err, ErrInvalid) || calendar.listCalls != 0 {
				t.Fatalf("noncanonical %s duration %d: error=%v calendarReads=%d", tc.unit, tc.duration, err, calendar.listCalls)
			}
		})
	}
}

func TestPublicAvailabilityFailsClosedWhenCalendarReaderErrors(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	calendarFailure := errors.New("private calendar backend detail")
	calendar := &calendarStub{
		intervals: []Interval{{StartAt: time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC), EndAt: time.Date(2030, time.May, 11, 16, 0, 0, 0, time.UTC)}},
		listErr:   calendarFailure,
	}
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	errorMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, calendarFailure)
	if !errorMapped {
		t.Errorf("calendar failure error=%v, want generic ErrUnavailable", err)
	}
	privateDetailHidden := err == nil || !strings.Contains(err.Error(), calendarFailure.Error())
	if !privateDetailHidden {
		t.Errorf("private calendar detail was exposed in error %q", err.Error())
	}
	emptyResponse := got.SpaceID == "" && got.TimeZone == "" && got.RateUnit == "" && got.CheckedAt.IsZero() && len(got.Items) == 0
	if !emptyResponse {
		t.Errorf("calendar failure returned partial public availability: %+v", got)
	}
	calendarReadCountMatches := calendar.listCalls == 1
	if !calendarReadCountMatches {
		t.Errorf("calendar reads=%d, want 1", calendar.listCalls)
	}
	if !errorMapped || !privateDetailHidden || !emptyResponse || !calendarReadCountMatches {
		return
	}
}

func TestPublicAvailabilityFailsClosedWhenListingReaderErrors(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	listingFailure := errors.New("private listing backend detail")
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing, err: listingFailure}
	calendar := &calendarStub{}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	listingFailureMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, listingFailure)
	emptyResult := got.SpaceID == "" && got.TimeZone == "" && got.RateUnit == "" && got.CheckedAt.IsZero() && len(got.Items) == 0
	if !listingFailureMapped || !emptyResult {
		t.Errorf("listing failure response=%+v error=%v, want empty result with generic ErrUnavailable", got, err)
	}
	listingFailureStoppedFlow := catalog.calls == 1 && calendar.listCalls == 0
	if !listingFailureStoppedFlow {
		t.Errorf("listing failure calls: catalog=%d calendar=%d, want 1 and 0", catalog.calls, calendar.listCalls)
	}
	if !listingFailureMapped || !emptyResult || !listingFailureStoppedFlow {
		return
	}
}
