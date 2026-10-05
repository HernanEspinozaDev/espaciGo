package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Test-only hasher keeps business concurrency tests fast. The concrete bcrypt
// adapter is verified separately at the required cost 12.
type authTestHasher struct{}

func (authTestHasher) Hash(raw identity.Secret) (identity.Secret, error) {
	return identity.Secret(identity.CredentialHash(raw)), nil
}
func (authTestHasher) Matches(hash, raw identity.Secret) (bool, error) {
	return string(hash) == identity.CredentialHash(raw), nil
}

type authTestGenerator struct{ count atomic.Int64 }

func (g *authTestGenerator) ID() (string, error) {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", g.count.Add(1)), nil
}
func (g *authTestGenerator) Token() (identity.Secret, error) {
	return identity.Secret(fmt.Sprintf("synthetic-test-credential-%d", g.count.Add(1))), nil
}

type authTestMail struct {
	mu         sync.Mutex
	messages   []identity.VerificationDelivery
	recoveries []identity.RecoveryDelivery
	alerts     int
	changes    int
	fail       bool
}

func (m *authTestMail) SendRecovery(_ context.Context, d identity.RecoveryDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("synthetic mail failure")
	}
	m.recoveries = append(m.recoveries, d)
	return nil
}
func (m *authTestMail) SendPasswordChanged(context.Context, string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("synthetic mail failure")
	}
	m.changes++
	return nil
}

func (m *authTestMail) SendVerification(_ context.Context, d identity.VerificationDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("synthetic mail failure")
	}
	m.messages = append(m.messages, d)
	return nil
}
func (m *authTestMail) SendLoginAlert(context.Context, string, time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("synthetic alert failure")
	}
	m.alerts++
	return nil
}
func (m *authTestMail) last() identity.VerificationDelivery {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages[len(m.messages)-1]
}

// Deliberately test-only policy, not a ratified deployment threshold.
type authTestIPLimiter struct {
	mu    sync.Mutex
	calls map[string][]time.Time
	max   int
}

func (l *authTestIPLimiter) AllowVerification(_ context.Context, ip string, at time.Time) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls == nil {
		l.calls = make(map[string][]time.Time)
	}
	var recent []time.Time
	for _, old := range l.calls[ip] {
		if !old.Before(at.Add(-time.Hour)) {
			recent = append(recent, old)
		}
	}
	if len(recent) >= l.max {
		l.calls[ip] = recent
		return false, nil
	}
	l.calls[ip] = append(recent, at)
	return true, nil
}

type authHarness struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	repo    *identitypg.IdentityRepository
	service *identity.AuthenticationService
	mail    *authTestMail
	limiter *authTestIPLimiter
	now     time.Time
}

