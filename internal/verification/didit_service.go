package verification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DiditProviderName      = "didit"
	DecisionSchemaVersion  = "1"
	DecisionAlgorithmV1    = "AES-256-GCM/v1"
	DefaultDiditAttemptTTL = 30 * time.Minute
)

// ProviderAttempt represents a durable row in public.identity_provider_attempt (V85).
type ProviderAttempt struct {
	ID                 string
	AccountID          string
	DeclarationID      string
	Provider           string
	Environment        string
	ApplicationID      string
	WorkflowID         string
	WorkflowVersion    string
	IdempotencyKey     string
	CorrelationID      string
	ProviderSessionID  string
	Status             string
	DecisionKeyVersion string
	DecisionAlgorithm  string
	EncryptedDecision  []byte
	CreatedAt          time.Time
	UpdatedAt          time.Time
	RetentionAt        time.Time
}

// ProviderEvent represents a durable row in public.identity_provider_event (V85).
type ProviderEvent struct {
	ID               string
	AttemptID        string
	Provider         string
	Environment      string
	ApplicationID    string
	EventID          string
	EventType        string
	BodySHA256       []byte
	ProcessingStatus string
	ReceivedAt       time.Time
	ProcessedAt      *time.Time
	RetentionAt      time.Time
}

// PrepareAttemptInput holds the parameters to prepare or resume a provider attempt.
type PrepareAttemptInput struct {
	ID              string
	AccountID       string
	DeclarationID   string
	Provider        string
	Environment     string
	ApplicationID   string
	WorkflowID      string
	WorkflowVersion string
	IdempotencyKey  string
	CorrelationID   string
	Now             time.Time
	AttemptTTL      time.Duration
	RetentionAt     time.Time
}

// WebhookEventRecordInput holds the parameters to persist a received webhook event.
type WebhookEventRecordInput struct {
	ID            string
	AttemptID     string
	Provider      string
	Environment   string
	ApplicationID string
	EventID       string
	EventType     string
	BodySHA256    []byte
	ReceivedAt    time.Time
	RetentionAt   time.Time
}

// CommitWebhookDecisionInput groups the durable effects of an authoritative
// decision into one transaction. The provider call runs only after commit.
type CommitWebhookDecisionInput struct {
	AttemptID          string
	EventRowID         string
	NextStatus         string
	DecisionKeyVersion string
	DecisionAlgorithm  string
	EncryptedDecision  []byte
	ProcessedAt        time.Time
	RutFingerprint     string
	DeleteSessionID    string
	PurgeDeclaration   bool
}

// ProviderAttemptRepository defines durable persistence for V85 identity_provider_attempt and identity_provider_event.
type ProviderAttemptRepository interface {
	PrepareAttempt(ctx context.Context, in PrepareAttemptInput) (attempt ProviderAttempt, reused bool, err error)
	MarkAttemptSessionReady(ctx context.Context, attemptID, providerSessionID string, updatedAt time.Time) (ProviderAttempt, error)
	MarkAttemptFailed(ctx context.Context, attemptID string, updatedAt time.Time) error
	GetAttemptForAccount(ctx context.Context, accountID, attemptID string, now time.Time, attemptTTL time.Duration) (ProviderAttempt, error)
	FindAttemptForWebhook(ctx context.Context, provider, environment, applicationID, correlationID, providerSessionID string) (ProviderAttempt, error)
	InsertOrGetWebhookEvent(ctx context.Context, in WebhookEventRecordInput) (event ProviderEvent, duplicate bool, err error)
	SaveAttemptDecisionAndCompleteEvent(ctx context.Context, attemptID, eventRowID, nextStatus, decisionKeyVersion, decisionAlgorithm string, encryptedDecision []byte, processedAt time.Time) (ProviderAttempt, error)
	CommitWebhookDecision(ctx context.Context, in CommitWebhookDecisionInput) (ProviderAttempt, error)
	UpdateAttemptStatusAndCompleteEvent(ctx context.Context, attemptID, eventRowID, nextStatus, eventProcessingStatus string, processedAt time.Time) (ProviderAttempt, error)
	CompleteWebhookEvent(ctx context.Context, eventRowID, processingStatus string, processedAt time.Time) error
}

// DiditSessionEraser is an optional capability on DiditClient to request immediate remote session/media deletion.
type DiditSessionEraser interface {
	DeleteSession(ctx context.Context, sessionID string) error
}

// RutClaimStore enforces 1 canonical RUT per account via HMAC-SHA256 fingerprint without storing plaintext RUT.
type RutClaimStore interface {
	Claim(ctx context.Context, accountID, declarationID, rutFingerprint string, expiresAt time.Time, now time.Time) error
	MarkVerified(ctx context.Context, accountID, declarationID, rutFingerprint string, now time.Time) error
	RevokeVerified(ctx context.Context, accountID string, now time.Time)
}

// ComputeRutFingerprint computes a versioned HMAC-SHA256 digest over a canonicalized Chilean RUT.
// Plain SHA-256 is forbidden because the Chilean RUT space is enumerable.
func ComputeRutFingerprint(rut string, secret []byte, keyVersion string) (string, error) {
	normalized, err := NormalizeRut(rut)
	if err != nil || !ValidateRut(normalized) {
		return "", ErrInvalid
	}
	trimmedVer := strings.TrimSpace(keyVersion)
	if len(secret) < 16 || trimmedVer == "" || strings.Contains(trimmedVer, ":") {
		return "", ErrInvalid
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("espacigo:rut-hmac:v1:"))
	_, _ = mac.Write([]byte(normalized))
	return trimmedVer + ":" + hex.EncodeToString(mac.Sum(nil)), nil
}

type rutClaimEntry struct {
	AccountID      string
	DeclarationID  string
	RutFingerprint string
	DigestHex      string
	ExpiresAt      time.Time
	Verified       bool
}

// MemoryRutClaimRegistry is a thread-safe in-memory RutClaimStore for local/unit enforcement.
type MemoryRutClaimRegistry struct {
	mu       sync.Mutex
	byDigest map[string]rutClaimEntry
}

func NewMemoryRutClaimRegistry() *MemoryRutClaimRegistry {
	return &MemoryRutClaimRegistry{
		byDigest: make(map[string]rutClaimEntry),
	}
}

