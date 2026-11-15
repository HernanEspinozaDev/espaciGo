package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorProcessOneRejectsMissingDependenciesBeforeClaiming(t *testing.T) {
	for _, tc := range []struct {
		name     string
		nilSelf  bool
		nilJobs  bool
		nilStore bool
	}{
		{name: "nil processor", nilSelf: true},
		{name: "nil jobs", nilJobs: true},
		{name: "nil document store", nilStore: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs := &retentionFake{claim: retentionClaim(dv.PurposeIdentity)}
			store := &retentionStoreFake{}
			var jobsPort retentionProcessorJobs = jobs
			var documentPort retentionDocuments = store
			if tc.nilJobs {
				jobsPort = nil
			}
			if tc.nilStore {
				documentPort = nil
			}
			processor := NewRetentionProcessor(jobsPort, documentPort, time.Minute, time.Second)
			if tc.nilSelf {
				processor = nil
			}
			if err := processor.ProcessOne(context.Background()); !errors.Is(err, dv.ErrInvalid) {
				t.Fatalf("ProcessOne() error=%v; want ErrInvalid", err)
			}
			if jobs.claimed != 0 || jobs.processed != 0 || store.identity != 0 || store.space != 0 {
				t.Fatalf("claim lease=%s processed=%d deletes identity=%d space=%d; want no work", jobs.claimed, jobs.processed, store.identity, store.space)
			}
		})
	}
}
