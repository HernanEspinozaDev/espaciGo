package dispute

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// ArchiveSectionRepository provides only participant-owned dispute facts and
// actor labels, never participant identifiers.
type ArchiveSectionRepository interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
}

var (
	ErrInvalid  = errors.New("dispute: invalid input")
	ErrNotFound = errors.New("dispute: not found")
	ErrConflict = errors.New("dispute: conflict")
	uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

const OpeningReason = "ensayo_privacidad"

var CloseReasons = map[string]struct{}{
	"ensayo_finalizado": {},
	"registro_erroneo":  {},
	"duplicada":         {},
}

type Dispute struct {
	ID            string     `json:"id"`
	ReservationID string     `json:"reservation_id"`
	HostID        string     `json:"host_id"`
	RenterID      string     `json:"renter_id"`
	OpenedBy      string     `json:"opened_by"`
	OpeningReason string     `json:"opening_reason_code"`
	State         string     `json:"state"`
	OpenedAt      time.Time  `json:"opened_at"`
	ClosedBy      *string    `json:"closed_by,omitempty"`
	CloseReason   *string    `json:"close_reason_code,omitempty"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	Reused        bool       `json:"reused"`
}

type Transition struct {
	Sequence      int64     `json:"sequence"`
	PreviousState *string   `json:"previous_state,omitempty"`
	NewState      string    `json:"new_state"`
	ActorID       string    `json:"actor_id"`
	ReasonCode    string    `json:"reason_code"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type Repository interface {
	Open(context.Context, string, string, string, string, []byte, func() time.Time) (Dispute, error)
	ListForParticipant(context.Context, string, string) ([]Dispute, error)
	ListOpen(context.Context) ([]Dispute, error)
	History(context.Context, string, string) ([]Transition, error)
	Close(context.Context, string, string, string, func() time.Time) (Dispute, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, now func() time.Time) (*Service, error) {
	if repo == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}, nil
}

func (s *Service) Open(ctx context.Context, hostID, reservationID, reasonCode, idempotencyKey string) (Dispute, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !uuidPattern.MatchString(hostID) || !uuidPattern.MatchString(reservationID) || reasonCode != OpeningReason || len(idempotencyKey) < 1 || len(idempotencyKey) > 200 {
		return Dispute{}, ErrInvalid
	}
	fingerprint := OpeningFingerprint(reservationID, reasonCode)
	return s.repo.Open(ctx, strings.ToLower(hostID), strings.ToLower(reservationID), reasonCode, idempotencyKey, fingerprint, s.now)
}

func (s *Service) ListForParticipant(ctx context.Context, actorID, reservationID string) ([]Dispute, error) {
	if !uuidPattern.MatchString(actorID) || !uuidPattern.MatchString(reservationID) {
		return nil, ErrNotFound
	}
	return s.repo.ListForParticipant(ctx, strings.ToLower(actorID), strings.ToLower(reservationID))
}

func (s *Service) ListOpen(ctx context.Context) ([]Dispute, error) {
	return s.repo.ListOpen(ctx)
}

func (s *Service) History(ctx context.Context, actorID, disputeID string) ([]Transition, error) {
	if !uuidPattern.MatchString(actorID) || !uuidPattern.MatchString(disputeID) {
		return nil, ErrNotFound
	}
	return s.repo.History(ctx, strings.ToLower(actorID), strings.ToLower(disputeID))
}

func (s *Service) Close(ctx context.Context, administratorID, disputeID, reasonCode string) (Dispute, error) {
	if !uuidPattern.MatchString(administratorID) || !uuidPattern.MatchString(disputeID) {
		return Dispute{}, ErrNotFound
	}
	if _, ok := CloseReasons[reasonCode]; !ok {
		return Dispute{}, ErrInvalid
	}
	return s.repo.Close(ctx, strings.ToLower(administratorID), strings.ToLower(disputeID), reasonCode, s.now)
}

func OpeningFingerprint(reservationID, reasonCode string) []byte {
	fingerprint := sha256.Sum256([]byte(strings.ToLower(reservationID) + "\x00" + reasonCode))
	return fingerprint[:]
}
