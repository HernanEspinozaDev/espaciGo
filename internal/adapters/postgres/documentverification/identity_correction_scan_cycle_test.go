package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	clamav "github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const (
	correctionScanActor   = "11111111-1111-4111-8111-111111111111"
	correctionScanSubject = "22222222-2222-4222-8222-222222222222"
	correctionScanCase    = "55555555-5555-4555-8555-555555555555"
	correctionScanBack    = "66666666-6666-4666-8666-666666666666"
	correctionScanSelfie  = "77777777-7777-4777-8777-777777777777"
	correctionScanDecl    = "88888888-8888-4888-8888-888888888888"
	correctionScanJob     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type correctionScanRepository struct {
	state dv.Snapshot
	audit dv.TransitionAudit
}

func (r *correctionScanRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, errors.New("unexpected CreateCase")
}
func (r *correctionScanRepository) LoadOwnCase(_ context.Context, actor, id string) (dv.Snapshot, error) {
	if actor != r.state.SubjectID || id != r.state.ID {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	return r.state, nil
}
func (r *correctionScanRepository) ListOwnCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, errors.New("unexpected ListOwnCases")
}
func (r *correctionScanRepository) Transition(_ context.Context, actor string, purpose dv.Purpose, id string, revision uint64, audit dv.TransitionAudit, change func(*dv.Snapshot, dv.Authorization) error) error {
	if purpose != r.state.Purpose || id != r.state.ID {
		return dv.ErrNotFound
	}
	if revision != r.state.Revision {
		return dv.ErrConflict
	}
	next := r.state
	next.Files = append([]dv.File(nil), r.state.Files...)
	auth := dv.Authorization{ActorID: actor, SubjectID: r.state.SubjectID, IdentityDeclarationRevisionID: r.state.IdentityDeclarationRevisionID}
	if actor == correctionScanActor {
		auth.ReviewerGrants = []dv.ReviewerGrant{{ReviewerID: correctionScanActor, Capability: dv.CapabilityIdentity, ScopeID: r.state.SubjectID}}
	} else if actor != r.state.SubjectID {
		return dv.ErrNotFound
	}
	if err := change(&next, auth); err != nil {
		return err
	}
	next.Revision++
	r.state, r.audit = next, audit
	return nil
}

type correctionScanUploadInspector struct{ raw []byte }

func (i *correctionScanUploadInspector) Inspect(p dv.Purpose, id string, raw []byte) (dv.File, []byte, error) {
	if p != dv.PurposeIdentity {
		return dv.File{}, nil, dv.ErrInvalid
	}
	i.raw = append([]byte(nil), raw...)
	return dv.File{ID: id, State: dv.FilePending, Size: int64(len(raw)), MIME: "image/png", Pages: 1, ContentSHA256: sha256.Sum256(raw)}, append([]byte(nil), raw...), nil
}

type correctionScanUploadStore struct {
	content                 []byte
	subject, caseID, fileID string
}

func (s *correctionScanUploadStore) PutIdentityDocument(_ context.Context, subject, caseID, fileID string, content []byte) error {
	s.subject, s.caseID, s.fileID = subject, caseID, fileID
	s.content = append([]byte(nil), content...)
	return nil
}
func (*correctionScanUploadStore) PutSpaceDocument(context.Context, string, string, string, string, []byte) error {
	return errors.New("unexpected space write")
}
func (*correctionScanUploadStore) DeleteIdentityDocument(context.Context, string, string, string) error {
	return nil
}
func (*correctionScanUploadStore) DeleteSpaceDocument(context.Context, string, string, string, string) error {
	return nil
}

type correctionScanDocuments struct {
	store                               *correctionScanUploadStore
	readSubject, readCaseID, readFileID string
	reads                               int
}

