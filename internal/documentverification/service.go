// Package documentverification contains purpose-scoped document-case rules.
package documentverification

import (
	"context"
	"errors"
	"github.com/google/uuid"

	"time"
)

var (
	ErrInvalid            = errors.New("document verification: invalid input")
	ErrConflict           = errors.New("document verification: concurrent update")
	ErrFilesNotClean      = errors.New("document verification: files not clean")
	ErrQuota              = errors.New("document verification: file quota exceeded")
	ErrTransition         = errors.New("document verification: invalid transition")
	ErrCapability         = errors.New("document verification: reviewer capability required")
	ErrSelfReview         = errors.New("document verification: self review prohibited")
	ErrIdentityNotCurrent = errors.New("document verification: current identity verification required")
	ErrNotFound           = errors.New("document verification: case not found")
)

type Purpose string

const (
	PurposeIdentity Purpose = "identity"
	PurposeSpace    Purpose = "space"
)

type CaseState string

const (
	CaseDraft         CaseState = "borrador"
	CaseSubmitted     CaseState = "enviada"
	CaseInReview      CaseState = "en_revision"
	CaseNeedsEvidence CaseState = "requiere_antecedentes"
	CaseApproved      CaseState = "aprobada"
	CaseRejected      CaseState = "rechazada"
	CaseExpired       CaseState = "vencida"
	CaseRevoked       CaseState = "revocada"
)

type FileState string

const (
	FilePending FileState = "pendiente"
	FileClean   FileState = "limpio"
	FileBlocked FileState = "bloqueado"
	FileError   FileState = "error"
)

type Capability string

const (
	CapabilityIdentity Capability = "review_identity"
	CapabilitySpace    Capability = "review_space"
)

type Decision string

const (
	DecisionApprove       Decision = "approve"
	DecisionReject        Decision = "reject"
	DecisionNeedsEvidence Decision = "needs_evidence"
)

type ReasonCode string

const (
	ReasonDocumentUnreadable ReasonCode = "documento_ilegible"
	ReasonDocumentInvalid    ReasonCode = "documento_invalido"
	ReasonDocumentExpired    ReasonCode = "documento_vencido"
	ReasonEvidenceIncomplete ReasonCode = "antecedentes_incompletos"
	ReasonOwnershipChanged   ReasonCode = "cambio_titularidad"
	ReasonConflictOfInterest ReasonCode = "conflicto_interes"
	ReasonMalwareDetected    ReasonCode = "malware_detectado"
	ReasonScannerUnavailable ReasonCode = "scanner_no_disponible"
	ReasonSignaturesStale    ReasonCode = "firmas_desactualizadas"
	ReasonOwnerRequest       ReasonCode = "solicitud_titular"
	ReasonRetentionDue       ReasonCode = "retencion_vencida"
)

type TransitionAudit struct {
	Action     string
	ResultCode string
	ReasonCode ReasonCode
}

// Metadata must be produced by trusted content inspection, never copied from client headers.
type File struct {
	ID            string
	State         FileState
	Size          int64
	MIME          string
	Pages         int
	ContentSHA256 [32]byte
	RetentionAt   time.Time
}
type Snapshot struct {
	Purpose                                      Purpose
	ID, SubjectID, SpaceID                       string
	State                                        CaseState
	Revision                                     uint64
	Files                                        []File
	ReviewerID                                   string
	VerifiedAt, VerifiedUntil, DocumentExpiresAt time.Time
	RevokedAt                                    *time.Time
	SupersededAt                                 *time.Time
}
type CreateInput struct {
	Purpose           Purpose
	SpaceID           string
	DocumentExpiresAt time.Time
}
type ReviewerGrant struct {
	ReviewerID string
	Capability Capability
	// ScopeID is the identity subject ID for review_identity, or the exact
	// space ID for review_space. Each grant must be authoritatively assigned to ReviewerID.
	ScopeID string
}

