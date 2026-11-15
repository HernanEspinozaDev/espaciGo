package documentverificationpg

import (
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorProcessOneRejectsNilContextBeforeClaiming(t *testing.T) {
	jobs := &retentionFake{claim: retentionClaim(dv.PurposeIdentity)}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)

	if err := processor.ProcessOne(nil); !errors.Is(err, dv.ErrInvalid) {
		t.Fatalf("ProcessOne(nil) error=%v; want ErrInvalid", err)
	}
	if jobs.claimed != 0 || jobs.processed != 0 || storage.identity != 0 || storage.space != 0 {
		t.Fatalf("claim lease=%s processed=%d deletes identity=%d space=%d; want no work", jobs.claimed, jobs.processed, storage.identity, storage.space)
	}
}
