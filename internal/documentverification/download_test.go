package documentverification

import (
	"context"
	"errors"
	"testing"
)

type downloadCaseRepo struct {
	snapshot Snapshot
	auth     Authorization
	actor    string
	caseID   string
}

func (r *downloadCaseRepo) LoadSpaceCaseForDownload(_ context.Context, actor, caseID string) (Snapshot, Authorization, error) {
	r.actor, r.caseID = actor, caseID
	return r.snapshot, r.auth, nil
}

type syntheticDocumentStore struct {
	content []byte
	calls   int
}

func (s *syntheticDocumentStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.calls++
	return append([]byte(nil), s.content...), nil
}

func TestSpaceDownloadAuthorizesBeforeReadingPrivateObject(t *testing.T) {
	content := []byte("synthetic clean document")
	x := downloadPolicySnapshot(CaseInReview, FileClean)
	x.Files[0].Size = int64(len(content))
	repo := &downloadCaseRepo{snapshot: x, auth: Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}}
	store := &syntheticDocumentStore{content: content}
	svc, err := NewSpaceDownloadService(repo, store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Download(context.Background(), downloadingOwnerID, downloadingCaseID, downloadingFileID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Content) != string(content) || got.MIME != "image/jpeg" {
		t.Fatalf("unexpected download: %+v", got)
	}
	if repo.actor != downloadingOwnerID || repo.caseID != downloadingCaseID || store.calls != 1 {
		t.Fatalf("authorization/storage scope mismatch: repo=%+v store calls=%d", repo, store.calls)
	}
}

func TestSpaceDownloadNeverReadsClosedOrUncleanFiles(t *testing.T) {
	for _, tc := range []struct {
		name      string
		caseState CaseState
		fileState FileState
		want      error
	}{
		{name: "closed", caseState: CaseApproved, fileState: FileClean, want: ErrTransition},
		{name: "pending", caseState: CaseInReview, fileState: FilePending, want: ErrFilesNotClean},
		{name: "blocked", caseState: CaseInReview, fileState: FileBlocked, want: ErrFilesNotClean},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte("synthetic clean document")
			x := downloadPolicySnapshot(tc.caseState, tc.fileState)
			x.Files[0].Size = int64(len(content))
			repo := &downloadCaseRepo{snapshot: x, auth: Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}}
			store := &syntheticDocumentStore{content: content}
			svc, _ := NewSpaceDownloadService(repo, store)
			if _, err := svc.Download(context.Background(), downloadingOwnerID, downloadingCaseID, downloadingFileID); !errors.Is(err, tc.want) {
				t.Fatalf("download err=%v want=%v", err, tc.want)
			}
			if store.calls != 0 {
				t.Fatal("unauthorized or unclean file reached private storage")
			}
		})
	}
}

func TestSpaceDownloadRejectsBlobLengthMismatchBeforeReturningBytes(t *testing.T) {
	x := downloadPolicySnapshot(CaseDraft, FileClean)
	x.Files[0].Size++
	repo := &downloadCaseRepo{snapshot: x, auth: Authorization{ActorID: downloadingOwnerID, SpaceOwnerID: downloadingOwnerID}}
	store := &syntheticDocumentStore{content: []byte("synthetic clean document")}
	svc, _ := NewSpaceDownloadService(repo, store)
	if _, err := svc.Download(context.Background(), downloadingOwnerID, downloadingCaseID, downloadingFileID); err == nil {
		t.Fatal("truncated/corrupt object was returned")
	}
}
