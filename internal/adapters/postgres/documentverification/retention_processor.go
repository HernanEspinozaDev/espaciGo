package documentverificationpg

import (
	"context"
	"errors"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"time"
)

const retentionStoreError = "retention_store_error"

const maxRetentionRetryDelay = 24 * time.Hour

func retentionRetryDelay(base time.Duration, attempt int16) time.Duration {
	if base <= 0 {
		return 0
	}
	delay := base
	for i := int16(1); i < attempt && delay < maxRetentionRetryDelay; i++ {
		if delay > maxRetentionRetryDelay/2 {
			return maxRetentionRetryDelay
		}
		delay *= 2
	}
	if delay > maxRetentionRetryDelay {
		return maxRetentionRetryDelay
	}
	return delay
}

type retentionProcessorJobs interface {
	ClaimReadyRetentionJob(context.Context, time.Duration) (*RetentionClaim, error)
	ProcessClaimedRetentionJob(context.Context, RetentionClaim, func(context.Context) error, time.Duration) error
}
type retentionDocuments interface {
	DeleteIdentityDocument(context.Context, string, string, string) error
	DeleteSpaceDocument(context.Context, string, string, string, string) error
}
type RetentionProcessor struct {
	jobs         retentionProcessorJobs
	documents    retentionDocuments
	lease, retry time.Duration
}

func NewRetentionProcessor(j retentionProcessorJobs, d retentionDocuments, lease, retry time.Duration) *RetentionProcessor {
	if lease <= 0 || lease > 5*time.Minute || retry <= 0 || retry > 24*time.Hour {
		return nil
	}
	return &RetentionProcessor{j, d, lease, retry}
}
func (p *RetentionProcessor) ProcessOne(ctx context.Context) error {
	if p == nil || p.jobs == nil || p.documents == nil {
		return dv.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c, err := p.jobs.ClaimReadyRetentionJob(ctx, p.lease)
	if err != nil || c == nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	deleteObject := func(deleteCtx context.Context) error {
		switch c.Purpose {
		case dv.PurposeIdentity:
			return p.documents.DeleteIdentityDocument(deleteCtx, c.SubjectID, c.CaseID, c.FileID)
		case dv.PurposeSpace:
			if c.SpaceID == "" {
				return errors.New("missing space scope")
			}
			return p.documents.DeleteSpaceDocument(deleteCtx, c.SubjectID, c.SpaceID, c.CaseID, c.FileID)
		default:
			return errors.New("invalid document purpose")
		}
	}
	return p.jobs.ProcessClaimedRetentionJob(ctx, *c, deleteObject, retentionRetryDelay(p.retry, c.Attempt))
}
func (p *RetentionProcessor) Run(ctx context.Context, poll time.Duration, onError func(error)) error {
	if ctx == nil || p == nil || poll <= 0 || onError == nil {
		return dv.ErrInvalid
	}
	t := time.NewTicker(poll)
	defer t.Stop()
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
		case <-t.C:
		}
	}
}
