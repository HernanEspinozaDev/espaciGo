package documentverification

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type identityReviewRaceRepository struct {
	*memoryRepository
	mu             sync.Mutex
	authorizations map[string]Authorization
}

func (r *identityReviewRaceRepository) Transition(ctx context.Context, actor string, purpose Purpose, id string, revision uint64, audit TransitionAudit, change func(*Snapshot, Authorization) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	authorization, ok := r.authorizations[actor]
	if !ok {
		return ErrCapability
	}
	r.memoryRepository.auth = authorization
	return r.memoryRepository.Transition(ctx, actor, purpose, id, revision, audit, change)
}

func TestConcurrentIdentityReviewDecisionsWithSameRevisionCommitOnce(t *testing.T) {
	const reviewerA = "99999999-9999-4999-8999-999999999991"
	const reviewerB = "99999999-9999-4999-8999-999999999992"

	_, base, _ := fixture(t, PurposeIdentity)
	base.state.Revision = 4
	base.state.IdentityDeclarationRevisionID = declarationRevisionID
	base.state.DocumentFormatDeclared = DocumentFormatNew
	base.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	base.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	base.state.DocumentExpiresAt = time.Time{}
	base.state.Files = []File{
		{ID: "11111111-1111-4111-8111-111111111121", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "11111111-1111-4111-8111-111111111122", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: "11111111-1111-4111-8111-111111111123", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 4, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
	}

	grantFor := func(reviewer string) Authorization {
		return Authorization{
			ActorID:                       reviewer,
			SubjectID:                     subject,
			IdentityDeclarationRevisionID: declarationRevisionID,
			ReviewerGrants: []ReviewerGrant{{
				ReviewerID: reviewer,
				Capability: CapabilityIdentity,
				ScopeID:    subject,
			}},
			IdentityCurrent: true,
		}
	}
	repo := &identityReviewRaceRepository{
		memoryRepository: base,
		authorizations: map[string]Authorization{
			reviewerA: grantFor(reviewerA),
			reviewerB: grantFor(reviewerB),
		},
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	service.SetClock(func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) })

	type result struct {
		actor string
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, reviewer := range []string{reviewerA, reviewerB} {
		go func(actor string) {
			<-start
			results <- result{actor: actor, err: service.ReviewIdentityWithReason(
				context.Background(), actor, caseID, DecisionApprove, "", DocumentFormatNew,
				time.Date(2030, 5, 1, 0, 0, 0, 0, time.UTC), passedIdentityReviewChecklist(), time.Time{}, 4,
			)}
		}(reviewer)
	}
	close(start)

	var winner string
	conflicts := 0
	for range 2 {
		outcome := <-results
		switch {
		case outcome.err == nil:
			if winner != "" {
				t.Fatal("both reviewer decisions committed")
			}
			winner = outcome.actor
		case errors.Is(outcome.err, ErrConflict):
			conflicts++
		default:
			t.Fatalf("reviewer %s returned unexpected error: %v", outcome.actor, outcome.err)
		}
	}

	if winner == "" || conflicts != 1 {
		t.Fatalf("winner=%q conflicts=%d, want one commit and one revision conflict", winner, conflicts)
	}
	if repo.state.State != CaseApproved || repo.state.Revision != 5 || repo.state.ReviewerID != winner {
		t.Fatalf("committed snapshot=%+v, winner=%s", repo.state, winner)
	}
	if repo.calls != 2 || repo.audit.Action != "review_decision" || repo.audit.ResultCode != string(CaseApproved) {
		t.Fatalf("transitions=%d audit=%+v, want two attempts and one committed decision audit", repo.calls, repo.audit)
	}
}
