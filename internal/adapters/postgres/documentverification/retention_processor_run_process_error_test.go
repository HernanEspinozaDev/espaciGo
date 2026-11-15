package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type retentionRunProcessFailureJobs struct {
	claim         *RetentionClaim
	processErr    error
	claimCalls    int
	leaseRequests []time.Duration
	processCalls  int
	afterSuccess  func()
}

func (j *retentionRunProcessFailureJobs) ClaimReadyRetentionJob(_ context.Context, lease time.Duration) (*RetentionClaim, error) {
	j.claimCalls++
	j.leaseRequests = append(j.leaseRequests, lease)
	return j.claim, nil
}

func (j *retentionRunProcessFailureJobs) ProcessClaimedRetentionJob(ctx context.Context, _ RetentionClaim, deleteObject func(context.Context) error, _ time.Duration) error {
	j.processCalls++
	if j.processCalls == 1 {
		return j.processErr
	}
	if err := deleteObject(ctx); err != nil {
		return err
	}
	if j.afterSuccess != nil {
		j.afterSuccess()
	}
	return nil
}

func TestRetentionProcessorRunContinuesAfterClaimedJobProcessingError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	processErr := errors.New("synthetic fenced processing failure")
	claim := retentionClaim(dv.PurposeIdentity)
	jobs := &retentionRunProcessFailureJobs{claim: claim, processErr: processErr, afterSuccess: cancel}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
	errorCallbacks := 0

	if err := processor.Run(ctx, time.Millisecond, func(err error) {
		errorCallbacks++
		if !errors.Is(err, processErr) {
			t.Errorf("onError(%v); want processing error", err)
		}
	}); err != nil {
		t.Fatalf("Run() error=%v; want graceful nil after later successful processing", err)
	}
	if jobs.claimCalls != 2 || jobs.processCalls != 2 || errorCallbacks != 1 {
		t.Fatalf("claims=%d processed=%d error callbacks=%d; want 2, 2, 1", jobs.claimCalls, jobs.processCalls, errorCallbacks)
	}
	if len(jobs.leaseRequests) != 2 || jobs.leaseRequests[0] != time.Minute || jobs.leaseRequests[1] != time.Minute {
		t.Fatalf("lease requests=%v; want two configured leases", jobs.leaseRequests)
	}
	if storage.identity != 1 || storage.space != 0 || storage.identitySubject != claim.SubjectID || storage.identityCase != claim.CaseID || storage.identityFile != claim.FileID {
		t.Fatalf("identity deletes=%d space deletes=%d scope=(%q,%q,%q); want one exact-scope delete after recovery", storage.identity, storage.space, storage.identitySubject, storage.identityCase, storage.identityFile)
	}
}