type Authorization struct {
	ActorID         string
	ReviewerGrants  []ReviewerGrant
	IdentityCurrent bool
	SubjectID       string
	SpaceOwnerID    string
}

// CreateCase must atomically authorize actor, derive subject from authorization, and enforce authoritative space ownership.
type Repository interface {
	CreateCase(context.Context, string, Purpose, string, func(Authorization) (Snapshot, error)) (Snapshot, error)
	// Transition must lock the scoped case, bind the actor to authenticated context,
	// load that actor's current reviewer grants for the exact person/resource scope,
	// and derive identity eligibility from trusted data. For PurposeSpace, SpaceOwnerID must be
	// derived from authoritative current ownership data for every transition. Run change on an isolated snapshot, and
	// commit the changed snapshot with an advanced revision atomically. Callback/backend
	// errors must roll back all changes. A CAS on the case alone does not serialize identity
	// revocation; adapters must lock/revalidate that dependency in the same transaction.
	Transition(context.Context, string, Purpose, string, uint64, TransitionAudit, func(*Snapshot, Authorization) error) error
}

type OwnCaseReader interface {
	ListOwnCases(context.Context, string) ([]Snapshot, error)
	LoadOwnCase(context.Context, string, string) (Snapshot, error)
}

type Service struct {
	repo  Repository
	clock func() time.Time
}

func NewService(r Repository) (*Service, error) {
	if r == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: r, clock: func() time.Time { return time.Now().UTC() }}, nil
}
func (s *Service) SetClock(c func() time.Time) {
	if s != nil && c != nil {
		s.clock = c
	}
}
func ValidateID(id string) error {
	u, e := uuid.Parse(id)
	if e != nil || u == uuid.Nil || u.String() != id {
		return ErrInvalid
	}
	return nil
}
func ValidateQuota(p Purpose, fs []File) error {
	n := 0
	switch p {
	case PurposeIdentity:
		n = 3
	case PurposeSpace:
		n = 10
	default:
		return ErrInvalid
	}
	if len(fs) > n {
		return ErrQuota
	}
	seen := map[string]bool{}
	for _, f := range fs {
		if ValidateID(f.ID) != nil || seen[f.ID] {
			return ErrInvalid
		}
		if f.ContentSHA256 == ([32]byte{}) {
			return ErrInvalid
		}
		seen[f.ID] = true
		if f.State != FilePending && f.State != FileClean && f.State != FileBlocked && f.State != FileError {
			return ErrInvalid
		}
		if f.Size <= 0 || f.Size > 10*1024*1024 {
			return ErrQuota
		}
		switch f.MIME {
		case "image/jpeg", "image/png":
			if f.Pages != 0 && f.Pages != 1 {
				return ErrInvalid
			}
		case "application/pdf":
			limit := 5
			if p == PurposeSpace {
				limit = 20
			}
			if f.Pages < 1 || f.Pages > limit {
				return ErrQuota
			}
		default:
			return ErrInvalid
		}
	}
	return nil
}
func validSnapshot(x Snapshot) bool {
	if x.Purpose != PurposeIdentity && x.Purpose != PurposeSpace {
		return false
	}
	if ValidateID(x.ID) != nil || ValidateID(x.SubjectID) != nil {
		return false
	}
	if x.Purpose == PurposeIdentity && x.SpaceID != "" {
		return false
	}
	if x.Purpose == PurposeSpace && ValidateID(x.SpaceID) != nil {
		return false
	}
	if x.SupersededAt != nil && x.Purpose != PurposeSpace {
		return false
	}
	return ValidateQuota(x.Purpose, x.Files) == nil
}
func (s *Service) Start(ctx context.Context, actor string, in CreateInput) (Snapshot, error) {
	if s == nil || s.repo == nil || ValidateID(actor) != nil || in.Purpose != PurposeIdentity && in.Purpose != PurposeSpace {
		return Snapshot{}, ErrInvalid
	}
	if in.Purpose == PurposeIdentity && in.SpaceID != "" {
		return Snapshot{}, ErrInvalid
	}
	if in.Purpose == PurposeSpace && ValidateID(in.SpaceID) != nil {
		return Snapshot{}, ErrInvalid
	}
	now := s.clock().UTC()
	if in.Purpose == PurposeIdentity && (in.DocumentExpiresAt.IsZero() || !in.DocumentExpiresAt.After(now)) ||
		!in.DocumentExpiresAt.IsZero() && !in.DocumentExpiresAt.After(now) {
		return Snapshot{}, ErrInvalid
	}
	return s.repo.CreateCase(ctx, actor, in.Purpose, in.SpaceID, func(a Authorization) (Snapshot, error) {
		if a.ActorID != actor || a.SubjectID != actor || ValidateID(a.SubjectID) != nil {
			return Snapshot{}, ErrCapability
		}
		if in.Purpose == PurposeSpace && (a.SpaceOwnerID != actor) {
			return Snapshot{}, ErrCapability
		}
		x := Snapshot{Purpose: in.Purpose, ID: uuid.NewString(), SubjectID: a.SubjectID, SpaceID: in.SpaceID, State: CaseDraft, DocumentExpiresAt: in.DocumentExpiresAt}
		if !validSnapshot(x) {
			return Snapshot{}, ErrInvalid
		}
		return x, nil
	})
}

