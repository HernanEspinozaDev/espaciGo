package credentials

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Generator struct{}

var _ identity.CredentialGenerator = Generator{}

func (Generator) ID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
func (Generator) Token() (identity.Secret, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return identity.Secret(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
