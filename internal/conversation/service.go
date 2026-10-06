package conversation

import (
	"context"
	"crypto/sha256"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const (
	DefaultPageSize        = 30
	MaxPageSize            = 100
	MaxMessageRunes        = 2000
	MaxIdempotencyKeyRunes = 200
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Service struct {
	repo Repository
	ids  identity.CredentialGenerator
	now  func() time.Time
}

func NewService(repo Repository, ids identity.CredentialGenerator, now func() time.Time) (*Service, error) {
	if repo == nil || ids == nil || now == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, ids: ids, now: now}, nil
}

func (s *Service) List(ctx context.Context, actor, reservationID string, before *int64, limit int) (Page, error) {
	if !uuidPattern.MatchString(actor) || !uuidPattern.MatchString(reservationID) || (before != nil && *before < 1) || limit < 1 || limit > MaxPageSize {
		return Page{}, ErrInvalid
	}
	return s.repo.List(ctx, actor, reservationID, before, limit)
}

func (s *Service) Send(ctx context.Context, actor, reservationID, key, body string) (Message, error) {
	if !uuidPattern.MatchString(actor) || !uuidPattern.MatchString(reservationID) || !validKey(key) || !validBody(body) {
		return Message{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Message{}, err
	}
	fingerprint := sha256.Sum256([]byte(body))
	return s.repo.Send(ctx, actor, reservationID, key, body, fingerprint[:], id, s.now)
}

func validKey(v string) bool {
	return strings.TrimSpace(v) != "" && utf8.ValidString(v) && utf8.RuneCountInString(v) <= MaxIdempotencyKeyRunes
}

func validBody(v string) bool {
	if !utf8.ValidString(v) || strings.TrimSpace(v) == "" {
		return false
	}
	runes := utf8.RuneCountInString(v)
	return runes >= 1 && runes <= MaxMessageRunes
}
