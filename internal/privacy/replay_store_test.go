package privacy

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

type replayOnlyRepository struct{ database SuppressionReplayManifest }

func (*replayOnlyRepository) GetProfile(context.Context, string) (Profile, error) {
	return Profile{}, ErrNotFound
}
func (*replayOnlyRepository) SaveProfile(context.Context, string, string, *string) (Profile, error) {
	return Profile{}, ErrInvalid
}
func (*replayOnlyRepository) CreateRightsRequest(context.Context, string, string, string) (RightsRequest, error) {
	return RightsRequest{}, ErrInvalid
}
func (*replayOnlyRepository) ListOwnRightsRequests(context.Context, string) ([]RightsRequest, error) {
	return nil, nil
}
func (*replayOnlyRepository) ExportOwnData(context.Context, string) (OwnData, error) {
	return OwnData{}, ErrExportUnavailable
}
func (*replayOnlyRepository) PurgeExpiredReservationLinks(context.Context, time.Time, int) (RetentionPurgeResult, error) {
	return RetentionPurgeResult{}, nil
}
func (r *replayOnlyRepository) ExportCompletedSuppressions(context.Context) (SuppressionReplayManifest, error) {
	return r.database, nil
}
func (*replayOnlyRepository) PrepareSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, time.Time) (string, string, string, bool, error) {
	return "", "", "", false, nil
}
func (*replayOnlyRepository) FinishSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, string, time.Time) error {
	return nil
}

type replayMemoryStore struct {
	mu             sync.Mutex
	manifest       SuppressionReplayManifest
	revision       int
	failLoad       bool
	loseNextWrite  bool
	conflictWrites int
}

type replayCleanerFailureRepository struct {
	*replayOnlyRepository
	failedFiles    int
	completedFiles int
	replayStatus   string
}

func (*replayCleanerFailureRepository) ExecuteSuppression(context.Context, string, string, string, string, func() time.Time) (SuppressionExecution, error) {
	return SuppressionExecution{RequestID: "restored-request", Status: "en_proceso"}, nil
}
func (*replayCleanerFailureRepository) PendingSuppressionFiles(context.Context, string, time.Time) ([]SuppressionFile, error) {
	return []SuppressionFile{{EvidenceID: "33333333-3333-4333-8333-333333333333", StorageID: "33333333-3333-4333-8333-333333333333"}}, nil
}
func (r *replayCleanerFailureRepository) CompleteSuppressionFile(context.Context, SuppressionFile, time.Time) error {
	r.completedFiles++
	return nil
}
func (r *replayCleanerFailureRepository) FailSuppressionFile(context.Context, SuppressionFile, time.Time) error {
	r.failedFiles++
	return nil
}
func (*replayCleanerFailureRepository) FinishSuppression(_ context.Context, requestID string, _ time.Time) (SuppressionExecution, error) {
	return SuppressionExecution{RequestID: requestID, Status: "pendiente", PendingFiles: 1}, nil
}
func (*replayCleanerFailureRepository) PendingSuppressionRequests(context.Context) ([]string, error) {
	return nil, nil
}
func (r *replayCleanerFailureRepository) PrepareSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, time.Time) (string, string, string, bool, error) {
	return "restored-request", "restore-key", "pendiente", false, nil
}
func (r *replayCleanerFailureRepository) FinishSuppressionReplay(_ context.Context, _ SuppressionReplayEntry, _, _, status string, _ time.Time) error {
	r.replayStatus = status
	return nil
}

type replayFailingCleaner struct{}

func (replayFailingCleaner) Delete(context.Context, string) error {
	return errors.New("remote object deletion failed")
}

func (s *replayMemoryStore) Load(ctx context.Context) (SuppressionReplaySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return SuppressionReplaySnapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failLoad {
		return SuppressionReplaySnapshot{}, ErrReplayStoreUnavailable
	}
	manifest, _ := NormalizeSuppressionReplayManifest(s.manifest)
	if manifest.Version == 0 {
		manifest = SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{}}
	}
	return SuppressionReplaySnapshot{Revision: strconv.Itoa(s.revision), Manifest: manifest}, nil
}

func (s *replayMemoryStore) CompareAndSwap(ctx context.Context, revision string, manifest SuppressionReplayManifest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != strconv.Itoa(s.revision) {
		s.conflictWrites++
		return "", ErrReplayStoreConflict
	}
	if _, err := NormalizeSuppressionReplayManifest(manifest); err != nil {
		return "", ErrReplayStoreConflict
	}
	s.manifest = manifest
	s.revision++
	if s.loseNextWrite {
		s.loseNextWrite = false
		return "", ErrReplayStoreUnavailable
	}
	return strconv.Itoa(s.revision), nil
}