func (s *Service) Create(ctx context.Context, actor string, in CreateInput) error {
	_, err := s.Start(ctx, actor, in)
	return err
}

func (s *Service) ListOwnCases(ctx context.Context, actor string) ([]Snapshot, error) {
	if s == nil || s.repo == nil || ValidateID(actor) != nil {
		return nil, ErrInvalid
	}
	reader, ok := s.repo.(OwnCaseReader)
	if !ok {
		return nil, ErrInvalid
	}
	return reader.ListOwnCases(ctx, actor)
}

func (s *Service) LoadOwnCase(ctx context.Context, actor, id string) (Snapshot, error) {
	if s == nil || s.repo == nil || ValidateID(actor) != nil || ValidateID(id) != nil {
		return Snapshot{}, ErrInvalid
	}
	reader, ok := s.repo.(OwnCaseReader)
	if !ok {
		return Snapshot{}, ErrInvalid
	}
	return reader.LoadOwnCase(ctx, actor, id)
}

func (s *Service) AddFile(ctx context.Context, actor, id string, p Purpose, rev uint64, file File) error {
	if file.State != FilePending || ValidateQuota(p, []File{file}) != nil {
		return ErrInvalid
	}
	return s.transact(ctx, p, id, rev, actor, "", TransitionAudit{Action: "file_added", ResultCode: string(FilePending)}, func(x *Snapshot, a Authorization) error {
		if x.SubjectID != actor || p == PurposeSpace && x.SubjectID != a.SpaceOwnerID {
			return ErrCapability
		}
		if x.State != CaseDraft && x.State != CaseNeedsEvidence {
			return ErrTransition
		}
		x.Files = append(x.Files, file)
		return ValidateQuota(p, x.Files)
	})
}

func allClean(fs []File) bool {
	if len(fs) == 0 {
		return false
	}
	for _, f := range fs {
		if f.State != FileClean {
			return false
		}
	}
	return true
}
func validFiles(x *Snapshot) error {
	if e := ValidateQuota(x.Purpose, x.Files); e != nil {
		return e
	}
	if !allClean(x.Files) {
		return ErrFilesNotClean
	}
	return nil
}
func (s *Service) Submit(ctx context.Context, actor, id string, p Purpose, rev uint64) error {
	return s.transact(ctx, p, id, rev, actor, "", TransitionAudit{Action: "case_transitioned", ResultCode: string(CaseSubmitted)}, func(x *Snapshot, a Authorization) error {
		if x.SubjectID != actor {
			return ErrCapability
		}
		if p == PurposeSpace && x.SubjectID != a.SpaceOwnerID {
			return ErrConflict
		}
		if x.State != CaseDraft && x.State != CaseNeedsEvidence {
			return ErrTransition
		}
		if err := validFiles(x); err != nil {
			return err
		}
		x.State = CaseSubmitted
		x.ReviewerID = ""
		return nil
	})
}
func (s *Service) Review(ctx context.Context, actor, id string, cap Capability, d Decision, requestedExpiry time.Time, rev uint64) error {
	return s.ReviewWithReason(ctx, actor, id, cap, d, "", requestedExpiry, rev)
}

