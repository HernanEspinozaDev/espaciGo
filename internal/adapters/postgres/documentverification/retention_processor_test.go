package documentverificationpg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
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
	claimErr        error
	err             error
	afterClaim      func()
}

func (f *retentionFake) ClaimReadyRetentionJob(_ context.Context, d time.Duration) (*RetentionClaim, error) {
	f.claimed = d
	if f.afterClaim != nil {
		f.afterClaim()
	}
	return f.claim, f.claimErr
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
	identity, space                             int
	err                                         error
	identitySubject, identityCase, identityFile string
	spaceSubject, spaceID, spaceCase, spaceFile string
}

func (f *retentionStoreFake) DeleteIdentityDocument(_ context.Context, subjectID, caseID, fileID string) error {
	f.identity++
	f.identitySubject, f.identityCase, f.identityFile = subjectID, caseID, fileID
	return f.err
}
func (f *retentionStoreFake) DeleteSpaceDocument(_ context.Context, subjectID, spaceID, caseID, fileID string) error {
	f.space++
	f.spaceSubject, f.spaceID, f.spaceCase, f.spaceFile = subjectID, spaceID, caseID, fileID
	return f.err
}
func retentionClaim(p dv.Purpose) *RetentionClaim {
	return &RetentionClaim{JobID: "j", FileID: "f", CaseID: "c", SubjectID: "o", Purpose: p, SpaceID: "s", LeaseUntil: time.Now().Add(time.Minute)}
}
func TestRetentionProcessorRoutesIdentityAndSpaceDeletes(t *testing.T) {
	for _, tc := range []struct {
		purpose                            dv.Purpose
		subjectID, spaceID, caseID, fileID string
	}{
		{dv.PurposeIdentity, "identity-subject", "identity-space-unused", "identity-case", "identity-file"},
		{dv.PurposeSpace, "space-subject", "space-scope", "space-case", "space-file"},
	} {
		claim := retentionClaim(tc.purpose)
		claim.SubjectID, claim.SpaceID, claim.CaseID, claim.FileID = tc.subjectID, tc.spaceID, tc.caseID, tc.fileID
		j := &retentionFake{claim: claim}
		s := &retentionStoreFake{}
		proc := NewRetentionProcessor(j, s, time.Minute, time.Second)
		if err := proc.ProcessOne(context.Background()); err != nil {
			t.Fatal(err)
		}
		if j.complete != 1 || s.identity != boolInt(tc.purpose == dv.PurposeIdentity) || s.space != boolInt(tc.purpose == dv.PurposeSpace) {
			t.Fatalf("purpose=%s jobs=%+v store=%+v", tc.purpose, j, s)
		}
		if tc.purpose == dv.PurposeIdentity {
			if s.identitySubject != tc.subjectID || s.identityCase != tc.caseID || s.identityFile != tc.fileID || s.spaceSubject != "" {
				t.Fatalf("identity delete scope=(%q,%q,%q), space call subject=%q", s.identitySubject, s.identityCase, s.identityFile, s.spaceSubject)
			}
		} else if s.spaceSubject != tc.subjectID || s.spaceID != tc.spaceID || s.spaceCase != tc.caseID || s.spaceFile != tc.fileID || s.identitySubject != "" {
			t.Fatalf("space delete scope=(%q,%q,%q,%q), identity call subject=%q", s.spaceSubject, s.spaceID, s.spaceCase, s.spaceFile, s.identitySubject)
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

func TestRetentionProcessorRejectsUnknownPurposeWithoutDeleting(t *testing.T) {
	claim := retentionClaim(dv.Purpose("unknown-purpose"))
	jobs := &retentionFake{claim: claim}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)

	if err := processor.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.processed != 1 || jobs.retry != 1 || jobs.complete != 0 || jobs.retryCode != retentionStoreError {
		t.Fatalf("processed=%d retry=%d complete=%d retry code=%q", jobs.processed, jobs.retry, jobs.complete, jobs.retryCode)
	}
	if storage.identity != 0 || storage.space != 0 {
		t.Fatalf("unknown purpose deleted objects: identity=%d space=%d", storage.identity, storage.space)
	}
}

func TestRetentionProcessorRejectsSpaceClaimWithoutScope(t *testing.T) {
	claim := retentionClaim(dv.PurposeSpace)
	claim.SpaceID = ""
	jobs := &retentionFake{claim: claim}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)

	if err := processor.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.processed != 1 || jobs.retry != 1 || jobs.complete != 0 || jobs.retryCode != retentionStoreError {
		t.Fatalf("processed=%d retry=%d complete=%d retry code=%q", jobs.processed, jobs.retry, jobs.complete, jobs.retryCode)
	}
	if storage.identity != 0 || storage.space != 0 {
		t.Fatalf("claim without space scope deleted objects: identity=%d space=%d", storage.identity, storage.space)
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

func TestRetentionProcessorRetriesWhenPrivateStoreRejectsUnsafeScope(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		jobID     = "11111111-1111-4111-8111-111111111111"
		fileID    = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
		subjectID = "44444444-4444-4444-8444-444444444444"
	)
	ctx := context.Background()
	content := []byte("synthetic identity evidence")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, content); err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(root, "users", subjectID, "identity", caseID, fileID+".bin")
	before, err := os.ReadFile(objectPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(objectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	claim := retentionClaim(dv.PurposeIdentity)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID = jobID, fileID, caseID, subjectID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.processed != 1 || jobs.retry != 1 || jobs.complete != 0 || jobs.retryCode != retentionStoreError || jobs.retryDelay != time.Second {
		t.Fatalf("processed=%d retry=%d complete=%d retry code=%q delay=%s", jobs.processed, jobs.retry, jobs.complete, jobs.retryCode, jobs.retryDelay)
	}
	dirInfo, err := os.Stat(filepath.Dir(objectPath))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o755 {
		t.Fatalf("unsafe scope directory mode after rejected delete=%v, want 0755", dirInfo.Mode().Perm())
	}
	after, err := os.ReadFile(objectPath)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("stored envelope after rejected deletion changed: equal=%t err=%v", bytes.Equal(after, before), err)
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

func TestRetentionProcessorDeletesIdentityFromPrivateStore(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		jobID     = "11111111-1111-4111-8111-111111111111"
		fileID    = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
		subjectID = "44444444-4444-4444-8444-444444444444"
		spaceID   = "55555555-5555-4555-8555-555555555555"
	)
	ctx := context.Background()
	identityBytes := []byte("synthetic identity evidence")
	spaceBytes := []byte("synthetic space document")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, identityBytes); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSpaceDocument(ctx, subjectID, spaceID, caseID, fileID, spaceBytes); err != nil {
		t.Fatal(err)
	}
	claim := retentionClaim(dv.PurposeIdentity)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID, claim.SpaceID = jobID, fileID, caseID, subjectID, spaceID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("fake completion=%d retry=%d", jobs.complete, jobs.retry)
	}
	if _, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity document after retention process error=%v, want not-exist", err)
	}
	identityPath := filepath.Join(root, "users", subjectID, "identity", caseID, fileID+".bin")
	if _, err := os.Lstat(identityPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity object at %q after retention process error=%v, want physically absent", identityPath, err)
	}
	got, err := store.ReadSpaceDocument(ctx, subjectID, spaceID, caseID, fileID)
	if err != nil || !bytes.Equal(got, spaceBytes) {
		t.Fatalf("space document after identity retention delete=%q, err=%v", got, err)
	}
}

