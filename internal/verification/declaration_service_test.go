package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

type declarationIDFunc func() (string, error)

func (f declarationIDFunc) ID() (string, error) { return f() }

type declarationMemoryRepository struct {
	rows              map[string]DeclarationRevision
	lastInput         DeclarationRevisionInput
	createCalls       int
	readRevisionCalls int
	readCurrentCalls  int
	getRevisionErr    error
	truncateTimes     bool
}

func (r *declarationMemoryRepository) CreateRevision(_ context.Context, input DeclarationRevisionInput, seal DeclarationSealer) (DeclarationRevision, error) {
	r.createCalls++
	r.lastInput = input
	if r.truncateTimes {
		input.ConsentGivenAt = input.ConsentGivenAt.Truncate(time.Microsecond)
		input.RetentionAt = input.RetentionAt.Truncate(time.Microsecond)
	}
	var revision int64 = 1
	for _, row := range r.rows {
		if row.AccountID == input.AccountID && row.Revision >= revision {
			revision = row.Revision + 1
		}
	}
	keyVersion, ciphertext, err := seal(revision)
	if err != nil {
		return DeclarationRevision{}, err
	}
	row := DeclarationRevision{ID: input.ID, AccountID: input.AccountID, Revision: revision, KeyVersion: keyVersion, Ciphertext: ciphertext, SchemaVersion: input.SchemaVersion, ConsentVersion: input.ConsentVersion, ConsentGivenAt: input.ConsentGivenAt, RetentionAt: input.RetentionAt, CreatedAt: input.ConsentGivenAt}
	r.rows[row.ID] = row
	return row, nil
}

func (r *declarationMemoryRepository) GetCurrent(_ context.Context, accountID string) (DeclarationRevision, error) {
	r.readCurrentCalls++
	var found DeclarationRevision
	for _, row := range r.rows {
		if row.AccountID == accountID && row.Revision > found.Revision {
			found = row
		}
	}
	if found.ID == "" {
		return DeclarationRevision{}, ErrNotFound
	}
	return found, nil
}

func (r *declarationMemoryRepository) GetRevision(_ context.Context, accountID, id string) (DeclarationRevision, error) {
	r.readRevisionCalls++
	if r.getRevisionErr != nil {
		return DeclarationRevision{}, r.getRevisionErr
	}
	row, ok := r.rows[id]
	if !ok || row.AccountID != accountID {
		return DeclarationRevision{}, ErrNotFound
	}
	return row, nil
}

func newDeclarationServiceFixture(t *testing.T, repo *declarationMemoryRepository, retention DeclarationRetentionPolicy) (*DeclarationService, time.Time) {
	t.Helper()
	const declarationID = "85000000-0000-4000-8000-000000000011"
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewDeclarationServiceWithRutHMAC(repo, keyring, declarationIDFunc(func() (string, error) { return declarationID, nil }), func() time.Time { return now }, retention, "consent-v1", []byte("synthetic-dedicated-hmac-key"), "v1")
	if err != nil {
		t.Fatal(err)
	}
	return service, now
}

func TestDeclarationServiceRejectsMinorBeforePersistence(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "2009-11-09", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Save error = %v, want ErrInvalid", err)
	}
	if repo.createCalls != 0 || len(repo.rows) != 0 {
		t.Fatalf("minor declaration persisted: calls=%d rows=%d", repo.createCalls, len(repo.rows))
	}
}

func TestDeclarationConsentVersionRetainsExistingContract(t *testing.T) {
	for _, version := range []string{"consent-v1", "versión-1", "consent v1"} {
		if !validDeclarationConsentVersion(version) {
			t.Error("existing consent token rejected")
		}
	}
	for _, version := range []string{"", " v1", "v1 ", strings.Repeat("a", 21)} {
		if validDeclarationConsentVersion(version) {
			t.Error("invalid consent token accepted")
		}
	}
}

func TestLegacyDeclarationServiceSaveDoesNotRequireRutFingerprintConfiguration(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	const declarationID = "85000000-0000-4000-8000-000000000011"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	service, err := NewDeclarationService(repo, keyring, declarationIDFunc(func() (string, error) { return declarationID, nil }), func() time.Time { return now }, func(at time.Time) time.Time { return at.Add(time.Hour) }, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); err != nil {
		t.Fatalf("legacy Save: %v", err)
	}
	if repo.createCalls != 1 || repo.lastInput.RutFingerprint != "" {
		t.Fatalf("legacy repository input: calls=%d fingerprint=%q", repo.createCalls, repo.lastInput.RutFingerprint)
	}
}

