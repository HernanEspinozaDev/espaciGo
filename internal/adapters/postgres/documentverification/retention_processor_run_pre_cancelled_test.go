package documentverificationpg

import (
	"context"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorRunStopsGracefullyWhenAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs := &retentionFake{claim: retentionClaim(dv.PurposeIdentity)}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
	callbackCalls := 0

	if err := processor.Run(ctx, time.Second, func(error) { callbackCalls++ }); err != nil {
		t.Fatalf("Run() error=%v; want graceful nil for canceled context", err)
	}
	if jobs.claimed != 0 || jobs.processed != 0 || callbackCalls != 0 || storage.identity != 0 || storage.space != 0 {
		t.Fatalf("claim lease=%s processed=%d error callbacks=%d deletes identity=%d space=%d; want no work or error log", jobs.claimed, jobs.processed, callbackCalls, storage.identity, storage.space)
	}
}