func TestRetentionProcessorDeletesSpaceFromPrivateStore(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		jobID     = "11111111-1111-4111-8111-111111111111"
		fileID    = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
		subjectID = "44444444-4444-4444-8444-444444444444"
		spaceID   = "55555555-5555-4555-8555-555555555555"
	)
	ctx := context.Background()
	identityBytes := []byte("synthetic identity evidence")
	spaceBytes := []byte("synthetic space document")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, identityBytes); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSpaceDocument(ctx, subjectID, spaceID, caseID, fileID, spaceBytes); err != nil {
		t.Fatal(err)
	}
	claim := retentionClaim(dv.PurposeSpace)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID, claim.SpaceID = jobID, fileID, caseID, subjectID, spaceID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("fake completion=%d retry=%d", jobs.complete, jobs.retry)
	}
	if _, err := store.ReadSpaceDocument(ctx, subjectID, spaceID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("space document after retention process error=%v, want not-exist", err)
	}
	spacePath := filepath.Join(root, "users", subjectID, "spaces", spaceID, "documents", caseID, fileID+".bin")
	if _, err := os.Lstat(spacePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("space object at %q after retention process error=%v, want physically absent", spacePath, err)
	}
	got, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID)
	if err != nil || !bytes.Equal(got, identityBytes) {
		t.Fatalf("identity document after space retention delete=%q, err=%v", got, err)
	}
}

func TestRetentionProcessorMissingSpaceObjectIsSuccessWithoutDeletingIdentity(t *testing.T) {
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		fileID    = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
		subjectID = "44444444-4444-4444-8444-444444444444"
		spaceID   = "55555555-5555-4555-8555-555555555555"
	)
	ctx := context.Background()
	identityBytes := []byte("synthetic identity evidence")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, identityBytes); err != nil {
		t.Fatal(err)
	}
	claim := retentionClaim(dv.PurposeSpace)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID, claim.SpaceID = "11111111-1111-4111-8111-111111111111", fileID, caseID, subjectID, spaceID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.processed != 1 || jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("processed=%d complete=%d retry=%d", jobs.processed, jobs.complete, jobs.retry)
	}
	if _, err := store.ReadSpaceDocument(ctx, subjectID, spaceID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing space object read error=%v, want not-exist", err)
	}
	got, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID)
	if err != nil || !bytes.Equal(got, identityBytes) {
		t.Fatalf("identity object after missing space deletion=%q err=%v", got, err)
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

func TestRetentionProcessorRunReportsClaimFailureAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	claimErr := errors.New("synthetic claim failure")
	jobs := &retentionFake{claimErr: claimErr}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)
	callbackCount := 0
	err := processor.Run(ctx, time.Hour, func(err error) {
		callbackCount++
		if !errors.Is(err, claimErr) {
			t.Errorf("onError got %v, want claim error", err)
		}
		cancel()
	})
	if err != nil || callbackCount != 1 || jobs.processed != 0 || storage.identity != 0 || storage.space != 0 {
		t.Fatalf("Run err=%v callback=%d processed=%d identity=%d space=%d", err, callbackCount, jobs.processed, storage.identity, storage.space)
	}
}

func TestRetentionProcessorCancellationAfterClaimSkipsDeletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs := &retentionFake{claim: retentionClaim(dv.PurposeIdentity), afterClaim: cancel}
	storage := &retentionStoreFake{}
	processor := NewRetentionProcessor(jobs, storage, time.Minute, time.Second)

	err := processor.ProcessOne(ctx)
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("claim fake did not cancel the context: %v", ctx.Err())
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want context cancellation", err)
	}
	if jobs.claimed != time.Minute || jobs.processed != 0 || jobs.complete != 0 || jobs.retry != 0 {
		t.Fatalf("claimed=%s processed=%d complete=%d retry=%d", jobs.claimed, jobs.processed, jobs.complete, jobs.retry)
	}
	if storage.identity != 0 || storage.space != 0 {
		t.Fatalf("canceled claim deleted objects: identity=%d space=%d", storage.identity, storage.space)
	}
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
