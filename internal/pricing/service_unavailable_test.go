package pricing

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
)

type unavailableSimulationRepository struct {
	currentRateCalls      int
	createSimulationCalls int
}

func (r *unavailableSimulationRepository) CurrentRate(context.Context, string, string) (Rate, error) {
	r.currentRateCalls++
	return Rate{}, nil
}

func (*unavailableSimulationRepository) RateHistory(context.Context, string, string) ([]Rate, error) {
	return nil, nil
}

func (*unavailableSimulationRepository) UpdateRate(context.Context, string, string, RateInput) (Rate, error) {
	return Rate{}, nil
}

func (r *unavailableSimulationRepository) CreateSimulation(context.Context, string, string, string, string, Rate, time.Time, time.Time, int64, int64) (Simulation, error) {
	r.createSimulationCalls++
	return Simulation{}, nil
}

func (*unavailableSimulationRepository) GetSimulation(context.Context, string, string, string) (Simulation, error) {
	return Simulation{}, nil
}

type successfulSimulationRepository struct {
	unavailableSimulationRepository
	rate            Rate
	rateErr         error
	createdOwner    string
	createdSpaceID  string
	createdID       string
	createdTimeZone string
	createdRate     Rate
	createdStart    time.Time
	createdEnd      time.Time
	createdUnits    int64
	createdSubtotal int64
}

func (r *successfulSimulationRepository) CurrentRate(context.Context, string, string) (Rate, error) {
	r.currentRateCalls++
	return r.rate, r.rateErr
}

func (r *successfulSimulationRepository) CreateSimulation(_ context.Context, owner, spaceID, id, zone string, rate Rate, start, end time.Time, units, subtotal int64) (Simulation, error) {
	r.createSimulationCalls++
	r.createdOwner, r.createdSpaceID, r.createdID, r.createdTimeZone = owner, spaceID, id, zone
	r.createdRate, r.createdStart, r.createdEnd = rate, start, end
	r.createdUnits, r.createdSubtotal = units, subtotal
	return Simulation{ID: id, SpaceID: spaceID, RateVersion: rate.Version, RateUnit: rate.Unit, BasePrice: rate.Amount, BilledUnits: units, Currency: rate.Currency, Subtotal: subtotal, StartAt: start, EndAt: end, TimeZone: zone}, nil
}

type unavailableSimulationCalendar struct {
	result  occupancy.Availability
	err     error
	calls   int
	owner   string
	spaceID string
	startAt string
	endAt   string
}

func (c *unavailableSimulationCalendar) Availability(_ context.Context, owner, spaceID, startAt, endAt string) (occupancy.Availability, error) {
	c.calls++
	c.owner, c.spaceID, c.startAt, c.endAt = owner, spaceID, startAt, endAt
	return c.result, c.err
}

type unavailableSimulationIDs struct {
	calls int
	err   error
}

func (g *unavailableSimulationIDs) ID() (string, error) {
	g.calls++
	return "55555555-5555-4555-8555-555555555555", g.err
}

func (*unavailableSimulationIDs) Token() (identity.Secret, error) {
	return "synthetic-token", nil
}

func TestSimulateStopsBeforeRateReadAndPersistenceWhenIntervalUnavailable(t *testing.T) {
	const (
		ownerID = "33333333-3333-4333-8333-333333333333"
		spaceID = "11111111-1111-4111-8111-111111111111"
		startAt = "2030-01-02T13:00:00.123456789Z"
		endAt   = "2030-01-02T14:00:00.987654321Z"
	)
	start, err := time.Parse(time.RFC3339Nano, startAt)
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse(time.RFC3339Nano, endAt)
	if err != nil {
		t.Fatal(err)
	}

	repo := &unavailableSimulationRepository{}
	calendar := &unavailableSimulationCalendar{result: occupancy.Availability{
		SpaceID: spaceID, TimeZone: "America/Santiago", StartAt: start, EndAt: end, Available: false,
	}}
	ids := &unavailableSimulationIDs{}
	service, err := NewService(repo, calendar, ids)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Simulate(context.Background(), ownerID, spaceID, SimulationInput{StartAt: startAt, EndAt: endAt})
	if err != ErrConflict {
		t.Fatalf("Simulate error=%v, want %v", err, ErrConflict)
	}
	if calendar.calls != 1 || calendar.owner != ownerID || calendar.spaceID != spaceID || calendar.startAt != startAt || calendar.endAt != endAt {
		t.Fatalf("calendar calls=%d owner=%q space=%q start=%q end=%q", calendar.calls, calendar.owner, calendar.spaceID, calendar.startAt, calendar.endAt)
	}
	if repo.currentRateCalls != 0 || repo.createSimulationCalls != 0 || ids.calls != 0 {
		t.Fatalf("unavailable interval triggered downstream work: rateReads=%d simulationWrites=%d generatedIDs=%d", repo.currentRateCalls, repo.createSimulationCalls, ids.calls)
	}
}

