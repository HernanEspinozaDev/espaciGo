package documentstorage

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentcodec"
	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
)

var (
	ErrInvalid = errors.New("private document storage: invalid scope or content")
	ErrStorage = errors.New("private document storage unavailable")
)

// BlobStore must provide create-only Put semantics. Object identifiers are
// scoped opaque UUIDs; document authorization remains above this adapter.
type BlobStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

// Store encrypts private documents before handing their envelopes to a blob
// backend and authenticates owner/case/purpose scope when reading them.
type Store struct {
	blobs BlobStore
	key   []byte
}

var _ documentverification.UploadDocumentStore = (*Store)(nil)
var _ documentverification.ReviewerDocumentStore = (*Store)(nil)

func New(blobs BlobStore, encryptionKey []byte) (*Store, error) {
	if blobs == nil || len(encryptionKey) != 32 {
		return nil, ErrInvalid
	}
	return &Store{blobs: blobs, key: append([]byte(nil), encryptionKey...)}, nil
}

func (s *Store) PutIdentityDocument(ctx context.Context, ownerID, caseID, fileID string, content []byte) error {
	scope, err := documentcodec.IdentityScope(ownerID, caseID, fileID)
	if err != nil {
		return ErrInvalid
	}
	objectID, err := scopedObjectID(scope)
	if err != nil {
		return ErrInvalid
	}
	return s.put(ctx, objectID, scope, content)
}

func (s *Store) PutSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string, content []byte) error {
	scope, err := documentcodec.SpaceScope(ownerID, spaceID, caseID, fileID)
	if err != nil {
		return ErrInvalid
	}
	objectID, err := scopedObjectID(scope)
	if err != nil {
		return ErrInvalid
	}
	return s.put(ctx, objectID, scope, content)
}

func (s *Store) ReadIdentityDocument(ctx context.Context, ownerID, caseID, fileID string) ([]byte, error) {
	scope, err := documentcodec.IdentityScope(ownerID, caseID, fileID)
	if err != nil {
		return nil, ErrInvalid
	}
	objectID, err := scopedObjectID(scope)
	if err != nil {
		return nil, ErrInvalid
	}
	return s.read(ctx, objectID, scope)
}

func (s *Store) ReadSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) ([]byte, error) {
	scope, err := documentcodec.SpaceScope(ownerID, spaceID, caseID, fileID)
	if err != nil {
		return nil, ErrInvalid
	}
	objectID, err := scopedObjectID(scope)
	if err != nil {
		return nil, ErrInvalid
	}
	return s.read(ctx, objectID, scope)
}

func (s *Store) DeleteIdentityDocument(ctx context.Context, ownerID, caseID, fileID string) error {
	scope, err := documentcodec.IdentityScope(ownerID, caseID, fileID)
	if err != nil {
		return ErrInvalid
	}
	objectID, err := scopedObjectID(scope)
	if err != nil {
		return ErrInvalid
	}
	return s.remove(ctx, objectID)
}

func (s *Store) DeleteSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) error {
	scope, err := documentcodec.SpaceScope(ownerID, spaceID, caseID, fileID)
	if err != nil {
		return ErrInvalid
	}
	objectID, err := scopedObjectID(scope)
	if err != nil {
		return ErrInvalid
	}
	return s.remove(ctx, objectID)
}

// scopedObjectID hashes the codec's canonical AAD into a deterministic UUID.
// The hash is an object-key transform, not an authorization secret.
func scopedObjectID(scope documentcodec.Scope) (string, error) {
	aad, err := documentcodec.KeyForScope(scope)
	if err != nil {
		return "", err
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("espacigo-private-document-v1/"+aad)).String(), nil
}

func (s *Store) put(ctx context.Context, objectID string, scope documentcodec.Scope, content []byte) error {
	if err := s.validateContext(ctx); err != nil {
		return err
	}
	sealed, err := documentcodec.Seal(s.key, scope, content)
	if errors.Is(err, documentcodec.ErrInvalid) {
		return ErrInvalid
	}
	if err != nil {
		return ErrStorage
	}
	if err := s.blobs.Put(ctx, objectID, sealed); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrStorage
	}
	return nil
}

func (s *Store) read(ctx context.Context, objectID string, scope documentcodec.Scope) ([]byte, error) {
	if err := s.validateContext(ctx); err != nil {
		return nil, err
	}
	sealed, err := s.blobs.Get(ctx, objectID)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, ErrStorage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := documentcodec.Open(s.key, scope, sealed)
	if err != nil {
		return nil, ErrStorage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return content, nil
}

func (s *Store) remove(ctx context.Context, objectID string) error {
	if err := s.validateContext(ctx); err != nil {
		return err
	}
	if err := s.blobs.Delete(ctx, objectID); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrStorage
	}
	return nil
}

func (s *Store) validateContext(ctx context.Context) error {
	if s == nil || s.blobs == nil || len(s.key) != 32 || ctx == nil {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
