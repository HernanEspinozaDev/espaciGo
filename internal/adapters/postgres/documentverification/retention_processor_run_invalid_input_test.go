package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorRunRejectsInvalidInputBeforeClaiming(t *testing.T) {
	for _, tc := range []struct {
		name         string
		ctx          context.Context
		poll         time.Duration
		nilProcessor bool
		nilOnError   bool
	}{
		{name: "nil context", ctx: nil, poll: time.Second},
		{name: "nil processor", ctx: context.Background(), poll: time.Second, nilProcessor: true},
		{name: "zero poll interval", ctx: context.Background(), poll: 0},
		{name: "negative poll interval", ctx: context.Background(), poll: -time.Second},
		{name: "nil error callback", ctx: context.Background(), poll: time.Second, nilOnError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs := &retentionFake{claim: retentionClaim(dv.PurposeIdentity)}
			storage := &retentionStoreFake{}
			processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
			if tc.nilProcessor {
				processor = nil
			}
			callbackCalls := 0
			onError := func(error) { callbackCalls++ }
			if tc.nilOnError {
				onError = nil
			}
			if err := processor.Run(tc.ctx, tc.poll, onError); !errors.Is(err, dv.ErrInvalid) {
				t.Fatalf("Run() error=%v; want ErrInvalid", err)
			}
			if jobs.claimed != 0 || jobs.processed != 0 || callbackCalls != 0 {
				t.Fatalf("claim lease=%s processed=%d error callbacks=%d; want no worker activity", jobs.claimed, jobs.processed, callbackCalls)
			}
		})
	}
}
