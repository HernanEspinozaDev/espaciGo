package spaces

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

type Service struct{ repo Repository }

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("spaces: repository required")
	}
	return &Service{repo: repo}, nil
}

func (s *Service) Categories(ctx context.Context) ([]Category, error) { return s.repo.Categories(ctx) }
func (s *Service) Create(ctx context.Context, owner string, in Input) (Draft, error) {
	if !validOwner(owner) || in.Validate() != nil {
		return Draft{}, ErrInvalid
	}
	return s.repo.Create(ctx, owner, in)
}
func (s *Service) ListOwn(ctx context.Context, owner string) ([]Draft, error) {
	if !validOwner(owner) {
		return nil, ErrInvalid
	}
	return s.repo.ListOwn(ctx, owner)
}
func (s *Service) GetOwn(ctx context.Context, owner, id string) (Draft, error) {
	if !validOwner(owner) || !validID(id) {
		return Draft{}, ErrNotFound
	}
	return s.repo.GetOwn(ctx, owner, id)
}
func (s *Service) UpdateOwn(ctx context.Context, owner, id string, in Input) (Draft, error) {
	if !validOwner(owner) || !validID(id) {
		return Draft{}, ErrNotFound
	}
	if in.Validate() != nil {
		return Draft{}, ErrInvalid
	}
	return s.repo.UpdateOwn(ctx, owner, id, in)
}

var uuidPattern = func() *regexp.Regexp {
	return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
}()

func validID(v string) bool    { return uuidPattern.MatchString(v) }
func validOwner(v string) bool { return validID(strings.TrimSpace(v)) }
