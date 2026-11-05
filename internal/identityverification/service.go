package identityverification

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalid     = errors.New("documentary identity: invalid input")
	ErrNotFound    = errors.New("documentary identity: verification not found")
	ErrNotVerified = errors.New("documentary identity: current manual verification required")
)

const (
	KindIdentityDocument        = "identidad_documental"
	StateApproved               = "aprobada"
	ReviewMethodManualDocuments = "manual_document_review_v1"
)

// Verification is the minimal, non-documentary assertion consumed by booking.
// It deliberately contains no evidence bytes, document number, biometric data,
// original filename, or storage key.
type Verification struct {
	SubjectID    string
	Kind         string
	State        string
	ReviewMethod string
	VerifiedAt   time.Time
	// ExpiresAt is the effective deadline: the earlier of document expiry and
	// the 24-month manual-review renewal window.
	ExpiresAt time.Time
	// Any recorded revocation is fail-closed, regardless of its timestamp;
	// scheduled future-effective revocations are not part of this contract.
	RevokedAt *time.Time
}

// Repository returns an account-scoped verification assertion. Implementations
// must derive the account from authenticated ownership and never trust a caller's
// document or storage identifiers.
type Repository interface {
	CurrentForAccount(context.Context, string) (Verification, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, ErrInvalid
	}
	return &Service{repository: repository}, nil
}

// RequireForBooking allows confirmation only when both distinct parties have a
// current, approved identity-document review performed manually. Synthetic KYC
// or KYB fixture eligibility is intentionally not accepted as identity proof.
func (s *Service) RequireForBooking(ctx context.Context, hostID, renterID string, now time.Time) error {
	if s == nil || s.repository == nil || strings.TrimSpace(hostID) == "" || strings.TrimSpace(renterID) == "" || hostID == renterID || now.IsZero() {
		return ErrInvalid
	}
	for _, accountID := range []string{hostID, renterID} {
		verification, err := s.repository.CurrentForAccount(ctx, accountID)
		if errors.Is(err, ErrNotFound) {
			return ErrNotVerified
		}
		if err != nil {
			return err
		}
		if !verification.currentFor(accountID, now) {
			return ErrNotVerified
		}
	}
	return nil
}

func (v Verification) currentFor(accountID string, now time.Time) bool {
	if v.SubjectID != accountID || v.Kind != KindIdentityDocument || v.State != StateApproved ||
		v.ReviewMethod != ReviewMethodManualDocuments || v.VerifiedAt.IsZero() || v.VerifiedAt.After(now) ||
		v.ExpiresAt.IsZero() || !v.ExpiresAt.After(now) || v.ExpiresAt.After(v.VerifiedAt.AddDate(0, 24, 0)) || v.RevokedAt != nil {
		return false
	}
	return true
}
