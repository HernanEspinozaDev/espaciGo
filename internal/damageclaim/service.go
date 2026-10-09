package damageclaim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Service struct {
	repo  Repository
	files PrivateFileStore
	now   func() time.Time
}

func New(repo Repository, now func() time.Time) (*Service, error) {
	if repo == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}, nil
}

func NewWithEvidenceStore(repo Repository, files PrivateFileStore, now func() time.Time) (*Service, error) {
	service, err := New(repo, now)
	if err != nil {
		return nil, err
	}
	if files == nil {
		return nil, ErrInvalid
	}
	service.files = files
	return service, nil
}
func (s *Service) Open(ctx context.Context, actor, reservation, key string, input Input) (Claim, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	reservation = strings.ToLower(strings.TrimSpace(reservation))
	key = strings.TrimSpace(key)
	input.Description = strings.TrimSpace(input.Description)
	if !uuidPattern.MatchString(actor) || !uuidPattern.MatchString(reservation) || len(key) < 8 || len(key) > 200 || input.Description == "" {
		return Claim{}, ErrInvalid
	}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return s.repo.Open(ctx, actor, reservation, key, input, sum[:], s.now)
}
func (s *Service) Get(ctx context.Context, actor, reservation string) (Claim, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	reservation = strings.ToLower(strings.TrimSpace(reservation))
	if !uuidPattern.MatchString(actor) || !uuidPattern.MatchString(reservation) {
		return Claim{}, ErrNotFound
	}
	return s.repo.Get(ctx, actor, reservation)
}
func (s *Service) Defend(ctx context.Context, actor, reservation, key string, input Input) (Defense, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	reservation = strings.ToLower(strings.TrimSpace(reservation))
	key = strings.TrimSpace(key)
	input.Description = strings.TrimSpace(input.Description)
	if !uuidPattern.MatchString(actor) || !uuidPattern.MatchString(reservation) || len(key) < 8 || len(key) > 200 || input.Description == "" {
		return Defense{}, ErrInvalid
	}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return s.repo.Defend(ctx, actor, reservation, key, input, sum[:], s.now)
}

var resolutionReasons = map[string]struct{}{
	"evidencia_suficiente":     {},
	"evidencia_insuficiente":   {},
	"hecho_no_acreditado":      {},
	"informacion_insuficiente": {},
}

func (s *Service) ListOpen(ctx context.Context) ([]Claim, error) {
	return s.repo.ListOpen(ctx)
}

func (s *Service) GetAdmin(ctx context.Context, claimID string) (Claim, error) {
	claimID = strings.ToLower(strings.TrimSpace(claimID))
	if !uuidPattern.MatchString(claimID) {
		return Claim{}, ErrNotFound
	}
	return s.repo.GetAdmin(ctx, claimID)
}

func (s *Service) Resolve(ctx context.Context, administrator, claimID, key string, input ResolutionInput) (Claim, error) {
	administrator = strings.ToLower(strings.TrimSpace(administrator))
	claimID = strings.ToLower(strings.TrimSpace(claimID))
	key = strings.TrimSpace(key)
	input.Outcome = strings.TrimSpace(input.Outcome)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if !uuidPattern.MatchString(administrator) || !uuidPattern.MatchString(claimID) || len(key) < 8 || len(key) > 200 {
		return Claim{}, ErrInvalid
	}
	if input.Outcome != "acogido" && input.Outcome != "rechazado" {
		return Claim{}, ErrInvalid
	}
	if _, ok := resolutionReasons[input.ReasonCode]; !ok {
		return Claim{}, ErrInvalid
	}
	if input.Outcome == "acogido" && input.ReasonCode != "evidencia_suficiente" || input.Outcome == "rechazado" && input.ReasonCode == "evidencia_suficiente" {
		return Claim{}, ErrInvalid
	}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return s.repo.Resolve(ctx, administrator, claimID, key, input, sum[:], s.now)
}

func (s *Service) AdminEvidenceContent(ctx context.Context, claimID, evidenceID string) (ClaimEvidence, []byte, error) {
	claimID, evidenceID = strings.ToLower(strings.TrimSpace(claimID)), strings.ToLower(strings.TrimSpace(evidenceID))
	if !uuidPattern.MatchString(claimID) || !uuidPattern.MatchString(evidenceID) || s.files == nil {
		return ClaimEvidence{}, nil, ErrNotFound
	}
	evidence, err := s.repo.AdminEvidence(ctx, claimID, evidenceID)
	if err != nil {
		return ClaimEvidence{}, nil, err
	}
	content, err := s.files.Get(ctx, evidence.ID)
	if err != nil {
		return ClaimEvidence{}, nil, ErrNotFound
	}
	digest := sha256.Sum256(content)
	if int64(len(content)) != evidence.SizeBytes || hex.EncodeToString(digest[:]) != evidence.SHA256 {
		return ClaimEvidence{}, nil, ErrNotFound
	}
	return evidence, content, nil
}
