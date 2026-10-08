package identity

import (
	"context"
	"time"
)

// Exactly one lookup is required. Locks serialize login, action-token, reissue
// and authorization for an account across processes, not just goroutines.
type AccountLookup struct{ Email, VerificationID, ActionTokenID, SessionHash string }

type AuthenticationRepository interface {
	CreateWithTerms(context.Context, Account, []TermsAcceptance) error
	TermsVersion(context.Context, string) (TermsVersion, error)
	WithLockedAccount(context.Context, AccountLookup, func(Account, AuthenticationTransaction) error) error
	CredentialNoticeRepository
}

// Callbacks return persistence errors to roll back. Business rejections whose
// counters must persist are returned by the service only after a successful commit.
type AuthenticationTransaction interface {
	SaveLoginState(context.Context, string, AccountState, int, *time.Time) error
	CreateSession(context.Context, Session) error
	SessionByTokenHash(context.Context, string) (Session, error)
	RevokeSession(context.Context, string, time.Time) error
	TouchSession(context.Context, string, time.Time) (bool, error)
	Roles(context.Context, string) ([]Role, error)
	ActionToken(context.Context, string) (ActionToken, error)
	ReplaceActionToken(context.Context, ActionToken) error
	UpdatePasswordHash(context.Context, string, Secret) error
	DeleteExpiredPasswordHistory(context.Context, string, time.Time) error
	PreviousPasswordHashes(context.Context, string, time.Time) ([]Secret, error)
	StorePreviousPasswordHash(context.Context, string, string, Secret, time.Time, time.Time, time.Time) error
	EnqueueCredentialChanged(context.Context, string, string, string, time.Time) error
	RecordCredentialChangeAudit(context.Context, string, string, string, string, time.Time, time.Time) error
	RevokeActiveSessions(context.Context, string, time.Time) error
	CountActionTokenEmissions(context.Context, string, string, time.Time) (int64, error)
	RecordActionTokenFailure(context.Context, string, time.Time) (bool, error)
	ConsumeActionToken(context.Context, string, time.Time) (bool, error)
}

type CredentialNotice struct {
	ID, AccountID           string
	Attempts, CycleAttempts int
	CycleNumber             int
	LeaseUntil              time.Time
}

type CredentialNoticeSummary struct {
	ID            string    `json:"event_id"`
	AccountID     string    `json:"-"`
	ErrorCode     string    `json:"error_code"`
	TotalAttempts int       `json:"total_attempts"`
	CycleNumber   int       `json:"cycle_number"`
	CycleAttempts int       `json:"cycle_attempts"`
	FailedAt      time.Time `json:"failed_at"`
	RemoveAt      time.Time `json:"remove_at"`
}

type CredentialNoticeCycle struct {
	EventID        string    `json:"event_id"`
	State          string    `json:"state"`
	ReasonCode     string    `json:"reason_code"`
	IdempotencyKey string    `json:"idempotency_key"`
	CycleNumber    int       `json:"cycle_number"`
	TotalAttempts  int       `json:"total_attempts"`
	CycleAttempts  int       `json:"cycle_attempts"`
	StartedAt      time.Time `json:"started_at"`
	Reused         bool      `json:"reused"`
}

type ReopenCredentialNoticeInput struct {
	EventID, ActorID, AuditID, ReasonCode, CorrelationID, IdempotencyKey string
	At, AuditRemoveAt                                                    time.Time
}

type CredentialNoticeRepository interface {
	PurgeExpiredPasswordHistory(context.Context, time.Time) error
	PurgeExpiredCredentialNotices(context.Context, time.Time, int) (int64, error)
	CancelInactiveCredentialNotices(context.Context, time.Time, int) (int64, error)
	ListTerminalCredentialNotices(context.Context) ([]CredentialNoticeSummary, error)
	ReopenCredentialNotice(context.Context, ReopenCredentialNoticeInput) (CredentialNoticeCycle, error)
	ClaimCredentialNotice(context.Context, time.Time, time.Time) (CredentialNotice, error)
	ActiveAccountEmail(context.Context, string) (string, error)
	CancelInactiveCredentialNotice(context.Context, string, time.Time, time.Time) error
	CompleteCredentialNotice(context.Context, string, time.Time, time.Time) error
	RetryCredentialNotice(context.Context, string, time.Time, time.Time, time.Time, string) error
}

type PasswordHasher interface {
	Hash(Secret) (Secret, error)
	Matches(Secret, Secret) (bool, error)
}

type VerificationDelivery struct {
	AccountID, TokenID, Email string
	Token                     Secret
	ExpiresAt                 time.Time
}

type RecoveryDelivery struct {
	AccountID, TokenID, Email string
	Token                     Secret
	ExpiresAt                 time.Time
}

// Delivery happens after commit. Failure is visible and retry uses the ordinary
// reissue flow; this port does not claim durable/real provider delivery.
type AuthenticationMailer interface {
	SendVerification(context.Context, VerificationDelivery) error
	SendRecovery(context.Context, RecoveryDelivery) error
	SendPasswordChanged(context.Context, string) error
	SendLoginAlert(context.Context, string, time.Time) error
}

// A deployment must supply an atomic policy implementation and a trusted client
// IP from its transport. No production threshold was ratified in M01.
type VerificationIPLimiter interface {
	AllowVerification(context.Context, string, time.Time) (bool, error)
}

type CredentialGenerator interface {
	ID() (string, error)
	Token() (Secret, error)
}
