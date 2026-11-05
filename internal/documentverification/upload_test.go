package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
)

const uploadFileID = "77777777-7777-4777-8777-777777777777"

type uploadInspectorFake struct {
	file    File
	content []byte
	err     error
	calls   int
	purpose Purpose
	id      string
	input   []byte
}

func (f *uploadInspectorFake) Inspect(p Purpose, id string, data []byte) (File, []byte, error) {
	f.calls++
	f.purpose, f.id = p, id
	f.input = append([]byte(nil), data...)
	if f.err != nil {
		return File{}, nil, f.err
	}
	file := f.file
	file.ID = id
	return file, append([]byte(nil), f.content...), nil
}

type uploadStoreFake struct {
	putCalls    int
	deleteCalls int
	putErr      error
	deleteErr   error
	owner       string
	space       string
	caseID      string
	fileID      string
	content     []byte
	purpose     Purpose
	cancel      context.CancelFunc
}

func (f *uploadStoreFake) PutIdentityDocument(_ context.Context, ownerID, caseID, fileID string, content []byte) error {
	f.putCalls++
	f.record(PurposeIdentity, ownerID, "", caseID, fileID, content)
	if f.cancel != nil {
		f.cancel()
	}
	return f.putErr
}

func (f *uploadStoreFake) PutSpaceDocument(_ context.Context, ownerID, spaceID, caseID, fileID string, content []byte) error {
	f.putCalls++
	f.record(PurposeSpace, ownerID, spaceID, caseID, fileID, content)
	if f.cancel != nil {
		f.cancel()
	}
	return f.putErr
}

func (f *uploadStoreFake) DeleteIdentityDocument(_ context.Context, ownerID, caseID, fileID string) error {
	f.record(PurposeIdentity, ownerID, "", caseID, fileID, nil)
	f.deleteCalls++
	return f.deleteErr
}

func (f *uploadStoreFake) DeleteSpaceDocument(_ context.Context, ownerID, spaceID, caseID, fileID string) error {
	f.record(PurposeSpace, ownerID, spaceID, caseID, fileID, nil)
	f.deleteCalls++
	return f.deleteErr
}

func (f *uploadStoreFake) record(p Purpose, ownerID, spaceID, caseID, fileID string, content []byte) {
	f.purpose, f.owner, f.space, f.caseID, f.fileID = p, ownerID, spaceID, caseID, fileID
	f.content = append([]byte(nil), content...)
}

func uploadServiceFixture(t *testing.T, p Purpose) (*UploadService, *memoryRepository, *uploadInspectorFake, *uploadStoreFake) {
	t.Helper()
	cases, repo, _ := fixture(t, p)
	repo.state.State = CaseDraft
	repo.state.Revision = 1
	repo.auth.ActorID = subject
	repo.auth.SubjectID = subject
	repo.auth.SpaceOwnerID = subject
	normalized := []byte("normalized synthetic document")
	inspector := &uploadInspectorFake{
		content: normalized,
		file: File{
			State:         FilePending,
			Size:          int64(len(normalized)),
			MIME:          "image/png",
			Pages:         1,
			ContentSHA256: sha256.Sum256(normalized),
		},
	}
	store := &uploadStoreFake{}
	uploader, err := NewUploadService(cases, inspector, store)
	if err != nil {
		t.Fatal(err)
	}
	uploader.newID = func() string { return uploadFileID }
	return uploader, repo, inspector, store
}

