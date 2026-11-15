package privacy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockReviewAndRetentionRepo implements the various privacy repository interfaces
// for local unit tests without requiring a real database.
type mockReviewAndRetentionRepo struct {
	profile                 Profile
	requests                []RightsRequest
	getProfileErr           error
	listRequestsErr         error
	reviewResult            SuppressionReview
	reviewErr               error
	pendingItems            []SuppressionQueueItem
	pendingErr              error
	purgeReservationResult  RetentionPurgeResult
	purgeReservationErr     error
	prepareRetentionResults []RetentionPurgeResult
	prepareRetentionErrs    []error
	prepareRetentionErr     error
	claimFilesResult        []SuppressionFile
	claimFilesErr           error
	completeFileErr         error
	failFileErr             error
	domainContractsPurged   int64
	domainPublicationPurged int64
	domainPurgeErr          error
	executionResult         SuppressionExecution
	executionErr            error
	finishResult            SuppressionExecution
	finishErr               error
	pendingRequests         []string
	pendingRequestsErr      error
	claimPendingFilesResult []SuppressionFile
	claimPendingFilesErr    error

	reviewCalls               int
	lastReviewerID            string
	lastRequestID             string
	lastIdempotencyKey        string
	lastCorrelationID         string
	lastReviewAt              time.Time
	lastReservationPurgeAt    time.Time
	lastReservationPurgeLimit int
	completedFiles            []SuppressionFile
	failedFiles               []SuppressionFile
	completedSuppressionFiles []SuppressionFile
	failedSuppressionFiles    []SuppressionFile
}

func (m *mockReviewAndRetentionRepo) GetProfile(_ context.Context, accountID string) (Profile, error) {
	if m.getProfileErr != nil {
		return Profile{}, m.getProfileErr
	}
	if m.profile.AccountID == "" {
		return Profile{AccountID: accountID, Name: "Mock"}, nil
	}
	return m.profile, nil
}

func (m *mockReviewAndRetentionRepo) SaveProfile(context.Context, string, string, *string) (Profile, error) {
	return Profile{}, nil
}

func (m *mockReviewAndRetentionRepo) CreateRightsRequest(context.Context, string, string, string) (RightsRequest, error) {
	return RightsRequest{}, nil
}

func (m *mockReviewAndRetentionRepo) ListOwnRightsRequests(_ context.Context, _ string) ([]RightsRequest, error) {
	if m.listRequestsErr != nil {
		return nil, m.listRequestsErr
	}
	return m.requests, nil
}

func (m *mockReviewAndRetentionRepo) ExportOwnData(context.Context, string) (OwnData, error) {
	return OwnData{}, nil
}

func (m *mockReviewAndRetentionRepo) ReviewSuppression(_ context.Context, reviewerID, requestID, idempotencyKey, correlationID string, now func() time.Time) (SuppressionReview, error) {
	m.reviewCalls++
	m.lastReviewerID = reviewerID
	m.lastRequestID = requestID
	m.lastIdempotencyKey = idempotencyKey
	m.lastCorrelationID = correlationID
	if now != nil {
		m.lastReviewAt = now()
	}
	if m.reviewErr != nil {
		return SuppressionReview{}, m.reviewErr
	}
	return m.reviewResult, nil
}

func (m *mockReviewAndRetentionRepo) ListPendingSuppressions(context.Context) ([]SuppressionQueueItem, error) {
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
	return m.pendingItems, nil
}

func (m *mockReviewAndRetentionRepo) PurgeExpiredReservationLinks(_ context.Context, at time.Time, limit int) (RetentionPurgeResult, error) {
	m.lastReservationPurgeAt = at
	m.lastReservationPurgeLimit = limit
	if m.purgeReservationErr != nil {
		return RetentionPurgeResult{}, m.purgeReservationErr
	}
	return m.purgeReservationResult, nil
}

func (m *mockReviewAndRetentionRepo) ExportCompletedSuppressions(context.Context) (SuppressionReplayManifest, error) {
	return SuppressionReplayManifest{}, nil
}

func (m *mockReviewAndRetentionRepo) PrepareSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, time.Time) (string, string, string, bool, error) {
	return "", "", "", false, nil
}

func (m *mockReviewAndRetentionRepo) FinishSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, string, time.Time) error {
	return nil
}

