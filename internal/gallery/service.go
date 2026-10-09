package gallery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type Service struct {
	repo  Repository
	files FileStore
	ids   IDGenerator
	now   func() time.Time
}

func NewService(repo Repository, files FileStore, ids IDGenerator, now func() time.Time) (*Service, error) {
	if repo == nil || files == nil || ids == nil || now == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, files: files, ids: ids, now: now}, nil
}

// AddSynthetic creates a fixed server-side PNG and persists owner-scoped metadata.
// The file is removed if the database rejects the operation or an idempotent
// retry resolves to a previous photo.
func (s *Service) AddSynthetic(ctx context.Context, owner, spaceID, key string) (Photo, bool, error) {
	key = strings.TrimSpace(key)
	if !validUUID(owner) || !validUUID(spaceID) || len(key) < 8 || len(key) > 128 {
		return Photo{}, false, ErrInvalid
	}
	blob, err := verification.SyntheticPNG()
	if err != nil {
		return Photo{}, false, err
	}
	id, err := s.ids.ID()
	if err != nil {
		return Photo{}, false, err
	}
	if !validUUID(id) {
		return Photo{}, false, ErrInvalid
	}
	digest := sha256.Sum256(blob)
	item := Photo{ID: id, Fixture: SyntheticFixture, MIME: "image/png", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(blob)), CreatedAt: s.now().UTC()}
	if err := s.files.Put(ctx, id, blob); err != nil {
		return Photo{}, false, ErrFileStore
	}
	created := false
	defer func() {
		if !created {
			_ = s.files.Delete(context.Background(), id)
		}
	}()
	result, reused, err := s.repo.Add(ctx, owner, spaceID, item, key)
	if err != nil {
		return Photo{}, false, err
	}
	if reused {
		return result, true, nil
	}
	created = true
	return result, false, nil
}

func (s *Service) List(ctx context.Context, owner, spaceID string) ([]Photo, error) {
	if !validUUID(owner) || !validUUID(spaceID) {
		return nil, ErrNotFound
	}
	return s.repo.List(ctx, owner, spaceID)
}

func (s *Service) Content(ctx context.Context, owner, spaceID, photoID string) (Photo, []byte, error) {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return Photo{}, nil, ErrNotFound
	}
	item, err := s.repo.Get(ctx, owner, spaceID, photoID)
	if err != nil {
		return Photo{}, nil, err
	}
	blob, err := s.files.Get(ctx, item.ID)
	if err != nil {
		return Photo{}, nil, ErrFileStore
	}
	digest := sha256.Sum256(blob)
	if int64(len(blob)) != item.Size || hex.EncodeToString(digest[:]) != item.SHA256 {
		return Photo{}, nil, ErrFileStore
	}
	return item, blob, nil
}

func (s *Service) Remove(ctx context.Context, owner, spaceID, photoID string) (Removal, error) {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return Removal{}, ErrNotFound
	}
	return s.repo.Remove(ctx, owner, spaceID, photoID)
}

// CleanRetiredOnce is safe to repeat. Suppression has its own durable file-job
// worker; concurrent deletes of the same content are idempotent at the store.
func (s *Service) CleanRetiredOnce(ctx context.Context, limit int) error {
	if limit < 1 || limit > 500 {
		return ErrInvalid
	}
	ids, err := s.repo.PendingCleanup(ctx, limit)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.files.Delete(ctx, id); err != nil {
			if markErr := s.repo.FailCleanup(ctx, id, s.now().UTC()); markErr != nil {
				return markErr
			}
			continue
		}
		if err := s.repo.CompleteCleanup(ctx, id, s.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
