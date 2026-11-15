package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestIdentityReviewRejectsSelfReviewWithoutMutatingCaseOrAudit(t *testing.T) {
	service, repo, _ := fixture(t, PurposeIdentity)
	repo.state.SubjectID = actor
	repo.auth.ActorID = actor
	repo.auth.SubjectID = actor
	repo.auth.ReviewerGrants = []ReviewerGrant{{
		ReviewerID: actor,
		Capability: CapabilityIdentity,
		ScopeID:    actor,
	}}
	beforeCase := repo.state
	beforeAudit := repo.audit

	err := service.ReviewIdentityWithReason(
		context.Background(),
		actor,
		repo.state.ID,
		DecisionReject,
		ReasonDocumentInvalid,
		DocumentFormatLegacy,
		time.Time{},
		nil,
		time.Time{},
		repo.state.Revision,
	)
	if !errors.Is(err, ErrSelfReview) {
		t.Fatalf("identity self-review error=%v, want ErrSelfReview", err)
	}
	if !reflect.DeepEqual(repo.state, beforeCase) || !reflect.DeepEqual(repo.audit, beforeAudit) {
		t.Fatal("rejected identity self-review mutated the case or audit")
	}
	if repo.calls != 1 || repo.transitionActor != actor {
		t.Fatalf("transition calls=%d actor=%q, want one authenticated reviewer transition", repo.calls, repo.transitionActor)
	}
}
