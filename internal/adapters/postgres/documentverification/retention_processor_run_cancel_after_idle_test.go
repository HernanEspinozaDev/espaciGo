package documentverificationpg

import (
	"context"
	"testing"
	"time"
)

type retentionRunCancelAfterIdleJobs struct {
	claimCalls   int
	processCalls int
	cancel       context.CancelFunc
}

func (j *retentionRunCancelAfterIdleJobs) ClaimReadyRetentionJob(context.Context, time.Duration) (*RetentionClaim, error) {
	j.claimCalls++
	j.cancel()
	return nil, nil
}

func (j *retentionRunCancelAfterIdleJobs) ProcessClaimedRetentionJob(context.Context, RetentionClaim, func(context.Context) error, time.Duration) error {
	j.processCalls++
	return nil
}

func TestRetentionProcessorRunStopsAfterCancellationDuringIdlePass(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	jobs := &retentionRunCancelAfterIdleJobs{cancel: cancel}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
	errorCallbacks := 0

	if err := processor.Run(ctx, time.Hour, func(error) { errorCallbacks++ }); err != nil {
		t.Fatalf("Run() error=%v; want graceful nil after idle-pass cancellation", err)
	}
	if jobs.claimCalls != 1 || jobs.processCalls != 0 || errorCallbacks != 0 || storage.identity != 0 || storage.space != 0 {
		t.Fatalf("claims=%d processed=%d error callbacks=%d deletes identity=%d space=%d; want one idle claim and no further work", jobs.claimCalls, jobs.processCalls, errorCallbacks, storage.identity, storage.space)
	}
}
