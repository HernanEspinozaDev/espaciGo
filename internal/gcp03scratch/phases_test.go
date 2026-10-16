package gcp03scratch

import (
	"context"
	"errors"
	"testing"
)

func TestExecutePhaseInvokesOnlyTheNamedPhase(t *testing.T) {
	for _, phase := range []Phase{PhaseBootstrap, PhaseDatabaseACL, PhaseMigrate, PhaseFinalize, PhaseGrantOwnerHistory, PhaseRetire, PhaseVerifyRetired, PhaseRevokeRetiredMembership} {
		t.Run(string(phase), func(t *testing.T) {
			calls := map[Phase]int{}
			actions := PhaseActions{
				Bootstrap:               func(context.Context) error { calls[PhaseBootstrap]++; return nil },
				DatabaseACL:             func(context.Context) error { calls[PhaseDatabaseACL]++; return nil },
				Migrate:                 func(context.Context) error { calls[PhaseMigrate]++; return nil },
				Finalize:                func(context.Context) error { calls[PhaseFinalize]++; return nil },
				Retire:                  func(context.Context) error { calls[PhaseRetire]++; return nil },
				GrantOwnerHistory:       func(context.Context) error { calls[PhaseGrantOwnerHistory]++; return nil },
				VerifyRetired:           func(context.Context) error { calls[PhaseVerifyRetired]++; return nil },
				RevokeRetiredMembership: func(context.Context) error { calls[PhaseRevokeRetiredMembership]++; return nil },
			}
			if err := ExecutePhase(context.Background(), phase, actions); err != nil {
				t.Fatal(err)
			}
			for _, candidate := range []Phase{PhaseBootstrap, PhaseDatabaseACL, PhaseMigrate, PhaseFinalize, PhaseGrantOwnerHistory, PhaseRetire, PhaseVerifyRetired, PhaseRevokeRetiredMembership} {
				want := 0
				if candidate == phase {
					want = 1
				}
				if calls[candidate] != want {
					t.Errorf("phase %s call count=%d, want %d", candidate, calls[candidate], want)
				}
			}
		})
	}
}

func TestExecutePhaseStopsOnItsOwnErrorAndRejectsUnknownPhase(t *testing.T) {
	wantErr := errors.New("bootstrap failed")
	finalized := false
	err := ExecutePhase(context.Background(), PhaseBootstrap, PhaseActions{
		Bootstrap: func(context.Context) error { return wantErr },
		Finalize:  func(context.Context) error { finalized = true; return nil },
	})
	if !errors.Is(err, wantErr) || finalized {
		t.Fatal("phase error was ignored or execution fell through")
	}
	if err := ExecutePhase(context.Background(), Phase("sql"), PhaseActions{}); err == nil {
		t.Fatal("unknown phase was accepted")
	}
}