func TestDeclarationServiceWithRutHMACRejectsInvalidConfigurationWithoutLeakingSecrets(t *testing.T) {
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	base := []any{repo, keyring, declarationIDFunc(func() (string, error) { return "85000000-0000-4000-8000-000000000011", nil }), func() time.Time { return time.Now() }, DeclarationRetentionPolicy(func(at time.Time) time.Time { return at.Add(time.Hour) }), "consent-v1"}
	for _, tc := range []struct {
		name    string
		secret  []byte
		version string
	}{
		{"missing secret", nil, "v1"}, {"short secret", []byte("short"), "v1"},
		{"empty version", []byte("synthetic-dedicated-hmac-key"), ""},
		{"leading punctuation", []byte("synthetic-dedicated-hmac-key"), "-v1"},
		{"whitespace", []byte("synthetic-dedicated-hmac-key"), "v 1"},
		{"non-ascii", []byte("synthetic-dedicated-hmac-key"), "vé"},
		{"too long", []byte("synthetic-dedicated-hmac-key"), "123456789012345678901234567890123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDeclarationServiceWithRutHMAC(base[0].(DeclarationRevisionRepository), base[1].(DeclarationKeyring), base[2].(IDGenerator), base[3].(func() time.Time), base[4].(DeclarationRetentionPolicy), base[5].(string), tc.secret, tc.version)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("constructor error=%v; want ErrInvalid", err)
			}
			if err != nil && ((len(tc.secret) > 0 && strings.Contains(err.Error(), string(tc.secret))) || (tc.version != "" && strings.Contains(err.Error(), tc.version))) {
				t.Fatalf("configuration error leaked input: %v", err)
			}
		})
	}
}

func TestDeclarationServiceSavePersistsAndReadsBackEncryptedRevision(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, now := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(24 * time.Hour) })
	got, err := service.Save(context.Background(), accountID, DeclarationPayload{FirstName: "Synthetic", LastName: "Pérez", DateOfBirth: "1990-01-02", RUT: "11.222.333-9", IDCountry: "CHL", DocumentType: "ID"}, true, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "85000000-0000-4000-8000-000000000011" || got.Revision != 1 || got.Payload.RUT != "11222333-9" || got.Payload.FirstName != "Synthetic" || got.ConsentVersion != "consent-v1" || !got.ConsentGivenAt.Equal(now) {
		t.Fatalf("saved declaration mismatch: %#v", got)
	}
	if repo.createCalls != 1 || repo.readRevisionCalls != 1 {
		t.Fatalf("save/readback calls = %d/%d, want 1/1", repo.createCalls, repo.readRevisionCalls)
	}
	if repo.readCurrentCalls != 0 {
		t.Fatalf("save used current-row read instead of exact independent readback: %d", repo.readCurrentCalls)
	}
}

func TestDeclarationServiceNormalizesTimestampsToPostgresPrecision(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	const declarationID = "85000000-0000-4000-8000-000000000011"
	now := time.Date(2026, time.November, 8, 12, 0, 0, 123456789, time.UTC)
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision), truncateTimes: true}
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewDeclarationServiceWithRutHMAC(repo, keyring, declarationIDFunc(func() (string, error) { return declarationID, nil }), func() time.Time { return now }, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "consent-v1", []byte("synthetic-dedicated-hmac-key"), "v1")
	if err != nil {
		t.Fatal(err)
	}
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	got, err := service.Save(context.Background(), accountID, payload, true, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.ConsentGivenAt.Equal(now.Truncate(time.Microsecond)) {
		t.Fatalf("consent timestamp=%s; want database precision %s", got.ConsentGivenAt, now.Truncate(time.Microsecond))
	}
}

func TestDeclarationServiceRequiresConsentAndExplicitRetention(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	valid := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	for _, tc := range []struct {
		name      string
		consent   bool
		retention DeclarationRetentionPolicy
	}{
		{name: "consent missing", consent: false, retention: func(at time.Time) time.Time { return at.Add(time.Hour) }},
		{name: "retention not after consent", consent: true, retention: func(at time.Time) time.Time { return at }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
			service, _ := newDeclarationServiceFixture(t, repo, tc.retention)
			if _, err := service.Save(context.Background(), accountID, valid, tc.consent, "consent-v1"); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Save error=%v; want ErrInvalid", err)
			}
			if repo.createCalls != 0 {
				t.Fatalf("invalid declaration reached repository %d times", repo.createCalls)
			}
		})
	}
}

func TestDeclarationServiceRejectsStaleConsentVersionBeforeWriting(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v0"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale consent version error=%v; want ErrInvalid", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("stale consent version reached repository %d times", repo.createCalls)
	}
}

func TestDeclarationServicePassesServerCorrelationIDForAudit(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	const correlationID = "request-correlation-synthetic"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1", correlationID); err != nil {
		t.Fatal(err)
	}
	if repo.lastInput.CorrelationID != correlationID {
		t.Fatalf("repository correlation ID=%q; want %q", repo.lastInput.CorrelationID, correlationID)
	}
}

