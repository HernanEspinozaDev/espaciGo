package verification

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"sync"
	"testing"
	"time"
)

type memoryEvidenceRepo struct {
	mu         sync.Mutex
	items      map[string]Evidence
	owners     map[string]string
	createErr  error
	listOwnErr error
	listRevErr error
	getOwnErr  error
	getRevErr  error
	delOwnErr  error
	delRevErr  error
}

func newMemoryEvidenceRepo() *memoryEvidenceRepo {
	return &memoryEvidenceRepo{
		items:  make(map[string]Evidence),
		owners: make(map[string]string),
	}
}

func (r *memoryEvidenceRepo) CreateEvidence(_ context.Context, item Evidence) (Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return Evidence{}, r.createErr
	}
	r.items[item.ID] = item
	return item, nil
}

func (r *memoryEvidenceRepo) ListOwnEvidence(_ context.Context, owner, caseID string) ([]Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listOwnErr != nil {
		return nil, r.listOwnErr
	}
	if r.owners[caseID] != "" && r.owners[caseID] != owner {
		return nil, ErrNotFound
	}
	var res []Evidence
	for _, it := range r.items {
		if it.VerificationID == caseID {
			res = append(res, it)
		}
	}
	return res, nil
}

func (r *memoryEvidenceRepo) ListReviewEvidence(_ context.Context, caseID string) ([]Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listRevErr != nil {
		return nil, r.listRevErr
	}
	var res []Evidence
	for _, it := range r.items {
		if it.VerificationID == caseID {
			res = append(res, it)
		}
	}
	return res, nil
}

func (r *memoryEvidenceRepo) GetOwnEvidence(_ context.Context, owner, caseID, evidenceID string) (Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getOwnErr != nil {
		return Evidence{}, r.getOwnErr
	}
	if r.owners[caseID] != "" && r.owners[caseID] != owner {
		return Evidence{}, ErrNotFound
	}
	it, ok := r.items[evidenceID]
	if !ok || it.VerificationID != caseID {
		return Evidence{}, ErrNotFound
	}
	return it, nil
}

func (r *memoryEvidenceRepo) GetReviewEvidence(_ context.Context, caseID, evidenceID string) (Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getRevErr != nil {
		return Evidence{}, r.getRevErr
	}
	it, ok := r.items[evidenceID]
	if !ok || it.VerificationID != caseID {
		return Evidence{}, ErrNotFound
	}
	return it, nil
}

func (r *memoryEvidenceRepo) DeleteOwnEvidence(_ context.Context, owner, caseID, evidenceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.delOwnErr != nil {
		return r.delOwnErr
	}
	if r.owners[caseID] != "" && r.owners[caseID] != owner {
		return ErrNotFound
	}
	it, ok := r.items[evidenceID]
	if !ok || it.VerificationID != caseID {
		return ErrNotFound
	}
	delete(r.items, evidenceID)
	return nil
}

func (r *memoryEvidenceRepo) DeleteReviewEvidence(_ context.Context, caseID, evidenceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.delRevErr != nil {
		return r.delRevErr
	}
	it, ok := r.items[evidenceID]
	if !ok || it.VerificationID != caseID {
		return ErrNotFound
	}
	delete(r.items, evidenceID)
	return nil
}

func TestMemoryEvidenceRepoRejectsOtherOwners(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryEvidenceRepo()
	repo.owners["case-1"] = "owner-1"
	repo.items["evidence-1"] = Evidence{ID: "evidence-1", VerificationID: "case-1"}

	if _, err := repo.ListOwnEvidence(ctx, "owner-2", "case-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ListOwnEvidence error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetOwnEvidence(ctx, "owner-2", "case-1", "evidence-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetOwnEvidence error = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteOwnEvidence(ctx, "owner-2", "case-1", "evidence-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteOwnEvidence error = %v, want ErrNotFound", err)
	}
	if _, ok := repo.items["evidence-1"]; !ok {
		t.Fatal("foreign-owner delete removed evidence")
	}
}

