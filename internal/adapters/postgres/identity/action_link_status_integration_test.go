package postgres_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
)

func TestActionLinkStatusVerificationAndRecoveryLifecycle(t *testing.T) {
	h, clock := newAuthHarnessWithAtomicClock(t)
	initialTime := time.Unix(0, clock.Load()).UTC()
	api := identityhttp.NewHandler(h.service, h.repo, nil)

	email := "lifecycle-link@ejemplo.invalid"
	accountID := h.register(t, email)
	if accountID == "" {
		t.Fatal("failed to register account")
	}

	verif := h.mail.last()
	if verif.TokenID == "" || verif.Token == "" {
		t.Fatal("verification delivery missing")
	}

	// 1. Fresh verification link -> ready
	status, err := h.service.InspectVerificationLink(h.ctx, verif.TokenID, verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusReady {
		t.Fatalf("expected ready status, got status=%q err=%v", status, err)
	}

	// Upper-case UUID canonical acceptance -> ready
	status, err = h.service.InspectVerificationLink(h.ctx, strings.ToUpper(verif.TokenID), verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusReady {
		t.Fatalf("expected ready status for uppercase UUID, got status=%q err=%v", status, err)
	}

	// Oversized raw secret (> 2048 chars) -> invalid (no DB search or consumption)
	oversizedSecret := identity.Secret(strings.Repeat("x", 2049))
	status, err = h.service.InspectVerificationLink(h.ctx, verif.TokenID, oversizedSecret, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for oversized secret, got status=%q err=%v", status, err)
	}

	// 2. Wrong secret -> invalid (oracle safety, no side effects)
	status, err = h.service.InspectVerificationLink(h.ctx, verif.TokenID, "WrongSecretToken#999", "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for wrong secret, got status=%q err=%v", status, err)
	}

	// 3. Wrong purpose -> invalid (recovery endpoint on verification token)
	status, err = h.service.InspectRecoveryLink(h.ctx, verif.TokenID, verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for wrong purpose, got status=%q err=%v", status, err)
	}

	// 4. Unknown UUID -> invalid
	status, err = h.service.InspectVerificationLink(h.ctx, "00000000-0000-4000-8000-000000000099", verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for unknown UUID, got status=%q err=%v", status, err)
	}

	// 5. Malformed ID -> invalid
	status, err = h.service.InspectVerificationLink(h.ctx, "not-a-uuid", verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for malformed ID, got status=%q err=%v", status, err)
	}

	// 6. Expired link
	clock.Store(initialTime.Add(25 * time.Hour).UnixNano())
	status, err = h.service.InspectVerificationLink(h.ctx, verif.TokenID, verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusExpired {
		t.Fatalf("expected expired status, got status=%q err=%v", status, err)
	}

	// Wrong secret against expired token -> invalid (hash check precedes status check)
	status, err = h.service.InspectVerificationLink(h.ctx, verif.TokenID, "WrongSecretToken#999", "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for wrong secret against expired token, got status=%q err=%v", status, err)
	}

	// Rewind clock for verification consumption
	clock.Store(initialTime.UnixNano())
	if err := h.service.VerifyEmail(h.ctx, verif.TokenID, verif.Token, "192.0.2.1"); err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	// 7. Consumed verification token -> used
	status, err = h.service.InspectVerificationLink(h.ctx, verif.TokenID, verif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusUsed {
		t.Fatalf("expected used status, got status=%q err=%v", status, err)
	}

	// Wrong secret against consumed token -> invalid (hash check precedes status check)
	status, err = h.service.InspectVerificationLink(h.ctx, verif.TokenID, "WrongSecretToken#999", "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid status for wrong secret against consumed token, got status=%q err=%v", status, err)
	}

	// 8. Second VerifyEmail replay fails with ErrTokenInvalid
	if err := h.service.VerifyEmail(h.ctx, verif.TokenID, verif.Token, "192.0.2.1"); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid on verification replay, got %v", err)
	}

	// 9. Password recovery flow
	if err := h.service.RequestPasswordRecovery(h.ctx, email, "192.0.2.1"); err != nil {
		t.Fatalf("request recovery failed: %v", err)
	}
	recovery := h.mail.lastRecovery()
	if recovery.TokenID == "" || recovery.Token == "" {
		t.Fatal("recovery delivery missing")
	}

	// Recovery token -> ready
	status, err = h.service.InspectRecoveryLink(h.ctx, recovery.TokenID, recovery.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusReady {
		t.Fatalf("expected recovery ready, got %q err=%v", status, err)
	}

	// Recovery token inspected on verification endpoint -> invalid
	status, err = h.service.InspectVerificationLink(h.ctx, recovery.TokenID, recovery.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected invalid on cross-purpose inspection, got %q err=%v", status, err)
	}

	// Recovery token expiry (15m TTL)
	clock.Store(initialTime.Add(16 * time.Minute).UnixNano())
	status, err = h.service.InspectRecoveryLink(h.ctx, recovery.TokenID, recovery.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusExpired {
		t.Fatalf("expected recovery expired, got %q err=%v", status, err)
	}

	// Rewind clock back within TTL
	clock.Store(initialTime.UnixNano())

	// 10. First reset password succeeds
	firstPass := identity.Secret("FirstNewPassword#123")
	err = h.service.ResetPassword(h.ctx, identity.ResetPasswordInput{
		TokenID:      recovery.TokenID,
		Token:        recovery.Token,
		Password:     firstPass,
		Confirmation: firstPass,
		ClientIP:     "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("first reset password failed: %v", err)
	}

	// Recovery status after consumption -> used
	status, err = h.service.InspectRecoveryLink(h.ctx, recovery.TokenID, recovery.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusUsed {
		t.Fatalf("expected recovery used after reset, got %q err=%v", status, err)
	}

	// 11. Replay password reset with a different password fails with ErrTokenInvalid
	secondPass := identity.Secret("SecondNewPassword#456")
	err = h.service.ResetPassword(h.ctx, identity.ResetPasswordInput{
		TokenID:      recovery.TokenID,
		Token:        recovery.Token,
		Password:     secondPass,
		Confirmation: secondPass,
		ClientIP:     "192.0.2.1",
	})
	if !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid on second reset, got %v", err)
	}

	// Assert password hash unchanged: login with firstPass succeeds, secondPass fails
	loginRes, err := h.service.Login(h.ctx, identity.LoginInput{Email: email, Password: firstPass})
	if err != nil || loginRes.AccountID == "" {
		t.Fatalf("login with first new password failed: %v", err)
	}
	_, err = h.service.Login(h.ctx, identity.LoginInput{Email: email, Password: secondPass})
	if !errors.Is(err, identity.ErrCredentials) {
		t.Fatalf("expected ErrCredentials with second password, got %v", err)
	}

	// 12. HTTP Handler integration tests
	httpReq := func(method, path string, body any) *httptest.ResponseRecorder {
		var jsonBytes []byte
		if body != nil {
			var err error
			jsonBytes, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(jsonBytes))
		req.RemoteAddr = "192.0.2.55:9000"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}

	// Verification status HTTP test -> used
	rec := httpReq(http.MethodPost, "/api/v1/auth/verification/status", map[string]string{
		"token_id": verif.TokenID,
		"token":    string(verif.Token),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Status != "used" {
		t.Fatalf("expected {status: used}, got %s", rec.Body.String())
	}

	// Recovery status HTTP test -> used
	rec = httpReq(http.MethodPost, "/api/v1/auth/password/recovery/status", map[string]string{
		"token_id": recovery.TokenID,
		"token":    string(recovery.Token),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Status != "used" {
		t.Fatalf("expected {status: used}, got %s", rec.Body.String())
	}

	// HTTP reject other methods (GET -> 405)
	rec = httpReq(http.MethodGet, "/api/v1/auth/verification/status", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", rec.Code)
	}

	// HTTP malformed / unknown fields -> 400
	rec = httpReq(http.MethodPost, "/api/v1/auth/verification/status", map[string]any{
		"token_id": verif.TokenID,
		"token":    string(verif.Token),
		"extra":    "forbidden_field",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown fields, got %d", rec.Code)
	}
}

func TestActionLinkStatusReissueAndAttemptsExhausted(t *testing.T) {
	h := newAuthHarness(t)

	email := "reissue-attempts@ejemplo.invalid"
	h.register(t, email)
	firstVerif := h.mail.last()

	// Reissue verification -> invalidates first token
	if err := h.service.ReissueVerification(h.ctx, email, "192.0.2.1"); err != nil {
		t.Fatalf("reissue verification failed: %v", err)
	}
	secondVerif := h.mail.last()
	if firstVerif.TokenID == secondVerif.TokenID {
		t.Fatal("expected new token on reissue")
	}

	// First token must now be invalid
	status, err := h.service.InspectVerificationLink(h.ctx, firstVerif.TokenID, firstVerif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected first token invalid after reissue, got status=%q err=%v", status, err)
	}

	// Second token must be ready
	status, err = h.service.InspectVerificationLink(h.ctx, secondVerif.TokenID, secondVerif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusReady {
		t.Fatalf("expected second token ready, got status=%q err=%v", status, err)
	}

	// Exhaust attempts on second token (5 failures)
	for i := 0; i < 5; i++ {
		_ = h.service.VerifyEmail(h.ctx, secondVerif.TokenID, "WrongTokenSecret#123", "192.0.2.1")
	}

	// Now second token with correct secret must report invalid
	status, err = h.service.InspectVerificationLink(h.ctx, secondVerif.TokenID, secondVerif.Token, "192.0.2.1")
	if err != nil || status != identity.LinkStatusInvalid {
		t.Fatalf("expected token invalid after 5 failures, got status=%q err=%v", status, err)
	}
}

func TestActionLinkInspectorConcurrencyNoMutations(t *testing.T) {
	h := newAuthHarness(t)
	email := "concurrent-inspect@ejemplo.invalid"
	h.register(t, email)
	verif := h.mail.last()

	const numWorkers = 10
	var wg sync.WaitGroup
	errs := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, err := h.service.InspectVerificationLink(h.ctx, verif.TokenID, verif.Token, "192.0.2.1")
			if err != nil {
				errs <- err
				return
			}
			if st != identity.LinkStatusReady {
				errs <- errors.New("concurrent inspector did not return ready")
				return
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	// Confirm token was not consumed and attempts not incremented
	token, err := h.repo.ActionTokenByHash(h.ctx, identity.CredentialHash(verif.Token))
	if err != nil {
		t.Fatal(err)
	}
	if token.ConsumedAt != nil || token.Attempts != 0 {
		t.Fatalf("inspection modified action token: consumed=%v attempts=%d", token.ConsumedAt, token.Attempts)
	}
}