func newAuthHarness(t *testing.T) *authHarness {
	t.Helper()
	ctx, pool := newIdentityTestPool(t)
	h := &authHarness{ctx: ctx, pool: pool, repo: identitypg.NewIdentityRepository(pool), mail: &authTestMail{}, limiter: &authTestIPLimiter{max: 100}, now: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	service, err := identity.NewAuthenticationService(h.repo, authTestHasher{}, h.mail, h.limiter, &authTestGenerator{}, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	h.service = service
	return h
}
func (h *authHarness) register(t *testing.T, email string) string {
	t.Helper()
	id, err := h.service.Register(h.ctx, identity.RegisterInput{Email: email, Password: "Synthetic#123", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func (h *authHarness) verify(t *testing.T) {
	t.Helper()
	d := h.mail.last()
	if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); err != nil {
		t.Fatal(err)
	}
}
func (h *authHarness) login(t *testing.T, email string) identity.LoginResult {
	t.Helper()
	result, err := h.service.Login(h.ctx, identity.LoginInput{Email: email, Password: "Synthetic#123"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAuthRegistrationTermsDuplicateAndUnverifiedLogin(t *testing.T) {
	h := newAuthHarness(t)
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "absent@ejemplo.invalid", Password: "Synthetic#123"}); !errors.Is(err, identity.ErrCredentials) {
		t.Fatal("unknown account credentials not generic")
	}
	if _, err := h.service.Register(h.ctx, identity.RegisterInput{Email: "bad", Password: "bad"}); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("invalid registration accepted")
	}
	if _, err := h.service.Register(h.ctx, identity.RegisterInput{Email: "terms@ejemplo.invalid", Password: "Synthetic#123", Channel: "api", ClientIP: "192.0.2.1"}); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("registration without terms accepted")
	}
	id := h.register(t, " Straße+alias@Example.INVALID ")
	a, err := h.repo.AccountByNormalizedEmail(h.ctx, "strasse+alias@example.invalid")
	if err != nil || a.ID != id || a.Email != " Straße+alias@Example.INVALID " || string(a.PasswordHash) == "Synthetic#123" {
		t.Fatal("registration identity/hash roundtrip failed")
	}
	d := h.mail.last()
	token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(d.Token))
	if err != nil || token.ExpiresAt.Sub(token.CreatedAt) != 24*time.Hour || token.Hash == string(d.Token) {
		t.Fatal("verification TTL/hash not persisted")
	}
	var terms, tenants, admins int
	if err := h.pool.QueryRow(h.ctx, "SELECT (SELECT count(*) FROM aceptacion_terminos WHERE usuario_id=$1), (SELECT count(*) FROM rol_usuario WHERE usuario_id=$1 AND rol='arrendatario'), (SELECT count(*) FROM rol_usuario WHERE usuario_id=$1 AND rol='administrador')", id).Scan(&terms, &tenants, &admins); err != nil {
		t.Fatal(err)
	}
	if terms != 1 || tenants != 1 || admins != 0 {
		t.Fatal("terms/least role invariant broken")
	}
	if _, err := h.service.Register(h.ctx, identity.RegisterInput{Email: "STRASSE+alias@example.invalid", Password: "Synthetic#123", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"}); !errors.Is(err, identity.ErrEmailRegistered) {
		t.Fatal("canonical duplicate not explicit")
	}
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "strasse+alias@example.invalid", Password: "Synthetic#123"}); !errors.Is(err, identity.ErrEmailUnverified) {
		t.Fatal("unverified login allowed")
	}
	h.verify(t)
	login := h.login(t, "STRASSE+alias@example.invalid")
	if login.AccountID != id || len(login.Roles) != 1 || login.Roles[0] != identity.RoleTenant || login.ExpiresAt.Sub(h.now) != 8*time.Hour {
		t.Fatal("login roles/absolute expiry invalid")
	}
}

func TestAuthVerificationReissueFailureLimitTTLAndReplay(t *testing.T) {
	for _, scenario := range []string{"reissue", "failures", "ttl", "replay"} {
		t.Run(scenario, func(t *testing.T) {
			h := newAuthHarness(t)
			id := h.register(t, "verify@ejemplo.invalid")
			d := h.mail.last()
			switch scenario {
			case "reissue":
				if err := h.service.ReissueVerification(h.ctx, "VERIFY@ejemplo.invalid", "192.0.2.1"); err != nil {
					t.Fatal(err)
				}
				if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); !errors.Is(err, identity.ErrTokenInvalid) {
					t.Fatal("replaced token accepted")
				}
				h.verify(t)
			case "failures":
				for i := 0; i < 5; i++ {
					if err := h.service.VerifyEmail(h.ctx, d.TokenID, "incorrect-token", "192.0.2.99"); !errors.Is(err, identity.ErrTokenInvalid) {
						t.Fatal("wrong token accepted")
					}
				}
				token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(d.Token))
				if err != nil || token.Attempts != 5 {
					t.Fatal("failure counter not persisted")
				}
				if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); !errors.Is(err, identity.ErrTokenInvalid) {
					t.Fatal("exhausted token accepted")
				}
				a, err := h.repo.AccountByID(h.ctx, id)
				if err != nil || a.FailedAttempts != 0 || a.BlockedUntil != nil {
					t.Fatal("token failures blocked login")
				}
			case "ttl":
				h.now = h.now.Add(24 * time.Hour)
				if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); !errors.Is(err, identity.ErrTokenInvalid) {
					t.Fatal("expired boundary token accepted")
				}
			case "replay":
				h.verify(t)
				if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); !errors.Is(err, identity.ErrTokenInvalid) {
					t.Fatal("verification replay accepted")
				}
			}
		})
	}
}

