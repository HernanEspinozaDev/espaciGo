package verification

import (
	"errors"
	"strings"
	"time"
)

// DeclarationOpener decrypts authenticated declaration ciphertext without granting write access.
type DeclarationOpener interface {
	Open(keyVersion string, aad, ciphertext []byte) ([]byte, error)
}

// EncryptedDeclarationRevision is the minimal encrypted persistence row input.
type EncryptedDeclarationRevision struct {
	AccountID     string
	DeclarationID string
	Revision      int64
	SchemaVersion string
	KeyVersion    string
	Ciphertext    []byte
	CreatedAt     time.Time
	ConsentAt     time.Time
	RetainUntil   time.Time
}

// LegalIdentityProjection contains only fields permitted for this read purpose.
type LegalIdentityProjection struct {
	LegalFirstName string
	LegalLastName  string
	CanonicalRUT   string
}

var errDeclarationUnavailable = errors.New("declaration unavailable")

// DecodeDeclarationRevision validates scope and lifetime before decrypting an exact bound revision.
func DecodeDeclarationRevision(row EncryptedDeclarationRevision, expectedAccountID, expectedRevisionID string, asOf time.Time, keyring DeclarationOpener) (LegalIdentityProjection, error) {
	if keyring == nil || asOf.IsZero() || row.AccountID != expectedAccountID || row.DeclarationID != expectedRevisionID || row.KeyVersion == "" || len(row.Ciphertext) == 0 || row.Revision <= 0 || row.SchemaVersion != "1" || row.CreatedAt.IsZero() || row.ConsentAt.IsZero() || row.RetainUntil.IsZero() || row.CreatedAt.After(asOf) || row.ConsentAt.After(asOf) || !row.RetainUntil.After(asOf) {
		return LegalIdentityProjection{}, errDeclarationUnavailable
	}
	aad, err := BuildDeclarationAAD(row.AccountID, row.DeclarationID, row.Revision, row.SchemaVersion)
	if err != nil {
		return LegalIdentityProjection{}, errDeclarationUnavailable
	}
	plaintext, err := keyring.Open(row.KeyVersion, aad, row.Ciphertext)
	if err != nil {
		return LegalIdentityProjection{}, errDeclarationUnavailable
	}
	payload, err := DecodeDeclarationPayload(plaintext)
	if err != nil {
		return LegalIdentityProjection{}, errDeclarationUnavailable
	}
	return LegalIdentityProjection{LegalFirstName: strings.TrimSpace(payload.FirstName), LegalLastName: strings.TrimSpace(payload.LastName), CanonicalRUT: payload.RUT}, nil
}
