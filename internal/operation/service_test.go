package operation

import (
	"context"
	"testing"
	"time"
)

const operationTestID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type clockRepository struct {
	candidateAt time.Time
	cleanupAt   time.Time
	writer      *clockWriter
}

func (r *clockRepository) ReserveCandidate(_ context.Context, _, _, _, _ string, at time.Time) error {
	r.candidateAt = at
	return nil
}
func (r *clockRepository) BeginCandidate(context.Context, string, string, string, string) (CandidateWriter, error) {
	return r.writer, nil
}
func (*clockRepository) QueueCandidateCleanup(context.Context, string, time.Time) error { return nil }
func (*clockRepository) List(context.Context, string, string) ([]Item, error)           { return nil, nil }
func (*clockRepository) Evidence(context.Context, string, string, string) (Evidence, error) {
	return Evidence{}, nil
}
func (r *clockRepository) ClaimCandidateCleanup(_ context.Context, at time.Time, _ int) ([]string, error) {
	r.cleanupAt = at
	return []string{operationTestID}, nil
}
func (*clockRepository) CompleteCandidateCleanup(context.Context, string) error { return nil }
func (*clockRepository) FailCandidateCleanup(context.Context, string, time.Time) error {
	return nil
}

type clockWriter struct{ occurredAt time.Time }

func (w *clockWriter) Apply(_ context.Context, id, _ string, _ Input, _ Evidence, _ []byte, now func() time.Time) (Item, bool, error) {
	w.occurredAt = now()
	return Item{ID: id, OccurredAt: w.occurredAt}, false, nil
}
func (*clockWriter) QueueCleanup(context.Context, time.Time) error { return nil }
func (*clockWriter) Close() error                                  { return nil }

type clockFileStore struct{}

func (clockFileStore) Put(context.Context, string, []byte) error   { return nil }
func (clockFileStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (clockFileStore) Delete(context.Context, string) error        { return nil }

type clockIDGenerator struct{}

func (clockIDGenerator) ID() (string, error) { return operationTestID, nil }

func TestBusinessEventsUseInjectedClockAndCandidateRecoveryUsesWallClock(t *testing.T) {
	businessAt := time.Date(2001, time.January, 2, 3, 4, 5, 0, time.UTC)
	writer := &clockWriter{}
	repo := &clockRepository{writer: writer}
	service, err := NewService(repo, clockFileStore{}, clockIDGenerator{}, func() time.Time { return businessAt })
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Record(context.Background(), operationTestID, operationTestID, "attempt-123", Input{Kind: Receipt}); err != nil {
		t.Fatal(err)
	}
	if !writer.occurredAt.Equal(businessAt) {
		t.Fatalf("business event time=%s, want injected %s", writer.occurredAt, businessAt)
	}
	if delta := time.Since(repo.candidateAt); delta < 0 || delta > time.Second {
		t.Fatalf("candidate housekeeping used non-real time: %s", repo.candidateAt)
	}

	if err := service.CleanRetiredOnce(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if delta := time.Since(repo.cleanupAt); delta < 0 || delta > time.Second {
		t.Fatalf("cleanup scheduling used non-real time: %s", repo.cleanupAt)
	}
}
