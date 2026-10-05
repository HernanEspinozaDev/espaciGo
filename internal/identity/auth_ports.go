package identity

import (
	"context"
	"time"
)

// Exactly one lookup is required. Locks serialize login, verification, reissue
// and authorization for an account across processes, not just goroutines.
type AccountLookup struct{ Email, VerificationID, SessionHash string }

type AuthenticationRepository interface {
	CreateWithTerms(context.Context, Account, []TermsAcceptance) error
	TermsVersion(context.Context, string) (TermsVersion, error)
	WithLockedAccount(context.Context, AccountLookup, func(Account, AuthenticationTransaction) error) error
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
	VerificationToken(context.Context, string) (ActionToken, error)
	ReplaceVerificationToken(context.Context, ActionToken) error
	CountActionTokenEmissions(context.Context, string, string, time.Time) (int64, error)
	RecordActionTokenFailure(context.Context, string, time.Time) (bool, error)
	ConsumeActionToken(context.Context, string, time.Time) (bool, error)
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

// Delivery happens after commit. Failure is visible and retry uses the ordinary
// reissue flow; this port does not claim durable/real provider delivery.
type AuthenticationMailer interface {
	SendVerification(context.Context, VerificationDelivery) error
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
