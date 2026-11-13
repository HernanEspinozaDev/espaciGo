package media

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type cleanupRepo struct {
	Repository
	mu                sync.Mutex
	completed, failed int
	intent            ObjectIntent
}

func (r *cleanupRepo) ClaimCleanup(context.Context, int, time.Time) ([]CleanupItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completed > 0 {
		return nil, nil
	}
	intent := r.intent
	if intent.ObjectKey == "" {
		intent.ObjectKey = "key"
	}
	return []CleanupItem{{AssetID: "asset", Token: 2, Intents: []ObjectIntent{intent}}}, nil
}
func (r *cleanupRepo) CompleteCleanup(context.Context, string, int64, time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completed++
	return nil
}
func (r *cleanupRepo) FailCleanup(context.Context, string, int64, time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed++
	return nil
}
func (r *cleanupRepo) FinishVariantWrite(_ context.Context, _ string, name VariantName, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intent = ObjectIntent{Name: name, ObjectKey: key, WriteComplete: true}
	return nil
}

func TestFinishVariantWriteSurvivesRequestCancellation(t *testing.T) {
	repo := &cancellationFinisher{}
	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := finishVariantWrite(requestCtx, repo, "asset", VariantDisplay, "key"); err != nil {
		t.Fatal(err)
	}
	if !repo.called {
		t.Fatal("write completion was not persisted after request cancellation")
	}
}

type cancellationFinisher struct {
	Repository
	called bool
}

func (r *cancellationFinisher) FinishVariantWrite(ctx context.Context, _ string, _ VariantName, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		return context.DeadlineExceeded
	}
	r.called = true
	return nil
}

type cleanupStore struct {
	ObjectStore
	started     chan struct{}
	release     chan struct{}
	mu          sync.Mutex
	putFinished bool
	deleted     []int64
}

func (s *cleanupStore) Generation(context.Context, string) (int64, error) {
	s.mu.Lock()
	finished := s.putFinished
	s.mu.Unlock()
	close(s.started)
	<-s.release
	if !finished {
		return 0, ErrNotFound
	}
	return 42, nil
}
func (s *cleanupStore) Delete(_ context.Context, _ string, generation int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, generation)
	return nil
}

type cleanupIDs struct{}

func (cleanupIDs) ID() (string, error) { return "unused", nil }

