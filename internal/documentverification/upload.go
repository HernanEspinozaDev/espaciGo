package documentverification

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
)

const MaxUploadBytes = 10 << 20

const uploadCleanupTimeout = 5 * time.Second

type UploadInspector interface {
	Inspect(Purpose, string, []byte) (File, []byte, error)
}

type UploadDocumentStore interface {
	PutIdentityDocument(context.Context, string, string, string, []byte) error
	PutSpaceDocument(context.Context, string, string, string, string, []byte) error
	DeleteIdentityDocument(context.Context, string, string, string) error
	DeleteSpaceDocument(context.Context, string, string, string, string) error
}

type UploadReceipt struct {
	File     File
	Revision uint64
}

type UploadService struct {
	cases     *Service
	inspector UploadInspector
	store     UploadDocumentStore
	newID     func() string
}

func NewUploadService(cases *Service, inspector UploadInspector, store UploadDocumentStore) (*UploadService, error) {
	if cases == nil || inspector == nil || store == nil {
		return nil, ErrInvalid
	}
	return &UploadService{cases: cases, inspector: inspector, store: store, newID: uuid.NewString}, nil
}

// Upload inspects and stores normalized bytes before atomically adding pending
// metadata. A failed CAS or database write compensates by removing only the
// immutable object created for this upload.
func (s *UploadService) Upload(ctx context.Context, actor, caseID string, expectedRevision uint64, raw []byte) (UploadReceipt, error) {
	if s == nil || s.cases == nil || s.inspector == nil || s.store == nil || ctx == nil ||
		ValidateID(actor) != nil || ValidateID(caseID) != nil || len(raw) == 0 {
		return UploadReceipt{}, ErrInvalid
	}
	if len(raw) > MaxUploadBytes {
		return UploadReceipt{}, ErrQuota
	}
	if err := ctx.Err(); err != nil {
		return UploadReceipt{}, err
	}

	snapshot, err := s.cases.LoadOwnCase(ctx, actor, caseID)
	if err != nil {
		return UploadReceipt{}, err
	}
	if snapshot.Revision != expectedRevision || snapshot.Revision == ^uint64(0) {
		return UploadReceipt{}, ErrConflict
	}
	if snapshot.State != CaseDraft && snapshot.State != CaseNeedsEvidence {
		return UploadReceipt{}, ErrTransition
	}
	if snapshot.Purpose != PurposeIdentity && snapshot.Purpose != PurposeSpace {
		return UploadReceipt{}, ErrInvalid
	}
	if err := ValidateQuota(snapshot.Purpose, snapshot.Files); err != nil {
		return UploadReceipt{}, err
	}
	limit := 3
	if snapshot.Purpose == PurposeSpace {
		limit = 10
	}
	if len(snapshot.Files) >= limit {
		return UploadReceipt{}, ErrQuota
	}
	if s.newID == nil {
		return UploadReceipt{}, ErrInvalid
	}
	fileID := s.newID()
	if ValidateID(fileID) != nil {
		return UploadReceipt{}, ErrInvalid
	}

	file, normalized, err := s.inspector.Inspect(snapshot.Purpose, fileID, raw)
	if err != nil {
		return UploadReceipt{}, ErrInvalid
	}
	if len(normalized) == 0 || len(normalized) > MaxUploadBytes || file.ID != fileID ||
		file.State != FilePending || file.Size != int64(len(normalized)) ||
		file.ContentSHA256 != sha256.Sum256(normalized) || ValidateQuota(snapshot.Purpose, append(append([]File(nil), snapshot.Files...), file)) != nil {
		return UploadReceipt{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return UploadReceipt{}, err
	}

	var putErr error
	switch snapshot.Purpose {
	case PurposeIdentity:
		putErr = s.store.PutIdentityDocument(ctx, snapshot.SubjectID, snapshot.ID, file.ID, normalized)
	case PurposeSpace:
		putErr = s.store.PutSpaceDocument(ctx, snapshot.SubjectID, snapshot.SpaceID, snapshot.ID, file.ID, normalized)
	}
	if putErr != nil {
		return UploadReceipt{}, ErrDocumentStorage
	}

	if err := s.cases.AddFile(ctx, actor, snapshot.ID, snapshot.Purpose, expectedRevision, file); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), uploadCleanupTimeout)
		defer cancel()
		var cleanupErr error
		switch snapshot.Purpose {
		case PurposeIdentity:
			cleanupErr = s.store.DeleteIdentityDocument(cleanupCtx, snapshot.SubjectID, snapshot.ID, file.ID)
		case PurposeSpace:
			cleanupErr = s.store.DeleteSpaceDocument(cleanupCtx, snapshot.SubjectID, snapshot.SpaceID, snapshot.ID, file.ID)
		}
		if cleanupErr != nil {
			return UploadReceipt{}, ErrDocumentStorage
		}
		return UploadReceipt{}, err
	}
	return UploadReceipt{File: file, Revision: expectedRevision + 1}, nil
}
