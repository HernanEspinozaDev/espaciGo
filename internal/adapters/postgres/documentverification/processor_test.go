package documentverificationpg

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
	"time"

	clamav "github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type processorJobs struct {
	claim        *ScanClaim
	claimed      int
	retryCode    string
	retryDelay   time.Duration
	retryCalls   int
	outcome      *ScanOutcome
	outcomeClaim ScanClaim
	outcomeCalls int
	onOutcome    func()
}

func (j *processorJobs) ClaimReadyScanJob(context.Context, time.Duration) (*ScanClaim, error) {
	j.claimed++
	return j.claim, nil
}
func (j *processorJobs) RecordScanRetry(_ context.Context, _ ScanClaim, code string, d time.Duration) error {
	j.retryCalls++
	j.retryCode = code
	j.retryDelay = d
	return nil
}
func (j *processorJobs) RecordScanOutcome(_ context.Context, claim ScanClaim, o ScanOutcome) error {
	j.outcomeCalls++
	j.outcomeClaim = claim
	j.outcome = &o
	if j.onOutcome != nil {
		j.onOutcome()
	}
	return nil
}

type processorDocs struct {
	content                        []byte
	err                            error
	purpose                        dv.Purpose
	subject, space, caseID, fileID string
	identityReads, spaceReads      int
}

func (d *processorDocs) ReadIdentityDocument(_ context.Context, subject, caseID, fileID string) ([]byte, error) {
	d.identityReads++
	d.purpose = dv.PurposeIdentity
	d.subject = subject
	d.caseID = caseID
	d.fileID = fileID
	return d.content, d.err
}
func (d *processorDocs) ReadSpaceDocument(_ context.Context, subject, space, caseID, fileID string) ([]byte, error) {
	d.spaceReads++
	d.purpose = dv.PurposeSpace
	d.subject = subject
	d.space = space
	d.caseID = caseID
	d.fileID = fileID
	return d.content, d.err
}

type processorScanner struct {
	result clamav.ScanResult
	err    error
	calls  int
}

func (s *processorScanner) ScanReaderDetailed(context.Context, io.Reader) (clamav.ScanResult, error) {
	s.calls++
	return s.result, s.err
}
func claimFor(content []byte) *ScanClaim {
	h := sha256.Sum256(content)
	return &ScanClaim{JobID: "job", FileID: "file", CaseID: "case", SubjectID: "subject", Purpose: dv.PurposeSpace, SpaceID: "space", ContentSHA256: h}
}

func processorValidResult(state dv.FileState) clamav.ScanResult {
	return clamav.ScanResult{State: state, ScannerVersion: "ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026", SignaturesAt: time.Date(2026, 10, 28, 6, 24, 17, 0, time.UTC)}
}