func TestUploadPersistsInspectedEncryptedObjectAndPendingMetadata(t *testing.T) {
	for _, purpose := range []Purpose{PurposeIdentity, PurposeSpace} {
		t.Run(string(purpose), func(t *testing.T) {
			uploader, repo, inspector, store := uploadServiceFixture(t, purpose)
			raw := []byte("raw synthetic document")
			receipt, err := uploader.Upload(context.Background(), subject, caseID, 1, raw)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.File.ID != uploadFileID || receipt.File.State != FilePending || receipt.Revision != 2 {
				t.Fatalf("unexpected upload receipt: %+v", receipt)
			}
			if inspector.calls != 1 || inspector.purpose != purpose || inspector.id != uploadFileID || string(inspector.input) != string(raw) {
				t.Fatalf("inspector received wrong scope/content: %+v", inspector)
			}
			if store.putCalls != 1 || store.deleteCalls != 0 || store.purpose != purpose || store.owner != subject || store.caseID != caseID || store.fileID != uploadFileID || string(store.content) != string(inspector.content) {
				t.Fatalf("stored object has wrong scope/content: %+v", store)
			}
			if purpose == PurposeSpace && store.space != space || purpose == PurposeIdentity && store.space != "" {
				t.Fatalf("object purpose scope mismatch: %+v", store)
			}
			if repo.state.Revision != 2 || len(repo.state.Files) != 2 || repo.state.Files[1] != receipt.File {
				t.Fatalf("pending metadata was not persisted: %+v", repo.state)
			}
		})
	}
}

func TestUploadRejectsStaleRevisionBeforeInspectingOrWriting(t *testing.T) {
	uploader, _, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	if _, err := uploader.Upload(context.Background(), subject, caseID, 2, []byte("synthetic")); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale upload error=%v, want ErrConflict", err)
	}
	if inspector.calls != 0 || store.putCalls != 0 {
		t.Fatal("stale upload reached inspection or private storage")
	}
}

func TestUploadCompensatesStoredObjectWhenPersistenceFails(t *testing.T) {
	uploader, repo, _, store := uploadServiceFixture(t, PurposeSpace)
	dbErr := errors.New("synthetic persistence failure")
	repo.err = dbErr
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, []byte("synthetic")); !errors.Is(err, dbErr) {
		t.Fatalf("upload error=%v, want persistence error", err)
	}
	if store.putCalls != 1 || store.deleteCalls != 1 || repo.state.Revision != 1 || len(repo.state.Files) != 1 {
		t.Fatalf("failed persistence was not compensated: store=%+v repo=%+v", store, repo.state)
	}
}

func TestUploadReportsStorageFailureWhenCompensationFails(t *testing.T) {
	uploader, repo, _, store := uploadServiceFixture(t, PurposeIdentity)
	repo.err = errors.New("synthetic persistence failure")
	store.deleteErr = errors.New("synthetic cleanup failure")
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, []byte("synthetic")); !errors.Is(err, ErrDocumentStorage) {
		t.Fatalf("upload error=%v, want safe storage error", err)
	}
}

func TestUploadRejectsInconsistentInspectionBeforeStorage(t *testing.T) {
	uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	inspector.file.ContentSHA256 = [32]byte{1}
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, []byte("synthetic")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("inconsistent inspection result error=%v, want ErrInvalid", err)
	}
	if store.putCalls != 0 || len(repo.state.Files) != 1 {
		t.Fatal("inconsistent inspected bytes were persisted or stored")
	}
}

func TestUploadRejectsOversizeBeforeInspection(t *testing.T) {
	uploader, _, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, make([]byte, MaxUploadBytes+1)); !errors.Is(err, ErrQuota) {
		t.Fatalf("oversize upload error=%v, want ErrQuota", err)
	}
	if inspector.calls != 0 || store.putCalls != 0 {
		t.Fatal("oversize upload reached inspection or private storage")
	}
}

func TestUploadCompensationIgnoresCanceledRequestContext(t *testing.T) {
	uploader, repo, _, store := uploadServiceFixture(t, PurposeIdentity)
	repo.err = context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.cancel = cancel
	if _, err := uploader.Upload(ctx, subject, caseID, 1, []byte("synthetic")); !errors.Is(err, context.Canceled) {
		t.Fatalf("upload error=%v, want context cancellation", err)
	}
	if store.putCalls != 1 || store.deleteCalls != 1 {
		t.Fatalf("canceled request did not compensate object write: %+v", store)
	}
}
