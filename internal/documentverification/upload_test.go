package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
	"time"
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
	cancel  func()
}

func (f *uploadInspectorFake) Inspect(p Purpose, id string, data []byte) (File, []byte, error) {
	f.calls++
	f.purpose, f.id = p, id
	f.input = append([]byte(nil), data...)
	if f.cancel != nil {
		f.cancel()
	}
	if f.err != nil {
		return File{}, nil, f.err
	}
	file := f.file
	file.ID = id
	return file, append([]byte(nil), f.content...), nil
}

type uploadStoreFake struct {
	putCalls          int
	deleteCalls       int
	putErr            error
	persistOnPutError bool
	deleteErr         error
	stored            bool
	owner             string
	space             string
	caseID            string
	fileID            string
	content           []byte
	purpose           Purpose
	cancel            context.CancelFunc
}

func (f *uploadStoreFake) PutIdentityDocument(_ context.Context, ownerID, caseID, fileID string, content []byte) error {
	f.putCalls++
	f.record(PurposeIdentity, ownerID, "", caseID, fileID, content)
	if f.cancel != nil {
		f.cancel()
	}
	if f.putErr == nil || f.persistOnPutError {
		f.stored = true
	}
	return f.putErr
}

func (f *uploadStoreFake) PutSpaceDocument(_ context.Context, ownerID, spaceID, caseID, fileID string, content []byte) error {
	f.putCalls++
	f.record(PurposeSpace, ownerID, spaceID, caseID, fileID, content)
	if f.cancel != nil {
		f.cancel()
	}
	if f.putErr == nil || f.persistOnPutError {
		f.stored = true
	}
	return f.putErr
}

func (f *uploadStoreFake) DeleteIdentityDocument(_ context.Context, ownerID, caseID, fileID string) error {
	f.record(PurposeIdentity, ownerID, "", caseID, fileID, nil)
	f.deleteCalls++
	if f.deleteErr == nil {
		f.stored = false
	}
	return f.deleteErr
}

func (f *uploadStoreFake) DeleteSpaceDocument(_ context.Context, ownerID, spaceID, caseID, fileID string) error {
	f.record(PurposeSpace, ownerID, spaceID, caseID, fileID, nil)
	f.deleteCalls++
	if f.deleteErr == nil {
		f.stored = false
	}
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
			receipt, err := uploader.Upload(context.Background(), subject, caseID, 1, "", raw)
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

func TestTypedIdentityUploadRejectsPDFBeforeStorage(t *testing.T) {
	uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	repo.state.DocumentFormatDeclared = DocumentFormatNew
	repo.state.IdentityDeclarationRevisionID = "88888888-8888-4888-8888-888888888888"
	repo.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	repo.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo.state.Files = []File{
		{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 10, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 10, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 10, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
	}
	repo.auth.IdentityDeclarationRevisionID = repo.state.IdentityDeclarationRevisionID
	inspector.file.MIME = "application/pdf"
	inspector.file.EvidenceRole = EvidenceRoleIdentityFront
	inspector.file.Pages = 1
	inspector.file.Size = int64(len(inspector.content))
	inspector.file.ContentSHA256 = sha256.Sum256(inspector.content)

	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, EvidenceRoleIdentityFront, []byte("synthetic PDF")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("PDF upload error=%v, want ErrInvalid", err)
	}
	if inspector.calls != 1 || store.putCalls != 0 || store.deleteCalls != 0 {
		t.Fatalf("PDF upload reached private storage: inspections=%d storage=%+v", inspector.calls, store)
	}
}

func TestTypedIdentityUploadReplacesRoleAtFullQuota(t *testing.T) {
	uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	repo.state.DocumentFormatDeclared = DocumentFormatNew
	repo.state.IdentityDeclarationRevisionID = "88888888-8888-4888-8888-888888888888"
	repo.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	repo.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo.state.Files = []File{
		{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 10, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 10, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}},
		{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 10, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
	}
	repo.auth.IdentityDeclarationRevisionID = repo.state.IdentityDeclarationRevisionID
	inspector.file.EvidenceRole = EvidenceRoleIdentityFront
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, EvidenceRoleIdentityFront, []byte("replacement")); err != nil {
		t.Fatalf("replace matching slot at full quota: %v", err)
	}
	if len(repo.state.Files) != 3 || repo.state.Files[0].ID != uploadFileID || repo.state.Files[0].EvidenceRole != EvidenceRoleIdentityFront || store.putCalls != 1 {
		t.Fatalf("role replacement was not persisted: files=%+v store=%+v", repo.state.Files, store)
	}
}

