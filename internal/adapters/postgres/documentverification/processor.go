package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
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

const scanClaimPollInterval = 250 * time.Millisecond

var errScanClaimFenced = errors.New("scan claim is no longer current")

type scanProcessorJobs interface {
	ClaimReadyScanJob(context.Context, time.Duration) (*ScanClaim, error)
	VerifyScanClaim(context.Context, ScanClaim) (bool, error)
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
	jobs              scanProcessorJobs
	documents         scanProcessorDocuments
	scanner           detailedScanner
	lease, retry      time.Duration
	maxSignatureAge   time.Duration
	claimPollInterval time.Duration
	now               func() time.Time
}

func NewScanProcessor(jobs scanProcessorJobs, documents scanProcessorDocuments, scanner detailedScanner, lease, retry, maxSignatureAge time.Duration) *ScanProcessor {
	if lease <= 0 || lease > 5*time.Minute || retry <= 0 || retry > 24*time.Hour || maxSignatureAge <= 0 || maxSignatureAge > 30*24*time.Hour {
		return nil
	}
	return &ScanProcessor{jobs: jobs, documents: documents, scanner: scanner, lease: lease, retry: retry, maxSignatureAge: maxSignatureAge, claimPollInterval: scanClaimPollInterval, now: func() time.Time { return time.Now().UTC() }}
}

func (p *ScanProcessor) monitorClaim(ctx context.Context, claim ScanClaim) (context.Context, func()) {
	operationCtx, cancel := context.WithCancelCause(ctx)
	stopped := make(chan struct{})
	interval := p.claimPollInterval
	if interval <= 0 {
		interval = scanClaimPollInterval
	}
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-operationCtx.Done():
				return
			case <-ticker.C:
				current, err := p.jobs.VerifyScanClaim(operationCtx, claim)
				if err != nil {
					if operationCtx.Err() == nil {
						cancel(err)
					}
					return
				}
				if !current {
					cancel(errScanClaimFenced)
					return
				}
			}
		}
	}()
	return operationCtx, func() {
		cancel(context.Canceled)
		<-stopped
	}
}

func scanCancellationResult(parent, operation context.Context) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if cause := context.Cause(operation); cause != nil {
		if errors.Is(cause, errScanClaimFenced) {
			return nil
		}
		return cause
	}
	return operation.Err()
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
	current, err := p.jobs.VerifyScanClaim(ctx, *claim)
	if err != nil {
		return err
	}
	if !current {
		return nil
	}
	operationCtx, stopMonitor := p.monitorClaim(ctx, *claim)
	defer stopMonitor()
	retry := func(code string) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if operationCtx.Err() != nil {
			return scanCancellationResult(ctx, operationCtx)
		}
		return p.jobs.RecordScanRetry(ctx, *claim, code, p.retry)
	}
	var content []byte
	switch claim.Purpose {
	case dv.PurposeIdentity:
		content, err = p.documents.ReadIdentityDocument(operationCtx, claim.SubjectID, claim.CaseID, claim.FileID)
	case dv.PurposeSpace:
		if claim.SpaceID == "" {
			return retry(processorErrRead)
		}
		content, err = p.documents.ReadSpaceDocument(operationCtx, claim.SubjectID, claim.SpaceID, claim.CaseID, claim.FileID)
	default:
		return retry(processorErrRead)
	}
	if err != nil {
		return retry(processorErrRead)
	}
	if operationCtx.Err() != nil {
		return scanCancellationResult(ctx, operationCtx)
	}
	if len(content) == 0 || sha256.Sum256(content) != claim.ContentSHA256 {
		return retry(processorErrMalformed)
	}
	current, err = p.jobs.VerifyScanClaim(operationCtx, *claim)
	if err != nil {
		if operationCtx.Err() != nil {
			return scanCancellationResult(ctx, operationCtx)
		}
		return err
	}
	if !current {
		return nil
	}
	if operationCtx.Err() != nil {
		return scanCancellationResult(ctx, operationCtx)
	}
	result, err := p.scanner.ScanReaderDetailed(operationCtx, bytes.NewReader(content))
	if err != nil {
		return retry(processorErrUnavailable)
	}
	if operationCtx.Err() != nil {
		return scanCancellationResult(ctx, operationCtx)
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
	if operationCtx.Err() != nil {
		return scanCancellationResult(ctx, operationCtx)
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
