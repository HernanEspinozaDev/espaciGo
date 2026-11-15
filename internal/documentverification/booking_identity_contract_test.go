package documentverification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
)

const syntheticBookingRenterID = "99999999-9999-4999-8999-999999999999"

type bookingIdentityRepository map[string]identityverification.Verification

func (r bookingIdentityRepository) CurrentForAccount(_ context.Context, accountID string) (identityverification.Verification, error) {
	verification, ok := r[accountID]
	if !ok {
		return identityverification.Verification{}, identityverification.ErrNotFound
	}
	return verification, nil
}

func TestTypedIdentityApprovalSuppliesExclusiveBookingExpiry(t *testing.T) {
	decisionAt := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	boundary := decisionAt.AddDate(0, 6, 0)

	review, repository, caseSnapshot := fixture(t, PurposeIdentity)
	review.SetClock(func() time.Time { return decisionAt })
	repository.state.IdentityDeclarationRevisionID = declarationRevisionID
	repository.state.DocumentFormatDeclared = DocumentFormatNew
	repository.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	repository.state.CaptureConsentedAt = decisionAt.Add(-time.Hour)
	repository.state.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
	}

	if err := review.ReviewIdentityWithReason(
		context.Background(), actor, caseSnapshot.ID, DecisionApprove, "",
		DocumentFormatNew, boundary, passedIdentityReviewChecklist(), time.Time{}, caseSnapshot.Revision,
	); err != nil {
		t.Fatalf("approve synthetic identity case: %v", err)
	}
	if repository.state.State != CaseApproved ||
		!repository.state.DocumentExpiresAt.Equal(boundary) ||
		!repository.state.VerifiedUntil.Equal(boundary) {
		t.Fatalf("approved snapshot does not carry the exclusive document deadline: state=%q document_expires_at=%s verified_until=%s",
			repository.state.State, repository.state.DocumentExpiresAt, repository.state.VerifiedUntil)
	}

	approved := repository.state
	bookingRepository := bookingIdentityRepository{
		approved.SubjectID: {
			SubjectID:    approved.SubjectID,
			Kind:         identityverification.KindIdentityDocument,
			State:        string(approved.State),
			ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt:   approved.VerifiedAt,
			ExpiresAt:    approved.VerifiedUntil,
			RevokedAt:    approved.RevokedAt,
		},
		syntheticBookingRenterID: {
			SubjectID:    syntheticBookingRenterID,
			Kind:         identityverification.KindIdentityDocument,
			State:        identityverification.StateApproved,
			ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt:   decisionAt,
			ExpiresAt:    decisionAt.AddDate(0, 24, 0),
		},
	}
	booking, err := identityverification.NewService(bookingRepository)
	if err != nil {
		t.Fatalf("create synthetic booking verifier: %v", err)
	}

	if err := booking.RequireForBooking(context.Background(), approved.SubjectID, syntheticBookingRenterID, boundary.Add(-time.Nanosecond)); err != nil {
		t.Fatalf("identity approval should satisfy booking gate immediately before expiry: %v", err)
	}
	if err := booking.RequireForBooking(context.Background(), approved.SubjectID, syntheticBookingRenterID, boundary); !errors.Is(err, identityverification.ErrNotVerified) {
		t.Fatalf("booking gate at exclusive document-expiry boundary error=%v, want ErrNotVerified", err)
	}
}
