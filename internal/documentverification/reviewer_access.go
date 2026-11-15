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

type ReviewerIdentityDeclaration struct {
	FirstName   string
	LastName    string
	RUT         string
	DateOfBirth string
}

type ReviewerIdentityDeclarationReader interface {
	LoadReviewerIdentityDeclaration(context.Context, string, string) (ReviewerIdentityDeclaration, error)
}

type ReviewerIdentityDeclarationReaderFunc func(context.Context, string, string) (ReviewerIdentityDeclaration, error)

func (f ReviewerIdentityDeclarationReaderFunc) LoadReviewerIdentityDeclaration(ctx context.Context, accountID, revisionID string) (ReviewerIdentityDeclaration, error) {
	return f(ctx, accountID, revisionID)
}

type ReviewerCaseDetail struct {
	Snapshot            Snapshot
	IdentityDeclaration *ReviewerIdentityDeclaration
}

type ReviewerDocumentStore interface {
	SpaceDocumentStore
	IdentityDocumentStore
}

type ReviewerAccessService struct {
	repository   ReviewerCaseReader
	cases        *Service
	store        ReviewerDocumentStore
	declarations ReviewerIdentityDeclarationReader
	clock        func() time.Time
}

func NewReviewerAccessService(repository ReviewerCaseReader, cases *Service, store ReviewerDocumentStore) (*ReviewerAccessService, error) {
	if repository == nil || cases == nil || store == nil {
		return nil, ErrInvalid
	}
	return &ReviewerAccessService{repository: repository, cases: cases, store: store, clock: func() time.Time { return time.Now().UTC() }}, nil
}

func NewReviewerAccessServiceWithIdentityDeclarationReader(repository ReviewerCaseReader, cases *Service, store ReviewerDocumentStore, declarations ReviewerIdentityDeclarationReader) (*ReviewerAccessService, error) {
	if declarations == nil {
		return nil, ErrInvalid
	}
	service, err := NewReviewerAccessService(repository, cases, store)
	if err != nil {
		return nil, err
	}
	service.declarations = declarations
	return service, nil
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
	if ctx.Err() != nil {
		return nil, ErrDocumentStorage
	}
	assigned, err := s.repository.ListReviewerCases(ctx, actor)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ErrDocumentStorage
	}
	queue := make([]Snapshot, 0, len(assigned))
	for _, x := range assigned {
		if reviewerQueueCaseEligible(actor, x) {
			queue = append(queue, x)
		}
	}
	return queue, nil
}

// reviewerQueueCaseEligible filters malformed or non-actionable results from
// the already-assignment-filtered ReviewerCaseReader contract.
func reviewerQueueCaseEligible(actor string, x Snapshot) bool {
	if ValidateID(x.ID) != nil || ValidateID(x.SubjectID) != nil || x.SubjectID == actor ||
		x.SupersededAt != nil || x.RevokedAt != nil {
		return false
	}
	switch x.Purpose {
	case PurposeIdentity:
	case PurposeSpace:
		if ValidateID(x.SpaceID) != nil {
			return false
		}
	default:
		return false
	}
	switch x.State {
	case CaseSubmitted, CaseInReview, CaseNeedsEvidence:
		return true
	default:
		return false
	}
}

