package identity

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/mail"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrEmailRegistered = errors.New("El correo ya está registrado; inicia sesión o recupera tu contraseña")
	ErrCredentials     = errors.New("identity: incorrect credentials")
	ErrEmailUnverified = errors.New("identity: verify email before signing in")
	ErrAccountDisabled = errors.New("identity: account disabled")
	ErrTokenInvalid    = errors.New("identity: invalid verification token")
	ErrRateLimited     = errors.New("identity: verification rate limited")
	ErrUnauthorized    = errors.New("identity: unauthorized")
	ErrForbidden       = errors.New("identity: role not granted")
	ErrDelivery        = errors.New("identity: mail delivery failed")
	ErrCurrentPassword = errors.New("La contraseña actual no es correcta")
	ErrPasswordSame    = errors.New("La contraseña nueva debe ser distinta de la actual")
	ErrPasswordConfirm = errors.New("La repetición de la contraseña nueva no coincide")
)

const (
	verificationPurpose = "verificar_correo"
	recoveryPurpose     = "recuperar_clave"
)

type LoginBlockedError struct{ Until time.Time }

func (e *LoginBlockedError) Error() string {
	return "identity: login blocked until " + e.Until.UTC().Format(time.RFC3339)
}

type AuthenticationService struct {
	repo        AuthenticationRepository
	passwords   PasswordHasher
	mailer      AuthenticationMailer
	ipLimiter   VerificationIPLimiter
	credentials CredentialGenerator
	now         func() time.Time
}

func NewAuthenticationService(repo AuthenticationRepository, passwords PasswordHasher, mailer AuthenticationMailer, limiter VerificationIPLimiter, credentials CredentialGenerator, now func() time.Time) (*AuthenticationService, error) {
	if repo == nil || passwords == nil || mailer == nil || limiter == nil || credentials == nil || now == nil {
		return nil, ErrInvalid
	}
	return &AuthenticationService{repo, passwords, mailer, limiter, credentials, now}, nil
}

// Validate the received email before deriving the shared M01 key. Display names,
// comments and multiple addresses are not account emails. Outer space is retained.
func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Name != "" || parsed.Address != trimmed || strings.Count(trimmed, "@") != 1 {
		return ErrInvalid
	}
	return nil
}

func ValidatePassword(password Secret) error {
	value := string(password)
	// bcrypt's byte limit is enforced explicitly; never silently truncate.
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) < 8 || len(value) > 72 {
		return ErrInvalid
	}
	var upper, digit, special bool
	for _, r := range value {
		upper = upper || unicode.IsUpper(r)
		digit = digit || unicode.IsDigit(r)
		special = special || unicode.IsPunct(r) || unicode.IsSymbol(r)
	}
	if !upper || !digit || !special {
		return ErrInvalid
	}
	return nil
}

func CredentialHash(raw Secret) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

type RegisterInput struct {
	Email             string
	Password          Secret
	TermsVersionIDs   []string
	Channel, ClientIP string
}

// Registration persists account/tenant role/terms atomically. Token issuance and
// mail follow commit: a delivery failure leaves a pending account that can reissue.
func (s *AuthenticationService) Register(ctx context.Context, input RegisterInput) (string, error) {
	if ValidateEmail(input.Email) != nil || ValidatePassword(input.Password) != nil || len(input.TermsVersionIDs) == 0 {
		return "", ErrInvalid
	}
	if input.Channel != "web" && input.Channel != "api" {
		return "", ErrInvalid
	}
	if _, err := netip.ParseAddr(input.ClientIP); err != nil {
		return "", ErrInvalid
	}
	now := s.now()
	versions := make(map[string]bool)
	hasTerms := false
	for _, id := range input.TermsVersionIDs {
		if versions[id] {
			return "", ErrInvalid
		}
		versions[id] = true
		version, err := s.repo.TermsVersion(ctx, id)
		if err != nil {
			return "", err
		}
		if version.PublishedAt.After(now) {
			return "", ErrInvalid
		}
		hasTerms = hasTerms || version.Type == "terminos"
	}
	if !hasTerms {
		return "", ErrInvalid
	}
	hash, err := s.passwords.Hash(input.Password)
	if err != nil {
		return "", err
	}
	id, err := s.credentials.ID()
	if err != nil {
		return "", err
	}
	account := Account{ID: id, Email: input.Email, NormalizedEmail: NormalizeEmail(input.Email), PasswordHash: hash, State: AccountEmailPending, CreatedAt: now, UpdatedAt: now}
	acceptances := make([]TermsAcceptance, 0, len(versions))
	for _, version := range input.TermsVersionIDs {
		acceptanceID, err := s.credentials.ID()
		if err != nil {
			return "", err
		}
		acceptances = append(acceptances, TermsAcceptance{ID: acceptanceID, AccountID: id, VersionID: version, Channel: input.Channel, AcceptedAt: now})
	}
	if err := s.repo.CreateWithTerms(ctx, account, acceptances); err != nil {
		if errors.Is(err, ErrConflict) {
			return "", ErrEmailRegistered
		}
		return "", err
	}
	return id, s.ReissueVerification(ctx, input.Email, input.ClientIP)
}

