package operation

import (
	"context"
	"errors"
	"testing"
)

type returnsTestRepository struct {
	Repository
	actor, space string
	items        []SpaceReturn
	err          error
}

func (r *returnsTestRepository) ListSpaceReturns(_ context.Context, actor, space string) ([]SpaceReturn, error) {
	r.actor, r.space = actor, space
	return r.items, r.err
}
func TestListSpaceReturnsValidatesScopeAndPropagatesAuthorization(t *testing.T) {
	ctx := context.Background()
	repo := &returnsTestRepository{err: ErrNotFound}
	s := &Service{repo: repo}
	if _, err := s.ListSpaceReturns(ctx, operationTestID, "bad-space"); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid space accepted", err)
	}
	if repo.actor != "" {
		t.Fatal("invalid scope reached repository")
	}
	if _, err := s.ListSpaceReturns(ctx, operationTestID, operationTestID); !errors.Is(err, ErrNotFound) {
		t.Fatal("authorization failure swallowed", err)
	}
	repo.err = nil
	got, err := s.ListSpaceReturns(ctx, operationTestID, operationTestID)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty returns must be a JSON array", err)
	}
	if repo.actor != operationTestID || repo.space != operationTestID {
		t.Fatal("owner/space scope lost")
	}
}
