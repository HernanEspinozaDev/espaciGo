package damageclaim

import (
	"context"
	"encoding/json"
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

func (claim Claim) MarshalJSON() ([]byte, error) {
	type response struct {
		ID                  string          `json:"id"`
		ReservationID       *string         `json:"reservation_id"`
		HostID              *string         `json:"host_id"`
		RenterID            *string         `json:"renter_id"`
		CheckoutOperationID *string         `json:"checkout_operation_id"`
		CheckoutEvidenceID  *string         `json:"checkout_evidence_id"`
		Description         string          `json:"description"`
		State               string          `json:"state"`
		OpenedAt            time.Time       `json:"opened_at"`
		ClaimDeadlineAt     time.Time       `json:"claim_deadline_at"`
		Reused              bool            `json:"reused,omitempty"`
		Defense             *Defense        `json:"defense,omitempty"`
		Resolution          *Resolution     `json:"resolution,omitempty"`
		Evidence            []ClaimEvidence `json:"evidence"`
		History             []Transition    `json:"history"`
	}
	return json.Marshal(response{
		ID: claim.ID, ReservationID: nullableIdentifier(claim.ReservationID),
		HostID: nullableIdentifier(claim.HostID), RenterID: nullableIdentifier(claim.RenterID),
		CheckoutOperationID: nullableIdentifier(claim.CheckoutOperationID), CheckoutEvidenceID: nullableIdentifier(claim.CheckoutEvidenceID),
		Description: claim.Description, State: claim.State, OpenedAt: claim.OpenedAt, ClaimDeadlineAt: claim.ClaimDeadlineAt,
		Reused: claim.Reused, Defense: claim.Defense, Resolution: claim.Resolution,
		Evidence: nonNilEvidence(claim.Evidence), History: nonNilTransitions(claim.History),
	})
}

func nullableIdentifier(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nonNilEvidence(items []ClaimEvidence) []ClaimEvidence {
	if items == nil {
		return []ClaimEvidence{}
	}
	return items
}

func nonNilTransitions(items []Transition) []Transition {
	if items == nil {
		return []Transition{}
	}
	return items
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

func (resolution Resolution) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Outcome    string    `json:"outcome"`
		ReasonCode string    `json:"reason_code"`
		ActorID    *string   `json:"actor_id"`
		ResolvedAt time.Time `json:"resolved_at"`
		Reused     bool      `json:"reused,omitempty"`
	}{resolution.Outcome, resolution.ReasonCode, nullableIdentifier(resolution.ActorID), resolution.ResolvedAt, resolution.Reused})
}

type Transition struct {
	Sequence   int64     `json:"sequence"`
	Action     string    `json:"action"`
	ActorID    string    `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (transition Transition) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sequence   int64     `json:"sequence"`
		Action     string    `json:"action"`
		ActorID    *string   `json:"actor_id"`
		OccurredAt time.Time `json:"occurred_at"`
	}{transition.Sequence, transition.Action, nullableIdentifier(transition.ActorID), transition.OccurredAt})
}

type Defense struct {
	ID          string    `json:"id"`
	ActorID     string    `json:"actor_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Reused      bool      `json:"reused,omitempty"`
}

func (defense Defense) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID          string    `json:"id"`
		ActorID     *string   `json:"actor_id"`
		Description string    `json:"description"`
		CreatedAt   time.Time `json:"created_at"`
		Reused      bool      `json:"reused,omitempty"`
	}{defense.ID, nullableIdentifier(defense.ActorID), defense.Description, defense.CreatedAt, defense.Reused})
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
