package rentalflow

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type IdentityGate interface {
	RequireForBooking(context.Context, string, string, time.Time) error
}

// ContractIdentityReader is a purpose-limited identity-data port. Implementations
// must return legal name and RUT only for a party current at the supplied as-of
// instant when the requester participates in the quote. It must not expose
// documentary evidence.
type ContractIdentityReader interface {
	CurrentForRentalContract(context.Context, string, string, string, time.Time) (ContractIdentity, error)
}

type ContractIdentity struct {
	SubjectID string
	LegalName string
	RUT       string
}

type ContractParty struct {
	Role      string `json:"role"`
	LegalName string `json:"legal_name"`
	RUT       string `json:"rut"`
}

type ContractPreview struct {
	State        string          `json:"state"`
	QuoteID      string          `json:"quote_id"`
	SpaceID      string          `json:"space_id"`
	SpaceTitle   string          `json:"space_title"`
	RateUnit     string          `json:"rate_unit"`
	UnitPriceCLP int64           `json:"unit_price_clp"`
	Units        int64           `json:"units"`
	SubtotalCLP  int64           `json:"subtotal_clp"`
	Currency     string          `json:"currency"`
	StartAt      time.Time       `json:"start_at"`
	EndAt        time.Time       `json:"end_at"`
	TimeZone     string          `json:"time_zone"`
	Parties      []ContractParty `json:"parties"`
	GeneratedAt  time.Time       `json:"generated_at"`
}

type ContractPreviewService struct {
	listings   PublicListingReader
	calendar   AvailabilityReader
	quotes     QuoteRepository
	identity   IdentityGate
	identities ContractIdentityReader
	now        func() time.Time
}

func NewContractPreviewService(listings PublicListingReader, calendar AvailabilityReader, quotes QuoteRepository, identity IdentityGate, identities ContractIdentityReader, now func() time.Time) (*ContractPreviewService, error) {
	if listings == nil || calendar == nil || quotes == nil || identity == nil || identities == nil || now == nil {
		return nil, ErrInvalid
	}
	return &ContractPreviewService{listings: listings, calendar: calendar, quotes: quotes, identity: identity, identities: identities, now: now}, nil
}