func (d *correctionScanDocuments) ReadIdentityDocument(_ context.Context, subject, caseID, fileID string) ([]byte, error) {
	d.reads++
	if subject != correctionScanSubject || caseID != correctionScanCase || fileID != d.store.fileID ||
		d.store.subject != subject || d.store.caseID != caseID || d.store.fileID != fileID {
		return nil, dv.ErrNotFound
	}
	d.readSubject, d.readCaseID, d.readFileID = subject, caseID, fileID
	return append([]byte(nil), d.store.content...), nil
}
func (*correctionScanDocuments) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, errors.New("unexpected space read")
}

type correctionScanScanner struct {
	result            clamav.ScanResult
	expected, scanned []byte
	calls             int
}

func (s *correctionScanScanner) ScanReaderDetailed(_ context.Context, r io.Reader) (clamav.ScanResult, error) {
	s.calls++
	content, err := io.ReadAll(r)
	if err != nil {
		return clamav.ScanResult{}, err
	}
	if !bytes.Equal(content, s.expected) {
		return clamav.ScanResult{}, errors.New("scanner received unexpected bytes")
	}
	s.scanned = content
	return s.result, nil
}

type correctionScanJobs struct {
	repo         *correctionScanRepository
	claim        ScanClaim
	claimed      int
	outcome      *ScanOutcome
	outcomeClaim ScanClaim
	verifyCalls  int
}

func (j *correctionScanJobs) ClaimReadyScanJob(context.Context, time.Duration) (*ScanClaim, error) {
	j.claimed++
	c := j.claim
	return &c, nil
}
func (j *correctionScanJobs) VerifyScanClaim(_ context.Context, c ScanClaim) (bool, error) {
	j.verifyCalls++
	return c == j.claim, nil
}
func (j *correctionScanJobs) RecordScanRetry(context.Context, ScanClaim, string, time.Duration) error {
	return errors.New("unexpected retry")
}
func (j *correctionScanJobs) RecordScanOutcome(_ context.Context, c ScanClaim, o ScanOutcome) error {
	j.outcomeClaim = c
	j.outcome = &o
	if c.CaseRevision != int64(j.repo.state.Revision) {
		return dv.ErrConflict
	}
	for i := range j.repo.state.Files {
		if j.repo.state.Files[i].ID == c.FileID {
			j.repo.state.Files[i].State = o.State
			j.repo.state.Revision++
			return nil
		}
	}
	return dv.ErrNotFound
}

