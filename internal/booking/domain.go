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
	ErrInvalid                     = errors.New("booking: invalid request")
	ErrNotFound                    = errors.New("booking: resource not found")
	ErrConflict                    = errors.New("booking: state or availability conflict")
	ErrUnauthenticatedPaymentEvent = errors.New("booking: unauthenticated payment event")
	ErrSimulatedNoResponse         = errors.New("booking: fake payment timed out without a response")
	ErrSimulatedRefundNoResponse   = errors.New("booking: fake refund timed out without a response")
	ErrAdminAuditExportLimit       = errors.New("booking: audit export exceeds limit")
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
	GuaranteePolicyVersion    *string         `json:"guarantee_policy_version,omitempty"`
	GuaranteeCurrency         *string         `json:"guarantee_currency,omitempty"`
	GuaranteeExpectedCLP      *int64          `json:"guarantee_expected_clp,omitempty"`
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
	GuaranteePolicyVersion    *string    `json:"guarantee_policy_version,omitempty"`
	GuaranteeCurrency         *string    `json:"guarantee_currency,omitempty"`
	GuaranteeExpectedCLP      *int64     `json:"guarantee_expected_clp,omitempty"`
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

// AdminReservationSummary is a purpose-limited, read-only projection. Participant
// identifiers are nullable because privacy retention can unlink them.
type AdminReservationSummary struct {
	ID        string    `json:"id"`
	HostID    *string   `json:"host_id"`
	RenterID  *string   `json:"renter_id"`
	State     string    `json:"state"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Subtotal  int64     `json:"subtotal_clp"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type AdminReservationFilter struct {
	ID          string
	State       string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}
type AdminReservationPage struct {
	Items      []AdminReservationSummary `json:"items"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}
type AdminPaymentFact struct {
	Result    string    `json:"result"`
	AmountCLP int64     `json:"amount_clp"`
	At        time.Time `json:"at"`
}
type AdminPaymentOperation struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Requested string    `json:"requested_result"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type AdminRefundFact struct {
	AmountCLP  int64     `json:"amount_clp"`
	Currency   string    `json:"currency"`
	State      string    `json:"state"`
	LastResult *string   `json:"last_result"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type AdminClaimFact struct {
	ID         string     `json:"id"`
	State      string     `json:"state"`
	OpenedAt   time.Time  `json:"opened_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Outcome    *string    `json:"outcome"`
}
type AdminFinancialDecision struct {
	Outcome      string    `json:"outcome"`
	DeductionCLP int64     `json:"deduction_clp"`
	ReasonCode   string    `json:"reason_code"`
	State        string    `json:"state"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type AdminGuaranteeSnapshot struct {
	PolicyVersion         string                  `json:"policy_version"`
	Currency              string                  `json:"currency"`
	ExpectedCLP           int64                   `json:"expected_clp"`
	AuthorizedCLP         int64                   `json:"authorized_clp"`
	CapturedCLP           int64                   `json:"captured_clp"`
	ReleasedCLP           int64                   `json:"released_clp"`
	State                 string                  `json:"state"`
	AuthorizationDeadline *time.Time              `json:"authorization_deadline,omitempty"`
	Operations            []GuaranteeOperation    `json:"operations"`
	Decision              *AdminFinancialDecision `json:"financial_decision,omitempty"`
}
type AdminReservationDetail struct {
	AdminReservationSummary
	SpaceID           string                  `json:"space_id"`
	History           []AdminTransition       `json:"history"`
	Payments          []AdminPaymentFact      `json:"rental_payments"`
	PaymentOperations []AdminPaymentOperation `json:"rental_payment_operations"`
	Refund            *AdminRefundFact        `json:"refund"`
	Guarantee         *AdminGuaranteeSnapshot `json:"guarantee"`
	Claim             *AdminClaimFact         `json:"claim"`
}

// AdminTransition is the minimized administrative projection of reservation
// history. Free-form transition reasons may contain user-provided text and are
// intentionally omitted from this read-only view.
type AdminTransition struct {
	Sequence int64     `json:"sequence"`
	From     *string   `json:"from"`
	To       string    `json:"to"`
	Actor    *string   `json:"actor_id"`
	At       time.Time `json:"at"`
}
type AdminReservationRepository interface {
	ListAdminReservations(context.Context, string, string, AdminReservationFilter, int, string) (AdminReservationPage, error)
	GetAdminReservation(context.Context, string, string, string) (AdminReservationDetail, error)
}

// AdminAuditEntry is a strictly minimized projection of the append-only local audit stream.
type AdminAuditEntry struct {
	ID            string    `json:"id"`
	OccurredAt    time.Time `json:"occurred_at"`
	ActorID       *string   `json:"actor_id"`
	ResourceType  string    `json:"resource_type"`
	ResourceID    string    `json:"resource_id"`
	Action        string    `json:"action"`
	Result        string    `json:"result"`
	ReasonCode    string    `json:"reason_code"`
	CorrelationID string    `json:"correlation_id"`
}
type AdminAuditFilter struct {
	From         time.Time `json:"from"`
	Until        time.Time `json:"until"`
	ActorID      string    `json:"actor_id,omitempty"`
	ResourceType string    `json:"resource_type,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Action       string    `json:"action,omitempty"`
	Result       string    `json:"result,omitempty"`
}
type AdminAuditPage struct {
	Items      []AdminAuditEntry `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}
type AdminAuditExport struct {
	Version     int               `json:"version"`
	GeneratedAt time.Time         `json:"generated_at"`
	Filters     AdminAuditFilter  `json:"filters"`
	Events      []AdminAuditEntry `json:"events"`
}
type AdminAuditRepository interface {
	ListAdminAudit(context.Context, string, string, AdminAuditFilter, int, string) (AdminAuditPage, error)
	ExportAdminAudit(context.Context, string, string, AdminAuditFilter) (AdminAuditExport, error)
	RecordAdminAuditAttempt(context.Context, string, string, string, string, string) error
}
type RequestInput struct {
	QuoteID string `json:"quote_id"`
}
type PaymentInput struct {
	Outcome string `json:"outcome"`
}

// PaymentOperation is a durable local payment intent. It is written before
// the fake adapter is invoked, so an idempotent retry can query/reconcile the
// existing operation without initiating another charge.
type PaymentOperation struct {
	ID               string    `json:"id"`
	ReservationID    string    `json:"reservation_id"`
	RenterID         string    `json:"renter_id"`
	IdempotencyKey   string    `json:"-"`
	Fingerprint      []byte    `json:"-"`
	Requested        string    `json:"-"`
	State            string    `json:"state"`
	CreatedAt        time.Time `json:"created_at"`
	ReservationState string    `json:"-"`
	PayExpiresAt     time.Time `json:"-"`
}

// PaymentEvent contains a provider result. Signature is accepted only at the
// local fake callback boundary and is never persisted or serialized.
type PaymentEvent struct {
	EventID     string `json:"event_id"`
	OperationID string `json:"operation_id"`
	Outcome     string `json:"outcome"`
	Signature   string `json:"-"`
}

type PaymentEventInput struct {
	EventID     string `json:"event_id"`
	OperationID string `json:"operation_id"`
	Outcome     string `json:"outcome"`
}

type PaymentEventReceipt struct {
	Accepted bool `json:"accepted"`
	Reused   bool `json:"reused"`
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

const LocalGuaranteePolicyVersion = "garantia_local_fija_v1"

type GuaranteeSnapshot struct {
	PolicyVersion         string               `json:"policy_version"`
	Currency              string               `json:"currency"`
	ExpectedCLP           int64                `json:"expected_clp"`
	AuthorizedCLP         int64                `json:"authorized_clp"`
	CapturedCLP           int64                `json:"captured_clp"`
	ReleasedCLP           int64                `json:"released_clp"`
	State                 string               `json:"state"`
	AuthorizationDeadline *time.Time           `json:"authorization_deadline,omitempty"`
	Operations            []GuaranteeOperation `json:"operations"`
	Decision              *FinancialDecision   `json:"financial_decision,omitempty"`
}

type GuaranteeOperation struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	AmountCLP  int64     `json:"amount_clp"`
	State      string    `json:"state"`
	LastResult string    `json:"last_result,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type FinancialDecisionInput struct {
	ClaimID      string `json:"claim_id"`
	Outcome      string `json:"outcome"`
	DeductionCLP int64  `json:"deduction_clp"`
	ReasonCode   string `json:"reason_code"`
	EvidenceID   string `json:"evidence_id,omitempty"`
}

type FinancialDecision struct {
	ID           string    `json:"id"`
	ClaimID      string    `json:"claim_id,omitempty"`
	Outcome      string    `json:"outcome"`
	DeductionCLP int64     `json:"deduction_clp"`
	ReasonCode   string    `json:"reason_code"`
	EvidenceID   *string   `json:"evidence_id,omitempty"`
	State        string    `json:"state"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Reused       bool      `json:"reused,omitempty"`
}

type FinancialEvent struct {
	EventID     string `json:"event_id"`
	OperationID string `json:"operation_id"`
	Outcome     string `json:"outcome"`
	Signature   string `json:"-"`
}

type FinancialEventReceipt struct {
	Accepted bool `json:"accepted"`
	Reused   bool `json:"reused"`
}

type GuaranteeOperationInput struct {
	Kind      string `json:"kind"`
	AmountCLP int64  `json:"amount_clp"`
	Outcome   string `json:"outcome"`
}

type GuaranteeLifecycleRepository interface {
	Guarantee(context.Context, string, string) (GuaranteeSnapshot, error)
	GuaranteeForAdministrator(context.Context, string) (GuaranteeSnapshot, error)
	RunGuaranteeOperation(context.Context, string, string, string, string, int64, string, string, bool, func() time.Time) (GuaranteeOperation, bool, error)
	DecideGuarantee(context.Context, string, string, FinancialDecisionInput, string, []byte, func() time.Time) (FinancialDecision, bool, error)
	ResolveGuaranteeOperation(context.Context, string, string, string, string, func() time.Time) (GuaranteeOperation, error)
	ExpireDueGuarantees(context.Context, func() time.Time) error
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
	Decide(context.Context, string, string, string, string, func() time.Time) (Reservation, error)
	Cancel(context.Context, string, string, string, string, []byte, func() time.Time) (CancellationResult, error)
	CancellationPreview(context.Context, string, string, time.Time) (CancellationPreview, error)
	RefundOperation(context.Context, string, string) (RefundResult, error)
	RecordRefund(context.Context, string, string, string, time.Time) (RefundResult, error)
	NoticeRecipients(context.Context, string, string) ([]string, error)
	Expire(context.Context, time.Time) error
}

// ArchiveSectionRepository exposes an owner-scoped, read-only projection of
// booking-owned facts for the local privacy archive. Implementations must omit
// counterpart identifiers and unstructured shared text.
type ArchiveSectionRepository interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
}

// PaymentLifecycleRepository separates durable event handling from the
// broader booking repository contract so non-payment test doubles stay small.
type PaymentLifecycleRepository interface {
	BeginPayment(context.Context, string, string, string, string, []byte, string, func() time.Time) (PaymentOperation, bool, error)
	PendingPayments(context.Context, int) ([]PaymentOperation, error)
	PendingPaymentEventIDs(context.Context, int) ([]string, error)
	RecordPaymentEvent(context.Context, PaymentEvent, []byte, time.Time) (bool, error)
	ApplyPaymentEvent(context.Context, string, func() time.Time, time.Duration) (Reservation, error)
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
	StartPayment(context.Context, string, string) (*PaymentEvent, error)
	LookupPayment(context.Context, PaymentOperation) (*PaymentEvent, error)
	VerifyPaymentEvent(PaymentEvent) bool
}

var uuid = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
