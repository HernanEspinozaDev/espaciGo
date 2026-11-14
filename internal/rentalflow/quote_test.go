package rentalflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const testRenterID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

func TestCreateQuoteDoesNotStartWorkForCanceledContext(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = service.Create(ctx, testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled request error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 0 || calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("canceled request started work: listing=%d availability=%d quote_write=%d", catalog.calls, calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteDoesNotWriteWhenContextIsCanceledDuringAvailabilityCheck(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}}
	ctx, cancel := context.WithCancel(context.Background())
	calendar := &calendarStub{available: true, onCheck: cancel}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(ctx, testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled request error=%v, want ErrUnavailable", err)
	}
	if calendar.checkCalls != 1 || quotes.calls != 0 {
		t.Fatalf("cancellation after availability started quote write: availability=%d quote_write=%d", calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteStopsBeforeAvailabilityWhenContextIsCanceledByListingRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}, onRead: cancel}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(ctx, testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled request error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 1 || calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("canceled request continued after listing read: listing=%d availability=%d quote_write=%d", catalog.calls, calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteMapsCanceledListingNotFoundAsUnavailable(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	catalog := &catalogStub{err: ErrNotFound, onRead: cancel}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(ctx, testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled listing read error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 1 || calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("canceled listing read continued: listing=%d availability=%d quote_write=%d", catalog.calls, calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteMapsCanceledConflictFromQuoteWriteAsUnavailable(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	catalog := &catalogStub{listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{
		result:   Quote{ID: testQuoteID, SpaceID: testSpaceID, SpaceTitle: "partial-canceled-quote-secret", SubtotalCLP: 456789},
		err:      ErrConflict,
		onCreate: cancel,
	}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(ctx, testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	cancellationMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, ErrConflict)
	if !cancellationMapped {
		t.Errorf("canceled quote write error=%v, want ErrUnavailable", err)
	}
	quoteDiscarded := got == (Quote{})
	if !quoteDiscarded {
		t.Errorf("canceled conflict returned partial quote: %+v", got)
	}
	writeAttemptObserved := quotes.calls == 1 && calendar.checkCalls == 1
	if !writeAttemptObserved {
		t.Errorf("quote write path calls: quote=%d availability=%d", quotes.calls, calendar.checkCalls)
	}
	if !cancellationMapped || !quoteDiscarded || !writeAttemptObserved {
		return
	}
}

type quoteRepositoryStub struct {
	result   Quote
	err      error
	calls    int
	input    Quote
	onCreate func()
}

func (s *quoteRepositoryStub) CreateValidatedQuote(_ context.Context, renterID string, listing Listing, interval Interval, units, subtotal int64, createdAt time.Time) (Quote, error) {
	s.calls++
	s.input = Quote{RenterID: renterID, SpaceID: listing.ID, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: units, SubtotalCLP: subtotal, StartAt: interval.StartAt, EndAt: interval.EndAt, TimeZone: listing.TimeZone, CreatedAt: createdAt}
	if s.onCreate != nil {
		s.onCreate()
	}
	return s.result, s.err
}

func (s *quoteRepositoryStub) GetForParticipant(context.Context, string, string) (Quote, error) {
	return Quote{}, ErrNotFound
}

func TestCreateQuoteRejectsUnavailableIntervalWithoutCreatingQuote(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: false}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	intervalRejected := errors.Is(err, ErrConflict)
	if !intervalRejected {
		t.Errorf("unavailable interval error=%v, want ErrConflict", err)
	}
	availabilityChecked := calendar.checkCalls == 1
	if !availabilityChecked {
		t.Errorf("availability checks=%d, want 1", calendar.checkCalls)
	}
	quoteWriteBlocked := quotes.calls == 0
	if !quoteWriteBlocked {
		t.Errorf("quote repository calls=%d, want 0 when interval is unavailable", quotes.calls)
	}
	if !intervalRejected || !availabilityChecked || !quoteWriteBlocked {
		return
	}
}

func TestCreateQuoteRejectsIntervalsBeyondPublicAvailabilityHorizon(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.September, 15, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("out-of-horizon quote error=%v, want ErrInvalid", err)
	}
	if calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("out-of-horizon request reached availability/repository: calendar=%d quote=%d", calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteSnapshotsCurrentCLPRateAndListingTimeZone(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 30, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	result := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "America/Santiago", CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{result: result}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != result.ID || got.SubtotalCLP != 8000 || got.Currency != "CLP" || got.TimeZone != "America/Santiago" {
		t.Fatalf("quote response=%+v", got)
	}
	if quotes.calls != 1 || quotes.input.RateVersion != 7 || quotes.input.Units != 1 || quotes.input.SubtotalCLP != 8000 || quotes.input.TimeZone != "America/Santiago" {
		t.Fatalf("authoritative quote snapshot=%+v calls=%d", quotes.input, quotes.calls)
	}
}

func TestCreateQuoteAllowsRTLTitleWithoutBidiControls(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 30, 0, 0, time.UTC)
	title := "قاعة عمل"
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: title, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	result := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{result: result}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("ordinary RTL title was rejected: %v", err)
	}
	if got.SpaceTitle != title || quotes.calls != 1 || calendar.checkCalls != 1 {
		t.Fatalf("RTL title or synthetic quote flow changed: quote=%+v quoteWrites=%d availabilityChecks=%d", got, quotes.calls, calendar.checkCalls)
	}
}

func TestCreateHourlyQuoteUsesElapsedTimeAcrossDSTTransitions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		now   time.Time
		start string
		end   string
	}{
		{
			name:  "fall repeated local hour",
			now:   time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
			start: "2026-11-01T01:30:00-04:00",
			end:   "2026-11-01T01:30:00-05:00",
		},
		{
			name:  "spring skipped local hour",
			now:   time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
			start: "2026-03-08T01:30:00-05:00",
			end:   "2026-03-08T03:30:00-04:00",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tc.start)
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339, tc.end)
			if err != nil {
				t.Fatal(err)
			}
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/New_York"}
			result := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: listing.ID, OwnerID: listing.OwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start.UTC(), EndAt: end.UTC(), TimeZone: listing.TimeZone, CreatedAt: tc.now, ExpiresAt: tc.now.Add(5 * time.Minute)}
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{result: result}
			service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return tc.now })
			if err != nil {
				t.Fatal(err)
			}
			got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: listing.ID, StartAt: tc.start, EndAt: tc.end})
			if err != nil {
				t.Fatalf("Create() error=%v", err)
			}
			if got.Units != 1 || got.SubtotalCLP != 8000 || quotes.calls != 1 || calendar.checkCalls != 1 ||
				!quotes.input.StartAt.Equal(start.UTC()) || !quotes.input.EndAt.Equal(end.UTC()) {
				t.Fatalf("quote=%+v repository snapshot=%+v repository calls=%d availability checks=%d; want one elapsed hour at CLP 8000", got, quotes.input, quotes.calls, calendar.checkCalls)
			}
		})
	}
}