func (m *mockReviewAndRetentionRepo) PrepareExpiredLocalRetentions(context.Context, time.Time, int) (RetentionPurgeResult, error) {
	if len(m.prepareRetentionErrs) > 0 {
		err := m.prepareRetentionErrs[0]
		m.prepareRetentionErrs = m.prepareRetentionErrs[1:]
		if err != nil {
			return RetentionPurgeResult{}, err
		}
	}
	if m.prepareRetentionErr != nil {
		return RetentionPurgeResult{}, m.prepareRetentionErr
	}
	if len(m.prepareRetentionResults) > 0 {
		res := m.prepareRetentionResults[0]
		m.prepareRetentionResults = m.prepareRetentionResults[1:]
		return res, nil
	}
	return RetentionPurgeResult{}, nil
}

func (m *mockReviewAndRetentionRepo) ClaimExpiredVerificationFiles(context.Context, time.Time, int) ([]SuppressionFile, error) {
	if m.claimFilesErr != nil {
		return nil, m.claimFilesErr
	}
	return m.claimFilesResult, nil
}

func (m *mockReviewAndRetentionRepo) CompleteExpiredVerificationFile(_ context.Context, file SuppressionFile, _ time.Time) error {
	m.completedFiles = append(m.completedFiles, file)
	return m.completeFileErr
}

func (m *mockReviewAndRetentionRepo) FailExpiredVerificationFile(_ context.Context, file SuppressionFile, _ time.Time) error {
	m.failedFiles = append(m.failedFiles, file)
	return m.failFileErr
}

func (m *mockReviewAndRetentionRepo) PurgeExpiredLocalDomainDocuments(context.Context, time.Time, int) (int64, int64, error) {
	if m.domainPurgeErr != nil {
		return 0, 0, m.domainPurgeErr
	}
	return m.domainContractsPurged, m.domainPublicationPurged, nil
}

func (m *mockReviewAndRetentionRepo) ExecuteSuppression(context.Context, string, string, string, string, func() time.Time) (SuppressionExecution, error) {
	if m.executionErr != nil {
		return SuppressionExecution{}, m.executionErr
	}
	return m.executionResult, nil
}

func (m *mockReviewAndRetentionRepo) ClaimPendingSuppressionFiles(context.Context, string, string, time.Time, time.Time, int) ([]SuppressionFile, error) {
	if m.claimPendingFilesErr != nil {
		return nil, m.claimPendingFilesErr
	}
	return m.claimPendingFilesResult, nil
}

func (m *mockReviewAndRetentionRepo) CompleteSuppressionFile(_ context.Context, file SuppressionFile, _ time.Time) error {
	m.completedSuppressionFiles = append(m.completedSuppressionFiles, file)
	return nil
}

func (m *mockReviewAndRetentionRepo) FailSuppressionFile(_ context.Context, file SuppressionFile, _ time.Time) error {
	m.failedSuppressionFiles = append(m.failedSuppressionFiles, file)
	return nil
}

func (m *mockReviewAndRetentionRepo) FinishSuppression(context.Context, string, time.Time) (SuppressionExecution, error) {
	if m.finishErr != nil {
		return SuppressionExecution{}, m.finishErr
	}
	return m.finishResult, nil
}

func (m *mockReviewAndRetentionRepo) PendingSuppressionRequests(context.Context) ([]string, error) {
	if m.pendingRequestsErr != nil {
		return nil, m.pendingRequestsErr
	}
	return m.pendingRequests, nil
}

type mockEvidenceCleaner struct {
	deleteErr  error
	deletedIDs []string
}

func (c *mockEvidenceCleaner) Delete(_ context.Context, storageID string) error {
	if c.deleteErr != nil {
		return c.deleteErr
	}
	c.deletedIDs = append(c.deletedIDs, storageID)
	return nil
}

