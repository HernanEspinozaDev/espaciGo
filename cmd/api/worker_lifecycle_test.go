package main

import (
	"context"
	"testing"
)

func TestTrackedWorkerGroupCancelsAndJoinsWorkers(t *testing.T) {
	group := newTrackedWorkerGroup(context.Background())
	started := make(chan struct{})
	finished := make(chan struct{})
	group.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(finished)
	})
	<-started

	group.StopAndWait()

	select {
	case <-finished:
	default:
		t.Fatal("StopAndWait returned before the worker exited")
	}
}