type memoryEvidenceStorage struct {
	mu        sync.Mutex
	blobs     map[string][]byte
	putErr    error
	getErr    error
	delErr    error
	getCalls  int
	deletedID string
}

func newMemoryEvidenceStorage() *memoryEvidenceStorage {
	return &memoryEvidenceStorage{blobs: make(map[string][]byte)}
}

func (s *memoryEvidenceStorage) Put(_ context.Context, key string, blob []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return s.putErr
	}
	s.blobs[key] = append([]byte(nil), blob...)
	return nil
}

func (s *memoryEvidenceStorage) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	b, ok := s.blobs[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), b...), nil
}

func (s *memoryEvidenceStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedID = key
	if s.delErr != nil {
		return s.delErr
	}
	delete(s.blobs, key)
	return nil
}

type generatorWithErr struct {
	err error
	n   int
}

func (g *generatorWithErr) ID() (string, error) {
	if g.err != nil {
		return "", g.err
	}
	g.n++
	return "evid-00000000-0000-4000-8000-000000000001", nil
}

func TestEvidenceServiceConstructorBoundaries(t *testing.T) {
	cases := newMemoryRepo()
	repo := newMemoryEvidenceRepo()
	store := newMemoryEvidenceStorage()
	gen := &generatorWithErr{}
	now := func() time.Time { return time.Unix(100, 0) }

	if _, err := NewEvidenceService(nil, repo, store, gen, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil cases")
	}
	if _, err := NewEvidenceService(cases, nil, store, gen, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil repo")
	}
	if _, err := NewEvidenceService(cases, repo, nil, gen, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil store")
	}
	if _, err := NewEvidenceService(cases, repo, store, nil, now); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil generator")
	}
	if _, err := NewEvidenceService(cases, repo, store, gen, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil now")
	}
	svc, err := NewEvidenceService(cases, repo, store, gen, now)
	if err != nil || svc == nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
}

func TestEvidenceServiceUploadBoundaries(t *testing.T) {
	ctx := context.Background()
	cases := newMemoryRepo()
	repo := newMemoryEvidenceRepo()
	store := newMemoryEvidenceStorage()
	gen := &generatorWithErr{}
	now := func() time.Time { return time.Unix(100, 0) }
	svc, _ := NewEvidenceService(cases, repo, store, gen, now)

	// Empty parameters
	if _, err := svc.Upload(ctx, "", "case-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner")
	}
	if _, err := svc.Upload(ctx, "owner-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty caseID")
	}

	// Case not found in cases repo
	if _, err := svc.Upload(ctx, "owner-1", "missing-case"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound from cases.GetOwn, got: %v", err)
	}

	// Add valid case to cases repo
	validCase := Case{ID: "case-1", OwnerID: "owner-1", State: "en_revision", Type: "kyc", Idempotency: "idem-1"}
	_, _ = cases.Create(ctx, validCase)

	// ID generator failure
	gen.err = errors.New("id generation failed")
	if _, err := svc.Upload(ctx, "owner-1", "case-1"); err == nil || err.Error() != "id generation failed" {
		t.Fatalf("expected id generator error, got: %v", err)
	}
	gen.err = nil

	// Store Put error
	store.putErr = errors.New("storage put failure")
	if _, err := svc.Upload(ctx, "owner-1", "case-1"); err == nil || err.Error() != "storage put failure" {
		t.Fatalf("expected storage put error, got: %v", err)
	}
	store.putErr = nil

	// Repo CreateEvidence error triggers compensation store.Delete
	repo.createErr = errors.New("db insert failed")
	if _, err := svc.Upload(ctx, "owner-1", "case-1"); err == nil || err.Error() != "db insert failed" {
		t.Fatalf("expected db insert error, got: %v", err)
	}
	if store.deletedID == "" {
		t.Fatal("expected compensation store.Delete call upon db insert failure")
	}
	if len(store.blobs) != 0 {
		t.Fatalf("expected 0 blobs in store after compensation, got %d", len(store.blobs))
	}
	repo.createErr = nil
	store.deletedID = ""

	// Success path
	ev, err := svc.Upload(ctx, "owner-1", "case-1")
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}
	if ev.ID == "" || ev.VerificationID != "case-1" || ev.MIMEType != "image/png" || !ev.FileAvailable || ev.SizeBytes <= 0 || ev.SHA256 == "" {
		t.Fatalf("unexpected evidence attributes: %+v", ev)
	}
	if ev.FixtureCode != SyntheticEvidenceFixture {
		t.Fatalf("expected fixture code %s, got: %s", SyntheticEvidenceFixture, ev.FixtureCode)
	}
}

