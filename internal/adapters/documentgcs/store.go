// Package documentgcs encrypts verification documents before storing them in
// purpose-specific private object stores.
package documentgcs

import (
	"bytes"
	"context"
	"errors"
	"regexp"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentcodec"
	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const keyVersionMetadata = "key_version"
const maxEnvelopeBytes = documentcodec.MaxDocumentBytes + 64

var versionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
var errUnavailable = errors.New("private document unavailable")

// ObjectStore is compatible with cloudstorage.Store's metadata-aware methods.
type ObjectStore interface {
	PutWithMetadata(context.Context, string, []byte, map[string]string) error
	GetWithMetadata(context.Context, string, int64) ([]byte, map[string]string, error)
	Delete(context.Context, string) error
}

type Option func(*Store) error

// WithCurrentKeyVersion selects the version used for new documents.
func WithCurrentKeyVersion(version string) Option {
	return func(s *Store) error { s.currentVersion = version; return nil }
}

type Store struct {
	identity, space ObjectStore
	keys            map[string][]byte
	currentVersion  string
}

// New validates and clones the keyring. Identity and space stores are separate
// injections; callers choose their backing stores without inferred bucket names.
func New(identity, space ObjectStore, keys map[string][]byte, options ...Option) (*Store, error) {
	if identity == nil || space == nil || len(keys) == 0 {
		return nil, documentverification.ErrInvalid
	}
	s := &Store{identity: identity, space: space, keys: make(map[string][]byte, len(keys))}
	for version, key := range keys {
		if !versionPattern.MatchString(version) || len(key) != 32 {
			return nil, documentverification.ErrInvalid
		}
		for otherVersion, existing := range s.keys {
			if otherVersion != version && bytes.Equal(existing, key) {
				return nil, documentverification.ErrInvalid
			}
		}
		s.keys[version] = append([]byte(nil), key...)
	}
	for _, option := range options {
		if option == nil || option(s) != nil {
			return nil, documentverification.ErrInvalid
		}
	}
	if s.currentVersion == "" {
		if len(s.keys) != 1 {
			return nil, documentverification.ErrInvalid
		}
		for v := range s.keys {
			s.currentVersion = v
		}
	}
	if !versionPattern.MatchString(s.currentVersion) || s.keys[s.currentVersion] == nil {
		return nil, documentverification.ErrInvalid
	}
	return s, nil
}

// Close releases the injected object-store clients when they expose Close.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	closeBackend := func(backend ObjectStore) error {
		if closer, ok := backend.(interface{ Close() error }); ok {
			return closer.Close()
		}
		return nil
	}
	return errors.Join(closeBackend(s.identity), closeBackend(s.space))
}

func (s *Store) PutIdentityDocument(ctx context.Context, owner, caseID, fileID string, content []byte) error {
	scope, err := documentcodec.IdentityScope(owner, caseID, fileID)
	if err != nil {
		return documentverification.ErrInvalid
	}
	return s.put(ctx, s.identity, scope, fileID, content)
}
func (s *Store) PutSpaceDocument(ctx context.Context, owner, spaceID, caseID, fileID string, content []byte) error {
	scope, err := documentcodec.SpaceScope(owner, spaceID, caseID, fileID)
	if err != nil {
		return documentverification.ErrInvalid
	}
	return s.put(ctx, s.space, scope, fileID, content)
}
func (s *Store) ReadIdentityDocument(ctx context.Context, owner, caseID, fileID string) ([]byte, error) {
	scope, err := documentcodec.IdentityScope(owner, caseID, fileID)
	if err != nil {
		return nil, documentverification.ErrInvalid
	}
	return s.read(ctx, s.identity, scope, fileID)
}
func (s *Store) ReadSpaceDocument(ctx context.Context, owner, spaceID, caseID, fileID string) ([]byte, error) {
	scope, err := documentcodec.SpaceScope(owner, spaceID, caseID, fileID)
	if err != nil {
		return nil, documentverification.ErrInvalid
	}
	return s.read(ctx, s.space, scope, fileID)
}
func (s *Store) DeleteIdentityDocument(ctx context.Context, owner, caseID, fileID string) error {
	scope, err := documentcodec.IdentityScope(owner, caseID, fileID)
	if err != nil {
		return documentverification.ErrInvalid
	}
	return s.delete(ctx, s.identity, scope, fileID)
}
func (s *Store) DeleteSpaceDocument(ctx context.Context, owner, spaceID, caseID, fileID string) error {
	scope, err := documentcodec.SpaceScope(owner, spaceID, caseID, fileID)
	if err != nil {
		return documentverification.ErrInvalid
	}
	return s.delete(ctx, s.space, scope, fileID)
}
func (s *Store) put(ctx context.Context, backend ObjectStore, scope documentcodec.Scope, objectID string, content []byte) error {
	if s == nil || len(content) == 0 || len(content) > documentcodec.MaxDocumentBytes {
		return documentverification.ErrQuota
	}
	if documentverification.ValidateID(objectID) != nil {
		return documentverification.ErrInvalid
	}
	blob, err := documentcodec.Seal(s.keys[s.currentVersion], scope, content)
	if err != nil {
		return documentverification.ErrInvalid
	}
	if err = backend.PutWithMetadata(ctx, objectID, blob, map[string]string{keyVersionMetadata: s.currentVersion}); err != nil {
		return errUnavailable
	}
	return nil
}
func (s *Store) read(ctx context.Context, backend ObjectStore, scope documentcodec.Scope, objectID string) ([]byte, error) {
	if s == nil {
		return nil, errUnavailable
	}
	if documentverification.ValidateID(objectID) != nil {
		return nil, documentverification.ErrInvalid
	}
	blob, metadata, err := backend.GetWithMetadata(ctx, objectID, maxEnvelopeBytes)
	if err != nil {
		return nil, errUnavailable
	}
	version := metadata[keyVersionMetadata]
	if !versionPattern.MatchString(version) {
		return nil, errUnavailable
	}
	key, ok := s.keys[version]
	if !ok {
		return nil, errUnavailable
	}
	plain, err := documentcodec.Open(key, scope, blob)
	if err != nil {
		return nil, errUnavailable
	}
	return plain, nil
}
func (s *Store) delete(ctx context.Context, backend ObjectStore, scope documentcodec.Scope, objectID string) error {
	if s == nil {
		return errUnavailable
	}
	if documentverification.ValidateID(objectID) != nil {
		return documentverification.ErrInvalid
	}
	if err := backend.Delete(ctx, objectID); err != nil {
		return errUnavailable
	}
	return nil
}

var _ documentverification.UploadDocumentStore = (*Store)(nil)
var _ documentverification.ReviewerDocumentStore = (*Store)(nil)