func TestCreateQuoteUsesListingLocalDateAcrossUTCDateBoundary(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, time.May, 10, 0, 30, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 9, 18, 0, 0, 0, loc)
	end := start.Add(time.Hour)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: loc.String()}
	result := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: listing.ID, OwnerID: listing.OwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1, SubtotalCLP: 8000, StartAt: start.UTC(), EndAt: end.UTC(), TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{result: result}
	service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), testRenterID, QuoteRequest{
		SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: end.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("future listing-local slot on previous UTC date rejected: %v", err)
	}
	if got.Units != 1 || got.SubtotalCLP != 8000 || calendar.checkCalls != 1 || quotes.calls != 1 ||
		!quotes.input.StartAt.Equal(start.UTC()) || !quotes.input.EndAt.Equal(end.UTC()) {
		t.Fatalf("quote=%+v snapshot=%+v availability=%d writes=%d; want one local-calendar hour", got, quotes.input, calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteRejectsMismatchedRepositorySnapshots(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		mutate func(*Quote)
	}{
		{name: "malformed quote identifier", mutate: func(quote *Quote) { quote.ID = "not-a-uuid" }},
		{name: "wrong space", mutate: func(quote *Quote) { quote.SpaceID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee" }},
		{name: "wrong owner", mutate: func(quote *Quote) { quote.OwnerID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee" }},
		{name: "wrong renter", mutate: func(quote *Quote) { quote.RenterID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee" }},
		{name: "wrong title", mutate: func(quote *Quote) { quote.SpaceTitle += " alterado" }},
		{name: "stale rate revision", mutate: func(quote *Quote) { quote.RateVersion++ }},
		{name: "wrong rate unit", mutate: func(quote *Quote) { quote.RateUnit = "dia" }},
		{name: "wrong unit price", mutate: func(quote *Quote) { quote.UnitPriceCLP++ }},
		{name: "wrong currency", mutate: func(quote *Quote) { quote.Currency = "USD" }},
		{name: "wrong billed units", mutate: func(quote *Quote) { quote.Units++ }},
		{name: "wrong subtotal", mutate: func(quote *Quote) { quote.SubtotalCLP++ }},
		{name: "wrong start", mutate: func(quote *Quote) { quote.StartAt = quote.StartAt.Add(30 * time.Minute) }},
		{name: "wrong end", mutate: func(quote *Quote) { quote.EndAt = quote.EndAt.Add(30 * time.Minute) }},
		{name: "wrong time zone", mutate: func(quote *Quote) { quote.TimeZone = "UTC" }},
		{name: "missing creation timestamp", mutate: func(quote *Quote) { quote.CreatedAt = time.Time{} }},
		{name: "creation timestamp in future", mutate: func(quote *Quote) { quote.CreatedAt = now.Add(time.Nanosecond) }},
		{name: "already expired", mutate: func(quote *Quote) { quote.ExpiresAt = now }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
			result := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
			tc.mutate(&result)
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{result: result}
			service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
			invalidSnapshotMapped := errors.Is(err, ErrUnavailable)
			if !invalidSnapshotMapped {
				t.Errorf("invalid repository quote error=%v, want ErrUnavailable", err)
			}
			snapshotDiscarded := got == (Quote{})
			if !snapshotDiscarded {
				t.Errorf("invalid repository snapshot returned partial quote: %+v", got)
			}
			dependenciesMatched := quotes.calls == 1 && calendar.checkCalls == 1
			if !dependenciesMatched {
				t.Errorf("repository snapshot validation calls: quote=%d calendar=%d", quotes.calls, calendar.checkCalls)
			}
			if !invalidSnapshotMapped || !snapshotDiscarded || !dependenciesMatched {
				return
			}
		})
	}
}

func TestCreateQuoteUsesSantiagoCalendarDaysAndMonthsAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		unit        string
		now         time.Time
		start       time.Time
		end         time.Time
		elapsedWant time.Duration
	}{
		{
			name: "one local day spanning skipped midnight", unit: "dia",
			now:   time.Date(2026, time.September, 5, 12, 0, 0, 0, loc),
			start: time.Date(2026, time.September, 6, 1, 0, 0, 0, loc),
			end:   time.Date(2026, time.September, 7, 0, 0, 0, 0, loc), elapsedWant: 23 * time.Hour,
		},
		{
			name: "one local day spanning repeated hour", unit: "dia",
			now:   time.Date(2026, time.March, 10, 12, 0, 0, 0, loc),
			start: time.Date(2026, time.April, 4, 0, 0, 0, 0, loc),
			end:   time.Date(2026, time.April, 5, 0, 0, 0, 0, loc), elapsedWant: 25 * time.Hour,
		},
		{
			name: "one local month clamped from January 31", unit: "mes",
			now:   time.Date(2026, time.January, 30, 12, 0, 0, 0, loc),
			start: time.Date(2026, time.January, 31, 0, 0, 0, 0, loc),
			end:   time.Date(2026, time.February, 28, 0, 0, 0, 0, loc),
		},
		{
			name: "one local month clamped to leap day", unit: "mes",
			now:   time.Date(2028, time.January, 30, 12, 0, 0, 0, loc),
			start: time.Date(2028, time.January, 31, 0, 0, 0, 0, loc),
			end:   time.Date(2028, time.February, 29, 0, 0, 0, 0, loc),
		},
		{
			name: "one calendar month ending at first instant after skipped midnight", unit: "mes",
			now:   time.Date(2026, time.August, 1, 12, 0, 0, 0, loc),
			start: time.Date(2026, time.August, 6, 0, 0, 0, 0, loc),
			end:   time.Date(2026, time.September, 6, 1, 0, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: tc.unit, UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 3, TimeZone: loc.String()}
			result := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: listing.ID, OwnerID: listing.OwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: tc.start.UTC(), EndAt: tc.end.UTC(), TimeZone: listing.TimeZone, CreatedAt: tc.now, ExpiresAt: tc.now.Add(5 * time.Minute)}
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{result: result}
			service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return tc.now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: tc.start.Format(time.RFC3339), EndAt: tc.end.Format(time.RFC3339)})
			if err != nil {
				t.Fatalf("calendar-unit quote rejected: %v", err)
			}
			if got.Units != 1 || got.SubtotalCLP != 12000 || !got.StartAt.Equal(tc.start.UTC()) || !got.EndAt.Equal(tc.end.UTC()) {
				t.Fatalf("calendar-unit quote=%+v", got)
			}
			if tc.elapsedWant != 0 && tc.end.Sub(tc.start) != tc.elapsedWant {
				t.Fatalf("elapsed duration=%v, want %v", tc.end.Sub(tc.start), tc.elapsedWant)
			}
		})
	}
}

