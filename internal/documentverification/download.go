package documentverification

import (
	"context"
	"errors"
)

var ErrDocumentStorage = errors.New("document verification: private document unavailable")

// SpaceDownloadRepository must load the case and current owner authoritatively,
// then invoke the callback before releasing the lock that serializes owner
// changes. It must not infer ownership from object keys or historical SubjectID.
type SpaceDownloadRepository interface {
	WithSpaceCaseForDownload(context.Context, string, string, func(context.Context, Snapshot, Authorization) error) error
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
	var downloaded DownloadedDocument
	err := s.repository.WithSpaceCaseForDownload(ctx, actor, caseID, func(readCtx context.Context, x Snapshot, auth Authorization) error {
		if x.ID != caseID {
			return ErrNotFound
		}
		if err := AuthorizeSpaceOwnerDownload(x, auth, actor, fileID); err != nil {
			return err
		}
		var selected *File
		for i := range x.Files {
			if x.Files[i].ID == fileID {
				selected = &x.Files[i]
				break
			}
		}
		if selected == nil {
			return ErrNotFound
		}
		content, err := s.store.ReadSpaceDocument(readCtx, auth.SpaceOwnerID, x.SpaceID, x.ID, selected.ID)
		if err != nil {
			return ErrDocumentStorage
		}
		if int64(len(content)) != selected.Size {
			return ErrDocumentStorage
		}
		downloaded = DownloadedDocument{Content: content, MIME: selected.MIME}
		return nil
	})
	if err != nil {
		return DownloadedDocument{}, err
	}
	return downloaded, nil
}
