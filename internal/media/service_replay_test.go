package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"testing"
	"time"
)

const (
	replayOwnerID = "00000000-0000-4000-8000-000000000001"
	replaySpaceID = "00000000-0000-4000-8000-000000000002"
	replayPhotoID = "00000000-0000-4000-8000-000000000003"
	replayNewID   = "00000000-0000-4000-8000-000000000004"
)

type replacementReplayRepo struct {
	Repository
	prior Reservation
	photo Photo
}

func (r *replacementReplayRepo) Reserve(context.Context, string, string, string, Purpose, string, string, time.Time, time.Time, ReservationOptions) (Reservation, bool, error) {
	return r.prior, true, nil
}

func (r *replacementReplayRepo) GetSpace(context.Context, string, string, string) (Photo, error) {
	return r.photo, nil
}

type replacementReplayStore struct{ ObjectStore }
type replacementReplayIDs struct{}

func (replacementReplayIDs) ID() (string, error) { return replayNewID, nil }

func TestUploadSpaceReplacementReplayRequiresSameExpectedRevision(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	raw := encoded.Bytes()
	digest := sha256.Sum256(raw)
	inputHash := hex.EncodeToString(digest[:])
	storedRevision := "2026-10-27T22:18:16Z"
	changedRevision := "2026-10-27T22:18:15Z"

	for _, tc := range []struct {
		name              string
		requestedRevision string
		wantConflict      bool
	}{
		{name: "same revision reuses ready result", requestedRevision: storedRevision},
		{name: "different revision conflicts", requestedRevision: changedRevision, wantConflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &replacementReplayRepo{
				prior: Reservation{
					ID:                  replayNewID,
					OwnerID:             replayOwnerID,
					SpaceID:             replaySpaceID,
					Purpose:             PurposeSpace,
					State:               "ready",
					InputHash:           inputHash,
					ReplacementTargetID: replayPhotoID,
					ExpectedRevision:    storedRevision,
				},
				photo: Photo{ID: replayNewID, SpaceID: replaySpaceID, State: "ready"},
			}
			svc, err := NewService(repo, &replacementReplayStore{}, replacementReplayIDs{}, func() time.Time {
				return time.Date(2026, 10, 27, 22, 20, 0, 0, time.UTC)
			})
			if err != nil {
				t.Fatal(err)
			}

			result, err := svc.UploadSpaceReplacement(context.Background(), replayOwnerID, replaySpaceID, "synthetic-idempotency-key", replayPhotoID, tc.requestedRevision, raw)
			if tc.wantConflict {
				if !errors.Is(err, ErrConflict) {
					t.Fatalf("changed expected revision error = %v, want ErrConflict", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("same expected revision replay: %v", err)
			}
			if !result.Reused || result.Photo.ID != replayNewID {
				t.Fatalf("same expected revision replay result = %+v, want reused ready asset %s", result, replayNewID)
			}
		})
	}
}
