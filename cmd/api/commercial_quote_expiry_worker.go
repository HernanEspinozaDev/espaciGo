package main

import (
	"context"
	"errors"
	"time"
)

type commercialQuoteExpiryPurger interface {
	PurgeExpiredCommercialQuotes(context.Context, time.Time, int) (int64, error)
}

func startCommercialQuoteExpiryWorker(group *trackedWorkerGroup, purger commercialQuoteExpiryPurger, interval time.Duration, limit int, now func() time.Time, onError func(error), onStopped func()) bool {
	return group.Go(func(ctx context.Context) {
		if err := runCommercialQuoteExpiryWorker(ctx, purger, interval, limit, now, onError); err != nil && onStopped != nil {
			onStopped()
		}
	})
}

func runCommercialQuoteExpiryWorker(ctx context.Context, purger commercialQuoteExpiryPurger, interval time.Duration, limit int, now func() time.Time, onError func(error)) error {
	if ctx == nil || purger == nil || interval <= 0 || limit < 1 || limit > 500 || now == nil {
		return errors.New("invalid commercial quote expiry worker configuration")
	}
	purge := func() {
		if _, err := purger.PurgeExpiredCommercialQuotes(ctx, now().UTC(), limit); err != nil && onError != nil {
			onError(err)
		}
	}
	purge()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			purge()
		}
	}
}