func TestAuthLoginFailureBlockResetAndPendingAccountSafety(t *testing.T) {
	for _, verified := range []bool{true, false} {
		t.Run(fmt.Sprint(verified), func(t *testing.T) {
			h := newAuthHarness(t)
			id := h.register(t, "login@ejemplo.invalid")
			if verified {
				h.verify(t)
			}
			for i := 1; i <= 5; i++ {
				_, err := h.service.Login(h.ctx, identity.LoginInput{Email: "LOGIN@ejemplo.invalid", Password: "Wrong#123"})
				if i < 5 && !errors.Is(err, identity.ErrCredentials) {
					t.Fatal("incorrect credential not generic")
				}
				if i == 5 {
					var blocked *identity.LoginBlockedError
					if !errors.As(err, &blocked) || !blocked.Until.Equal(h.now.Add(30*time.Minute)) {
						t.Fatal("fifth failure did not block")
					}
				}
			}
			if h.mail.alerts != 1 {
				t.Fatal("missing block alert")
			}
			if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "login@ejemplo.invalid", Password: "Synthetic#123"}); err == nil {
				t.Fatal("blocked login accepted")
			}
			h.now = h.now.Add(30 * time.Minute)
			_, err := h.service.Login(h.ctx, identity.LoginInput{Email: "login@ejemplo.invalid", Password: "Synthetic#123"})
			if verified && err != nil {
				t.Fatal(err)
			}
			if !verified && !errors.Is(err, identity.ErrEmailUnverified) {
				t.Fatal("unlock activated pending account")
			}
			a, err := h.repo.AccountByID(h.ctx, id)
			if err != nil || a.FailedAttempts != 0 || a.BlockedUntil != nil {
				t.Fatal("expired block did not reset")
			}
		})
	}
}

func TestAuthSessionRoleStateTrafficExpiryAndLogout(t *testing.T) {
	for _, traffic := range []identity.ActivityKind{identity.UserOperation, identity.HealthCheck, identity.AutomaticPolling, identity.Keepalive, identity.TokenRenewal} {
		t.Run(fmt.Sprint(traffic), func(t *testing.T) {
			h := newAuthHarness(t)
			id := h.register(t, "session@ejemplo.invalid")
			h.verify(t)
			login := h.login(t, "session@ejemplo.invalid")
			created := h.now
			h.now = h.now.Add(20 * time.Minute)
			if _, err := h.service.Authorize(h.ctx, login.Token, identity.RoleAdministrator, traffic); !errors.Is(err, identity.ErrForbidden) {
				t.Fatal("tenant gained admin")
			}
			if _, err := h.service.Authorize(h.ctx, login.Token, identity.RoleTenant, traffic); err != nil {
				t.Fatal(err)
			}
			stored, err := h.repo.SessionByTokenHash(h.ctx, identity.CredentialHash(login.Token))
			if err != nil {
				t.Fatal(err)
			}
			expected := created
			if traffic == identity.UserOperation {
				expected = h.now
			}
			if !stored.LastActivityAt.Equal(expected) || !stored.ExpiresAt.Equal(created.Add(8*time.Hour)) {
				t.Fatal("excluded traffic renewed session or absolute expiry changed")
			}
			h.now = expected.Add(30 * time.Minute)
			if _, err := h.service.Authorize(h.ctx, login.Token, "", identity.UserOperation); !errors.Is(err, identity.ErrUnauthorized) {
				t.Fatal("idle boundary accepted")
			}
			h.now = created.Add(time.Hour)
			next := h.login(t, "session@ejemplo.invalid")
			if err := h.service.Logout(h.ctx, next.Token); err != nil {
				t.Fatal(err)
			}
			if err := h.service.Logout(h.ctx, next.Token); err != nil {
				t.Fatal("logout not idempotent")
			}
			if _, err := h.service.Authorize(h.ctx, next.Token, "", identity.UserOperation); !errors.Is(err, identity.ErrUnauthorized) {
				t.Fatal("revoked session replay allowed")
			}
			next = h.login(t, "session@ejemplo.invalid")
			if err := h.repo.SaveLoginState(h.ctx, id, identity.AccountClosureRequested, 0, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := h.service.Authorize(h.ctx, next.Token, "", identity.UserOperation); !errors.Is(err, identity.ErrUnauthorized) {
				t.Fatal("disabled account authorized")
			}
		})
	}
	h := newAuthHarness(t)
	h.register(t, "absolute@ejemplo.invalid")
	h.verify(t)
	login := h.login(t, "absolute@ejemplo.invalid")
	created := h.now
	for elapsed := 20 * time.Minute; elapsed < 8*time.Hour; elapsed += 20 * time.Minute {
		h.now = created.Add(elapsed)
		if _, err := h.service.Authorize(h.ctx, login.Token, "", identity.UserOperation); err != nil {
			t.Fatal(err)
		}
	}
	h.now = created.Add(8 * time.Hour)
	if _, err := h.service.Authorize(h.ctx, login.Token, "", identity.UserOperation); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatal("absolute boundary accepted")
	}
}

