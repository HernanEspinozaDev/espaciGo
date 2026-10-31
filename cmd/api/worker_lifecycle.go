package main

import (
	"context"
	"sync"
)

type trackedWorkerGroup struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	stopped bool
	workers sync.WaitGroup
}

func newTrackedWorkerGroup(parent context.Context) *trackedWorkerGroup {
	ctx, cancel := context.WithCancel(parent)
	return &trackedWorkerGroup{ctx: ctx, cancel: cancel}
}

func (g *trackedWorkerGroup) Go(run func(context.Context)) bool {
	if g == nil || run == nil {
		return false
	}
	g.mu.Lock()
	if g.stopped {
		g.mu.Unlock()
		return false
	}
	g.workers.Add(1)
	g.mu.Unlock()

	go func() {
		defer g.workers.Done()
		run(g.ctx)
	}()
	return true
}

func (g *trackedWorkerGroup) StopAndWait() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.stopped = true
	g.cancel()
	g.mu.Unlock()
	g.workers.Wait()
}