func (s *AuthenticationService) ReissueVerification(ctx context.Context, email, clientIP string) error {
	if ValidateEmail(email) != nil {
		return ErrInvalid
	}
	address, err := netip.ParseAddr(clientIP)
	if err != nil {
		return ErrInvalid
	}
	now := s.now()
	allowed, err := s.ipLimiter.AllowVerification(ctx, address.Unmap().String(), now)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimited
	}
	var delivery VerificationDelivery
	var denied error
	err = s.repo.WithLockedAccount(ctx, AccountLookup{Email: NormalizeEmail(email)}, func(account Account, tx AuthenticationTransaction) error {
		if account.State != AccountEmailPending {
			denied = ErrAccountDisabled
			return nil
		}
		count, err := tx.CountActionTokenEmissions(ctx, account.ID, verificationPurpose, now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if count >= 3 {
			denied = ErrRateLimited
			return nil
		}
		raw, err := s.credentials.Token()
		if err != nil {
			return err
		}
		id, err := s.credentials.ID()
		if err != nil {
			return err
		}
		token := ActionToken{ID: id, AccountID: account.ID, Purpose: verificationPurpose, Hash: CredentialHash(raw), CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
		if err := tx.ReplaceActionToken(ctx, token); err != nil {
			return err
		}
		delivery = VerificationDelivery{AccountID: account.ID, TokenID: id, Email: account.Email, Token: raw, ExpiresAt: token.ExpiresAt}
		return nil
	})
	if err != nil {
		return err
	}
	if denied != nil {
		return denied
	}
	if s.mailer.SendVerification(ctx, delivery) != nil {
		return ErrDelivery
	}
	return nil
}

func (s *AuthenticationService) VerifyEmail(ctx context.Context, tokenID string, raw Secret, clientIP string) error {
	if tokenID == "" {
		return ErrTokenInvalid
	}
	now := s.now()
	address, err := netip.ParseAddr(clientIP)
	if err != nil {
		return ErrInvalid
	}
	allowed, err := s.ipLimiter.AllowVerification(ctx, address.Unmap().String(), now)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimited
	}
	var denied error
	err = s.repo.WithLockedAccount(ctx, AccountLookup{VerificationID: tokenID}, func(account Account, tx AuthenticationTransaction) error {
		token, err := tx.ActionToken(ctx, tokenID)
		if err != nil {
			return err
		}
		if account.State != AccountEmailPending || token.Purpose != verificationPurpose || token.AccountID != account.ID || now.Before(token.CreatedAt) || !now.Before(token.ExpiresAt) || token.ConsumedAt != nil || token.InvalidatedAt != nil || token.Attempts >= 5 {
			denied = ErrTokenInvalid
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(token.Hash), []byte(CredentialHash(raw))) != 1 {
			_, err := tx.RecordActionTokenFailure(ctx, token.Hash, now)
			denied = ErrTokenInvalid
			return err
		}
		consumed, err := tx.ConsumeActionToken(ctx, token.Hash, now)
		if err != nil {
			return err
		}
		if !consumed {
			denied = ErrTokenInvalid
			return nil
		}
		// Consumption and activation commit together, or both roll back.
		// Email verification must not shorten an existing login block.
		if account.BlockedUntil != nil && now.Before(*account.BlockedUntil) {
			return tx.SaveLoginState(ctx, account.ID, AccountBlocked, account.FailedAttempts, account.BlockedUntil)
		}
		return tx.SaveLoginState(ctx, account.ID, AccountActive, 0, nil)
	})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		return ErrTokenInvalid
	}
	if err != nil {
		return err
	}
	return denied
}

