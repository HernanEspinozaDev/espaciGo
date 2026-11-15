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

type retentionEncryptedBlobs struct {
	objects    map[string][]byte
	putID      string
	deleteID   string
	deleteCall int
}

func (b *retentionEncryptedBlobs) Put(ctx context.Context, id string, envelope []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.objects == nil {
		b.objects = make(map[string][]byte)
	}
	if _, exists := b.objects[id]; exists {
		return errors.New("object already exists")
	}
	b.objects[id] = append([]byte(nil), envelope...)
	b.putID = id
	return nil
}

func (b *retentionEncryptedBlobs) Get(ctx context.Context, id string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	envelope, ok := b.objects[id]
	if !ok {
		return nil, errors.New("object missing")
	}
	return append([]byte(nil), envelope...), nil
}

func (b *retentionEncryptedBlobs) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.deleteCall++
	b.deleteID = id
	delete(b.objects, id)
	return nil
}

func TestRetentionProcessorDeletesEncryptedDocumentsInTheirPurposeScope(t *testing.T) {
	for _, tc := range []struct {
		purpose dv.Purpose
		spaceID string
	}{
		{purpose: dv.PurposeIdentity},
		{purpose: dv.PurposeSpace, spaceID: "44444444-4444-4444-8444-444444444444"},
	} {
		t.Run(string(tc.purpose), func(t *testing.T) {
			const (
				ownerID = "11111111-1111-4111-8111-111111111111"
				caseID  = "22222222-2222-4222-8222-222222222222"
				fileID  = "33333333-3333-4333-8333-333333333333"
			)
			blobs := &retentionEncryptedBlobs{}
			store, err := encryptedstore.New(blobs, []byte("0123456789abcdef0123456789abcdef"))
			if err != nil {
				t.Fatal(err)
			}
			content := []byte("synthetic private evidence")
			if tc.purpose == dv.PurposeIdentity {
				err = store.PutIdentityDocument(context.Background(), ownerID, caseID, fileID, content)
			} else {
				err = store.PutSpaceDocument(context.Background(), ownerID, tc.spaceID, caseID, fileID, content)
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(blobs.objects) != 1 || blobs.putID == "" {
				t.Fatalf("encrypted put objects=%d objectID=%q", len(blobs.objects), blobs.putID)
			}
			if bytes.Equal(blobs.objects[blobs.putID], content) {
				t.Fatal("retention BlobStore fake received plaintext instead of an encrypted envelope")
			}

			claim := retentionClaim(tc.purpose)
			claim.SubjectID, claim.CaseID, claim.FileID, claim.SpaceID = ownerID, caseID, fileID, tc.spaceID
			jobs := &retentionFake{claim: claim}
			processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)
			if err := processor.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}

			if jobs.processed != 1 || jobs.complete != 1 || jobs.retry != 0 {
				t.Fatalf("retention outcome: processed=%d complete=%d retry=%d", jobs.processed, jobs.complete, jobs.retry)
			}
			if blobs.deleteCall != 1 || blobs.deleteID != blobs.putID || len(blobs.objects) != 0 {
				t.Fatalf("encrypted object not deleted by exact scope: putID=%q deleteID=%q calls=%d remaining=%d", blobs.putID, blobs.deleteID, blobs.deleteCall, len(blobs.objects))
			}
		})
	}
}
