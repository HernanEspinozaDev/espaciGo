package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type runtimeRotationDeclarations struct {
	*runtimeDeclarationRepo
	input verification.DeclarationRevisionInput
}

func (r *runtimeRotationDeclarations) CreateRevision(ctx context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	r.input = in
	return r.runtimeDeclarationRepo.CreateRevision(ctx, in, seal)
}

func runtimeRotationEnvironment(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	aes := filepath.Join(dir, "declaration.json")
	writeKeyring(t, aes, "v1", map[string]string{"v1": hex.EncodeToString(bytes.Repeat([]byte{1}, 32))})
	path := filepath.Join(dir, "rut.json")
	body, err := json.Marshal(map[string]any{"active": "v2", "keys": map[string]string{"v1": hex.EncodeToString(bytes.Repeat([]byte{2}, 32)), "v2": hex.EncodeToString(bytes.Repeat([]byte{3}, 32))}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"IDENTITY_DECLARATION_KEYRING_FILE": aes, "IDENTITY_RUT_HMAC_KEYRING_FILE": path, "DIDIT_API_KEY": "synthetic-runtime-key", "DIDIT_WORKFLOW_ID": "synthetic-workflow"}
}

func TestRuntimeIdentityRutKeyringFileSuppliesSaveCandidates(t *testing.T) {
	values := runtimeRotationEnvironment(t)
	repo := &runtimeRotationDeclarations{runtimeDeclarationRepo: &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}}
	now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	svc, err := newRuntimeIdentityDeclarationService(env(values), repo, now)
	if err != nil || svc == nil {
		t.Fatalf("configured private HMAC keyring was not accepted: %v", err)
	}
	if svc.RutKeyring() == nil {
		t.Fatal("runtime did not retain its HMAC snapshot")
	}
	if err := os.Remove(values["IDENTITY_RUT_HMAC_KEYRING_FILE"]); err != nil {
		t.Fatal(err)
	}
	payload := verification.DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-9", IDCountry: "CHL", DocumentType: "ID"}
	if _, err := svc.Save(context.Background(), "11111111-1111-4111-8111-111111111111", payload, true, defaultIdentityDeclarationConsentVersion); err != nil {
		t.Fatal(err)
	}
	want, err := svc.RutKeyring().Fingerprints(payload.RUT)
	if err != nil {
		t.Fatal(err)
	}
	if repo.input.RutFingerprint != want.Active || !reflect.DeepEqual(repo.input.RutFingerprintCandidates, want.Candidates) || len(want.Candidates) != 2 {
		t.Fatal("runtime Save did not use active and historical candidates from its captured keyring")
	}
}

type runtimeRotationClaims struct {
	*verification.MemoryRutClaimRegistry
	observed verification.RutFingerprintSet
}

func (r *runtimeRotationClaims) ClaimCandidates(ctx context.Context, a, d string, set verification.RutFingerprintSet, expiry, now time.Time) (string, error) {
	r.observed = set
	return set.Active, nil
}
func (r *runtimeRotationClaims) ResolveCandidates(context.Context, string, string, verification.RutFingerprintSet) (string, error) {
	return "", verification.ErrConflict
}

type runtimeRotationStopRepo struct {
	runtimeAttemptRepo
	calls int
}

func (r *runtimeRotationStopRepo) PrepareAttempt(context.Context, verification.PrepareAttemptInput) (verification.ProviderAttempt, bool, error) {
	r.calls++
	return verification.ProviderAttempt{}, false, verification.ErrInvalid
}

func TestLegacyDiditTestFactoryUsesDeclarationRutSnapshotWithoutReload(t *testing.T) {
	values := runtimeRotationEnvironment(t)
	now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	repo := &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}
	declarations, err := newRuntimeIdentityDeclarationService(env(values), repo, now)
	if err != nil || declarations == nil {
		t.Fatalf("configured keyring: %v", err)
	}
	payload := verification.DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-9", IDCountry: "CHL", DocumentType: "ID"}
	account := "11111111-1111-4111-8111-111111111111"
	if _, err := declarations.Save(context.Background(), account, payload, true, defaultIdentityDeclarationConsentVersion); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(values["IDENTITY_RUT_HMAC_KEYRING_FILE"]); err != nil {
		t.Fatal(err)
	}
	store := &runtimeRotationClaims{MemoryRutClaimRegistry: verification.NewMemoryRutClaimRegistry()}
	attempts := &runtimeRotationStopRepo{}
	didit, err := newTestDiditService(env(values), attempts, declarations, now, store, &fakeSessionDeletionQueue{})
	if err != nil || didit == nil {
		t.Fatalf("Didit must reuse declaration keyring rather than reload its removed source file: %v", err)
	}
	_, err = didit.StartOrResume(context.Background(), account, "snapshot-reuse", verification.StartDiditIntentInput{ConsentGiven: true, ConsentVersion: defaultDiditConsentVersion})
	want, _ := declarations.RutKeyring().Fingerprints(payload.RUT)
	if !errors.Is(err, verification.ErrInvalid) || attempts.calls != 1 || !reflect.DeepEqual(store.observed, want) {
		t.Fatal("Didit did not reach PrepareAttempt using the exact declaration keyset")
	}
}

func TestRuntimeIdentityRutRejectsAmbiguousFileAndLegacySecret(t *testing.T) {
	values := runtimeRotationEnvironment(t)
	values["IDENTITY_RUT_HMAC_SECRET"] = "synthetic-secret-of-at-least-32-bytes"
	svc, err := newRuntimeIdentityDeclarationService(env(values), &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}, time.Now)
	if err == nil || svc != nil {
		t.Fatal("runtime accepted ambiguous HMAC configuration sources")
	}
}
