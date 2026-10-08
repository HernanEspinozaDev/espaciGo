package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	ErrInvalid  = errors.New("invalid privacy input")
	ErrNotFound = errors.New("privacy resource not found")
)

type Profile struct {
	AccountID string
	Name      string
	Phone     *string
	UpdatedAt time.Time
}

type RightsRequest struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

// SuppressionReview reports only structured obligation codes. It never executes
// suppression; unresolved retention rules keep an otherwise clear request in review.
type SuppressionReview struct {
	RequestID     string    `json:"request_id"`
	Outcome       string    `json:"outcome"`
	Obligations   []string  `json:"obligations_detected"`
	PendingChecks []string  `json:"pending_checks"`
	ReviewedAt    time.Time `json:"reviewed_at"`
	Reused        bool      `json:"reused"`
}

type SuppressionQueueItem struct {
	RequestID string    `json:"request_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SuppressionExecution is a local-only result. RetainedCodes explain residual
// links; this status never claims anonymization or full erasure.
type SuppressionExecution struct {
	RequestID    string     `json:"request_id"`
	Status       string     `json:"status"`
	Outcome      string     `json:"outcome"`
	DecisionCode string     `json:"decision_code"`
	Obligations  []string   `json:"obligations_detected"`
	Removed      []string   `json:"removed"`
	Retained     []string   `json:"retained"`
	PendingFiles int        `json:"pending_files"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Reused       bool       `json:"reused"`
}

type SuppressionFile struct {
	ExecutionID string
	EvidenceID  string
}

// SuppressionReplayEntry is the minimum outside-database record needed to
// reapply an already completed local suppression after restoring an older
// backup. It intentionally contains no contact, credential, or token data.
type SuppressionReplayEntry struct {
	ExecutionID string    `json:"execution_id"`
	RequestID   string    `json:"request_id"`
	AccountID   string    `json:"account_id"`
	RequestedAt time.Time `json:"requested_at"`
	CompletedAt time.Time `json:"completed_at"`
}

type SuppressionReplayManifest struct {
	Version int                      `json:"version"`
	Entries []SuppressionReplayEntry `json:"entries"`
}

type RetentionPurgeResult struct {
	Scanned  int `json:"scanned"`
	Purged   int `json:"purged"`
	Deferred int `json:"deferred"`
}

type SuppressionReplayResult struct {
	ExecutionID string `json:"execution_id"`
	AccountID   string `json:"account_id"`
	Status      string `json:"status"`
	Reused      bool   `json:"reused"`
}

type LocalPrivacyOperationsRepository interface {
	PurgeExpiredReservationLinks(context.Context, time.Time, int) (RetentionPurgeResult, error)
	ExportCompletedSuppressions(context.Context) (SuppressionReplayManifest, error)
	PrepareSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, time.Time) (requestID, operationKey, status string, reused bool, err error)
	FinishSuppressionReplay(context.Context, SuppressionReplayEntry, string, string, string, time.Time) error
}

type SuppressionExecutionRepository interface {
	ExecuteSuppression(context.Context, string, string, string, string, func() time.Time) (SuppressionExecution, error)
	PendingSuppressionFiles(context.Context, string, time.Time) ([]SuppressionFile, error)
	CompleteSuppressionFile(context.Context, SuppressionFile, time.Time) error
	FailSuppressionFile(context.Context, SuppressionFile, time.Time) error
	FinishSuppression(context.Context, string, time.Time) (SuppressionExecution, error)
	PendingSuppressionRequests(context.Context) ([]string, error)
}

type SyntheticEvidenceCleaner interface {
	Delete(context.Context, string) error
}

type SuppressionReviewRepository interface {
	ListPendingSuppressions(context.Context) ([]SuppressionQueueItem, error)
	ReviewSuppression(context.Context, string, string, string, string, func() time.Time) (SuppressionReview, error)
}

// OwnData is the bounded identity export available in the local prototype.
// It intentionally excludes credential hashes, sessions, action tokens and
// records belonging to other participants or domain owners.
type OwnData struct {
	Account     ExportAccount     `json:"account"`
	Profile     *ExportProfile    `json:"profile,omitempty"`
	Roles       []string          `json:"roles"`
	Acceptances []TermsAcceptance `json:"terms_acceptances"`
	Requests    []RightsRequest   `json:"rights_requests"`
	Scope       string            `json:"scope"`
}

