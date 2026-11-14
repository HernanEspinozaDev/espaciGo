package rentalflow

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/calendar"
)

var (
	ErrInvalid             = errors.New("rental flow: invalid request or contract")
	ErrNotFound            = errors.New("rental flow: resource not found")
	ErrConflict            = errors.New("rental flow: quote or availability changed")
	ErrUnavailable         = errors.New("rental flow: availability unavailable")
	ErrIdentityRequired    = errors.New("rental flow: current identity verification required")
	ErrIdentityUnavailable = errors.New("rental flow: identity verification unavailable")
)

const publicAvailabilityHorizonDays = 90

type Listing struct {
	ID           string
	OwnerID      string
	State        string
	Title        string
	RateUnit     string
	UnitPriceCLP int64
	Currency     string
	RateVersion  int64
	TimeZone     string
}

type Interval struct {
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

type Availability struct {
	SpaceID   string     `json:"space_id"`
	TimeZone  string     `json:"time_zone"`
	RateUnit  string     `json:"rate_unit"`
	CheckedAt time.Time  `json:"checked_at"`
	Items     []Interval `json:"items"`
}

type PublicListingReader interface {
	PublicListing(context.Context, string) (Listing, error)
}

type AvailabilityReader interface {
	PublicIntervals(context.Context, Listing, string, int) ([]Interval, error)
	IntervalAvailable(context.Context, Listing, Interval) (bool, error)
}

type AvailabilityService struct {
	listings PublicListingReader
	calendar AvailabilityReader
	now      func() time.Time
}

func NewAvailabilityService(listings PublicListingReader, calendar AvailabilityReader, now func() time.Time) (*AvailabilityService, error) {
	if listings == nil || calendar == nil || now == nil {
		return nil, ErrInvalid
	}
	return &AvailabilityService{listings: listings, calendar: calendar, now: now}, nil
}

// ListPublic returns only consultable intervals. The reader is responsible for
// combining the host schedule with authoritative occupancy; this method never
// exposes owners, occupants, block reasons, or reservation identifiers.
func (s *AvailabilityService) ListPublic(ctx context.Context, spaceID, dateText string, duration int) (Availability, error) {
	if s == nil || ctx == nil || !validUUID(spaceID) || !validDate(dateText) {
		return Availability{}, ErrInvalid
	}
	spaceID = canonicalUUID(spaceID)
	if ctx.Err() != nil {
		return Availability{}, ErrUnavailable
	}
	now := s.now()
	if now.IsZero() {
		return Availability{}, ErrInvalid
	}
	now = now.UTC()
	listing, err := s.listings.PublicListing(ctx, spaceID)
	if ctx.Err() != nil {
		return Availability{}, ErrUnavailable
	}
	listing.ID = canonicalUUID(listing.ID)
	listing.OwnerID = canonicalUUID(listing.OwnerID)
	if errors.Is(err, ErrNotFound) {
		return Availability{}, ErrNotFound
	}
	if err != nil {
		return Availability{}, ErrUnavailable
	}
	if listing.ID != spaceID || listing.State != "activa" {
		return Availability{}, ErrNotFound
	}
	loc, err := validateListing(listing)
	if err != nil {
		return Availability{}, err
	}
	if !dateWithinHorizon(dateText, now, loc) || !validDuration(listing.RateUnit, duration) {
		return Availability{}, ErrInvalid
	}
	items, err := s.calendar.PublicIntervals(ctx, listing, dateText, duration)
	if err != nil {
		return Availability{}, ErrUnavailable
	}
	if ctx.Err() != nil {
		return Availability{}, ErrUnavailable
	}
	validated := make([]Interval, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !intervalMatchesRequest(item, listing.RateUnit, dateText, duration, listing.TimeZone, now) {
			return Availability{}, ErrUnavailable
		}
		key := item.StartAt.UTC().Format(time.RFC3339Nano) + "/" + item.EndAt.UTC().Format(time.RFC3339Nano)
		if _, exists := seen[key]; exists {
			return Availability{}, ErrUnavailable
		}
		seen[key] = struct{}{}
		validated = append(validated, Interval{StartAt: item.StartAt.UTC(), EndAt: item.EndAt.UTC()})
	}
	sort.Slice(validated, func(i, j int) bool { return validated[i].StartAt.Before(validated[j].StartAt) })
	return Availability{SpaceID: listing.ID, TimeZone: listing.TimeZone, RateUnit: listing.RateUnit, CheckedAt: now, Items: validated}, nil
}

func validateListing(listing Listing) (*time.Location, error) {
	if !validUUID(listing.ID) || !validUUID(listing.OwnerID) || listing.State != "activa" ||
		!utf8.ValidString(listing.Title) || listing.Title == "" || listing.Title != strings.TrimSpace(listing.Title) || utf8.RuneCountInString(listing.Title) > 70 ||
		(listing.RateUnit != "hora" && listing.RateUnit != "dia" && listing.RateUnit != "mes") ||
		listing.UnitPriceCLP <= 5000 || listing.Currency != "CLP" || listing.RateVersion < 1 ||
		listing.TimeZone == "" || listing.TimeZone == "Local" {
		return nil, ErrUnavailable
	}
	for _, r := range listing.Title {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return nil, ErrUnavailable
		}
	}
	loc, err := time.LoadLocation(listing.TimeZone)
	if err != nil {
		return nil, ErrUnavailable
	}
	return loc, nil
}