// RequestPasswordRecovery intentionally returns the same result for missing,
// ineligible and over-account-quota accounts. Only a trusted-IP quota rejection
// is visible, and it happens before account lookup.
func (s *AuthenticationService) RequestPasswordRecovery(ctx context.Context, email, clientIP string) error {
	if ValidateEmail(email) != nil {
		return ErrInvalid
	}
	address, err := netip.ParseAddr(clientIP)
	if err != nil {
		return ErrInvalid
	}
	now := s.now()
	allowed, err := s.ipLimiter.AllowVerification(ctx, address.Unmap().String(), now)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimited
	}
	var delivery RecoveryDelivery
	err = s.repo.WithLockedAccount(ctx, AccountLookup{Email: NormalizeEmail(email)}, func(account Account, tx AuthenticationTransaction) error {
		// Do not send a credential reset to an address that has not been verified,
		// or to a disabled account. The public response remains generic.
		if account.State != AccountActive && account.State != AccountBlocked {
			return nil
		}
		count, err := tx.CountActionTokenEmissions(ctx, account.ID, recoveryPurpose, now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if count >= 3 {
			return nil
		}
		raw, err := s.credentials.Token()
		if err != nil {
			return err
		}
		id, err := s.credentials.ID()
		if err != nil {
			return err
		}
		token := ActionToken{ID: id, AccountID: account.ID, Purpose: recoveryPurpose, Hash: CredentialHash(raw), CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
		if err := tx.ReplaceActionToken(ctx, token); err != nil {
			return err
		}
		delivery = RecoveryDelivery{AccountID: account.ID, TokenID: id, Email: account.Email, Token: raw, ExpiresAt: token.ExpiresAt}
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if delivery.TokenID != "" {
		// Delivery failure is deliberately hidden from the requester to preserve
		// the same public result as an unknown address. No token is logged.
		_ = s.mailer.SendRecovery(ctx, delivery)
	}
	return nil
}

type ResetPasswordInput struct {
	TokenID, ClientIP             string
	Token, Password, Confirmation Secret
}

func (s *AuthenticationService) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	if input.TokenID == "" || input.Token == "" {
		return ErrTokenInvalid
	}
	if err := ValidatePassword(input.Password); err != nil {
		return err
	}
	if input.Password != input.Confirmation {
		return ErrPasswordConfirm
	}
	address, err := netip.ParseAddr(input.ClientIP)
	if err != nil {
		return ErrInvalid
	}
	now := s.now()
	allowed, err := s.ipLimiter.AllowVerification(ctx, address.Unmap().String(), now)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimited
	}
	var email string
	var denied error
	err = s.repo.WithLockedAccount(ctx, AccountLookup{ActionTokenID: input.TokenID}, func(account Account, tx AuthenticationTransaction) error {
		token, err := tx.ActionToken(ctx, input.TokenID)
		if err != nil {
			return err
		}
		if token.Purpose != recoveryPurpose || token.AccountID != account.ID || now.Before(token.CreatedAt) || !now.Before(token.ExpiresAt) || token.ConsumedAt != nil || token.InvalidatedAt != nil || token.Attempts >= 5 {
			denied = ErrTokenInvalid
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(token.Hash), []byte(CredentialHash(input.Token))) != 1 {
			_, err := tx.RecordActionTokenFailure(ctx, token.Hash, now)
			denied = ErrTokenInvalid
			return err
		}
		consumed, err := tx.ConsumeActionToken(ctx, token.Hash, now)
		if err != nil {
			return err
		}
		if !consumed {
			denied = ErrTokenInvalid
			return nil
		}
		newHash, err := s.passwords.Hash(input.Password)
		if err != nil {
			return err
		}
		if err := tx.UpdatePasswordHash(ctx, account.ID, newHash); err != nil {
			return err
		}
		if err := tx.RevokeActiveSessions(ctx, account.ID, now); err != nil {
			return err
		}
		email = account.Email
		return nil
	})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		return ErrTokenInvalid
	}
	if err != nil {
		return err
	}
	if denied != nil {
		return denied
	}
	if s.mailer.SendPasswordChanged(ctx, email) != nil {
		return ErrDelivery
	}
	return nil
}

