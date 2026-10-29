package booking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
)

type identityGateRepository struct {
	Repository
	detail       Detail
	decideCalled bool
}

func (r *identityGateRepository) Get(context.Context, string, string) (Detail, error) {
	return r.detail, nil
}

func (r *identityGateRepository) Decide(context.Context, string, string, string, string, func() time.Time) (Reservation, error) {
	r.decideCalled = true
	return Reservation{State: "aprobada_host"}, nil
}

func (r *identityGateRepository) Expire(context.Context, time.Time) error { return nil }

type identityGateVerificationRepository map[string]identityverification.Verification

func (r identityGateVerificationRepository) CurrentForAccount(_ context.Context, accountID string) (identityverification.Verification, error) {
	verification, ok := r[accountID]
	if !ok {
		return identityverification.Verification{}, identityverification.ErrNotFound
	}
	return verification, nil
}

const (
	identityGateHostID    = "10000000-0000-4000-8000-000000000001"
	identityGateRenterID  = "10000000-0000-4000-8000-000000000002"
	identityGateBookingID = "10000000-0000-4000-8000-000000000003"
)

func identityGateService(t *testing.T, now time.Time, repo *identityGateRepository, verifier BookingIdentityVerifier) *Service {
	t.Helper()
	service := &Service{
		repo:               repo,
		now:                func() time.Time { return now },
		globalExpirySweeps: false,
	}
	service.SetBookingIdentityVerifier(verifier)
	return service
}

func identityGateApproved(subject string, now time.Time) identityverification.Verification {
	verifiedAt := now.AddDate(0, -1, 0)
	return identityverification.Verification{
		SubjectID:    subject,
		Kind:         identityverification.KindIdentityDocument,
		State:        identityverification.StateApproved,
		ReviewMethod: identityverification.ReviewMethodManualDocuments,
		VerifiedAt:   verifiedAt,
		ExpiresAt:    verifiedAt.AddDate(0, 24, 0),
	}
}

func TestApproveFailsClosedWithoutIdentityVerifier(t *testing.T) {
	now := time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC)
	repo := &identityGateRepository{detail: Detail{Reservation: Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}}}
	service := identityGateService(t, now, repo, nil)

	if _, err := service.Decide(context.Background(), identityGateHostID, identityGateBookingID, "aprobar", ""); !errors.Is(err, ErrIdentityVerificationUnavailable) {
		t.Fatalf("approval without verifier: err=%v, want ErrIdentityVerificationUnavailable", err)
	}
	if repo.decideCalled {
		t.Fatal("repository approved booking without identity verification")
	}
}

func TestApproveRequiresCurrentIdentityVerificationForBothParties(t *testing.T) {
	now := time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC)
	repo := &identityGateRepository{detail: Detail{Reservation: Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}}}
	verificationRepo := identityGateVerificationRepository{
		identityGateHostID:   identityGateApproved(identityGateHostID, now),
		identityGateRenterID: identityGateApproved(identityGateRenterID, now),
	}
	verifier, err := identityverification.NewService(verificationRepo)
	if err != nil {
		t.Fatal(err)
	}
	service := identityGateService(t, now, repo, verifier)

	approved, err := service.Decide(context.Background(), identityGateHostID, identityGateBookingID, "aprobar", "")
	if err != nil {
		t.Fatalf("approval with both current identities: %v", err)
	}
	if approved.State != "aprobada_host" || !repo.decideCalled {
		t.Fatalf("approval not delegated after identity gate: state=%q decided=%v", approved.State, repo.decideCalled)
	}
}

func TestApproveRejectsWhenEitherPartyLacksCurrentIdentityVerification(t *testing.T) {
	now := time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		records identityGateVerificationRepository
	}{
		{
			name: "host missing",
			records: identityGateVerificationRepository{
				identityGateRenterID: identityGateApproved(identityGateRenterID, now),
			},
		},
		{
			name: "renter missing",
			records: identityGateVerificationRepository{
				identityGateHostID: identityGateApproved(identityGateHostID, now),
			},
		},
		{
			name: "host expired",
			records: identityGateVerificationRepository{
				identityGateHostID: func() identityverification.Verification {
					value := identityGateApproved(identityGateHostID, now)
					value.ExpiresAt = now
					return value
				}(),
				identityGateRenterID: identityGateApproved(identityGateRenterID, now),
			},
		},
		{
			name: "renter revoked",
			records: identityGateVerificationRepository{
				identityGateHostID: identityGateApproved(identityGateHostID, now),
				identityGateRenterID: func() identityverification.Verification {
					value := identityGateApproved(identityGateRenterID, now)
					revokedAt := now.Add(time.Minute)
					value.RevokedAt = &revokedAt
					return value
				}(),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &identityGateRepository{detail: Detail{Reservation: Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}}}
			verifier, err := identityverification.NewService(tc.records)
			if err != nil {
				t.Fatal(err)
			}
			service := identityGateService(t, now, repo, verifier)

			if _, err := service.Decide(context.Background(), identityGateHostID, identityGateBookingID, "aprobar", ""); !errors.Is(err, ErrIdentityVerificationRequired) {
				t.Fatalf("approval with non-current participant identity: err=%v, want ErrIdentityVerificationRequired", err)
			}
			if repo.decideCalled {
				t.Fatal("repository approved booking despite missing or non-current identity")
			}
		})
	}
}