func TestCreateQuoteRejectsFixed24HourDayAcrossSantiagoDST(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, loc)
	start := time.Date(2026, time.September, 6, 1, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "dia", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 3, TimeZone: loc.String()}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: end.Format(time.RFC3339)})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("fixed-24-hour calendar-day quote error=%v, want ErrInvalid", err)
	}
	if calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("fixed-24-hour day reached availability/repository: calendar=%d quote=%d", calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteRejectsHourlyStartOffTheCalendarHalfHourGrid(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 15, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("off-grid hourly quote error=%v, want ErrInvalid", err)
	}
	if calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("off-grid interval reached availability/repository: calendar=%d quote=%d", calendar.checkCalls, quotes.calls)
	}
}

func TestQuoteWireRepresentationDoesNotExposeRateVersion(t *testing.T) {
	quote := Quote{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", RateVersion: 17}
	encoded, err := json.Marshal(quote)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "rate_version") {
		t.Fatalf("quote wire representation exposes internal rate revision: %s", encoded)
	}
}

func TestCreateQuoteRejectsCLPSubtotalOverflowBeforeAvailabilityOrPersistence(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 9223372036854775807, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(2 * time.Hour).Format(time.RFC3339)})
	overflowRejected := errors.Is(err, ErrInvalid)
	if !overflowRejected {
		t.Errorf("overflowing subtotal error=%v, want ErrInvalid", err)
	}
	overflowSideEffectsBlocked := calendar.checkCalls == 0 && quotes.calls == 0
	if !overflowSideEffectsBlocked {
		t.Errorf("overflowing subtotal reached availability/repository: calendar=%d quote=%d", calendar.checkCalls, quotes.calls)
	}
	if !overflowRejected || !overflowSideEffectsBlocked {
		return
	}
}