func TestEvidenceServiceListBoundaries(t *testing.T) {
	ctx := context.Background()
	cases := newMemoryRepo()
	repo := newMemoryEvidenceRepo()
	store := newMemoryEvidenceStorage()
	gen := &generatorWithErr{}
	now := func() time.Time { return time.Unix(100, 0) }
	svc, _ := NewEvidenceService(cases, repo, store, gen, now)

	validCase := Case{ID: "case-1", OwnerID: "owner-1", State: "en_revision", Type: "kyc", Idempotency: "idem-1"}
	_, _ = cases.Create(ctx, validCase)
	repo.owners[validCase.ID] = validCase.OwnerID

	// ListOwn parameter boundaries
	if _, err := svc.ListOwn(ctx, "", "case-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner")
	}
	if _, err := svc.ListOwn(ctx, "owner-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty caseID")
	}
	if _, err := svc.ListOwn(ctx, "owner-1", "missing-case"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound from cases.GetOwn, got: %v", err)
	}

	// ListReview parameter boundaries
	if _, err := svc.ListReview(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty caseID")
	}

	// Success listing
	_, _ = svc.Upload(ctx, "owner-1", "case-1")
	ownList, err := svc.ListOwn(ctx, "owner-1", "case-1")
	if err != nil || len(ownList) != 1 {
		t.Fatalf("expected 1 item in ListOwn, got %d, err=%v", len(ownList), err)
	}
	revList, err := svc.ListReview(ctx, "case-1")
	if err != nil || len(revList) != 1 {
		t.Fatalf("expected 1 item in ListReview, got %d, err=%v", len(revList), err)
	}
}

