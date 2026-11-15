// Package documentverification contains purpose-scoped document-case rules.
package documentverification

import (
	"context"
	"errors"
	"github.com/google/uuid"

	"time"
)

var (
	ErrInvalid             = errors.New("document verification: invalid input")
	ErrConflict            = errors.New("document verification: concurrent update")
	ErrFilesNotClean       = errors.New("document verification: files not clean")
	ErrEvidenceIncomplete  = errors.New("document verification: identity evidence incomplete")
	ErrQuota               = errors.New("document verification: file quota exceeded")
	ErrTransition          = errors.New("document verification: invalid transition")
	ErrCapability          = errors.New("document verification: reviewer capability required")
	ErrSelfReview          = errors.New("document verification: self review prohibited")
	ErrIdentityNotCurrent  = errors.New("document verification: current identity verification required")
	ErrDeclarationRequired = errors.New("document verification: current identity declaration required")
	ErrNotFound            = errors.New("document verification: case not found")
)

type Purpose string

const (
	PurposeIdentity Purpose = "identity"
	PurposeSpace    Purpose = "space"
)

// IdentityCaptureConsentVersion is only the local synthetic-test notice version.
// It is not approval for collecting real personal data.
const IdentityCaptureConsentVersion = "identity-capture-local-v1"

type DocumentFormat string

const (
	DocumentFormatLegacy       DocumentFormat = "legacy"
	DocumentFormatNew          DocumentFormat = "new"
	DocumentFormatUnsure       DocumentFormat = "unsure"
	DocumentFormatUndetermined DocumentFormat = "undetermined"
)

type EvidenceRole string

const (
	EvidenceRoleIdentityFront  EvidenceRole = "identity_front"
	EvidenceRoleIdentityBack   EvidenceRole = "identity_back"
	EvidenceRoleIdentitySelfie EvidenceRole = "identity_selfie"
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
	Action                    string
	ResultCode                string
	ReasonCode                ReasonCode
	DocumentFormatObserved    DocumentFormat
	DocumentExpiresAtObserved time.Time
	IdentityReviewChecklist   *IdentityReviewChecklist
}

// IdentityReviewChecklist records explicit reviewer observations. Pointer
// fields distinguish an unchecked criterion from an explicit negative result.
type IdentityReviewChecklist struct {
	NamesMatch                 *bool `json:"names_match"`
	RUTMatches                 *bool `json:"rut_matches"`
	BirthDateAndMajorityMatch  *bool `json:"birth_date_and_majority_match"`
	PortraitMatchesSelfie      *bool `json:"portrait_matches_selfie"`
	EvidenceLegible            *bool `json:"evidence_legible"`
	DocumentUnexpired          *bool `json:"document_unexpired"`
	IssueDateAndFormatVerified *bool `json:"issue_date_and_format_verified"`
}

func (c *IdentityReviewChecklist) Complete() bool {
	return c != nil && c.NamesMatch != nil && c.RUTMatches != nil &&
		c.BirthDateAndMajorityMatch != nil && c.PortraitMatchesSelfie != nil &&
		c.EvidenceLegible != nil && c.DocumentUnexpired != nil &&
		c.IssueDateAndFormatVerified != nil
}

func (c *IdentityReviewChecklist) AllPassed() bool {
	return c.Complete() && *c.NamesMatch && *c.RUTMatches &&
		*c.BirthDateAndMajorityMatch && *c.PortraitMatchesSelfie &&
		*c.EvidenceLegible && *c.DocumentUnexpired && *c.IssueDateAndFormatVerified
}

