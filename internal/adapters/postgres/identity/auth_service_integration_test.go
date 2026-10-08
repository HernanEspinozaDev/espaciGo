package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	disputepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/dispute"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	disputedomain "github.com/HernanEspinozaDev/espaciGo/internal/dispute"
	disputehttp "github.com/HernanEspinozaDev/espaciGo/internal/dispute/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
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
func (m *authTestMail) lastRecovery() identity.RecoveryDelivery {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recoveries[len(m.recoveries)-1]
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

func newAuthHarnessWithAtomicClock(t *testing.T) (*authHarness, *atomic.Int64) {
	t.Helper()
	ctx, pool := newIdentityTestPool(t)
	clock := &atomic.Int64{}
	initial := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	clock.Store(initial.UnixNano())
	h := &authHarness{ctx: ctx, pool: pool, repo: identitypg.NewIdentityRepository(pool), mail: &authTestMail{}, limiter: &authTestIPLimiter{max: 100}, now: initial}
	service, err := identity.NewAuthenticationService(h.repo, authTestHasher{}, h.mail, h.limiter, &authTestGenerator{}, func() time.Time {
		return time.Unix(0, clock.Load()).UTC()
	})
	if err != nil {
		t.Fatal(err)
	}
	h.service = service
	return h, clock
}

func newRuntimeIdentityRepository(t *testing.T, h *authHarness) *identitypg.IdentityRepository {
	return identitypg.NewIdentityRepository(newRuntimePool(t, h))
}

func newRuntimePool(t *testing.T, h *authHarness) *pgxpool.Pool {
	t.Helper()
	if _, err := h.pool.Exec(h.ctx, `DO $$ BEGIN
IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN
    CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
END IF;
END $$`); err != nil {
		t.Fatal(err)
	}
	conn, err := h.pool.Acquire(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := dbbootstrap.GrantRuntimePermissions(h.ctx, conn.Conn()); err != nil {
		conn.Release()
		t.Fatal(err)
	}
	conn.Release()
	config := h.pool.Config().Copy()
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	runtimePool, err := pgxpool.NewWithConfig(h.ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimePool.Close)
	return runtimePool
}

func holdAccountRowLock(t *testing.T, h *authHarness, accountID string) func() {
	t.Helper()
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(h.ctx, `SELECT id FROM public.usuario WHERE id=$1 FOR UPDATE`, accountID); err != nil {
		_ = tx.Rollback(h.ctx)
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := tx.Commit(h.ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func waitForAccountLockWait(t *testing.T, h *authHarness) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var waiting int
	for time.Now().Before(deadline) {
		err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query ILIKE '%FOR UPDATE%'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("operation did not wait for the held account row lock")
}

func waitForAccountLockWaitCount(t *testing.T, h *authHarness, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var waiting int
	for time.Now().Before(deadline) {
		err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query ILIKE '%FOR UPDATE%'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("only %d operations waited on the account lock, want %d", waiting, want)
}
func (h *authHarness) register(t *testing.T, email string) string {
	t.Helper()
	id, err := h.service.Register(h.ctx, identity.RegisterInput{Email: email, Password: "Synthetic#123", UsePreference: "arrendar", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"})
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
	if _, err := h.service.Register(h.ctx, identity.RegisterInput{Email: "preference@ejemplo.invalid", Password: "Synthetic#123", UsePreference: "administrador", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"}); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("invalid onboarding preference accepted")
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
	if _, err := h.service.Register(h.ctx, identity.RegisterInput{Email: "STRASSE+alias@example.invalid", Password: "Synthetic#123", UsePreference: "ofrecer", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"}); !errors.Is(err, identity.ErrEmailRegistered) {
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

func TestRegistrationAPIPersistsNonExclusiveOnboardingPreference(t *testing.T) {
	h := newAuthHarness(t)
	api := identityhttp.NewHandler(h.service, h.repo, nil)
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		req.RemoteAddr = "192.0.2.40:8080"
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	missing := request(`{"email":"preference-missing@ejemplo.invalid","password":"Synthetic#123","terms_version_ids":["00000000-0000-4000-8000-000000000001"]}`)
	if missing.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing preference status=%d body=%s", missing.Code, missing.Body.String())
	}
	created := request(`{"email":"preference-api@ejemplo.invalid","password":"Synthetic#123","use_preference":"ofrecer","terms_version_ids":["00000000-0000-4000-8000-000000000001"]}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", created.Code, created.Body.String())
	}
	var payload struct {
		AccountID     string `json:"account_id"`
		UsePreference string `json:"use_preference"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	account, err := h.repo.AccountByID(h.ctx, payload.AccountID)
	if err != nil || payload.UsePreference != "ofrecer" || account.UsePreference != "ofrecer" {
		t.Fatalf("API preference response=%q stored=%q err=%v", payload.UsePreference, account.UsePreference, err)
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

func TestM02AuthenticatedOperationsRenewActivityButPollingAndAbsoluteExpiryDoNot(t *testing.T) {
	h := newAuthHarness(t)
	h.register(t, "m02-activity@ejemplo.invalid")
	h.verify(t)
	login := h.login(t, "m02-activity@ejemplo.invalid")
	created := h.now
	profiles, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	api := identityhttp.NewHandler(h.service, h.repo, nil, profiles)
	call := func(method, path, body string, expected int) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "192.0.2.10:8080"
		req.Header.Set("Authorization", "Bearer "+string(login.Token))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		if response.Code != expected {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.Code, expected, response.Body.String())
		}
	}
	assertSession := func(wantActivity time.Time) {
		t.Helper()
		session, err := h.repo.SessionByTokenHash(h.ctx, identity.CredentialHash(login.Token))
		if err != nil {
			t.Fatal(err)
		}
		if !session.LastActivityAt.Equal(wantActivity) {
			t.Fatalf("last activity=%s want=%s", session.LastActivityAt, wantActivity)
		}
		if !session.ExpiresAt.Equal(created.Add(8 * time.Hour)) {
			t.Fatalf("absolute expiry moved: got=%s want=%s", session.ExpiresAt, created.Add(8*time.Hour))
		}
	}

	h.now = created.Add(20 * time.Minute)
	call(http.MethodPut, "/api/v1/profile", `{"display_name":"Synthetic Profile"}`, http.StatusOK)
	assertSession(h.now)

	h.now = created.Add(40 * time.Minute)
	call(http.MethodPost, "/api/v1/rights-requests", `{"type":"acceso"}`, http.StatusAccepted)
	assertSession(h.now)

	h.now = created.Add(50 * time.Minute)
	call(http.MethodGet, "/api/v1/auth/session", "", http.StatusOK)
	assertSession(created.Add(40 * time.Minute))

	h.now = created.Add(60 * time.Minute)
	call(http.MethodGet, "/api/v1/profile", "", http.StatusOK)
	assertSession(h.now)

	for elapsed := 80 * time.Minute; elapsed < 8*time.Hour; elapsed += 20 * time.Minute {
		h.now = created.Add(elapsed)
		call(http.MethodPut, "/api/v1/profile", `{"display_name":"Synthetic Profile"}`, http.StatusOK)
		assertSession(h.now)
	}
	h.now = created.Add(7*time.Hour + 59*time.Minute)
	call(http.MethodPost, "/api/v1/rights-requests", `{"type":"supresion"}`, http.StatusAccepted)
	assertSession(h.now)

	h.now = created.Add(8 * time.Hour)
	call(http.MethodPut, "/api/v1/profile", `{"display_name":"Synthetic Profile"}`, http.StatusUnauthorized)
	assertSession(created.Add(7*time.Hour + 59*time.Minute))
}

func TestM02IdentityExportUsesAuthenticatedAccountAndExcludesCredentials(t *testing.T) {
	h := newAuthHarness(t)
	h.register(t, "export-owner@ejemplo.invalid")
	h.verify(t)
	h.register(t, "export-other@ejemplo.invalid")
	h.verify(t)
	owner := h.login(t, "export-owner@ejemplo.invalid")
	other := h.login(t, "export-other@ejemplo.invalid")
	service, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	api := identityhttp.NewHandler(h.service, h.repo, nil, service)
	call := func(token identity.Secret) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/export", nil)
		req.RemoteAddr = "192.0.2.10:8080"
		req.Header.Set("Authorization", "Bearer "+string(token))
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	ownerResponse := call(owner.Token)
	if ownerResponse.Code != http.StatusOK {
		t.Fatalf("owner export status=%d body=%s", ownerResponse.Code, ownerResponse.Body.String())
	}
	var exported privacy.OwnData
	if err := json.Unmarshal(ownerResponse.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Account.Email != "export-owner@ejemplo.invalid" || exported.Scope != "identidad_local_v1" || exported.Account.Email == "export-other@ejemplo.invalid" {
		t.Fatalf("export is not scoped to authenticated account: %+v", exported.Account)
	}
	for _, forbidden := range []string{"password_hash", "token_hash", "access_token", "session"} {
		if strings.Contains(ownerResponse.Body.String(), forbidden) {
			t.Fatalf("export response contains forbidden credential field %q", forbidden)
		}
	}
	otherResponse := call(other.Token)
	if otherResponse.Code != http.StatusOK || !strings.Contains(otherResponse.Body.String(), "export-other@ejemplo.invalid") || strings.Contains(otherResponse.Body.String(), "export-owner@ejemplo.invalid") {
		t.Fatalf("second account export leaked another account: status=%d body=%s", otherResponse.Code, otherResponse.Body.String())
	}
}

func TestM02SuppressionReviewRequiresAdminAndReusesPersistedIncompleteAssessment(t *testing.T) {
	h := newAuthHarness(t)
	requesterID := h.register(t, "suppression-requester@ejemplo.invalid")
	h.verify(t)
	requester := h.login(t, "suppression-requester@ejemplo.invalid")
	privacyRepo := newRuntimeIdentityRepository(t, h)
	privacyService, err := privacy.NewService(privacyRepo)
	if err != nil {
		t.Fatal(err)
	}
	request, err := privacyService.RequestRight(h.ctx, requesterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}

	adminID := h.register(t, "suppression-reviewer@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES ($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	admin := h.login(t, "suppression-reviewer@ejemplo.invalid")
	api := identityhttp.NewHandler(h.service, h.repo, nil, privacyService)
	queueCall := func(token identity.Secret) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/suppression-requests", nil)
		req.RemoteAddr = "192.0.2.10:8080"
		req.Header.Set("Authorization", "Bearer "+string(token))
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	call := func(token identity.Secret, key string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/privacy/suppression-requests/"+request.ID+"/review", nil)
		req.RemoteAddr = "192.0.2.10:8080"
		req.Header.Set("Authorization", "Bearer "+string(token))
		req.Header.Set("Idempotency-Key", key)
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	if denied := call(requester.Token, "requester-must-not-review"); denied.Code != http.StatusForbidden {
		t.Fatalf("requester review status=%d body=%s, want 403", denied.Code, denied.Body.String())
	}
	if denied := queueCall(requester.Token); denied.Code != http.StatusForbidden {
		t.Fatalf("requester queue status=%d body=%s, want 403", denied.Code, denied.Body.String())
	}
	queue := queueCall(admin.Token)
	if queue.Code != http.StatusOK || !strings.Contains(queue.Body.String(), request.ID) || strings.Contains(queue.Body.String(), "suppression-requester@ejemplo.invalid") {
		t.Fatalf("admin queue status=%d body=%s", queue.Code, queue.Body.String())
	}
	first := call(admin.Token, "assessment-1")
	if first.Code != http.StatusOK {
		t.Fatalf("admin assessment status=%d body=%s", first.Code, first.Body.String())
	}
	var firstResult privacy.SuppressionReview
	if err := json.Unmarshal(first.Body.Bytes(), &firstResult); err != nil {
		t.Fatal(err)
	}
	if firstResult.Outcome != "revision_incompleta" || len(firstResult.Obligations) != 0 || !reflect.DeepEqual(firstResult.PendingChecks, []string{"matriz_retencion_historicos_incompleta"}) || firstResult.Reused {
		t.Fatalf("unexpected first assessment: %+v", firstResult)
	}
	second := call(admin.Token, "assessment-1")
	var secondResult privacy.SuppressionReview
	if err := json.Unmarshal(second.Body.Bytes(), &secondResult); err != nil {
		t.Fatal(err)
	}
	if second.Code != http.StatusOK || !secondResult.Reused || !secondResult.ReviewedAt.Equal(firstResult.ReviewedAt) || secondResult.Outcome != firstResult.Outcome {
		t.Fatalf("idempotent replay status=%d result=%+v first=%+v", second.Code, secondResult, firstResult)
	}
	var auditRows int
	var auditAt, auditExpires time.Time
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*),min(ocurrido_en),min(retirar_en) FROM public.evento_auditoria_local WHERE recurso_id=$1 AND accion='privacy.suppression.review' AND clave_idempotencia='assessment-1'`, request.ID).Scan(&auditRows, &auditAt, &auditExpires); err != nil {
		t.Fatal(err)
	}
	if auditRows != 1 {
		t.Fatalf("review replay duplicated audit: rows=%d", auditRows)
	}
	if !auditAt.Equal(firstResult.ReviewedAt) || !auditExpires.Equal(identity.AddCalendarMonthsUTC(firstResult.ReviewedAt, 60)) {
		t.Fatalf("audit retention does not start at review event: at=%s remove=%s", auditAt, auditExpires)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.evento_auditoria_local (
		id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,
		ocurrido_en,retirar_en,clave_idempotencia,detalle_codigos
	) VALUES ('80000000-0000-4000-8000-000000000001',$1,'solicitud_titular',$2,
		'privacy.suppression.review','exito','supresion_revision_incompleta','invalid-payload',
		$3,$4,'invalid-payload','{"obligations_detected":[],"pending_checks":[],"email":"private@example.invalid"}'::jsonb)`, adminID, request.ID, firstResult.ReviewedAt, identity.AddCalendarMonthsUTC(firstResult.ReviewedAt, 60)); err == nil {
		t.Fatal("audit allowed an unclassified free-text field")
	}
	requests, err := privacyService.OwnRequests(h.ctx, requesterID)
	if err != nil || len(requests) != 1 || requests[0].State != "en_revision" {
		t.Fatalf("assessment must keep the request open for access rights: requests=%+v err=%v", requests, err)
	}
	export, err := privacyService.ExportOwnData(h.ctx, requesterID)
	if err != nil || export.Account.Email != "suppression-requester@ejemplo.invalid" {
		t.Fatalf("suppression review blocked another data right: export=%+v err=%v", export.Account, err)
	}
}

func TestLocalDisputeBlocksBothParticipantsUntilAdministratorCloses(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "dispute-host@ejemplo.invalid")
	h.verify(t)
	host := h.login(t, "dispute-host@ejemplo.invalid")
	renterID := h.register(t, "dispute-renter@ejemplo.invalid")
	h.verify(t)
	renter := h.login(t, "dispute-renter@ejemplo.invalid")
	otherHostID := h.register(t, "dispute-outsider@ejemplo.invalid")
	h.verify(t)
	outsider := h.login(t, "dispute-outsider@ejemplo.invalid")
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.espacio (
		id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
		reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria
	) VALUES ('72000000-0000-4000-0000-000000000009',$1,'oficina','Other host space',repeat('Synthetic description ',6),20,2,'Synthetic rules','hora',12000,'Synthetic address','borrador','UTC')`, otherHostID); err != nil {
		t.Fatal(err)
	}
	adminID := h.register(t, "dispute-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES ($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	admin := h.login(t, "dispute-admin@ejemplo.invalid")
	const reservationID = "72000000-0000-4000-8000-000000000003"
	const occupancyID = "72000000-0000-4000-8000-000000000004"
	seedPrivacyReviewReservation(t, h, "72000000-0000-4000-8000-000000000001", "72000000-0000-4000-8000-000000000002", reservationID, occupancyID, hostID, renterID, "cancelada_arrendatario")
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE id=$1`, occupancyID, h.now); err != nil {
		t.Fatal(err)
	}
	runtimePool := newRuntimePool(t, h)
	disputeService, err := disputedomain.NewService(disputepg.New(runtimePool), func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	privacyService, err := privacy.NewService(identitypg.NewIdentityRepository(runtimePool))
	if err != nil {
		t.Fatal(err)
	}
	api := disputehttp.NewHandler(h.service, disputeService, nil)
	call := func(method, path string, token identity.Secret, key, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "192.0.2.10:8080"
		req.Header.Set("Authorization", "Bearer "+string(token))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	openPath := "/api/v1/local/booking-trial/reservations/" + reservationID + "/disputes"
	first := call(http.MethodPost, openPath, host.Token, "dispute-open-1", `{"reason_code":"ensayo_privacidad"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("open status=%d body=%s", first.Code, first.Body.String())
	}
	var opened disputedomain.Dispute
	if err := json.Unmarshal(first.Body.Bytes(), &opened); err != nil {
		t.Fatal(err)
	}
	replay := call(http.MethodPost, openPath, host.Token, "dispute-open-1", `{"reason_code":"ensayo_privacidad"}`)
	var replayed disputedomain.Dispute
	if err := json.Unmarshal(replay.Body.Bytes(), &replayed); err != nil {
		t.Fatal(err)
	}
	if replay.Code != http.StatusOK || !replayed.Reused || replayed.ID != opened.ID {
		t.Fatalf("idempotent replay status=%d result=%+v initial=%+v", replay.Code, replayed, opened)
	}
	if _, err := runtimePool.Exec(h.ctx, `UPDATE public.disputa_ensayo_historial SET motivo_codigo='duplicada' WHERE disputa_id=$1`, opened.ID); err == nil {
		t.Fatal("runtime role unexpectedly modified dispute history")
	}
	if _, err := runtimePool.Exec(h.ctx, `DELETE FROM public.disputa_ensayo_historial WHERE disputa_id=$1`, opened.ID); err == nil {
		t.Fatal("runtime role unexpectedly deleted dispute history")
	}
	if _, err := runtimePool.Exec(h.ctx, `UPDATE public.disputa_ensayo_local SET abierta_por=$2 WHERE id=$1`, opened.ID, renterID); err == nil {
		t.Fatal("runtime role unexpectedly modified dispute opening actor")
	}
	if duplicate := call(http.MethodPost, openPath, host.Token, "dispute-open-2", `{"reason_code":"ensayo_privacidad"}`); duplicate.Code != http.StatusConflict {
		t.Fatalf("second open dispute status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	if wrongReason := call(http.MethodPost, openPath, host.Token, "dispute-open-bad", `{"reason_code":"arriendo_en_curso"}`); wrongReason.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unsupported opening reason status=%d body=%s", wrongReason.Code, wrongReason.Body.String())
	} else {
		assertCommonHTTPError(t, wrongReason, "invalid_request")
	}
	if renterOpen := call(http.MethodPost, openPath, renter.Token, "renter-open", `{"reason_code":"ensayo_privacidad"}`); renterOpen.Code != http.StatusNotFound {
		t.Fatalf("non-host open status=%d body=%s", renterOpen.Code, renterOpen.Body.String())
	}
	if otherHostOpen := call(http.MethodPost, openPath, outsider.Token, "other-host-open", `{"reason_code":"ensayo_privacidad"}`); otherHostOpen.Code != http.StatusNotFound {
		t.Fatalf("host of another space open status=%d body=%s", otherHostOpen.Code, otherHostOpen.Body.String())
	}
	if participantView := call(http.MethodGet, openPath, renter.Token, "", ""); participantView.Code != http.StatusOK || !strings.Contains(participantView.Body.String(), opened.ID) {
		t.Fatalf("renter view status=%d body=%s", participantView.Code, participantView.Body.String())
	}
	if outsiderView := call(http.MethodGet, openPath, outsider.Token, "", ""); outsiderView.Code != http.StatusNotFound {
		t.Fatalf("outsider view status=%d body=%s", outsiderView.Code, outsiderView.Body.String())
	} else {
		assertCommonHTTPError(t, outsiderView, "not_found")
	}
	adminQueue := call(http.MethodGet, "/api/v1/admin/disputes", admin.Token, "", "")
	if adminQueue.Code != http.StatusOK || !strings.Contains(adminQueue.Body.String(), opened.ID) {
		t.Fatalf("admin dispute queue status=%d body=%s", adminQueue.Code, adminQueue.Body.String())
	}
	if renterQueue := call(http.MethodGet, "/api/v1/admin/disputes", renter.Token, "", ""); renterQueue.Code != http.StatusForbidden {
		t.Fatalf("participant read admin queue status=%d body=%s", renterQueue.Code, renterQueue.Body.String())
	}
	closePath := "/api/v1/admin/disputes/" + opened.ID + "/close"
	if renterClose := call(http.MethodPost, closePath, renter.Token, "", `{"reason_code":"ensayo_finalizado"}`); renterClose.Code != http.StatusForbidden {
		t.Fatalf("participant close status=%d body=%s", renterClose.Code, renterClose.Body.String())
	} else {
		assertCommonHTTPError(t, renterClose, "forbidden")
	}
	if badClose := call(http.MethodPost, closePath, admin.Token, "", `{"reason_code":"resuelta_a_favor_del_host"}`); badClose.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid close reason status=%d body=%s", badClose.Code, badClose.Body.String())
	}

	hostRequest, err := privacyService.RequestRight(h.ctx, hostID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	renterRequest, err := privacyService.RequestRight(h.ctx, renterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []privacy.RightsRequest{hostRequest, renterRequest} {
		review, err := privacyService.ReviewSuppression(h.ctx, adminID, request.ID, "dispute-open-"+request.ID, "dispute-test", func() time.Time { return h.now })
		if err != nil {
			t.Fatal(err)
		}
		if review.Outcome != "bloqueada" || !reflect.DeepEqual(review.Obligations, []string{"disputa_abierta"}) || !reflect.DeepEqual(review.PendingChecks, []string{"matriz_retencion_historicos_incompleta"}) {
			t.Fatalf("open dispute did not block participant=%s: %+v", request.ID, review)
		}
	}

	closedResponse := call(http.MethodPost, closePath, admin.Token, "", `{"reason_code":"ensayo_finalizado"}`)
	var closed disputedomain.Dispute
	if closedResponse.Code != http.StatusOK || json.Unmarshal(closedResponse.Body.Bytes(), &closed) != nil || closed.State != "cerrada" || closed.CloseReason == nil || *closed.CloseReason != "ensayo_finalizado" {
		t.Fatalf("close status=%d body=%s result=%+v", closedResponse.Code, closedResponse.Body.String(), closed)
	}
	if repeatedClose := call(http.MethodPost, closePath, admin.Token, "", `{"reason_code":"duplicada"}`); repeatedClose.Code != http.StatusConflict {
		t.Fatalf("closed dispute was reopened status=%d body=%s", repeatedClose.Code, repeatedClose.Body.String())
	} else {
		assertCommonHTTPError(t, repeatedClose, "conflict")
	}
	closedReplay := call(http.MethodPost, openPath, host.Token, "dispute-open-1", `{"reason_code":"ensayo_privacidad"}`)
	var replayedClosed disputedomain.Dispute
	if closedReplay.Code != http.StatusOK || json.Unmarshal(closedReplay.Body.Bytes(), &replayedClosed) != nil || !replayedClosed.Reused || replayedClosed.State != "cerrada" {
		t.Fatalf("closed opening replay changed state status=%d body=%s result=%+v", closedReplay.Code, closedReplay.Body.String(), replayedClosed)
	}
	historyPath := "/api/v1/local/booking-trial/disputes/" + opened.ID + "/history"
	historyResponse := call(http.MethodGet, historyPath, host.Token, "", "")
	var history struct {
		Items []disputedomain.Transition `json:"items"`
	}
	if historyResponse.Code != http.StatusOK || json.Unmarshal(historyResponse.Body.Bytes(), &history) != nil || len(history.Items) != 2 || history.Items[0].NewState != "abierta" || history.Items[1].NewState != "cerrada" || history.Items[0].Sequence >= history.Items[1].Sequence {
		t.Fatalf("history status=%d body=%s parsed=%+v", historyResponse.Code, historyResponse.Body.String(), history)
	}
	for _, request := range []privacy.RightsRequest{hostRequest, renterRequest} {
		review, err := privacyService.ReviewSuppression(h.ctx, adminID, request.ID, "dispute-closed-"+request.ID, "dispute-test", func() time.Time { return h.now })
		if err != nil {
			t.Fatal(err)
		}
		if review.Outcome != "revision_incompleta" || len(review.Obligations) != 0 || !reflect.DeepEqual(review.PendingChecks, []string{"matriz_retencion_historicos_incompleta"}) {
			t.Fatalf("closed dispute remained a blocker participant=%s: %+v", request.ID, review)
		}
	}
	var bookingState string
	var activeOccupancy bool
	if err := h.pool.QueryRow(h.ctx, `SELECT r.estado,o.activo FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.id=r.ocupacion_id WHERE r.id=$1`, reservationID).Scan(&bookingState, &activeOccupancy); err != nil {
		t.Fatal(err)
	}
	if bookingState != "cancelada_arrendatario" || activeOccupancy {
		t.Fatalf("local dispute mutated booking/occupancy: state=%s active=%v", bookingState, activeOccupancy)
	}
}

func assertCommonHTTPError(t *testing.T, response *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()
	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("error response is not JSON: %v; body=%s", err, response.Body.String())
	}
	id := payload.Error.RequestID
	if payload.Error.Code != expectedCode || strings.TrimSpace(payload.Error.Message) == "" || len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' || response.Header().Get("X-Request-ID") != id {
		t.Fatalf("error response does not satisfy common OpenAPI fields: status=%d payload=%+v header request id=%q", response.Code, payload.Error, response.Header().Get("X-Request-ID"))
	}
}

func TestLocalDisputeOpenAndCloseSerializeWithSuppressionReview(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "dispute-race-host@ejemplo.invalid")
	h.verify(t)
	renterID := h.register(t, "dispute-race-renter@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "dispute-race-admin@ejemplo.invalid")
	h.verify(t)
	const reservationID = "73000000-0000-4000-8000-000000000003"
	const occupancyID = "73000000-0000-4000-8000-000000000004"
	seedPrivacyReviewReservation(t, h, "73000000-0000-4000-8000-000000000001", "73000000-0000-4000-8000-000000000002", reservationID, occupancyID, hostID, renterID, "cancelada_arrendatario")
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE id=$1`, occupancyID, h.now); err != nil {
		t.Fatal(err)
	}
	runtimePool := newRuntimePool(t, h)
	disputes, err := disputedomain.NewService(disputepg.New(runtimePool), func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	privacyService, err := privacy.NewService(identitypg.NewIdentityRepository(runtimePool))
	if err != nil {
		t.Fatal(err)
	}
	request, err := privacyService.RequestRight(h.ctx, hostID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	lockedAccount := hostID
	if renterID < hostID {
		lockedAccount = renterID
	}
	release := holdAccountRowLock(t, h, lockedAccount)
	type openResult struct {
		item disputedomain.Dispute
		err  error
	}
	opened := make(chan openResult, 1)
	go func() {
		item, err := disputes.Open(h.ctx, hostID, reservationID, disputedomain.OpeningReason, "race-open")
		opened <- openResult{item: item, err: err}
	}()
	waitForAccountLockWait(t, h)
	type reviewResult struct {
		item privacy.SuppressionReview
		err  error
	}
	firstReview := make(chan reviewResult, 1)
	go func() {
		item, err := privacyService.ReviewSuppression(h.ctx, adminID, request.ID, "race-open-review", "race", func() time.Time { return h.now })
		firstReview <- reviewResult{item: item, err: err}
	}()
	waitForAccountLockWaitCount(t, h, 2)
	release()
	openOutcome, reviewOutcome := <-opened, <-firstReview
	if openOutcome.err != nil || reviewOutcome.err != nil || reviewOutcome.item.Outcome != "bloqueada" || !reflect.DeepEqual(reviewOutcome.item.Obligations, []string{"disputa_abierta"}) {
		t.Fatalf("open/review lock ordering lost blocker: open=%+v review=%+v", openOutcome, reviewOutcome)
	}
	var records, histories int
	if err := h.pool.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM public.disputa_ensayo_local WHERE reserva_id=$1),(SELECT count(*) FROM public.disputa_ensayo_historial WHERE disputa_id=$2)`, reservationID, openOutcome.item.ID).Scan(&records, &histories); err != nil {
		t.Fatal(err)
	}
	if records != 1 || histories != 1 {
		t.Fatalf("opening race left partial/duplicate state: disputes=%d history=%d", records, histories)
	}

	closeRequest := make(chan error, 1)
	release = holdAccountRowLock(t, h, lockedAccount)
	go func() {
		_, err := disputes.Close(h.ctx, adminID, openOutcome.item.ID, "ensayo_finalizado")
		closeRequest <- err
	}()
	waitForAccountLockWait(t, h)
	secondReview := make(chan reviewResult, 1)
	go func() {
		item, err := privacyService.ReviewSuppression(h.ctx, adminID, request.ID, "race-close-review", "race", func() time.Time { return h.now })
		secondReview <- reviewResult{item: item, err: err}
	}()
	waitForAccountLockWaitCount(t, h, 2)
	release()
	closeErr, reviewedAfterClose := <-closeRequest, <-secondReview
	if closeErr != nil || reviewedAfterClose.err != nil || reviewedAfterClose.item.Outcome != "revision_incompleta" || len(reviewedAfterClose.item.Obligations) != 0 {
		t.Fatalf("close/review lock ordering left stale blocker: close=%v review=%+v", closeErr, reviewedAfterClose)
	}
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.disputa_ensayo_historial WHERE disputa_id=$1`, openOutcome.item.ID).Scan(&histories); err != nil {
		t.Fatal(err)
	}
	if histories != 2 {
		t.Fatalf("close/review race produced wrong history count: %d", histories)
	}
}

func seedPrivacyReviewReservation(t *testing.T, h *authHarness, spaceID, quoteID, reservationID, occupancyID, hostID, renterID, state string) {
	t.Helper()
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(h.ctx) }()
	start := h.now.Add(48 * time.Hour)
	end := start.Add(24 * time.Hour)
	if _, err := tx.Exec(h.ctx, `INSERT INTO public.espacio (
		id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
		reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria
	) VALUES ($1,$2,'oficina','Synthetic',repeat('Synthetic description ',6),20,2,'Synthetic rules','hora',12000,'Synthetic address','borrador','UTC')`, spaceID, hostID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(h.ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp,moneda,creada_en) VALUES ($1,1,'hora',12000,'CLP',$2)`, spaceID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(h.ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores,actualizado_en) VALUES ($1,'oficina',1,'{}'::jsonb,$2)`, spaceID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(h.ctx, `INSERT INTO public.cotizacion_reserva_ensayo (
		id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,
		categoria_codigo,perfil_version,perfil_valores_snapshot,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en,condiciones_snapshot,politica_cancelacion_version
	) VALUES ($1,$2,$3,$4,1,'hora',12000,'oficina',1,'{}'::jsonb,'CLP',1,12000,$5,$6,'UTC',$7::timestamptz,$7::timestamptz + interval '15 minutes','synthetic use conditions','local_flexible_v1')`, quoteID, spaceID, hostID, renterID, start, end, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(h.ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,creada_en)
	VALUES ($1,$2,$3,tstzrange($4,$5,'[)'),'reserva',true,$6)`, occupancyID, spaceID, reservationID, start, end, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(h.ctx, `INSERT INTO public.reserva_ensayo_local (
		id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,
		estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,
		pago_vence_en,creada_en,actualizada_en,condiciones_snapshot,politica_cancelacion_version,ocupacion_id
	) VALUES ($1,$2,$3,$4,$5,'synthetic-key',decode(repeat('11',32),'hex'),$6,12000,1,12000,'hora','CLP',$7,$8,'UTC',$9,$10,$10,'synthetic use conditions','local_flexible_v1',$11)`, reservationID, quoteID, spaceID, hostID, renterID, state, start, end, h.now.Add(time.Hour), h.now, occupancyID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(h.ctx); err != nil {
		t.Fatal(err)
	}
}

func TestM02SuppressionReviewListsOnlyLiveReservationAndPaymentObligations(t *testing.T) {
	h := newAuthHarness(t)
	requesterID := h.register(t, "suppression-live@ejemplo.invalid")
	h.verify(t)
	otherID := h.register(t, "suppression-counterparty@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "suppression-observer@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES ($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	activeRequest, err := service.RequestRight(h.ctx, requesterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	seedPrivacyReviewReservation(t, h, "70000000-0000-4000-8000-000000000001", "70000000-0000-4000-8000-000000000002", "70000000-0000-4000-8000-000000000003", "70000000-0000-4000-8000-000000000004", requesterID, otherID, "aprobada_host")
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_pago_ensayo_operacion (
		id,reserva_id,arrendatario_id,clave_idempotencia,huella_solicitud,resultado_solicitado,estado,creada_en,actualizada_en
	) VALUES ('70000000-0000-4000-8000-000000000005','70000000-0000-4000-8000-000000000003',$1,'payment-pending',decode(repeat('22',32),'hex'),'exito','pendiente',$2,$2)`, otherID, h.now); err != nil {
		t.Fatal(err)
	}
	activeResult, err := h.repo.ReviewSuppression(h.ctx, adminID, activeRequest.ID, "live-obligations", "test-correlation-live", func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	if activeResult.Outcome != "bloqueada" || !reflect.DeepEqual(activeResult.Obligations, []string{"reserva_activa", "pago_o_devolucion_pendiente"}) {
		t.Fatalf("live obligations=%+v", activeResult)
	}

	terminalRequester := h.register(t, "suppression-historical@ejemplo.invalid")
	h.verify(t)
	terminalRequest, err := service.RequestRight(h.ctx, terminalRequester, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	seedPrivacyReviewReservation(t, h, "71000000-0000-4000-8000-000000000001", "71000000-0000-4000-8000-000000000002", "71000000-0000-4000-8000-000000000003", "71000000-0000-4000-8000-000000000004", terminalRequester, adminID, "cancelada_arrendatario")
	terminalResult, err := h.repo.ReviewSuppression(h.ctx, adminID, terminalRequest.ID, "terminal-history", "test-correlation-terminal", func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	if terminalResult.Outcome != "revision_incompleta" || len(terminalResult.Obligations) != 0 {
		t.Fatalf("terminal history alone should not be an obligation: %+v", terminalResult)
	}
}

func TestM02SuppressionReviewSamplesClockAfterAccountLock(t *testing.T) {
	h := newAuthHarness(t)
	requesterID := h.register(t, "suppression-clock@ejemplo.invalid")
	h.verify(t)
	reviewerID := h.register(t, "suppression-clock-reviewer@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES ($1,'administrador')`, reviewerID); err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	request, err := service.RequestRight(h.ctx, requesterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Int64{}
	clock.Store(h.now.UnixNano())
	resultCh := make(chan privacy.SuppressionReview, 1)
	errCh := make(chan error, 1)
	release := holdAccountRowLock(t, h, requesterID)
	go func() {
		result, reviewErr := h.repo.ReviewSuppression(h.ctx, reviewerID, request.ID, "clock-after-lock", "clock-correlation", func() time.Time {
			return time.Unix(0, clock.Load()).UTC()
		})
		resultCh <- result
		errCh <- reviewErr
	}()
	waitForAccountLockWait(t, h)
	advanced := h.now.Add(4 * time.Hour)
	clock.Store(advanced.UnixNano())
	release()
	result := <-resultCh
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if !result.ReviewedAt.Equal(advanced) {
		t.Fatalf("review timestamp=%s, want post-lock clock %s", result.ReviewedAt, advanced)
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
	pending, err := h2.service.Register(h2.ctx, identity.RegisterInput{Email: "delivery@ejemplo.invalid", Password: "Synthetic#123", UsePreference: "arrendar", TermsVersionIDs: []string{"00000000-0000-4000-8000-000000000001"}, Channel: "api", ClientIP: "192.0.2.1"})
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
	if err := h.service.DispatchOneCredentialNotice(h.ctx); err != nil {
		t.Fatalf("dispatch recovery notice: %v", err)
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
	if err := h.service.DispatchOneCredentialNotice(h.ctx); err != nil {
		t.Fatalf("dispatch password change notice: %v", err)
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

func TestLocalCredentialHistoryPreferenceAndDurableNoticeRecovery(t *testing.T) {
	h := newAuthHarness(t)
	accountID := h.register(t, "local-auth-history@ejemplo.invalid")
	account, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil || account.UsePreference != "arrendar" {
		t.Fatalf("stored onboarding preference=%q err=%v", account.UsePreference, err)
	}
	h.verify(t)
	first := h.login(t, "local-auth-history@ejemplo.invalid")
	start := h.now
	if err := h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(first.Token), CurrentPassword: "Synthetic#123", Password: "Changed#234", Confirmation: "Changed#234"}); err != nil {
		t.Fatalf("initial password change: %v", err)
	}
	second, err := h.service.Login(h.ctx, identity.LoginInput{Email: "local-auth-history@ejemplo.invalid", Password: "Changed#234"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(second.Token), CurrentPassword: "Changed#234", Password: "Synthetic#123", Confirmation: "Synthetic#123"}); !errors.Is(err, identity.ErrPasswordRecentlyUsed) {
		t.Fatalf("reused key within three calendar months err=%v", err)
	}
	var historyRows, outboxRows, auditRows int
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.historial_clave_local WHERE usuario_id=$1`, accountID).Scan(&historyRows); err != nil {
		t.Fatal(err)
	}
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id=$1`, accountID).Scan(&outboxRows); err != nil {
		t.Fatal(err)
	}
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.evento_auditoria_local WHERE recurso_id=$1`, accountID).Scan(&auditRows); err != nil {
		t.Fatal(err)
	}
	if historyRows != 1 || outboxRows != 1 || auditRows != 1 {
		t.Fatalf("history/outbox/audit rows=%d/%d/%d; rejected reuse must add no effects", historyRows, outboxRows, auditRows)
	}
	var storedHash string
	var expiresAt time.Time
	if err = h.pool.QueryRow(h.ctx, `SELECT hash_clave, retirar_en FROM public.historial_clave_local WHERE usuario_id=$1`, accountID).Scan(&storedHash, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if storedHash != string(identity.CredentialHash("Synthetic#123")) || !expiresAt.Equal(identity.AddCalendarMonthsUTC(start, 3)) {
		t.Fatalf("history hash or calendar expiration mismatch: expires=%s", expiresAt)
	}

	// Force one SMTP failure, restart the service object, advance the retry clock,
	// then verify the same durable intent is delivered once and completed.
	h.mail.mu.Lock()
	h.mail.fail = true
	h.mail.mu.Unlock()
	if err := h.service.DispatchOneCredentialNotice(h.ctx); !errors.Is(err, identity.ErrDelivery) {
		t.Fatalf("first dispatch err=%v; want retryable delivery error", err)
	}
	var attempts int
	var delivered *time.Time
	if err = h.pool.QueryRow(h.ctx, `SELECT intentos, entregada_en FROM public.outbox_evento_local WHERE agregado_id=$1`, accountID).Scan(&attempts, &delivered); err != nil || attempts != 1 || delivered != nil {
		t.Fatalf("failed delivery outbox attempts=%d delivered=%v err=%v", attempts, delivered, err)
	}
	h.now = h.now.Add(3 * time.Second)
	h.mail.mu.Lock()
	h.mail.fail = false
	h.mail.mu.Unlock()
	restarted, err := identity.NewAuthenticationService(h.repo, authTestHasher{}, h.mail, h.limiter, &authTestGenerator{}, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.DispatchOneCredentialNotice(h.ctx); err != nil {
		t.Fatalf("restarted dispatcher: %v", err)
	}
	h.mail.mu.Lock()
	changeNotices := h.mail.changes
	h.mail.mu.Unlock()
	if changeNotices != 1 {
		t.Fatalf("durable notice sends=%d; want exactly one after retry", changeNotices)
	}
	if err = h.pool.QueryRow(h.ctx, `SELECT intentos, entregada_en FROM public.outbox_evento_local WHERE agregado_id=$1`, accountID).Scan(&attempts, &delivered); err != nil || attempts != 2 || delivered == nil {
		t.Fatalf("completed outbox attempts=%d delivered=%v err=%v", attempts, delivered, err)
	}
	if err := restarted.DispatchOneCredentialNotice(h.ctx); err != nil {
		t.Fatal(err)
	}
	h.mail.mu.Lock()
	changeNotices = h.mail.changes
	h.mail.mu.Unlock()
	if changeNotices != 1 {
		t.Fatalf("completed event replay duplicated notice: sends=%d", changeNotices)
	}

	// At the exact expiry instant the previous key is no longer needed; the
	// cleanup and the next password update commit together.
	h.now = expiresAt
	third, err := h.service.Login(h.ctx, identity.LoginInput{Email: "local-auth-history@ejemplo.invalid", Password: "Changed#234"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(third.Token), CurrentPassword: "Changed#234", Password: "Synthetic#123", Confirmation: "Synthetic#123"}); err != nil {
		t.Fatalf("key reuse at inclusive three-month expiry: %v", err)
	}
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.historial_clave_local WHERE usuario_id=$1`, accountID).Scan(&historyRows); err != nil || historyRows != 1 {
		t.Fatalf("expired history was not deleted before new history insert; rows=%d err=%v", historyRows, err)
	}
}

func TestConcurrentPasswordChangesProduceOneHistoryAndOneDurableNotice(t *testing.T) {
	h := newAuthHarness(t)
	accountID := h.register(t, "local-auth-race@ejemplo.invalid")
	h.verify(t)
	login := h.login(t, "local-auth-race@ejemplo.invalid")
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, candidate := range []string{"First#2345", "Second#2345"} {
		candidate := candidate
		go func() {
			<-start
			results <- h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(login.Token), CurrentPassword: "Synthetic#123", Password: identity.Secret(candidate), Confirmation: identity.Secret(candidate)})
		}()
	}
	close(start)
	var successes, rejected int
	for range 2 {
		if err := <-results; err == nil {
			successes++
		} else if errors.Is(err, identity.ErrUnauthorized) {
			rejected++
		} else {
			t.Fatalf("concurrent password result: %v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("concurrent changes success/rejected=%d/%d", successes, rejected)
	}
	var history, notices, audits int
	for query, target := range map[string]*int{
		`SELECT count(*) FROM public.historial_clave_local WHERE usuario_id=$1`:  &history,
		`SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id=$1`:   &notices,
		`SELECT count(*) FROM public.evento_auditoria_local WHERE recurso_id=$1`: &audits,
	} {
		if err := h.pool.QueryRow(h.ctx, query, accountID).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if history != 1 || notices != 1 || audits != 1 {
		t.Fatalf("concurrent side effects history/outbox/audit=%d/%d/%d", history, notices, audits)
	}
}

func TestPasswordUpdateHistoryAuditAndOutboxCommitAtomically(t *testing.T) {
	h := newAuthHarness(t)
	accountID := h.register(t, "local-auth-atomic@ejemplo.invalid")
	h.verify(t)
	login := h.login(t, "local-auth-atomic@ejemplo.invalid")
	before, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `CREATE FUNCTION public.fail_test_audit_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'intentional disposable test failure'; END $$;
CREATE TRIGGER fail_test_audit_insert BEFORE INSERT ON public.evento_auditoria_local FOR EACH ROW EXECUTE FUNCTION public.fail_test_audit_insert()`); err != nil {
		t.Fatalf("install failure trigger on disposable audit table: %v", err)
	}
	err = h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(login.Token), CurrentPassword: "Synthetic#123", Password: "Changed#234", Confirmation: "Changed#234"})
	if err == nil {
		t.Fatal("password update unexpectedly committed despite audit insert failure")
	}
	after, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Fatal("password hash committed without audit/outbox transaction")
	}
	for table := range map[string]bool{"historial_clave_local": true, "outbox_evento_local": true, "evento_auditoria_local": true} {
		var count int
		query := "SELECT count(*) FROM public." + table + " WHERE "
		if table == "historial_clave_local" {
			query += "usuario_id=$1"
		} else if table == "outbox_evento_local" {
			query += "agregado_id=$1"
		} else {
			query += "recurso_id=$1"
		}
		if err := h.pool.QueryRow(h.ctx, query, accountID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s side effect count=%d err=%v; want rollback", table, count, err)
		}
	}
}

func TestPasswordChangeRevalidatesExpiredSessionAfterAccountLockWait(t *testing.T) {
	h, clock := newAuthHarnessWithAtomicClock(t)
	accountID := h.register(t, "local-auth-session-lock@ejemplo.invalid")
	h.verify(t)
	login := h.login(t, "local-auth-session-lock@ejemplo.invalid")
	before, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	release := holdAccountRowLock(t, h, accountID)
	finished := make(chan error, 1)
	go func() {
		finished <- h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(login.Token), CurrentPassword: "Synthetic#123", Password: "Changed#234", Confirmation: "Changed#234"})
	}()
	waitForAccountLockWait(t, h)
	expiredAt := h.now.Add(30 * time.Minute)
	clock.Store(expiredAt.UnixNano())
	release()
	if err := <-finished; !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("change with session expired while waiting err=%v", err)
	}
	after, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil || after.PasswordHash != before.PasswordHash {
		t.Fatalf("expired-session request changed password hash err=%v", err)
	}
	for table, column := range map[string]string{"historial_clave_local": "usuario_id", "outbox_evento_local": "agregado_id", "evento_auditoria_local": "recurso_id"} {
		var count int
		if err := h.pool.QueryRow(h.ctx, "SELECT count(*) FROM public."+table+" WHERE "+column+"=$1", accountID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("expired-session request left %s rows=%d err=%v", table, count, err)
		}
	}
	session, err := h.repo.SessionByTokenHash(h.ctx, identity.CredentialHash(login.Token))
	if err != nil || session.RevokedAt != nil {
		t.Fatalf("rejected request partially revoked session: revoked=%v err=%v", session.RevokedAt, err)
	}
}

func TestPasswordResetRevalidatesExpiredTokenAfterAccountLockWait(t *testing.T) {
	h, clock := newAuthHarnessWithAtomicClock(t)
	accountID := h.register(t, "local-auth-token-lock@ejemplo.invalid")
	h.verify(t)
	if err := h.service.RequestPasswordRecovery(h.ctx, "local-auth-token-lock@ejemplo.invalid", "192.0.2.5"); err != nil {
		t.Fatal(err)
	}
	recovery := h.mail.lastRecovery()
	before, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	release := holdAccountRowLock(t, h, accountID)
	finished := make(chan error, 1)
	go func() {
		finished <- h.service.ResetPassword(h.ctx, identity.ResetPasswordInput{TokenID: recovery.TokenID, Token: recovery.Token, Password: "Recovered#234", Confirmation: "Recovered#234", ClientIP: "192.0.2.6"})
	}()
	waitForAccountLockWait(t, h)
	clock.Store(recovery.ExpiresAt.UnixNano())
	release()
	if err := <-finished; !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("reset with token expired while waiting err=%v", err)
	}
	after, err := h.repo.AccountByID(h.ctx, accountID)
	if err != nil || after.PasswordHash != before.PasswordHash {
		t.Fatalf("expired-token request changed password hash err=%v", err)
	}
	stored, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(recovery.Token))
	if err != nil || stored.ConsumedAt != nil {
		t.Fatalf("expired-token request consumed recovery token: consumed=%v err=%v", stored.ConsumedAt, err)
	}
	for table, column := range map[string]string{"historial_clave_local": "usuario_id", "outbox_evento_local": "agregado_id", "evento_auditoria_local": "recurso_id"} {
		var count int
		if err := h.pool.QueryRow(h.ctx, "SELECT count(*) FROM public."+table+" WHERE "+column+"=$1", accountID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("expired-token request left %s rows=%d err=%v", table, count, err)
		}
	}
}

func TestPasswordHistoryWindowStartsWhenLockedChangeCommits(t *testing.T) {
	h, clock := newAuthHarnessWithAtomicClock(t)
	accountID := h.register(t, "local-auth-history-lock@ejemplo.invalid")
	h.verify(t)
	login := h.login(t, "local-auth-history-lock@ejemplo.invalid")
	startedAt := h.now
	release := holdAccountRowLock(t, h, accountID)
	finished := make(chan error, 1)
	go func() {
		finished <- h.service.ChangePassword(h.ctx, identity.ChangePasswordInput{SessionToken: string(login.Token), CurrentPassword: "Synthetic#123", Password: "Changed#234", Confirmation: "Changed#234"})
	}()
	waitForAccountLockWait(t, h)
	committedAt := startedAt.Add(7 * time.Minute)
	clock.Store(committedAt.UnixNano())
	release()
	if err := <-finished; err != nil {
		t.Fatalf("change after lock released: %v", err)
	}
	wantExpiry := identity.AddCalendarMonthsUTC(committedAt, 3)
	var historyStart, historyCreated, historyExpiry, noticeAt, auditAt, auditExpiry time.Time
	if err := h.pool.QueryRow(h.ctx, `SELECT dejo_de_ser_vigente_en, creado_en, retirar_en FROM public.historial_clave_local WHERE usuario_id=$1`, accountID).Scan(&historyStart, &historyCreated, &historyExpiry); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(h.ctx, `SELECT creada_en FROM public.outbox_evento_local WHERE agregado_id=$1`, accountID).Scan(&noticeAt); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(h.ctx, `SELECT ocurrido_en, retirar_en FROM public.evento_auditoria_local WHERE recurso_id=$1`, accountID).Scan(&auditAt, &auditExpiry); err != nil {
		t.Fatal(err)
	}
	if !historyStart.Equal(committedAt) || !historyCreated.Equal(committedAt) || !historyExpiry.Equal(wantExpiry) || !noticeAt.Equal(committedAt) || !auditAt.Equal(committedAt) || !auditExpiry.Equal(identity.AddCalendarMonthsUTC(committedAt, 60)) {
		t.Fatalf("lock-time stamps mismatch history=%s/%s/%s notice=%s audit=%s/%s", historyStart, historyCreated, historyExpiry, noticeAt, auditAt, auditExpiry)
	}
}
