package bcrypt_test

import (
	"errors"
	"strings"
	"testing"

	adapter "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptCostMatchingAndNoSilentTruncation(t *testing.T) {
	h := adapter.Hasher{}
	password := identity.Secret("Synthetic#123")
	hash, err := h.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil || cost < 12 {
		t.Fatal("hash below required bcrypt cost")
	}
	if string(hash) == string(password) {
		t.Fatal("password stored in clear")
	}
	if ok, err := h.Matches(hash, password); err != nil || !ok {
		t.Fatal("correct password rejected")
	}
	if ok, err := h.Matches(hash, "Wrong#123"); err != nil || ok {
		t.Fatal("wrong password accepted")
	}
	if _, err := h.Hash(identity.Secret("A1!" + strings.Repeat("x", 70))); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("bcrypt-overlength password accepted")
	}
	weak, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Matches(identity.Secret(weak), password); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("weak legacy hash accepted")
	}
}
