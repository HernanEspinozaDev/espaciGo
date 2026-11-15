package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type commercialQuoteExpiryPurgerStub struct {
	calls  int
	at     time.Time
	limit  int
	cancel context.CancelFunc
	err    error
}

type blockingCommercialQuoteExpiryPurger struct {
	started  chan struct{}
	finished chan struct{}
}

func (p blockingCommercialQuoteExpiryPurger) PurgeExpiredCommercialQuotes(ctx context.Context, _ time.Time, _ int) (int64, error) {
	close(p.started)
	<-ctx.Done()
	close(p.finished)
	return 0, ctx.Err()
}

func (s *commercialQuoteExpiryPurgerStub) PurgeExpiredCommercialQuotes(_ context.Context, at time.Time, limit int) (int64, error) {
	s.calls++
	s.at, s.limit = at, limit
	if s.cancel != nil {
		s.cancel()
	}
	return 0, s.err
}

func TestCommercialQuoteExpiryWorkerRunsImmediatelyAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fixed := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("test", -3*60*60))
	wantErr := errors.New("synthetic purge error")
	purger := &commercialQuoteExpiryPurgerStub{cancel: cancel, err: wantErr}
	var reported []error
	err := runCommercialQuoteExpiryWorker(ctx, purger, time.Hour, 100, func() time.Time { return fixed }, func(err error) { reported = append(reported, err) })
	if err != nil {
		t.Fatal(err)
	}
	if purger.calls != 1 || !purger.at.Equal(fixed.UTC()) || purger.limit != 100 {
		t.Fatalf("purge calls=%d at=%s limit=%d", purger.calls, purger.at, purger.limit)
	}
	if len(reported) != 1 || !errors.Is(reported[0], wantErr) {
		t.Fatalf("reported errors=%v, want the synthetic database error", reported)
	}
}

func TestCommercialQuoteExpiryWorkerLifecycleWaitsForActivePurge(t *testing.T) {
	group := newTrackedWorkerGroup(context.Background())
	purger := blockingCommercialQuoteExpiryPurger{
		started: make(chan struct{}), finished: make(chan struct{}),
	}
	if !startCommercialQuoteExpiryWorker(group, purger, time.Hour, 100, time.Now, nil, nil) {
		t.Fatal("expected expiry worker to start")
	}
	<-purger.started

	group.StopAndWait()

	select {
	case <-purger.finished:
	default:
		t.Fatal("worker group returned before the active quote purge finished")
	}
}

func TestCommercialQuoteExpiryWorkerRejectsInvalidConfiguration(t *testing.T) {
	purger := &commercialQuoteExpiryPurgerStub{}
	cases := []struct {
		name     string
		ctx      context.Context
		purger   commercialQuoteExpiryPurger
		interval time.Duration
		limit    int
		now      func() time.Time
	}{
		{name: "nil context", purger: purger, interval: time.Minute, limit: 1, now: time.Now},
		{name: "nil purger", ctx: context.Background(), interval: time.Minute, limit: 1, now: time.Now},
		{name: "zero interval", ctx: context.Background(), purger: purger, limit: 1, now: time.Now},
		{name: "oversized batch", ctx: context.Background(), purger: purger, interval: time.Minute, limit: 501, now: time.Now},
		{name: "nil clock", ctx: context.Background(), purger: purger, interval: time.Minute, limit: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := runCommercialQuoteExpiryWorker(test.ctx, test.purger, test.interval, test.limit, test.now, nil); err == nil {
				t.Fatal("expected invalid worker configuration error")
			}
		})
	}
}
