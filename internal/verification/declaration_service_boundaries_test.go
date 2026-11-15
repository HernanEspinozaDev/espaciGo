package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

type mockKeyringForBoundaries struct {
	sealErr  error
	openErr  error
	openData []byte
}

func (m *mockKeyringForBoundaries) Seal([]byte, []byte) (string, []byte, error) {
	if m.sealErr != nil {
		return "", nil, m.sealErr
	}
	return "v1", []byte("encrypted-payload"), nil
}

func (m *mockKeyringForBoundaries) Open(string, []byte, []byte) ([]byte, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	if m.openData != nil {
		return m.openData, nil
	}
	return EncodeDeclarationPayload(DeclarationPayload{
		FirstName:    "Synthetic",
		LastName:     "Persona",
		DateOfBirth:  "1990-01-01",
		RUT:          "11.222.333-9",
		IDCountry:    "CHL",
		DocumentType: "ID",
	})
}

func TestDeclarationServiceConstructorBoundaries(t *testing.T) {
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	keys, _ := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	ids := declarationIDFunc(func() (string, error) { return "85000000-0000-4000-8000-000000000011", nil })
	now := func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }
	retention := DeclarationRetentionPolicy(func(at time.Time) time.Time { return at.Add(24 * time.Hour) })
	consent := "consent-v1"

	// 1. NewDeclarationService validation
	if _, err := NewDeclarationService(nil, keys, ids, now, retention, consent); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil repo")
	}
	if _, err := NewDeclarationService(repo, nil, ids, now, retention, consent); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil keys")
	}
	if _, err := NewDeclarationService(repo, keys, nil, now, retention, consent); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil ids")
	}
	if _, err := NewDeclarationService(repo, keys, ids, nil, retention, consent); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil now")
	}
	if _, err := NewDeclarationService(repo, keys, ids, now, nil, consent); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil retention")
	}
	for _, badConsent := range []string{"", "   ", strings.Repeat("c", 21)} {
		if _, err := NewDeclarationService(repo, keys, ids, now, retention, badConsent); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for bad consent %q", badConsent)
		}
	}

	// Valid base service
	svc, err := NewDeclarationService(repo, keys, ids, now, retention, consent)
	if err != nil || svc == nil {
		t.Fatalf("unexpected error for valid service: %v", err)
	}

	// 2. NewDeclarationServiceWithRutHMAC and NewDeclarationServiceWithRutKeyring validation
	shortSecret := []byte("short")
	validSecret := []byte("synthetic-dedicated-hmac-key-16b")
	if _, err := NewDeclarationServiceWithRutHMAC(repo, keys, ids, now, retention, consent, shortSecret, "v1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for short secret")
	}
	for _, badVer := range []string{"", "   ", "v 1", "bad@ver", strings.Repeat("v", 33), "-leading-dash"} {
		if _, err := NewDeclarationServiceWithRutHMAC(repo, keys, ids, now, retention, consent, validSecret, badVer); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for bad version %q", badVer)
		}
	}
	hmacSvc, err := NewDeclarationServiceWithRutHMAC(repo, keys, ids, now, retention, consent, validSecret, "v1")
	if err != nil || hmacSvc == nil || hmacSvc.RutKeyring() == nil {
		t.Fatalf("unexpected failure constructing HMAC service: %v", err)
	}
	keyringSvc, err := NewDeclarationServiceWithRutKeyring(repo, keys, ids, now, retention, consent, hmacSvc.RutKeyring())
	if err != nil || keyringSvc == nil || keyringSvc.RutKeyring() == nil {
		t.Fatalf("unexpected failure constructing service with RutKeyring: %v", err)
	}

	// 3. RutKeyring on nil receiver
	var nilSvc *DeclarationService
	if nilSvc.RutKeyring() != nil {
		t.Fatal("expected nil RutKeyring for nil service")
	}
}

