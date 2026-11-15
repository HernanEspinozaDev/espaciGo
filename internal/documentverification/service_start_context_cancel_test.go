package documentverification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type cancelBeforeCreateCaseCallbackRepository struct {
	*memoryRepository
	cancel context.CancelFunc
}

func (r *cancelBeforeCreateCaseCallbackRepository) CreateCase(ctx context.Context, actor string, p Purpose, sid string, build func(Authorization) (Snapshot, error)) (Snapshot, error) {
	r.cancel()
	return r.memoryRepository.CreateCase(ctx, actor, p, sid, build)
}

func TestStartDoesNotReachRepositoryWhenContextAlreadyCanceled(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before, beforeCalls := r.state, r.calls

	_, err := s.Start(ctx, actor, identityCreateInput(DocumentFormatNew))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled start error=%v, want context.Canceled", err)
	}
	if r.calls != beforeCalls || r.state.ID != before.ID || r.state.Revision != before.Revision {
		t.Fatalf("pre-canceled start reached/mutated repository: calls=%d state=%+v", r.calls, r.state)
	}
}

func TestStartStopsWhenContextCancelsBeforeCreateCaseCallback(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	ctx, cancel := context.WithCancel(context.Background())
	s.repo = &cancelBeforeCreateCaseCallbackRepository{memoryRepository: r, cancel: cancel}
	before, beforeCalls := r.state, r.calls

	_, err := s.Start(ctx, actor, identityCreateInput(DocumentFormatNew))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled start error=%v, want context.Canceled", err)
	}
	if ctx.Err() != context.Canceled || r.calls != beforeCalls+1 || r.state.ID != before.ID || r.state.Revision != before.Revision {
		t.Fatalf("canceled create callback mutated state: canceled=%v calls=%d state=%+v", ctx.Err() != nil, r.calls, r.state)
	}
}

func TestStartDoesNotPersistWhenContextCancelsWhileBuildingSnapshot(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	ctx, cancel := context.WithCancel(context.Background())
	s.SetClock(func() time.Time {
		cancel()
		return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	})
	before, beforeCalls := r.state, r.calls

	_, err := s.Start(ctx, actor, identityCreateInput(DocumentFormatNew))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("start canceled while building snapshot error=%v, want context.Canceled", err)
	}
	if ctx.Err() != context.Canceled || r.calls != beforeCalls+1 || r.state.ID != before.ID || r.state.Revision != before.Revision {
		t.Fatalf("canceled snapshot was persisted: canceled=%v calls=%d state=%+v", ctx.Err() != nil, r.calls, r.state)
	}
}