type ExportAccount struct {
	Email         string    `json:"email"`
	State         string    `json:"state"`
	UsePreference *string   `json:"use_preference,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type ExportProfile struct {
	Name      string    `json:"display_name"`
	Phone     *string   `json:"phone,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TermsAcceptance struct {
	Code       string    `json:"code"`
	Type       string    `json:"type"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type Repository interface {
	GetProfile(context.Context, string) (Profile, error)
	SaveProfile(context.Context, string, string, *string) (Profile, error)
	CreateRightsRequest(context.Context, string, string, string) (RightsRequest, error)
	ListOwnRightsRequests(context.Context, string) ([]RightsRequest, error)
	ExportOwnData(context.Context, string) (OwnData, error)
}

type Service struct{ repo Repository }

var suppressionRegistryWriteMu sync.Mutex

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo}, nil
}

func (s *Service) Profile(ctx context.Context, accountID string) (Profile, error) {
	if accountID == "" {
		return Profile{}, ErrInvalid
	}
	return s.repo.GetProfile(ctx, accountID)
}

var nineDigits = regexp.MustCompile(`^[0-9]{9}$`)

func (s *Service) UpdateProfile(ctx context.Context, accountID, name, phone string) (Profile, error) {
	name = strings.TrimSpace(name)
	validName := name != "" && len([]rune(name)) <= 120
	for _, character := range name {
		validName = validName && (unicode.IsLetter(character) || character == ' ')
	}
	if accountID == "" || !validName {
		return Profile{}, ErrInvalid
	}
	var normalized *string
	if phone != "" {
		if !nineDigits.MatchString(phone) {
			return Profile{}, ErrInvalid
		}
		normalized = &phone
	}
	return s.repo.SaveProfile(ctx, accountID, name, normalized)
}

func (s *Service) RequestRight(ctx context.Context, accountID, kind, channel string) (RightsRequest, error) {
	if accountID == "" || (kind != "acceso" && kind != "supresion") || (channel != "web" && channel != "api") {
		return RightsRequest{}, ErrInvalid
	}
	return s.repo.CreateRightsRequest(ctx, accountID, kind, channel)
}

func (s *Service) OwnRequests(ctx context.Context, accountID string) ([]RightsRequest, error) {
	if accountID == "" {
		return nil, ErrInvalid
	}
	return s.repo.ListOwnRightsRequests(ctx, accountID)
}

func (s *Service) ExportOwnData(ctx context.Context, accountID string) (OwnData, error) {
	if accountID == "" {
		return OwnData{}, ErrInvalid
	}
	return s.repo.ExportOwnData(ctx, accountID)
}

func (s *Service) ReviewSuppression(ctx context.Context, reviewerID, requestID, idempotencyKey, correlationID string, now func() time.Time) (SuppressionReview, error) {
	if reviewerID == "" || requestID == "" || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 200 || correlationID == "" || len(correlationID) > 120 {
		return SuppressionReview{}, ErrInvalid
	}
	repo, ok := s.repo.(SuppressionReviewRepository)
	if !ok {
		return SuppressionReview{}, errors.New("privacy: suppression review repository unavailable")
	}
	if now == nil {
		now = time.Now
	}
	return repo.ReviewSuppression(ctx, reviewerID, requestID, idempotencyKey, correlationID, now)
}

func (s *Service) PendingSuppressions(ctx context.Context) ([]SuppressionQueueItem, error) {
	repo, ok := s.repo.(SuppressionReviewRepository)
	if !ok {
		return nil, errors.New("privacy: suppression review repository unavailable")
	}
	return repo.ListPendingSuppressions(ctx)
}

func (s *Service) ExecuteSuppression(ctx context.Context, actorID, requestID, key, correlationID string, now func() time.Time, cleaner SyntheticEvidenceCleaner) (SuppressionExecution, error) {
	if actorID == "" || requestID == "" || strings.TrimSpace(key) == "" || len(key) > 200 || correlationID == "" || len(correlationID) > 120 {
		return SuppressionExecution{}, ErrInvalid
	}
	repo, ok := s.repo.(SuppressionExecutionRepository)
	if !ok {
		return SuppressionExecution{}, errors.New("privacy: suppression execution repository unavailable")
	}
	if now == nil {
		now = time.Now
	}
	result, err := repo.ExecuteSuppression(ctx, actorID, requestID, key, correlationID, now)
	if err != nil || result.Status == "bloqueada" || result.Status == "completada" {
		return result, err
	}
	if cleaner == nil {
		return result, nil
	}
	files, err := repo.PendingSuppressionFiles(ctx, result.RequestID, now().UTC())
	if err != nil {
		return result, err
	}
	for _, file := range files {
		if err := cleaner.Delete(ctx, file.EvidenceID); err != nil {
			_ = repo.FailSuppressionFile(ctx, file, now().UTC())
			continue
		}
		if err := repo.CompleteSuppressionFile(ctx, file, now().UTC()); err != nil {
			return result, err
		}
	}
	return repo.FinishSuppression(ctx, result.RequestID, now().UTC())
}

func (s *Service) RunSuppressionCleanupOnce(ctx context.Context, now func() time.Time, cleaner SyntheticEvidenceCleaner) error {
	repo, ok := s.repo.(SuppressionExecutionRepository)
	if !ok {
		return errors.New("privacy: suppression execution repository unavailable")
	}
	if now == nil {
		now = time.Now
	}
	ids, err := repo.PendingSuppressionRequests(ctx)
	if err != nil {
		return err
	}
	for _, requestID := range ids {
		files, err := repo.PendingSuppressionFiles(ctx, requestID, now().UTC())
		if err != nil {
			return err
		}
		for _, file := range files {
			if cleaner == nil {
				continue
			}
			if err := cleaner.Delete(ctx, file.EvidenceID); err != nil {
				_ = repo.FailSuppressionFile(ctx, file, now().UTC())
				continue
			}
			if err := repo.CompleteSuppressionFile(ctx, file, now().UTC()); err != nil {
				return err
			}
		}
		if _, err := repo.FinishSuppression(ctx, requestID, now().UTC()); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

func (s *Service) RunSuppressionCleanupWorker(ctx context.Context, interval time.Duration, cleaner SyntheticEvidenceCleaner, replayRegistryPath ...string) {
	if interval <= 0 {
		interval = time.Minute
	}
	process := func() {
		_ = s.RunSuppressionCleanupOnce(ctx, time.Now, cleaner)
		if len(replayRegistryPath) > 0 && replayRegistryPath[0] != "" {
			_ = s.ExportSuppressionReplayManifestToFile(ctx, replayRegistryPath[0])
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	process()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}

func (s *Service) RunReservationRetentionWorker(ctx context.Context, interval time.Duration, batchSize int) {
	if interval <= 0 {
		interval = time.Hour
	}
	if batchSize <= 0 || batchSize > 500 {
		batchSize = 100
	}
	process := func() { _, _ = s.PurgeExpiredReservationLinks(ctx, time.Now().UTC(), batchSize) }
	process()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}

// PurgeExpiredReservationLinks performs one bounded local-retention pass. The
// repository rechecks deadlines and obligations while holding the participant
// account locks and reservation row lock.
func (s *Service) PurgeExpiredReservationLinks(ctx context.Context, now time.Time, limit int) (RetentionPurgeResult, error) {
	repo, ok := s.repo.(LocalPrivacyOperationsRepository)
	if !ok || limit < 1 || limit > 500 {
		return RetentionPurgeResult{}, ErrInvalid
	}
	return repo.PurgeExpiredReservationLinks(ctx, now.UTC(), limit)
}

func (s *Service) ExportSuppressionReplayManifest(ctx context.Context) (SuppressionReplayManifest, error) {
	repo, ok := s.repo.(LocalPrivacyOperationsRepository)
	if !ok {
		return SuppressionReplayManifest{}, errors.New("privacy: replay export unavailable")
	}
	return repo.ExportCompletedSuppressions(ctx)
}

// ExportSuppressionReplayManifestWithFile merges database executions with the
// existing external registry. This is essential after restoring an older
// database: its export is a subset of the still-authoritative sidecar.
func (s *Service) ExportSuppressionReplayManifestWithFile(ctx context.Context, path string) (SuppressionReplayManifest, error) {
	manifest, err := s.ExportSuppressionReplayManifest(ctx)
	if err != nil || path == "" {
		return manifest, err
	}
	external, err := readSuppressionReplayManifest(path)
	if errors.Is(err, os.ErrNotExist) {
		return manifest, nil
	}
	if err != nil {
		return SuppressionReplayManifest{}, err
	}
	return mergeSuppressionReplayManifests(manifest, external)
}

func readSuppressionReplayManifest(path string) (SuppressionReplayManifest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return SuppressionReplayManifest{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16<<20 {
		return SuppressionReplayManifest{}, ErrInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return SuppressionReplayManifest{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 16<<20))
	decoder.DisallowUnknownFields()
	var manifest SuppressionReplayManifest
	if err := decoder.Decode(&manifest); err != nil {
		return SuppressionReplayManifest{}, ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF || manifest.Version != 1 {
		return SuppressionReplayManifest{}, ErrInvalid
	}
	for _, entry := range manifest.Entries {
		if entry.ExecutionID == "" || entry.RequestID == "" || entry.AccountID == "" || entry.RequestedAt.IsZero() || entry.CompletedAt.IsZero() {
			return SuppressionReplayManifest{}, ErrInvalid
		}
	}
	return manifest, nil
}

func mergeSuppressionReplayManifests(manifests ...SuppressionReplayManifest) (SuppressionReplayManifest, error) {
	merged := SuppressionReplayManifest{Version: 1, Entries: []SuppressionReplayEntry{}}
	byID := make(map[string]SuppressionReplayEntry)
	for _, manifest := range manifests {
		if manifest.Version != 1 {
			return SuppressionReplayManifest{}, ErrInvalid
		}
		for _, entry := range manifest.Entries {
			if entry.ExecutionID == "" || entry.RequestID == "" || entry.AccountID == "" || entry.RequestedAt.IsZero() || entry.CompletedAt.IsZero() {
				return SuppressionReplayManifest{}, ErrInvalid
			}
			if prior, exists := byID[entry.ExecutionID]; exists {
				if prior.RequestID != entry.RequestID || prior.AccountID != entry.AccountID || !prior.RequestedAt.Equal(entry.RequestedAt) || !prior.CompletedAt.Equal(entry.CompletedAt) {
					return SuppressionReplayManifest{}, ErrInvalid
				}
				continue
			}
			byID[entry.ExecutionID] = entry
		}
	}
	for _, entry := range byID {
		merged.Entries = append(merged.Entries, entry)
	}
	sort.Slice(merged.Entries, func(i, j int) bool {
		if merged.Entries[i].CompletedAt.Equal(merged.Entries[j].CompletedAt) {
			return merged.Entries[i].ExecutionID < merged.Entries[j].ExecutionID
		}
		return merged.Entries[i].CompletedAt.Before(merged.Entries[j].CompletedAt)
	})
	if len(merged.Entries) > 10000 {
		return SuppressionReplayManifest{}, ErrInvalid
	}
	return merged, nil
}

// ExportSuppressionReplayManifestToFile refreshes a mode-0600 sidecar outside
// PostgreSQL/pgdata. The atomic replacement lets a local database restore keep
// a registry of completed synthetic suppressions to reapply.
func (s *Service) ExportSuppressionReplayManifestToFile(ctx context.Context, path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return ErrInvalid
	}
	suppressionRegistryWriteMu.Lock()
	defer suppressionRegistryWriteMu.Unlock()
	manifest, err := s.ExportSuppressionReplayManifestWithFile(ctx, path)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrInvalid
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".privacy-replay-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// ReplaySuppressionManifest reapplies only source executions that had already
// completed. A restored snapshot may contain older obligations; the normal
// execution path still rechecks them before changing the restored database.
func (s *Service) ReplaySuppressionManifest(ctx context.Context, manifest SuppressionReplayManifest, restoreID, actorID string, now time.Time, cleaner SyntheticEvidenceCleaner) ([]SuppressionReplayResult, error) {
	repo, ok := s.repo.(LocalPrivacyOperationsRepository)
	if !ok || manifest.Version != 1 || restoreID == "" || actorID == "" || len(manifest.Entries) > 10000 {
		return nil, ErrInvalid
	}
	results := make([]SuppressionReplayResult, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if entry.ExecutionID == "" || entry.RequestID == "" || entry.AccountID == "" || entry.RequestedAt.IsZero() || entry.CompletedAt.IsZero() {
			return results, ErrInvalid
		}
		requestID, key, status, reused, err := repo.PrepareSuppressionReplay(ctx, entry, restoreID, actorID, now.UTC())
		if err != nil {
			return results, err
		}
		if status == "reaplicada" || status == "ya_presente" {
			results = append(results, SuppressionReplayResult{ExecutionID: entry.ExecutionID, AccountID: entry.AccountID, Status: status, Reused: true})
			continue
		}
		outcome, err := s.ExecuteSuppression(ctx, actorID, requestID, key, "restore:"+restoreID, func() time.Time { return entry.CompletedAt.UTC() }, cleaner)
		if err != nil {
			return results, err
		}
		resultStatus := "pendiente"
		if outcome.Status == "completada" {
			resultStatus = "reaplicada"
		}
		if err := repo.FinishSuppressionReplay(ctx, entry, restoreID, key, resultStatus, now.UTC()); err != nil {
			return results, err
		}
		results = append(results, SuppressionReplayResult{ExecutionID: entry.ExecutionID, AccountID: entry.AccountID, Status: resultStatus, Reused: reused || outcome.Reused})
	}
	return results, nil
}