func TestAuthConcurrentReissueEnforcesAccountLimitAndIPIsIndependent(t *testing.T) {
	h := newAuthHarness(t)
	h.register(t, "concurrent@ejemplo.invalid")
	var successes, limited atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := h.service.ReissueVerification(h.ctx, "concurrent@ejemplo.invalid", "192.0.2.2")
			if err == nil {
				successes.Add(1)
			} else if errors.Is(err, identity.ErrRateLimited) {
				limited.Add(1)
			} else {
				t.Errorf("reissue: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 2 || limited.Load() != 6 {
		t.Fatal("concurrent emissions exceeded three per hour")
	}
	var activeID string
	if err := h.pool.QueryRow(h.ctx, "SELECT id::text FROM token_accion WHERE invalidado_en IS NULL AND consumido_en IS NULL").Scan(&activeID); err != nil {
		t.Fatal(err)
	}
	h.mail.mu.Lock()
	var d identity.VerificationDelivery
	for _, message := range h.mail.messages {
		if message.TokenID == activeID {
			d = message
		}
	}
	h.mail.mu.Unlock()
	if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); err != nil {
		t.Fatal(err)
	}
	var tokenCount int
	if err := h.pool.QueryRow(h.ctx, "SELECT count(*) FROM token_accion WHERE usuario_id=$1 AND invalidado_en IS NULL AND consumido_en IS NULL", d.AccountID).Scan(&tokenCount); err != nil || tokenCount != 0 {
		t.Fatal("replacement left more active tokens")
	}
	h.login(t, "concurrent@ejemplo.invalid")
	h2 := newAuthHarness(t)
	h2.limiter.max = 1
	h2.register(t, "ip@ejemplo.invalid")
	if err := h2.service.ReissueVerification(h2.ctx, "ip@ejemplo.invalid", "192.0.2.1"); !errors.Is(err, identity.ErrRateLimited) {
		t.Fatal("IP limit not applied")
	}
	if err := h2.service.ReissueVerification(h2.ctx, "ip@ejemplo.invalid", "192.0.2.2"); err != nil {
		t.Fatal("independent IP unexpectedly blocked")
	}
	h2.verify(t)
	h2.login(t, "ip@ejemplo.invalid")
}

func TestAuthConcurrentVerificationHasOneWinnerAndLoginCountsAllFailures(t *testing.T) {
	h := newAuthHarness(t)
	id := h.register(t, "races@ejemplo.invalid")
	d := h.mail.last()
	var successes atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99")
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, identity.ErrTokenInvalid) {
				t.Errorf("verify: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("verification not single-use under concurrency")
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.service.Login(h.ctx, identity.LoginInput{Email: "races@ejemplo.invalid", Password: "Wrong#123"})
			if err == nil {
				t.Error("wrong password accepted")
			}
		}()
	}
	wg.Wait()
	a, err := h.repo.AccountByID(h.ctx, id)
	if err != nil || a.FailedAttempts != 5 || a.State != identity.AccountBlocked || h.mail.alerts != 1 {
		t.Fatal("concurrent login failures lost updates/block")
	}
}