func replayFixtureEntry(executionID, requestID, accountID string) SuppressionReplayEntry {
	return SuppressionReplayEntry{
		ExecutionID: executionID,
		RequestID:   requestID,
		AccountID:   accountID,
		RequestedAt: time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC),
		CompletedAt: time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC),
	}
}

func TestReplayStoreSyncMergesWithoutLostUpdatesAndIsIdempotent(t *testing.T) {
	old := replayFixtureEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	newEntry := replayFixtureEntry("cccccccc-cccc-4ccc-8ccc-cccccccccccc", "22222222-2222-4222-8222-222222222222", "dddddddd-dddd-4ddd-8ddd-dddddddddddd")
	repo := &replayOnlyRepository{database: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{newEntry}}}
	store := &replayMemoryStore{manifest: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{old}}}
	service, _ := NewService(repo)
	manifest, err := service.ExportSuppressionReplayManifestWithStore(context.Background(), store)
	if err != nil || len(manifest.Entries) != 2 {
		t.Fatalf("merged manifest=%+v err=%v", manifest, err)
	}
	if err := service.ExportSuppressionReplayManifestToStore(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.manifest.Entries) != 2 || store.revision != 1 {
		t.Fatalf("repeat sync rewrote or lost entries: manifest=%+v revision=%d", store.manifest, store.revision)
	}
}

func TestReplayStoreSyncReconcilesLostWriteResponseAndConcurrentCAS(t *testing.T) {
	first := replayFixtureEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	second := replayFixtureEntry("cccccccc-cccc-4ccc-8ccc-cccccccccccc", "22222222-2222-4222-8222-222222222222", "dddddddd-dddd-4ddd-8ddd-dddddddddddd")
	store := &replayMemoryStore{loseNextWrite: true}
	services := make([]*Service, 2)
	for i, entry := range []SuppressionReplayEntry{first, second} {
		repo := &replayOnlyRepository{database: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{entry}}}
		services[i], _ = NewService(repo)
	}
	var wait sync.WaitGroup
	errorsSeen := make(chan error, len(services))
	for _, service := range services {
		wait.Add(1)
		go func(service *Service) {
			defer wait.Done()
			errorsSeen <- service.ExportSuppressionReplayManifestToStore(context.Background(), store)
		}(service)
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("concurrent sync: %v", err)
		}
	}
	snapshot, err := store.Load(context.Background())
	if err != nil || len(snapshot.Manifest.Entries) != 2 {
		t.Fatalf("concurrent sync lost a replay entry: snapshot=%+v err=%v", snapshot, err)
	}
}

func TestReplayStoreConflictOrUnavailableNeverReturnsConfirmed(t *testing.T) {
	entry := replayFixtureEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	conflicting := entry
	conflicting.AccountID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	repo := &replayOnlyRepository{database: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{entry}}}
	store := &replayMemoryStore{manifest: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{conflicting}}}
	service, _ := NewService(repo)
	if err := service.ExportSuppressionReplayManifestToStore(context.Background(), store); !errors.Is(err, ErrReplayStoreConflict) {
		t.Fatalf("conflicting immutable record error=%v", err)
	}
	store = &replayMemoryStore{failLoad: true}
	if err := service.ExportSuppressionReplayManifestToStore(context.Background(), store); !errors.Is(err, ErrReplayStoreUnavailable) {
		t.Fatalf("unavailable external store error=%v", err)
	}
}

func TestReplayAfterRestoreMergesCurrentExternalRegistry(t *testing.T) {
	entry := replayFixtureEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	store := &replayMemoryStore{manifest: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{entry}}}
	service, _ := NewService(&replayOnlyRepository{database: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{}}})
	manifest, err := service.MergeSuppressionReplayManifestWithStore(context.Background(), SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{}}, store)
	if err != nil || len(manifest.Entries) != 1 || manifest.Entries[0].ExecutionID != entry.ExecutionID {
		t.Fatalf("restored replay manifest=%+v err=%v", manifest, err)
	}
}

func TestReplayDoesNotCompleteWhenEvidenceDeletionFails(t *testing.T) {
	entry := replayFixtureEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	repo := &replayCleanerFailureRepository{replayOnlyRepository: &replayOnlyRepository{database: SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{}}}}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	results, err := service.ReplaySuppressionManifest(context.Background(), SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{entry}}, "restore-1", "admin-1", time.Now().UTC(), replayFailingCleaner{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "pendiente" || repo.failedFiles != 1 || repo.completedFiles != 0 || repo.replayStatus != "pendiente" {
		t.Fatalf("failed remote evidence deletion was confirmed: results=%+v failed=%d completed=%d replayStatus=%q", results, repo.failedFiles, repo.completedFiles, repo.replayStatus)
	}
}