func TestDeclarationServicePassesCanonicalRutFingerprintToAtomicRevisionInput(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	for _, tc := range []struct{ account, rut string }{
		{accountID, "11.222.333-9"},
		{"85000000-0000-4000-8000-000000000002", "11222333-9"},
	} {
		repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
		service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
		service, err := NewDeclarationServiceWithRutHMAC(repo, service.keys, service.ids, service.now, service.retention, service.consentVersion, []byte("synthetic-dedicated-hmac-key"), "v1")
		if err != nil {
			t.Fatal(err)
		}
		payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: tc.rut, IDCountry: "CHL", DocumentType: "ID"}
		if _, err := service.Save(context.Background(), tc.account, payload, true, "consent-v1"); err != nil {
			t.Fatal(err)
		}
		want, err := ComputeRutFingerprint("11222333-9", []byte("synthetic-dedicated-hmac-key"), "v1")
		if err != nil {
			t.Fatal(err)
		}
		if repo.lastInput.RutFingerprint != want {
			t.Fatalf("fingerprint=%q want %q", repo.lastInput.RutFingerprint, want)
		}
	}
}

func TestDeclarationServiceRejectsInvalidRutBeforeRepositoryInvocation(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-0", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Save error=%v; want ErrInvalid", err)
	}
	if repo.createCalls != 0 || len(repo.rows) != 0 {
		t.Fatalf("invalid RUT reached repository: calls=%d rows=%d", repo.createCalls, len(repo.rows))
	}
}

func TestDeclarationServiceDoesNotReportSuccessWithoutExactReadback(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision), getRevisionErr: errors.New("synthetic readback unavailable")}
	service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); err == nil {
		t.Fatal("reported success without exact persisted readback")
	}
	if repo.createCalls != 1 || repo.readRevisionCalls != 1 {
		t.Fatalf("save/readback calls = %d/%d, want 1/1", repo.createCalls, repo.readRevisionCalls)
	}
}

func TestDeclarationServiceGetCurrentDecryptsOwnerScopedSnapshot(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetCurrent(context.Background(), accountID)
	if err != nil || got.Payload != payload {
		t.Fatalf("GetCurrent = %#v, %v", got, err)
	}
	if repo.readCurrentCalls != 1 {
		t.Fatalf("current declaration reads=%d, want 1", repo.readCurrentCalls)
	}
}

func TestDeclarationServiceDoesNotReadExpiredDeclaration(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	service, now := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(time.Hour) })
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := service.GetCurrent(context.Background(), accountID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired declaration read error=%v; want ErrNotFound", err)
	}
}

func TestDeclarationServiceKeyringSavePassesActiveAndAllCandidates(t *testing.T) {
	const accountID = "85000000-0000-4000-8000-000000000001"
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	rutKeys, err := NewRutHMACKeyring("v2", map[string][]byte{"v1": []byte("synthetic-key-v1-material"), "v2": []byte("synthetic-key-v2-material")})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewDeclarationServiceWithRutKeyring(repo, keyring, declarationIDFunc(func() (string, error) { return "85000000-0000-4000-8000-000000000011", nil }), func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }, func(at time.Time) time.Time { return at.Add(time.Hour) }, "consent-v1", rutKeys)
	if err != nil {
		t.Fatal(err)
	}
	payload := DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := service.Save(context.Background(), accountID, payload, true, "consent-v1"); err != nil {
		t.Fatal(err)
	}
	want, err := rutKeys.Fingerprints("11222333-9")
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastInput.RutFingerprint != want.Active || len(repo.lastInput.RutFingerprintCandidates) != len(want.Candidates) {
		t.Fatalf("persisted fingerprints = %q/%v, want %q/%v", repo.lastInput.RutFingerprint, repo.lastInput.RutFingerprintCandidates, want.Active, want.Candidates)
	}
	for i := range want.Candidates {
		if repo.lastInput.RutFingerprintCandidates[i] != want.Candidates[i] {
			t.Fatalf("candidate set=%v want %v", repo.lastInput.RutFingerprintCandidates, want.Candidates)
		}
	}
}

func TestDeclarationServiceKeyringConstructorRejectsMissingKeyring(t *testing.T) {
	repo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	base := []any{repo, keyring, declarationIDFunc(func() (string, error) { return "85000000-0000-4000-8000-000000000011", nil }), func() time.Time { return time.Now() }, DeclarationRetentionPolicy(func(at time.Time) time.Time { return at.Add(time.Hour) }), "consent-v1"}
	for _, tc := range []struct {
		name string
		keys *RutHMACKeyring
	}{{"nil", nil}, {"zero", &RutHMACKeyring{}}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewDeclarationServiceWithRutKeyring(base[0].(DeclarationRevisionRepository), base[1].(DeclarationKeyring), base[2].(IDGenerator), base[3].(func() time.Time), base[4].(DeclarationRetentionPolicy), base[5].(string), tc.keys); !errors.Is(err, ErrInvalid) {
				t.Fatalf("error=%v want ErrInvalid", err)
			}
		})
	}
}
