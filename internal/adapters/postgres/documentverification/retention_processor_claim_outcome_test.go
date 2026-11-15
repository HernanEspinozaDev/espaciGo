package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetentionProcessorProcessOneStopsWhenClaimHasNoWorkOrFails(t *testing.T) {
	claimErr := errors.New("synthetic retention claim failure")
	for _, tc := range []struct {
		name     string
		claimErr error
	}{
		{name: "no work"},
		{name: "claim failure", claimErr: claimErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs := &retentionFake{claimErr: tc.claimErr}
			storage := &retentionStoreFake{}
			processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
			err := processor.ProcessOne(context.Background())
			if tc.claimErr == nil {
				if err != nil {
					t.Fatalf("ProcessOne() error=%v; want nil when claim has no work", err)
				}
			} else if !errors.Is(err, tc.claimErr) {
				t.Fatalf("ProcessOne() error=%v; want claim error %v", err, tc.claimErr)
			}
			if jobs.claimed != time.Minute || jobs.processed != 0 || jobs.complete != 0 || jobs.retry != 0 || storage.identity != 0 || storage.space != 0 {
				t.Fatalf("claim lease=%s processed=%d complete=%d retry=%d deletes identity=%d space=%d; want no processing or deletion", jobs.claimed, jobs.processed, jobs.complete, jobs.retry, storage.identity, storage.space)
			}
		})
	}
}
