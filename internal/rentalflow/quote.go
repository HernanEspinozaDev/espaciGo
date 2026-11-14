package rentalflow

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
)

type QuoteRequest struct {
	SpaceID string `json:"space_id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}

type Quote struct {
	ID           string    `json:"id"`
	SpaceID      string    `json:"space_id"`
	OwnerID      string    `json:"-"`
	RenterID     string    `json:"-"`
	SpaceTitle   string    `json:"space_title"`
	RateVersion  int64     `json:"-"`
	RateUnit     string    `json:"rate_unit"`
	UnitPriceCLP int64     `json:"unit_price_clp"`
	Currency     string    `json:"currency"`
	Units        int64     `json:"units"`
	SubtotalCLP  int64     `json:"subtotal_clp"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	TimeZone     string    `json:"time_zone"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// QuoteRepository must atomically re-read the active publication and current
// rate, re-check the interval against schedule and occupancy, and persist only
// the quote snapshot; it must never create an occupancy hold.
type QuoteRepository interface {
	CreateValidatedQuote(context.Context, string, Listing, Interval, int64, int64, time.Time) (Quote, error)
	GetForParticipant(context.Context, string, string) (Quote, error)
}

type QuoteService struct {
	listings PublicListingReader
	calendar AvailabilityReader
	quotes   QuoteRepository
	now      func() time.Time
}

func NewQuoteService(listings PublicListingReader, calendar AvailabilityReader, quotes QuoteRepository, now func() time.Time) (*QuoteService, error) {
	if listings == nil || calendar == nil || quotes == nil || now == nil {
		return nil, ErrInvalid
	}
	return &QuoteService{listings: listings, calendar: calendar, quotes: quotes, now: now}, nil
}

func (s *QuoteService) Create(ctx context.Context, renterID string, in QuoteRequest) (Quote, error) {
	if s == nil || ctx == nil || !validUUID(renterID) || !validUUID(in.SpaceID) {
		return Quote{}, ErrInvalid
	}
	renterID = canonicalUUID(renterID)
	in.SpaceID = canonicalUUID(in.SpaceID)
	if ctx.Err() != nil {
		return Quote{}, ErrUnavailable
	}
	now := s.now()
	if now.IsZero() {
		return Quote{}, ErrInvalid
	}
	start, valid := parseRFC3339Timestamp(in.StartAt)
	if !valid {
		return Quote{}, ErrInvalid
	}
	end, valid := parseRFC3339Timestamp(in.EndAt)
	if !valid || !end.After(start) {
		return Quote{}, ErrInvalid
	}
	now = now.UTC()
	if !start.After(now) {
		return Quote{}, ErrInvalid
	}
	listing, err := s.listings.PublicListing(ctx, in.SpaceID)
	if ctx.Err() != nil {
		return Quote{}, ErrUnavailable
	}
	listing.ID = canonicalUUID(listing.ID)
	listing.OwnerID = canonicalUUID(listing.OwnerID)
	if errors.Is(err, ErrNotFound) {
		return Quote{}, ErrNotFound
	}
	if err != nil {
		return Quote{}, ErrUnavailable
	}
	if listing.ID != in.SpaceID || listing.OwnerID == renterID || listing.State != "activa" {
		return Quote{}, ErrNotFound
	}
	loc, err := validateListing(listing)
	if err != nil {
		return Quote{}, err
	}
	units, err := pricing.BilledUnits(listing.RateUnit, start.UTC(), end.UTC(), listing.TimeZone)
	if err != nil || units < 1 || units > math.MaxInt64/listing.UnitPriceCLP || !validDuration(listing.RateUnit, int(units)) {
		return Quote{}, ErrInvalid
	}
	interval := Interval{StartAt: start.UTC(), EndAt: end.UTC()}
	dateText := start.In(loc).Format("2006-01-02")
	if !dateWithinHorizon(dateText, now, loc) || !intervalMatchesRequest(interval, listing.RateUnit, dateText, int(units), listing.TimeZone, now) {
		return Quote{}, ErrInvalid
	}
	available, err := s.calendar.IntervalAvailable(ctx, listing, interval)
	if err != nil {
		return Quote{}, ErrUnavailable
	}
	if ctx.Err() != nil {
		return Quote{}, ErrUnavailable
	}
	if !available {
		return Quote{}, ErrConflict
	}
	subtotal := units * listing.UnitPriceCLP
	quote, err := s.quotes.CreateValidatedQuote(ctx, renterID, listing, interval, units, subtotal, now)
	if err != nil {
		if ctx.Err() != nil {
			return Quote{}, ErrUnavailable
		}
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
			return Quote{}, err
		}
		return Quote{}, ErrUnavailable
	}
	quote.ID = canonicalUUID(quote.ID)
	quote.SpaceID = canonicalUUID(quote.SpaceID)
	quote.OwnerID = canonicalUUID(quote.OwnerID)
	quote.RenterID = canonicalUUID(quote.RenterID)
	if !validQuote(quote, renterID, listing, interval, units, subtotal, now) {
		return Quote{}, ErrUnavailable
	}
	return quote, nil
}

func parseRFC3339Timestamp(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return parsed, err == nil && !parsed.IsZero()
}

// ValidTimestamp reports whether value is a non-zero RFC3339 timestamp.
func ValidTimestamp(value string) bool {
	_, valid := parseRFC3339Timestamp(value)
	return valid
}

func validQuote(quote Quote, renterID string, listing Listing, interval Interval, units, subtotal int64, now time.Time) bool {
	return validUUID(quote.ID) && quote.SpaceID == listing.ID && quote.OwnerID == listing.OwnerID &&
		quote.RenterID == renterID && quote.SpaceTitle == listing.Title && quote.RateVersion == listing.RateVersion &&
		quote.RateUnit == listing.RateUnit && quote.UnitPriceCLP == listing.UnitPriceCLP && quote.Currency == "CLP" &&
		quote.Units == units && quote.SubtotalCLP == subtotal && quote.StartAt.Equal(interval.StartAt) &&
		quote.EndAt.Equal(interval.EndAt) && quote.TimeZone == listing.TimeZone &&
		!quote.CreatedAt.IsZero() && !quote.CreatedAt.After(now) && quote.ExpiresAt.After(now)
}
