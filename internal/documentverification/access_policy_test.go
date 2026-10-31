package documentverification

import (
	"errors"
	"testing"
	"time"
)

const (
	downloadingOwnerID = "66666666-6666-4666-8666-666666666666"
	downloadingActorID = "77777777-7777-4777-8777-777777777777"
	downloadingSpaceID = "88888888-8888-4888-8888-888888888888"
	downloadingCaseID  = "99999999-9999-4999-8999-999999999999"
	downloadingFileID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func downloadPolicySnapshot(state CaseState, fileState FileState) Snapshot {
	return Snapshot{
		Purpose:   PurposeSpace,
		ID:        downloadingCaseID,
		SubjectID: downloadingOwnerID,
		SpaceID:   downloadingSpaceID,
		State:     state,
		Files:     []File{{ID: downloadingFileID, State: fileState, Size: 64, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}},
	}
}

func TestAuthorizeSpaceOwnerDownloadAllowsCleanFileOnlyWhileCaseOpen(t *testing.T) {
	openStates := []CaseState{CaseDraft, CaseSubmitted, CaseInReview, CaseNeedsEvidence}
	for _, state := range openStates {
		t.Run(string(state), func(t *testing.T) {
			x := downloadPolicySnapshot(state, FileClean)
			a := Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}
			if err := AuthorizeSpaceOwnerDownload(x, a, downloadingOwnerID, downloadingFileID); err != nil {
				t.Fatalf("clean file in open case denied: %v", err)
			}
		})
	}
}

func TestAuthorizeSpaceOwnerDownloadDeniesClosedCase(t *testing.T) {
	closedStates := []CaseState{CaseApproved, CaseRejected, CaseExpired, CaseRevoked}
	for _, state := range closedStates {
		t.Run(string(state), func(t *testing.T) {
			x := downloadPolicySnapshot(state, FileClean)
			a := Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}
			if err := AuthorizeSpaceOwnerDownload(x, a, downloadingOwnerID, downloadingFileID); !errors.Is(err, ErrTransition) {
				t.Fatalf("closed case download err=%v, want %v", err, ErrTransition)
			}
		})
	}
}

func TestAuthorizeSpaceOwnerDownloadDeniesNonCleanFile(t *testing.T) {
	for _, state := range []FileState{FilePending, FileBlocked, FileError} {
		t.Run(string(state), func(t *testing.T) {
			x := downloadPolicySnapshot(CaseInReview, state)
			a := Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}
			if err := AuthorizeSpaceOwnerDownload(x, a, downloadingOwnerID, downloadingFileID); !errors.Is(err, ErrFilesNotClean) {
				t.Fatalf("non-clean file download err=%v, want %v", err, ErrFilesNotClean)
			}
		})
	}
}

func TestAuthorizeSpaceOwnerDownloadDeniesExpiredRetention(t *testing.T) {
	x := downloadPolicySnapshot(CaseInReview, FileClean)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	x.Files[0].RetentionAt = now
	a := Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}
	if err := AuthorizeSpaceOwnerDownloadAt(x, a, downloadingOwnerID, downloadingFileID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired retained document download err=%v, want %v", err, ErrNotFound)
	}
}

func TestAuthorizeSpaceOwnerDownloadAllowsUntilRetentionDeadline(t *testing.T) {
	x := downloadPolicySnapshot(CaseInReview, FileClean)
	deadline := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	x.Files[0].RetentionAt = deadline
	a := Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}
	if err := AuthorizeSpaceOwnerDownloadAt(x, a, downloadingOwnerID, downloadingFileID, deadline.Add(-time.Nanosecond)); err != nil {
		t.Fatalf("download before retention deadline err=%v, want nil", err)
	}
}

func TestAuthorizeSpaceOwnerDownloadRequiresCurrentOwnerAndOriginalCaseOwner(t *testing.T) {
	t.Run("non-owner actor", func(t *testing.T) {
		x := downloadPolicySnapshot(CaseInReview, FileClean)
		a := Authorization{ActorID: downloadingActorID, SpaceOwnerID: downloadingOwnerID}
		if err := AuthorizeSpaceOwnerDownload(x, a, downloadingActorID, downloadingFileID); !errors.Is(err, ErrCapability) {
			t.Fatalf("non-owner actor err=%v, want %v", err, ErrCapability)
		}
	})
	t.Run("ownership changed, new case required", func(t *testing.T) {
		x := downloadPolicySnapshot(CaseInReview, FileClean)
		a := Authorization{ActorID: downloadingActorID, SpaceOwnerID: downloadingActorID}
		if err := AuthorizeSpaceOwnerDownload(x, a, downloadingActorID, downloadingFileID); !errors.Is(err, ErrCapability) {
			t.Fatalf("old case after ownership transfer err=%v, want %v", err, ErrCapability)
		}
	})
}

func TestAuthorizeSpaceOwnerDownloadRejectsUnknownFileAndUntrustedOwner(t *testing.T) {
	x := downloadPolicySnapshot(CaseDraft, FileClean)
	a := Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}
	if err := AuthorizeSpaceOwnerDownload(x, a, downloadingOwnerID, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown file err=%v, want %v", err, ErrNotFound)
	}
	a.SpaceOwnerID = "not-a-uuid"
	if err := AuthorizeSpaceOwnerDownload(x, a, downloadingOwnerID, downloadingFileID); !errors.Is(err, ErrCapability) {
		t.Fatalf("invalid current owner err=%v, want %v", err, ErrCapability)
	}
}
