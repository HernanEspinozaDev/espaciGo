package operation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	if repo == nil || files == nil || ids == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, files: files, ids: ids, now: now}, nil
}

// Record adds one fixed server-generated PNG and commits its operation while
// holding the same participant and reservation locks used by M06/M07.
func (s *Service) Record(ctx context.Context, actor, reservation, key string, input Input) (Item, error) {
	actor, reservation, key = strings.ToLower(strings.TrimSpace(actor)), strings.ToLower(strings.TrimSpace(reservation)), strings.TrimSpace(key)
	if !idPattern.MatchString(actor) || !idPattern.MatchString(reservation) || len(key) < 8 || len(key) > 200 || !validInput(input) {
		return Item{}, ErrInvalid
	}
	blob, err := verification.SyntheticPNG()
	if err != nil {
		return Item{}, ErrFileStore
	}
	fileID, err := s.ids.ID()
	if err != nil || !idPattern.MatchString(fileID) {
		return Item{}, ErrInvalid
	}
	created := s.now().UTC().Truncate(time.Microsecond)
	if err = s.repo.ReserveCandidate(ctx, actor, reservation, input.Kind, fileID, created); err != nil {
		return Item{}, fmt.Errorf("reserve private evidence candidate: %w", err)
	}
	writer, err := s.repo.BeginCandidate(ctx, actor, reservation, input.Kind, fileID)
	if err != nil {
		_ = s.repo.QueueCandidateCleanup(context.Background(), fileID, s.now().UTC())
		return Item{}, fmt.Errorf("lock rental operation: %w", err)
	}
	defer func() { _ = writer.Close() }()
	if err = s.files.Put(ctx, fileID, blob); err != nil {
		_ = writer.QueueCleanup(context.Background(), s.now().UTC())
		return Item{}, ErrFileStore
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		_ = writer.QueueCleanup(context.Background(), s.now().UTC())
		return Item{}, ErrInvalid
	}
	fingerprint := sha256.Sum256(canonical)
	fileDigest := sha256.Sum256(blob)
	evidence := Evidence{ID: fileID, Fixture: "synthetic-png-v1", MIME: "image/png", SHA256: hex.EncodeToString(fileDigest[:]), SizeBytes: int64(len(blob))}
	item, reused, err := writer.Apply(ctx, fileID, key, input, evidence, fingerprint[:], s.now)
	if err != nil {
		// A committed operation removes its candidate in the same transaction.
		// QueueCandidateCleanup is harmless after such a commit, and recovers a
		// file when the operation was rejected or the result was ambiguous.
		_ = writer.Close()
		_ = s.repo.QueueCandidateCleanup(context.Background(), fileID, s.now().UTC())
		return Item{}, fmt.Errorf("commit rental operation: %w", err)
	}
	item.Reused = reused
	return item, nil
}

func validInput(input Input) bool {
	switch input.Kind {
	case CheckIn, CheckOut:
		return input.Observations == ""
	case Receipt:
		return input.Comments == ""
	default:
		return false
	}
}

func (s *Service) List(ctx context.Context, actor, reservation string) ([]Item, error) {
	if !idPattern.MatchString(actor) || !idPattern.MatchString(reservation) {
		return nil, ErrNotFound
	}
	items, err := s.repo.List(ctx, strings.ToLower(actor), strings.ToLower(reservation))
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Evidence = nonNilEvidence(items[i].Evidence)
		for j := range items[i].Evidence {
			items[i].Evidence[j].ContentURL = "/api/v1/local/booking-trial/reservations/" + items[i].ReservationID + "/evidence/" + items[i].Evidence[j].ID
		}
	}
	return items, nil
}

func (s *Service) EvidenceContent(ctx context.Context, actor, reservation, evidenceID string) (Evidence, []byte, error) {
	if !idPattern.MatchString(actor) || !idPattern.MatchString(reservation) || !idPattern.MatchString(evidenceID) {
		return Evidence{}, nil, ErrNotFound
	}
	item, err := s.repo.Evidence(ctx, strings.ToLower(actor), strings.ToLower(reservation), strings.ToLower(evidenceID))
	if err != nil {
		return Evidence{}, nil, err
	}
	content, err := s.files.Get(ctx, item.ID)
	if err != nil {
		return Evidence{}, nil, ErrFileStore
	}
	digest := sha256.Sum256(content)
	if int64(len(content)) != item.SizeBytes || hex.EncodeToString(digest[:]) != item.SHA256 {
		return Evidence{}, nil, ErrFileStore
	}
	return item, content, nil
}

func (s *Service) CleanRetiredOnce(ctx context.Context, limit int) error {
	if limit < 1 || limit > 500 {
		return ErrInvalid
	}
	ids, err := s.repo.ClaimCandidateCleanup(ctx, s.now().UTC(), limit)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.files.Delete(ctx, id); err != nil {
			if err = s.repo.FailCandidateCleanup(ctx, id, s.now().UTC()); err != nil {
				return err
			}
			continue
		}
		if err = s.repo.CompleteCandidateCleanup(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func nonNilEvidence(value []Evidence) []Evidence {
	if value == nil {
		return []Evidence{}
	}
	return value
}