func TestTypedIdentityUploadUsesRequestedRoleInsteadOfInspectorMetadata(t *testing.T) {
	uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	repo.state.DocumentFormatDeclared = DocumentFormatNew
	repo.state.IdentityDeclarationRevisionID = "88888888-8888-4888-8888-888888888888"
	repo.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	repo.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo.auth.IdentityDeclarationRevisionID = repo.state.IdentityDeclarationRevisionID
	repo.state.Files = nil
	inspector.file.EvidenceRole = EvidenceRoleIdentityBack

	requestedRole := EvidenceRoleIdentityFront
	receipt, err := uploader.Upload(context.Background(), subject, caseID, 1, requestedRole, []byte("synthetic"))
	if err != nil {
		t.Fatalf("typed identity upload: %v", err)
	}
	if receipt.File.EvidenceRole != requestedRole {
		t.Fatalf("receipt role=%q, want validated request role %q", receipt.File.EvidenceRole, requestedRole)
	}
	if repo.state.Revision != 2 || len(repo.state.Files) != 1 || repo.state.Files[0] != receipt.File || repo.state.Files[0].EvidenceRole != requestedRole {
		t.Fatalf("persisted file does not use requested role: snapshot=%+v receipt=%+v", repo.state, receipt)
	}
	if inspector.calls != 1 || store.putCalls != 1 || store.deleteCalls != 0 {
		t.Fatalf("inspection/storage calls mismatch: inspections=%d storage=%+v", inspector.calls, store)
	}
}

func TestUploadRejectsRoleForSpaceAndMissingRoleForTypedIdentity(t *testing.T) {
	t.Run("space rejects role", func(t *testing.T) {
		uploader, _, inspector, store := uploadServiceFixture(t, PurposeSpace)
		if _, err := uploader.Upload(context.Background(), subject, caseID, 1, EvidenceRoleIdentityFront, []byte("synthetic")); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error=%v, want ErrInvalid", err)
		}
		if inspector.calls != 0 || store.putCalls != 0 {
			t.Fatal("invalid space role reached inspection or storage")
		}
	})
	t.Run("typed identity requires role", func(t *testing.T) {
		uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
		repo.state.DocumentFormatDeclared = DocumentFormatUnsure
		repo.state.IdentityDeclarationRevisionID = "88888888-8888-4888-8888-888888888888"
		repo.state.CaptureConsentVersion = IdentityCaptureConsentVersion
		repo.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
		repo.auth.IdentityDeclarationRevisionID = repo.state.IdentityDeclarationRevisionID
		if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic")); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error=%v, want ErrInvalid", err)
		}
		if inspector.calls != 0 || store.putCalls != 0 {
			t.Fatal("missing identity role reached inspection or storage")
		}
	})
}

type misScopedOwnCaseReader struct {
	Repository
	snapshot Snapshot
}

func (r misScopedOwnCaseReader) LoadOwnCase(context.Context, string, string) (Snapshot, error) {
	return r.snapshot, nil
}

func (misScopedOwnCaseReader) ListOwnCases(context.Context, string) ([]Snapshot, error) {
	return nil, ErrInvalid
}

func TestUploadRejectsMisScopedSnapshotBeforeInspectionOrStorage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		purpose Purpose
		mutate  func(*Snapshot)
	}{
		{
			name:    "case ID differs from request",
			purpose: PurposeIdentity,
			mutate: func(snapshot *Snapshot) {
				snapshot.ID = "88888888-8888-4888-8888-888888888888"
			},
		},
		{
			name:    "subject differs from actor",
			purpose: PurposeIdentity,
			mutate: func(snapshot *Snapshot) {
				snapshot.SubjectID = "88888888-8888-4888-8888-888888888888"
			},
		},
		{
			name:    "identity snapshot has space ID",
			purpose: PurposeIdentity,
			mutate:  func(snapshot *Snapshot) { snapshot.SpaceID = space },
		},
		{
			name:    "space snapshot has malformed space ID",
			purpose: PurposeSpace,
			mutate:  func(snapshot *Snapshot) { snapshot.SpaceID = "not-a-uuid" },
		},
		{
			name:    "space snapshot has empty space ID",
			purpose: PurposeSpace,
			mutate:  func(snapshot *Snapshot) { snapshot.SpaceID = "" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uploader, repo, inspector, store := uploadServiceFixture(t, tc.purpose)
			snapshot, err := repo.LoadOwnCase(context.Background(), subject, caseID)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&snapshot)
			uploader.cases, err = NewService(misScopedOwnCaseReader{Repository: repo, snapshot: snapshot})
			if err != nil {
				t.Fatal(err)
			}

			receipt, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic"))
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("mis-scoped snapshot error=%v, want privacy-safe ErrNotFound", err)
			}
			if receipt.File.ID != "" || receipt.Revision != 0 {
				t.Errorf("mis-scoped snapshot returned an upload receipt: %+v", receipt)
			}
			if inspector.calls != 0 {
				t.Errorf("mis-scoped snapshot reached inspection: %d calls", inspector.calls)
			}
			if store.putCalls != 0 || store.deleteCalls != 0 {
				t.Errorf("mis-scoped snapshot reached storage: %+v", store)
			}
			if repo.calls != 0 || repo.state.Revision != 1 || len(repo.state.Files) != 1 || !reflect.DeepEqual(repo.audit, TransitionAudit{}) {
				t.Errorf("mis-scoped snapshot reached/mutated transition: calls=%d state=%+v audit=%+v", repo.calls, repo.state, repo.audit)
			}
		})
	}
}

