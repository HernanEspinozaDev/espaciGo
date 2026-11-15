package rentalflow

import (
	"context"
	"testing"
	"time"
)

func TestCreateQuoteNormalizesRFC3339OffsetsToUTCBeforeAvailabilityAndPersistence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset int
	}{
		{name: "positive half-hour offset", offset: 5*60*60 + 30*60},
		{name: "negative offset", offset: -(4 * 60 * 60)},
		{name: "maximum positive offset", offset: 14 * 60 * 60},
		{name: "maximum negative offset", offset: -(14 * 60 * 60)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertQuoteOffsetIsNormalized(t, tc.offset)
		})
	}
}

func FuzzCreateQuoteNormalizesRFC3339OffsetMinutesToUTC(f *testing.F) {
	for _, seed := range []uint16{0, 600, 840, 1170, 1680} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed uint16) {
		offsetMinutes := int(seed%1681) - 840
		if offsetMinutes == 0 {
			offsetMinutes = 1
		}
		assertQuoteOffsetIsNormalized(t, offsetMinutes*60)
	})
}

func assertQuoteOffsetIsNormalized(t *testing.T, offsetSeconds int) {
	t.Helper()
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	utcStart := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	utcEnd := utcStart.Add(time.Hour)
	listing := Listing{
		ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago",
	}
	inputZone := time.FixedZone("synthetic", offsetSeconds)
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{result: Quote{
		ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID,
		SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit,
		UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1,
		SubtotalCLP: listing.UnitPriceCLP, StartAt: utcStart, EndAt: utcEnd,
		TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), testRenterID, QuoteRequest{
		SpaceID: testSpaceID,
		StartAt: utcStart.In(inputZone).Format(time.RFC3339),
		EndAt:   utcEnd.In(inputZone).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.StartAt.Location() != time.UTC || !got.StartAt.Equal(utcStart) || !got.EndAt.Equal(utcEnd) {
		t.Fatalf("returned quote window = (%s, %s), want UTC (%s, %s)", got.StartAt, got.EndAt, utcStart, utcEnd)
	}
	if quotes.calls != 1 || quotes.input.StartAt.Location() != time.UTC || quotes.input.EndAt.Location() != time.UTC ||
		!quotes.input.StartAt.Equal(utcStart) || !quotes.input.EndAt.Equal(utcEnd) {
		t.Fatalf("repository received calls=%d window=(%s, %s), want one UTC-normalized write", quotes.calls, quotes.input.StartAt, quotes.input.EndAt)
	}
	checked := calendar.lastCheckedInterval
	if catalog.calls != 1 || calendar.checkCalls != 1 || checked.StartAt.Location() != time.UTC || checked.EndAt.Location() != time.UTC ||
		!checked.StartAt.Equal(utcStart) || !checked.EndAt.Equal(utcEnd) {
		t.Fatalf("calendar/listing calls: listing=%d availability=%d checked=(%s, %s), want one call each with UTC window (%s, %s)", catalog.calls, calendar.checkCalls, checked.StartAt, checked.EndAt, utcStart, utcEnd)
	}
}
