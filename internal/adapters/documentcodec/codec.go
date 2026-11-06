// Package documentcodec implements the stable authenticated envelope for private documents.
// It performs no I/O and accepts only scopes constructed from canonical UUID identifiers.
package documentcodec

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const MaxDocumentBytes = 10 << 20
const magic = "ESPACIGODOC1"

var ErrInvalid = errors.New("documentcodec: invalid key, scope, or content")
var ErrCorrupt = errors.New("documentcodec: invalid or unauthenticated envelope")

type Scope struct{ aad string }

func IdentityScope(ownerID, caseID, fileID string) (Scope, error) {
	if !valid(ownerID, caseID, fileID) {
		return Scope{}, ErrInvalid
	}
	return Scope{aad: path.Join("users", ownerID, "identity", caseID, fileID+".bin")}, nil
}
func SpaceScope(ownerID, spaceID, caseID, fileID string) (Scope, error) {
	if !valid(ownerID, spaceID, caseID, fileID) {
		return Scope{}, ErrInvalid
	}
	return Scope{aad: path.Join("users", ownerID, "spaces", spaceID, "documents", caseID, fileID+".bin")}, nil
}
func valid(ids ...string) bool {
	for _, id := range ids {
		if documentverification.ValidateID(id) != nil {
			return false
		}
	}
	return true
}

// ScopeFromAAD accepts only canonical path values emitted by this package's scope builders.
func ScopeFromAAD(aad string) Scope {
	p := strings.Split(aad, "/")
	if len(p) == 5 && p[0] == "users" && p[2] == "identity" && strings.HasSuffix(p[4], ".bin") {
		if s, e := IdentityScope(p[1], p[3], strings.TrimSuffix(p[4], ".bin")); e == nil && s.aad == aad {
			return s
		}
	}
	if len(p) == 7 && p[0] == "users" && p[2] == "spaces" && p[4] == "documents" && strings.HasSuffix(p[6], ".bin") {
		if s, e := SpaceScope(p[1], p[3], p[5], strings.TrimSuffix(p[6], ".bin")); e == nil && s.aad == aad {
			return s
		}
	}
	return Scope{}
}

func Seal(key []byte, scope Scope, plaintext []byte) ([]byte, error) {
	if scope.aad == "" || len(key) != 32 || len(plaintext) == 0 || len(plaintext) > MaxDocumentBytes {
		return nil, ErrInvalid
	}
	a, err := newAEAD(key)
	if err != nil {
		return nil, ErrInvalid
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(magic)+len(nonce)+len(plaintext)+a.Overhead())
	out = append(out, magic...)
	out = append(out, nonce...)
	return a.Seal(out, nonce, plaintext, []byte(scope.aad)), nil
}
func Open(key []byte, scope Scope, envelope []byte) ([]byte, error) {
	if scope.aad == "" || len(key) != 32 {
		return nil, ErrInvalid
	}
	a, err := newAEAD(key)
	if err != nil {
		return nil, ErrInvalid
	}
	start := len(magic)
	end := start + a.NonceSize()
	if len(envelope) < end+a.Overhead() || len(envelope) > len(magic)+a.NonceSize()+MaxDocumentBytes+a.Overhead() || string(envelope[:start]) != magic {
		return nil, ErrCorrupt
	}
	plain, err := a.Open(nil, envelope[start:end], envelope[end:], []byte(scope.aad))
	if err != nil || len(plain) == 0 || len(plain) > MaxDocumentBytes {
		return nil, ErrCorrupt
	}
	return plain, nil
}
func newAEAD(key []byte) (cipher.AEAD, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}

// KeyForScope returns the canonical filesystem-compatible AAD. It is not a client object key or URL.
func KeyForScope(scope Scope) (string, error) {
	if scope.aad == "" || strings.Contains(scope.aad, "..") {
		return "", ErrInvalid
	}
	return scope.aad, nil
}