func (s *Service) ReviewWithReason(ctx context.Context, actor, id string, cap Capability, d Decision, reason ReasonCode, requestedExpiry time.Time, rev uint64) error {
	p := purposeForCap(cap)
	if p == "" || ValidateID(actor) != nil || !validReviewReason(d, reason) {
		return ErrInvalid
	}
	resultCode := map[Decision]string{DecisionApprove: string(CaseApproved), DecisionReject: string(CaseRejected), DecisionNeedsEvidence: string(CaseNeedsEvidence)}[d]
	audit := TransitionAudit{Action: "review_decision", ResultCode: resultCode, ReasonCode: reason}
	return s.transact(ctx, p, id, rev, actor, cap, audit, func(x *Snapshot, a Authorization) error {
		if a.ActorID != actor || !hasReviewerGrant(a, cap, reviewerScopeID(p, *x)) {
			return ErrCapability
		}
		if p == PurposeSpace && a.SpaceOwnerID == actor {
			return ErrSelfReview
		}
		if p == PurposeSpace && x.SubjectID != a.SpaceOwnerID {
			return ErrConflict
		}
		if x.SubjectID == actor {
			return ErrSelfReview
		}
		if x.State != CaseSubmitted && x.State != CaseInReview && x.State != CaseNeedsEvidence {
			return ErrTransition
		}
		if e := validFiles(x); e != nil {
			return e
		}
		if d != DecisionApprove && d != DecisionReject && d != DecisionNeedsEvidence {
			return ErrInvalid
		}
		x.ReviewerID = actor
		switch d {
		case DecisionReject:
			x.State = CaseRejected
		case DecisionNeedsEvidence:
			x.State = CaseNeedsEvidence
		case DecisionApprove:
			now := s.clock().UTC()
			months := 24
			if x.Purpose == PurposeSpace {
				months = 12
				if !a.IdentityCurrent {
					return ErrIdentityNotCurrent
				}
			}
			deadline := now.AddDate(0, months, 0)
			if !requestedExpiry.IsZero() && requestedExpiry.Before(deadline) {
				deadline = requestedExpiry
			}
			if x.Purpose == PurposeIdentity && (x.DocumentExpiresAt.IsZero() || !x.DocumentExpiresAt.After(now)) ||
				!x.DocumentExpiresAt.IsZero() && !x.DocumentExpiresAt.After(now) {
				return ErrInvalid
			}
			if !x.DocumentExpiresAt.IsZero() && x.DocumentExpiresAt.Before(deadline) {
				deadline = x.DocumentExpiresAt
			}
			if !deadline.After(now) {
				return ErrInvalid
			}
			x.State = CaseApproved
			x.VerifiedAt = now
			x.VerifiedUntil = deadline
		}
		return nil
	})
}
func (s *Service) Revoke(ctx context.Context, actor, id string, p Purpose, reason ReasonCode, rev uint64) error {
	if !validRevokeReason(reason) || ValidateID(actor) != nil || (p != PurposeIdentity && p != PurposeSpace) {
		return ErrInvalid
	}
	cap := CapabilityIdentity
	if p == PurposeSpace {
		cap = CapabilitySpace
	}
	audit := TransitionAudit{Action: "revoked", ResultCode: string(CaseRevoked), ReasonCode: reason}
	return s.transact(ctx, p, id, rev, actor, cap, audit, func(x *Snapshot, a Authorization) error {
		if a.ActorID != actor || !hasReviewerGrant(a, cap, reviewerScopeID(p, *x)) {
			return ErrCapability
		}
		if p == PurposeSpace && a.SpaceOwnerID == actor {
			return ErrSelfReview
		}
		if p == PurposeSpace && x.SubjectID != a.SpaceOwnerID {
			return ErrConflict
		}
		if x.SubjectID == actor {
			return ErrSelfReview
		}
		if x.State != CaseApproved {
			return ErrTransition
		}
		t := s.clock().UTC()
		x.State = CaseRevoked
		x.RevokedAt = &t
		x.ReviewerID = actor
		return nil
	})
}

