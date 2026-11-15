package verification

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// DeclarationPayload is the versioned, allowlisted identity snapshot encrypted by KYC persistence.
type DeclarationPayload struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	DateOfBirth  string `json:"date_of_birth"`
	RUT          string `json:"rut"`
	IDCountry    string `json:"id_country"`
	DocumentType string `json:"document_type"`
}

func ValidateAdultBirthDate(birthDate string, now time.Time) error {
	if now.IsZero() || len(birthDate) != len("2006-01-02") {
		return ErrInvalid
	}
	parsed, err := time.Parse("2006-01-02", birthDate)
	if err != nil || parsed.Before(time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		return ErrInvalid
	}
	utcNow := now.UTC()
	cutoff := time.Date(utcNow.Year()-18, utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)
	if parsed.After(cutoff) {
		return ErrInvalid
	}
	return nil
}

var errInvalidDeclaration = errors.New("invalid declaration payload")
var errInvalidDeclarationAAD = errors.New("invalid declaration AAD")

func validateDeclarationPayload(p DeclarationPayload) error {
	if !utf8.ValidString(p.FirstName) || !utf8.ValidString(p.LastName) || strings.TrimSpace(p.FirstName) == "" || strings.TrimSpace(p.LastName) == "" || utf8.RuneCountInString(p.FirstName) > 150 || utf8.RuneCountInString(p.LastName) > 150 {
		return errInvalidDeclaration
	}
	if len(p.DateOfBirth) != len("2006-01-02") {
		return errInvalidDeclaration
	}
	birthDate, err := time.Parse("2006-01-02", p.DateOfBirth)
	if err != nil || birthDate.After(time.Now().UTC()) {
		return errInvalidDeclaration
	}
	normalized, err := NormalizeRut(p.RUT)
	if err != nil || normalized != p.RUT || p.IDCountry != "CHL" || p.DocumentType != "ID" {
		return errInvalidDeclaration
	}
	return nil
}

func EncodeDeclarationPayload(p DeclarationPayload) ([]byte, error) {
	if err := validateDeclarationPayload(p); err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

func DecodeDeclarationPayload(data []byte) (DeclarationPayload, error) {
	var p DeclarationPayload
	if !utf8.Valid(data) {
		return p, errInvalidDeclaration
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return DeclarationPayload{}, errInvalidDeclaration
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return DeclarationPayload{}, errInvalidDeclaration
	}
	if err := validateDeclarationPayload(p); err != nil {
		return DeclarationPayload{}, errInvalidDeclaration
	}
	return p, nil
}

func BuildDeclarationAAD(accountID, declarationID string, revision int64, schemaVersion string) ([]byte, error) {
	account, err := uuid.Parse(accountID)
	if err != nil || account.String() != accountID {
		return nil, errInvalidDeclarationAAD
	}
	declaration, err := uuid.Parse(declarationID)
	if err != nil || declaration.String() != declarationID {
		return nil, errInvalidDeclarationAAD
	}
	if revision <= 0 || schemaVersion != "1" {
		return nil, errInvalidDeclarationAAD
	}
	const purpose = "espacigo:identity-declaration:v1"
	var out bytes.Buffer
	writeAADField(&out, []byte(purpose))
	writeAADField(&out, account[:])
	writeAADField(&out, declaration[:])
	var rev [8]byte
	binary.BigEndian.PutUint64(rev[:], uint64(revision))
	writeAADField(&out, rev[:])
	writeAADField(&out, []byte(schemaVersion))
	return out.Bytes(), nil
}

func writeAADField(out *bytes.Buffer, value []byte) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	out.Write(size[:])
	out.Write(value)
}