func TestEvidenceServiceContentBoundaries(t *testing.T) {
	ctx := context.Background()
	cases := newMemoryRepo()
	repo := newMemoryEvidenceRepo()
	store := newMemoryEvidenceStorage()
	gen := &generatorWithErr{}
	now := func() time.Time { return time.Unix(100, 0) }
	svc, _ := NewEvidenceService(cases, repo, store, gen, now)

	validCase := Case{ID: "case-1", OwnerID: "owner-1", State: "en_revision", Type: "kyc", Idempotency: "idem-1"}
	_, _ = cases.Create(ctx, validCase)
	repo.owners[validCase.ID] = validCase.OwnerID
	ev, _ := svc.Upload(ctx, "owner-1", "case-1")
	if _, _, err := svc.OwnContent(ctx, "owner-2", "case-1", ev.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for foreign owner, got: %v", err)
	}
	if store.getCalls != 0 {
		t.Fatalf("storage reads for foreign owner = %d, want 0", store.getCalls)
	}

	// OwnContent boundaries
	if _, _, err := svc.OwnContent(ctx, "", "case-1", ev.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner in OwnContent")
	}
	if _, _, err := svc.OwnContent(ctx, "owner-1", "", ev.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty caseID in OwnContent")
	}
	if _, _, err := svc.OwnContent(ctx, "owner-1", "case-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty evidenceID in OwnContent")
	}
	if _, _, err := svc.OwnContent(ctx, "owner-1", "case-1", "missing-ev"); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for missing evidence in OwnContent")
	}

	// ReviewContent boundaries
	if _, _, err := svc.ReviewContent(ctx, "", ev.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty caseID in ReviewContent")
	}
	if _, _, err := svc.ReviewContent(ctx, "case-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty evidenceID in ReviewContent")
	}
	if _, _, err := svc.ReviewContent(ctx, "case-1", "missing-ev"); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for missing evidence in ReviewContent")
	}

	// Unavailable file returns ErrNotFound
	repo.items[ev.ID] = Evidence{ID: ev.ID, VerificationID: "case-1", FileAvailable: false, FileID: ev.ID}
	if _, _, err := svc.OwnContent(ctx, "owner-1", "case-1", ev.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound when FileAvailable=false in OwnContent")
	}
	if _, _, err := svc.ReviewContent(ctx, "case-1", ev.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound when FileAvailable=false in ReviewContent")
	}

	repo.items[ev.ID] = Evidence{ID: ev.ID, VerificationID: "case-1", FileAvailable: true, FileID: ""}
	if _, _, err := svc.OwnContent(ctx, "owner-1", "case-1", ev.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound when FileID is empty in OwnContent")
	}
	if _, _, err := svc.ReviewContent(ctx, "case-1", ev.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound when FileID is empty in ReviewContent")
	}

	// Store error
	repo.items[ev.ID] = ev
	store.getErr = errors.New("blob read failed")
	if _, _, err := svc.OwnContent(ctx, "owner-1", "case-1", ev.ID); err == nil || err.Error() != "blob read failed" {
		t.Fatalf("expected blob read error in OwnContent, got: %v", err)
	}
	if _, _, err := svc.ReviewContent(ctx, "case-1", ev.ID); err == nil || err.Error() != "blob read failed" {
		t.Fatalf("expected blob read error in ReviewContent, got: %v", err)
	}
	store.getErr = nil

	// Success retrieval
	item, blob, err := svc.OwnContent(ctx, "owner-1", "case-1", ev.ID)
	if err != nil || len(blob) == 0 || item.ID != ev.ID {
		t.Fatalf("unexpected OwnContent result: item=%+v len(blob)=%d err=%v", item, len(blob), err)
	}
	revItem, revBlob, err := svc.ReviewContent(ctx, "case-1", ev.ID)
	if err != nil || len(revBlob) == 0 || revItem.ID != ev.ID {
		t.Fatalf("unexpected ReviewContent result: item=%+v len(blob)=%d err=%v", revItem, len(revBlob), err)
	}
}