func TestAuthVerificationActivationRollsBackAndDeliveryFailureIsRetryable(t *testing.T) {
	h := newAuthHarness(t)
	id := h.register(t, "rollback@ejemplo.invalid")
	d := h.mail.last()
	// Inject an actual DB failure after consumption, before account activation.
	if _, err := h.pool.Exec(h.ctx, `CREATE FUNCTION reject_activation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic activation failure'; END; $$; CREATE TRIGGER reject_activation BEFORE UPDATE ON usuario FOR EACH ROW EXECUTE FUNCTION reject_activation();`); err != nil {
		t.Fatal(err)
	}
	if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.99"); err == nil {
		t.Fatal("activation failure hidden")
	}
	token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(d.Token))
	if err != nil || token.ConsumedAt != nil {
		t.Fatal("activation failure consumed token outside transaction")
	}
	if _, err := h.pool.Exec(h.ctx, "DROP TRIGGER reject_activation ON usuario; DROP FUNCTION reject_activation()"); err != nil {
		t.Fatal(err)
	}
	h.verify(t)
	a, err := h.repo.AccountByID(h.ctx, id)
	if err != nil || a.State != identity.AccountActive {
		t.Fatal("retry did not activate")
	}
	h2 := newAuthHarness(t)
	h2.mail.fail = true
	pending, err := h2.service.Register(h2.ctx, identity.RegisterInput{Email: "delivery@ejemplo.invalid", Password: "Synthetic#123", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"})
	if pending == "" || !errors.Is(err, identity.ErrDelivery) {
		t.Fatal("delivery failure did not expose committed pending account")
	}
	h2.mail.fail = false
	if err := h2.service.ReissueVerification(h2.ctx, "delivery@ejemplo.invalid", "192.0.2.1"); err != nil {
		t.Fatal(err)
	}
	h2.verify(t)
}

func TestAuthEmissionWindowSuccessfulLoginResetAndRoleRevocation(t *testing.T) {
	h := newAuthHarness(t)
	id := h.register(t, "window@ejemplo.invalid")
	for i := 0; i < 2; i++ {
		if err := h.service.ReissueVerification(h.ctx, "window@ejemplo.invalid", "192.0.2.1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.service.ReissueVerification(h.ctx, "window@ejemplo.invalid", "192.0.2.1"); !errors.Is(err, identity.ErrRateLimited) {
		t.Fatal("fourth issuance allowed")
	}
	h.now = h.now.Add(time.Hour + time.Microsecond)
	if err := h.service.ReissueVerification(h.ctx, "window@ejemplo.invalid", "192.0.2.1"); err != nil {
		t.Fatal("rolling window did not expire")
	}
	h.verify(t)
	for i := 0; i < 2; i++ {
		if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "window@ejemplo.invalid", Password: "Wrong#123"}); !errors.Is(err, identity.ErrCredentials) {
			t.Fatal(err)
		}
	}
	result := h.login(t, "window@ejemplo.invalid")
	account, err := h.repo.AccountByID(h.ctx, id)
	if err != nil || account.FailedAttempts != 0 {
		t.Fatal("successful login did not reset consecutive failures")
	}
	if _, err := h.pool.Exec(h.ctx, "DELETE FROM rol_usuario WHERE usuario_id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.Authorize(h.ctx, result.Token, identity.RoleTenant, identity.UserOperation); !errors.Is(err, identity.ErrForbidden) {
		t.Fatal("session retained revoked role")
	}
}

func TestAuthLoginSessionFailureRollsBackAndAlertFailureRemainsVisible(t *testing.T) {
	h := newAuthHarness(t)
	id := h.register(t, "session-failure@ejemplo.invalid")
	h.verify(t)
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "session-failure@ejemplo.invalid", Password: "Wrong#123"}); !errors.Is(err, identity.ErrCredentials) {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `CREATE FUNCTION reject_session() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic session failure'; END; $$; CREATE TRIGGER reject_session BEFORE INSERT ON sesion FOR EACH ROW EXECUTE FUNCTION reject_session();`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "session-failure@ejemplo.invalid", Password: "Synthetic#123"}); err == nil {
		t.Fatal("session persistence failure hidden")
	}
	account, err := h.repo.AccountByID(h.ctx, id)
	if err != nil || account.FailedAttempts != 1 {
		t.Fatal("failed session creation reset login counter")
	}
	var sessions int
	if err := h.pool.QueryRow(h.ctx, "SELECT count(*) FROM sesion WHERE usuario_id=$1", id).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("failed login persisted a session")
	}
	h.mail.fail = true
	for i := 0; i < 4; i++ {
		_, err := h.service.Login(h.ctx, identity.LoginInput{Email: "session-failure@ejemplo.invalid", Password: "Wrong#123"})
		if i == 3 {
			var blocked *identity.LoginBlockedError
			if !errors.As(err, &blocked) || !errors.Is(err, identity.ErrDelivery) {
				t.Fatal("alert failure or committed block hidden")
			}
		}
	}
	account, err = h.repo.AccountByID(h.ctx, id)
	if err != nil || account.State != identity.AccountBlocked {
		t.Fatal("mail failure rolled back security block")
	}
}