type ChangePasswordInput struct {
	SessionToken                            string
	CurrentPassword, Password, Confirmation Secret
}

func (s *AuthenticationService) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	if input.SessionToken == "" {
		return ErrUnauthorized
	}
	if err := ValidatePassword(input.Password); err != nil {
		return err
	}
	if input.Password != input.Confirmation {
		return ErrPasswordConfirm
	}
	now := s.now()
	hash := CredentialHash(Secret(input.SessionToken))
	var email string
	var denied error
	err := s.repo.WithLockedAccount(ctx, AccountLookup{SessionHash: hash}, func(account Account, tx AuthenticationTransaction) error {
		if account.State != AccountActive || (account.BlockedUntil != nil && now.Before(*account.BlockedUntil)) {
			denied = ErrUnauthorized
			return nil
		}
		session, err := tx.SessionByTokenHash(ctx, hash)
		if err != nil {
			return err
		}
		if session.AccountID != account.ID || now.Before(session.CreatedAt) || !session.ActiveAt(now) {
			denied = ErrUnauthorized
			return nil
		}
		currentOK, err := s.passwords.Matches(account.PasswordHash, input.CurrentPassword)
		if err != nil {
			return err
		}
		if !currentOK {
			denied = ErrCurrentPassword
			return nil
		}
		same, err := s.passwords.Matches(account.PasswordHash, input.Password)
		if err != nil {
			return err
		}
		if same {
			denied = ErrPasswordSame
			return nil
		}
		newHash, err := s.passwords.Hash(input.Password)
		if err != nil {
			return err
		}
		if err := tx.UpdatePasswordHash(ctx, account.ID, newHash); err != nil {
			return err
		}
		if err := tx.RevokeActiveSessions(ctx, account.ID, now); err != nil {
			return err
		}
		email = account.Email
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}
	if denied != nil {
		return denied
	}
	if s.mailer.SendPasswordChanged(ctx, email) != nil {
		return ErrDelivery
	}
	return nil
}

type LoginInput struct {
	Email         string
	Password      Secret
	ClientSummary string
}
type LoginResult struct {
	AccountID string
	Roles     []Role
	Token     Secret
	ExpiresAt time.Time
}

