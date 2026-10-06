package pricing

import (
	"context"
	"math"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
)

type Service struct {
	repo     Repository
	calendar interface {
		Availability(context.Context, string, string, string, string) (occupancy.Availability, error)
	}
	ids identity.CredentialGenerator
}

func NewService(repo Repository, calendar interface {
	Availability(context.Context, string, string, string, string) (occupancy.Availability, error)
}, ids identity.CredentialGenerator) (*Service, error) {
	if repo == nil || calendar == nil || ids == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, calendar: calendar, ids: ids}, nil
}

func (s *Service) CurrentRate(ctx context.Context, owner, spaceID string) (Rate, error) {
	if !uuidPattern.MatchString(owner) || !uuidPattern.MatchString(spaceID) {
		return Rate{}, ErrNotFound
	}
	return s.repo.CurrentRate(ctx, owner, spaceID)
}
func (s *Service) RateHistory(ctx context.Context, owner, spaceID string) ([]Rate, error) {
	if !uuidPattern.MatchString(owner) || !uuidPattern.MatchString(spaceID) {
		return nil, ErrNotFound
	}
	return s.repo.RateHistory(ctx, owner, spaceID)
}
func (s *Service) UpdateRate(ctx context.Context, owner, spaceID string, in RateInput) (Rate, error) {
	if !uuidPattern.MatchString(owner) || !uuidPattern.MatchString(spaceID) {
		return Rate{}, ErrNotFound
	}
	if validateRate(in) != nil {
		return Rate{}, ErrInvalid
	}
	return s.repo.UpdateRate(ctx, owner, spaceID, in)
}

func (s *Service) Simulate(ctx context.Context, owner, spaceID string, in SimulationInput) (Simulation, error) {
	if !uuidPattern.MatchString(owner) || !uuidPattern.MatchString(spaceID) {
		return Simulation{}, ErrNotFound
	}
	start, end, err := parseWindow(in)
	if err != nil {
		return Simulation{}, ErrInvalid
	}
	available, err := s.calendar.Availability(ctx, owner, spaceID, start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano))
	if err != nil {
		return Simulation{}, mapCalendarError(err)
	}
	if !available.Available {
		return Simulation{}, ErrConflict
	}
	rate, err := s.repo.CurrentRate(ctx, owner, spaceID)
	if err != nil {
		return Simulation{}, err
	}
	count, err := units(rate.Unit, start, end, available.TimeZone)
	if err != nil || count < 1 || rate.Amount > math.MaxInt64/count {
		return Simulation{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Simulation{}, err
	}
	return s.repo.CreateSimulation(ctx, owner, spaceID, id, available.TimeZone, rate, start, end, count, count*rate.Amount)
}
func (s *Service) GetSimulation(ctx context.Context, owner, spaceID, id string) (Simulation, error) {
	if !uuidPattern.MatchString(owner) || !uuidPattern.MatchString(spaceID) || !uuidPattern.MatchString(id) {
		return Simulation{}, ErrNotFound
	}
	return s.repo.GetSimulation(ctx, owner, spaceID, id)
}

// BilledUnits exposes the same timezone-aware unit calculation for the local
// tenant quote snapshot, avoiding a second tariff-boundary implementation.
func BilledUnits(unit string, startUTC, endUTC time.Time, zone string) (int64, error) {
	return units(unit, startUTC, endUTC, zone)
}

func mapCalendarError(err error) error {
	if err == occupancy.ErrNotFound {
		return ErrNotFound
	}
	if err == occupancy.ErrInvalid || err == occupancy.ErrTimezoneRequired {
		return ErrInvalid
	}
	return err
}
