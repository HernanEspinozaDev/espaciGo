package verification

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid verification input")
	ErrNotFound = errors.New("verification not found")
	ErrConflict = errors.New("verification state conflict")
)

type Case struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"-"`
	Type             string     `json:"type"`
	State            string     `json:"state"`
	Provider         string     `json:"provider"`
	EvidenceRef      string     `json:"evidence_ref"`
	RetryOf          string     `json:"retry_of,omitempty"`
	ReviewerID       string     `json:"-"`
	ReasonCode       string     `json:"reason_code,omitempty"`
	CorrectionCode   string     `json:"correction_code,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevocationReason string     `json:"revocation_reason_code,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	Idempotency      string     `json:"-"`
}

type Eligibility struct {
	Type             string     `json:"type"`
	Eligible         bool       `json:"eligible"`
	VerificationID   string     `json:"verification_id,omitempty"`
	GrantedAt        time.Time  `json:"granted_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevocationReason string     `json:"revocation_reason_code,omitempty"`
}

type HistoryEntry struct {
	Sequence   int64     `json:"sequence"`
	Action     string    `json:"action"`
	FromState  string    `json:"from_state,omitempty"`
	ToState    string    `json:"to_state"`
	ReasonCode string    `json:"reason_code,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Repository interface {
	Create(context.Context, Case) (Case, error)
	GetOwn(context.Context, string, string) (Case, error)
	ListOwn(context.Context, string) ([]Case, error)
	ListPending(context.Context) ([]Case, error)
	ListRejected(context.Context) ([]Case, error)
	ListHistory(context.Context, string, string) ([]HistoryEntry, error)
	ListEligibility(context.Context, string) ([]Eligibility, error)
	Review(context.Context, string, string, bool, string, func() time.Time) (Case, error)
	Revoke(context.Context, string, string, string, string, string, func() time.Time) (Case, error)
	Retry(context.Context, string, string, Case, func() time.Time) (Case, error)
}

type IDGenerator interface{ ID() (string, error) }
type Provider interface {
	Name() string
	Submit(context.Context, string, string) (string, error)
}
type Service struct {
	repo     Repository
	ids      IDGenerator
	provider Provider
	now      func() time.Time
}

func NewService(repo Repository, ids IDGenerator, provider Provider, now func() time.Time) (*Service, error) {
	if repo == nil || ids == nil || provider == nil || provider.Name() != "local-fixture-v1" || now == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, ids: ids, provider: provider, now: now}, nil
}

type LocalFixtureProvider struct{}

func (LocalFixtureProvider) Name() string { return "local-fixture-v1" }
func (LocalFixtureProvider) Submit(_ context.Context, kind, evidenceRef string) (string, error) {
	if (kind != "kyc" && kind != "kyb") || !strings.HasPrefix(evidenceRef, "fixture:") {
		return "", ErrInvalid
	}
	return "en_revision", nil
}

