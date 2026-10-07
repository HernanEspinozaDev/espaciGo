package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRegistrationEmailAndPasswordPolicy(t *testing.T) {
	for _, email := range []string{"user@ejemplo.invalid", " A.User+alias@Example.INVALID ", "straße@ejemplo.invalid"} {
		if err := ValidateEmail(email); err != nil {
			t.Fatalf("valid fixture email rejected: %v", err)
		}
	}
	for _, email := range []string{"", "invalid", "Name <user@ejemplo.invalid>", "user@ejemplo.invalid (comment)", "a@x.invalid,b@y.invalid", "user\n@ejemplo.invalid"} {
		if !errors.Is(ValidateEmail(email), ErrInvalid) {
			t.Fatal("malformed email accepted")
		}
	}
	if NormalizeEmail(" Straße+alias@Example.INVALID ") != "strasse+alias@example.invalid" {
		t.Fatal("Unicode casefold/provider aliases changed")
	}
	for _, password := range []Secret{"Valid#12", "Ñúmero!123"} {
		if ValidatePassword(password) != nil {
			t.Fatal("valid password rejected")
		}
	}
	for _, password := range []Secret{"Ab#1234", "lower#123", "Nodigits!", "NoSymbol123", "        ", Secret("A1!" + strings.Repeat("x", 70))} {
		if !errors.Is(ValidatePassword(password), ErrInvalid) {
			t.Fatal("invalid or bcrypt-overlength password accepted")
		}
	}
}

func TestAuthenticationOutputsRedactCredentials(t *testing.T) {
	raw := Secret("synthetic-token-which-must-not-be-logged")
	for _, value := range []any{LoginResult{Token: raw}, VerificationDelivery{Token: raw}} {
		for _, formatted := range []string{fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value)} {
			if strings.Contains(formatted, string(raw)) {
				t.Fatal("raw credential leaked in formatting")
			}
		}
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), string(raw)) {
			t.Fatal("raw credential leaked in JSON")
		}
	}
}

func TestAuthenticationRequiresExplicitDependencies(t *testing.T) {
	if _, err := NewAuthenticationService(nil, nil, nil, nil, nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("service accepted missing security policies")
	}
}

func TestAddCalendarMonthsUTCClampsCalendarDay(t *testing.T) {
	tests := []struct {
		input, want time.Time
	}{
		{time.Date(2026, time.January, 31, 10, 25, 0, 0, time.UTC), time.Date(2026, time.April, 30, 10, 25, 0, 0, time.UTC)},
		{time.Date(2026, time.May, 31, 23, 59, 59, 0, time.UTC), time.Date(2026, time.August, 31, 23, 59, 59, 0, time.UTC)},
		{time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC), time.Date(2024, time.May, 29, 12, 0, 0, 0, time.UTC)},
		{time.Date(2026, time.November, 30, 8, 0, 0, 0, time.UTC), time.Date(2027, time.February, 28, 8, 0, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		if got := AddCalendarMonthsUTC(test.input, 3); !got.Equal(test.want) {
			t.Fatalf("AddCalendarMonthsUTC(%s) = %s; want %s", test.input, got, test.want)
		}
	}
}
