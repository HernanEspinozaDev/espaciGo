package documentverificationpg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"testing"
	"time"
)

type retentionFake struct {
	claim           *RetentionClaim
	claimed         time.Duration
	complete, retry int
	processed       int
	retryDelay      time.Duration
	retryCode       string
	err             error
}

func (f *retentionFake) ClaimReadyRetentionJob(_ context.Context, d time.Duration) (*RetentionClaim, error) {
	f.claimed = d
	return f.claim, nil
}
func (f *retentionFake) ProcessClaimedRetentionJob(ctx context.Context, _ RetentionClaim, deleteObject func(context.Context) error, retryDelay time.Duration) error {
	f.processed++
	if err := deleteObject(ctx); err != nil {
		f.retry++
		f.retryCode = retentionStoreError
		f.retryDelay = retryDelay
		return f.err
	}
	f.complete++
	return f.err
}

type retentionStoreFake struct {
	identity, space int
	err             error
}

func (f *retentionStoreFake) DeleteIdentityDocument(context.Context, string, string, string) error {
	f.identity++
	return f.err
}
func (f *retentionStoreFake) DeleteSpaceDocument(context.Context, string, string, string, string) error {
	f.space++
	return f.err
}
func retentionClaim(p dv.Purpose) *RetentionClaim {
	return &RetentionClaim{JobID: "j", FileID: "f", CaseID: "c", SubjectID: "o", Purpose: p, SpaceID: "s", LeaseUntil: time.Now().Add(time.Minute)}
}
func TestRetentionProcessorRoutesIdentityAndSpaceDeletes(t *testing.T) {
	for _, p := range []dv.Purpose{dv.PurposeIdentity, dv.PurposeSpace} {
		j := &retentionFake{claim: retentionClaim(p)}
		s := &retentionStoreFake{}
		proc := NewRetentionProcessor(j, s, time.Minute, time.Second)
		if err := proc.ProcessOne(context.Background()); err != nil {
			t.Fatal(err)
		}
		if j.complete != 1 || s.identity != boolInt(p == dv.PurposeIdentity) || s.space != boolInt(p == dv.PurposeSpace) {
			t.Fatalf("purpose=%s jobs=%+v store=%+v", p, j, s)
		}
	}
}

func TestRetentionProcessorUsesFencedDeletionOperation(t *testing.T) {
	jobs := &retentionFake{claim: retentionClaim(dv.PurposeSpace)}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
	if err := processor.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.processed != 1 || jobs.complete != 1 || jobs.retry != 0 || storage.space != 1 {
		t.Fatalf("fenced processing=%d complete=%d retry=%d space deletes=%d", jobs.processed, jobs.complete, jobs.retry, storage.space)
	}
}

func TestRetentionProcessorRetriesStorageFailure(t *testing.T) {
	j := &retentionFake{claim: retentionClaim(dv.PurposeIdentity)}
	s := &retentionStoreFake{err: errors.New("disk")}
	if err := NewRetentionProcessor(j, s, time.Minute, time.Second).ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if j.retry != 1 || j.complete != 0 {
		t.Fatalf("retry=%d complete=%d", j.retry, j.complete)
	}
}

func TestRetentionProcessorUsesCappedExponentialRetryDelay(t *testing.T) {
	for _, tc := range []struct {
		attempt int16
		base    time.Duration
		want    time.Duration
	}{
		{attempt: 1, base: 30 * time.Second, want: 30 * time.Second},
		{attempt: 2, base: 30 * time.Second, want: 60 * time.Second},
		{attempt: 5, base: 30 * time.Second, want: 8 * time.Minute},
		{attempt: 5, base: 2 * time.Hour, want: 24 * time.Hour},
	} {
		t.Run(fmt.Sprintf("attempt_%d_base_%s", tc.attempt, tc.base), func(t *testing.T) {
			claim := retentionClaim(dv.PurposeIdentity)
			claim.Attempt = tc.attempt
			jobs := &retentionFake{claim: claim}
			storage := &retentionStoreFake{err: errors.New("disk")}
			processor := NewRetentionProcessor(jobs, storage, time.Minute, tc.base)
			if err := processor.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
			if jobs.retry != 1 || jobs.retryCode != retentionStoreError || jobs.retryDelay != tc.want {
				t.Fatalf("retry count=%d code=%q delay=%s, want one %q retry at %s", jobs.retry, jobs.retryCode, jobs.retryDelay, retentionStoreError, tc.want)
			}
		})
	}
}
func TestRetentionProcessorMissingObjectIsSuccess(t *testing.T) {
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	claim := retentionClaim(dv.PurposeIdentity)
	claim.JobID = "11111111-1111-4111-8111-111111111111"
	claim.FileID = "22222222-2222-4222-8222-222222222222"
	claim.CaseID = "33333333-3333-4333-8333-333333333333"
	claim.SubjectID = "44444444-4444-4444-8444-444444444444"
	j := &retentionFake{claim: claim}
	if err := NewRetentionProcessor(j, store, time.Minute, time.Second).ProcessOne(context.Background()); err != nil || j.complete != 1 || j.retry != 0 {
		t.Fatalf("complete=%d err=%v", j.complete, err)
	}
}
func TestRetentionProcessorCancellationAndIdle(t *testing.T) {
	j := &retentionFake{}
	s := &retentionStoreFake{}
	p := NewRetentionProcessor(j, s, time.Minute, time.Second)
	if err := p.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.ProcessOne(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