func TestSimulateMapsCalendarErrorsBeforeRateReadAndPersistence(t *testing.T) {
	const (
		ownerID = "33333333-3333-4333-8333-333333333333"
		spaceID = "11111111-1111-4111-8111-111111111111"
		startAt = "2030-01-02T13:00:00Z"
		endAt   = "2030-01-02T14:00:00Z"
	)
	backendErr := errors.New("synthetic calendar failure")
	for _, tc := range []struct {
		name      string
		calendar  error
		wantError error
	}{
		{name: "missing resource is hidden", calendar: occupancy.ErrNotFound, wantError: ErrNotFound},
		{name: "invalid calendar is invalid request", calendar: occupancy.ErrInvalid, wantError: ErrInvalid},
		{name: "missing time zone is invalid request", calendar: occupancy.ErrTimezoneRequired, wantError: ErrInvalid},
		{name: "operational error is preserved", calendar: backendErr, wantError: backendErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &unavailableSimulationRepository{}
			calendar := &unavailableSimulationCalendar{err: tc.calendar}
			ids := &unavailableSimulationIDs{}
			service, err := NewService(repo, calendar, ids)
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Simulate(context.Background(), ownerID, spaceID, SimulationInput{StartAt: startAt, EndAt: endAt})
			if err != tc.wantError {
				t.Fatalf("Simulate error=%v, want exact error %v", err, tc.wantError)
			}
			if calendar.calls != 1 || repo.currentRateCalls != 0 || repo.createSimulationCalls != 0 || ids.calls != 0 {
				t.Fatalf("calendar error triggered unexpected work: calendarCalls=%d rateReads=%d simulationWrites=%d generatedIDs=%d", calendar.calls, repo.currentRateCalls, repo.createSimulationCalls, ids.calls)
			}
		})
	}
}

func TestSimulateUsesAvailabilityTimeZoneForDailyUnits(t *testing.T) {
	const (
		ownerID = "33333333-3333-4333-8333-333333333333"
		spaceID = "11111111-1111-4111-8111-111111111111"
		startAt = "2025-01-02T02:30:00Z"
		endAt   = "2025-01-02T04:30:00Z"
		zone    = "America/Santiago"
		quoteID = "55555555-5555-4555-8555-555555555555"
	)
	start, err := time.Parse(time.RFC3339Nano, startAt)
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse(time.RFC3339Nano, endAt)
	if err != nil {
		t.Fatal(err)
	}
	rate := Rate{SpaceID: spaceID, Version: 7, Unit: "dia", Amount: 12000, Currency: "CLP"}
	repo := &successfulSimulationRepository{rate: rate}
	calendar := &unavailableSimulationCalendar{result: occupancy.Availability{
		SpaceID: spaceID, TimeZone: zone, StartAt: start, EndAt: end, Available: true,
	}}
	ids := &unavailableSimulationIDs{}
	service, err := NewService(repo, calendar, ids)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Simulate(context.Background(), ownerID, spaceID, SimulationInput{StartAt: startAt, EndAt: endAt})
	if err != nil {
		t.Fatal(err)
	}
	if calendar.calls != 1 || calendar.owner != ownerID || calendar.spaceID != spaceID || calendar.startAt != startAt || calendar.endAt != endAt {
		t.Fatalf("calendar calls=%d owner=%q space=%q start=%q end=%q", calendar.calls, calendar.owner, calendar.spaceID, calendar.startAt, calendar.endAt)
	}
	if repo.currentRateCalls != 1 || repo.createSimulationCalls != 1 || ids.calls != 1 {
		t.Fatalf("rateReads=%d simulationWrites=%d generatedIDs=%d", repo.currentRateCalls, repo.createSimulationCalls, ids.calls)
	}
	if repo.createdOwner != ownerID || repo.createdSpaceID != spaceID || repo.createdID != quoteID || repo.createdTimeZone != zone || repo.createdRate != rate || !repo.createdStart.Equal(start) || !repo.createdEnd.Equal(end) || repo.createdUnits != 2 || repo.createdSubtotal != 24000 {
		t.Fatalf("captured simulation owner=%q space=%q id=%q zone=%q rate=%+v start=%s end=%s units=%d subtotal=%d", repo.createdOwner, repo.createdSpaceID, repo.createdID, repo.createdTimeZone, repo.createdRate, repo.createdStart, repo.createdEnd, repo.createdUnits, repo.createdSubtotal)
	}
	if got.ID != quoteID || got.BilledUnits != 2 || got.Subtotal != 24000 || got.TimeZone != zone {
		t.Fatalf("simulation result=%+v", got)
	}
}