func TestAuthVerificationIPLimitDoesNotIncrementTokenFailures(t *testing.T) {
	h := newAuthHarness(t)
	id := h.register(t, "ip-verify@ejemplo.invalid")
	d := h.mail.last()
	h.limiter.max = 1
	if err := h.service.VerifyEmail(h.ctx, d.TokenID, "incorrect-token", "192.0.2.20"); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatal(err)
	}
	if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.20"); !errors.Is(err, identity.ErrRateLimited) {
		t.Fatal("verification IP gate bypassed")
	}
	token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(d.Token))
	if err != nil || token.Attempts != 1 || token.ConsumedAt != nil {
		t.Fatal("IP rejection modified token")
	}
	if err := h.service.VerifyEmail(h.ctx, d.TokenID, d.Token, "192.0.2.21"); err != nil {
		t.Fatal(err)
	}
	a, err := h.repo.AccountByID(h.ctx, id)
	if err != nil || a.FailedAttempts != 0 || a.BlockedUntil != nil {
		t.Fatal("IP rejection blocked login")
	}
	h.login(t, "ip-verify@ejemplo.invalid")
}

func TestAuthEmailVerificationPreservesExistingLoginBlock(t *testing.T) {
	h := newAuthHarness(t)
	id := h.register(t, "blocked-verification@ejemplo.invalid")
	for i := 0; i < 5; i++ {
		_, _ = h.service.Login(h.ctx, identity.LoginInput{Email: "blocked-verification@ejemplo.invalid", Password: "Wrong#123"})
	}
	h.verify(t)
	a, err := h.repo.AccountByID(h.ctx, id)
	if err != nil || a.State != identity.AccountBlocked || a.FailedAttempts != 5 || a.BlockedUntil == nil || !a.BlockedUntil.Equal(h.now.Add(30*time.Minute)) {
		t.Fatal("verification shortened the login block")
	}
	_, err = h.service.Login(h.ctx, identity.LoginInput{Email: "blocked-verification@ejemplo.invalid", Password: "Synthetic#123"})
	var blocked *identity.LoginBlockedError
	if !errors.As(err, &blocked) {
		t.Fatal("verified account bypassed the existing block")
	}
	h.now = h.now.Add(30 * time.Minute)
	h.login(t, "blocked-verification@ejemplo.invalid")
}