func TestProcessorForwardsCleanAndInfectedMetadata(t *testing.T) {
	result := processorValidResult(dv.FileClean)
	now := result.SignaturesAt.Add(time.Minute)
	for _, tc := range []struct {
		name  string
		state dv.FileState
		want  dv.ReasonCode
	}{{"clean", dv.FileClean, ""}, {"infected", dv.FileBlocked, dv.ReasonMalwareDetected}} {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte("synthetic document")
			jobs := &processorJobs{claim: claimFor(content)}
			docs := &processorDocs{content: content}
			result.State = tc.state
			scan := &processorScanner{result: result}
			p := NewScanProcessor(jobs, docs, scan, time.Minute, time.Second, 48*time.Hour)
			p.now = func() time.Time { return now }
			if err := p.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
			if jobs.outcome == nil || jobs.outcome.State != tc.state || jobs.outcome.ScannerVersion != result.ScannerVersion || !jobs.outcome.SignaturesAt.Equal(result.SignaturesAt) || jobs.outcome.ReasonCode != tc.want {
				t.Fatalf("outcome=%+v", jobs.outcome)
			}
			if jobs.outcomeCalls != 1 || jobs.retryCalls != 0 || jobs.outcomeClaim != *jobs.claim {
				t.Fatalf("outcome calls=%d retry calls=%d persisted claim=%+v", jobs.outcomeCalls, jobs.retryCalls, jobs.outcomeClaim)
			}
			if docs.purpose != dv.PurposeSpace || docs.subject != "subject" || docs.space != "space" || docs.caseID != "case" || docs.fileID != "file" {
				t.Fatalf("scope=%+v", docs)
			}
		})
	}
}
func TestProcessorRoutesIdentityAndSpaceClaimsByPurpose(t *testing.T) {
	for _, purpose := range []dv.Purpose{dv.PurposeIdentity, dv.PurposeSpace} {
		t.Run(string(purpose), func(t *testing.T) {
			content := []byte("synthetic document")
			claim := claimFor(content)
			claim.Purpose = purpose
			if purpose == dv.PurposeIdentity {
				claim.SpaceID = ""
			}
			docs := &processorDocs{content: content}
			jobs := &processorJobs{claim: claim}
			result := processorValidResult(dv.FileClean)
			processor := NewScanProcessor(jobs, docs, &processorScanner{result: result}, time.Minute, time.Second, 48*time.Hour)
			processor.now = func() time.Time { return result.SignaturesAt.Add(time.Minute) }
			if err := processor.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
			if docs.identityReads != boolInt(purpose == dv.PurposeIdentity) || docs.spaceReads != boolInt(purpose == dv.PurposeSpace) {
				t.Fatalf("identity reads=%d, space reads=%d", docs.identityReads, docs.spaceReads)
			}
		})
	}
}

func TestProcessorRejectsInvalidPurposeScopeBeforeStorageRead(t *testing.T) {
	for _, tc := range []struct {
		name    string
		purpose dv.Purpose
		spaceID string
	}{
		{name: "unknown purpose", purpose: dv.Purpose("unexpected"), spaceID: "space"},
		{name: "space without space id", purpose: dv.PurposeSpace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte("synthetic document")
			claim := claimFor(content)
			claim.Purpose, claim.SpaceID = tc.purpose, tc.spaceID
			jobs := &processorJobs{claim: claim}
			docs := &processorDocs{content: content}
			scanner := &processorScanner{}
			processor := NewScanProcessor(jobs, docs, scanner, time.Minute, time.Second, time.Hour)
			if err := processor.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
			if docs.identityReads != 0 || docs.spaceReads != 0 || scanner.calls != 0 || jobs.retryCode != processorErrRead || jobs.retryCalls != 1 || jobs.outcomeCalls != 0 {
				t.Fatalf("docs=%+v scanner calls=%d jobs=%+v", docs, scanner.calls, jobs)
			}
		})
	}
}

func TestProcessorCorruptContentNeverScansAndRetries(t *testing.T) {
	jobs := &processorJobs{claim: claimFor([]byte("expected"))}
	docs := &processorDocs{content: []byte("corrupt")}
	scan := &processorScanner{}
	p := NewScanProcessor(jobs, docs, scan, time.Minute, time.Second, time.Minute)
	if err := p.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if scan.calls != 0 || jobs.outcome != nil || jobs.retryCode != "scanner_malformed" {
		t.Fatalf("calls=%d outcome=%+v retry=%q", scan.calls, jobs.outcome, jobs.retryCode)
	}
}
func TestProcessorScannerFailureOrStaleMetadataRetries(t *testing.T) {
	const retryDelay = 3 * time.Second
	for _, tc := range []struct {
		name    string
		scan    processorScanner
		wantErr string
	}{{"unavailable", processorScanner{err: io.ErrUnexpectedEOF}, processorErrUnavailable}, {"stale", processorScanner{result: clamav.ScanResult{State: dv.FileClean, ScannerVersion: "ClamAV 1.5.4/28100/Tue Oct 20 06:24:17 2026", SignaturesAt: time.Date(2026, 10, 20, 6, 24, 17, 0, time.UTC)}}, processorErrStale}} {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte("doc")
			jobs := &processorJobs{claim: claimFor(content)}
			docs := &processorDocs{content: content}
			p := NewScanProcessor(jobs, docs, &tc.scan, time.Minute, retryDelay, time.Minute)
			p.now = func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC) }
			if err := p.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
			if jobs.outcome != nil || jobs.retryCode != tc.wantErr || jobs.retryDelay != retryDelay {
				t.Fatalf("outcome=%+v retry=%q delay=%s, want retry=%q delay=%s", jobs.outcome, jobs.retryCode, jobs.retryDelay, tc.wantErr, retryDelay)
			}
		})
	}
}