func TestUploadRejectsStaleRevisionBeforeInspectingOrWriting(t *testing.T) {
	uploader, _, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	if _, err := uploader.Upload(context.Background(), subject, caseID, 2, "", []byte("synthetic")); !errors.Is(err, ErrConflict) {
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
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic")); !errors.Is(err, dbErr) {
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
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic")); !errors.Is(err, ErrDocumentStorage) {
		t.Fatalf("upload error=%v, want safe storage error", err)
	}
}

func TestUploadCompensatesObjectWhenPutReturnsAmbiguousFailure(t *testing.T) {
	for _, purpose := range []Purpose{PurposeIdentity, PurposeSpace} {
		t.Run(string(purpose), func(t *testing.T) {
			uploader, repo, _, store := uploadServiceFixture(t, purpose)
			store.putErr = errors.New("synthetic response lost after object creation")
			store.persistOnPutError = true

			if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic")); !errors.Is(err, ErrDocumentStorage) {
				t.Fatalf("upload error=%v, want safe storage error", err)
			}
			if store.putCalls != 1 || store.deleteCalls != 1 || store.stored || store.purpose != purpose {
				t.Fatalf("ambiguous Put was not compensated: storage=%+v", store)
			}
			if repo.state.Revision != 1 || len(repo.state.Files) != 1 {
				t.Fatalf("ambiguous Put changed case metadata: %+v", repo.state)
			}
		})
	}
}

func TestUploadAmbiguousPutAndFailedCompensationReturnsSafeErrorWithoutMetadata(t *testing.T) {
	uploader, repo, _, store := uploadServiceFixture(t, PurposeIdentity)
	store.putErr = errors.New("synthetic response lost after object creation")
	store.persistOnPutError = true
	store.deleteErr = errors.New("synthetic cleanup failure")
	before := repo.state
	before.Files = append([]File(nil), repo.state.Files...)
	beforeAudit := repo.audit

	receipt, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic"))
	if err != ErrDocumentStorage || receipt != (UploadReceipt{}) {
		t.Fatalf("upload receipt=%+v error=%v; want empty receipt and safe storage error", receipt, err)
	}
	if store.putCalls != 1 || store.deleteCalls != 1 || !store.stored || store.purpose != PurposeIdentity || store.owner != subject || store.caseID != caseID || store.fileID != uploadFileID {
		t.Fatalf("ambiguous Put/failed Delete state=%+v", store)
	}
	if repo.calls != 0 || !reflect.DeepEqual(repo.state, before) || !reflect.DeepEqual(repo.audit, beforeAudit) {
		t.Fatalf("ambiguous write changed metadata: calls=%d state=%+v audit=%+v", repo.calls, repo.state, repo.audit)
	}
}