func TestProfileAndOwnRequestsBoundaries(t *testing.T) {
	repo := &mockReviewAndRetentionRepo{
		profile:  Profile{AccountID: "acc-1", Name: "Ada Ejemplo"},
		requests: []RightsRequest{{ID: "req-1", Kind: "acceso", State: "en_revision"}},
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("unexpected NewService error: %v", err)
	}

	// 1. Profile boundaries
	if _, err := svc.Profile(context.Background(), ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for blank accountID in Profile, got %v", err)
	}
	p, err := svc.Profile(context.Background(), "acc-1")
	if err != nil || p.Name != "Ada Ejemplo" {
		t.Fatalf("unexpected Profile result: %+v, err: %v", p, err)
	}
	profileErr := errors.New("profile repository failure")
	repo.getProfileErr = profileErr
	if _, err := svc.Profile(context.Background(), "acc-1"); !errors.Is(err, profileErr) {
		t.Fatalf("expected profileErr, got: %v", err)
	}
	repo.getProfileErr = nil

	// 2. OwnRequests boundaries
	if _, err := svc.OwnRequests(context.Background(), ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for blank accountID in OwnRequests, got %v", err)
	}
	reqs, err := svc.OwnRequests(context.Background(), "acc-1")
	if err != nil || len(reqs) != 1 || reqs[0].ID != "req-1" {
		t.Fatalf("unexpected OwnRequests result: %+v, err: %v", reqs, err)
	}
	listErr := errors.New("list requests repository failure")
	repo.listRequestsErr = listErr
	if _, err := svc.OwnRequests(context.Background(), "acc-1"); !errors.Is(err, listErr) {
		t.Fatalf("expected listErr, got: %v", err)
	}
	repo.listRequestsErr = nil
}

func TestSuppressionReviewAndPendingQueueBoundaries(t *testing.T) {
	// 1. Repository not implementing SuppressionReviewRepository
	svcBase, err := NewService(&memoryRepository{})
	if err != nil {
		t.Fatalf("unexpected NewService error: %v", err)
	}
	if _, err := svcBase.ReviewSuppression(context.Background(), "rev-1", "req-1", "idem-key", "corr-1", nil); err == nil || !strings.Contains(err.Error(), "suppression review repository unavailable") {
		t.Fatalf("expected suppression review repository unavailable error, got: %v", err)
	}
	if _, err := svcBase.PendingSuppressions(context.Background()); err == nil || !strings.Contains(err.Error(), "suppression review repository unavailable") {
		t.Fatalf("expected pending suppressions repository unavailable error, got: %v", err)
	}

	// 2. Input validation boundaries
	repo := &mockReviewAndRetentionRepo{
		reviewResult: SuppressionReview{
			RequestID:   "req-1",
			Outcome:     "aprobada",
			Obligations: []string{"ninguna"},
		},
		pendingItems: []SuppressionQueueItem{{
			RequestID: "req-1",
			CreatedAt: time.Unix(100, 0),
		}},
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("unexpected NewService error: %v", err)
	}

	oversizedKey := strings.Repeat("k", 201)
	oversizedCorr := strings.Repeat("c", 121)

	table := []struct {
		name    string
		revID   string
		reqID   string
		idemKey string
		corrID  string
	}{
		{"blank reviewer", "", "req-1", "idem-1", "corr-1"},
		{"blank request", "rev-1", "", "idem-1", "corr-1"},
		{"blank idempotency key", "rev-1", "req-1", "   ", "corr-1"},
		{"oversized idempotency key", "rev-1", "req-1", oversizedKey, "corr-1"},
		{"blank correlation id", "rev-1", "req-1", "idem-1", ""},
		{"oversized correlation id", "rev-1", "req-1", "idem-1", oversizedCorr},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.ReviewSuppression(context.Background(), tc.revID, tc.reqID, tc.idemKey, tc.corrID, nil); !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected ErrInvalid for %s, got: %v", tc.name, err)
			}
		})
	}
	if repo.reviewCalls != 0 {
		t.Fatalf("expected zero review repository calls on invalid inputs, got: %d", repo.reviewCalls)
	}

	// 3. Valid review with nil clock and custom clock
	rev, err := svc.ReviewSuppression(context.Background(), "rev-1", "req-1", "idem-1", "corr-1", nil)
	if err != nil || rev.Outcome != "aprobada" || repo.lastReviewerID != "rev-1" || repo.lastRequestID != "req-1" || repo.lastIdempotencyKey != "idem-1" || repo.lastCorrelationID != "corr-1" {
		t.Fatalf("unexpected ReviewSuppression result or forwarded fields: %+v, err: %v", rev, err)
	}

	customTime := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	revCustom, err := svc.ReviewSuppression(context.Background(), "rev-2", "req-2", "idem-2", "corr-2", func() time.Time { return customTime })
	if err != nil || repo.lastReviewerID != "rev-2" || !repo.lastReviewAt.Equal(customTime) {
		t.Fatalf("unexpected ReviewSuppression custom clock result: %+v, at: %v, err: %v", revCustom, repo.lastReviewAt, err)
	}

	// 4. ReviewSuppression repository error propagation
	sentinelReviewErr := errors.New("review repository transaction failed")
	repo.reviewErr = sentinelReviewErr
	if _, err := svc.ReviewSuppression(context.Background(), "rev-1", "req-1", "idem-1", "corr-1", nil); !errors.Is(err, sentinelReviewErr) {
		t.Fatalf("expected sentinelReviewErr, got: %v", err)
	}
	repo.reviewErr = nil

	// 5. Pending queue item retrieval and error propagation
	pending, err := svc.PendingSuppressions(context.Background())
	if err != nil || len(pending) != 1 || pending[0].RequestID != "req-1" {
		t.Fatalf("unexpected PendingSuppressions result: %+v, err: %v", pending, err)
	}
	sentinelPendingErr := errors.New("pending list query failed")
	repo.pendingErr = sentinelPendingErr
	if _, err := svc.PendingSuppressions(context.Background()); !errors.Is(err, sentinelPendingErr) {
		t.Fatalf("expected sentinelPendingErr, got: %v", err)
	}
	repo.pendingErr = nil
	repo.pendingItems = nil
	emptyPending, err := svc.PendingSuppressions(context.Background())
	if err != nil || len(emptyPending) != 0 {
		t.Fatalf("expected empty pending queue result, got: %+v, err: %v", emptyPending, err)
	}
}