func TestRejectDecisionDoesNotRequireIdentityVerification(t *testing.T) {
	repo := &identityGateRepository{detail: Detail{Reservation: Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}}}
	service := identityGateService(t, time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC), repo, nil)

	if _, err := service.Decide(context.Background(), identityGateHostID, identityGateBookingID, "rechazar", "No es compatible"); err != nil {
		t.Fatalf("rejection unexpectedly required identity verification: %v", err)
	}
	if !repo.decideCalled {
		t.Fatal("rejection was not delegated to the repository")
	}
}

type identityGateFailureVerifier struct{ err error }

func (v identityGateFailureVerifier) RequireForBooking(context.Context, string, string, time.Time) error {
	return v.err
}

func TestApproveReturnsUnavailableWhenIdentityRepositoryFails(t *testing.T) {
	repo := &identityGateRepository{detail: Detail{Reservation: Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}}}
	service := identityGateService(t, time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC), repo, identityGateFailureVerifier{err: errors.New("synthetic repository failure")})

	if _, err := service.Decide(context.Background(), identityGateHostID, identityGateBookingID, "aprobar", ""); !errors.Is(err, ErrIdentityVerificationUnavailable) {
		t.Fatalf("approval during identity repository failure: err=%v, want ErrIdentityVerificationUnavailable", err)
	}
	if repo.decideCalled {
		t.Fatal("repository approved booking after identity repository failure")
	}
}

type identityGateCountingVerifier struct{ calls int }

func (v *identityGateCountingVerifier) RequireForBooking(context.Context, string, string, time.Time) error {
	v.calls++
	return nil
}

func TestApproveRejectsMismatchedParticipantProjectionBeforeCallingVerifier(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Reservation)
	}{
		{name: "reservation id malformed", change: func(value *Reservation) { value.ID = "not-a-uuid" }},
		{name: "reservation id mismatch", change: func(value *Reservation) { value.ID = "10000000-0000-4000-8000-000000000004" }},
		{name: "host id malformed", change: func(value *Reservation) { value.HostID = "not-a-uuid" }},
		{name: "host id mismatch", change: func(value *Reservation) { value.HostID = "10000000-0000-4000-8000-000000000004" }},
		{name: "renter id malformed", change: func(value *Reservation) { value.RenterID = "not-a-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reservation := Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}
			tc.change(&reservation)
			repo := &identityGateRepository{detail: Detail{Reservation: reservation}}
			verifier := &identityGateCountingVerifier{}
			service := identityGateService(t, time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC), repo, verifier)

			if _, err := service.Decide(context.Background(), identityGateHostID, identityGateBookingID, "aprobar", ""); !errors.Is(err, ErrNotFound) {
				t.Fatalf("approval with mismatched participant projection: err=%v, want ErrNotFound", err)
			}
			if verifier.calls != 0 || repo.decideCalled {
				t.Fatalf("mismatch reached verifier/decision: verifier_calls=%d decide_called=%v", verifier.calls, repo.decideCalled)
			}
		})
	}
}

type identityGateContextVerifier struct{}

func (identityGateContextVerifier) RequireForBooking(ctx context.Context, _ string, _ string, _ time.Time) error {
	return ctx.Err()
}

func TestApprovePreservesContextCancellationAndDeadline(t *testing.T) {
	contexts := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{
			name: "canceled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
		},
		{
			name: "deadline exceeded",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(1, 0))
			},
		},
	}
	for _, tc := range contexts {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.ctx()
			defer cancel()
			repo := &identityGateRepository{detail: Detail{Reservation: Reservation{ID: identityGateBookingID, HostID: identityGateHostID, RenterID: identityGateRenterID, State: "pagada"}}}
			service := identityGateService(t, time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC), repo, identityGateContextVerifier{})

			_, err := service.Decide(ctx, identityGateHostID, identityGateBookingID, "aprobar", "")
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("approval did not preserve context termination: err=%v", err)
			}
			if repo.decideCalled {
				t.Fatal("repository approved booking after context termination")
			}
		})
	}
}