func TestIdentityCorrectionResubmitsThroughRealScanProcessor(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	repo := &correctionScanRepository{
		state: dv.Snapshot{ID: correctionScanCase, Purpose: dv.PurposeIdentity, SubjectID: correctionScanSubject, State: dv.CaseSubmitted, Revision: 5,
			IdentityDeclarationRevisionID: correctionScanDecl, DocumentFormatDeclared: dv.DocumentFormatNew,
			CaptureConsentVersion: dv.IdentityCaptureConsentVersion, CaptureConsentedAt: now.Add(-time.Hour),
			Files: []dv.File{
				{ID: "44444444-4444-4444-8444-444444444444", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: correctionScanBack, EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: correctionScanSelfie, EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			}},
	}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	service.SetClock(func() time.Time { return now })
	checklist := &dv.IdentityReviewChecklist{NamesMatch: boolPtr(true), RUTMatches: boolPtr(true), BirthDateAndMajorityMatch: boolPtr(true), PortraitMatchesSelfie: boolPtr(true), EvidenceLegible: boolPtr(false), DocumentUnexpired: boolPtr(true), IssueDateAndFormatVerified: boolPtr(true)}
	if err = service.ReviewIdentityWithReason(ctx, correctionScanActor, correctionScanCase, dv.DecisionNeedsEvidence, dv.ReasonEvidenceIncomplete, dv.DocumentFormatNew, now.AddDate(4, 0, 0), checklist, time.Time{}, 5); err != nil {
		t.Fatal(err)
	}
	inspector := &correctionScanUploadInspector{}
	store := &correctionScanUploadStore{}
	uploader, err := dv.NewUploadService(service, inspector, store)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("synthetic replacement identity front image")
	receipt, err := uploader.Upload(ctx, correctionScanSubject, correctionScanCase, repo.state.Revision, dv.EvidenceRoleIdentityFront, raw)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.File.ID == "" || receipt.File.EvidenceRole != dv.EvidenceRoleIdentityFront || receipt.Revision != 7 || !reflect.DeepEqual(store.content, inspector.raw) {
		t.Fatalf("upload receipt=%+v bytes differ", receipt)
	}
	before := repo.state
	before.Files = append([]dv.File(nil), repo.state.Files...)
	beforeAudit := repo.audit
	if err = service.Submit(ctx, correctionScanSubject, correctionScanCase, dv.PurposeIdentity, repo.state.Revision); !errors.Is(err, dv.ErrFilesNotClean) {
		t.Fatalf("pending submit error=%v", err)
	}
	if !reflect.DeepEqual(repo.state, before) || !reflect.DeepEqual(repo.audit, beforeAudit) {
		t.Fatal("pending submit mutated snapshot or audit")
	}

	result := processorValidResult(dv.FileClean)
	claim := ScanClaim{JobID: correctionScanJob, FileID: receipt.File.ID, CaseID: correctionScanCase, SubjectID: correctionScanSubject, Purpose: dv.PurposeIdentity, Size: int64(len(store.content)), MIME: "image/png", Pages: 1, ContentSHA256: sha256.Sum256(store.content), Attempt: 1, CaseRevision: int64(repo.state.Revision), LeaseUntil: now.Add(time.Minute)}
	jobs := &correctionScanJobs{repo: repo, claim: claim}
	docs := &correctionScanDocuments{store: store}
	scanner := &correctionScanScanner{result: result, expected: append([]byte(nil), store.content...)}
	processor := NewScanProcessor(jobs, docs, scanner, time.Minute, time.Second, 48*time.Hour)
	processor.now = func() time.Time { return result.SignaturesAt.Add(time.Minute) }
	if err = processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.claimed != 1 || jobs.outcome == nil || jobs.outcome.State != dv.FileClean || jobs.outcome.ScannerVersion != result.ScannerVersion || !jobs.outcome.SignaturesAt.Equal(result.SignaturesAt) || jobs.outcomeClaim != claim || jobs.verifyCalls < 2 || scanner.calls != 1 || !bytes.Equal(scanner.scanned, store.content) || docs.reads != 1 {
		t.Fatalf("jobs=%+v scanner=%+v docs=%+v", jobs, scanner, docs)
	}
	if docs.readSubject != correctionScanSubject || docs.readCaseID != correctionScanCase || docs.readFileID != receipt.File.ID || claim.SpaceID != "" || repo.state.Revision != 8 {
		t.Fatalf("scan scope=%+v case=%+v", docs, repo.state)
	}
	if err = service.Submit(ctx, correctionScanSubject, correctionScanCase, dv.PurposeIdentity, repo.state.Revision); err != nil {
		t.Fatal(err)
	}
	if repo.state.State != dv.CaseSubmitted || repo.state.ReviewerID != "" || repo.state.Revision != 9 || len(repo.state.Files) != 3 {
		t.Fatalf("resubmitted snapshot=%+v", repo.state)
	}
	if repo.audit.Action != "case_transitioned" || repo.audit.ResultCode != string(dv.CaseSubmitted) {
		t.Fatalf("submit audit=%+v", repo.audit)
	}
	roles := map[dv.EvidenceRole]string{}
	for _, f := range repo.state.Files {
		if _, ok := roles[f.EvidenceRole]; ok {
			t.Fatalf("duplicate role %q", f.EvidenceRole)
		}
		roles[f.EvidenceRole] = f.ID
	}
	if roles[dv.EvidenceRoleIdentityFront] != receipt.File.ID || roles[dv.EvidenceRoleIdentityBack] != correctionScanBack || roles[dv.EvidenceRoleIdentitySelfie] != correctionScanSelfie {
		t.Fatalf("roles=%v", roles)
	}
}

func boolPtr(v bool) *bool { return &v }