func TestCreateQuoteRejectsOwnerRentingOwnListingBeforeAvailability(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testRenterID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	selfRentalRejected := errors.Is(err, ErrNotFound)
	if !selfRentalRejected {
		t.Errorf("owner self-rental error=%v, want ErrNotFound", err)
	}
	selfRentalSideEffectsBlocked := calendar.checkCalls == 0 && quotes.calls == 0
	if !selfRentalSideEffectsBlocked {
		t.Errorf("owner self-rental reached availability/repository: calendar=%d quote=%d", calendar.checkCalls, quotes.calls)
	}
	if !selfRentalRejected || !selfRentalSideEffectsBlocked {
		return
	}
}

func TestCreateQuoteFailsClosedWhenCalendarReaderErrors(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	calendarFailure := errors.New("private calendar backend detail")
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	calendar := &calendarStub{available: true, checkErr: calendarFailure}
	quotes := &quoteRepositoryStub{result: Quote{
		ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID,
		SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit,
		UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1, SubtotalCLP: listing.UnitPriceCLP,
		StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone,
		CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}}
	service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	calendarFailureMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, calendarFailure)
	if !calendarFailureMapped {
		t.Errorf("calendar failure error=%v, want generic ErrUnavailable", err)
	}
	privateDetailHidden := err == nil || !strings.Contains(err.Error(), calendarFailure.Error())
	if !privateDetailHidden {
		t.Errorf("private calendar detail was exposed in error %q", err.Error())
	}
	quoteDiscarded := got == (Quote{})
	if !quoteDiscarded {
		t.Errorf("calendar failure returned partial quote: %+v", got)
	}
	calendarFailedClosed := calendar.checkCalls == 1 && quotes.calls == 0
	if !calendarFailedClosed {
		t.Errorf("calendar failure proceeded to quote persistence: calendar=%d quote=%d", calendar.checkCalls, quotes.calls)
	}
	if !calendarFailureMapped || !privateDetailHidden || !quoteDiscarded || !calendarFailedClosed {
		return
	}
}