func TestTypedIdentityUploadFailsClosedWhenInspectionErrors(t *testing.T) {
	uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	repo.state.DocumentFormatDeclared = DocumentFormatNew
	repo.state.IdentityDeclarationRevisionID = "88888888-8888-4888-8888-888888888888"
	repo.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	repo.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo.auth.IdentityDeclarationRevisionID = repo.state.IdentityDeclarationRevisionID
	inspectionErr := errors.New("synthetic inspection unavailable")
	inspector.err = inspectionErr
	before := repo.state
	before.Files = append([]File(nil), repo.state.Files...)
	if repo.state.RevokedAt != nil {
		value := *repo.state.RevokedAt
		before.RevokedAt = &value
	}
	if repo.state.SupersededAt != nil {
		value := *repo.state.SupersededAt
		before.SupersededAt = &value
	}
	beforeAudit := repo.audit

	receipt, err := uploader.Upload(
		context.Background(), subject, caseID, 1, EvidenceRoleIdentityFront,
		[]byte("synthetic untrusted document"),
	)
	if !errors.Is(err, ErrInvalid) || errors.Is(err, inspectionErr) {
		t.Fatalf("inspection failure error=%v, want safe ErrInvalid without exposing scanner details", err)
	}
	if receipt.File.ID != "" || receipt.Revision != 0 {
		t.Fatalf("failed inspection returned an upload receipt: %+v", receipt)
	}
	if inspector.calls != 1 || inspector.purpose != PurposeIdentity ||
		inspector.id != uploadFileID || string(inspector.input) != "synthetic untrusted document" {
		t.Fatalf("inspector received wrong scope/content: %+v", inspector)
	}
	if store.putCalls != 0 || store.deleteCalls != 0 {
		t.Fatalf("inspection failure reached private storage: %+v", store)
	}
	if !reflect.DeepEqual(repo.state, before) || !reflect.DeepEqual(repo.audit, beforeAudit) {
		t.Fatalf("inspection failure changed case metadata or audit: state=%+v audit=%+v", repo.state, repo.audit)
	}
}

func TestUploadRejectsInconsistentInspectionBeforeStorage(t *testing.T) {
	uploader, repo, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	inspector.file.ContentSHA256 = [32]byte{1}
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("synthetic")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("inconsistent inspection result error=%v, want ErrInvalid", err)
	}
	if store.putCalls != 0 || len(repo.state.Files) != 1 {
		t.Fatal("inconsistent inspected bytes were persisted or stored")
	}
}

func TestUploadRejectsOversizeBeforeInspection(t *testing.T) {
	uploader, _, inspector, store := uploadServiceFixture(t, PurposeIdentity)
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", make([]byte, MaxUploadBytes+1)); !errors.Is(err, ErrQuota) {
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
	if _, err := uploader.Upload(ctx, subject, caseID, 1, "", []byte("synthetic")); !errors.Is(err, context.Canceled) {
		t.Fatalf("upload error=%v, want context cancellation", err)
	}
	if store.putCalls != 1 || store.deleteCalls != 1 {
		t.Fatalf("canceled request did not compensate object write: %+v", store)
	}
}