func validReviewReason(decision Decision, reason ReasonCode) bool {
	switch decision {
	case DecisionApprove:
		return reason == ""
	case DecisionReject:
		return reason == ReasonDocumentUnreadable || reason == ReasonDocumentInvalid || reason == ReasonDocumentExpired || reason == ReasonOwnershipChanged
	case DecisionNeedsEvidence:
		return reason == ReasonEvidenceIncomplete
	default:
		return false
	}
}

func validRevokeReason(reason ReasonCode) bool {
	return reason == ReasonDocumentInvalid || reason == ReasonDocumentExpired || reason == ReasonOwnershipChanged || reason == ReasonConflictOfInterest || reason == ReasonOwnerRequest
}

func purposeForCap(c Capability) Purpose {
	if c == CapabilityIdentity {
		return PurposeIdentity
	}
	if c == CapabilitySpace {
		return PurposeSpace
	}
	return ""
}
func hasReviewerGrant(a Authorization, capability Capability, scopeID string) bool {
	if a.ActorID == "" || (capability != CapabilityIdentity && capability != CapabilitySpace) || ValidateID(scopeID) != nil {
		return false
	}
	capabilities := make(map[Capability]bool, 2)
	for _, grant := range a.ReviewerGrants {
		if ValidateID(grant.ReviewerID) != nil || grant.ReviewerID != a.ActorID ||
			(grant.Capability != CapabilityIdentity && grant.Capability != CapabilitySpace) || ValidateID(grant.ScopeID) != nil {
			return false
		}
		capabilities[grant.Capability] = true
	}
	// Dual-purpose reviewer assignments remain denied until explicitly approved.
	if len(capabilities) != 1 || !capabilities[capability] {
		return false
	}
	for _, grant := range a.ReviewerGrants {
		if grant.Capability == capability && grant.ScopeID == scopeID {
			return true
		}
	}
	return false
}

func reviewerScopeID(p Purpose, x Snapshot) string {
	if p == PurposeIdentity {
		return x.SubjectID
	}
	if p == PurposeSpace {
		return x.SpaceID
	}
	return ""
}

func (s *Service) transact(ctx context.Context, p Purpose, id string, rev uint64, actor string, cap Capability, audit TransitionAudit, change func(*Snapshot, Authorization) error) error {
	if s == nil || s.repo == nil || ValidateID(id) != nil || ValidateID(actor) != nil || (p != PurposeIdentity && p != PurposeSpace) {
		return ErrInvalid
	}
	return s.repo.Transition(ctx, actor, p, id, rev, audit, func(x *Snapshot, a Authorization) error {
		if x == nil || x.Purpose != p || x.ID != id || !validSnapshot(*x) {
			return ErrNotFound
		}
		if x.SupersededAt != nil {
			return ErrConflict
		}
		if x.Revision != rev {
			return ErrConflict
		}
		if a.ActorID != actor {
			return ErrCapability
		}
		if cap != "" && purposeForCap(cap) != p {
			return ErrCapability
		}
		return change(x, a)
	})
}
