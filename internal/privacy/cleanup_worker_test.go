package privacy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type cleanupLeaseRow struct {
	file      SuppressionFile
	available time.Time
	complete  bool
}

type cleanupLeaseRepository struct {
	mu       sync.Mutex
	rows     []*cleanupLeaseRow
	requests []string
}

func (*cleanupLeaseRepository) GetProfile(context.Context, string) (Profile, error) {
	return Profile{}, ErrNotFound
}
func (*cleanupLeaseRepository) SaveProfile(context.Context, string, string, *string) (Profile, error) {
	return Profile{}, ErrInvalid
}
func (*cleanupLeaseRepository) CreateRightsRequest(context.Context, string, string, string) (RightsRequest, error) {
	return RightsRequest{}, ErrInvalid
}
func (*cleanupLeaseRepository) ListOwnRightsRequests(context.Context, string) ([]RightsRequest, error) {
	return nil, nil
}
func (*cleanupLeaseRepository) ExportOwnData(context.Context, string) (OwnData, error) {
	return OwnData{}, ErrExportUnavailable
}

func (r *cleanupLeaseRepository) ExecuteSuppression(_ context.Context, _, requestID, _, _ string, _ func() time.Time) (SuppressionExecution, error) {
	return SuppressionExecution{RequestID: requestID, Status: "limpieza_pendiente", PendingFiles: len(r.rows)}, nil
}

func (r *cleanupLeaseRepository) ClaimPendingSuppressionFiles(_ context.Context, requestID, workerID string, now, leaseUntil time.Time, limit int) ([]SuppressionFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	claimed := make([]SuppressionFile, 0, limit)
	for _, row := range r.rows {
		if row.file.RequestID != requestID || row.complete || row.available.After(now) {
			continue
		}
		if !row.file.ClaimUntil.IsZero() && row.file.ClaimUntil.After(now) {
			continue
		}
		row.file.ClaimOwner = workerID
		row.file.ClaimUntil = leaseUntil
		row.file.ClaimGeneration++
		claimed = append(claimed, row.file)
		if len(claimed) == limit {
			break
		}
	}
	return claimed, nil
}

func (r *cleanupLeaseRepository) CompleteSuppressionFile(_ context.Context, file SuppressionFile, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.row(file)
	if row == nil || row.complete || row.file.ClaimOwner != file.ClaimOwner || row.file.ClaimGeneration != file.ClaimGeneration || !row.file.ClaimUntil.After(at) {
		return ErrSuppressionClaimLost
	}
	row.complete = true
	row.file.ClaimOwner = ""
	row.file.ClaimUntil = time.Time{}
	return nil
}

func (r *cleanupLeaseRepository) FailSuppressionFile(_ context.Context, file SuppressionFile, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.row(file)
	if row == nil || row.complete || row.file.ClaimOwner != file.ClaimOwner || row.file.ClaimGeneration != file.ClaimGeneration || !row.file.ClaimUntil.After(at) {
		return ErrSuppressionClaimLost
	}
	row.available = at.Add(5 * time.Second)
	row.file.ClaimOwner = ""
	row.file.ClaimUntil = time.Time{}
	return nil
}

func (r *cleanupLeaseRepository) FinishSuppression(_ context.Context, requestID string, _ time.Time) (SuppressionExecution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := 0
	for _, row := range r.rows {
		if row.file.RequestID == requestID && !row.complete {
			pending++
		}
	}
	status := "completada"
	if pending > 0 {
		status = "limpieza_pendiente"
	}
	return SuppressionExecution{RequestID: requestID, Status: status, PendingFiles: pending}, nil
}

func (r *cleanupLeaseRepository) PendingSuppressionRequests(context.Context) ([]string, error) {
	return append([]string(nil), r.requests...), nil
}

func (r *cleanupLeaseRepository) row(file SuppressionFile) *cleanupLeaseRow {
	for _, row := range r.rows {
		if row.file.ExecutionID == file.ExecutionID && row.file.EvidenceID == file.EvidenceID {
			return row
		}
	}
	return nil
}

type cleanupLeaseCleaner func(context.Context, string) error

func (c cleanupLeaseCleaner) Delete(ctx context.Context, id string) error { return c(ctx, id) }

