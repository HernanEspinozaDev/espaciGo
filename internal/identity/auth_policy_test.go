package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
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