func TestCleanOnceRetainsUnknownIntentAcrossConcurrentPut(t *testing.T) {
	repo := &cleanupRepo{}
	store := &cleanupStore{started: make(chan struct{}), release: make(chan struct{})}
	svc, err := NewService(repo, store, cleanupIDs{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svc.CleanOnce(context.Background(), 1) }()
	<-store.started // cleaner observed NotFound while Put is still in flight
	store.mu.Lock()
	store.putFinished = true
	store.mu.Unlock()
	close(store.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if repo.failed != 1 || repo.completed != 0 {
		t.Fatalf("first cleanup failed=%d completed=%d", repo.failed, repo.completed)
	}
	if err := repo.FinishVariantWrite(context.Background(), "asset", VariantDisplay, "key"); err != nil {
		t.Fatal(err)
	}

	store.started = make(chan struct{})
	store.release = make(chan struct{})
	close(store.release)
	if err := svc.CleanOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if repo.completed != 1 || len(store.deleted) != 1 || store.deleted[0] != 42 {
		t.Fatalf("completed=%d deleted generations=%v", repo.completed, store.deleted)
	}
}

func TestCleanOnceCompletesTerminalMissingWriteAndIsIdempotent(t *testing.T) {
	repo := &cleanupRepo{intent: ObjectIntent{ObjectKey: "key", WriteComplete: true}}
	store := &cleanupStore{started: make(chan struct{}), release: make(chan struct{})}
	close(store.release)
	svc, err := NewService(repo, store, cleanupIDs{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CleanOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if repo.completed != 1 || repo.failed != 0 {
		t.Fatalf("terminal missing write cleanup failed=%d completed=%d", repo.failed, repo.completed)
	}
	if err := svc.CleanOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if repo.completed != 1 {
		t.Fatalf("repeat cleanup completed %d times", repo.completed)
	}
}

type cleanupBoundaryRepository struct {
	Repository
	items                      []CleanupItem
	claimErr, failErr, doneErr error
	claims, failures, done     int
}

func (r *cleanupBoundaryRepository) ClaimCleanup(context.Context, int, time.Time) ([]CleanupItem, error) {
	r.claims++
	return r.items, r.claimErr
}

func (r *cleanupBoundaryRepository) FailCleanup(context.Context, string, int64, time.Time) error {
	r.failures++
	return r.failErr
}

func (r *cleanupBoundaryRepository) CompleteCleanup(context.Context, string, int64, time.Time) error {
	r.done++
	return r.doneErr
}

type cleanupBoundaryStore struct {
	ObjectStore
	generation    int64
	generationErr error
	deleteErr     error
	lookups       []string
	deletions     []struct {
		key        string
		generation int64
	}
}

func (s *cleanupBoundaryStore) Generation(_ context.Context, key string) (int64, error) {
	s.lookups = append(s.lookups, key)
	return s.generation, s.generationErr
}

func (s *cleanupBoundaryStore) Delete(_ context.Context, key string, generation int64) error {
	s.deletions = append(s.deletions, struct {
		key        string
		generation int64
	}{key: key, generation: generation})
	return s.deleteErr
}

func TestCleanOnceMarksGenerationAndObjectDeleteFailuresForRetry(t *testing.T) {
	generationErr := errors.New("synthetic generation lookup failure")
	deleteErr := errors.New("synthetic object deletion failure")
	cases := []struct {
		name    string
		item    CleanupItem
		store   cleanupBoundaryStore
		wantGet []string
		wantDel []struct {
			key        string
			generation int64
		}
	}{
		{
			name:    "generation lookup failure",
			item:    CleanupItem{AssetID: "asset", Token: 4, Intents: []ObjectIntent{{ObjectKey: "intent-key", WriteComplete: true}}},
			store:   cleanupBoundaryStore{generationErr: generationErr},
			wantGet: []string{"intent-key"},
		},
		{
			name:  "intent delete failure",
			item:  CleanupItem{AssetID: "asset", Token: 4, Intents: []ObjectIntent{{ObjectKey: "intent-key", Generation: 41, WriteComplete: true}}},
			store: cleanupBoundaryStore{deleteErr: deleteErr},
			wantDel: []struct {
				key        string
				generation int64
			}{{key: "intent-key", generation: 41}},
		},
		{
			name:  "variant delete failure",
			item:  CleanupItem{AssetID: "asset", Token: 4, Variants: []Variant{{ObjectKey: "variant-key", Generation: 42}}},
			store: cleanupBoundaryStore{deleteErr: deleteErr},
			wantDel: []struct {
				key        string
				generation int64
			}{{key: "variant-key", generation: 42}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &cleanupBoundaryRepository{items: []CleanupItem{tc.item}}
			store := tc.store
			svc, err := NewService(repo, &store, cleanupIDs{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.CleanOnce(context.Background(), 1); err != nil {
				t.Fatalf("CleanOnce returned error: %v", err)
			}
			if repo.claims != 1 || repo.failures != 1 || repo.done != 0 {
				t.Fatalf("claims=%d failures=%d completed=%d, want 1/1/0", repo.claims, repo.failures, repo.done)
			}
			if len(store.lookups) != len(tc.wantGet) || len(store.deletions) != len(tc.wantDel) {
				t.Fatalf("generation lookups=%v deletions=%v", store.lookups, store.deletions)
			}
			for i := range tc.wantGet {
				if store.lookups[i] != tc.wantGet[i] {
					t.Fatalf("generation lookups=%v, want %v", store.lookups, tc.wantGet)
				}
			}
			for i := range tc.wantDel {
				if store.deletions[i] != tc.wantDel[i] {
					t.Fatalf("deletions=%v, want %v", store.deletions, tc.wantDel)
				}
			}
		})
	}
}

func TestCleanOncePropagatesRepositoryBoundaryFailures(t *testing.T) {
	claimErr := errors.New("synthetic claim failure")
	failErr := errors.New("synthetic fail-cleanup failure")
	doneErr := errors.New("synthetic complete-cleanup failure")
	cases := []struct {
		name        string
		repo        cleanupBoundaryRepository
		store       cleanupBoundaryStore
		wantErr     error
		wantFails   int
		wantDone    int
		wantDeletes int
	}{
		{name: "claim failure", repo: cleanupBoundaryRepository{claimErr: claimErr}, store: cleanupBoundaryStore{}, wantErr: claimErr},
		{
			name:  "fail cleanup failure",
			repo:  cleanupBoundaryRepository{items: []CleanupItem{{AssetID: "asset", Token: 5, Intents: []ObjectIntent{{ObjectKey: "key", Generation: 7}}}}, failErr: failErr},
			store: cleanupBoundaryStore{deleteErr: errors.New("synthetic delete failure")}, wantErr: failErr, wantFails: 1, wantDeletes: 1,
		},
		{
			name:  "complete cleanup failure",
			repo:  cleanupBoundaryRepository{items: []CleanupItem{{AssetID: "asset", Token: 5, Variants: []Variant{{ObjectKey: "key", Generation: 7}}}}, doneErr: doneErr},
			store: cleanupBoundaryStore{}, wantErr: doneErr, wantDone: 1, wantDeletes: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.repo
			store := tc.store
			svc, err := NewService(&repo, &store, cleanupIDs{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			err = svc.CleanOnce(context.Background(), 1)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CleanOnce error=%v, want %v", err, tc.wantErr)
			}
			if repo.claims != 1 || repo.failures != tc.wantFails || repo.done != tc.wantDone || len(store.deletions) != tc.wantDeletes {
				t.Fatalf("claims=%d failures=%d completed=%d deletions=%d", repo.claims, repo.failures, repo.done, len(store.deletions))
			}
		})
	}
}

func TestCleanOnceRejectsInvalidBatchLimitsBeforeClaiming(t *testing.T) {
	for _, limit := range []int{0, 501} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			repo := &cleanupBoundaryRepository{}
			svc, err := NewService(repo, &cleanupBoundaryStore{}, cleanupIDs{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.CleanOnce(context.Background(), limit); !errors.Is(err, ErrInvalid) {
				t.Fatalf("CleanOnce error=%v, want ErrInvalid", err)
			}
			if repo.claims != 0 {
				t.Fatalf("invalid limit caused %d cleanup claims", repo.claims)
			}
		})
	}
}