func TestSuppressionCleanupStopsAfterLeaseExpiresAndTakeoverFencesOldAck(t *testing.T) {
	workerA := "77777777-0000-4000-8000-000000000001"
	workerB := "77777777-0000-4000-8000-000000000002"
	requestA := "77777777-0000-4000-8000-000000000010"
	requestB := "77777777-0000-4000-8000-000000000020"
	base := time.Now().UTC()
	clock := base
	repo := &cleanupLeaseRepository{requests: []string{requestA, requestB}}
	for i, evidenceID := range []string{
		"77777777-0000-4000-8000-000000000101",
		"77777777-0000-4000-8000-000000000102",
		"77777777-0000-4000-8000-000000000103",
	} {
		repo.rows = append(repo.rows, &cleanupLeaseRow{file: SuppressionFile{ExecutionID: "77777777-0000-4000-8000-000000000100", EvidenceID: evidenceID, StorageID: evidenceID, RequestID: requestA}, available: base.Add(time.Duration(i) * time.Nanosecond)})
	}
	repo.rows = append(repo.rows, &cleanupLeaseRow{file: SuppressionFile{ExecutionID: "77777777-0000-4000-8000-000000000200", EvidenceID: "77777777-0000-4000-8000-000000000201", StorageID: "77777777-0000-4000-8000-000000000201", RequestID: requestB}, available: base})

	var oldClaim SuppressionFile
	var deleted []string
	cleaner := cleanupLeaseCleaner(func(ctx context.Context, id string) error {
		deleted = append(deleted, id)
		if id == repo.rows[0].file.EvidenceID {
			deadline, ok := ctx.Deadline()
			if !ok || deadline.Sub(base.Add(defaultSuppressionCleanupLease)) > time.Second || base.Add(defaultSuppressionCleanupLease).Sub(deadline) > time.Second {
				t.Fatalf("delete context deadline=%v; want claim expiry near %v", deadline, base.Add(defaultSuppressionCleanupLease))
			}
			clock = deadline.Add(3 * time.Minute)
		}
		return nil
	})
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunSuppressionCleanupOnceWithWorker(context.Background(), workerA, func() time.Time { return clock }, cleaner); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 || deleted[0] != repo.rows[0].file.EvidenceID || deleted[1] != repo.rows[3].file.EvidenceID {
		t.Fatalf("worker started additional effects from the expired request batch: deleted=%v", deleted)
	}
	oldClaim = SuppressionFile{ExecutionID: repo.rows[0].file.ExecutionID, EvidenceID: repo.rows[0].file.EvidenceID, ClaimOwner: workerA, ClaimUntil: base.Add(defaultSuppressionCleanupLease), ClaimGeneration: 1}
	if repo.rows[0].complete || !repo.rows[3].complete || repo.rows[1].file.ClaimGeneration != 0 || repo.rows[2].file.ClaimGeneration != 0 {
		t.Fatalf("lease expiry state row1=%+v row2=%+v row3=%+v requestB=%+v", repo.rows[0], repo.rows[1], repo.rows[2], repo.rows[3])
	}

	takeoverAt := clock
	claims, err := repo.ClaimPendingSuppressionFiles(context.Background(), requestA, workerB, takeoverAt, takeoverAt.Add(time.Minute), 1)
	if err != nil || len(claims) != 1 || claims[0].EvidenceID != oldClaim.EvidenceID || claims[0].ClaimGeneration != oldClaim.ClaimGeneration+1 {
		t.Fatalf("takeover claim=%+v err=%v, want generation %d", claims, err, oldClaim.ClaimGeneration+1)
	}
	if err := repo.CompleteSuppressionFile(context.Background(), oldClaim, takeoverAt); !errors.Is(err, ErrSuppressionClaimLost) {
		t.Fatalf("old worker ack=%v, want claim lost", err)
	}
	if err := repo.FailSuppressionFile(context.Background(), oldClaim, takeoverAt); !errors.Is(err, ErrSuppressionClaimLost) {
		t.Fatalf("old worker retry=%v, want claim lost", err)
	}
	if err := repo.CompleteSuppressionFile(context.Background(), claims[0], takeoverAt.Add(time.Second)); err != nil {
		t.Fatalf("current worker ack after takeover: %v", err)
	}
}

func TestSuppressionCleanupCancelsDeleteAtLeaseDeadline(t *testing.T) {
	workerID := "77777777-0000-4000-8000-000000000001"
	requestID := "77777777-0000-4000-8000-000000000010"
	base := time.Now().UTC()
	repo := &cleanupLeaseRepository{requests: []string{requestID}, rows: []*cleanupLeaseRow{{
		file: SuppressionFile{ExecutionID: "77777777-0000-4000-8000-000000000100", EvidenceID: "77777777-0000-4000-8000-000000000101", StorageID: "77777777-0000-4000-8000-000000000101", RequestID: requestID}, available: base,
	}}}
	var deleteErr error
	completed, err := runSuppressionCleanupBatch(context.Background(), repo, requestID, workerID, time.Now, cleanupLeaseCleaner(func(ctx context.Context, _ string) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > defaultSuppressionCleanupLease || time.Until(deadline) <= 0 {
			t.Fatalf("delete context deadline=%v remaining=%s; want positive lease-bounded timeout", deadline, time.Until(deadline))
		}
		<-ctx.Done()
		deleteErr = ctx.Err()
		return deleteErr
	}), 50*time.Millisecond, 1)
	if err != nil || completed || !errors.Is(deleteErr, context.DeadlineExceeded) {
		t.Fatalf("cleanup result completed=%t delete error=%v err=%v", completed, deleteErr, err)
	}
}

func TestSuppressionReplayUsesOperationalClockForCleanupLease(t *testing.T) {
	requestID := "77777777-0000-4000-8000-000000000010"
	operationalNow := time.Now().UTC()
	repo := &cleanupLeaseRepository{rows: []*cleanupLeaseRow{{
		file: SuppressionFile{ExecutionID: "77777777-0000-4000-8000-000000000100", EvidenceID: "77777777-0000-4000-8000-000000000101", StorageID: "77777777-0000-4000-8000-000000000101", RequestID: requestID}, available: operationalNow,
	}}}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	historicalReplayTime := time.Date(2001, 1, 2, 3, 4, 5, 0, time.UTC)
	deleteCalls := 0
	result, err := service.ExecuteSuppression(context.Background(), "actor", requestID, "replay-key", "replay-correlation", func() time.Time { return historicalReplayTime }, cleanupLeaseCleaner(func(ctx context.Context, _ string) error {
		deleteCalls++
		deadline, ok := ctx.Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining <= 0 || remaining > defaultSuppressionCleanupLease {
			t.Fatalf("historical replay produced invalid operational delete deadline=%v remaining=%s", deadline, remaining)
		}
		return nil
	}))
	if err != nil || result.Status != "completada" || deleteCalls != 1 || !repo.rows[0].complete {
		t.Fatalf("historical replay cleanup result=%+v calls=%d complete=%t err=%v", result, deleteCalls, repo.rows[0].complete, err)
	}
}
