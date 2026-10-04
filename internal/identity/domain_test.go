package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSecretRedactedAcrossFormattingAndJSON(t *testing.T) {
	s := Secret("credential-that-must-not-leak")
	for _, got := range []string{fmt.Sprint(s), fmt.Sprintf("%#v", s)} {
		if strings.Contains(got, string(s)) {
			t.Fatalf("secret leaked in formatting: %q", got)
		}
	}
	b, err := json.Marshal(struct {
		Secret Secret `json:"secret"`
	}{s})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), string(s)) {
		t.Fatalf("secret leaked in JSON: %s", b)
	}
}
func TestRegistrationRolesNeverAdmin(t *testing.T) {
	for _, r := range RegistrationRoles() {
		if r == RoleAdministrator {
			t.Fatal("registration grants administrator")
		}
	}
}
func TestAccountValidationAndErrorIdentity(t *testing.T) {
	a := Account{ID: "id", Email: "a@example.invalid", NormalizedEmail: "a@example.invalid", PasswordHash: Secret("hash"), State: AccountEmailPending}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	a.State = "unexpected"
	if !errors.Is(a.Validate(), ErrInvalid) {
		t.Fatal("invalid state not rejected")
	}
}
func TestSessionExpiryUsesIdleAndAbsoluteLimits(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Session{CreatedAt: created, LastActivityAt: created, ExpiresAt: created.Add(8 * time.Hour)}
	if !s.ActiveAt(created.Add(29 * time.Minute)) {
		t.Fatal("session should still be active")
	}
	if s.ActiveAt(created.Add(30 * time.Minute)) {
		t.Fatal("idle expiry boundary must be exclusive")
	}
	s.LastActivityAt = created.Add(7*time.Hour + 45*time.Minute)
	if s.ActiveAt(created.Add(8 * time.Hour)) {
		t.Fatal("absolute expiry boundary must be exclusive")
	}
}
func TestEmailNormalizationTrimsAndIgnoresCase(t *testing.T) {
	if got := NormalizeEmail("  A.User@Example.INVALID "); got != "a.user@example.invalid" {
		t.Fatalf("got %q", got)
	}
}

func TestAccountValidationRequiresM01CanonicalEmail(t *testing.T) {
	a := Account{
		ID: "id", Email: " User@Example.INVALID ", NormalizedEmail: "user@example.invalid",
		PasswordHash: Secret("synthetic-hash"), State: AccountEmailPending,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("canonical email rejected: %v", err)
	}
	a.NormalizedEmail = "User@Example.INVALID"
	if !errors.Is(a.Validate(), ErrInvalid) {
		t.Fatal("non-canonical email key was accepted")
	}
}

func TestSessionAndActionTokenHashesRedacted(t *testing.T) {
	hash := strings.Repeat("a", 64)
	values := []any{
		Session{ID: "session-id", AccountID: "account-id", TokenHash: hash},
		ActionToken{ID: "token-id", AccountID: "account-id", Hash: hash},
	}
	for _, value := range values {
		if got := fmt.Sprintf("%+v", value); strings.Contains(got, hash) {
			t.Fatal("stored token hash leaked through formatting")
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), hash) {
			t.Fatal("stored token hash leaked through JSON")
		}
	}
}