func validDuration(unit string, duration int) bool {
	switch unit {
	case "hora":
		return duration == 1 || duration == 2 || duration == 4
	case "dia":
		return duration >= 1 && duration <= 3
	case "mes":
		return duration == 1
	default:
		return false
	}
}

func intervalMatchesRequest(interval Interval, unit, dateText string, duration int, zone string, now time.Time) bool {
	if interval.StartAt.IsZero() || interval.EndAt.IsZero() || !interval.StartAt.After(now) || !interval.EndAt.After(interval.StartAt) {
		return false
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false
	}
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		return false
	}
	startLocal := interval.StartAt.In(loc)
	year, month, day := startLocal.Date()
	if time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02") != dateText {
		return false
	}
	switch unit {
	case "hora":
		if interval.EndAt.Sub(interval.StartAt) != time.Duration(duration)*time.Hour {
			return false
		}
		_, minute, second := startLocal.Clock()
		return minute%30 == 0 && second == 0 && startLocal.Nanosecond() == 0
	case "dia":
		start, startOK := calendar.FirstInstantOfDate(date.Year(), date.Month(), date.Day(), loc)
		endDate := date.AddDate(0, 0, duration)
		end, endOK := calendar.FirstInstantOfDate(endDate.Year(), endDate.Month(), endDate.Day(), loc)
		return startOK && endOK && interval.StartAt.Equal(start) && interval.EndAt.Equal(end)
	case "mes":
		start, startOK := calendar.FirstInstantOfDate(date.Year(), date.Month(), date.Day(), loc)
		anniversary, anniversaryOK := calendar.AddMonthsClamped(date, 1)
		end, endOK := calendar.FirstInstantOfDate(anniversary.Year(), anniversary.Month(), anniversary.Day(), loc)
		return startOK && anniversaryOK && endOK && interval.StartAt.Equal(start) && interval.EndAt.Equal(end)
	default:
		return false
	}
}

func dateWithinHorizon(dateText string, now time.Time, loc *time.Location) bool {
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil || date.Format("2006-01-02") != dateText {
		return false
	}
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return !date.Before(today) && !date.After(today.AddDate(0, 0, publicAvailabilityHorizonDays))
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

// ValidDate reports whether value is a canonical Gregorian calendar date.
func ValidDate(value string) bool { return validDate(value) }

var rentalUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func validUUID(value string) bool { return rentalUUIDPattern.MatchString(value) }

func canonicalUUID(value string) string {
	if !validUUID(value) {
		return value
	}
	return strings.ToLower(value)
}

// ValidUUID reports whether value is a canonical hyphenated UUID.
func ValidUUID(value string) bool { return validUUID(value) }