func extractDigestHex(fingerprint string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(fingerprint), ":", 2)
	if len(parts) != 2 || len(parts[1]) != 64 {
		return "", ErrInvalid
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", ErrInvalid
	}
	return strings.ToLower(parts[1]), nil
}

func (r *MemoryRutClaimRegistry) Claim(ctx context.Context, accountID, declarationID, rutFingerprint string, expiresAt time.Time, now time.Time) error {
	_, err := r.claimMemory(ctx, accountID, declarationID, RutFingerprintSet{Active: rutFingerprint, Candidates: []string{rutFingerprint}}, expiresAt, now, false)
	return err
}
func (r *MemoryRutClaimRegistry) MarkVerified(_ context.Context, accountID, declarationID, rutFingerprint string, now time.Time) error {
	if r == nil {
		return ErrInvalid
	}
	digest, err := extractDigestHex(rutFingerprint)
	if err != nil || !rutFingerprintPattern.MatchString(rutFingerprint) || !canonicalDeclarationUUID(accountID) || !canonicalDeclarationUUID(declarationID) || now.IsZero() {
		return ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.byDigest[rutFingerprint]
	if !ok || old.AccountID != accountID || old.DeclarationID != declarationID || (!old.Verified && !old.ExpiresAt.After(now)) {
		return ErrConflict
	}
	for k, v := range r.byDigest {
		if v.AccountID == accountID && k != rutFingerprint && !v.Verified {
			delete(r.byDigest, k)
		}
	}
	r.byDigest[rutFingerprint] = rutClaimEntry{AccountID: accountID, RutFingerprint: rutFingerprint, DigestHex: digest, Verified: true}
	return nil
}
func (r *MemoryRutClaimRegistry) RevokeVerified(_ context.Context, accountID string, now time.Time) {
	// Effective approval lives on the attempt, not on this anti-reuse marker.
	// Revocation must not erase or downgrade a historical permanent reservation.
}

// BuildDecisionAAD constructs authenticated additional data for AEAD-encrypted decision records.
func BuildDecisionAAD(accountID, attemptID, provider, environment, schemaVersion string) ([]byte, error) {
	account, err := uuid.Parse(accountID)
	if err != nil || account.String() != accountID {
		return nil, ErrInvalid
	}
	attempt, err := uuid.Parse(attemptID)
	if err != nil || attempt.String() != attemptID {
		return nil, ErrInvalid
	}
	provider = strings.TrimSpace(provider)
	environment = strings.TrimSpace(environment)
	if provider == "" || (environment != "SANDBOX" && environment != "LIVE") || schemaVersion != DecisionSchemaVersion {
		return nil, ErrInvalid
	}
	const purpose = "espacigo:identity-decision:v1"
	var out bytes.Buffer
	writeAADField(&out, []byte(purpose))
	writeAADField(&out, account[:])
	writeAADField(&out, attempt[:])
	writeAADField(&out, []byte(provider))
	writeAADField(&out, []byte(environment))
	writeAADField(&out, []byte(schemaVersion))
	return out.Bytes(), nil
}

// EncryptedDecisionSnapshot is the minimal, media-free payload encrypted in identity_provider_attempt.encrypted_decision.
type EncryptedDecisionSnapshot struct {
	ProviderStatus  string                 `json:"provider_status"`
	EvaluatedStatus string                 `json:"evaluated_status"`
	LivenessPassed  bool                   `json:"liveness_passed"`
	FaceMatchPassed bool                   `json:"face_match_passed"`
	Flags           map[string]interface{} `json:"flags,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
	EvaluatedAt     time.Time              `json:"evaluated_at"`
}

// DiditServiceConfig holds configuration for DiditService.
type DiditServiceConfig struct {
	Environment                 string
	CallbackURL                 string
	AllowedCallbackOrigin       string
	ApplicationID               string
	WorkflowID                  string
	WorkflowVersion             string
	ConsentVersion              string
	DeclarationConsentVersion   string
	AttemptTTL                  time.Duration
	Retention                   DeclarationRetentionPolicy
	RequireLiveForVerifiedBadge bool
	RutHMACSecret               []byte
	RutHMACKeyVersion           string
	RutHMACKeyring              *RutHMACKeyring
}

// StartDiditIntentInput represents the client request to start or resume a Didit verification attempt.
type StartDiditIntentInput struct {
	FirstName      string
	LastName       string
	DateOfBirth    string
	RUT            string
	IDCountry      string
	DocumentType   string
	ConsentGiven   bool
	ConsentVersion string
}

// StartedDiditIntent is the response returned when starting or resuming an attempt.
type StartedDiditIntent struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	RedirectURL string    `json:"redirect_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// DiditAttemptStatusView is the response returned for GET /verifications/didit/intent/{id}.
type DiditAttemptStatusView struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	BadgeResult string    `json:"badge_result"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DiditWebhookInput represents a verified webhook delivery to be processed.
type DiditWebhookInput struct {
	EventID           string
	EventType         string
	CorrelationID     string
	ProviderSessionID string
	TopLevelStatus    string
	BodySHA256        []byte
	Decision          *DiditDecisionResponse
}

// DiditWebhookOutcome summarizes the result of processing a webhook.
type DiditWebhookOutcome struct {
	AttemptID        string
	AttemptStatus    string
	ProcessingStatus string
	Duplicate        bool
	RemoteDeleted    bool
}

type ephemeralSessionURL struct {
	URL       string
	ExpiresAt time.Time
}

// SessionDeletionQueue persists session IDs queued for remote deletion retry.
type SessionDeletionQueue interface {
	EnqueueSessionDeletion(ctx context.Context, sessionID string) error
	PendingSessionDeletions(ctx context.Context) ([]string, error)
	AcknowledgeSessionDeletion(ctx context.Context, sessionID string) error
}

type memorySessionDeletionQueue struct {
	mu       sync.Mutex
	sessions []string
}

// NewMemorySessionDeletionQueue returns an in-memory deletion queue for test or local environments.
func NewMemorySessionDeletionQueue() SessionDeletionQueue {
	return &memorySessionDeletionQueue{}
}

func (q *memorySessionDeletionQueue) EnqueueSessionDeletion(_ context.Context, sessionID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, s := range q.sessions {
		if s == sessionID {
			return nil
		}
	}
	q.sessions = append(q.sessions, sessionID)
	return nil
}

func (q *memorySessionDeletionQueue) PendingSessionDeletions(_ context.Context) ([]string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.sessions...), nil
}

func (q *memorySessionDeletionQueue) AcknowledgeSessionDeletion(_ context.Context, sessionID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	var remaining []string
	for _, s := range q.sessions {
		if s != sessionID {
			remaining = append(remaining, s)
		}
	}
	q.sessions = remaining
	return nil
}

// DiditService coordinates durable V85 attempts, Didit V3 session creation, decision evaluation, and media deletion.
type DiditService struct {
	repo         ProviderAttemptRepository
	declarations *DeclarationService
	client       DiditClient
	keys         DeclarationKeyring
	ids          IDGenerator
	now          func() time.Time
	cfg          DiditServiceConfig
	rutClaims    RutClaimStore

	urlMu            sync.Mutex
	ephemeralURLs    map[string]ephemeralSessionURL
	deleteOutboxMu   sync.Mutex
	pendingDeletions []string
	deletionQueue    SessionDeletionQueue
}

// ValidateDiditCallbackConfig applies the same server-origin boundary before
// runtime dependencies are loaded and when the service is constructed.
func ValidateDiditCallbackConfig(cfg DiditServiceConfig) error {
	if cfg.CallbackURL == "" {
		return nil
	}
	u, err := url.Parse(cfg.CallbackURL)
	if err != nil || u == nil || u.Scheme != "https" || u.User != nil || u.Path != "/verificaciones/retorno" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.Opaque != "" || strings.ContainsAny(cfg.CallbackURL, " \t\r\n") {
		return ErrInvalid
	}
	origin := strings.TrimSpace(cfg.AllowedCallbackOrigin)
	if origin == "" {
		if u.Host != "127.0.0.1:4219" {
			return ErrInvalid
		}
		return nil
	}
	o, err := url.Parse(origin)
	if err != nil || o == nil || o.Scheme != "https" || o.Hostname() == "" || o.User != nil || o.Path != "" || o.RawPath != "" || o.RawQuery != "" || o.ForceQuery || o.Fragment != "" || o.RawFragment != "" || o.Opaque != "" || o.String() != origin || strings.ContainsAny(origin, "* 	\r\n") || strings.HasSuffix(o.Hostname(), ".") {
		return ErrInvalid
	}
	if strings.HasSuffix(o.Host, ":") {
		return ErrInvalid
	}
	if port := o.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return ErrInvalid
		}
	}
	if o.Host != u.Host || cfg.CallbackURL != origin+"/verificaciones/retorno" {
		return ErrInvalid
	}
	return nil
}

func NewDiditService(
	repo ProviderAttemptRepository,
	declarations *DeclarationService,
	client DiditClient,
	keys DeclarationKeyring,
	ids IDGenerator,
	now func() time.Time,
	cfg DiditServiceConfig,
	rutClaims RutClaimStore,
	deletionQueues ...SessionDeletionQueue,
) (*DiditService, error) {
	if repo == nil || declarations == nil || client == nil || keys == nil || ids == nil || now == nil {
		return nil, ErrInvalid
	}
	if cfg.RutHMACKeyring != nil {
		if len(cfg.RutHMACSecret) > 0 || strings.TrimSpace(cfg.RutHMACKeyVersion) != "" {
			return nil, ErrInvalid
		}
		if cfg.RutHMACKeyring == nil || cfg.RutHMACKeyring.active == "" || len(cfg.RutHMACKeyring.keys) == 0 || len(cfg.RutHMACKeyring.versions) != len(cfg.RutHMACKeyring.keys) {
			return nil, ErrInvalid
		}
		if rutClaims == nil {
			return nil, ErrInvalid
		}
		if _, ok := rutClaims.(RotatingRutClaimStore); !ok {
			return nil, ErrInvalid
		}
	}
	if err := ValidateDiditCallbackConfig(cfg); err != nil {
		return nil, err
	}
	env := strings.ToUpper(strings.TrimSpace(cfg.Environment))
	if env == "" {
		env = "SANDBOX"
	}
	if env != "SANDBOX" && env != "LIVE" {
		return nil, ErrInvalid
	}
	cfg.Environment = env
	cfg.ApplicationID = strings.TrimSpace(cfg.ApplicationID)
	if cfg.ApplicationID == "" {
		cfg.ApplicationID = "espacigo-local-app"
	}
	cfg.WorkflowID = strings.TrimSpace(cfg.WorkflowID)
	if cfg.WorkflowID == "" {
		return nil, ErrInvalid
	}
	cfg.WorkflowVersion = strings.TrimSpace(cfg.WorkflowVersion)
	if cfg.WorkflowVersion == "" {
		cfg.WorkflowVersion = "1"
	}
	cfg.ConsentVersion = strings.TrimSpace(cfg.ConsentVersion)
	if !validDeclarationConsentVersion(cfg.ConsentVersion) {
		return nil, ErrInvalid
	}
	cfg.DeclarationConsentVersion = strings.TrimSpace(cfg.DeclarationConsentVersion)
	if cfg.DeclarationConsentVersion == "" {
		cfg.DeclarationConsentVersion = cfg.ConsentVersion
	}
	if !validDeclarationConsentVersion(cfg.DeclarationConsentVersion) {
		return nil, ErrInvalid
	}
	if cfg.AttemptTTL <= 0 {
		cfg.AttemptTTL = DefaultDiditAttemptTTL
	}
	if cfg.Retention == nil {
		cfg.Retention = func(consentAt time.Time) time.Time {
			return consentAt.UTC().Add(30 * 24 * time.Hour)
		}
	}
	if cfg.RutHMACKeyVersion == "" {
		cfg.RutHMACKeyVersion = "v1"
	}
	var dq SessionDeletionQueue
	if len(deletionQueues) > 0 && deletionQueues[0] != nil {
		dq = deletionQueues[0]
	}
	svc := &DiditService{
		repo:          repo,
		declarations:  declarations,
		client:        client,
		keys:          keys,
		ids:           ids,
		now:           now,
		cfg:           cfg,
		rutClaims:     rutClaims,
		ephemeralURLs: make(map[string]ephemeralSessionURL),
		deletionQueue: dq,
	}
	if dq != nil {
		pending, err := dq.PendingSessionDeletions(context.Background())
		if err != nil {
			return nil, err
		}
		svc.pendingDeletions = append([]string(nil), pending...)
	}
	return svc, nil
}

func ValidDiditRedirectURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "verify.didit.me" || host == "verification.didit.me"
}

func deriveAttemptIdempotencyUUID(accountID, declarationID, rawKey string, ids IDGenerator) (string, error) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed == "" {
		id, err := ids.ID()
		if err != nil || !canonicalDeclarationUUID(id) {
			return "", ErrInvalid
		}
		return id, nil
	}
	if len(trimmed) > 128 {
		return "", ErrInvalid
	}
	derived := uuid.NewSHA1(uuid.NameSpaceOID, []byte(accountID+":"+declarationID+":"+trimmed)).String()
	return derived, nil
}

// StartOrResume creates or resumes a durable V85 verification attempt using ONLY the server-saved declaration snapshot.
func (s *DiditService) StartOrResume(ctx context.Context, accountID, idempotencyKey string, req StartDiditIntentInput) (StartedDiditIntent, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) {
		return StartedDiditIntent{}, ErrInvalid
	}
	reqConsent := strings.TrimSpace(req.ConsentVersion)
	if !req.ConsentGiven || (reqConsent != s.cfg.ConsentVersion && reqConsent != s.cfg.DeclarationConsentVersion) {
		return StartedDiditIntent{}, ErrInvalid
	}
	if req.IDCountry != "" && req.IDCountry != "CHL" {
		return StartedDiditIntent{}, ErrInvalid
	}
	if req.DocumentType != "" && req.DocumentType != "ID" {
		return StartedDiditIntent{}, ErrInvalid
	}

	decl, err := s.declarations.GetCurrent(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return StartedDiditIntent{}, ErrConflict
		}
		return StartedDiditIntent{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if err := ValidateAdultBirthDate(decl.Payload.DateOfBirth, now); err != nil {
		return StartedDiditIntent{}, ErrInvalid
	}

	// Enforce that any client-supplied fields match the saved server declaration snapshot.
	if strings.TrimSpace(req.FirstName) != "" && strings.TrimSpace(req.FirstName) != strings.TrimSpace(decl.Payload.FirstName) {
		return StartedDiditIntent{}, ErrConflict
	}
	if strings.TrimSpace(req.LastName) != "" && strings.TrimSpace(req.LastName) != strings.TrimSpace(decl.Payload.LastName) {
		return StartedDiditIntent{}, ErrConflict
	}
	if strings.TrimSpace(req.DateOfBirth) != "" && strings.TrimSpace(req.DateOfBirth) != decl.Payload.DateOfBirth {
		return StartedDiditIntent{}, ErrConflict
	}
	if strings.TrimSpace(req.RUT) != "" {
		reqRut, err := NormalizeRut(req.RUT)
		if err != nil || !ValidateRut(reqRut) {
			return StartedDiditIntent{}, ErrInvalid
		}
		if reqRut != decl.Payload.RUT {
			return StartedDiditIntent{}, ErrConflict
		}
	}

	retentionAt := s.cfg.Retention(decl.ConsentGivenAt).UTC().Truncate(time.Microsecond)
	if !retentionAt.After(now) {
		return StartedDiditIntent{}, ErrConflict
	}

	if s.cfg.RutHMACKeyring != nil {
		set, err := s.cfg.RutHMACKeyring.Fingerprints(decl.Payload.RUT)
		if err != nil {
			return StartedDiditIntent{}, err
		}
		claims := s.rutClaims.(RotatingRutClaimStore)
		if _, err := claims.ClaimCandidates(ctx, accountID, decl.ID, set, retentionAt, now); err != nil {
			return StartedDiditIntent{}, err
		}
	} else if s.rutClaims != nil && len(s.cfg.RutHMACSecret) >= 16 {
		fp, err := ComputeRutFingerprint(decl.Payload.RUT, s.cfg.RutHMACSecret, s.cfg.RutHMACKeyVersion)
		if err != nil {
			return StartedDiditIntent{}, err
		}
		if err := s.rutClaims.Claim(ctx, accountID, decl.ID, fp, retentionAt, now); err != nil {
			return StartedDiditIntent{}, err
		}
	}

	idempUUID, err := deriveAttemptIdempotencyUUID(accountID, decl.ID, idempotencyKey, s.ids)
	if err != nil {
		return StartedDiditIntent{}, err
	}
	attemptID, err := s.ids.ID()
	if err != nil || !canonicalDeclarationUUID(attemptID) {
		return StartedDiditIntent{}, ErrInvalid
	}
	correlationID, err := s.ids.ID()
	if err != nil || !canonicalDeclarationUUID(correlationID) {
		return StartedDiditIntent{}, ErrInvalid
	}

	attempt, reused, err := s.repo.PrepareAttempt(ctx, PrepareAttemptInput{
		ID:              attemptID,
		AccountID:       accountID,
		DeclarationID:   decl.ID,
		Provider:        DiditProviderName,
		Environment:     s.cfg.Environment,
		ApplicationID:   s.cfg.ApplicationID,
		WorkflowID:      s.cfg.WorkflowID,
		WorkflowVersion: s.cfg.WorkflowVersion,
		IdempotencyKey:  idempUUID,
		CorrelationID:   correlationID,
		Now:             now,
		AttemptTTL:      s.cfg.AttemptTTL,
		RetentionAt:     retentionAt,
	})
	if err != nil {
		return StartedDiditIntent{}, err
	}

	if reused && attempt.ProviderSessionID != "" {
		cachedURL := s.getEphemeralURL(attempt.ID, now)
		return StartedDiditIntent{
			ID:          attempt.ID,
			Status:      attempt.Status,
			RedirectURL: cachedURL,
			CreatedAt:   attempt.CreatedAt.UTC(),
		}, nil
	}

	callback := ""
	if s.cfg.CallbackURL != "" {
		u, _ := url.Parse(s.cfg.CallbackURL)
		q := u.Query()
		q.Set("attempt_id", attempt.ID)
		u.RawQuery = q.Encode()
		callback = u.String()
	}
	resp, err := s.client.CreateSession(ctx, CreateSessionRequest{
		WorkflowID: s.cfg.WorkflowID,
		Callback:   callback,
		VendorData: attempt.CorrelationID,
		ExpectedDetails: ExpectedDetails{
			FirstName:             decl.Payload.FirstName,
			LastName:              decl.Payload.LastName,
			DateOfBirth:           decl.Payload.DateOfBirth,
			IDCountry:             "CHL",
			DocumentType:          "ID",
			IdentificationNumber:  decl.Payload.RUT,
			ExpectedDocumentTypes: []string{"ID"},
		},
	})
	if err != nil {
		_ = s.repo.MarkAttemptFailed(ctx, attempt.ID, s.now().UTC().Truncate(time.Microsecond))
		return StartedDiditIntent{}, err
	}
	if strings.TrimSpace(resp.SessionID) == "" || !ValidDiditRedirectURL(resp.SessionURL) {
		_ = s.repo.MarkAttemptFailed(ctx, attempt.ID, s.now().UTC().Truncate(time.Microsecond))
		return StartedDiditIntent{}, ErrDiditUnavailable
	}

	readyAt := s.now().UTC().Truncate(time.Microsecond)
	if readyAt.Before(attempt.CreatedAt) {
		readyAt = attempt.CreatedAt
	}
	updated, err := s.repo.MarkAttemptSessionReady(ctx, attempt.ID, strings.TrimSpace(resp.SessionID), readyAt)
	if err != nil {
		readyErr := err
		// A response may be lost after the write committed. Read back first:
		// never delete a session merely because its ready response was uncertain.
		// A canceled browser request must not cancel this bounded reconciliation.
		reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		observed, readErr := s.repo.GetAttemptForAccount(reconcileCtx, accountID, attempt.ID, readyAt, s.cfg.AttemptTTL)
		if readErr != nil && !errors.Is(readErr, ErrNotFound) {
			return StartedDiditIntent{}, errors.Join(readyErr, readErr)
		}
		if readErr == nil && (observed.ID != attempt.ID || observed.AccountID != accountID) {
			return StartedDiditIntent{}, errors.Join(readyErr, ErrConflict)
		}
		active := observed.Status == "CREATED" || observed.Status == "PENDING_USER_INPUT" || observed.Status == "PROCESSING" || observed.Status == "IN_REVIEW"
		if readErr == nil && active && observed.DeclarationID == attempt.DeclarationID && observed.ProviderSessionID == strings.TrimSpace(resp.SessionID) {
			updated = observed
		} else {
			// The attempt disappeared or became terminal while CreateSession was
			// in flight. Keep its newly known session in the deletion outbox;
			// neither this enqueue nor an enqueue error is a remote deletion ACK.
			if s.deletionQueue == nil {
				// Legacy/unit composition may lack a durable queue. Retain the ID
				// only in memory and report unavailability, never durable success.
				s.recordPendingDeletion(strings.TrimSpace(resp.SessionID))
				return StartedDiditIntent{}, errors.Join(readyErr, ErrDiditUnavailable)
			}
			if queueErr := s.deletionQueue.EnqueueSessionDeletion(reconcileCtx, strings.TrimSpace(resp.SessionID)); queueErr != nil {
				return StartedDiditIntent{}, errors.Join(readyErr, queueErr)
			}
			s.recordPendingDeletion(strings.TrimSpace(resp.SessionID))
			return StartedDiditIntent{}, readyErr
		}
	}
	s.setEphemeralURL(updated.ID, resp.SessionURL, readyAt.Add(s.cfg.AttemptTTL))
	return StartedDiditIntent{
		ID:          updated.ID,
		Status:      updated.Status,
		RedirectURL: resp.SessionURL,
		CreatedAt:   updated.CreatedAt.UTC(),
	}, nil
}

// GetStatus returns the minimal status view for the authenticated owner of the attempt.
func (s *DiditService) GetStatus(ctx context.Context, accountID, attemptID string) (DiditAttemptStatusView, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) || !canonicalDeclarationUUID(attemptID) {
		return DiditAttemptStatusView{}, ErrInvalid
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	attempt, err := s.repo.GetAttemptForAccount(ctx, accountID, attemptID, now, s.cfg.AttemptTTL)
	if err != nil {
		return DiditAttemptStatusView{}, err
	}
	if attempt.Status != "PENDING_USER_INPUT" {
		s.clearEphemeralURL(attempt.ID)
	}
	superseded := true
	if attempt.DeclarationID == "" {
		_, err := s.declarations.GetCurrent(ctx, accountID)
		superseded = err == nil
	} else if currentDecl, err := s.declarations.GetCurrent(ctx, accountID); err == nil && currentDecl.ID == attempt.DeclarationID {
		superseded = false
	}
	badge := s.computeBadgeResult(attempt, superseded)
	return DiditAttemptStatusView{
		ID:          attempt.ID,
		Status:      attempt.Status,
		BadgeResult: badge,
		CreatedAt:   attempt.CreatedAt.UTC(),
		UpdatedAt:   attempt.UpdatedAt.UTC(),
	}, nil
}

func (s *DiditService) computeBadgeResult(attempt ProviderAttempt, superseded bool) string {
	if superseded {
		return "NOT_VERIFIED"
	}
	switch attempt.Status {
	case "APPROVED":
		if s.cfg.RequireLiveForVerifiedBadge && attempt.Environment != "LIVE" {
			return "IN_REVIEW"
		}
		return "VERIFIED"
	case "IN_REVIEW":
		return "IN_REVIEW"
	case "REJECTED":
		return "REJECTED"
	default:
		return "NOT_VERIFIED"
	}
}

// ProcessWebhook records the minimal V85 webhook event, deduplicates replays, evaluates the decision against
// the exact declaration revision bound to the attempt, seals the minimal decision snapshot, and triggers remote deletion.
func (s *DiditService) ProcessWebhook(ctx context.Context, in DiditWebhookInput) (DiditWebhookOutcome, error) {
	if s == nil {
		return DiditWebhookOutcome{}, ErrInvalid
	}
	eventID := strings.TrimSpace(in.EventID)
	eventType := strings.TrimSpace(in.EventType)
	if eventID == "" || len(eventID) > 160 || len(in.BodySHA256) != 32 {
		return DiditWebhookOutcome{}, ErrInvalid
	}
	switch eventType {
	case "status.updated", "data.updated", "user.status.updated":
	default:
		return DiditWebhookOutcome{}, ErrInvalid
	}

	correlationID := strings.TrimSpace(in.CorrelationID)
	sessionID := strings.TrimSpace(in.ProviderSessionID)
	if in.Decision != nil {
		if correlationID == "" {
			correlationID = strings.TrimSpace(in.Decision.VendorData)
		}
		if sessionID == "" {
			sessionID = strings.TrimSpace(in.Decision.SessionID)
		}
	}

	attempt, err := s.repo.FindAttemptForWebhook(ctx, DiditProviderName, s.cfg.Environment, s.cfg.ApplicationID, correlationID, sessionID)
	if err != nil {
		return DiditWebhookOutcome{}, err
	}

	now := s.now().UTC().Truncate(time.Microsecond)
	eventRowID, err := s.ids.ID()
	if err != nil || !canonicalDeclarationUUID(eventRowID) {
		return DiditWebhookOutcome{}, ErrInvalid
	}
	eventRetention := attempt.RetentionAt
	if !eventRetention.After(now) {
		eventRetention = now.Add(24 * time.Hour)
	}

	evt, duplicate, err := s.repo.InsertOrGetWebhookEvent(ctx, WebhookEventRecordInput{
		ID:            eventRowID,
		AttemptID:     attempt.ID,
		Provider:      attempt.Provider,
		Environment:   attempt.Environment,
		ApplicationID: attempt.ApplicationID,
		EventID:       eventID,
		EventType:     eventType,
		BodySHA256:    in.BodySHA256,
		ReceivedAt:    now,
		RetentionAt:   eventRetention,
	})
	if err != nil {
		return DiditWebhookOutcome{}, err
	}
	if duplicate && (evt.ProcessingStatus == "PROCESSED" || evt.ProcessingStatus == "IGNORED") {
		return DiditWebhookOutcome{
			AttemptID:        attempt.ID,
			AttemptStatus:    attempt.Status,
			ProcessingStatus: evt.ProcessingStatus,
			Duplicate:        true,
		}, nil
	}

	processedAt := s.now().UTC().Truncate(time.Microsecond)
	if processedAt.Before(evt.ReceivedAt) {
		processedAt = evt.ReceivedAt
	}

	// Do not allow late webhooks to resurrect EXPIRED/FAILED/REJECTED attempts or superseded declarations.
	superseded := false
	if attempt.DeclarationID == "" && attempt.Status == "APPROVED" {
		// The verified snapshot was intentionally purged after authoritative approval.
	} else if currentDecl, err := s.declarations.GetCurrent(ctx, attempt.AccountID); err != nil || currentDecl.ID != attempt.DeclarationID {
		superseded = true
	}
	if attempt.Status == "EXPIRED" || attempt.Status == "FAILED" || attempt.Status == "REJECTED" || !attempt.RetentionAt.After(now) || superseded {
		if err := s.repo.CompleteWebhookEvent(ctx, evt.ID, "IGNORED", processedAt); err != nil {
			return DiditWebhookOutcome{}, err
		}
		return DiditWebhookOutcome{
			AttemptID:        attempt.ID,
			AttemptStatus:    attempt.Status,
			ProcessingStatus: "IGNORED",
			Duplicate:        duplicate,
		}, nil
	}

	topStatus := strings.ToUpper(strings.TrimSpace(in.TopLevelStatus))
	if in.Decision != nil && topStatus == "" {
		topStatus = strings.ToUpper(strings.TrimSpace(in.Decision.Status))
	}

	// Handle revocation / KYC expiration events that withdraw prior approval.
	if eventType == "user.status.updated" || topStatus == "REVOKED" || topStatus == "KYC EXPIRED" || topStatus == "KYC_EXPIRED" || topStatus == "EXPIRED" || topStatus == "BLOCKED" {
		nextStatus := "EXPIRED"
		if topStatus == "REVOKED" || topStatus == "BLOCKED" {
			nextStatus = "REJECTED"
		}
		updated, err := s.repo.UpdateAttemptStatusAndCompleteEvent(ctx, attempt.ID, evt.ID, nextStatus, "PROCESSED", processedAt)
		if err != nil {
			return DiditWebhookOutcome{}, err
		}
		if s.rutClaims != nil {
			s.rutClaims.RevokeVerified(ctx, attempt.AccountID, processedAt)
		}
		s.clearEphemeralURL(attempt.ID)
		return DiditWebhookOutcome{
			AttemptID:        updated.ID,
			AttemptStatus:    updated.Status,
			ProcessingStatus: "PROCESSED",
			Duplicate:        duplicate,
		}, nil
	}

	// If the attempt is already APPROVED, late intermediate events (e.g. IN_PROGRESS)
	// or duplicate webhooks must not revert or downgrade the approval.
	if attempt.Status == "APPROVED" {
		if err := s.repo.CompleteWebhookEvent(ctx, evt.ID, "PROCESSED", processedAt); err != nil {
			return DiditWebhookOutcome{}, err
		}
		return DiditWebhookOutcome{
			AttemptID:        attempt.ID,
			AttemptStatus:    attempt.Status,
			ProcessingStatus: "PROCESSED",
			Duplicate:        duplicate,
		}, nil
	}

	// If the attempt is already in a terminal state (REJECTED, EXPIRED, FAILED),
	// late events must never reactivate or overwrite the decision.
	if attempt.Status == "REJECTED" || attempt.Status == "EXPIRED" || attempt.Status == "FAILED" {
		if err := s.repo.CompleteWebhookEvent(ctx, evt.ID, "IGNORED", processedAt); err != nil {
			return DiditWebhookOutcome{}, err
		}
		return DiditWebhookOutcome{
			AttemptID:        attempt.ID,
			AttemptStatus:    attempt.Status,
			ProcessingStatus: "IGNORED",
			Duplicate:        duplicate,
		}, nil
	}

	// Handle intermediate non-terminal statuses without decision evaluation.
	switch topStatus {
	case "NOT STARTED", "NOT_STARTED":
		updated, err := s.repo.UpdateAttemptStatusAndCompleteEvent(ctx, attempt.ID, evt.ID, "PENDING_USER_INPUT", "PROCESSED", processedAt)
		if err != nil {
			return DiditWebhookOutcome{}, err
		}
		return DiditWebhookOutcome{AttemptID: updated.ID, AttemptStatus: updated.Status, ProcessingStatus: "PROCESSED", Duplicate: duplicate}, nil
	case "IN PROGRESS", "IN_PROGRESS", "RESUBMITTED":
		updated, err := s.repo.UpdateAttemptStatusAndCompleteEvent(ctx, attempt.ID, evt.ID, "PROCESSING", "PROCESSED", processedAt)
		if err != nil {
			return DiditWebhookOutcome{}, err
		}
		s.clearEphemeralURL(attempt.ID)
		return DiditWebhookOutcome{AttemptID: updated.ID, AttemptStatus: updated.Status, ProcessingStatus: "PROCESSED", Duplicate: duplicate}, nil
	case "ABANDONED":
		updated, err := s.repo.UpdateAttemptStatusAndCompleteEvent(ctx, attempt.ID, evt.ID, "EXPIRED", "PROCESSED", processedAt)
		if err != nil {
			return DiditWebhookOutcome{}, err
		}
		s.clearEphemeralURL(attempt.ID)
		return DiditWebhookOutcome{AttemptID: updated.ID, AttemptStatus: updated.Status, ProcessingStatus: "PROCESSED", Duplicate: duplicate}, nil
	}

	decl, err := s.declarations.GetRevision(ctx, attempt.AccountID, attempt.DeclarationID)
	if err != nil {
		_ = s.repo.CompleteWebhookEvent(ctx, evt.ID, "FAILED", processedAt)
		return DiditWebhookOutcome{}, err
	}

	var decision DiditDecisionResponse
	if attempt.ProviderSessionID == "" {
		_ = s.repo.CompleteWebhookEvent(ctx, evt.ID, "FAILED", processedAt)
		return DiditWebhookOutcome{}, ErrInvalid
	}
	decision, err = s.client.GetSessionDecision(ctx, attempt.ProviderSessionID)
	if err != nil {
		_ = s.repo.CompleteWebhookEvent(ctx, evt.ID, "FAILED", processedAt)
		return DiditWebhookOutcome{}, err
	}

	evalAttempt := &IdentityVerificationAttempt{
		ID:                    attempt.ID,
		AccountID:             attempt.AccountID,
		Status:                attempt.Status,
		IDCountry:             decl.Payload.IDCountry,
		DocumentType:          decl.Payload.DocumentType,
		DeclaredFirstName:     decl.Payload.FirstName,
		DeclaredLastName:      decl.Payload.LastName,
		DeclaredDateOfBirth:   decl.Payload.DateOfBirth,
		DeclaredRUTNormalized: decl.Payload.RUT,
		DiditSessionID:        attempt.ProviderSessionID,
		WorkflowID:            attempt.WorkflowID,
		Environment:           attempt.Environment,
		ConsentVersion:        decl.ConsentVersion,
	}
	if evalAttempt.DiditSessionID == "" {
		evalAttempt.DiditSessionID = sessionID
	}

	evalStatus, flags, warnings := EvaluateDecision(evalAttempt, &decision)
	nextStatus := evalStatus
	if (topStatus == "DECLINED" || topStatus == "REJECTED") && len(flags) == 0 {
		nextStatus = "REJECTED"
	}

	sealDecision := func(status string, decisionFlags map[string]interface{}) (string, []byte, error) {
		snapshot := EncryptedDecisionSnapshot{
			ProviderStatus:  decision.Status,
			EvaluatedStatus: status,
			LivenessPassed:  decision.Liveness || (len(decision.LivenessChecks) > 0 && isDiditApprovedStatus(decision.LivenessChecks[0].Status)),
			FaceMatchPassed: decision.FaceMatch || (len(decision.FaceMatches) > 0 && isDiditApprovedStatus(decision.FaceMatches[0].Status)),
			Flags:           decisionFlags,
			Warnings:        warnings,
			EvaluatedAt:     processedAt,
		}
		plain, err := json.Marshal(snapshot)
		if err != nil {
			return "", nil, err
		}
		aad, err := BuildDecisionAAD(attempt.AccountID, attempt.ID, attempt.Provider, attempt.Environment, DecisionSchemaVersion)
		if err != nil {
			return "", nil, err
		}
		return s.keys.Seal(aad, plain)
	}

	keyVersion, ciphertext, err := sealDecision(nextStatus, flags)
	if err != nil {
		return DiditWebhookOutcome{}, err
	}
	var rutFingerprint string
	if nextStatus == "APPROVED" && s.cfg.RutHMACKeyring != nil {
		set, err := s.cfg.RutHMACKeyring.Fingerprints(decl.Payload.RUT)
		if err != nil {
			_ = s.repo.CompleteWebhookEvent(ctx, evt.ID, "FAILED", processedAt)
			return DiditWebhookOutcome{}, err
		}
		rutFingerprint, err = s.rutClaims.(RotatingRutClaimStore).ResolveCandidates(ctx, attempt.AccountID, decl.ID, set)
		if err != nil {
			_ = s.repo.CompleteWebhookEvent(ctx, evt.ID, "FAILED", processedAt)
			return DiditWebhookOutcome{}, err
		}
		if rutFingerprint == "" {
			_ = s.repo.CompleteWebhookEvent(ctx, evt.ID, "FAILED", processedAt)
			return DiditWebhookOutcome{}, ErrInvalid
		}
	} else if nextStatus == "APPROVED" && s.rutClaims != nil && len(s.cfg.RutHMACSecret) >= 16 {
		rutFingerprint, err = ComputeRutFingerprint(decl.Payload.RUT, s.cfg.RutHMACSecret, s.cfg.RutHMACKeyVersion)
		if err != nil {
			return DiditWebhookOutcome{}, err
		}
	}
	sessionToDelete := attempt.ProviderSessionID
	if sessionToDelete == "" {
		sessionToDelete = sessionID
	}
	commit := CommitWebhookDecisionInput{
		AttemptID: attempt.ID, EventRowID: evt.ID, NextStatus: nextStatus,
		DecisionKeyVersion: keyVersion, DecisionAlgorithm: DecisionAlgorithmV1,
		EncryptedDecision: ciphertext, ProcessedAt: processedAt,
		RutFingerprint: rutFingerprint,
	}
	if nextStatus == "APPROVED" && sessionToDelete != "" {
		commit.DeleteSessionID = sessionToDelete
		commit.PurgeDeclaration = rutFingerprint != ""
	}
	updated, err := s.repo.CommitWebhookDecision(ctx, commit)
	if errors.Is(err, ErrConflict) && rutFingerprint != "" {
		flags["RUT_CLAIM_CONFLICT"] = true
		keyVersion, ciphertext, err = sealDecision("REJECTED", flags)
		if err != nil {
			return DiditWebhookOutcome{}, err
		}
		commit.NextStatus = "REJECTED"
		commit.DecisionKeyVersion = keyVersion
		commit.EncryptedDecision = ciphertext
		commit.RutFingerprint = ""
		commit.DeleteSessionID = ""
		commit.PurgeDeclaration = false
		updated, err = s.repo.CommitWebhookDecision(ctx, commit)
	}
	if err != nil {
		return DiditWebhookOutcome{}, err
	}
	s.clearEphemeralURL(attempt.ID)

	remoteDeleted := false
	if updated.Status == "APPROVED" && commit.DeleteSessionID != "" {
		if eraser, ok := s.client.(DiditSessionEraser); ok {
			if err := eraser.DeleteSession(ctx, commit.DeleteSessionID); err == nil {
				remoteDeleted = true
				if s.deletionQueue != nil {
					if err := s.deletionQueue.AcknowledgeSessionDeletion(ctx, commit.DeleteSessionID); err != nil {
						s.recordPendingDeletion(commit.DeleteSessionID)
						return DiditWebhookOutcome{}, err
					}
				}
			} else {
				s.recordPendingDeletion(commit.DeleteSessionID)
			}
		} else {
			s.recordPendingDeletion(commit.DeleteSessionID)
		}
	}

	return DiditWebhookOutcome{
		AttemptID:        updated.ID,
		AttemptStatus:    updated.Status,
		ProcessingStatus: "PROCESSED",
		Duplicate:        duplicate,
		RemoteDeleted:    remoteDeleted,
	}, nil
}

// DecryptAttemptDecision decrypts and validates the minimal decision snapshot stored on an attempt.
func (s *DiditService) DecryptAttemptDecision(attempt ProviderAttempt) (EncryptedDecisionSnapshot, error) {
	if s == nil || len(attempt.EncryptedDecision) == 0 || attempt.DecisionAlgorithm != DecisionAlgorithmV1 {
		return EncryptedDecisionSnapshot{}, ErrNotFound
	}
	aad, err := BuildDecisionAAD(attempt.AccountID, attempt.ID, attempt.Provider, attempt.Environment, DecisionSchemaVersion)
	if err != nil {
		return EncryptedDecisionSnapshot{}, err
	}
	plain, err := s.keys.Open(attempt.DecisionKeyVersion, aad, attempt.EncryptedDecision)
	if err != nil {
		return EncryptedDecisionSnapshot{}, err
	}
	var out EncryptedDecisionSnapshot
	if err := json.Unmarshal(plain, &out); err != nil {
		return EncryptedDecisionSnapshot{}, err
	}
	return out, nil
}

// PendingRemoteDeletions returns a copy of session IDs queued for remote deletion retry.
func (s *DiditService) PendingRemoteDeletions() []string {
	s.deleteOutboxMu.Lock()
	defer s.deleteOutboxMu.Unlock()
	return append([]string(nil), s.pendingDeletions...)
}

// RetryPendingDeletions attempts to delete sessions queued for remote retry.
func (s *DiditService) RetryPendingDeletions(ctx context.Context) ([]string, error) {
	eraser, ok := s.client.(DiditSessionEraser)
	if !ok {
		return nil, nil
	}

	s.deleteOutboxMu.Lock()
	sessions := append([]string(nil), s.pendingDeletions...)
	s.deleteOutboxMu.Unlock()
	if s.deletionQueue != nil {
		pending, err := s.deletionQueue.PendingSessionDeletions(ctx)
		if err != nil {
			return nil, err
		}
		sessions = pending
	}

	var deleted []string
	var remaining []string
	var firstErr error
	for _, sess := range sessions {
		if err := eraser.DeleteSession(ctx, sess); err != nil {
			remaining = append(remaining, sess)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if s.deletionQueue != nil {
			if err := s.deletionQueue.AcknowledgeSessionDeletion(ctx, sess); err != nil {
				// Keep the durable record pending. Deleting the provider session is idempotent,
				// so a later worker retry safely reconciles an acknowledgement failure.
				remaining = append(remaining, sess)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
		}
		deleted = append(deleted, sess)
	}

	s.deleteOutboxMu.Lock()
	s.pendingDeletions = remaining
	s.deleteOutboxMu.Unlock()
	return deleted, firstErr
}

func (s *DiditService) recordPendingDeletion(sessionID string) {
	s.deleteOutboxMu.Lock()
	defer s.deleteOutboxMu.Unlock()
	for _, existing := range s.pendingDeletions {
		if existing == sessionID {
			return
		}
	}
	s.pendingDeletions = append(s.pendingDeletions, sessionID)
}

func (s *DiditService) setEphemeralURL(attemptID, sessionURL string, expiresAt time.Time) {
	s.urlMu.Lock()
	defer s.urlMu.Unlock()
	s.ephemeralURLs[attemptID] = ephemeralSessionURL{URL: sessionURL, ExpiresAt: expiresAt}
}

func (s *DiditService) getEphemeralURL(attemptID string, now time.Time) string {
	s.urlMu.Lock()
	defer s.urlMu.Unlock()
	entry, ok := s.ephemeralURLs[attemptID]
	if !ok {
		return ""
	}
	if !entry.ExpiresAt.After(now) {
		delete(s.ephemeralURLs, attemptID)
		return ""
	}
	return entry.URL
}

func (s *DiditService) clearEphemeralURL(attemptID string) {
	s.urlMu.Lock()
	defer s.urlMu.Unlock()
	delete(s.ephemeralURLs, attemptID)
}
