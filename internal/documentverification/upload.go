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
// metadata. Ambiguous storage writes and failed CAS/database writes compensate
// by removing only the immutable object created for this upload.
func (s *UploadService) Upload(ctx context.Context, actor, caseID string, expectedRevision uint64, role EvidenceRole, raw []byte) (UploadReceipt, error) {
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
	if snapshot.ID != caseID || snapshot.SubjectID != actor ||
		(snapshot.Purpose == PurposeIdentity && snapshot.SpaceID != "") ||
		(snapshot.Purpose == PurposeSpace && ValidateID(snapshot.SpaceID) != nil) {
		return UploadReceipt{}, ErrNotFound
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
	typedIdentity := snapshot.Purpose == PurposeIdentity && snapshot.DocumentFormatDeclared != ""
	if typedIdentity {
		if !validEvidenceRole(role) {
			return UploadReceipt{}, ErrInvalid
		}
	} else if role != "" {
		return UploadReceipt{}, ErrInvalid
	}
	if err := ValidateQuota(snapshot.Purpose, snapshot.Files); err != nil {
		return UploadReceipt{}, err
	}
	limit := 3
	if snapshot.Purpose == PurposeSpace {
		limit = 10
	}
	replacingRole := false
	if typedIdentity {
		for _, existing := range snapshot.Files {
			if existing.EvidenceRole == role {
				replacingRole = true
				break
			}
		}
	}
	if len(snapshot.Files) >= limit && !replacingRole {
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
	if typedIdentity && file.MIME != "image/jpeg" && file.MIME != "image/png" {
		return UploadReceipt{}, ErrInvalid
	}
	file.EvidenceRole = role
	filesForQuota := append([]File(nil), snapshot.Files...)
	if replacingRole {
		for i := range filesForQuota {
			if filesForQuota[i].EvidenceRole == role {
				filesForQuota = append(filesForQuota[:i], filesForQuota[i+1:]...)
				break
			}
		}
	}
	filesForQuota = append(filesForQuota, file)
	if len(normalized) == 0 || len(normalized) > MaxUploadBytes || file.ID != fileID ||
		file.State != FilePending || file.Size != int64(len(normalized)) ||
		file.ContentSHA256 != sha256.Sum256(normalized) || ValidateQuota(snapshot.Purpose, filesForQuota) != nil {
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
		_ = s.cleanupStoredObject(snapshot, file.ID)
		return UploadReceipt{}, ErrDocumentStorage
	}

	if err := s.cases.AddFile(ctx, actor, snapshot.ID, snapshot.Purpose, expectedRevision, file); err != nil {
		if cleanupErr := s.cleanupStoredObject(snapshot, file.ID); cleanupErr != nil {
			return UploadReceipt{}, ErrDocumentStorage
		}
		return UploadReceipt{}, err
	}
	return UploadReceipt{File: file, Revision: expectedRevision + 1}, nil
}

func (s *UploadService) cleanupStoredObject(snapshot Snapshot, fileID string) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), uploadCleanupTimeout)
	defer cancel()
	switch snapshot.Purpose {
	case PurposeIdentity:
		return s.store.DeleteIdentityDocument(cleanupCtx, snapshot.SubjectID, snapshot.ID, fileID)
	case PurposeSpace:
		return s.store.DeleteSpaceDocument(cleanupCtx, snapshot.SubjectID, snapshot.SpaceID, snapshot.ID, fileID)
	default:
		return ErrInvalid
	}
}
