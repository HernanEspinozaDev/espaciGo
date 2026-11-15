package verification

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const declarationSchemaVersion = "1"

type DeclarationRevisionInput struct {
	ID, AccountID, CorrelationID  string
	RutFingerprint                string
	RutFingerprintCandidates      []string
	SchemaVersion, ConsentVersion string
	ConsentGivenAt, RetentionAt   time.Time
}
type DeclarationRevision struct {
	ID, AccountID                          string
	Revision                               int64
	KeyVersion                             string
	Ciphertext                             []byte
	SchemaVersion, ConsentVersion          string
	ConsentGivenAt, RetentionAt, CreatedAt time.Time
}
type DeclarationSealer func(int64) (string, []byte, error)
type DeclarationRevisionRepository interface {
	CreateRevision(context.Context, DeclarationRevisionInput, DeclarationSealer) (DeclarationRevision, error)
	GetRevision(context.Context, string, string) (DeclarationRevision, error)
	GetCurrent(context.Context, string) (DeclarationRevision, error)
}
type DeclarationRetentionPolicy func(time.Time) time.Time
type DeclarationKeyring interface {
	Seal([]byte, []byte) (string, []byte, error)
	Open(string, []byte, []byte) ([]byte, error)
}
type DeclarationService struct {
	repo           DeclarationRevisionRepository
	keys           DeclarationKeyring
	ids            IDGenerator
	now            func() time.Time
	retention      DeclarationRetentionPolicy
	consentVersion string
	rutKeyring     *RutHMACKeyring
}
type SavedDeclaration struct {
	ID             string
	Revision       int64
	Payload        DeclarationPayload
	ConsentVersion string
	ConsentGivenAt time.Time
}

func validDeclarationConsentVersion(version string) bool {
	return utf8.ValidString(version) && utf8.RuneCountInString(version) >= 1 && utf8.RuneCountInString(version) <= 20 && strings.TrimSpace(version) == version
}

func validDeclarationRutHMACVersion(version string) bool {
	if !utf8.ValidString(version) || len(version) < 1 || len(version) > 32 {
		return false
	}
	for i, b := range []byte(version) {
		valid := b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
		if i > 0 {
			valid = valid || b == '.' || b == '_' || b == '-'
		}
		if !valid {
			return false
		}
	}
	return true
}

func validDeclarationCorrelationID(id string) bool {
	return utf8.ValidString(id) && utf8.RuneCountInString(id) >= 1 && utf8.RuneCountInString(id) <= 120 && strings.TrimSpace(id) == id
}

func NewDeclarationService(repo DeclarationRevisionRepository, keys DeclarationKeyring, ids IDGenerator, now func() time.Time, retention DeclarationRetentionPolicy, consentVersion string) (*DeclarationService, error) {
	if repo == nil || keys == nil || ids == nil || now == nil || retention == nil || !validDeclarationConsentVersion(consentVersion) {
		return nil, ErrInvalid
	}
	return &DeclarationService{repo: repo, keys: keys, ids: ids, now: now, retention: retention, consentVersion: consentVersion}, nil
}

func NewDeclarationServiceWithRutKeyring(repo DeclarationRevisionRepository, keys DeclarationKeyring, ids IDGenerator, now func() time.Time, retention DeclarationRetentionPolicy, consentVersion string, rutKeys *RutHMACKeyring) (*DeclarationService, error) {
	service, err := NewDeclarationService(repo, keys, ids, now, retention, consentVersion)
	if err != nil {
		return nil, err
	}
	if rutKeys == nil || len(rutKeys.Versions()) == 0 {
		return nil, ErrInvalid
	}
	if _, err := rutKeys.Fingerprints("11222333-9"); err != nil {
		return nil, ErrInvalid
	}
	service.rutKeyring = rutKeys
	return service, nil
}

func NewDeclarationServiceWithRutHMAC(repo DeclarationRevisionRepository, keys DeclarationKeyring, ids IDGenerator, now func() time.Time, retention DeclarationRetentionPolicy, consentVersion string, secret []byte, version string) (*DeclarationService, error) {
	if len(secret) < 16 || !validDeclarationRutHMACVersion(version) {
		return nil, ErrInvalid
	}
	keyring, err := NewRutHMACKeyring(version, map[string][]byte{version: append([]byte(nil), secret...)})
	if err != nil {
		return nil, ErrInvalid
	}
	return NewDeclarationServiceWithRutKeyring(repo, keys, ids, now, retention, consentVersion, keyring)
}

// RutKeyring lets runtime composition reuse the exact immutable snapshot used
// by Save. No key bytes or mutating operations are exposed by its public API.
func (s *DeclarationService) RutKeyring() *RutHMACKeyring {
	if s == nil {
		return nil
	}
	return s.rutKeyring
}

func canonicalDeclarationUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s
}
func (s *DeclarationService) Save(ctx context.Context, accountID string, payload DeclarationPayload, consent bool, consentVersion string, correlationIDs ...string) (SavedDeclaration, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) || !consent || !validDeclarationConsentVersion(consentVersion) || consentVersion != s.consentVersion || len(correlationIDs) > 1 {
		return SavedDeclaration{}, ErrInvalid
	}
	correlationID := ""
	if len(correlationIDs) == 1 {
		correlationID = correlationIDs[0]
	} else {
		var err error
		correlationID, err = s.ids.ID()
		if err != nil {
			return SavedDeclaration{}, ErrInvalid
		}
	}
	if !validDeclarationCorrelationID(correlationID) {
		return SavedDeclaration{}, ErrInvalid
	}
	payload.RUT, _ = NormalizeRut(payload.RUT)
	var err error
	fingerprints := RutFingerprintSet{}
	if s.rutKeyring != nil {
		fingerprints, err = s.rutKeyring.Fingerprints(payload.RUT)
		if err != nil || ValidateRutFingerprintSet(fingerprints) != nil {
			return SavedDeclaration{}, ErrInvalid
		}
	}
	plain, err := EncodeDeclarationPayload(payload)
	if err != nil {
		return SavedDeclaration{}, ErrInvalid
	}
	// PostgreSQL timestamptz stores microsecond precision; normalize before
	// writing so an exact readback compares against the persisted instants.
	at := s.now().UTC().Truncate(time.Microsecond)
	if err := ValidateAdultBirthDate(payload.DateOfBirth, at); err != nil {
		return SavedDeclaration{}, err
	}
	retentionAt := s.retention(at).UTC().Truncate(time.Microsecond)
	if at.IsZero() || !retentionAt.After(at) {
		return SavedDeclaration{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil || !canonicalDeclarationUUID(id) {
		return SavedDeclaration{}, ErrInvalid
	}
	input := DeclarationRevisionInput{ID: id, AccountID: accountID, CorrelationID: correlationID, RutFingerprint: fingerprints.Active, RutFingerprintCandidates: append([]string(nil), fingerprints.Candidates...), SchemaVersion: declarationSchemaVersion, ConsentVersion: consentVersion, ConsentGivenAt: at, RetentionAt: retentionAt}
	created, err := s.repo.CreateRevision(ctx, input, func(revision int64) (string, []byte, error) {
		aad, e := BuildDeclarationAAD(accountID, id, revision, declarationSchemaVersion)
		if e != nil {
			return "", nil, ErrInvalid
		}
		return s.keys.Seal(aad, plain)
	})
	if err != nil {
		return SavedDeclaration{}, err
	}
	row, err := s.repo.GetRevision(ctx, accountID, id)
	if err != nil {
		return SavedDeclaration{}, err
	}
	if row.ID != id || row.AccountID != accountID || row.Revision <= 0 || row.SchemaVersion != declarationSchemaVersion || row.ConsentVersion != consentVersion || !row.ConsentGivenAt.Equal(at) || !row.RetentionAt.Equal(retentionAt) || row.Revision != created.Revision {
		return SavedDeclaration{}, ErrInvalid
	}
	return s.decode(row)
}
func (s *DeclarationService) GetCurrent(ctx context.Context, accountID string) (SavedDeclaration, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) {
		return SavedDeclaration{}, ErrInvalid
	}
	row, err := s.repo.GetCurrent(ctx, accountID)
	if err != nil {
		return SavedDeclaration{}, err
	}
	if row.AccountID != accountID {
		return SavedDeclaration{}, ErrNotFound
	}
	return s.decode(row)
}
func (s *DeclarationService) GetRevision(ctx context.Context, accountID, id string) (SavedDeclaration, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) || !canonicalDeclarationUUID(id) {
		return SavedDeclaration{}, ErrInvalid
	}
	row, err := s.repo.GetRevision(ctx, accountID, id)
	if err != nil {
		return SavedDeclaration{}, err
	}
	if row.AccountID != accountID || row.ID != id {
		return SavedDeclaration{}, ErrNotFound
	}
	return s.decode(row)
}
func (s *DeclarationService) decode(row DeclarationRevision) (SavedDeclaration, error) {
	if row.Revision <= 0 || row.SchemaVersion != declarationSchemaVersion {
		return SavedDeclaration{}, ErrInvalid
	}
	if row.RetentionAt.IsZero() || !row.RetentionAt.After(s.now().UTC()) {
		return SavedDeclaration{}, ErrNotFound
	}
	aad, err := BuildDeclarationAAD(row.AccountID, row.ID, row.Revision, row.SchemaVersion)
	if err != nil {
		return SavedDeclaration{}, ErrInvalid
	}
	plain, err := s.keys.Open(row.KeyVersion, aad, row.Ciphertext)
	if err != nil {
		return SavedDeclaration{}, errors.New("declaration unavailable")
	}
	payload, err := DecodeDeclarationPayload(plain)
	if err != nil {
		return SavedDeclaration{}, ErrInvalid
	}
	return SavedDeclaration{ID: row.ID, Revision: row.Revision, Payload: payload, ConsentVersion: row.ConsentVersion, ConsentGivenAt: row.ConsentGivenAt}, nil
}
