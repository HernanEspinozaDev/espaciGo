package identity

import (
	"context"
	"time"
)

// AccountRepository persists M01 account-owned identity data. CreateWithTerms is atomic.
type AccountRepository interface {
	CreateWithTerms(context.Context, Account, []TermsAcceptance) error
	AccountByNormalizedEmail(context.Context, string) (Account, error)
	AccountByID(context.Context, string) (Account, error)
	SaveLoginState(context.Context, string, AccountState, int, *time.Time) error
	CreateSession(context.Context, Session) error
	SessionByTokenHash(context.Context, string) (Session, error)
	RevokeSession(context.Context, string, time.Time) error
	TouchSession(context.Context, string, time.Time) (bool, error)
	TermsVersion(context.Context, string) (TermsVersion, error)
	AcceptTerms(context.Context, TermsAcceptance) error
}

// ActionTokenRepository exposes only the M01 primitives needed by Backend flows.
// Replacing is atomic: active prior tokens for the same account/purpose are invalidated
// before the new token is inserted. Emission counts include expired/invalidated rows
// created in the requested window, because each row represents one issuance.
type ActionTokenRepository interface {
	ReplaceActiveActionToken(context.Context, ActionToken) error
	ActionTokenByHash(context.Context, string) (ActionToken, error)
	RecordActionTokenFailure(context.Context, string, time.Time) (bool, error)
	ConsumeActionToken(context.Context, string, time.Time) (bool, error)
	CountActionTokenEmissions(context.Context, string, string, time.Time) (int64, error)
}