func TestDeclarationServiceSaveBoundaries(t *testing.T) {
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	retention := DeclarationRetentionPolicy(func(at time.Time) time.Time { return at.Add(24 * time.Hour) })
	svc, _ := newDeclarationServiceFixture(t, repo, retention)
	accountID := "a0000000-0000-4000-8000-000000000001"
	payload := DeclarationPayload{
		FirstName:    "Synthetic",
		LastName:     "Persona",
		DateOfBirth:  "1990-01-01",
		RUT:          "11.222.333-9",
		IDCountry:    "CHL",
		DocumentType: "ID",
	}

	// 1. Nil receiver
	var nilSvc *DeclarationService
	if _, err := nilSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil receiver in Save")
	}

	// 2. Non-canonical accountID
	for _, badAccount := range []string{"", "123", strings.ToUpper(accountID), "../escape", " " + accountID} {
		if _, err := svc.Save(context.Background(), badAccount, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for non-canonical accountID %q", badAccount)
		}
	}

	// 3. Consent false or version mismatch
	if _, err := svc.Save(context.Background(), accountID, payload, false, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for consent=false")
	}
	if _, err := svc.Save(context.Background(), accountID, payload, true, "consent-v2"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for mismatched consent version")
	}

	// 4. Multiple or invalid correlation IDs
	if _, err := svc.Save(context.Background(), accountID, payload, true, "consent-v1", "corr-1", "corr-2"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for multiple correlation IDs")
	}
	for _, badCorr := range []string{"", "  ", strings.Repeat("c", 121), " leading-space"} {
		if _, err := svc.Save(context.Background(), accountID, payload, true, "consent-v1", badCorr); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for bad correlation ID %q", badCorr)
		}
	}

	// 5. Invalid retention policy where retentionAt <= at
	badRetentionSvc, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(-time.Hour) })
	if _, err := badRetentionSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid when retentionAt is not after givenAt")
	}

	// 6. ID generator returning error or non-canonical UUID
	errIDSvc, _ := NewDeclarationServiceWithRutHMAC(repo, svc.keys, declarationIDFunc(func() (string, error) { return "", errors.New("uuid failure") }), svc.now, retention, "consent-v1", []byte("synthetic-dedicated-hmac-key"), "v1")
	if _, err := errIDSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid when ID generator fails")
	}
	badUUIDSvc, _ := NewDeclarationServiceWithRutHMAC(repo, svc.keys, declarationIDFunc(func() (string, error) { return "not-a-uuid", nil }), svc.now, retention, "consent-v1", []byte("synthetic-dedicated-hmac-key"), "v1")
	if _, err := badUUIDSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid when ID generator returns invalid UUID")
	}

	// 7. CreateRevision error propagation
	failingRepo := &mockFailingDeclarationRepo{createErr: errors.New("db insert error")}
	failingSvc, _ := NewDeclarationService(failingRepo, svc.keys, svc.ids, svc.now, retention, "consent-v1")
	if _, err := failingSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); err == nil || !strings.Contains(err.Error(), "db insert error") {
		t.Fatalf("expected db insert error, got: %v", err)
	}

	// 8. GetRevision readback error and readback validation mismatch
	failingRepo.createErr = nil
	failingRepo.getRevErr = errors.New("readback query failed")
	if _, err := failingSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); err == nil || !strings.Contains(err.Error(), "readback query failed") {
		t.Fatalf("expected readback query failed, got: %v", err)
	}

	failingRepo.getRevErr = nil
	at := svc.now().UTC().Truncate(time.Microsecond)
	retAt := retention(at).UTC().Truncate(time.Microsecond)
	baseRow := DeclarationRevision{
		ID:             "85000000-0000-4000-8000-000000000011",
		AccountID:      accountID,
		Revision:       1,
		SchemaVersion:  "1",
		ConsentVersion: "consent-v1",
		ConsentGivenAt: at,
		RetentionAt:    retAt,
	}
	failingRepo.createdRow = DeclarationRevision{Revision: 1}

	mismatches := []struct {
		name string
		row  DeclarationRevision
	}{
		{"mismatched ID", func() DeclarationRevision { r := baseRow; r.ID = "wrong-id"; return r }()},
		{"mismatched AccountID", func() DeclarationRevision { r := baseRow; r.AccountID = "wrong-acc"; return r }()},
		{"zero revision", func() DeclarationRevision { r := baseRow; r.Revision = 0; return r }()},
		{"mismatched SchemaVersion", func() DeclarationRevision { r := baseRow; r.SchemaVersion = "2"; return r }()},
		{"mismatched ConsentVersion", func() DeclarationRevision { r := baseRow; r.ConsentVersion = "consent-v2"; return r }()},
		{"mismatched ConsentGivenAt", func() DeclarationRevision { r := baseRow; r.ConsentGivenAt = at.Add(time.Second); return r }()},
		{"mismatched RetentionAt", func() DeclarationRevision { r := baseRow; r.RetentionAt = retAt.Add(time.Second); return r }()},
		{"mismatched created revision", func() DeclarationRevision { r := baseRow; r.Revision = 2; return r }()},
	}
	for _, tc := range mismatches {
		t.Run(tc.name, func(t *testing.T) {
			failingRepo.getRow = tc.row
			if _, err := failingSvc.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected ErrInvalid for %s, got: %v", tc.name, err)
			}
		})
	}
}

