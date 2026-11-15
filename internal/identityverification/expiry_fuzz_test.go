package identityverification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func FuzzRequireForBookingHonorsExpiryNanosecondBoundaryAcrossCalendarDates(f *testing.F) {
	f.Add(uint16(0), int8(-1))
	f.Add(uint16(59), int8(0))
	f.Add(uint16(365), int8(1))
	f.Fuzz(func(t *testing.T, dateSeed uint16, boundarySeed int8) {
		year := 2000 + int(dateSeed%40)
		dayOfYear := int(dateSeed % 366)
		verifiedAt := time.Date(year, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, dayOfYear)
		expiresAt := verifiedAt.AddDate(0, 24, 0)

		var boundaryOffset int64
		switch boundarySeed % 3 {
		case -1, 2:
			boundaryOffset = -1
		case 0:
			boundaryOffset = 0
		default:
			boundaryOffset = 1
		}
		now := expiresAt.Add(time.Duration(boundaryOffset) * time.Nanosecond)
		for _, targetID := range []string{syntheticHost, syntheticRenter} {
			t.Run(targetID, func(t *testing.T) {
				verification := func(subjectID string, issuedAt, deadline time.Time) Verification {
					return Verification{
						SubjectID:    subjectID,
						Kind:         KindIdentityDocument,
						State:        StateApproved,
						ReviewMethod: ReviewMethodManualDocuments,
						VerifiedAt:   issuedAt,
						ExpiresAt:    deadline,
					}
				}
				records := memoryRepository{
					syntheticHost:   verification(syntheticHost, verifiedAt, expiresAt),
					syntheticRenter: verification(syntheticRenter, verifiedAt, expiresAt),
				}
				otherID := syntheticHost
				if targetID == syntheticHost {
					otherID = syntheticRenter
				}
				otherIssuedAt := verifiedAt.Add(2 * time.Nanosecond)
				records[otherID] = verification(otherID, otherIssuedAt, otherIssuedAt.AddDate(0, 24, 0))
				service, err := NewService(records)
				if err != nil {
					t.Fatal(err)
				}

				err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now)
				if boundaryOffset < 0 {
					if err != nil {
						t.Fatalf("identity expired before deadline: target=%s verified=%s expires=%s now=%s err=%v", targetID, verifiedAt, expiresAt, now, err)
					}
					return
				}
				if !errors.Is(err, ErrNotVerified) {
					t.Fatalf("identity at/after deadline accepted: target=%s verified=%s expires=%s now=%s err=%v", targetID, verifiedAt, expiresAt, now, err)
				}
			})
		}
	})
}

func FuzzRequireForBookingRejectsFutureVerificationAtNanosecondBoundaryAcrossCalendarDates(f *testing.F) {
	f.Add(uint16(0), int8(-1))
	f.Add(uint16(59), int8(0))
	f.Add(uint16(365), int8(1))
	f.Fuzz(func(t *testing.T, dateSeed uint16, boundarySeed int8) {
		year := 2000 + int(dateSeed%40)
		dayOfYear := int(dateSeed % 366)
		now := time.Date(year, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, dayOfYear)

		var boundaryOffset int64
		switch boundarySeed % 3 {
		case -1, 2:
			boundaryOffset = -1
		case 0:
			boundaryOffset = 0
		default:
			boundaryOffset = 1
		}
		targetVerifiedAt := now.Add(time.Duration(boundaryOffset) * time.Nanosecond)
		otherVerifiedAt := now.Add(-2 * time.Nanosecond)
		verification := func(subjectID string, issuedAt time.Time) Verification {
			return Verification{
				SubjectID:    subjectID,
				Kind:         KindIdentityDocument,
				State:        StateApproved,
				ReviewMethod: ReviewMethodManualDocuments,
				VerifiedAt:   issuedAt,
				ExpiresAt:    issuedAt.AddDate(0, 24, 0),
			}
		}
		for _, targetID := range []string{syntheticHost, syntheticRenter} {
			t.Run(targetID, func(t *testing.T) {
				records := memoryRepository{
					syntheticHost:   verification(syntheticHost, otherVerifiedAt),
					syntheticRenter: verification(syntheticRenter, otherVerifiedAt),
				}
				records[targetID] = verification(targetID, targetVerifiedAt)
				service, err := NewService(records)
				if err != nil {
					t.Fatal(err)
				}

				err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now)
				if boundaryOffset <= 0 {
					if err != nil {
						t.Fatalf("verification at/before now was rejected: target=%s verified=%s now=%s err=%v", targetID, targetVerifiedAt, now, err)
					}
					return
				}
				if !errors.Is(err, ErrNotVerified) {
					t.Fatalf("future verification accepted: target=%s verified=%s now=%s err=%v", targetID, targetVerifiedAt, now, err)
				}
			})
		}
	})
}

func FuzzRequireForBookingRejectsValidityBeyondRenewalLimitAcrossCalendarDates(f *testing.F) {
	f.Add(uint16(0))
	f.Add(uint16(59))
	f.Add(uint16(365))
	f.Fuzz(func(t *testing.T, dateSeed uint16) {
		year := 2000 + int(dateSeed%40)
		dayOfYear := int(dateSeed % 366)
		verifiedAt := time.Date(year, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, dayOfYear)
		renewalLimit := verifiedAt.AddDate(0, 24, 0)
		now := renewalLimit.Add(-time.Nanosecond)
		overLimitExpiry := renewalLimit.Add(time.Nanosecond)
		verification := func(subjectID string, issuedAt, deadline time.Time) Verification {
			return Verification{
				SubjectID:    subjectID,
				Kind:         KindIdentityDocument,
				State:        StateApproved,
				ReviewMethod: ReviewMethodManualDocuments,
				VerifiedAt:   issuedAt,
				ExpiresAt:    deadline,
			}
		}
		for _, targetID := range []string{syntheticHost, syntheticRenter} {
			t.Run(targetID, func(t *testing.T) {
				records := memoryRepository{
					syntheticHost:   verification(syntheticHost, verifiedAt, renewalLimit),
					syntheticRenter: verification(syntheticRenter, verifiedAt, renewalLimit),
				}
				otherID := syntheticHost
				if targetID == syntheticHost {
					otherID = syntheticRenter
				}
				otherVerifiedAt := verifiedAt.Add(2 * time.Nanosecond)
				records[otherID] = verification(otherID, otherVerifiedAt, otherVerifiedAt.AddDate(0, 24, 0))
				records[targetID] = verification(targetID, verifiedAt, overLimitExpiry)
				service, err := NewService(records)
				if err != nil {
					t.Fatal(err)
				}

				if err = service.RequireForBooking(context.Background(), syntheticHost, syntheticRenter, now); !errors.Is(err, ErrNotVerified) {
					t.Fatalf("verification beyond 24-month limit accepted: target=%s verified=%s limit=%s expiry=%s now=%s err=%v", targetID, verifiedAt, renewalLimit, overLimitExpiry, now, err)
				}
			})
		}
	})
}
