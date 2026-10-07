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
	ErrInvalid                   = errors.New("booking: invalid request")
	ErrNotFound                  = errors.New("booking: resource not found")
	ErrConflict                  = errors.New("booking: state or availability conflict")
	ErrSimulatedNoResponse       = errors.New("booking: fake payment timed out without a response")
	ErrSimulatedRefundNoResponse = errors.New("booking: fake refund timed out without a response")
)

const SafetyBanner = "ENSAYO LOCAL — SIN COBRO REAL"
const LocalCancellationPolicyVersion = "local_flexible_v1"

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
	CategoryOrder  int             `json:"-"`
}

type CatalogPage struct {
	Items      []CatalogItem `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
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
type AvailabilityOptionsInput struct {
	Date     string
	Duration int
}
type AvailableInterval struct {
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}
type AvailabilityOptions struct {
	SpaceID  string              `json:"space_id"`
	TimeZone string              `json:"time_zone"`
	RateUnit string              `json:"rate_unit"`
	Items    []AvailableInterval `json:"items"`
}
type Quote struct {
	ID                        string          `json:"id"`
	SpaceID                   string          `json:"space_id"`
	RateVersion               int64           `json:"rate_version"`
	RateUnit                  string          `json:"rate_unit"`
	UnitPrice                 int64           `json:"unit_price_clp"`
	Currency                  string          `json:"currency"`
	Units                     int64           `json:"units"`
	Subtotal                  int64           `json:"subtotal_clp"`
	StartAt                   time.Time       `json:"start_at"`
	EndAt                     time.Time       `json:"end_at"`
	TimeZone                  string          `json:"time_zone"`
	Conditions                string          `json:"conditions"`
	CategoryCode              string          `json:"category_code"`
	ProfileVersion            int             `json:"profile_version"`
	ProfileValues             json.RawMessage `json:"profile_values"`
	CancellationPolicyVersion string          `json:"cancellation_policy_version"`
	CreatedAt                 time.Time       `json:"created_at"`
	ExpiresAt                 time.Time       `json:"expires_at"`
}
type Reservation struct {
	ID                        string     `json:"id"`
	QuoteID                   string     `json:"quote_id"`
	SpaceID                   string     `json:"space_id"`
	HostID                    string     `json:"host_id"`
	RenterID                  string     `json:"renter_id"`
	State                     string     `json:"state"`
	RateUnit                  string     `json:"rate_unit"`
	UnitPrice                 int64      `json:"unit_price_clp"`
	Currency                  string     `json:"currency"`
	Units                     int64      `json:"units"`
	Subtotal                  int64      `json:"subtotal_clp"`
	StartAt                   time.Time  `json:"start_at"`
	EndAt                     time.Time  `json:"end_at"`
	TimeZone                  string     `json:"time_zone"`
	Conditions                string     `json:"conditions"`
	CancellationPolicyVersion string     `json:"cancellation_policy_version"`
	RefundID                  *string    `json:"refund_id,omitempty"`
	RefundOperationID         *string    `json:"refund_operation_id,omitempty"`
	RefundAmountCLP           *int64     `json:"refund_amount_clp,omitempty"`
	RefundState               *string    `json:"refund_state,omitempty"`
	RefundLastResult          *string    `json:"refund_last_result,omitempty"`
	RefundUpdatedAt           *time.Time `json:"refund_updated_at,omitempty"`
	PayExpiresAt              time.Time  `json:"pay_expires_at"`
	HostExpiresAt             *time.Time `json:"host_expires_at,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
	UnreadCount               int64      `json:"unread_count"`
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
	Reason   string `json:"reason,omitempty"`
}

type CancellationInput struct {
	Reason string `json:"reason,omitempty"`
}

type RefundInput struct {
	Outcome string `json:"outcome"`
}

type CancellationPreview struct {
	ReservationID string    `json:"reservation_id"`
	PolicyVersion string    `json:"policy_version"`
	Eligible      bool      `json:"eligible"`
	Deadline      time.Time `json:"deadline"`
	AmountCLP     int64     `json:"amount_clp"`
	Currency      string    `json:"currency"`
	RefundLabel   string    `json:"refund_label"`
	ReasonCode    string    `json:"reason_code,omitempty"`
}

type CancellationResult struct {
	Reservation     Reservation `json:"reservation"`
	RefundAmountCLP *int64      `json:"refund_amount_clp,omitempty"`
	RefundState     string      `json:"refund_state"`
	NoticeStatus    string      `json:"notice_status"`
	Replayed        bool        `json:"replayed,omitempty"`
}

type RefundResult struct {
	ReservationID string    `json:"reservation_id"`
	OperationID   string    `json:"operation_id"`
	AmountCLP     int64     `json:"amount_clp"`
	Currency      string    `json:"currency"`
	State         string    `json:"state"`
	LastResult    string    `json:"last_result,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	NoticeStatus  string    `json:"notice_status"`
	Reused        bool      `json:"reused,omitempty"`
}

type LocalNoticeSender interface {
	SendLocalBookingNotice(context.Context, string, string, string) error
}

type LocalRefundAdapter interface {
	ProcessRefund(context.Context, string, string, string) (string, error)
}

type Repository interface {
	Fixture(context.Context, string) (Fixture, error)
	Catalog(context.Context, string, CatalogFilter) ([]CatalogItem, error)
	CatalogProfile(context.Context, string, int) (spaces.Profile, error)
	CatalogDetail(context.Context, string, string) (CatalogItem, error)
	AvailableIntervals(context.Context, string, string, []AvailableInterval) ([]bool, error)
	Quote(context.Context, string, string, string, time.Time, time.Time, func() time.Time, time.Duration) (Quote, error)
	Create(context.Context, string, string, string, []byte, string, string, time.Duration, func() time.Time) (Reservation, error)
	Get(context.Context, string, string) (Detail, error)
	List(context.Context, string) ([]Reservation, error)
	Pay(context.Context, string, string, string, string, func() time.Time, time.Duration) (Reservation, error)
	Decide(context.Context, string, string, string, string, func() time.Time) (Reservation, error)
	Cancel(context.Context, string, string, string, string, []byte, func() time.Time) (CancellationResult, error)
	CancellationPreview(context.Context, string, string, time.Time) (CancellationPreview, error)
	RefundOperation(context.Context, string, string) (RefundResult, error)
	RecordRefund(context.Context, string, string, string, time.Time) (RefundResult, error)
	NoticeRecipients(context.Context, string, string) ([]string, error)
	Expire(context.Context, time.Time) error
}

// WeeklyHoursRepository is optional in test doubles and required by the
// PostgreSQL local-trial repository. Schedule writes are host-owned and
// serialized on the space row with quote/request transactions.
type WeeklyHoursRepository interface {
	WeeklyHoursForSpace(context.Context, string) (WeeklyHours, error)
	WeeklyHoursForHost(context.Context, string, string) (WeeklyHours, error)
	SaveWeeklyHours(context.Context, string, string, WeeklyHours) (WeeklyHours, error)
}

// LocalPaymentAdapter is intentionally a narrow port. The only production in
// this slice is the local fake; a gateway must not be wired into this profile.
type LocalPaymentAdapter interface {
	Process(context.Context, string) (string, error)
}

var uuid = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