// Preview builds an in-memory contract preview for a quote participant. It does
// not persist a contract, create a reservation, or trigger signature/payment.
func (s *ContractPreviewService) Preview(ctx context.Context, actorID, quoteID string) (ContractPreview, error) {
	if s == nil || ctx == nil || !validUUID(actorID) || !validUUID(quoteID) {
		return ContractPreview{}, ErrInvalid
	}
	actorID = canonicalUUID(actorID)
	quoteID = canonicalUUID(quoteID)
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	now := s.now()
	if now.IsZero() {
		return ContractPreview{}, ErrInvalid
	}
	now = now.UTC()
	quote, err := s.quotes.GetForParticipant(ctx, actorID, quoteID)
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	if errors.Is(err, ErrNotFound) {
		return ContractPreview{}, ErrNotFound
	}
	if err != nil {
		return ContractPreview{}, ErrUnavailable
	}
	quote.ID = canonicalUUID(quote.ID)
	quote.SpaceID = canonicalUUID(quote.SpaceID)
	quote.OwnerID = canonicalUUID(quote.OwnerID)
	quote.RenterID = canonicalUUID(quote.RenterID)
	if quote.ID != quoteID || !validUUID(quote.SpaceID) || !validUUID(quote.OwnerID) || !validUUID(quote.RenterID) || quote.OwnerID == quote.RenterID ||
		(actorID != quote.OwnerID && actorID != quote.RenterID) {
		return ContractPreview{}, ErrNotFound
	}
	if !quote.ExpiresAt.After(now) {
		return ContractPreview{}, ErrConflict
	}
	listing, err := s.listings.PublicListing(ctx, quote.SpaceID)
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	listing.ID = canonicalUUID(listing.ID)
	listing.OwnerID = canonicalUUID(listing.OwnerID)
	if errors.Is(err, ErrNotFound) {
		return ContractPreview{}, ErrConflict
	}
	if err != nil {
		return ContractPreview{}, ErrUnavailable
	}
	if listing.ID != quote.SpaceID || listing.State != "activa" {
		return ContractPreview{}, ErrConflict
	}
	loc, err := validateListing(listing)
	if err != nil {
		return ContractPreview{}, ErrConflict
	}
	interval := Interval{StartAt: quote.StartAt.UTC(), EndAt: quote.EndAt.UTC()}
	units, unitsErr := pricing.BilledUnits(listing.RateUnit, interval.StartAt, interval.EndAt, listing.TimeZone)
	dateText := interval.StartAt.In(loc).Format("2006-01-02")
	if unitsErr != nil || units < 1 || units > 4 || units > math.MaxInt64/listing.UnitPriceCLP ||
		!validDuration(listing.RateUnit, int(units)) || !dateWithinHorizon(dateText, now, loc) ||
		!intervalMatchesRequest(interval, listing.RateUnit, dateText, int(units), listing.TimeZone, now) ||
		units*listing.UnitPriceCLP != quote.SubtotalCLP ||
		!validQuote(quote, quote.RenterID, listing, interval, units, quote.SubtotalCLP, now) {
		return ContractPreview{}, ErrConflict
	}
	available, err := s.calendar.IntervalAvailable(ctx, listing, interval)
	if err != nil {
		return ContractPreview{}, ErrUnavailable
	}
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	if !available {
		return ContractPreview{}, ErrConflict
	}
	err = s.identity.RequireForBooking(ctx, quote.OwnerID, quote.RenterID, now)
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	if errors.Is(err, identityverification.ErrNotVerified) {
		return ContractPreview{}, ErrIdentityRequired
	} else if err != nil {
		return ContractPreview{}, ErrIdentityUnavailable
	}
	ownerIdentity, err := s.identities.CurrentForRentalContract(ctx, actorID, quoteID, quote.OwnerID, now)
	if err != nil {
		if ctx.Err() != nil {
			return ContractPreview{}, ErrUnavailable
		}
		return ContractPreview{}, ErrIdentityUnavailable
	}
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	ownerIdentity, valid := normalizeContractIdentity(ownerIdentity, quote.OwnerID)
	if !valid {
		return ContractPreview{}, ErrIdentityUnavailable
	}
	renterIdentity, err := s.identities.CurrentForRentalContract(ctx, actorID, quoteID, quote.RenterID, now)
	if err != nil {
		if ctx.Err() != nil {
			return ContractPreview{}, ErrUnavailable
		}
		return ContractPreview{}, ErrIdentityUnavailable
	}
	if ctx.Err() != nil {
		return ContractPreview{}, ErrUnavailable
	}
	renterIdentity, valid = normalizeContractIdentity(renterIdentity, quote.RenterID)
	if !valid {
		return ContractPreview{}, ErrIdentityUnavailable
	}
	return ContractPreview{
		State: "preview", QuoteID: quote.ID, SpaceID: quote.SpaceID, SpaceTitle: quote.SpaceTitle,
		RateUnit: quote.RateUnit, UnitPriceCLP: quote.UnitPriceCLP, Units: quote.Units,
		SubtotalCLP: quote.SubtotalCLP, Currency: quote.Currency, StartAt: quote.StartAt,
		EndAt: quote.EndAt, TimeZone: quote.TimeZone,
		Parties: []ContractParty{
			{Role: "arrendador", LegalName: ownerIdentity.LegalName, RUT: ownerIdentity.RUT},
			{Role: "arrendatario", LegalName: renterIdentity.LegalName, RUT: renterIdentity.RUT},
		},
		GeneratedAt: now,
	}, nil
}

func normalizeContractIdentity(identity ContractIdentity, subjectID string) (ContractIdentity, bool) {
	identity.SubjectID = canonicalUUID(identity.SubjectID)
	if identity.SubjectID != subjectID || !utf8.ValidString(identity.LegalName) ||
		identity.LegalName != strings.TrimSpace(identity.LegalName) || identity.RUT != strings.TrimSpace(identity.RUT) ||
		identity.LegalName == "" || identity.RUT == "" || utf8.RuneCountInString(identity.LegalName) > 200 || len(identity.RUT) > 20 {
		return ContractIdentity{}, false
	}
	for _, value := range identity.LegalName + identity.RUT {
		if unicode.IsControl(value) || unicode.Is(unicode.Bidi_Control, value) || unicode.Is(unicode.Zl, value) || unicode.Is(unicode.Zp, value) {
			return ContractIdentity{}, false
		}
	}
	canonicalRut, err := verification.NormalizeRut(identity.RUT)
	if err != nil || strings.HasPrefix(canonicalRut, "-") {
		return ContractIdentity{}, false
	}
	identity.RUT = canonicalRut
	return identity, true
}
