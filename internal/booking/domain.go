package booking

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

var (
	ErrInvalid             = errors.New("booking: invalid request")
	ErrNotFound            = errors.New("booking: resource not found")
	ErrConflict            = errors.New("booking: state or availability conflict")
	ErrSimulatedNoResponse = errors.New("booking: fake payment timed out without a response")
)

const SafetyBanner = "ENSAYO LOCAL — SIN COBRO REAL"

type Fixture struct {
	SpaceID  string `json:"space_id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	OwnerID  string `json:"host_id"`
	RenterID string `json:"renter_id"`
	RateUnit string `json:"rate_unit"`
	Price    int64  `json:"base_price_clp"`
	Currency string `json:"currency"`
	TimeZone string `json:"time_zone"`
}
type CatalogItem struct {
	SpaceID        string          `json:"space_id"`
	CategoryCode   string          `json:"category_code"`
	CategoryName   string          `json:"category_name"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	RateUnit       string          `json:"rate_unit"`
	Price          int64           `json:"base_price_clp"`
	Currency       string          `json:"currency"`
	TimeZone       string          `json:"time_zone"`
	ProfileVersion int             `json:"profile_version"`
	Profile        json.RawMessage `json:"profile"`
	Attributes     json.RawMessage `json:"attributes"`
	Available      *bool           `json:"available,omitempty"`
	EstimatedTotal *int64          `json:"estimated_total_clp,omitempty"`
	DistanceKM     *float64        `json:"distance_km,omitempty"`
	DistanceKind   string          `json:"distance_kind,omitempty"`
	DistanceMeters float64         `json:"-"`
}
type CatalogFilter struct {
	CategoryCode   string
	StartAt        *time.Time
	EndAt          *time.Time
	MinTotalCLP    *int64
	MaxTotalCLP    *int64
	ProfileVersion int
	Attributes     map[string]any
	Latitude       *float64
	Longitude      *float64
	RadiusKM       *int
}
type QuoteInput struct {
	SpaceID string `json:"space_id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}
type Quote struct {
	ID             string          `json:"id"`
	SpaceID        string          `json:"space_id"`
	RateVersion    int64           `json:"rate_version"`
	RateUnit       string          `json:"rate_unit"`
	UnitPrice      int64           `json:"unit_price_clp"`
	Currency       string          `json:"currency"`
	Units          int64           `json:"units"`
	Subtotal       int64           `json:"subtotal_clp"`
	StartAt        time.Time       `json:"start_at"`
	EndAt          time.Time       `json:"end_at"`
	TimeZone       string          `json:"time_zone"`
	Conditions     string          `json:"conditions"`
	CategoryCode   string          `json:"category_code"`
	ProfileVersion int             `json:"profile_version"`
	ProfileValues  json.RawMessage `json:"profile_values"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
}
type Reservation struct {
	ID            string     `json:"id"`
	QuoteID       string     `json:"quote_id"`
	SpaceID       string     `json:"space_id"`
	HostID        string     `json:"host_id"`
	RenterID      string     `json:"renter_id"`
	State         string     `json:"state"`
	RateUnit      string     `json:"rate_unit"`
	UnitPrice     int64      `json:"unit_price_clp"`
	Currency      string     `json:"currency"`
	Units         int64      `json:"units"`
	Subtotal      int64      `json:"subtotal_clp"`
	StartAt       time.Time  `json:"start_at"`
	EndAt         time.Time  `json:"end_at"`
	TimeZone      string     `json:"time_zone"`
	Conditions    string     `json:"conditions"`
	PayExpiresAt  time.Time  `json:"pay_expires_at"`
	HostExpiresAt *time.Time `json:"host_expires_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	UnreadCount   int64      `json:"unread_count"`
}
type Transition struct {
	Sequence int64     `json:"sequence"`
	From     *string   `json:"from,omitempty"`
	To       string    `json:"to"`
	Actor    *string   `json:"actor_id,omitempty"`
	Reason   string    `json:"reason"`
	At       time.Time `json:"at"`
}
type Detail struct {
	Reservation `json:",inline"`
	History     []Transition `json:"history"`
}
type RequestInput struct {
	QuoteID string `json:"quote_id"`
}
type PaymentInput struct {
	Outcome string `json:"outcome"`
}
type DecisionInput struct {
	Decision string `json:"decision"`
}

type Repository interface {
	Fixture(context.Context, string) (Fixture, error)
	Catalog(context.Context, string, CatalogFilter) ([]CatalogItem, error)
	CatalogProfile(context.Context, string, int) (spaces.Profile, error)
	CatalogDetail(context.Context, string, string) (CatalogItem, error)
	Quote(context.Context, string, string, string, time.Time, time.Time, func() time.Time, time.Duration) (Quote, error)
	Create(context.Context, string, string, string, []byte, string, string, time.Duration, func() time.Time) (Reservation, error)
	Get(context.Context, string, string) (Detail, error)
	List(context.Context, string) ([]Reservation, error)
	Pay(context.Context, string, string, string, string, time.Time, time.Time) (Reservation, error)
	Decide(context.Context, string, string, string, time.Time) (Reservation, error)
	Cancel(context.Context, string, string, time.Time) (Reservation, error)
	Expire(context.Context, time.Time) error
}

// LocalPaymentAdapter is intentionally a narrow port. The only production in
// this slice is the local fake; a gateway must not be wired into this profile.
type LocalPaymentAdapter interface {
	Process(context.Context, string) (string, error)
}

var uuid = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
