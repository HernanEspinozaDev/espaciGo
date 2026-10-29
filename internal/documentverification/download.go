package documentverification

import (
	"context"
	"errors"
)

var ErrDocumentStorage = errors.New("document verification: private document unavailable")

// SpaceDownloadRepository must return the case, current SpaceOwnerID, and actor
// identity from one authoritative read. It must not infer ownership from object
// keys or from the historical SubjectID alone.
type SpaceDownloadRepository interface {
	LoadSpaceCaseForDownload(context.Context, string, string) (Snapshot, Authorization, error)
}

// SpaceDocumentStore reads a private object using IDs derived from the loaded
// authoritative case. Implementations must not accept a client-provided key.
type SpaceDocumentStore interface {
	ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error)
}

type DownloadedDocument struct {
	Content []byte
	MIME    string
}

type SpaceDownloadService struct {
	repository SpaceDownloadRepository
	store      SpaceDocumentStore
}

func NewSpaceDownloadService(repository SpaceDownloadRepository, store SpaceDocumentStore) (*SpaceDownloadService, error) {
	if repository == nil || store == nil {
		return nil, ErrInvalid
	}
	return &SpaceDownloadService{repository: repository, store: store}, nil
}

func (s *SpaceDownloadService) Download(ctx context.Context, actor, caseID, fileID string) (DownloadedDocument, error) {
	if s == nil || s.repository == nil || s.store == nil || ValidateID(actor) != nil || ValidateID(caseID) != nil || ValidateID(fileID) != nil {
		return DownloadedDocument{}, ErrInvalid
	}
	x, auth, err := s.repository.LoadSpaceCaseForDownload(ctx, actor, caseID)
	if err != nil {
		return DownloadedDocument{}, err
	}
	if x.ID != caseID {
		return DownloadedDocument{}, ErrNotFound
	}
	if err := AuthorizeSpaceOwnerDownload(x, auth, actor, fileID); err != nil {
		return DownloadedDocument{}, err
	}
	var selected *File
	for i := range x.Files {
		if x.Files[i].ID == fileID {
			selected = &x.Files[i]
			break
		}
	}
	if selected == nil {
		return DownloadedDocument{}, ErrNotFound
	}
	content, err := s.store.ReadSpaceDocument(ctx, auth.SpaceOwnerID, x.SpaceID, x.ID, selected.ID)
	if err != nil {
		return DownloadedDocument{}, ErrDocumentStorage
	}
	if int64(len(content)) != selected.Size {
		return DownloadedDocument{}, ErrDocumentStorage
	}
	return DownloadedDocument{Content: content, MIME: selected.MIME}, nil
}