func TestProcessorRejectsMalformedScannerMetadata(t *testing.T) {
	content := []byte("synthetic document")
	now := time.Now().UTC()
	jobs := &processorJobs{claim: claimFor(content)}
	scanner := &processorScanner{result: clamav.ScanResult{
		State:          dv.FileBlocked,
		ScannerVersion: "ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026\nforged",
		SignaturesAt:   now,
	}}
	processor := NewScanProcessor(jobs, &processorDocs{content: content}, scanner, time.Minute, time.Second, time.Hour)
	processor.now = func() time.Time { return now }
	if err := processor.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.outcome != nil || jobs.retryCode != processorErrMalformed {
		t.Fatalf("outcome=%+v retry=%q, want malformed result retry", jobs.outcome, jobs.retryCode)
	}
}

func TestProcessorRejectsNilContext(t *testing.T) {
	processor := NewScanProcessor(&processorJobs{}, &processorDocs{}, &processorScanner{}, time.Minute, time.Second, time.Minute)
	if err := processor.ProcessOne(nil); !errors.Is(err, dv.ErrInvalid) {
		t.Fatalf("ProcessOne(nil) error=%v, want ErrInvalid", err)
	}
}

type cancelAwareProcessorDocs struct{ started chan struct{} }

func (d *cancelAwareProcessorDocs) ReadIdentityDocument(ctx context.Context, _, _, _ string) ([]byte, error) {
	close(d.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (d *cancelAwareProcessorDocs) ReadSpaceDocument(ctx context.Context, _, _, _, _ string) ([]byte, error) {
	close(d.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestProcessorCancellationDuringDocumentReadDoesNotRetry(t *testing.T) {
	content := []byte("synthetic document")
	jobs := &processorJobs{claim: claimFor(content)}
	docs := &cancelAwareProcessorDocs{started: make(chan struct{})}
	scanner := &processorScanner{}
	processor := NewScanProcessor(jobs, docs, scanner, time.Minute, time.Second, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- processor.ProcessOne(ctx) }()
	select {
	case <-docs.started:
	case <-time.After(time.Second):
		t.Fatal("processor did not pass context to document storage")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ProcessOne error=%v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("processor did not stop after document-read cancellation")
	}
	if jobs.retryCalls != 0 || jobs.outcomeCalls != 0 || scanner.calls != 0 {
		t.Fatalf("retry calls=%d outcome calls=%d scan calls=%d", jobs.retryCalls, jobs.outcomeCalls, scanner.calls)
	}
}

type cancelAwareProcessorScanner struct{ started chan struct{} }

func (s *cancelAwareProcessorScanner) ScanReaderDetailed(ctx context.Context, _ io.Reader) (clamav.ScanResult, error) {
	close(s.started)
	<-ctx.Done()
	return clamav.ScanResult{}, ctx.Err()
}

func TestProcessorCancellationAfterClaimLeavesBoundedLeaseForExpiry(t *testing.T) {
	content := []byte("synthetic document")
	lease := 5 * time.Second
	claim := claimFor(content)
	claim.LeaseUntil = time.Now().Add(lease)
	jobs := &processorJobs{claim: claim}
	scanner := &cancelAwareProcessorScanner{started: make(chan struct{})}
	processor := NewScanProcessor(jobs, &processorDocs{content: content}, scanner, lease, time.Second, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- processor.ProcessOne(ctx) }()
	select {
	case <-scanner.started:
	case <-time.After(time.Second):
		t.Fatal("processor did not reach scanner after claiming the job")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ProcessOne error=%v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("processor did not stop promptly after cancellation")
	}
	if jobs.claimed != 1 || jobs.outcome != nil || jobs.retryCode != "" {
		t.Fatalf("claimed=%d outcome=%+v retry=%q", jobs.claimed, jobs.outcome, jobs.retryCode)
	}
	if !claim.LeaseUntil.After(time.Now()) || time.Until(claim.LeaseUntil) > lease {
		t.Fatalf("cancellation did not leave the bounded lease for expiry: lease_until=%s", claim.LeaseUntil)
	}
}

func TestProcessorNoClaimDoesNothing(t *testing.T) {
	jobs := &processorJobs{}
	p := NewScanProcessor(jobs, &processorDocs{}, &processorScanner{}, time.Minute, time.Second, time.Minute)
	if err := p.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.claimed != 1 || jobs.outcome != nil || jobs.retryCode != "" {
		t.Fatalf("jobs=%+v", jobs)
	}
}

func TestProcessorRunProcessesClaimAndStopsOnCancellation(t *testing.T) {
	content := []byte("synthetic document")
	jobs := &processorJobs{claim: claimFor(content)}
	ctx, cancel := context.WithCancel(context.Background())
	jobs.onOutcome = cancel
	result := processorValidResult(dv.FileClean)
	now := result.SignaturesAt.Add(time.Minute)
	processor := NewScanProcessor(jobs, &processorDocs{content: content}, &processorScanner{result: result}, time.Minute, time.Second, 48*time.Hour)
	processor.now = func() time.Time { return now }
	if err := processor.Run(ctx, time.Millisecond, func(err error) { t.Errorf("unexpected processor error: %v", err) }); err != nil {
		t.Fatal(err)
	}
	if jobs.claimed != 1 || jobs.outcome == nil || jobs.retryCode != "" {
		t.Fatalf("claimed=%d outcome=%+v retry=%q", jobs.claimed, jobs.outcome, jobs.retryCode)
	}
}

func TestScanProcessorDurationBounds(t *testing.T) {
	jobs, docs, scanner := &processorJobs{}, &processorDocs{}, &processorScanner{}
	for _, tc := range []struct {
		name              string
		lease, retry, age time.Duration
	}{
		{name: "zero lease", lease: 0, retry: time.Second, age: time.Hour},
		{name: "excessive lease", lease: 5*time.Minute + time.Nanosecond, retry: time.Second, age: time.Hour},
		{name: "zero retry", lease: time.Minute, retry: 0, age: time.Hour},
		{name: "excessive retry", lease: time.Minute, retry: 24*time.Hour + time.Nanosecond, age: time.Hour},
		{name: "zero signature age", lease: time.Minute, retry: time.Second, age: 0},
		{name: "excessive signature age", lease: time.Minute, retry: time.Second, age: 30*24*time.Hour + time.Nanosecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if processor := NewScanProcessor(jobs, docs, scanner, tc.lease, tc.retry, tc.age); processor != nil {
				t.Fatalf("NewScanProcessor()=%+v, want nil for out-of-bounds durations", processor)
			}
		})
	}
	for _, tc := range []struct {
		name              string
		lease, retry, age time.Duration
	}{
		{name: "minimum", lease: time.Nanosecond, retry: time.Nanosecond, age: time.Nanosecond},
		{name: "maximum", lease: 5 * time.Minute, retry: 24 * time.Hour, age: 30 * 24 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if processor := NewScanProcessor(jobs, docs, scanner, tc.lease, tc.retry, tc.age); processor == nil {
				t.Fatal("NewScanProcessor() rejected a documented duration boundary")
			}
		})
	}
}

func TestScanErrorCodeUsesFixedAllowlist(t *testing.T) {
	for _, code := range []string{"scanner_unavailable", "scanner_malformed", "signatures_stale", "malware_detected", "inspection_rejected", "retention_store_error"} {
		if !scanErrorCode(code) {
			t.Errorf("scanErrorCode(%q)=false, want true", code)
		}
	}
	for _, code := range []string{"", "scanner_unavailable\nforged", "database password leaked", "unexpected"} {
		if scanErrorCode(code) {
			t.Errorf("scanErrorCode(%q)=true, want false", code)
		}
	}
}

var _ io.Reader