func TestSimulateStopsBeforeIDOrWriteOnPricingStageFailures(t *testing.T) {
	const (
		ownerID = "33333333-3333-4333-8333-333333333333"
		spaceID = "11111111-1111-4111-8111-111111111111"
	)
	rateErr := errors.New("synthetic current rate failure")
	idErr := errors.New("synthetic ID generation failure")
	for _, tc := range []struct {
		name      string
		startAt   string
		endAt     string
		zone      string
		rate      Rate
		rateErr   error
		idErr     error
		wantError error
		wantID    int
	}{
		{name: "rate read error", startAt: "2030-01-02T13:00:00Z", endAt: "2030-01-02T14:00:00Z", zone: "UTC", rate: Rate{Unit: "hora", Amount: 12000}, rateErr: rateErr, wantError: rateErr},
		{name: "invalid availability time zone", startAt: "2030-01-02T13:00:00Z", endAt: "2030-01-02T14:00:00Z", zone: "Not/AZone", rate: Rate{Unit: "hora", Amount: 12000}, wantError: ErrInvalid},
		{name: "subtotal overflow", startAt: "2030-01-02T13:00:00Z", endAt: "2030-01-02T16:00:00Z", zone: "UTC", rate: Rate{Unit: "hora", Amount: math.MaxInt64}, wantError: ErrInvalid},
		{name: "ID generation error", startAt: "2030-01-02T13:00:00Z", endAt: "2030-01-02T14:00:00Z", zone: "UTC", rate: Rate{Unit: "hora", Amount: 12000}, idErr: idErr, wantError: idErr, wantID: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, err := time.Parse(time.RFC3339Nano, tc.startAt)
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339Nano, tc.endAt)
			if err != nil {
				t.Fatal(err)
			}
			repo := &successfulSimulationRepository{rate: tc.rate, rateErr: tc.rateErr}
			calendar := &unavailableSimulationCalendar{result: occupancy.Availability{
				SpaceID: spaceID, TimeZone: tc.zone, StartAt: start, EndAt: end, Available: true,
			}}
			ids := &unavailableSimulationIDs{err: tc.idErr}
			service, err := NewService(repo, calendar, ids)
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Simulate(context.Background(), ownerID, spaceID, SimulationInput{StartAt: tc.startAt, EndAt: tc.endAt})
			if err != tc.wantError {
				t.Fatalf("Simulate error=%v, want exact error %v", err, tc.wantError)
			}
			if calendar.calls != 1 || repo.currentRateCalls != 1 || ids.calls != tc.wantID || repo.createSimulationCalls != 0 {
				t.Fatalf("pricing stage failure calls: calendar=%d rateReads=%d IDs=%d simulationWrites=%d", calendar.calls, repo.currentRateCalls, ids.calls, repo.createSimulationCalls)
			}
		})
	}
}
