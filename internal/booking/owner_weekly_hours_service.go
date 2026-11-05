package booking

import (
	"context"
	"time"
)

// OwnerWeeklyHoursService manages a space owner's schedule without requiring
// the payment, reservation, or worker dependencies of the local booking service.
type OwnerWeeklyHoursService struct {
	repo OwnerWeeklyHoursRepository
}

func NewOwnerWeeklyHoursService(repo OwnerWeeklyHoursRepository) *OwnerWeeklyHoursService {
	return &OwnerWeeklyHoursService{repo: repo}
}

func (s *OwnerWeeklyHoursService) OwnerWeeklyHours(ctx context.Context, owner, spaceID string) (WeeklyHours, error) {
	if !uuid.MatchString(owner) || !uuid.MatchString(spaceID) {
		return WeeklyHours{}, ErrNotFound
	}
	if s == nil || s.repo == nil {
		return WeeklyHours{}, ErrNotFound
	}
	return s.repo.WeeklyHoursForOwner(ctx, owner, spaceID)
}

func (s *OwnerWeeklyHoursService) SaveOwnerWeeklyHoursCAS(ctx context.Context, owner, spaceID string, value WeeklyHours, expected time.Time) (WeeklyHours, error) {
	if !uuid.MatchString(owner) || !uuid.MatchString(spaceID) || expected.IsZero() || ValidateWeeklyHours(value) != nil {
		return WeeklyHours{}, ErrInvalid
	}
	if s == nil || s.repo == nil {
		return WeeklyHours{}, ErrNotFound
	}
	value.Days = normalizeWeeklyDays(value.Days)
	return s.repo.SaveWeeklyHoursForOwnerCAS(ctx, owner, spaceID, value, expected.UTC())
}