func TestCreateQuoteFailsClosedWhenListingReaderErrors(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listingFailure := errors.New("private listing backend detail")
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing, err: listingFailure}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
	listingFailureMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, listingFailure)
	if !listingFailureMapped {
		t.Errorf("listing failure error=%v, want generic ErrUnavailable", err)
	}
	listingFailureStoppedFlow := catalog.calls == 1 && calendar.checkCalls == 0 && quotes.calls == 0
	if !listingFailureStoppedFlow {
		t.Errorf("listing failure proceeded past catalog: catalog=%d calendar=%d quote=%d", catalog.calls, calendar.checkCalls, quotes.calls)
	}
	if !listingFailureMapped || !listingFailureStoppedFlow {
		return
	}
}

func TestCreateQuoteStopsWhenListingIsMissingOrMalformed(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	validListing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}
	cases := []struct {
		name       string
		listing    Listing
		title      string
		catalogErr error
		wantErr    error
	}{
		{name: "publication missing", catalogErr: ErrNotFound, wantErr: ErrNotFound},
		{name: "malformed listing time zone", listing: Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "Local"}, wantErr: ErrUnavailable},
		{name: "title with control character", title: "Sala\nprivada", wantErr: ErrUnavailable},
		{name: "title with Unicode line separator", title: "Sala\u2028privada", wantErr: ErrUnavailable},
		{name: "title with Unicode paragraph separator", title: "Sala\u2029privada", wantErr: ErrUnavailable},
		{name: "title with bidi control", title: "Sala\u202eprivada", wantErr: ErrUnavailable},
		{name: "title with invalid UTF-8", title: string([]byte{'S', 'a', 'l', 'a', 0xff}), wantErr: ErrUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listing := tc.listing
			if listing.ID == "" {
				listing = validListing
			}
			if tc.title != "" {
				listing.Title = tc.title
			}
			catalog := &catalogStub{listing: listing, err: tc.catalogErr}
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{}
			service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
			errorMatches := errors.Is(err, tc.wantErr)
			if !errorMatches {
				t.Errorf("listing result error=%v, want %v", err, tc.wantErr)
			}
			quoteDiscarded := got == (Quote{})
			if !quoteDiscarded {
				t.Errorf("listing failure returned partial quote: %+v", got)
			}
			downstreamStopped := catalog.calls == 1 && calendar.checkCalls == 0 && quotes.calls == 0
			if !downstreamStopped {
				t.Errorf("listing failure continued downstream: catalog=%d calendar=%d quote=%d", catalog.calls, calendar.checkCalls, quotes.calls)
			}
			if !errorMatches || !quoteDiscarded || !downstreamStopped {
				return
			}
		})
	}
}