type mockFailingDeclarationRepo struct {
	createErr     error
	getRevErr     error
	getCurrentErr error
	createdRow    DeclarationRevision
	getRow        DeclarationRevision
	getCurrentRow DeclarationRevision
}

func (m *mockFailingDeclarationRepo) CreateRevision(_ context.Context, _ DeclarationRevisionInput, _ DeclarationSealer) (DeclarationRevision, error) {
	if m.createErr != nil {
		return DeclarationRevision{}, m.createErr
	}
	return m.createdRow, nil
}

func (m *mockFailingDeclarationRepo) GetRevision(_ context.Context, _, _ string) (DeclarationRevision, error) {
	if m.getRevErr != nil {
		return DeclarationRevision{}, m.getRevErr
	}
	return m.getRow, nil
}

func (m *mockFailingDeclarationRepo) GetCurrent(_ context.Context, _ string) (DeclarationRevision, error) {
	if m.getCurrentErr != nil {
		return DeclarationRevision{}, m.getCurrentErr
	}
	return m.getCurrentRow, nil
}

func TestDeclarationServiceGetAndDecodeBoundaries(t *testing.T) {
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	retention := DeclarationRetentionPolicy(func(at time.Time) time.Time { return at.Add(24 * time.Hour) })
	svc, _ := newDeclarationServiceFixture(t, repo, retention)
	accountID := "a0000000-0000-4000-8000-000000000001"
	revID := "85000000-0000-4000-8000-000000000011"

	// 1. Nil receiver
	var nilSvc *DeclarationService
	if _, err := nilSvc.GetCurrent(context.Background(), accountID); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil receiver in GetCurrent")
	}
	if _, err := nilSvc.GetRevision(context.Background(), accountID, revID); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for nil receiver in GetRevision")
	}

	// 2. Non-canonical UUIDs
	if _, err := svc.GetCurrent(context.Background(), "bad-id"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for non-canonical accountID in GetCurrent")
	}
	if _, err := svc.GetRevision(context.Background(), "bad-acc", revID); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for non-canonical accountID in GetRevision")
	}
	if _, err := svc.GetRevision(context.Background(), accountID, "bad-rev"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for non-canonical revID in GetRevision")
	}

	// Repository errors and mismatch in GetCurrent and GetRevision
	failingRepo := &mockFailingDeclarationRepo{}
	failingSvc, _ := NewDeclarationService(failingRepo, svc.keys, svc.ids, svc.now, retention, "consent-v1")

	failingRepo.getCurrentErr = errors.New("db query error in GetCurrent")
	if _, err := failingSvc.GetCurrent(context.Background(), accountID); err == nil || !strings.Contains(err.Error(), "db query error in GetCurrent") {
		t.Fatalf("expected db query error in GetCurrent, got: %v", err)
	}
	failingRepo.getCurrentErr = nil
	failingRepo.getCurrentRow = DeclarationRevision{AccountID: "different-account", ID: revID, Revision: 1, SchemaVersion: "1", RetentionAt: time.Now().Add(time.Hour)}
	if _, err := failingSvc.GetCurrent(context.Background(), accountID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when GetCurrent row has mismatched AccountID, got: %v", err)
	}

	failingRepo.getRevErr = errors.New("db query error in GetRevision")
	if _, err := failingSvc.GetRevision(context.Background(), accountID, revID); err == nil || !strings.Contains(err.Error(), "db query error in GetRevision") {
		t.Fatalf("expected db query error in GetRevision, got: %v", err)
	}
	failingRepo.getRevErr = nil
	failingRepo.getRow = DeclarationRevision{AccountID: accountID, ID: "different-rev-id", Revision: 1, SchemaVersion: "1", RetentionAt: time.Now().Add(time.Hour)}
	if _, err := failingSvc.GetRevision(context.Background(), accountID, revID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound when GetRevision row has mismatched ID, got: %v", err)
	}

	// 3. decode with invalid schema version or non-positive revision
	badRevRow := DeclarationRevision{
		ID:            revID,
		AccountID:     accountID,
		Revision:      0,
		SchemaVersion: "1",
		RetentionAt:   time.Now().Add(time.Hour),
	}
	if _, err := svc.decode(badRevRow); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for zero revision")
	}
	badSchemaRow := DeclarationRevision{
		ID:            revID,
		AccountID:     accountID,
		Revision:      1,
		SchemaVersion: "unknown-schema",
		RetentionAt:   time.Now().Add(time.Hour),
	}
	if _, err := svc.decode(badSchemaRow); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for unknown schema version")
	}

	// 4. decode with zero or expired retention
	expiredRow := DeclarationRevision{
		ID:            revID,
		AccountID:     accountID,
		Revision:      1,
		SchemaVersion: "1",
		RetentionAt:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := svc.decode(expiredRow); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for expired retention")
	}
	zeroRetentionRow := DeclarationRevision{
		ID:            revID,
		AccountID:     accountID,
		Revision:      1,
		SchemaVersion: "1",
		RetentionAt:   time.Time{},
	}
	if _, err := svc.decode(zeroRetentionRow); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for zero retention")
	}

	// 5. decode with decryption failure
	badKeyring := &mockKeyringForBoundaries{openErr: errors.New("aead decryption failed")}
	decFailSvc, _ := NewDeclarationService(repo, badKeyring, svc.ids, svc.now, retention, "consent-v1")
	validCipherRow := DeclarationRevision{
		ID:            revID,
		AccountID:     accountID,
		Revision:      1,
		KeyVersion:    "v1",
		Ciphertext:    []byte("corrupt-cipher"),
		SchemaVersion: "1",
		RetentionAt:   time.Now().Add(24 * time.Hour),
	}
	if _, err := decFailSvc.decode(validCipherRow); err == nil || !strings.Contains(err.Error(), "declaration unavailable") {
		t.Fatalf("expected declaration unavailable error, got: %v", err)
	}

	// 6. decode with invalid payload JSON
	corruptJSONKeyring := &mockKeyringForBoundaries{openData: []byte("not-json")}
	corruptJSONSvc, _ := NewDeclarationService(repo, corruptJSONKeyring, svc.ids, svc.now, retention, "consent-v1")
	if _, err := corruptJSONSvc.decode(validCipherRow); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for corrupted payload JSON, got: %v", err)
	}
}
