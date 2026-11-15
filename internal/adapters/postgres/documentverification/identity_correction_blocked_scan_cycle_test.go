package documentverificationpg

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestIdentityCorrectionBlockedScanCannotResubmit(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	repo := &correctionScanRepository{
		state: dv.Snapshot{
			ID:                            correctionScanCase,
			Purpose:                       dv.PurposeIdentity,
			SubjectID:                     correctionScanSubject,
			State:                         dv.CaseSubmitted,
			Revision:                      5,
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
		NamesMatch:                 boolPtr(true),
		RUTMatches:                 boolPtr(true),
		BirthDateAndMajorityMatch:  boolPtr(true),
		PortraitMatchesSelfie:      boolPtr(true),
		EvidenceLegible:            boolPtr(false),
		DocumentUnexpired:          boolPtr(true),
		IssueDateAndFormatVerified: boolPtr(true),
	}
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

	result := processorValidResult(dv.FileBlocked)
	claim := ScanClaim{
		JobID: correctionScanJob, FileID: receipt.File.ID, CaseID: correctionScanCase, SubjectID: correctionScanSubject,
		Purpose: dv.PurposeIdentity, Size: int64(len(store.content)), MIME: "image/png", Pages: 1,
		ContentSHA256: sha256.Sum256(store.content), Attempt: 1, CaseRevision: int64(repo.state.Revision), LeaseUntil: now.Add(time.Minute),
	}
	jobs := &correctionScanJobs{repo: repo, claim: claim}
	docs := &correctionScanDocuments{store: store}
	scanner := &correctionScanScanner{result: result, expected: append([]byte(nil), store.content...)}
	processor := NewScanProcessor(jobs, docs, scanner, time.Minute, time.Second, 48*time.Hour)
	processor.now = func() time.Time { return result.SignaturesAt.Add(time.Minute) }
	if err = processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.outcome == nil || jobs.outcome.State != dv.FileBlocked || jobs.outcome.ErrorCode != "malware_detected" || jobs.outcome.ReasonCode != dv.ReasonMalwareDetected || jobs.outcomeClaim != claim {
		t.Fatalf("scan outcome=%+v claim=%+v", jobs.outcome, jobs.outcomeClaim)
	}
	if scanner.calls != 1 || !reflect.DeepEqual(scanner.scanned, store.content) || docs.reads != 1 || docs.readSubject != correctionScanSubject || docs.readCaseID != correctionScanCase || docs.readFileID != receipt.File.ID {
		t.Fatalf("scan interactions: scanner=%+v documents=%+v", scanner, docs)
	}
	if repo.state.State != dv.CaseNeedsEvidence || repo.state.Revision != 8 {
		t.Fatalf("scan changed case state/revision: %+v", repo.state)
	}
	blockedFound := false
	for _, file := range repo.state.Files {
		if file.ID == receipt.File.ID && file.EvidenceRole == dv.EvidenceRoleIdentityFront && file.State == dv.FileBlocked {
			blockedFound = true
		}
	}
	if !blockedFound {
		t.Fatalf("replacement file was not blocked: %+v", repo.state.Files)
	}

	before := repo.state
	before.Files = append([]dv.File(nil), repo.state.Files...)
	beforeAudit := repo.audit
	if err = service.Submit(ctx, correctionScanSubject, correctionScanCase, dv.PurposeIdentity, repo.state.Revision); !errors.Is(err, dv.ErrFilesNotClean) {
		t.Fatalf("blocked submit error=%v", err)
	}
	if !reflect.DeepEqual(repo.state, before) || !reflect.DeepEqual(repo.audit, beforeAudit) {
		t.Fatal("blocked submit mutated snapshot or audit")
	}
}
