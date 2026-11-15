package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRevokeDoesNotReachRepositoryWhenContextAlreadyCanceled(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseApproved
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before, beforeAudit, beforeCalls := r.state, r.audit, r.calls

	err := s.Revoke(ctx, actor, x.ID, PurposeIdentity, ReasonOwnerRequest, x.Revision)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled revoke error=%v, want context.Canceled", err)
	}
	if r.calls != beforeCalls || !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
		t.Fatalf("pre-canceled revoke reached/mutated repository: calls=%d state=%+v audit=%+v", r.calls, r.state, r.audit)
	}
}

func TestRevokeStopsWhenContextCancelsBeforeTransitionCallback(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseApproved
	ctx, cancel := context.WithCancel(context.Background())
	s.repo = &cancelBeforeTransitionCallbackRepository{memoryRepository: r, cancel: cancel}
	before, beforeAudit, beforeCalls := r.state, r.audit, r.calls

	err := s.Revoke(ctx, actor, x.ID, PurposeIdentity, ReasonOwnerRequest, x.Revision)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled transition error=%v, want context.Canceled", err)
	}
	if ctx.Err() != context.Canceled || r.calls != beforeCalls+1 || !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
		t.Fatalf("canceled transition mutated state: canceled=%v calls=%d state=%+v audit=%+v", ctx.Err() != nil, r.calls, r.state, r.audit)
	}
}

func TestRevokeDoesNotCommitWhenContextCancelsDuringDomainChange(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseApproved
	ctx, cancel := context.WithCancel(context.Background())
	s.SetClock(func() time.Time {
		cancel()
		return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	})
	before, beforeAudit, beforeCalls := r.state, r.audit, r.calls

	err := s.Revoke(ctx, actor, x.ID, PurposeIdentity, ReasonOwnerRequest, x.Revision)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("revoke canceled during domain change error=%v, want context.Canceled", err)
	}
	if ctx.Err() != context.Canceled || r.calls != beforeCalls+1 || !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
		t.Fatalf("canceled domain change committed: canceled=%v calls=%d state=%+v audit=%+v", ctx.Err() != nil, r.calls, r.state, r.audit)
	}
}