func TestPurgeExpiredReservationLinksBoundaries(t *testing.T) {
	// 1. Repo not implementing LocalPrivacyOperationsRepository
	svcBase, err := NewService(&memoryRepository{})
	if err != nil {
		t.Fatalf("unexpected NewService error: %v", err)
	}
	if _, err := svcBase.PurgeExpiredReservationLinks(context.Background(), time.Now(), 100); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for non-supporting repo, got: %v", err)
	}

	// 2. Limit boundary checks (< 1 or > 500)
	repo := &mockReviewAndRetentionRepo{
		purgeReservationResult: RetentionPurgeResult{TermsPurged: 5},
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("unexpected NewService error: %v", err)
	}
	if _, err := svc.PurgeExpiredReservationLinks(context.Background(), time.Now(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for limit 0, got: %v", err)
	}
	if _, err := svc.PurgeExpiredReservationLinks(context.Background(), time.Now(), -5); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for negative limit, got: %v", err)
	}
	if _, err := svc.PurgeExpiredReservationLinks(context.Background(), time.Now(), 501); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for limit 501, got: %v", err)
	}

	// 3. Success check with UTC conversion and result passthrough
	loc := time.FixedZone("TEST", -4*3600)
	nonUTC := time.Date(2030, 5, 10, 15, 30, 0, 0, loc)
	result, err := svc.PurgeExpiredReservationLinks(context.Background(), nonUTC, 50)
	if err != nil || result.TermsPurged != 5 {
		t.Fatalf("unexpected PurgeExpiredReservationLinks result: %+v, err: %v", result, err)
	}
	if repo.lastReservationPurgeAt.Location() != time.UTC || repo.lastReservationPurgeLimit != 50 {
		t.Fatalf("expected time in UTC and limit 50, got at=%v, limit=%d", repo.lastReservationPurgeAt, repo.lastReservationPurgeLimit)
	}

	// 4. Error propagation
	sentinelErr := errors.New("db connection error")
	repo.purgeReservationErr = sentinelErr
	if _, err := svc.PurgeExpiredReservationLinks(context.Background(), time.Now(), 50); !errors.Is(err, sentinelErr) {
		t.Fatalf("expected error propagation, got: %v", err)
	}
}

