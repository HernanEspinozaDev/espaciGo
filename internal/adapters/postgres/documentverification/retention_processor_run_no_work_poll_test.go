package documentverificationpg

import (
	"context"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type retentionRunNoWorkThenClaimJobs struct {
	claim         *RetentionClaim
	claimCalls    int
	leaseRequests []time.Duration
	processCalls  int
	afterProcess  func()
}

func (j *retentionRunNoWorkThenClaimJobs) ClaimReadyRetentionJob(_ context.Context, lease time.Duration) (*RetentionClaim, error) {
	j.claimCalls++
	j.leaseRequests = append(j.leaseRequests, lease)
	if j.claimCalls == 1 {
		return nil, nil
	}
	return j.claim, nil
}

func (j *retentionRunNoWorkThenClaimJobs) ProcessClaimedRetentionJob(ctx context.Context, _ RetentionClaim, deleteObject func(context.Context) error, _ time.Duration) error {
	j.processCalls++
	if err := deleteObject(ctx); err != nil {
		return err
	}
	if j.afterProcess != nil {
		j.afterProcess()
	}
	return nil
}

func TestRetentionProcessorRunPollsAgainAfterNoWork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	claim := retentionClaim(dv.PurposeIdentity)
	jobs := &retentionRunNoWorkThenClaimJobs{claim: claim, afterProcess: cancel}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
	errorCallbacks := 0

	if err := processor.Run(ctx, time.Millisecond, func(error) { errorCallbacks++ }); err != nil {
		t.Fatalf("Run() error=%v; want graceful nil after later claim cancels context", err)
	}
	if jobs.claimCalls != 2 || jobs.processCalls != 1 || errorCallbacks != 0 {
		t.Fatalf("claims=%d processed=%d error callbacks=%d; want 2, 1, 0", jobs.claimCalls, jobs.processCalls, errorCallbacks)
	}
	if len(jobs.leaseRequests) != 2 || jobs.leaseRequests[0] != time.Minute || jobs.leaseRequests[1] != time.Minute {
		t.Fatalf("lease requests=%v; want two configured leases", jobs.leaseRequests)
	}
	if storage.identity != 1 || storage.space != 0 || storage.identitySubject != claim.SubjectID || storage.identityCase != claim.CaseID || storage.identityFile != claim.FileID {
		t.Fatalf("identity deletes=%d space deletes=%d identity scope=(%q,%q,%q); want one exact-scope identity delete", storage.identity, storage.space, storage.identitySubject, storage.identityCase, storage.identityFile)
	}
}
