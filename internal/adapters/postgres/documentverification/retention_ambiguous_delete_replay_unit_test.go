package documentverificationpg

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	encryptedstore "github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentstorage"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type ambiguousDeleteBlobStore struct {
	objects    map[string][]byte
	putID      string
	deleteID   string
	deleteCall int
	ambiguous  error
}

func (b *ambiguousDeleteBlobStore) Put(ctx context.Context, id string, envelope []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.objects == nil {
		b.objects = make(map[string][]byte)
	}
	if _, exists := b.objects[id]; exists {
		return errors.New("synthetic object already exists")
	}
	b.objects[id] = append([]byte(nil), envelope...)
	b.putID = id
	return nil
}

func (b *ambiguousDeleteBlobStore) Get(ctx context.Context, id string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	envelope, exists := b.objects[id]
	if !exists {
		return nil, errors.New("synthetic object missing")
	}
	return append([]byte(nil), envelope...), nil
}

func (b *ambiguousDeleteBlobStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.deleteCall++
	b.deleteID = id
	delete(b.objects, id)
	if b.deleteCall == 1 {
		return b.ambiguous
	}
	return nil
}

type ambiguousRetentionReplayJobs struct {
	claims    []RetentionClaim
	txs       []*retentionFencingUnitTx
	nextClaim int
	processed int
}

func (j *ambiguousRetentionReplayJobs) ClaimReadyRetentionJob(context.Context, time.Duration) (*RetentionClaim, error) {
	if j.nextClaim >= len(j.claims) {
		return nil, nil
	}
	claim := j.claims[j.nextClaim]
	j.nextClaim++
	return &claim, nil
}

func (j *ambiguousRetentionReplayJobs) ProcessClaimedRetentionJob(ctx context.Context, claim RetentionClaim, deleteObject func(context.Context) error, retry time.Duration) error {
	if j.processed >= len(j.txs) {
		return errors.New("unexpected synthetic retention attempt")
	}
	tx := j.txs[j.processed]
	j.processed++
	repository := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
	return repository.ProcessClaimedRetentionJob(ctx, claim, deleteObject, retry)
}

func TestRetentionProcessorReplaysAmbiguousEncryptedDeleteAsIdempotent(t *testing.T) {
	const (
		jobID   = "11111111-1111-4111-8111-111111111111"
		ownerID = "22222222-2222-4222-8222-222222222222"
		caseID  = "33333333-3333-4333-8333-333333333333"
		fileID  = "44444444-4444-4444-8444-444444444444"
	)
	ctx := context.Background()
	content := []byte("synthetic private identity evidence")
	blobs := &ambiguousDeleteBlobStore{ambiguous: errors.New("synthetic ambiguous delete response")}
	store, err := encryptedstore.New(blobs, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(ctx, ownerID, caseID, fileID, content); err != nil {
		t.Fatalf("store encrypted identity evidence: %v", err)
	}
	if blobs.putID == "" || bytes.Equal(blobs.objects[blobs.putID], content) {
		t.Fatal("blob store did not receive the encrypted envelope")
	}

	claim := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: ownerID,
		Purpose: dv.PurposeIdentity, Attempt: 3,
		LeaseUntil: time.Date(2031, 4, 5, 12, 0, 0, 0, time.UTC),
	}
	firstTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	retryClaim := claim
	retryClaim.Attempt++
	secondTx := &retentionFencingUnitTx{claim: retryClaim, leaseMatches: true, leaseLive: true}
	jobs := &ambiguousRetentionReplayJobs{
		claims: []RetentionClaim{claim, retryClaim},
		txs:    []*retentionFencingUnitTx{firstTx, secondTx},
	}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("first retention attempt: %v", err)
	}
	if firstTx.execs == nil || len(firstTx.execs) != 1 || firstTx.execs[0] != "retry" || !firstTx.committed {
		t.Fatalf("ambiguous delete did not schedule a committed retry: execs=%v committed=%t", firstTx.execs, firstTx.committed)
	}
	if len(blobs.objects) != 0 || blobs.deleteCall != 1 || blobs.deleteID != blobs.putID {
		t.Fatalf("first delete state: objects=%d calls=%d putID=%q deleteID=%q", len(blobs.objects), blobs.deleteCall, blobs.putID, blobs.deleteID)
	}
	if len(firstTx.execArgs) != 1 || len(firstTx.execArgs[0]) != 5 ||
		firstTx.execArgs[0][0] != jobID || firstTx.execArgs[0][1] != fileID ||
		firstTx.execArgs[0][2] != retentionRetryDelay(time.Second, claim.Attempt).String() ||
		firstTx.execArgs[0][3] != retentionStoreError || firstTx.execArgs[0][4] != claim.LeaseUntil {
		t.Fatalf("retry arguments=%#v, want exact claim, bounded delay, sanitized code and lease", firstTx.execArgs)
	}

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("retention replay after object was already deleted: %v", err)
	}
	if !secondTx.committed || len(secondTx.execs) != 2 || secondTx.execs[0] != "complete" || secondTx.execs[1] != "audit" {
		t.Fatalf("replay did not complete and audit the job: execs=%v committed=%t", secondTx.execs, secondTx.committed)
	}
	if len(blobs.objects) != 0 || blobs.deleteCall != 2 || blobs.deleteID != blobs.putID || jobs.processed != 2 {
		t.Fatalf("replay state: objects=%d deleteCalls=%d putID=%q deleteID=%q processed=%d", len(blobs.objects), blobs.deleteCall, blobs.putID, blobs.deleteID, jobs.processed)
	}
}
