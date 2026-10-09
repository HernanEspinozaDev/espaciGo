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

// AddSynthetic creates a durable candidate record before writing the fixed
// server-side PNG. Rejections and reused idempotency results leave a durable
// cleanup job; successful gallery metadata consumes that candidate atomically.
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
	if err := s.repo.ReserveCandidate(ctx, owner, spaceID, id, item.CreatedAt); err != nil {
		return Photo{}, false, err
	}
	writer, err := s.repo.BeginCandidate(ctx, owner, spaceID, id)
	if err != nil {
		_ = s.repo.QueueCandidateCleanup(context.Background(), id, s.now().UTC())
		return Photo{}, false, err
	}
	defer func() { _ = writer.Close() }()
	if err := s.files.Put(ctx, id, blob); err != nil {
		_ = writer.QueueCleanup(context.Background(), s.now().UTC())
		return Photo{}, false, ErrFileStore
	}
	result, reused, err := writer.Add(ctx, item, key, s.now().UTC())
	if err != nil {
		// Commit errors can be ambiguous. The repository removes the candidate
		// row atomically with a confirmed gallery insert; queueing only changes
		// candidates still present, so a committed file is never deleted.
		_ = writer.Close()
		_ = s.repo.QueueCandidateCleanup(context.Background(), id, s.now().UTC())
		return Photo{}, false, err
	}
	return result, reused, nil
}

func (s *Service) List(ctx context.Context, owner, spaceID string) ([]Photo, error) {
	if !validUUID(owner) || !validUUID(spaceID) {
		return nil, ErrNotFound
	}
	return s.repo.List(ctx, owner, spaceID)
}

func (s *Service) Content(ctx context.Context, owner, spaceID, photoID string) (Photo, []byte, error) {
	item, err := s.Metadata(ctx, owner, spaceID, photoID)
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

func (s *Service) Metadata(ctx context.Context, owner, spaceID, photoID string) (Photo, error) {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return Photo{}, ErrNotFound
	}
	item, err := s.repo.Get(ctx, owner, spaceID, photoID)
	if err != nil {
		return Photo{}, err
	}
	return item, nil
}

func (s *Service) Remove(ctx context.Context, owner, spaceID, photoID string) (Removal, error) {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return Removal{}, ErrNotFound
	}
	return s.repo.Remove(ctx, owner, spaceID, photoID)
}

// CleanRetiredOnce drains withdrawn photos and files that never became photos.
// Both paths are durable, retryable and safe across Backend restarts.
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
	candidates, err := s.repo.ClaimCandidateCleanup(ctx, limit)
	if err != nil {
		return err
	}
	for _, id := range candidates {
		if err := s.files.Delete(ctx, id); err != nil {
			if markErr := s.repo.FailCandidateCleanup(ctx, id, s.now().UTC()); markErr != nil {
				return markErr
			}
			continue
		}
		if err := s.repo.CompleteCandidateCleanup(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