// Metadata must be produced by trusted content inspection, never copied from client headers.
type File struct {
	ID            string
	EvidenceRole  EvidenceRole
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
	OwnerDecisionReason                          ReasonCode `json:"-"`
	Files                                        []File
	ReviewerID                                   string
	IdentityDeclarationRevisionID                string
	CaptureConsentVersion                        string
	CaptureConsentedAt                           time.Time
	DocumentFormatDeclared                       DocumentFormat
	DocumentFormatObserved                       DocumentFormat
	VerifiedAt, VerifiedUntil, DocumentExpiresAt time.Time
	RevokedAt                                    *time.Time
	SupersededAt                                 *time.Time
}
type CreateInput struct {
	Purpose                Purpose
	SpaceID                string
	DocumentExpiresAt      time.Time
	DocumentFormatDeclared DocumentFormat
	CaptureConsentGiven    bool
	CaptureConsentVersion  string
}
type ReviewerGrant struct {
	ReviewerID string
	Capability Capability
	// ScopeID is the identity subject ID for review_identity, or the exact
	// space ID for review_space. Each grant must be authoritatively assigned to ReviewerID.
	ScopeID string
}

type Authorization struct {
	ActorID                       string
	ReviewerGrants                []ReviewerGrant
	IdentityCurrent               bool
	SubjectID                     string
	SpaceOwnerID                  string
	IdentityDeclarationRevisionID string
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
func validDeclaredDocumentFormat(format DocumentFormat) bool {
	return format == DocumentFormatLegacy || format == DocumentFormatNew || format == DocumentFormatUnsure
}

func validObservedDocumentFormat(format DocumentFormat) bool {
	return format == DocumentFormatLegacy || format == DocumentFormatNew || format == DocumentFormatUndetermined
}

func validEvidenceRole(role EvidenceRole) bool {
	return role == EvidenceRoleIdentityFront || role == EvidenceRoleIdentityBack || role == EvidenceRoleIdentitySelfie
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
	if x.Purpose == PurposeIdentity {
		legacy := x.IdentityDeclarationRevisionID == "" && x.DocumentFormatDeclared == "" && x.DocumentFormatObserved == "" &&
			x.CaptureConsentVersion == "" && x.CaptureConsentedAt.IsZero()
		if !legacy {
			if !validDeclaredDocumentFormat(x.DocumentFormatDeclared) ||
				(x.DocumentFormatObserved != "" && !validObservedDocumentFormat(x.DocumentFormatObserved)) {
				return false
			}
			consentVersionSet := x.CaptureConsentVersion != ""
			consentTimeSet := !x.CaptureConsentedAt.IsZero()
			if consentVersionSet != consentTimeSet || consentVersionSet && x.CaptureConsentVersion != IdentityCaptureConsentVersion {
				return false
			}
			if x.IdentityDeclarationRevisionID == "" {
				if x.State != CaseRejected && x.State != CaseExpired && x.State != CaseRevoked {
					return false
				}
			} else if ValidateID(x.IdentityDeclarationRevisionID) != nil ||
				x.State == CaseRejected || x.State == CaseExpired || x.State == CaseRevoked {
				return false
			}
		}
	} else if x.IdentityDeclarationRevisionID != "" || x.DocumentFormatDeclared != "" || x.DocumentFormatObserved != "" {
		return false
	} else if x.CaptureConsentVersion != "" || !x.CaptureConsentedAt.IsZero() {
		return false
	}
	if err := ValidateQuota(x.Purpose, x.Files); err != nil {
		return false
	}
	seenRoles := map[EvidenceRole]bool{}
	for _, file := range x.Files {
		if file.EvidenceRole == "" {
			if x.DocumentFormatDeclared != "" {
				return false
			}
			continue
		}
		if x.Purpose != PurposeIdentity || !validEvidenceRole(file.EvidenceRole) || seenRoles[file.EvidenceRole] || file.MIME == "application/pdf" {
			return false
		}
		seenRoles[file.EvidenceRole] = true
	}
	return true
}
func (s *Service) Start(ctx context.Context, actor string, in CreateInput) (Snapshot, error) {
	if s == nil || s.repo == nil || ctx == nil || ValidateID(actor) != nil || in.Purpose != PurposeIdentity && in.Purpose != PurposeSpace {
		return Snapshot{}, ErrInvalid
	}
	if in.Purpose == PurposeIdentity {
		if in.SpaceID != "" || !validDeclaredDocumentFormat(in.DocumentFormatDeclared) || !in.DocumentExpiresAt.IsZero() ||
			!in.CaptureConsentGiven || in.CaptureConsentVersion != IdentityCaptureConsentVersion {
			return Snapshot{}, ErrInvalid
		}
	} else {
		if ValidateID(in.SpaceID) != nil || in.DocumentFormatDeclared != "" ||
			!in.DocumentExpiresAt.IsZero() && !in.DocumentExpiresAt.After(s.clock().UTC()) ||
			in.CaptureConsentGiven || in.CaptureConsentVersion != "" {
			return Snapshot{}, ErrInvalid
		}
	}
	if ctx.Err() != nil {
		return Snapshot{}, ctx.Err()
	}
	return s.repo.CreateCase(ctx, actor, in.Purpose, in.SpaceID, func(a Authorization) (Snapshot, error) {
		if ctx.Err() != nil {
			return Snapshot{}, ctx.Err()
		}
		if a.ActorID != actor || a.SubjectID != actor || ValidateID(a.SubjectID) != nil {
			return Snapshot{}, ErrCapability
		}
		if in.Purpose == PurposeSpace && a.SpaceOwnerID != actor {
			return Snapshot{}, ErrCapability
		}
		x := Snapshot{Purpose: in.Purpose, ID: uuid.NewString(), SubjectID: a.SubjectID, SpaceID: in.SpaceID, State: CaseDraft, DocumentExpiresAt: in.DocumentExpiresAt}
		if in.Purpose == PurposeIdentity {
			if ValidateID(a.IdentityDeclarationRevisionID) != nil {
				return Snapshot{}, ErrDeclarationRequired
			}
			x.IdentityDeclarationRevisionID = a.IdentityDeclarationRevisionID
			x.CaptureConsentVersion = IdentityCaptureConsentVersion
			x.CaptureConsentedAt = s.clock().UTC()
			x.DocumentFormatDeclared = in.DocumentFormatDeclared
			x.DocumentExpiresAt = time.Time{}
		}
		if !validSnapshot(x) {
			return Snapshot{}, ErrInvalid
		}
		if ctx.Err() != nil {
			return Snapshot{}, ctx.Err()
		}
		return x, nil
	})
}

func (s *Service) Create(ctx context.Context, actor string, in CreateInput) error {
	_, err := s.Start(ctx, actor, in)
	return err
}

func (s *Service) ListOwnCases(ctx context.Context, actor string) ([]Snapshot, error) {
	if s == nil || s.repo == nil || ctx == nil || ValidateID(actor) != nil {
		return nil, ErrInvalid
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	reader, ok := s.repo.(OwnCaseReader)
	if !ok {
		return nil, ErrInvalid
	}
	cases, err := reader.ListOwnCases(ctx, actor)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return cases, nil
}

func (s *Service) LoadOwnCase(ctx context.Context, actor, id string) (Snapshot, error) {
	if s == nil || s.repo == nil || ctx == nil || ValidateID(actor) != nil || ValidateID(id) != nil {
		return Snapshot{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return Snapshot{}, ctx.Err()
	}
	reader, ok := s.repo.(OwnCaseReader)
	if !ok {
		return Snapshot{}, ErrInvalid
	}
	item, err := reader.LoadOwnCase(ctx, actor, id)
	if err != nil {
		return Snapshot{}, err
	}
	if ctx.Err() != nil {
		return Snapshot{}, ctx.Err()
	}
	return item, nil
}

func (s *Service) AddFile(ctx context.Context, actor, id string, p Purpose, rev uint64, file File) error {
	if file.State != FilePending || ValidateQuota(p, []File{file}) != nil ||
		file.EvidenceRole != "" && (p != PurposeIdentity || !validEvidenceRole(file.EvidenceRole)) {
		return ErrInvalid
	}
	return s.transact(ctx, p, id, rev, actor, "", TransitionAudit{Action: "file_added", ResultCode: string(FilePending)}, func(x *Snapshot, a Authorization) error {
		if x.SubjectID != actor || p == PurposeSpace && x.SubjectID != a.SpaceOwnerID {
			return ErrCapability
		}
		if x.State != CaseDraft && x.State != CaseNeedsEvidence {
			return ErrTransition
		}
		if p == PurposeSpace && file.EvidenceRole != "" || p == PurposeIdentity && x.DocumentFormatDeclared != "" &&
			(file.EvidenceRole == "" || file.MIME == "application/pdf") ||
			p == PurposeIdentity && x.DocumentFormatDeclared == "" && file.EvidenceRole != "" {
			return ErrInvalid
		}
		if p == PurposeIdentity && x.DocumentFormatDeclared != "" {
			files := make([]File, 0, len(x.Files)+1)
			replaced := false
			for _, existing := range x.Files {
				if existing.EvidenceRole == file.EvidenceRole {
					files = append(files, file)
					replaced = true
					continue
				}
				files = append(files, existing)
			}
			if !replaced {
				files = append(files, file)
			}
			x.Files = files
			return ValidateQuota(p, x.Files)
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
	if x.Purpose == PurposeIdentity && x.DocumentFormatDeclared != "" {
		if len(x.Files) != 3 {
			return ErrEvidenceIncomplete
		}
		seen := map[EvidenceRole]bool{}
		for _, file := range x.Files {
			if !validEvidenceRole(file.EvidenceRole) || seen[file.EvidenceRole] || file.MIME == "application/pdf" {
				return ErrEvidenceIncomplete
			}
			seen[file.EvidenceRole] = true
		}
		if !seen[EvidenceRoleIdentityFront] || !seen[EvidenceRoleIdentityBack] || !seen[EvidenceRoleIdentitySelfie] {
			return ErrEvidenceIncomplete
		}
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
	return s.reviewWithObservation(ctx, actor, id, cap, d, reason, "", time.Time{}, nil, false, requestedExpiry, rev)
}

func (s *Service) ReviewIdentityWithReason(ctx context.Context, actor, id string, d Decision, reason ReasonCode, observedFormat DocumentFormat, observedExpiry time.Time, checklist *IdentityReviewChecklist, requestedExpiry time.Time, rev uint64) error {
	return s.reviewWithObservation(ctx, actor, id, CapabilityIdentity, d, reason, observedFormat, observedExpiry, checklist, true, requestedExpiry, rev)
}

func (s *Service) reviewWithObservation(ctx context.Context, actor, id string, cap Capability, d Decision, reason ReasonCode, observedFormat DocumentFormat, observedExpiry time.Time, checklist *IdentityReviewChecklist, hasObservation bool, requestedExpiry time.Time, rev uint64) error {
	p := purposeForCap(cap)
	if p == "" || ValidateID(actor) != nil || !validReviewReason(d, reason) {
		return ErrInvalid
	}
	if hasObservation && (p != PurposeIdentity || !validObservedDocumentFormat(observedFormat)) {
		return ErrInvalid
	}
	resultCode := map[Decision]string{DecisionApprove: string(CaseApproved), DecisionReject: string(CaseRejected), DecisionNeedsEvidence: string(CaseNeedsEvidence)}[d]
	audit := TransitionAudit{Action: "review_decision", ResultCode: resultCode, ReasonCode: reason, DocumentFormatObserved: observedFormat, DocumentExpiresAtObserved: observedExpiry.UTC(), IdentityReviewChecklist: checklist}
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
		typedIdentity := p == PurposeIdentity && x.DocumentFormatDeclared != ""
		if typedIdentity {
			if !hasObservation || !validObservedDocumentFormat(observedFormat) || !checklist.Complete() {
				return ErrInvalid
			}
			x.DocumentFormatObserved = observedFormat
			x.DocumentExpiresAt = observedExpiry.UTC()
			if d == DecisionApprove && (observedFormat == DocumentFormatUndetermined || observedExpiry.IsZero() || !checklist.AllPassed()) {
				return ErrInvalid
			}
		} else if checklist != nil {
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
	if s == nil || s.repo == nil || ctx == nil || ValidateID(id) != nil || ValidateID(actor) != nil || (p != PurposeIdentity && p != PurposeSpace) {
		return ErrInvalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.repo.Transition(ctx, actor, p, id, rev, audit, func(x *Snapshot, a Authorization) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
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
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := change(x, a); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil
	})
}