func TestCreateQuoteMapsAtomicRepositoryOutcomesWithoutReturningPartialQuote(t *testing.T) {
	privateFailure := errors.New("private quote repository detail")
	cases := []struct {
		name     string
		repoErr  error
		wantErr  error
		contains string
	}{
		{name: "availability changed atomically", repoErr: ErrConflict, wantErr: ErrConflict},
		{name: "publication disappeared atomically", repoErr: ErrNotFound, wantErr: ErrNotFound},
		{name: "repository unavailable", repoErr: privateFailure, wantErr: ErrUnavailable, contains: privateFailure.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
			start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{
				result: Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: "partial-repository-quote-secret", SubtotalCLP: 987654},
				err:    tc.repoErr,
			}
			service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
			errorMapped := errors.Is(err, tc.wantErr)
			if !errorMapped {
				t.Errorf("repository outcome error=%v, want %v", err, tc.wantErr)
			}
			partialQuoteDiscarded := got == (Quote{})
			if !partialQuoteDiscarded {
				t.Errorf("repository error returned partial quote: %+v", got)
			}
			revalidationCompleted := quotes.calls == 1 && calendar.checkCalls == 1
			if !revalidationCompleted {
				t.Errorf("repository calls=%d calendar calls=%d; want one write attempt after one availability recheck", quotes.calls, calendar.checkCalls)
			}
			privateErrorHidden := tc.contains == "" || err == nil || !strings.Contains(err.Error(), tc.contains)
			if !privateErrorHidden {
				t.Errorf("repository detail %q was exposed in error %q", tc.contains, err.Error())
			}
			if !errorMapped || !partialQuoteDiscarded || !revalidationCompleted || !privateErrorHidden {
				return
			}
		})
	}
}

func TestCreateQuoteRejectsInvalidActorOrSpaceIDBeforeAnyDependencyRead(t *testing.T) {
	cases := []struct {
		name     string
		renterID string
		spaceID  string
	}{
		{name: "empty renter", renterID: "", spaceID: testSpaceID},
		{name: "malformed renter", renterID: "not-a-uuid", spaceID: testSpaceID},
		{name: "malformed space", renterID: testRenterID, spaceID: "not-a-uuid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
			catalog := &catalogStub{}
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{}
			service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Create(context.Background(), tc.renterID, QuoteRequest{SpaceID: tc.spaceID})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid actor/space error=%v, want ErrInvalid", err)
			}
			if catalog.calls != 0 || calendar.checkCalls != 0 || quotes.calls != 0 {
				t.Fatalf("invalid actor/space reached dependencies: catalog=%d calendar=%d quote=%d", catalog.calls, calendar.checkCalls, quotes.calls)
			}
		})
	}
}

func TestCreateQuoteRejectsMalformedOrNonFutureIntervalBeforeDependencyRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	futureStart := "2030-05-11T15:00:00Z"
	cases := []struct {
		name    string
		startAt string
		endAt   string
	}{
		{name: "malformed start", startAt: "tomorrow", endAt: "2030-05-11T16:00:00Z"},
		{name: "malformed end", startAt: futureStart, endAt: "2030-05-11"},
		{name: "zero start", startAt: "0001-01-01T00:00:00Z", endAt: "0001-01-01T01:00:00Z"},
		{name: "start is not future", startAt: now.Format(time.RFC3339Nano), endAt: "2030-05-10T13:00:00Z"},
		{name: "end equals start", startAt: futureStart, endAt: futureStart},
		{name: "end before start", startAt: "2030-05-11T16:00:00Z", endAt: futureStart},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			catalog := &catalogStub{}
			calendar := &calendarStub{available: true}
			quotes := &quoteRepositoryStub{}
			service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Create(context.Background(), testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: tc.startAt, EndAt: tc.endAt})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid interval error=%v, want ErrInvalid", err)
			}
			if catalog.calls != 0 || calendar.checkCalls != 0 || quotes.calls != 0 {
				t.Fatalf("invalid interval reached dependencies: catalog=%d calendar=%d quote=%d", catalog.calls, calendar.checkCalls, quotes.calls)
			}
		})
	}
}
