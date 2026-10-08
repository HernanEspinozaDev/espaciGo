package privacy

import (
	"context"
	"errors"
	"regexp"
	"strings"
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

func (s *Service) RunSuppressionCleanupWorker(ctx context.Context, interval time.Duration, cleaner SyntheticEvidenceCleaner) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	_ = s.RunSuppressionCleanupOnce(ctx, time.Now, cleaner)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.RunSuppressionCleanupOnce(ctx, time.Now, cleaner)
		}
	}
}
