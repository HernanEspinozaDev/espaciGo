package verification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type lateReadyWithoutQueueRepo struct{ *memProviderAttemptRepo }

func (*lateReadyWithoutQueueRepo) MarkAttemptSessionReady(context.Context, string, string, time.Time) (ProviderAttempt, error) {
	return ProviderAttempt{}, ErrConflict
}
func (*lateReadyWithoutQueueRepo) GetAttemptForAccount(context.Context, string, string, time.Time, time.Duration) (ProviderAttempt, error) {
	return ProviderAttempt{}, ErrNotFound
}

func TestDiditLateReadyWithoutQueueReturnsExplicitFailure(t *testing.T) {
	repo := &lateReadyWithoutQueueRepo{newMemProviderAttemptRepo()}
	svc, declarations, _, _ := rotationFixture(t, NewMemoryRutClaimRegistry(), repo, nil)
	saveRotationDeclaration(t, declarations, rotationAccountA)
	started, err := startRotation(t, svc, rotationAccountA, "late-no-queue")
	if !errors.Is(err, ErrConflict) || !errors.Is(err, ErrDiditUnavailable) || started.RedirectURL != "" {
		t.Fatalf("missing queue must remain explicit without claiming start: %v", err)
	}
	if pending := svc.PendingRemoteDeletions(); len(pending) != 1 || pending[0] != rotationSession {
		t.Fatal("legacy memory-only recovery must retain the known session without claiming durable enqueue")
	}
}
