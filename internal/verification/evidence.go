package verification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"time"
)

const SyntheticEvidenceFixture = "synthetic-png-v1"

type Evidence struct {
	ID             string    `json:"id"`
	VerificationID string    `json:"verification_id"`
	FixtureCode    string    `json:"fixture_code"`
	MIMEType       string    `json:"mime_type"`
	SizeBytes      int64     `json:"size_bytes"`
	SHA256         string    `json:"sha256"`
	CreatedAt      time.Time `json:"created_at"`
}

type EvidenceRepository interface {
	CreateEvidence(context.Context, Evidence) (Evidence, error)
	ListOwnEvidence(context.Context, string, string) ([]Evidence, error)
	ListReviewEvidence(context.Context, string) ([]Evidence, error)
	GetOwnEvidence(context.Context, string, string, string) (Evidence, error)
	GetReviewEvidence(context.Context, string, string) (Evidence, error)
	DeleteOwnEvidence(context.Context, string, string, string) error
	DeleteReviewEvidence(context.Context, string, string) error
}

type EvidenceStorage interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type EvidenceService struct {
	cases Repository
	repo  EvidenceRepository
	store EvidenceStorage
	ids   IDGenerator
	now   func() time.Time
}

func NewEvidenceService(cases Repository, repo EvidenceRepository, store EvidenceStorage, ids IDGenerator, now func() time.Time) (*EvidenceService, error) {
	if cases == nil || repo == nil || store == nil || ids == nil || now == nil {
		return nil, ErrInvalid
	}
	return &EvidenceService{cases: cases, repo: repo, store: store, ids: ids, now: now}, nil
}

// Upload creates a fixed synthetic PNG on the server. Raw user files are never
// accepted by this local prototype.
func (s *EvidenceService) Upload(ctx context.Context, owner, caseID string) (Evidence, error) {
	if owner == "" || caseID == "" {
		return Evidence{}, ErrInvalid
	}
	if _, err := s.cases.GetOwn(ctx, owner, caseID); err != nil {
		return Evidence{}, err
	}
	blob, err := syntheticEvidencePNG()
	if err != nil {
		return Evidence{}, err
	}
	id, err := s.ids.ID()
	if err != nil {
		return Evidence{}, err
	}
	checksum := sha256.Sum256(blob)
	item := Evidence{ID: id, VerificationID: caseID, FixtureCode: SyntheticEvidenceFixture, MIMEType: "image/png", SizeBytes: int64(len(blob)), SHA256: hex.EncodeToString(checksum[:]), CreatedAt: s.now().UTC()}
	if err := s.store.Put(ctx, id, blob); err != nil {
		return Evidence{}, err
	}
	created, err := s.repo.CreateEvidence(ctx, item)
	if err != nil {
		_ = s.store.Delete(context.Background(), id)
		return Evidence{}, err
	}
	return created, nil
}

func (s *EvidenceService) ListOwn(ctx context.Context, owner, caseID string) ([]Evidence, error) {
	if owner == "" || caseID == "" {
		return nil, ErrInvalid
	}
	if _, err := s.cases.GetOwn(ctx, owner, caseID); err != nil {
		return nil, err
	}
	return s.repo.ListOwnEvidence(ctx, owner, caseID)
}

func (s *EvidenceService) ListReview(ctx context.Context, caseID string) ([]Evidence, error) {
	if caseID == "" {
		return nil, ErrInvalid
	}
	return s.repo.ListReviewEvidence(ctx, caseID)
}

func (s *EvidenceService) OwnContent(ctx context.Context, owner, caseID, evidenceID string) (Evidence, []byte, error) {
	if owner == "" || caseID == "" || evidenceID == "" {
		return Evidence{}, nil, ErrInvalid
	}
	item, err := s.repo.GetOwnEvidence(ctx, owner, caseID, evidenceID)
	if err != nil {
		return Evidence{}, nil, err
	}
	blob, err := s.store.Get(ctx, item.ID)
	if err != nil {
		return Evidence{}, nil, err
	}
	return item, blob, nil
}

func (s *EvidenceService) ReviewContent(ctx context.Context, caseID, evidenceID string) (Evidence, []byte, error) {
	if caseID == "" || evidenceID == "" {
		return Evidence{}, nil, ErrInvalid
	}
	item, err := s.repo.GetReviewEvidence(ctx, caseID, evidenceID)
	if err != nil {
		return Evidence{}, nil, err
	}
	blob, err := s.store.Get(ctx, item.ID)
	if err != nil {
		return Evidence{}, nil, err
	}
	return item, blob, nil
}

func (s *EvidenceService) DeleteOwn(ctx context.Context, owner, caseID, evidenceID string) error {
	item, err := s.repo.GetOwnEvidence(ctx, owner, caseID, evidenceID)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, item.ID); err != nil {
		return err
	}
	return s.repo.DeleteOwnEvidence(ctx, owner, caseID, evidenceID)
}

func (s *EvidenceService) DeleteReview(ctx context.Context, caseID, evidenceID string) error {
	item, err := s.repo.GetReviewEvidence(ctx, caseID, evidenceID)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, item.ID); err != nil {
		return err
	}
	return s.repo.DeleteReviewEvidence(ctx, caseID, evidenceID)
}

func syntheticEvidencePNG() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 96, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 96; x++ {
			if ((x/8)+(y/8))%2 == 0 {
				img.Set(x, y, color.RGBA{R: 33, G: 102, B: 115, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 220, G: 232, B: 233, A: 255})
			}
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, errors.Join(ErrInvalid, err)
	}
	return out.Bytes(), nil
}

// SyntheticPNG returns the fixed local fixture used by verification and the
// M02 profile-photo prototype. Callers never provide image bytes.
func SyntheticPNG() ([]byte, error) { return syntheticEvidencePNG() }
