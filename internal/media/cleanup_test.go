package media

import (
	"context"
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
	repo.intent.WriteComplete = true

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