func (s *AuthenticationService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if ValidateEmail(input.Email) != nil {
		return LoginResult{}, ErrCredentials
	}
	now := s.now()
	var result LoginResult
	var denied error
	var alertEmail string
	var alertUntil time.Time
	err := s.repo.WithLockedAccount(ctx, AccountLookup{Email: NormalizeEmail(input.Email)}, func(account Account, tx AuthenticationTransaction) error {
		state, attempts, blocked := account.State, account.FailedAttempts, account.BlockedUntil
		if blocked != nil && now.Before(*blocked) {
			denied = &LoginBlockedError{*blocked}
			return nil
		}
		if blocked != nil && !now.Before(*blocked) {
			if state == AccountBlocked {
				state = AccountActive
			}
			attempts = 0
			blocked = nil
			if err := tx.SaveLoginState(ctx, account.ID, state, attempts, blocked); err != nil {
				return err
			}
		}
		if state != AccountActive && state != AccountEmailPending {
			denied = ErrAccountDisabled
			return nil
		}
		correct, err := s.passwords.Matches(account.PasswordHash, input.Password)
		if err != nil {
			return err
		}
		if !correct {
			attempts++
			if attempts >= 5 {
				until := now.Add(30 * time.Minute)
				blocked = &until
				// Never convert an unverified account into an active account on unlock.
				if state == AccountActive {
					state = AccountBlocked
				}
				alertEmail = account.Email
				alertUntil = until
				denied = &LoginBlockedError{until}
			} else {
				denied = ErrCredentials
			}
			return tx.SaveLoginState(ctx, account.ID, state, attempts, blocked)
		}
		if state == AccountEmailPending {
			denied = ErrEmailUnverified
			return nil
		}
		roles, err := tx.Roles(ctx, account.ID)
		if err != nil {
			return err
		}
		raw, err := s.credentials.Token()
		if err != nil {
			return err
		}
		id, err := s.credentials.ID()
		if err != nil {
			return err
		}
		session := Session{ID: id, AccountID: account.ID, TokenHash: CredentialHash(raw), CreatedAt: now, LastActivityAt: now, ExpiresAt: now.Add(8 * time.Hour), ClientSummary: input.ClientSummary}
		if err := tx.CreateSession(ctx, session); err != nil {
			return err
		}
		if err := tx.SaveLoginState(ctx, account.ID, AccountActive, 0, nil); err != nil {
			return err
		}
		result = LoginResult{AccountID: account.ID, Roles: roles, Token: raw, ExpiresAt: session.ExpiresAt}
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return LoginResult{}, ErrCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if alertEmail != "" && s.mailer.SendLoginAlert(ctx, alertEmail, alertUntil) != nil {
		return LoginResult{}, errors.Join(denied, ErrDelivery)
	}
	return result, denied
}

type ActivityKind int

const (
	UserOperation ActivityKind = iota
	HealthCheck
	AutomaticPolling
	Keepalive
	TokenRenewal
)

type Principal struct {
	AccountID string
	Roles     []Role
}

func (s *AuthenticationService) Authorize(ctx context.Context, raw Secret, required Role, activity ActivityKind) (Principal, error) {
	if raw == "" || activity < UserOperation || activity > TokenRenewal {
		return Principal{}, ErrUnauthorized
	}
	now := s.now()
	hash := CredentialHash(raw)
	var result Principal
	var denied error
	err := s.repo.WithLockedAccount(ctx, AccountLookup{SessionHash: hash}, func(account Account, tx AuthenticationTransaction) error {
		if account.State != AccountActive || (account.BlockedUntil != nil && now.Before(*account.BlockedUntil)) {
			denied = ErrUnauthorized
			return nil
		}
		session, err := tx.SessionByTokenHash(ctx, hash)
		if err != nil {
			return err
		}
		if session.AccountID != account.ID || now.Before(session.CreatedAt) || !session.ActiveAt(now) {
			denied = ErrUnauthorized
			return nil
		}
		roles, err := tx.Roles(ctx, account.ID)
		if err != nil {
			return err
		}
		if required != "" {
			granted := false
			for _, role := range roles {
				granted = granted || role == required
			}
			if !granted {
				denied = ErrForbidden
				return nil
			}
		}
		if activity == UserOperation {
			updated, err := tx.TouchSession(ctx, session.ID, now)
			if err != nil {
				return err
			}
			if !updated {
				denied = ErrUnauthorized
				return nil
			}
		}
		result = Principal{AccountID: account.ID, Roles: roles}
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, err
	}
	return result, denied
}

func (s *AuthenticationService) Logout(ctx context.Context, raw Secret) error {
	if raw == "" {
		return ErrUnauthorized
	}
	hash := CredentialHash(raw)
	now := s.now()
	err := s.repo.WithLockedAccount(ctx, AccountLookup{SessionHash: hash}, func(account Account, tx AuthenticationTransaction) error {
		session, err := tx.SessionByTokenHash(ctx, hash)
		if err != nil {
			return err
		}
		if session.AccountID != account.ID || now.Before(session.CreatedAt) {
			return ErrUnauthorized
		}
		if session.RevokedAt != nil {
			return nil
		}
		return tx.RevokeSession(ctx, session.ID, now)
	})
	if errors.Is(err, ErrNotFound) {
		return ErrUnauthorized
	}
	return err
}