func TestUploadServiceConstructorAndParameterBoundaries(t *testing.T) {
	cases, _, _ := fixture(t, PurposeIdentity)
	inspector := &uploadInspectorFake{}
	store := &uploadStoreFake{}

	// 1. Constructor parameter validations
	if _, err := NewUploadService(nil, inspector, store); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil cases, got %v", err)
	}
	if _, err := NewUploadService(cases, nil, store); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil inspector, got %v", err)
	}
	if _, err := NewUploadService(cases, inspector, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil store, got %v", err)
	}
	uploaderValid, err := NewUploadService(cases, inspector, store)
	if err != nil || uploaderValid == nil || uploaderValid.newID == nil {
		t.Fatalf("expected valid service with newID generator, got %v (service: %+v)", err, uploaderValid)
	}

	// 2. Upload receiver and input argument validations
	var nilUploader *UploadService
	if _, err := nilUploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil receiver, got %v", err)
	}
	uploader, repo, _, _ := uploadServiceFixture(t, PurposeIdentity)
	if _, err := uploader.Upload(nil, subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil context, got %v", err)
	}
	if _, err := uploader.Upload(context.Background(), "bad-actor", caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for invalid actor UUID, got %v", err)
	}
	if _, err := uploader.Upload(context.Background(), subject, "bad-case-id", 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for invalid caseID UUID, got %v", err)
	}
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for empty raw bytes, got %v", err)
	}

	// 3. Pre-canceled context returns error directly
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := uploader.Upload(canceledCtx, subject, caseID, 1, "", []byte("data")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// 4. Maximum uint64 revision rejected
	if _, err := uploader.Upload(context.Background(), subject, caseID, ^uint64(0), "", []byte("data")); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for max uint64 revision, got %v", err)
	}

	// 5. Ineligible case states rejected with ErrTransition
	for _, st := range []CaseState{CaseApproved, CaseRejected, CaseInReview, CaseRevoked} {
		t.Run(string(st), func(t *testing.T) {
			repo.state.State = st
			if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrTransition) {
				t.Fatalf("expected ErrTransition for state %q, got %v", st, err)
			}
		})
	}
	repo.state.State = CaseDraft

	// 6. Unknown case purpose rejected with ErrInvalid
	repo.state.Purpose = Purpose("unknown-purpose")
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for unknown purpose, got %v", err)
	}
	repo.state.Purpose = PurposeIdentity

	// 7. Nil or invalid newID generator
	uploader.newID = nil
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil newID, got %v", err)
	}
	uploader.newID = func() string { return "not-a-uuid" }
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for invalid fileID from newID, got %v", err)
	}
	uploader.newID = func() string { return uploadFileID }

	// 8. cleanupStoredObject default branch for unknown purpose
	if err := uploader.cleanupStoredObject(Snapshot{Purpose: Purpose("unknown")}, uploadFileID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid from cleanupStoredObject with unknown purpose, got %v", err)
	}

	// 9. Case not found from LoadOwnCase
	const notFoundCaseID = "00000000-0000-4000-8000-000000000000"
	if _, err := uploader.Upload(context.Background(), subject, notFoundCaseID, 1, "", []byte("data")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent caseID, got %v", err)
	}

	// 10. Existing corrupt files in repo state fails initial ValidateQuota
	repo.state.Files = []File{{ID: "invalid-file-uuid"}}
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for corrupt pre-existing files, got %v", err)
	}

	// 11. Initial files at limit rejected with ErrQuota
	validFile := func(hexDigit rune) File {
		return File{
			ID:            "00000000-0000-4000-8000-00000000000" + string(hexDigit),
			State:         FilePending,
			Size:          10,
			MIME:          "image/png",
			Pages:         1,
			ContentSHA256: [32]byte{1},
		}
	}
	repo.state.Files = []File{validFile('1'), validFile('2'), validFile('3')}
	if _, err := uploader.Upload(context.Background(), subject, caseID, 1, "", []byte("data")); !errors.Is(err, ErrQuota) {
		t.Fatalf("expected ErrQuota when untyped files at limit, got %v", err)
	}
	repo.state.Files = nil

	// 12. Context canceled during inspection caught before storage write
	uploaderInspect, _, _, storeInspect := uploadServiceFixture(t, PurposeIdentity)
	ctxInspect, cancelInspect := context.WithCancel(context.Background())
	defer cancelInspect()
	normalizedInspect := []byte("inspected")
	inspectorInspect := &uploadInspectorFake{
		file: File{
			State:         FilePending,
			Size:          int64(len(normalizedInspect)),
			MIME:          "image/png",
			Pages:         1,
			ContentSHA256: sha256.Sum256(normalizedInspect),
		},
		content: normalizedInspect,
		cancel:  cancelInspect,
	}
	uploaderInspect.inspector = inspectorInspect
	if _, err := uploaderInspect.Upload(ctxInspect, subject, caseID, 1, "", []byte("data")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled during inspection, got %v", err)
	}
	if storeInspect.putCalls != 0 {
		t.Fatalf("canceled upload reached storage: %+v", storeInspect)
	}
}

func TestUploadSpaceDocumentFlowAndCompensation(t *testing.T) {
	uploader, repo, _, store := uploadServiceFixture(t, PurposeSpace)
	raw := []byte("synthetic space document raw bytes")

	// 1. Successful upload on PurposeSpace invokes PutSpaceDocument
	receipt, err := uploader.Upload(context.Background(), subject, caseID, 1, "", raw)
	if err != nil {
		t.Fatalf("unexpected space upload error: %v", err)
	}
	if receipt.Revision != 2 || receipt.File.ID != uploadFileID {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if store.putCalls != 1 || store.purpose != PurposeSpace || store.owner != subject || store.space != space || store.caseID != caseID || !store.stored {
		t.Fatalf("unexpected store state after PutSpaceDocument: %+v", store)
	}
	if len(repo.state.Files) != 2 || repo.state.Files[1].ID != uploadFileID {
		t.Fatalf("file not added to repo state: %+v", repo.state.Files)
	}

	// 2. Compensation on CAS persistence failure invokes DeleteSpaceDocument
	uploader2, repo2, _, store2 := uploadServiceFixture(t, PurposeSpace)
	dbErr := errors.New("synthetic persistence failure")
	repo2.err = dbErr
	if _, err := uploader2.Upload(context.Background(), subject, caseID, 1, "", raw); !errors.Is(err, dbErr) {
		t.Fatalf("expected dbErr, got %v", err)
	}
	if store2.putCalls != 1 || store2.deleteCalls != 1 || store2.stored {
		t.Fatalf("compensation failed to remove space document from store: %+v", store2)
	}
}
