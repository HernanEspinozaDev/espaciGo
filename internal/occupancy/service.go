package occupancy

import (
	"context"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // Ship IANA data with minimal containers that lack zoneinfo files.

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Service struct {
	repo Repository
	ids  identity.CredentialGenerator
}

func NewService(repo Repository, ids identity.CredentialGenerator) (*Service, error) {
	if repo == nil || ids == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, ids: ids}, nil
}

func (s *Service) SetTimeZone(ctx context.Context, owner, spaceID, zone string) error {
	if !validUUID(owner) || !validUUID(spaceID) || !validTimeZone(zone) {
		return ErrInvalid
	}
	return s.repo.SetTimeZone(ctx, owner, spaceID, zone)
}

func (s *Service) TimeZone(ctx context.Context, owner, spaceID string) (string, error) {
	if !validUUID(owner) || !validUUID(spaceID) {
		return "", ErrInvalid
	}
	return s.repo.TimeZone(ctx, owner, spaceID)
}

func (s *Service) Availability(ctx context.Context, owner, spaceID, from, to string) (Availability, error) {
	start, end, err := parseInterval(from, to)
	if err != nil || !validUUID(owner) || !validUUID(spaceID) {
		return Availability{}, ErrInvalid
	}
	return s.repo.Availability(ctx, owner, spaceID, start, end)
}

func (s *Service) ListBlocks(ctx context.Context, owner, spaceID, from, to string) (Calendar, error) {
	start, end, err := parseInterval(from, to)
	if err != nil || !validUUID(owner) || !validUUID(spaceID) {
		return Calendar{}, ErrInvalid
	}
	return s.repo.ListBlocks(ctx, owner, spaceID, start, end)
}

func (s *Service) CreateBlock(ctx context.Context, owner, spaceID string, in BlockInput) (Block, error) {
	start, err := parseTimestamp(in.StartAt)
	if err != nil {
		return Block{}, ErrInvalid
	}
	end, err := parseTimestamp(in.EndAt)
	if err != nil || !end.After(start) || strings.TrimSpace(in.Reason) == "" || len([]rune(in.Reason)) > 500 || !validUUID(owner) || !validUUID(spaceID) {
		return Block{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Block{}, err
	}
	return s.repo.CreateBlock(ctx, owner, spaceID, id, start, end, strings.TrimSpace(in.Reason))
}

func (s *Service) DeleteBlock(ctx context.Context, owner, spaceID, blockID string) error {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(blockID) {
		return ErrNotFound
	}
	return s.repo.DeleteBlock(ctx, owner, spaceID, blockID)
}

func parseInterval(from, to string) (time.Time, time.Time, error) {
	start, err := parseTimestamp(from)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalid
	}
	end, err := parseTimestamp(to)
	if err != nil || !end.After(start) {
		return time.Time{}, time.Time{}, ErrInvalid
	}
	return start, end, nil
}

func parseTimestamp(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, ErrInvalid
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || t.IsZero() {
		return time.Time{}, ErrInvalid
	}
	return t.UTC(), nil
}

func validTimeZone(zone string) bool {
	if strings.TrimSpace(zone) != zone || zone == "" || zone == "Local" || strings.Contains(zone, "..") {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func validUUID(value string) bool { return uuidPattern.MatchString(value) }