func (s *ReviewerAccessService) Load(ctx context.Context, actor, id string) (Snapshot, error) {
	if s == nil || ValidateID(actor) != nil || ValidateID(id) != nil {
		return Snapshot{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return Snapshot{}, ErrDocumentStorage
	}
	snapshot, err := s.repository.LoadReviewerCase(ctx, actor, id)
	if err != nil {
		return Snapshot{}, err
	}
	if ctx.Err() != nil {
		return Snapshot{}, ErrDocumentStorage
	}
	return snapshot, nil
}

func (s *ReviewerAccessService) LoadDetail(ctx context.Context, actor, id string) (ReviewerCaseDetail, error) {
	if s == nil || ValidateID(actor) != nil || ValidateID(id) != nil {
		return ReviewerCaseDetail{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return ReviewerCaseDetail{}, ErrDocumentStorage
	}
	var first Snapshot
	err := s.repository.WithReviewerCaseForRead(ctx, actor, id, func(_ context.Context, current Snapshot, authorization Authorization) error {
		if !reviewerCanReadCase(actor, id, current, authorization) {
			return ErrNotFound
		}
		first = current
		return nil
	})
	if err != nil {
		return ReviewerCaseDetail{}, err
	}
	if ctx.Err() != nil {
		return ReviewerCaseDetail{}, ErrDocumentStorage
	}
	detail := ReviewerCaseDetail{Snapshot: first}
	if first.Purpose != PurposeIdentity || first.DocumentFormatDeclared == "" ||
		(first.State != CaseSubmitted && first.State != CaseInReview && first.State != CaseNeedsEvidence) {
		return detail, nil
	}
	if s.declarations == nil || ValidateID(first.IdentityDeclarationRevisionID) != nil {
		return ReviewerCaseDetail{}, ErrDocumentStorage
	}
	var confirmed ReviewerCaseDetail
	err = s.repository.WithReviewerCaseForRead(ctx, actor, id, func(readCtx context.Context, current Snapshot, authorization Authorization) error {
		if !reviewerCanReadCase(actor, id, current, authorization) || !sameReviewerCase(first, current) {
			return ErrNotFound
		}
		declaration, err := s.declarations.LoadReviewerIdentityDeclaration(readCtx, current.SubjectID, current.IdentityDeclarationRevisionID)
		if err != nil || readCtx.Err() != nil || declaration.FirstName == "" || declaration.LastName == "" || declaration.RUT == "" || declaration.DateOfBirth == "" {
			return ErrDocumentStorage
		}
		copy := declaration
		confirmed = ReviewerCaseDetail{Snapshot: current, IdentityDeclaration: &copy}
		return nil
	})
	if err != nil {
		return ReviewerCaseDetail{}, err
	}
	if ctx.Err() != nil {
		return ReviewerCaseDetail{}, ErrDocumentStorage
	}
	var latest Snapshot
	err = s.repository.WithReviewerCaseForRead(ctx, actor, id, func(_ context.Context, current Snapshot, authorization Authorization) error {
		if !reviewerCanReadCase(actor, id, current, authorization) || !sameReviewerCase(first, current) {
			return ErrNotFound
		}
		latest = current
		return nil
	})
	if err != nil {
		return ReviewerCaseDetail{}, err
	}
	if ctx.Err() != nil {
		return ReviewerCaseDetail{}, ErrDocumentStorage
	}
	confirmed.Snapshot = latest
	return confirmed, nil
}

func reviewerCanReadCase(actor, id string, x Snapshot, authorization Authorization) bool {
	if authorization.ActorID != actor || x.ID != id || x.SubjectID == actor || x.SupersededAt != nil {
		return false
	}
	capability, scope := CapabilityIdentity, x.SubjectID
	if x.Purpose == PurposeSpace {
		capability, scope = CapabilitySpace, x.SpaceID
	} else if x.Purpose != PurposeIdentity {
		return false
	}
	return hasReviewerGrant(authorization, capability, scope)
}

func sameReviewerCase(first, current Snapshot) bool {
	return first.ID == current.ID && first.Purpose == current.Purpose && first.SubjectID == current.SubjectID &&
		first.SpaceID == current.SpaceID && first.State == current.State && first.Revision == current.Revision &&
		first.IdentityDeclarationRevisionID == current.IdentityDeclarationRevisionID &&
		first.DocumentFormatDeclared == current.DocumentFormatDeclared &&
		first.CaptureConsentVersion == current.CaptureConsentVersion &&
		first.CaptureConsentedAt.Equal(current.CaptureConsentedAt) &&
		first.DocumentFormatObserved == current.DocumentFormatObserved &&
		first.DocumentExpiresAt.Equal(current.DocumentExpiresAt) && first.SupersededAt == nil && current.SupersededAt == nil
}

func (s *ReviewerAccessService) Review(ctx context.Context, actor, id string, capability Capability, decision Decision, reason ReasonCode, verifiedUntil time.Time, revision uint64) error {
	if s == nil || ValidateID(id) != nil {
		return ErrInvalid
	}
	return s.cases.ReviewWithReason(ctx, actor, id, capability, decision, reason, verifiedUntil, revision)
}
func (s *ReviewerAccessService) ReviewIdentity(ctx context.Context, actor, id string, decision Decision, reason ReasonCode, observedFormat DocumentFormat, observedExpiry time.Time, checklist *IdentityReviewChecklist, verifiedUntil time.Time, revision uint64) error {
	if s == nil || ValidateID(id) != nil {
		return ErrInvalid
	}
	return s.cases.ReviewIdentityWithReason(ctx, actor, id, decision, reason, observedFormat, observedExpiry, checklist, verifiedUntil, revision)
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
	if ctx.Err() != nil {
		return DownloadedDocument{}, ErrDocumentStorage
	}
	var out DownloadedDocument
	err := s.repository.WithReviewerCaseForRead(ctx, actor, caseID, func(readCtx context.Context, x Snapshot, a Authorization) error {
		if x.ID != caseID || x.SupersededAt != nil || a.ActorID != actor || x.SubjectID == actor ||
			(x.Purpose != PurposeIdentity && x.Purpose != PurposeSpace) {
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
				if readCtx.Err() != nil {
					return ErrDocumentStorage
				}
				var b []byte
				var err error
				if x.Purpose == PurposeIdentity {
					b, err = s.store.ReadIdentityDocument(readCtx, x.SubjectID, x.ID, f.ID)
				} else {
					b, err = s.store.ReadSpaceDocument(readCtx, x.SubjectID, x.SpaceID, x.ID, f.ID)
				}
				if err != nil || readCtx.Err() != nil || int64(len(b)) != f.Size || sha256.Sum256(b) != f.ContentSHA256 {
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
	if ctx.Err() != nil {
		return DownloadedDocument{}, ErrDocumentStorage
	}
	return out, nil
}
