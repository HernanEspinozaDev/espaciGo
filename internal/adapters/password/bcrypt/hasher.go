package bcrypt

import (
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"golang.org/x/crypto/bcrypt"
)

type Hasher struct{}

var _ identity.PasswordHasher = Hasher{}

func (Hasher) Hash(password identity.Secret) (identity.Secret, error) {
	if identity.ValidatePassword(password) != nil {
		return "", identity.ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return identity.Secret(hash), err
}
func (Hasher) Matches(hash, password identity.Secret) (bool, error) {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil || cost < 12 {
		return false, identity.ErrInvalid
	}
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) || errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return false, nil
	}
	return err == nil, err
}
