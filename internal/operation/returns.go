package operation

import (
	"context"
	"time"
)

// SpaceReturn is an owner-only operational projection. Contract parties,
// identity, payment and guarantee data are deliberately not part of this DTO.
type SpaceReturn struct {
	ReservationID    string    `json:"reservation_id"`
	State            string    `json:"state"`
	StartAt          time.Time `json:"start_at"`
	EndAt            time.Time `json:"end_at"`
	CheckOutRecorded bool      `json:"check_out_recorded"`
	VacancyConfirmed bool      `json:"vacancy_confirmed"`
}

// SpaceReturnsRepository is separate from the private participant history port:
// the current owner may manage a return without inheriting the old contract.
type SpaceReturnsRepository interface {
	ListSpaceReturns(context.Context, string, string) ([]SpaceReturn, error)
}

func (s *Service) ListSpaceReturns(ctx context.Context, actor, space string) ([]SpaceReturn, error) {
	if !idPattern.MatchString(actor) || !idPattern.MatchString(space) {
		return nil, ErrInvalid
	}
	repo, ok := s.repo.(SpaceReturnsRepository)
	if !ok {
		return nil, ErrNotFound
	}
	items, err := repo.ListSpaceReturns(ctx, actor, space)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = make([]SpaceReturn, 0)
	}
	return items, nil
}
