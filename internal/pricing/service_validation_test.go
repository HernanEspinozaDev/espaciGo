package pricing

import (
	"context"
	"testing"
)

func TestSimulateRejectsInvalidOwnerSpaceAndWindowsBeforeCalendar(t *testing.T) {
	const (
		ownerID = "33333333-3333-4333-8333-333333333333"
		spaceID = "11111111-1111-4111-8111-111111111111"
		startAt = "2030-01-02T13:00:00Z"
		endAt   = "2030-01-02T14:00:00Z"
	)
	for _, tc := range []struct {
		name    string
		owner   string
		space   string
		startAt string
		endAt   string
		wantErr error
	}{
		{name: "invalid owner", owner: "invalid", space: spaceID, startAt: startAt, endAt: endAt, wantErr: ErrNotFound},
		{name: "invalid space", owner: ownerID, space: "invalid", startAt: startAt, endAt: endAt, wantErr: ErrNotFound},
		{name: "non-UTC offset", owner: ownerID, space: spaceID, startAt: "2030-01-02T13:00:00+00:00", endAt: endAt, wantErr: ErrInvalid},
		{name: "malformed timestamp", owner: ownerID, space: spaceID, startAt: "malformedZ", endAt: endAt, wantErr: ErrInvalid},
		{name: "empty interval", owner: ownerID, space: spaceID, startAt: startAt, endAt: startAt, wantErr: ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &unavailableSimulationRepository{}
			calendar := &unavailableSimulationCalendar{}
			ids := &unavailableSimulationIDs{}
			service, err := NewService(repo, calendar, ids)
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Simulate(context.Background(), tc.owner, tc.space, SimulationInput{StartAt: tc.startAt, EndAt: tc.endAt})
			if calendar.calls != 0 || repo.currentRateCalls != 0 || repo.createSimulationCalls != 0 || ids.calls != 0 {
				t.Fatalf("invalid request reached dependencies: calendar=%d rateReads=%d IDs=%d simulationWrites=%d", calendar.calls, repo.currentRateCalls, ids.calls, repo.createSimulationCalls)
			}
			if err != tc.wantErr {
				t.Fatalf("Simulate error=%v, want exact error %v", err, tc.wantErr)
			}
		})
	}
}