func TestPurgeExpiredLocalRetentionsBoundaries(t *testing.T) {
	cleaner := &mockEvidenceCleaner{}
	now := time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC)

	// 1. Missing interface or invalid arguments
	svcBase, _ := NewService(&memoryRepository{})
	if _, err := svcBase.PurgeExpiredLocalRetentions(context.Background(), now, 100, cleaner); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid when repo lacks LocalRetentionRepository, got: %v", err)
	}

	repo := &mockReviewAndRetentionRepo{}
	svc, _ := NewService(repo)
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 100, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil cleaner, got: %v", err)
	}
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 0, cleaner); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for limit 0, got: %v", err)
	}
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 501, cleaner); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for limit 501, got: %v", err)
	}

	// 2. Reservation purge failure propagation
	resErr := errors.New("reservation purge failed")
	repo.purgeReservationErr = resErr
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); !errors.Is(err, resErr) {
		t.Fatalf("expected resErr, got: %v", err)
	}
	repo.purgeReservationErr = nil

	// 3. Prepare retentions failure propagation
	prepErr := errors.New("prepare retentions failed")
	repo.prepareRetentionErr = prepErr
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); !errors.Is(err, prepErr) {
		t.Fatalf("expected prepErr, got: %v", err)
	}
	repo.prepareRetentionErr = nil

	// 4. Claim expired files failure propagation
	claimErr := errors.New("claim files failed")
	repo.claimFilesErr = claimErr
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); !errors.Is(err, claimErr) {
		t.Fatalf("expected claimErr, got: %v", err)
	}
	repo.claimFilesErr = nil

	// 5. Evidence file deletion failure with fail save error
	repo.claimFilesResult = []SuppressionFile{{
		StorageID: "storage-fail-1",
		RequestID: "req-1",
	}}
	cleaner.deleteErr = errors.New("cloud storage deletion timed out")
	repo.failFileErr = errors.New("failed to update file state")
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); err == nil || !strings.Contains(err.Error(), "failed to update file state") {
		t.Fatalf("expected failFileErr, got: %v", err)
	}

	// 6. Evidence file deletion failure with successful fail state recording (deferred increment)
	repo.failFileErr = nil
	cleaner.deleteErr = errors.New("storage unavailable")
	repo.prepareRetentionResults = []RetentionPurgeResult{{
		TermsPurged: 1,
	}, {
		RightsPurged: 2,
	}}
	resDeferred, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner)
	if err != nil {
		t.Fatalf("unexpected error when deferred file is recorded: %v", err)
	}
	if resDeferred.EvidenceFilesDeferred != 1 || resDeferred.EvidenceFilesRemoved != 0 {
		t.Fatalf("expected deferred=1, removed=0, got %+v", resDeferred)
	}
	if len(repo.failedFiles) == 0 || repo.failedFiles[0].StorageID != "storage-fail-1" {
		t.Fatalf("expected failedFile recorded, got: %+v", repo.failedFiles)
	}

	// 7. Evidence file deletion success with CompleteExpiredVerificationFile failure
	cleaner.deleteErr = nil
	compErr := errors.New("complete file state failure")
	repo.completeFileErr = compErr
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); !errors.Is(err, compErr) {
		t.Fatalf("expected completeFileErr, got: %v", err)
	}
	repo.completeFileErr = nil

	// 7b. Finalized PrepareExpiredLocalRetentions failure propagation
	repo.prepareRetentionErrs = []error{nil, errors.New("finalized prepare failed")}
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); err == nil || !strings.Contains(err.Error(), "finalized prepare failed") {
		t.Fatalf("expected finalized prepare error, got: %v", err)
	}
	repo.prepareRetentionErrs = nil

	// 8. Full successful purge pass with count accumulation and domain document retention
	cleaner.deletedIDs = nil
	repo.completedFiles = nil
	repo.claimFilesResult = []SuppressionFile{
		{StorageID: "storage-ok-1", RequestID: "req-1"},
		{StorageID: "storage-ok-2", RequestID: "req-2"},
	}
	repo.purgeReservationResult = RetentionPurgeResult{TermsPurged: 3}
	repo.prepareRetentionResults = []RetentionPurgeResult{
		{RightsPurged: 4, QuotesPurged: 2, VerificationsPurged: 1, EvidenceFilesQueued: 5},
		{RightsPurged: 1, QuotesPurged: 1, VerificationsPurged: 2, EvidenceFilesQueued: 1},
	}
	repo.domainContractsPurged = 10
	repo.domainPublicationPurged = 20

	fullResult, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner)
	if err != nil {
		t.Fatalf("unexpected PurgeExpiredLocalRetentions error: %v", err)
	}
	if fullResult.TermsPurged != 3 {
		t.Errorf("expected TermsPurged=3, got %d", fullResult.TermsPurged)
	}
	if fullResult.RightsPurged != 5 {
		t.Errorf("expected RightsPurged=5 (4+1), got %d", fullResult.RightsPurged)
	}
	if fullResult.QuotesPurged != 3 {
		t.Errorf("expected QuotesPurged=3 (2+1), got %d", fullResult.QuotesPurged)
	}
	if fullResult.VerificationsPurged != 3 {
		t.Errorf("expected VerificationsPurged=3 (1+2), got %d", fullResult.VerificationsPurged)
	}
	if fullResult.EvidenceFilesQueued != 6 {
		t.Errorf("expected EvidenceFilesQueued=6 (5+1), got %d", fullResult.EvidenceFilesQueued)
	}
	if fullResult.EvidenceFilesRemoved != 2 {
		t.Errorf("expected EvidenceFilesRemoved=2, got %d", fullResult.EvidenceFilesRemoved)
	}
	if fullResult.ContractsPurged != 10 {
		t.Errorf("expected ContractsPurged=10, got %d", fullResult.ContractsPurged)
	}
	if fullResult.PublicationHistoryPurged != 20 {
		t.Errorf("expected PublicationHistoryPurged=20, got %d", fullResult.PublicationHistoryPurged)
	}
	if len(cleaner.deletedIDs) != 2 || len(repo.completedFiles) != 2 {
		t.Errorf("expected 2 cleaner deletions and 2 completed files, got cleaner=%v completed=%v", cleaner.deletedIDs, repo.completedFiles)
	}

	// 9. Domain purge failure propagation
	domainErr := errors.New("domain purge failed")
	repo.domainPurgeErr = domainErr
	if _, err := svc.PurgeExpiredLocalRetentions(context.Background(), now, 50, cleaner); !errors.Is(err, domainErr) {
		t.Fatalf("expected domainErr, got: %v", err)
	}
}

