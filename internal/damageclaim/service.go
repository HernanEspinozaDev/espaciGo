package damageclaim

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Service struct {
	repo Repository
	now  func() time.Time
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
