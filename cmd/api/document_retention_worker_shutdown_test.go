package main

import (
	"context"
	"testing"
	"time"
)

type retentionWorkerGracefulStopFake struct {
	called bool
	ctxErr error
	poll   time.Duration
}

func (f *retentionWorkerGracefulStopFake) Run(ctx context.Context, poll time.Duration, _ func(error)) error {
	f.called = true
	f.ctxErr = ctx.Err()
	f.poll = poll
	return nil
}

func TestDocumentRetentionWorkerDoesNotLogOnGracefulStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := &retentionWorkerGracefulStopFake{}
	var logs []string
	runDocumentRetentionWorker(ctx, worker, func(message string) {
		logs = append(logs, message)
	})
	if !worker.called || worker.ctxErr != context.Canceled || worker.poll != time.Second {
		t.Fatalf("runner called=%t context error=%v poll=%s; want canceled context and one-second interval", worker.called, worker.ctxErr, worker.poll)
	}
	if len(logs) != 0 {
		t.Fatalf("logs=%q; want no failure/stop message for graceful cancellation", logs)
	}
}