// Start stores only a synthetic evidence reference; this local fixture is not identity proof.
func (s *Service) Start(ctx context.Context, owner, kind, idempotency string) (Case, error) {
	if owner == "" || (kind != "kyc" && kind != "kyb") || len(idempotency) < 8 || len(idempotency) > 80 || strings.TrimSpace(idempotency) != idempotency {
		return Case{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Case{}, err
	}
	ref, err := s.ids.ID()
	if err != nil {
		return Case{}, err
	}
	now := s.now().UTC()
	evidenceRef := "fixture:" + ref
	state, err := s.provider.Submit(ctx, kind, evidenceRef)
	if err != nil || state != "en_revision" {
		return Case{}, ErrInvalid
	}
	created, err := s.repo.Create(ctx, Case{ID: id, OwnerID: owner, Type: kind, State: state, Provider: s.provider.Name(), EvidenceRef: evidenceRef, CreatedAt: now, Idempotency: idempotency})
	if err != nil {
		return Case{}, err
	}
	if created.OwnerID != owner || created.Type != kind {
		return Case{}, ErrConflict
	}
	return created, nil
}

func (s *Service) Own(ctx context.Context, owner, id string) (Case, error) {
	if owner == "" || id == "" {
		return Case{}, ErrInvalid
	}
	return s.repo.GetOwn(ctx, owner, id)
}
func (s *Service) ListOwn(ctx context.Context, owner string) ([]Case, error) {
	if owner == "" {
		return nil, ErrInvalid
	}
	return s.repo.ListOwn(ctx, owner)
}
func (s *Service) Pending(ctx context.Context) ([]Case, error)  { return s.repo.ListPending(ctx) }
func (s *Service) Rejected(ctx context.Context) ([]Case, error) { return s.repo.ListRejected(ctx) }
func (s *Service) Review(ctx context.Context, id, reviewer, outcome, reason string) (Case, error) {
	if id == "" || reviewer == "" || (outcome != "aprobada" && outcome != "rechazada") {
		return Case{}, ErrInvalid
	}
	reason = strings.TrimSpace(reason)
	if outcome == "rechazada" && !validReason(reason) {
		return Case{}, ErrInvalid
	}
	if outcome == "aprobada" && reason != "" {
		return Case{}, ErrInvalid
	}
	return s.repo.Review(ctx, id, reviewer, outcome == "aprobada", reason, s.now)
}

func (s *Service) Revoke(ctx context.Context, id, reviewer, reason, idempotencyKey, correlationID string) (Case, error) {
	if id == "" || reviewer == "" || !validRevocationReason(reason) || len(idempotencyKey) < 8 || len(idempotencyKey) > 200 || strings.TrimSpace(correlationID) == "" || len(correlationID) > 120 {
		return Case{}, ErrInvalid
	}
	return s.repo.Revoke(ctx, id, reviewer, reason, idempotencyKey, correlationID, s.now)
}

func (s *Service) Eligibility(ctx context.Context, owner string) ([]Eligibility, error) {
	if owner == "" {
		return nil, ErrInvalid
	}
	return s.repo.ListEligibility(ctx, owner)
}

func (s *Service) History(ctx context.Context, owner, id string) ([]HistoryEntry, error) {
	if owner == "" || id == "" {
		return nil, ErrInvalid
	}
	return s.repo.ListHistory(ctx, owner, id)
}

func validRevocationReason(code string) bool {
	return code == "aprobacion_fixture_incorrecta" || code == "revision_fixture_actualizada"
}
func validReason(code string) bool {
	return code == "documento_vencido" || code == "antecedentes_incompletos" || code == "inicio_actividades_no_confirmado"
}
func (s *Service) Retry(ctx context.Context, owner, priorID, idempotency, correctionCode string) (Case, error) {
	if owner == "" || priorID == "" || len(idempotency) < 8 || len(idempotency) > 80 {
		return Case{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Case{}, err
	}
	ref, err := s.ids.ID()
	if err != nil {
		return Case{}, err
	}
	prior, err := s.repo.GetOwn(ctx, owner, priorID)
	if err != nil {
		return Case{}, err
	}
	if prior.State != "rechazada" || !validCorrection(prior.ReasonCode, correctionCode) {
		return Case{}, ErrInvalid
	}
	state, err := s.provider.Submit(ctx, prior.Type, "fixture:"+ref)
	if err != nil || state != "en_revision" {
		return Case{}, ErrInvalid
	}
	created, err := s.repo.Retry(ctx, owner, priorID, Case{ID: id, OwnerID: owner, Provider: s.provider.Name(), EvidenceRef: "fixture:" + ref, State: state, CorrectionCode: correctionCode, CreatedAt: s.now().UTC(), Idempotency: idempotency}, s.now)
	if err != nil {
		return Case{}, err
	}
	if created.OwnerID != owner || created.RetryOf != priorID {
		return Case{}, ErrConflict
	}
	return created, nil
}

func validCorrection(reason, correction string) bool {
	switch reason {
	case "documento_vencido":
		return correction == "fixture_vigente_actualizado"
	case "antecedentes_incompletos":
		return correction == "antecedentes_fixture_actualizados"
	case "inicio_actividades_no_confirmado":
		return correction == "inicio_actividades_fixture_actualizadas"
	default:
		return false
	}
}