func TestEvidenceServiceDeleteBoundaries(t *testing.T) {
	ctx := context.Background()
	cases := newMemoryRepo()
	repo := newMemoryEvidenceRepo()
	store := newMemoryEvidenceStorage()
	gen := &generatorWithErr{}
	now := func() time.Time { return time.Unix(100, 0) }
	svc, _ := NewEvidenceService(cases, repo, store, gen, now)

	validCase := Case{ID: "case-1", OwnerID: "owner-1", State: "en_revision", Type: "kyc", Idempotency: "idem-1"}
	_, _ = cases.Create(ctx, validCase)
	repo.owners[validCase.ID] = validCase.OwnerID

	ev1, _ := svc.Upload(ctx, "owner-1", "case-1")

	// Storage delete error propagates
	store.delErr = errors.New("store delete failure")
	if err := svc.DeleteOwn(ctx, "owner-1", "case-1", ev1.ID); err == nil || err.Error() != "store delete failure" {
		t.Fatalf("expected store delete failure in DeleteOwn, got: %v", err)
	}
	if err := svc.DeleteReview(ctx, "case-1", ev1.ID); err == nil || err.Error() != "store delete failure" {
		t.Fatalf("expected store delete failure in DeleteReview, got: %v", err)
	}
	store.delErr = nil

	// Metadata deletion failure is surfaced after storage deletion; metadata remains for reconciliation.
	repo.delOwnErr = errors.New("metadata delete failure")
	if err := svc.DeleteOwn(ctx, "owner-1", "case-1", ev1.ID); err == nil || err.Error() != "metadata delete failure" {
		t.Fatalf("expected metadata delete failure in DeleteOwn, got: %v", err)
	}
	if _, ok := store.blobs[ev1.FileID]; ok {
		t.Fatal("expected blob removed before metadata delete error")
	}
	if _, ok := repo.items[ev1.ID]; !ok {
		t.Fatal("expected metadata to remain after failed repository delete")
	}
	repo.delOwnErr = nil

	// Delete when FileAvailable is false deletes from repo without calling store.Delete
	store.deletedID = ""
	repo.items[ev1.ID] = Evidence{ID: ev1.ID, VerificationID: "case-1", FileAvailable: false, FileID: ""}
	if err := svc.DeleteOwn(ctx, "owner-1", "case-1", ev1.ID); err != nil {
		t.Fatalf("unexpected DeleteOwn error: %v", err)
	}
	if store.deletedID != "" {
		t.Fatal("store.Delete should not be called when FileAvailable is false")
	}

	// Re-upload for DeleteReview test
	ev2, _ := svc.Upload(ctx, "owner-1", "case-1")
	// Metadata deletion failure is surfaced for reviewer deletion.
	repo.delRevErr = errors.New("review metadata delete failure")
	if err := svc.DeleteReview(ctx, "case-1", ev2.ID); err == nil || err.Error() != "review metadata delete failure" {
		t.Fatalf("expected metadata delete failure in DeleteReview, got: %v", err)
	}
	if _, ok := store.blobs[ev2.FileID]; ok {
		t.Fatal("expected reviewer blob removed before metadata delete error")
	}
	if _, ok := repo.items[ev2.ID]; !ok {
		t.Fatal("expected reviewer metadata to remain after failed repository delete")
	}
	repo.delRevErr = nil
	if err := svc.DeleteReview(ctx, "case-1", ev2.ID); err != nil {
		t.Fatalf("retry DeleteReview: %v", err)
	}
	if _, ok := repo.items[ev2.ID]; ok {
		t.Fatal("expected item to be deleted from repo after retry")
	}
	if _, ok := store.blobs[ev2.FileID]; ok {
		t.Fatal("expected blob to be deleted from store")
	}
}

func TestEvidenceServiceUploadCompensationDeleteFailure(t *testing.T) {
	ctx := context.Background()
	cases := newMemoryRepo()
	repo := newMemoryEvidenceRepo()
	store := newMemoryEvidenceStorage()
	gen := &generatorWithErr{}
	now := func() time.Time { return time.Unix(100, 0) }
	svc, err := NewEvidenceService(cases, repo, store, gen, now)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = cases.Create(ctx, Case{ID: "case-1", OwnerID: "owner-1", State: "en_revision", Type: "kyc", Idempotency: "idem-1"})
	repo.createErr = errors.New("metadata create failure")
	store.delErr = errors.New("compensating delete failure")

	if _, err := svc.Upload(ctx, "owner-1", "case-1"); err == nil || err.Error() != "metadata create failure" {
		t.Fatalf("Upload error = %v, want metadata create failure", err)
	}
	if store.deletedID != "evid-00000000-0000-4000-8000-000000000001" {
		t.Fatalf("compensation key = %q, want the uploaded evidence key", store.deletedID)
	}
	if _, ok := store.blobs[store.deletedID]; !ok {
		t.Fatal("expected orphan blob to remain when compensating delete fails")
	}
	if len(repo.items) != 0 {
		t.Fatalf("metadata rows = %d, want 0 after failed create", len(repo.items))
	}
}

func TestSyntheticPNGGeneratesValidCheckerboardPNG(t *testing.T) {
	blob, err := SyntheticPNG()
	if err != nil {
		t.Fatalf("SyntheticPNG failed: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("SyntheticPNG returned empty bytes")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(blob))
	if err != nil {
		t.Fatalf("failed to decode PNG config: %v", err)
	}
	if cfg.Width != 96 || cfg.Height != 64 {
		t.Fatalf("expected dimensions 96x64, got %dx%d", cfg.Width, cfg.Height)
	}
}
