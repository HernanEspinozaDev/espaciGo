package damageclaim

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("damage claim: invalid request")
	ErrNotFound = errors.New("damage claim: resource not found")
	ErrConflict = errors.New("damage claim: state or deadline conflict")
)

const SafetyNotice = "ENSAYO LOCAL — RECLAMO SINTÉTICO; SIN ADJUDICACIÓN NI MOVIMIENTO DE FONDOS"

type Claim struct {
	ID                  string          `json:"id"`
	ReservationID       string          `json:"reservation_id"`
	HostID              string          `json:"host_id"`
	RenterID            string          `json:"renter_id"`
	CheckoutOperationID string          `json:"checkout_operation_id"`
	CheckoutEvidenceID  string          `json:"checkout_evidence_id"`
	Description         string          `json:"description"`
	State               string          `json:"state"`
	OpenedAt            time.Time       `json:"opened_at"`
	ClaimDeadlineAt     time.Time       `json:"claim_deadline_at"`
	Reused              bool            `json:"reused,omitempty"`
	Defense             *Defense        `json:"defense,omitempty"`
	Resolution          *Resolution     `json:"resolution,omitempty"`
	History             []Transition    `json:"history,omitempty"`
	Evidence            []ClaimEvidence `json:"evidence,omitempty"`
}

type ClaimEvidence struct {
	ID         string    `json:"id"`
	Operation  string    `json:"operation"`
	Fixture    string    `json:"fixture_code"`
	MIME       string    `json:"mime_type"`
	SHA256     string    `json:"sha256"`
	SizeBytes  int64     `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
	ContentURL string    `json:"content_url"`
}

type Resolution struct {
	Outcome    string    `json:"outcome"`
	ReasonCode string    `json:"reason_code"`
	ActorID    string    `json:"actor_id"`
	ResolvedAt time.Time `json:"resolved_at"`
	Reused     bool      `json:"reused,omitempty"`
}

type Transition struct {
	Sequence   int64     `json:"sequence"`
	Action     string    `json:"action"`
	ActorID    string    `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Defense struct {
	ID          string    `json:"id"`
	ActorID     string    `json:"actor_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Reused      bool      `json:"reused,omitempty"`
}

type Input struct {
	Description string `json:"description"`
}
type ResolutionInput struct {
	Outcome    string `json:"outcome"`
	ReasonCode string `json:"reason_code"`
}
type Repository interface {
	Open(context.Context, string, string, string, Input, []byte, func() time.Time) (Claim, error)
	Get(context.Context, string, string) (Claim, error)
	Defend(context.Context, string, string, string, Input, []byte, func() time.Time) (Defense, error)
	ListOpen(context.Context) ([]Claim, error)
	GetAdmin(context.Context, string) (Claim, error)
	Resolve(context.Context, string, string, string, ResolutionInput, []byte, func() time.Time) (Claim, error)
	AdminEvidence(context.Context, string, string) (ClaimEvidence, error)
}

type PrivateFileStore interface {
	Get(context.Context, string) ([]byte, error)
}
