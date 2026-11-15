package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type cancelingOwnCaseReaderRepository struct {
	*memoryRepository
	cancel    context.CancelFunc
	cancelOn  string
	listCalls int
	loadCalls int
}

func (r *cancelingOwnCaseReaderRepository) ListOwnCases(ctx context.Context, actor string) ([]Snapshot, error) {
	r.listCalls++
	cases, err := r.memoryRepository.ListOwnCases(ctx, actor)
	if r.cancelOn == "list" {
		r.cancel()
	}
	return cases, err
}

func (r *cancelingOwnCaseReaderRepository) LoadOwnCase(ctx context.Context, actor, id string) (Snapshot, error) {
	r.loadCalls++
	item, err := r.memoryRepository.LoadOwnCase(ctx, actor, id)
	if r.cancelOn == "load" {
		r.cancel()
	}
	return item, err
}

func TestOwnerCaseReadsStopWhenContextCancels(t *testing.T) {
	for _, tc := range []struct {
		name          string
		operation     string
		cancelBefore  bool
		cancelOnRead  string
		wantReadCalls int
	}{
		{name: "list skips repository when pre-canceled", operation: "list", cancelBefore: true},
		{name: "list discards result when read cancels", operation: "list", cancelOnRead: "list", wantReadCalls: 1},
		{name: "load skips repository when pre-canceled", operation: "load", cancelBefore: true},
		{name: "load discards result when read cancels", operation: "load", cancelOnRead: "load", wantReadCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, base, snapshot := fixture(t, PurposeIdentity)
			ctx, cancel := context.WithCancel(context.Background())
			if tc.cancelBefore {
				cancel()
			}
			repo := &cancelingOwnCaseReaderRepository{memoryRepository: base, cancel: cancel, cancelOn: tc.cancelOnRead}
			service.repo = repo

			var err error
			var hasResult bool
			if tc.operation == "list" {
				var cases []Snapshot
				cases, err = service.ListOwnCases(ctx, subject)
				hasResult = len(cases) != 0
			} else {
				var got Snapshot
				got, err = service.LoadOwnCase(ctx, subject, snapshot.ID)
				hasResult = !reflect.DeepEqual(got, Snapshot{})
			}
			if !errors.Is(err, context.Canceled) || hasResult {
				t.Fatalf("canceled %s returned result=%v err=%v", tc.operation, hasResult, err)
			}
			calls := repo.listCalls
			if tc.operation == "load" {
				calls = repo.loadCalls
			}
			if calls != tc.wantReadCalls {
				t.Fatalf("%s repository calls=%d, want %d", tc.operation, calls, tc.wantReadCalls)
			}
		})
	}
}