func TestExecuteSuppressionAndCleanupOnceBoundaries(t *testing.T) {
	cleaner := &mockEvidenceCleaner{}
	now := func() time.Time { return time.Date(2030, 5, 10, 12, 0, 0, 0, time.UTC) }

	// 1. Missing SuppressionExecutionRepository on base repo
	svcBase, _ := NewService(&memoryRepository{})
	if _, err := svcBase.ExecuteSuppression(context.Background(), "act-1", "req-1", "key-1", "corr-1", now, cleaner); err == nil || !strings.Contains(err.Error(), "suppression execution repository unavailable") {
		t.Fatalf("expected suppression execution repository unavailable error, got: %v", err)
	}
	if err := svcBase.RunSuppressionCleanupOnce(context.Background(), now, cleaner); err == nil || !strings.Contains(err.Error(), "suppression execution repository unavailable") {
		t.Fatalf("expected suppression execution repository unavailable error for cleanup once, got: %v", err)
	}

	// 2. Input validation on ExecuteSuppression
	repo := &mockReviewAndRetentionRepo{
		executionResult: SuppressionExecution{RequestID: "req-1", Status: "bloqueada"},
	}
	svc, _ := NewService(repo)
	oversizedKey := strings.Repeat("k", 201)
	oversizedCorr := strings.Repeat("c", 121)
	table := []struct {
		name    string
		actorID string
		reqID   string
		key     string
		corrID  string
	}{
		{"blank actor", "", "req-1", "key-1", "corr-1"},
		{"blank req", "act-1", "", "key-1", "corr-1"},
		{"blank key", "act-1", "req-1", "  ", "corr-1"},
		{"oversized key", "act-1", "req-1", oversizedKey, "corr-1"},
		{"blank corr", "act-1", "req-1", "key-1", ""},
		{"oversized corr", "act-1", "req-1", "key-1", oversizedCorr},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.ExecuteSuppression(context.Background(), tc.actorID, tc.reqID, tc.key, tc.corrID, now, cleaner); !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected ErrInvalid for %s, got: %v", tc.name, err)
			}
		})
	}

	// 3. Status bloqueada returns immediately without cleaner invocations
	resBloq, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-1", "key-1", "corr-1", now, cleaner)
	if err != nil || resBloq.Status != "bloqueada" || len(cleaner.deletedIDs) != 0 {
		t.Fatalf("unexpected bloqueada execution: %+v, err: %v", resBloq, err)
	}

	// 4. Status completada returns immediately without cleaner invocations
	repo.executionResult = SuppressionExecution{RequestID: "req-2", Status: "completada"}
	resComp, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-2", "key-2", "corr-2", nil, cleaner)
	if err != nil || resComp.Status != "completada" || len(cleaner.deletedIDs) != 0 {
		t.Fatalf("unexpected completada execution: %+v, err: %v", resComp, err)
	}

	// 5. Cleaner is nil returns result directly
	repo.executionResult = SuppressionExecution{RequestID: "req-3", Status: "limpieza_pendiente"}
	resNilCleaner, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-3", "key-3", "corr-3", now, nil)
	if err != nil || resNilCleaner.Status != "limpieza_pendiente" {
		t.Fatalf("unexpected nil cleaner execution: %+v, err: %v", resNilCleaner, err)
	}

	// 6. ExecuteSuppression repository error propagation
	sentinelExecErr := errors.New("execution repository failed")
	repo.executionErr = sentinelExecErr
	if _, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-3", "key-3", "corr-3", now, cleaner); !errors.Is(err, sentinelExecErr) {
		t.Fatalf("expected sentinelExecErr, got: %v", err)
	}
	repo.executionErr = nil

	// 7. Active cleanup pass with claimable files, cleaner deletion and finish suppression
	cleaner.deletedIDs = nil
	repo.claimPendingFilesResult = []SuppressionFile{{
		StorageID:  "storage-clean-123",
		RequestID:  "req-3",
		ClaimUntil: time.Now().Add(time.Hour),
	}}
	repo.finishResult = SuppressionExecution{RequestID: "req-3", Status: "completada"}
	resFull, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-3", "key-3", "corr-3", now, cleaner)
	if err != nil || resFull.Status != "completada" {
		t.Fatalf("expected finished suppression with completada status, got %+v, err: %v", resFull, err)
	}
	if len(cleaner.deletedIDs) != 1 || cleaner.deletedIDs[0] != "storage-clean-123" {
		t.Fatalf("expected cleaner to delete storage-clean-123, got: %v", cleaner.deletedIDs)
	}
	if len(repo.completedSuppressionFiles) != 1 || repo.completedSuppressionFiles[0].StorageID != "storage-clean-123" {
		t.Fatalf("expected completedSuppressionFiles recorded, got: %+v", repo.completedSuppressionFiles)
	}

	// 8. Cleanup pass failure propagation (claim failure and finish failure)
	claimFailErr := errors.New("claim pending files failed")
	repo.claimPendingFilesErr = claimFailErr
	if _, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-3", "key-3", "corr-3", now, cleaner); !errors.Is(err, claimFailErr) {
		t.Fatalf("expected claimFailErr, got: %v", err)
	}
	repo.claimPendingFilesErr = nil

	finishFailErr := errors.New("finish suppression failed")
	repo.finishErr = finishFailErr
	if _, err := svc.ExecuteSuppression(context.Background(), "act-1", "req-3", "key-3", "corr-3", now, cleaner); !errors.Is(err, finishFailErr) {
		t.Fatalf("expected finishFailErr, got: %v", err)
	}
	repo.finishErr = nil

	// 9. RunSuppressionCleanupOnceWithWorker validation
	// Non-UUID workerID returns ErrInvalid
	if err := svc.RunSuppressionCleanupOnceWithWorker(context.Background(), "invalid-worker", now, cleaner); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for invalid worker ID, got: %v", err)
	}
	// nil cleaner returns nil
	validWorker := "10000000-0000-4000-8000-000000000001"
	if err := svc.RunSuppressionCleanupOnceWithWorker(context.Background(), validWorker, now, nil); err != nil {
		t.Fatalf("expected nil for nil cleaner in cleanup once, got: %v", err)
	}
	// PendingSuppressionRequests error propagation
	repo.pendingRequestsErr = errors.New("pending list failed")
	if err := svc.RunSuppressionCleanupOnceWithWorker(context.Background(), validWorker, now, cleaner); err == nil || !strings.Contains(err.Error(), "pending list failed") {
		t.Fatalf("expected pendingRequestsErr, got: %v", err)
	}
	repo.pendingRequestsErr = nil

	// 7. Successful RunSuppressionCleanupOnce with empty pending requests
	repo.pendingRequests = nil
	if err := svc.RunSuppressionCleanupOnce(context.Background(), now, cleaner); err != nil {
		t.Fatalf("unexpected RunSuppressionCleanupOnce error on empty list: %v", err)
	}
}
