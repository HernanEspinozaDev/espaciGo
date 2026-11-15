package verification

import (
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
	"github.com/google/uuid"
)

func TestDecodeEncryptedDeclarationReturnsOnlyLegalIdentity(t *testing.T) {
	accountID, declarationID := uuid.NewString(), uuid.NewString()
	payload, err := EncodeDeclarationPayload(DeclarationPayload{FirstName: " Ana ", LastName: " Pérez ", DateOfBirth: "1990-01-01", RUT: "12345678-5", IDCountry: "CHL", DocumentType: "ID"})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := declarationcrypto.New("v1", map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	aad, err := BuildDeclarationAAD(accountID, declarationID, 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	version, ciphertext, err := keys.Seal(aad, payload)
	if err != nil {
		t.Fatal(err)
	}
	row := EncryptedDeclarationRevision{AccountID: accountID, DeclarationID: declarationID, Revision: 1, SchemaVersion: "1", KeyVersion: version, Ciphertext: ciphertext, CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), ConsentAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), RetainUntil: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}
	got, err := DecodeDeclarationRevision(row, accountID, declarationID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), keys)
	if err != nil {
		t.Fatal(err)
	}
	if got.LegalFirstName != "Ana" || got.LegalLastName != "Pérez" || got.CanonicalRUT != "12345678-5" {
		t.Fatalf("unexpected projection: %#v", got)
	}
}

func TestDecodeDeclarationRevisionFailsClosed(t *testing.T) {
	accountID, declarationID := uuid.NewString(), uuid.NewString()
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	payload, err := EncodeDeclarationPayload(DeclarationPayload{FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-01", RUT: "12345678-5", IDCountry: "CHL", DocumentType: "ID"})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := declarationcrypto.New("v1", map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	aad, err := BuildDeclarationAAD(accountID, declarationID, 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	version, ciphertext, err := keys.Seal(aad, payload)
	if err != nil {
		t.Fatal(err)
	}
	valid := EncryptedDeclarationRevision{AccountID: accountID, DeclarationID: declarationID, Revision: 1, SchemaVersion: "1", KeyVersion: version, Ciphertext: ciphertext, CreatedAt: asOf.Add(-time.Hour), ConsentAt: asOf.Add(-2 * time.Hour), RetainUntil: asOf.Add(time.Hour)}

	tests := []struct {
		name               string
		row                EncryptedDeclarationRevision
		expectedAccountID  string
		expectedRevisionID string
		keyring            DeclarationKeyring
	}{
		{name: "wrong account", row: valid, expectedAccountID: uuid.NewString(), expectedRevisionID: declarationID, keyring: keys},
		{name: "wrong bound revision", row: valid, expectedAccountID: accountID, expectedRevisionID: uuid.NewString(), keyring: keys},
		{name: "retention boundary", row: func() EncryptedDeclarationRevision { v := valid; v.RetainUntil = asOf; return v }(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "future creation", row: func() EncryptedDeclarationRevision { v := valid; v.CreatedAt = asOf.Add(time.Nanosecond); return v }(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "future consent", row: func() EncryptedDeclarationRevision { v := valid; v.ConsentAt = asOf.Add(time.Nanosecond); return v }(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "invalid schema", row: func() EncryptedDeclarationRevision { v := valid; v.SchemaVersion = "2"; return v }(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "invalid revision", row: func() EncryptedDeclarationRevision { v := valid; v.Revision = 0; return v }(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "unknown key", row: func() EncryptedDeclarationRevision { v := valid; v.KeyVersion = "missing"; return v }(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "tampered ciphertext", row: func() EncryptedDeclarationRevision {
			v := valid
			v.Ciphertext = append([]byte(nil), v.Ciphertext...)
			v.Ciphertext[len(v.Ciphertext)-1] ^= 0xff
			return v
		}(), expectedAccountID: accountID, expectedRevisionID: declarationID, keyring: keys},
		{name: "missing keyring", row: valid, expectedAccountID: accountID, expectedRevisionID: declarationID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeDeclarationRevision(tt.row, tt.expectedAccountID, tt.expectedRevisionID, asOf, tt.keyring)
			if err == nil {
				t.Fatal("expected declaration to be withheld")
			}
			if got != (LegalIdentityProjection{}) {
				t.Fatalf("failure returned a partial projection: %#v", got)
			}
			if strings.Contains(err.Error(), "Ana") || strings.Contains(err.Error(), "Pérez") {
				t.Fatalf("error disclosed legal identity: %v", err)
			}
		})
	}
}
