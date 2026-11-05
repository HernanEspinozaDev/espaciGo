package documentverification

import (
	"context"
	"crypto/sha256"
	"time"
)

// ReviewerCaseReader exposes only cases assigned to the authenticated reviewer.
type ReviewerCaseReader interface {
	ListReviewerCases(context.Context, string) ([]Snapshot, error)
	LoadReviewerCase(context.Context, string, string) (Snapshot, error)
	WithReviewerCaseForRead(context.Context, string, string, func(context.Context, Snapshot, Authorization) error) error
}

type IdentityDocumentStore interface {
	ReadIdentityDocument(context.Context, string, string, string) ([]byte, error)
}
type ReviewerDocumentStore interface {
	SpaceDocumentStore
	IdentityDocumentStore
}

type ReviewerAccessService struct {
	repository ReviewerCaseReader
	cases      *Service
	store      ReviewerDocumentStore
	clock      func() time.Time
}

func NewReviewerAccessService(repository ReviewerCaseReader, cases *Service, store ReviewerDocumentStore) (*ReviewerAccessService, error) {
	if repository == nil || cases == nil || store == nil {
		return nil, ErrInvalid
	}
	return &ReviewerAccessService{repository: repository, cases: cases, store: store, clock: func() time.Time { return time.Now().UTC() }}, nil
}
func (s *ReviewerAccessService) SetClock(clock func() time.Time) {
	if s != nil && clock != nil {
		s.clock = clock
	}
}
func (s *ReviewerAccessService) List(ctx context.Context, actor string) ([]Snapshot, error) {
	if s == nil || ValidateID(actor) != nil {
		return nil, ErrInvalid
	}
	return s.repository.ListReviewerCases(ctx, actor)
}
func (s *ReviewerAccessService) Load(ctx context.Context, actor, id string) (Snapshot, error) {
	if s == nil || ValidateID(actor) != nil || ValidateID(id) != nil {
		return Snapshot{}, ErrInvalid
	}
	return s.repository.LoadReviewerCase(ctx, actor, id)
}
func (s *ReviewerAccessService) Review(ctx context.Context, actor, id string, capability Capability, decision Decision, reason ReasonCode, verifiedUntil time.Time, revision uint64) error {
	if s == nil || ValidateID(id) != nil {
		return ErrInvalid
	}
	return s.cases.ReviewWithReason(ctx, actor, id, capability, decision, reason, verifiedUntil, revision)
}
func (s *ReviewerAccessService) Revoke(ctx context.Context, actor, id string, purpose Purpose, reason ReasonCode, revision uint64) error {
	if s == nil || ValidateID(id) != nil {
		return ErrInvalid
	}
	return s.cases.Revoke(ctx, actor, id, purpose, reason, revision)
}
func (s *ReviewerAccessService) Download(ctx context.Context, actor, caseID, fileID string) (DownloadedDocument, error) {
	if s == nil || ValidateID(actor) != nil || ValidateID(caseID) != nil || ValidateID(fileID) != nil {
		return DownloadedDocument{}, ErrInvalid
	}
	var out DownloadedDocument
	err := s.repository.WithReviewerCaseForRead(ctx, actor, caseID, func(readCtx context.Context, x Snapshot, a Authorization) error {
		if x.ID != caseID || x.SupersededAt != nil || a.ActorID != actor || x.SubjectID == actor {
			return ErrNotFound
		}
		cap := CapabilityIdentity
		scope := x.SubjectID
		if x.Purpose == PurposeSpace {
			cap = CapabilitySpace
			scope = x.SpaceID
			if a.SpaceOwnerID == actor || a.SpaceOwnerID != x.SubjectID {
				return ErrNotFound
			}
		}
		if !hasReviewerGrant(a, cap, scope) {
			return ErrNotFound
		}
		if x.State != CaseSubmitted && x.State != CaseInReview && x.State != CaseNeedsEvidence {
			return ErrNotFound
		}
		for _, f := range x.Files {
			if f.ID == fileID {
				if f.State != FileClean || (!f.RetentionAt.IsZero() && !f.RetentionAt.After(s.clock())) {
					return ErrNotFound
				}
				var b []byte
				var err error
				if x.Purpose == PurposeIdentity {
					b, err = s.store.ReadIdentityDocument(readCtx, x.SubjectID, x.ID, f.ID)
				} else {
					b, err = s.store.ReadSpaceDocument(readCtx, x.SubjectID, x.SpaceID, x.ID, f.ID)
				}
				if err != nil || int64(len(b)) != f.Size || sha256.Sum256(b) != f.ContentSHA256 {
					return ErrDocumentStorage
				}
				out = DownloadedDocument{Content: b, MIME: f.MIME}
				return nil
			}
		}
		return ErrNotFound
	})
	if err != nil {
		return DownloadedDocument{}, err
	}
	return out, nil
}