func TestAuthRecoveryAndPasswordChangeRevokeSessions(t *testing.T) {
	h := newAuthHarness(t)
	h.register(t, "credentials@ejemplo.invalid")
	h.verify(t)
	first := h.login(t, "credentials@ejemplo.invalid")
	if err := h.service.RequestPasswordRecovery(h.ctx, "unknown@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatalf("unknown recovery leaked account existence: %v", err)
	}
	if err := h.service.RequestPasswordRecovery(h.ctx, "credentials@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatal(err)
	}
	h.mail.mu.Lock()
	recovery := h.mail.recoveries[len(h.mail.recoveries)-1]
	h.mail.mu.Unlock()
	if err := h.service.RequestPasswordRecovery(h.ctx, "credentials@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatal(err)
	}
	h.mail.mu.Lock()
	previousRecovery := recovery
	recovery = h.mail.recoveries[len(h.mail.recoveries)-1]
	h.mail.mu.Unlock()
	if err := h.service.ResetPassword(h.ctx, identity.ResetPasswordInput{TokenID: previousRecovery.TokenID, Token: previousRecovery.Token, Password: "Changed#234", Confirmation: "Changed#234", ClientIP: "192.0.2.6"}); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatal("replaced recovery token accepted")
	}
	if err := h.service.RequestPasswordRecovery(h.ctx, "credentials@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatal(err)
	}
	h.mail.mu.Lock()
	recovery = h.mail.recoveries[len(h.mail.recoveries)-1]
	h.mail.mu.Unlock()
	if err := h.service.RequestPasswordRecovery(h.ctx, "credentials@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatal("account emission cap disclosed through error")
	}
	h.mail.mu.Lock()
	issued := len(h.mail.recoveries)
	h.mail.mu.Unlock()
	if issued != 3 {
		t.Fatalf("expected 3 account recovery messages, got %d", issued)
	}
	token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(recovery.Token))
	if err != nil || token.Purpose != "recuperar_clave" || token.Hash == string(recovery.Token) || token.ExpiresAt.Sub(token.CreatedAt) != 15*time.Minute {
		t.Fatal("recovery token persistence/TTL/hash invalid")
	}
	if err := h.service.ResetPassword(h.ctx, identity.ResetPasswordInput{TokenID: recovery.TokenID, Token: recovery.Token, Password: "Changed#234", Confirmation: "Changed#234", ClientIP: "192.0.2.6"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.Authorize(h.ctx, first.Token, "", identity.AutomaticPolling); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatal("recovery did not revoke active session")
	}
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "credentials@ejemplo.invalid", Password: "Synthetic#123"}); !errors.Is(err, identity.ErrCredentials) {
		t.Fatal("old credential remained valid")
	}
	second, err := h.service.Login(h.ctx, identity.LoginInput{Email: "credentials@ejemplo.invalid", Password: "Changed#234"})
	if err != nil {
		t.Fatalf("recovered credential rejected: %v", err)
	}
	if err := h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(second.Token), CurrentPassword: "Changed#234", Password: "Changed#345", Confirmation: "Changed#345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.Authorize(h.ctx, second.Token, "", identity.AutomaticPolling); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatal("change did not revoke active session")
	}
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "credentials@ejemplo.invalid", Password: "Changed#234"}); !errors.Is(err, identity.ErrCredentials) {
		t.Fatal("pre-change credential remained valid")
	}
	if _, err := h.service.Login(h.ctx, identity.LoginInput{Email: "credentials@ejemplo.invalid", Password: "Changed#345"}); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	h.mail.mu.Lock()
	changes := h.mail.changes
	h.mail.mu.Unlock()
	if changes != 2 {
		t.Fatalf("expected reset and change notifications, got %d", changes)
	}
}

func TestAuthRecoveryTokenStopsAfterFiveFailuresWithoutBlockingLogin(t *testing.T) {
	h := newAuthHarness(t)
	h.register(t, "recovery-attempts@ejemplo.invalid")
	h.verify(t)
	if err := h.service.RequestPasswordRecovery(h.ctx, "recovery-attempts@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatal(err)
	}
	h.mail.mu.Lock()
	recovery := h.mail.recoveries[len(h.mail.recoveries)-1]
	h.mail.mu.Unlock()
	input := identity.ResetPasswordInput{TokenID: recovery.TokenID, Password: "Recovered#234", Confirmation: "Recovered#234", ClientIP: "192.0.2.6"}
	for i := 0; i < 5; i++ {
		input.Token = "incorrect"
		if err := h.service.ResetPassword(h.ctx, input); !errors.Is(err, identity.ErrTokenInvalid) {
			t.Fatalf("incorrect recovery token %d accepted: %v", i+1, err)
		}
	}
	input.Token = recovery.Token
	if err := h.service.ResetPassword(h.ctx, input); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatal("recovery token accepted after five failures")
	}
	token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(recovery.Token))
	if err != nil || token.Attempts != 5 {
		t.Fatal("recovery failure count was not persisted")
	}
	account, err := h.repo.AccountByNormalizedEmail(h.ctx, "recovery-attempts@ejemplo.invalid")
	if err != nil || account.FailedAttempts != 0 || account.BlockedUntil != nil {
		t.Fatal("recovery failures affected login lock")
	}
}
