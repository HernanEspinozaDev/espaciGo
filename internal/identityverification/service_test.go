package identityverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const (
	syntheticHost   = "10000000-0000-4000-8000-000000000001"
	syntheticRenter = "10000000-0000-4000-8000-000000000002"
)

type memoryRepository map[string]Verification

func (r memoryRepository) CurrentForAccount(_ context.Context, accountID string) (Verification, error) {
	verification, ok := r[accountID]
	if !ok {
		return Verification{}, ErrNotFound
	}
	return verification, nil
}

type failingRepository struct {
	err   error
	calls int
}

func (r *failingRepository) CurrentForAccount(context.Context, string) (Verification, error) {
	r.calls++
	return Verification{}, r.err
}

func currentManualVerification(accountID string, now time.Time) Verification {
	return Verification{
		SubjectID:    accountID,
		Kind:         KindIdentityDocument,
		State:        StateApproved,
		ReviewMethod: ReviewMethodManualDocuments,
		VerifiedAt:   now.Add(-24 * time.Hour),
		ExpiresAt:    now.Add(24 * time.Hour),
	}
}

func TestRequireForBookingAcceptsCurrentManualDocumentReviewForBothParties(t *testing.T) {
	now := time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC)
	repo := memoryRepository{
		syntheticHost:   currentManualVerification(syntheticHost, now),
		syntheticRenter: currentManualVerification(syntheticRenter, now),
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now); err != nil {
		t.Fatalf("current manual identity review should satisfy both-party gate: %v", err)
	}
}

func TestRequireForBookingDoesNotTreatKYCFixtureAsDocumentaryIdentity(t *testing.T) {
	now := time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC)
	fixtureKYC := Verification{
		SubjectID:    syntheticRenter,
		Kind:         "kyc",
		State:        StateApproved,
		ReviewMethod: "local-fixture-v1",
		VerifiedAt:   now.Add(-time.Hour),
		ExpiresAt:    now.Add(24 * time.Hour),
	}
	repo := memoryRepository{
		syntheticHost:   currentManualVerification(syntheticHost, now),
		syntheticRenter: fixtureKYC,
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now); !errors.Is(err, ErrNotVerified) {
		t.Fatalf("fixture KYC alone must not satisfy documentary identity gate: err=%v", err)
	}
}

func TestRequireForBookingFailsClosedForInvalidPartyVerification(t *testing.T) {
	now := time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		mutate func(map[string]Verification)
	}{
		{
			name: "missing host identity",
			mutate: func(records map[string]Verification) {
				delete(records, syntheticHost)
			},
		},
		{
			name: "expired renter identity",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticRenter]
				verification.ExpiresAt = now
				records[syntheticRenter] = verification
			},
		},
		{
			name: "revoked host identity",
			mutate: func(records map[string]Verification) {
				revokedAt := now.Add(-time.Minute)
				verification := records[syntheticHost]
				verification.RevokedAt = &revokedAt
				records[syntheticHost] = verification
			},
		},
		{
			name: "future revocation fails closed",
			mutate: func(records map[string]Verification) {
				revokedAt := now.Add(time.Hour)
				verification := records[syntheticHost]
				verification.RevokedAt = &revokedAt
				records[syntheticHost] = verification
			},
		},
		{
			name: "future verification",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticRenter]
				verification.VerifiedAt = now.Add(time.Second)
				records[syntheticRenter] = verification
			},
		},
		{
			name: "validity exceeds 24 months",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticHost]
				verification.ExpiresAt = verification.VerifiedAt.AddDate(0, 24, 0).Add(time.Second)
				records[syntheticHost] = verification
			},
		},
		{
			name: "wrong subject",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticHost]
				verification.SubjectID = syntheticRenter
				records[syntheticHost] = verification
			},
		},
		{
			name: "wrong verification kind",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticRenter]
				verification.Kind = "kyc"
				records[syntheticRenter] = verification
			},
		},
		{
			name: "unapproved decision",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticHost]
				verification.State = "rechazada"
				records[syntheticHost] = verification
			},
		},
		{
			name: "non-manual review method",
			mutate: func(records map[string]Verification) {
				verification := records[syntheticRenter]
				verification.ReviewMethod = "local-fixture-v1"
				records[syntheticRenter] = verification
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			records := map[string]Verification{
				syntheticHost:   currentManualVerification(syntheticHost, now),
				syntheticRenter: currentManualVerification(syntheticRenter, now),
			}
			test.mutate(records)
			service, err := NewService(memoryRepository(records))
			if err != nil {
				t.Fatal(err)
			}
			if err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now); !errors.Is(err, ErrNotVerified) {
				t.Fatalf("invalid %s identity must block booking: err=%v", test.name, err)
			}
		})
	}
}

func TestRequireForBookingRejectsSamePartyBeforeRepositoryLookup(t *testing.T) {
	now := time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC)
	repository := &failingRepository{err: errors.New("should not be called")}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RequireForBooking(context.Background(), syntheticHost, syntheticHost, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("same account cannot be both booking parties: err=%v", err)
	}
	if repository.calls != 0 {
		t.Fatalf("repository calls=%d; same-party input should be rejected before lookup", repository.calls)
	}
}

func TestRequireForBookingPropagatesRepositoryFailures(t *testing.T) {
	now := time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC)
	lookupErr := errors.New("synthetic repository unavailable")
	repository := &failingRepository{err: lookupErr}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now); !errors.Is(err, lookupErr) {
		t.Fatalf("repository failure must not satisfy booking gate: err=%v", err)
	}
	if repository.calls != 1 {
		t.Fatalf("repository calls=%d; expected to stop after first failed lookup", repository.calls)
	}
}

func TestVerificationAssertionUsesMinimizedFields(t *testing.T) {
	want := []string{"SubjectID", "Kind", "State", "ReviewMethod", "VerifiedAt", "ExpiresAt", "RevokedAt"}
	verificationType := reflect.TypeOf(Verification{})
	if verificationType.NumField() != len(want) {
		t.Fatalf("verification assertion has %d fields; want only minimized fields %v", verificationType.NumField(), want)
	}
	for index, fieldName := range want {
		if got := verificationType.Field(index).Name; got != fieldName {
			t.Fatalf("verification assertion field[%d]=%q; want %q", index, got, fieldName)
		}
	}
}
