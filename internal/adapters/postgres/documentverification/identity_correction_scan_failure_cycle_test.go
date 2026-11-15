package documentverificationpg

import (
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

type correctionScanRetryJobs struct {
	*correctionScanJobs
	calls int
	code  string
}

func (j *correctionScanRetryJobs) RecordScanRetry(_ context.Context, _ ScanClaim, code string, _ time.Duration) error {
	j.calls++
	j.code = code
	return nil
}

type correctionScanFailingScanner struct {
	calls int
}

func (s *correctionScanFailingScanner) ScanReaderDetailed(context.Context, io.Reader) (clamav.ScanResult, error) {
	s.calls++
	return clamav.ScanResult{}, errors.New("synthetic scanner unavailable")
}

func TestIdentityCorrectionScannerFailureKeepsReplacementPending(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	repo := &correctionScanRepository{
		state: dv.Snapshot{
			ID: correctionScanCase, Purpose: dv.PurposeIdentity, SubjectID: correctionScanSubject,
			State: dv.CaseSubmitted, Revision: 5,
			IdentityDeclarationRevisionID: correctionScanDecl,
			DocumentFormatDeclared:        dv.DocumentFormatNew,
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            now.Add(-time.Hour),
			Files: []dv.File{
				{ID: "44444444-4444-4444-8444-444444444444", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: correctionScanBack, EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: correctionScanSelfie, EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			},
		},
	}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	service.SetClock(func() time.Time { return now })
	checklist := &dv.IdentityReviewChecklist{
		NamesMatch: boolPtr(true), RUTMatches: boolPtr(true), BirthDateAndMajorityMatch: boolPtr(true),
		PortraitMatchesSelfie: boolPtr(true), EvidenceLegible: boolPtr(false), DocumentUnexpired: boolPtr(true),
		IssueDateAndFormatVerified: boolPtr(true),
	}
	if err := service.ReviewIdentityWithReason(ctx, correctionScanActor, correctionScanCase, dv.DecisionNeedsEvidence, dv.ReasonEvidenceIncomplete, dv.DocumentFormatNew, now.AddDate(4, 0, 0), checklist, time.Time{}, 5); err != nil {
		t.Fatalf("request correction: %v", err)
	}

	store := &correctionScanUploadStore{}
	uploader, err := dv.NewUploadService(service, &correctionScanUploadInspector{}, store)
	if err != nil {
		t.Fatal(err)
	}
	replacementBytes := []byte("synthetic replacement identity image")
	receipt, err := uploader.Upload(ctx, correctionScanSubject, correctionScanCase, repo.state.Revision, dv.EvidenceRoleIdentityFront, replacementBytes)
	if err != nil {
		t.Fatalf("upload replacement: %v", err)
	}

	claim := ScanClaim{
		JobID: correctionScanJob, FileID: receipt.File.ID, CaseID: correctionScanCase,
		SubjectID: correctionScanSubject, Purpose: dv.PurposeIdentity,
		Size: int64(len(store.content)), MIME: "image/png", Pages: 1,
		ContentSHA256: sha256.Sum256(store.content), Attempt: 1,
		CaseRevision: int64(repo.state.Revision), LeaseUntil: now.Add(time.Minute),
	}
	baseJobs := &correctionScanJobs{repo: repo, claim: claim}
	jobs := &correctionScanRetryJobs{correctionScanJobs: baseJobs}
	docs := &correctionScanDocuments{store: store}
	scanner := &correctionScanFailingScanner{}
	processor := NewScanProcessor(jobs, docs, scanner, time.Minute, time.Second, 48*time.Hour)
	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("process synthetic scan failure: %v", err)
	}

	if jobs.calls != 1 || jobs.code != "scanner_unavailable" || baseJobs.outcome != nil || scanner.calls != 1 || docs.reads != 1 {
		t.Fatalf("retry=%d/%q outcomes=%+v scanner=%d reads=%d", jobs.calls, jobs.code, baseJobs.outcome, scanner.calls, docs.reads)
	}
	if repo.state.State != dv.CaseNeedsEvidence || repo.state.Revision != 7 || len(repo.state.Files) != 3 || repo.audit.Action != "file_added" {
		t.Fatalf("scanner failure mutated correction state: snapshot=%+v audit=%+v", repo.state, repo.audit)
	}
	foundPendingReplacement := false
	for _, file := range repo.state.Files {
		if file.ID == receipt.File.ID && file.EvidenceRole == dv.EvidenceRoleIdentityFront && file.State == dv.FilePending {
			foundPendingReplacement = true
		}
	}
	if !foundPendingReplacement {
		t.Fatalf("replacement was not retained as pending: %+v", repo.state.Files)
	}

	before := repo.state
	before.Files = append([]dv.File(nil), repo.state.Files...)
	beforeAudit := repo.audit
	if err := service.Submit(ctx, correctionScanSubject, correctionScanCase, dv.PurposeIdentity, repo.state.Revision); !errors.Is(err, dv.ErrFilesNotClean) {
		t.Fatalf("submit after scanner failure error=%v, want ErrFilesNotClean", err)
	}
	if !reflect.DeepEqual(repo.state, before) || !reflect.DeepEqual(repo.audit, beforeAudit) {
		t.Fatal("submit after scanner failure mutated case or audit")
	}
}
