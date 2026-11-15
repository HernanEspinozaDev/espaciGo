package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	encryptedstore "github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentstorage"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerEncryptedBlobs struct {
	objects map[string][]byte
	getHook func()
}

func (b *reviewerEncryptedBlobs) Put(_ context.Context, id string, envelope []byte) error {
	if b.objects == nil {
		b.objects = make(map[string][]byte)
	}
	if _, exists := b.objects[id]; exists {
		return errors.New("object already exists")
	}
	b.objects[id] = append([]byte(nil), envelope...)
	return nil
}

func (b *reviewerEncryptedBlobs) Get(_ context.Context, id string) ([]byte, error) {
	if b.getHook != nil {
		b.getHook()
	}
	envelope, ok := b.objects[id]
	if !ok {
		return nil, errors.New("object missing")
	}
	return append([]byte(nil), envelope...), nil
}

func (b *reviewerEncryptedBlobs) Delete(_ context.Context, id string) error {
	delete(b.objects, id)
	return nil
}

func TestReviewerDownloadHTTPUsesEncryptedStoreAndFailsClosedOnCancellation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2031, 4, 5, 12, 0, 0, 0, time.UTC)
	ownerID := "77777777-7777-4777-8777-777777777777"
	content := []byte("synthetic reviewer identity image")
	blobs := &reviewerEncryptedBlobs{}
	store, err := encryptedstore.New(blobs, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(ctx, ownerID, reviewerID, reviewerFile, content); err != nil {
		t.Fatal(err)
	}

	repo := &reviewerDownloadIntegrityRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseInReview, Revision: 4,
			Files: []dv.File{{
				ID: reviewerFile, State: dv.FileClean, Size: int64(len(content)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(content), RetentionAt: now.Add(time.Hour),
			}},
		},
		auth: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
	}
	caseService, err := dv.NewService(reviewerDownloadUnusedRepository{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, caseService, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return now })
	handler := NewReviewerCasesHandler(reviewerAuth{}, reviewerDownloadIntegrityService{access: access})
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, handler)
	path := reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content"

	// Positive control exercises the route, reviewer service, authenticated scope,
	// encrypted adapter, integrity checks and response serialization.
	positive := httptest.NewRecorder()
	mux.ServeHTTP(positive, reviewerReq(http.MethodGet, path, ""))
	if positive.Code != http.StatusOK || !bytes.Equal(positive.Body.Bytes(), content) {
		t.Fatalf("positive download status=%d body=%q", positive.Code, positive.Body.Bytes())
	}
	if positive.Header().Get("Content-Type") != "image/png" || positive.Header().Get("Content-Length") != strconv.Itoa(len(content)) ||
		positive.Header().Get("Content-Disposition") == "" || positive.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("positive download headers=%v", positive.Header())
	}

	readCtx, cancel := context.WithCancel(ctx)
	blobs.getHook = cancel
	request := reviewerReq(http.MethodGet, path, "").WithContext(readCtx)
	canceled := httptest.NewRecorder()
	mux.ServeHTTP(canceled, request)
	if canceled.Code != http.StatusServiceUnavailable {
		t.Fatalf("canceled download status=%d body=%q", canceled.Code, canceled.Body.String())
	}
	if bytes.Contains(canceled.Body.Bytes(), content) || canceled.Header().Get("Content-Disposition") != "" ||
		canceled.Header().Get("Content-Length") != "" || canceled.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("canceled download leaked document response: headers=%v body=%q", canceled.Header(), canceled.Body.Bytes())
	}
	if canceled.Header().Get("Cache-Control") != "private, no-store" || canceled.Header().Get("X-Content-Type-Options") != "nosniff" || canceled.Header().Get("Vary") != "Authorization" {
		t.Fatalf("canceled download security headers=%v", canceled.Header())
	}
}

type reviewerDownloadCancelAfterReadTxRepository struct {
	*reviewerDownloadIntegrityRepository
	cancel context.CancelFunc
}

func (r *reviewerDownloadCancelAfterReadTxRepository) WithReviewerCaseForRead(ctx context.Context, actor, caseID string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	err := r.reviewerDownloadIntegrityRepository.WithReviewerCaseForRead(ctx, actor, caseID, read)
	if err == nil {
		r.cancel()
	}
	return err
}

func TestReviewerDownloadHTTPFailsClosedWhenContextCancelsAfterReadTransaction(t *testing.T) {
	now := time.Date(2031, 4, 5, 12, 0, 0, 0, time.UTC)
	ownerID := "77777777-7777-4777-8777-777777777777"
	content := []byte("synthetic reviewer identity image")
	blobs := &reviewerEncryptedBlobs{}
	getCalls := 0
	blobs.getHook = func() { getCalls++ }
	store, err := encryptedstore.New(blobs, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(context.Background(), ownerID, reviewerID, reviewerFile, content); err != nil {
		t.Fatal(err)
	}
	repo := &reviewerDownloadIntegrityRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseInReview, Revision: 4,
			Files: []dv.File{{
				ID: reviewerFile, State: dv.FileClean, Size: int64(len(content)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(content), RetentionAt: now.Add(time.Hour),
			}},
		},
		auth: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
			}},
		},
	}
	caseService, err := dv.NewService(reviewerDownloadUnusedRepository{})
	if err != nil {
		t.Fatal(err)
	}
	controlAccess, err := dv.NewReviewerAccessService(repo, caseService, store)
	if err != nil {
		t.Fatal(err)
	}
	controlAccess.SetClock(func() time.Time { return now })
	path := reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content"
	control := httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, reviewerDownloadIntegrityService{access: controlAccess}).ServeHTTP(control, reviewerReq(http.MethodGet, path, ""))
	if control.Code != http.StatusOK || !bytes.Equal(control.Body.Bytes(), content) {
		t.Fatalf("positive control status=%d body=%q", control.Code, control.Body.Bytes())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelingRepo := &reviewerDownloadCancelAfterReadTxRepository{reviewerDownloadIntegrityRepository: repo, cancel: cancel}
	access, err := dv.NewReviewerAccessService(cancelingRepo, caseService, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return now })
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodGet, path, "").WithContext(ctx)
	NewReviewerCasesHandler(reviewerAuth{}, reviewerDownloadIntegrityService{access: access}).ServeHTTP(response, request)
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("repository fake did not cancel after the read callback: %v", ctx.Err())
	}
	if response.Code != http.StatusServiceUnavailable || bytes.Contains(response.Body.Bytes(), content) ||
		response.Header().Get("Content-Disposition") != "" || response.Header().Get("Content-Length") != "" ||
		response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("canceled HTTP response status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.Bytes())
	}
	if getCalls != 2 {
		t.Fatalf("encrypted BlobStore reads=%d, want the positive and canceled requests", getCalls)
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("canceled HTTP response security headers=%v", response.Header())
	}
}
