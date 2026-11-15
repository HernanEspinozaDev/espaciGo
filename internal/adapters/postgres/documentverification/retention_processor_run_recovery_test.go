package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type retentionRunSequenceJobs struct {
	firstClaimErr  error
	claim          *RetentionClaim
	claimCalls     int
	leaseRequests  []time.Duration
	processCalls   int
	processedClaim RetentionClaim
	retryDelay     time.Duration
	afterProcess   func()
}

func (j *retentionRunSequenceJobs) ClaimReadyRetentionJob(_ context.Context, lease time.Duration) (*RetentionClaim, error) {
	j.claimCalls++
	j.leaseRequests = append(j.leaseRequests, lease)
	if j.claimCalls == 1 {
		return nil, j.firstClaimErr
	}
	return j.claim, nil
}

func (j *retentionRunSequenceJobs) ProcessClaimedRetentionJob(ctx context.Context, claim RetentionClaim, deleteObject func(context.Context) error, retryDelay time.Duration) error {
	j.processCalls++
	j.processedClaim = claim
	j.retryDelay = retryDelay
	if err := deleteObject(ctx); err != nil {
		return err
	}
	if j.afterProcess != nil {
		j.afterProcess()
	}
	return nil
}

func TestRetentionProcessorRunContinuesAfterClaimErrorUntilNextClaimSucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	claimErr := errors.New("synthetic transient claim error")
	claim := retentionClaim(dv.PurposeIdentity)
	claim.Attempt = 2
	jobs := &retentionRunSequenceJobs{
		firstClaimErr: claimErr,
		claim:         claim,
		afterProcess:  cancel,
	}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, 3*time.Second)
	callbackCalls := 0
	runErr := processor.Run(ctx, time.Millisecond, func(err error) {
		callbackCalls++
		if !errors.Is(err, claimErr) {
			t.Errorf("onError(%v); want transient claim error", err)
		}
	})
	if runErr != nil {
		t.Fatalf("Run() error=%v; want graceful nil after second claim cancels context", runErr)
	}
	if callbackCalls != 1 || jobs.claimCalls != 2 || jobs.processCalls != 1 {
		t.Fatalf("error callbacks=%d claims=%d processed=%d; want 1, 2, 1", callbackCalls, jobs.claimCalls, jobs.processCalls)
	}
	for i, lease := range jobs.leaseRequests {
		if lease != time.Minute {
			t.Fatalf("lease request[%d]=%s; want %s", i, lease, time.Minute)
		}
	}
	if len(jobs.leaseRequests) != 2 || jobs.processedClaim != *claim || jobs.retryDelay != 6*time.Second {
		t.Fatalf("lease requests=%v processed claim=%+v retry delay=%s; want two leases, exact claim, and 6s delay", jobs.leaseRequests, jobs.processedClaim, jobs.retryDelay)
	}
	if storage.identity != 1 || storage.space != 0 || storage.identitySubject != claim.SubjectID || storage.identityCase != claim.CaseID || storage.identityFile != claim.FileID {
		t.Fatalf("identity deletes=%d space deletes=%d identity scope=(%q,%q,%q); want one exact-scope identity delete", storage.identity, storage.space, storage.identitySubject, storage.identityCase, storage.identityFile)
	}
}
