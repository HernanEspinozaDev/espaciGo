package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"time"

	clamav "github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const (
	processorErrUnavailable = "scanner_unavailable"
	processorErrMalformed   = "scanner_malformed"
	processorErrStale       = "signatures_stale"
	processorErrRead        = "retention_store_error"
)

type scanProcessorJobs interface {
	ClaimReadyScanJob(context.Context, time.Duration) (*ScanClaim, error)
	RecordScanRetry(context.Context, ScanClaim, string, time.Duration) error
	RecordScanOutcome(context.Context, ScanClaim, ScanOutcome) error
}
type scanProcessorDocuments interface {
	ReadIdentityDocument(context.Context, string, string, string) ([]byte, error)
	ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error)
}
type detailedScanner interface {
	ScanReaderDetailed(context.Context, io.Reader) (clamav.ScanResult, error)
}

type ScanProcessor struct {
	jobs            scanProcessorJobs
	documents       scanProcessorDocuments
	scanner         detailedScanner
	lease, retry    time.Duration
	maxSignatureAge time.Duration
	now             func() time.Time
}

func NewScanProcessor(jobs scanProcessorJobs, documents scanProcessorDocuments, scanner detailedScanner, lease, retry, maxSignatureAge time.Duration) *ScanProcessor {
	if lease <= 0 || lease > 5*time.Minute || retry <= 0 || retry > 24*time.Hour || maxSignatureAge <= 0 || maxSignatureAge > 30*24*time.Hour {
		return nil
	}
	return &ScanProcessor{jobs: jobs, documents: documents, scanner: scanner, lease: lease, retry: retry, maxSignatureAge: maxSignatureAge, now: func() time.Time { return time.Now().UTC() }}
}

func (p *ScanProcessor) ProcessOne(ctx context.Context) error {
	if ctx == nil || p == nil || p.jobs == nil || p.documents == nil || p.scanner == nil {
		return dv.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	claim, err := p.jobs.ClaimReadyScanJob(ctx, p.lease)
	if err != nil {
		return err
	}
	if claim == nil {
		return nil
	}
	retry := func(code string) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		return p.jobs.RecordScanRetry(ctx, *claim, code, p.retry)
	}
	var content []byte
	switch claim.Purpose {
	case dv.PurposeIdentity:
		content, err = p.documents.ReadIdentityDocument(ctx, claim.SubjectID, claim.CaseID, claim.FileID)
	case dv.PurposeSpace:
		if claim.SpaceID == "" {
			return retry(processorErrRead)
		}
		content, err = p.documents.ReadSpaceDocument(ctx, claim.SubjectID, claim.SpaceID, claim.CaseID, claim.FileID)
	default:
		return retry(processorErrRead)
	}
	if err != nil {
		return retry(processorErrRead)
	}
	if len(content) == 0 || sha256.Sum256(content) != claim.ContentSHA256 {
		return retry(processorErrMalformed)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	result, err := p.scanner.ScanReaderDetailed(ctx, bytes.NewReader(content))
	if err != nil {
		return retry(processorErrUnavailable)
	}
	if err := clamav.ValidateScanResultMetadata(result); err != nil {
		return retry(processorErrMalformed)
	}
	now := p.now().UTC()
	if result.SignaturesAt.After(now.Add(5*time.Minute)) || now.Sub(result.SignaturesAt) > p.maxSignatureAge {
		return retry(processorErrStale)
	}
	outcome := ScanOutcome{State: result.State, ScannerVersion: result.ScannerVersion, SignaturesAt: result.SignaturesAt.UTC()}
	switch result.State {
	case dv.FileClean:
	case dv.FileBlocked:
		outcome.ErrorCode = "malware_detected"
		outcome.ReasonCode = dv.ReasonMalwareDetected
	default:
		return retry(processorErrMalformed)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.jobs.RecordScanOutcome(ctx, *claim, outcome)
}

func (p *ScanProcessor) Run(ctx context.Context, pollInterval time.Duration, onError func(error)) error {
	if ctx == nil || p == nil || p.jobs == nil || p.documents == nil || p.scanner == nil || pollInterval <= 0 || onError == nil {
		return dv.ErrInvalid
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if err := p.ProcessOne(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			onError(err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
