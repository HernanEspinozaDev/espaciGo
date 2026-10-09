package contract

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("contract: not found")
	ErrConflict = errors.New("contract: conflict")
	ErrInvalid  = errors.New("contract: invalid request")
)

const SafetyNotice = "ENSAYO SINTÉTICO LOCAL — SIN VALIDEZ JURÍDICA"

type Signature struct {
	SignerID  string    `json:"signer_id"`
	Role      string    `json:"role"`
	State     string    `json:"state"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Event struct {
	Sequence int64     `json:"sequence"`
	ActorID  string    `json:"actor_id,omitempty"`
	Action   string    `json:"action"`
	Reason   string    `json:"reason,omitempty"`
	At       time.Time `json:"at"`
}
type Contract struct {
	ID            string      `json:"id"`
	ReservationID string      `json:"reservation_id"`
	DocumentID    string      `json:"-"`
	Version       int         `json:"version"`
	State         string      `json:"state"`
	Snapshot      []byte      `json:"snapshot"`
	SHA256        string      `json:"sha256"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Signatures    []Signature `json:"signatures"`
	History       []Event     `json:"history"`
	Artifact      []byte      `json:"-"`
}
type Repository interface {
	Create(context.Context, string, string, func() time.Time) (Contract, error)
	Get(context.Context, string, string) (Contract, error)
	Sign(context.Context, string, string, func() time.Time) (Contract, error)
	Reject(context.Context, string, string, string, func() time.Time) (Contract, error)
	EnsureApproved(context.Context, func() time.Time) (int, error)
	ExpireDue(context.Context, func() time.Time) (int, error)
}
type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, now func() time.Time) (*Service, error) {
	if repo == nil || now == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, now: now}, nil
}
func (s *Service) Create(ctx context.Context, actor, reservationID string) (Contract, error) {
	return s.repo.Create(ctx, actor, reservationID, s.now)
}
func (s *Service) Get(ctx context.Context, actor, contractID string) (Contract, error) {
	return s.repo.Get(ctx, actor, contractID)
}
func (s *Service) Sign(ctx context.Context, actor, contractID string) (Contract, error) {
	return s.repo.Sign(ctx, actor, contractID, s.now)
}
func (s *Service) Reject(ctx context.Context, actor, contractID, reason string) (Contract, error) {
	return s.repo.Reject(ctx, actor, contractID, reason, s.now)
}
func (s *Service) ExpireDue(ctx context.Context) (int, error) { return s.repo.ExpireDue(ctx, s.now) }
func (s *Service) EnsureApproved(ctx context.Context) (int, error) {
	return s.repo.EnsureApproved(ctx, s.now)
}
