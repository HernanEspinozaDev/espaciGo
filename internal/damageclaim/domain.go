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
	ID                  string    `json:"id"`
	ReservationID       string    `json:"reservation_id"`
	HostID              string    `json:"host_id"`
	RenterID            string    `json:"renter_id"`
	CheckoutOperationID string    `json:"checkout_operation_id"`
	CheckoutEvidenceID  string    `json:"checkout_evidence_id"`
	Description         string    `json:"description"`
	State               string    `json:"state"`
	OpenedAt            time.Time `json:"opened_at"`
	ClaimDeadlineAt     time.Time `json:"claim_deadline_at"`
	Reused              bool      `json:"reused,omitempty"`
	Defense             *Defense  `json:"defense,omitempty"`
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
type Repository interface {
	Open(context.Context, string, string, string, Input, []byte, func() time.Time) (Claim, error)
	Get(context.Context, string, string) (Claim, error)
	Defend(context.Context, string, string, string, Input, []byte, func() time.Time) (Defense, error)
}
