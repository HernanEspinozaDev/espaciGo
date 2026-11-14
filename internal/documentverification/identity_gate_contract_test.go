package documentverification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
)

type rentalIdentityAssertions map[string]identityverification.Verification

func (r rentalIdentityAssertions) CurrentForAccount(ctx context.Context, accountID string) (identityverification.Verification, error) {
	if err := ctx.Err(); err != nil {
		return identityverification.Verification{}, err
	}
	assertion, ok := r[accountID]
	if !ok {
		return identityverification.Verification{}, identityverification.ErrNotFound
	}
	return assertion, nil
}

func TestApprovedIdentityReviewFeedsRentalGateUntilEffectiveExpiry(t *testing.T) {
	reviewedAt := time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC)
	review, repository, snapshot := fixture(t, PurposeIdentity)
	review.SetClock(func() time.Time { return reviewedAt })
	repository.state.DocumentExpiresAt = reviewedAt.AddDate(4, 0, 0)
	if err := review.Review(context.Background(), actor, snapshot.ID, CapabilityIdentity, DecisionApprove, time.Time{}, 0); err != nil {
		t.Fatal(err)
	}
	approved := repository.state
	if !approved.VerifiedAt.Equal(reviewedAt) || !approved.VerifiedUntil.Equal(reviewedAt.AddDate(0, 24, 0)) {
		t.Fatalf("approved identity dates=%s..%s; want review time through 24-month renewal limit", approved.VerifiedAt, approved.VerifiedUntil)
	}

	renterVerifiedAt := reviewedAt.Add(time.Nanosecond)
	assertions := rentalIdentityAssertions{
		subject: {
			SubjectID:    approved.SubjectID,
			Kind:         identityverification.KindIdentityDocument,
			State:        string(approved.State),
			ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt:   approved.VerifiedAt,
			ExpiresAt:    approved.VerifiedUntil,
			RevokedAt:    approved.RevokedAt,
		},
		space: {
			SubjectID:    space,
			Kind:         identityverification.KindIdentityDocument,
			State:        identityverification.StateApproved,
			ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt:   renterVerifiedAt,
			ExpiresAt:    renterVerifiedAt.AddDate(0, 24, 0),
		},
	}
	gate, err := identityverification.NewService(assertions)
	if err != nil {
		t.Fatal(err)
	}
	if err = gate.RequireForBooking(context.Background(), subject, space, approved.VerifiedUntil.Add(-time.Nanosecond)); err != nil {
		t.Fatalf("rental gate rejected both current identities immediately before the effective deadline: %v", err)
	}
	if err = gate.RequireForBooking(context.Background(), subject, space, approved.VerifiedUntil); !errors.Is(err, identityverification.ErrNotVerified) {
		t.Fatalf("rental gate error at effective deadline=%v; want ErrNotVerified", err)
	}
}

func TestDocumentExpiryOneNanosecondAfterReviewIsEffectiveRentalGateDeadline(t *testing.T) {
	reviewedAt := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	documentExpiry := reviewedAt.Add(time.Nanosecond)
	review, repository, snapshot := fixture(t, PurposeIdentity)
	review.SetClock(func() time.Time { return reviewedAt })
	repository.state.DocumentExpiresAt = documentExpiry
	if err := review.Review(context.Background(), actor, snapshot.ID, CapabilityIdentity, DecisionApprove, time.Time{}, 0); err != nil {
		t.Fatal(err)
	}
	approved := repository.state
	if !approved.VerifiedUntil.Equal(documentExpiry) {
		t.Fatalf("verifiedUntil=%s; want document expiry %s", approved.VerifiedUntil, documentExpiry)
	}

	assertions := rentalIdentityAssertions{
		subject: {
			SubjectID:    approved.SubjectID,
			Kind:         identityverification.KindIdentityDocument,
			State:        string(approved.State),
			ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt:   approved.VerifiedAt,
			ExpiresAt:    approved.VerifiedUntil,
			RevokedAt:    approved.RevokedAt,
		},
		space: {
			SubjectID:    space,
			Kind:         identityverification.KindIdentityDocument,
			State:        identityverification.StateApproved,
			ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt:   reviewedAt,
			ExpiresAt:    reviewedAt.AddDate(0, 24, 0),
		},
	}
	gate, err := identityverification.NewService(assertions)
	if err != nil {
		t.Fatal(err)
	}
	if err = gate.RequireForBooking(context.Background(), subject, space, reviewedAt); err != nil {
		t.Fatalf("rental gate rejected identity before document expiry: %v", err)
	}
	if err = gate.RequireForBooking(context.Background(), subject, space, documentExpiry); !errors.Is(err, identityverification.ErrNotVerified) {
		t.Fatalf("rental gate error at document expiry=%v; want ErrNotVerified", err)
	}
}
