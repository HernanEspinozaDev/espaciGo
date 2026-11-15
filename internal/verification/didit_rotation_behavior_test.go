package verification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

const (
	rotationAccountA = "11111111-1111-4111-8111-111111111111"
	rotationAccountB = "22222222-2222-4222-8222-222222222222"
	rotationSession  = "didit-rotation-session"
)

var rotationNow = time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC)
var rotationHMACKeys = map[string][]byte{
	"v1": []byte("0123456789abcdef0123456789abcdef"),
	"v2": []byte("abcdef0123456789abcdef0123456789"),
}

func rotationFixture(t *testing.T, claims RutClaimStore, repo ProviderAttemptRepository, client *fakeDiditClient) (*DiditService, *DeclarationService, *declarationMemoryRepository, *RutHMACKeyring) {
	t.Helper()
	ids := &counterUUIDs{}
	declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	declKeys := newTestKeyring(t)
	declarations, err := NewDeclarationService(declRepo, declKeys, ids, func() time.Time { return rotationNow }, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	decisionKeys, err := declarationcrypto.New("v1", map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	ring, err := NewRutHMACKeyring("v2", rotationHMACKeys)
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		repo = newMemProviderAttemptRepo()
	}
	if client == nil {
		client = &fakeDiditClient{createResp: CreateSessionResponse{SessionID: rotationSession, SessionURL: "https://verify.didit.me/session/rotation"}}
	}
	service, err := NewDiditService(repo, declarations, client, decisionKeys, ids, func() time.Time { return rotationNow }, DiditServiceConfig{
		Environment: "SANDBOX", ApplicationID: "rotation-test", WorkflowID: "wf-chile-id", WorkflowVersion: "1", ConsentVersion: "consent-v1", AttemptTTL: time.Hour, RutHMACKeyring: ring,
	}, claims)
	if err != nil {
		t.Fatal(err)
	}
	return service, declarations, declRepo, ring
}

func saveRotationDeclaration(t *testing.T, svc *DeclarationService, account string) SavedDeclaration {
	t.Helper()
	revision, err := svc.Save(context.Background(), account, DeclarationPayload{FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-02", RUT: "12.345.670-K", IDCountry: "CHL", DocumentType: "ID"}, true, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	return revision
}

func startRotation(t *testing.T, svc *DiditService, account, key string) (StartedDiditIntent, error) {
	t.Helper()
	return svc.StartOrResume(context.Background(), account, key, StartDiditIntentInput{ConsentGiven: true, ConsentVersion: "consent-v1"})
}

func TestDiditStartRetainsLegacyClaimFingerprintAcrossRotation(t *testing.T) {
	claims := NewMemoryRutClaimRegistry()
	repo := newMemProviderAttemptRepo()
	client := &fakeDiditClient{createResp: CreateSessionResponse{SessionID: rotationSession, SessionURL: "https://verify.didit.me/session/rotation"}}
	svc, declarations, _, ring := rotationFixture(t, claims, repo, client)
	declA := saveRotationDeclaration(t, declarations, rotationAccountA)
	_ = saveRotationDeclaration(t, declarations, rotationAccountB)
	set, err := ring.Fingerprints("12.345.670-K")
	if err != nil {
		t.Fatal(err)
	}
	legacy := ""
	for _, candidate := range set.Candidates {
		if candidate[:3] == "v1:" {
			legacy = candidate
		}
	}
	if legacy == "" {
		t.Fatal("fixture has no v1 fingerprint")
	}
	claims.mu.Lock()
	claims.byDigest[legacy] = rutClaimEntry{AccountID: rotationAccountA, DeclarationID: declA.ID, RutFingerprint: legacy, DigestHex: legacy[3:], ExpiresAt: rotationNow.Add(time.Hour)}
	claims.mu.Unlock()
	if _, err := startRotation(t, svc, rotationAccountA, "a-start"); err != nil {
		t.Fatalf("start A: %v", err)
	}
	claims.mu.Lock()
	_, legacyRetained := claims.byDigest[legacy]
	claims.mu.Unlock()
	if !legacyRetained {
		t.Fatal("Start did not retain legacy v1 canonical claim")
	}
	beforePrepared, beforeCreated := len(repo.order), len(client.created)
	if _, err := startRotation(t, svc, rotationAccountB, "b-start"); !errors.Is(err, ErrConflict) {
		t.Fatalf("start B error=%v, want ErrConflict", err)
	}
	if len(repo.order) != beforePrepared || len(client.created) != beforeCreated {
		t.Fatalf("conflicting start reached provider preparation: prepared %d->%d, create %d->%d", beforePrepared, len(repo.order), beforeCreated, len(client.created))
	}
}

type rotationRecordingRepo struct {
	*memProviderAttemptRepo
	commits []CommitWebhookDecisionInput
}

func (r *rotationRecordingRepo) CommitWebhookDecision(ctx context.Context, in CommitWebhookDecisionInput) (ProviderAttempt, error) {
	r.commits = append(r.commits, in)
	return r.memProviderAttemptRepo.CommitWebhookDecision(ctx, in)
}

type rotationFailingClaims struct {
	*MemoryRutClaimRegistry
	claimErr, resolveErr error
}

func (r *rotationFailingClaims) ClaimCandidates(ctx context.Context, a, d string, set RutFingerprintSet, expiry, now time.Time) (string, error) {
	if r.claimErr != nil {
		return "", r.claimErr
	}
	return r.MemoryRutClaimRegistry.ClaimCandidates(ctx, a, d, set, expiry, now)
}
func (r *rotationFailingClaims) ResolveCandidates(ctx context.Context, a, d string, set RutFingerprintSet) (string, error) {
	if r.resolveErr != nil {
		return "", r.resolveErr
	}
	return r.MemoryRutClaimRegistry.ResolveCandidates(ctx, a, d, set)
}

func TestDiditApprovedWebhookCommitsResolvedLegacyFingerprintAndPurges(t *testing.T) {
	claims := NewMemoryRutClaimRegistry()
	baseRepo := newMemProviderAttemptRepo()
	repo := &rotationRecordingRepo{memProviderAttemptRepo: baseRepo}
	client := &fakeDiditClient{createResp: CreateSessionResponse{SessionID: rotationSession, SessionURL: "https://verify.didit.me/session/rotation"}}
	svc, declarations, declRepo, ring := rotationFixture(t, claims, repo, client)
	decl := saveRotationDeclaration(t, declarations, rotationAccountA)
	baseRepo.claims, baseRepo.declarations = claims, declRepo
	set, _ := ring.Fingerprints("12.345.670-K")
	legacy := set.Candidates[0]
	if legacy[:3] != "v1:" {
		legacy = set.Candidates[1]
	}
	claims.mu.Lock()
	claims.byDigest[legacy] = rutClaimEntry{AccountID: rotationAccountA, DeclarationID: decl.ID, RutFingerprint: legacy, DigestHex: legacy[3:], ExpiresAt: rotationNow.Add(time.Hour)}
	claims.mu.Unlock()
	started, err := startRotation(t, svc, rotationAccountA, "approved")
	if err != nil {
		t.Fatal(err)
	}
	decision := DiditDecisionResponse{SessionID: rotationSession, WorkflowID: "wf-chile-id", Status: "Approved", IDVerifications: []DiditIDVerificationNode{{Status: "Approved", FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-02", IdentificationNumber: "12.345.670-K", IssuingState: "CHL", DocumentType: "Identity Card", ExpirationDate: "2030-01-01"}}, LivenessChecks: []DiditLivenessNode{{Status: "Approved", Score: 98.2}}, FaceMatches: []DiditFaceMatchNode{{Status: "Approved", Score: 95.4}}}
	control := &IdentityVerificationAttempt{ID: started.ID, AccountID: rotationAccountA, IDCountry: "CHL", DocumentType: "ID", DeclaredFirstName: "Ana", DeclaredLastName: "Pérez", DeclaredDateOfBirth: "1990-01-02", DeclaredRUTNormalized: "12345670-K", DiditSessionID: rotationSession, WorkflowID: "wf-chile-id"}
	if status, _, _ := EvaluateDecision(control, &decision); status != "APPROVED" {
		t.Fatalf("fixture independently evaluates to %q", status)
	}
	client.decision = decision
	hash := sha256.Sum256([]byte("approved-rotation-event"))
	out, err := svc.ProcessWebhook(context.Background(), DiditWebhookInput{EventID: "approved-rotation-event", EventType: "status.updated", ProviderSessionID: rotationSession, TopLevelStatus: "Approved", BodySHA256: hash[:]})
	if err != nil {
		t.Fatal(err)
	}
	if client.decisionGets != 1 {
		t.Fatalf("authoritative GetSessionDecision calls=%d, want 1", client.decisionGets)
	}
	if len(repo.commits) != 1 || repo.commits[0].RutFingerprint != legacy {
		t.Fatalf("commit fingerprint=%v, want legacy %s", repo.commits, legacy)
	}
	if out.AttemptStatus != "APPROVED" {
		t.Fatalf("authoritative approved decision outcome=%+v", out)
	}
	if !out.RemoteDeleted || len(client.deleted) != 1 || client.deleted[0] != rotationSession {
		t.Fatalf("session delete missing: outcome=%+v deleted=%v", out, client.deleted)
	}
	stored := baseRepo.attempts[started.ID]
	if stored.Status != "APPROVED" || stored.DeclarationID != "" || len(stored.EncryptedDecision) != 0 {
		t.Fatalf("verified attempt not purged: %+v", stored)
	}
	claims.mu.Lock()
	verified := claims.byDigest[legacy].Verified
	claims.mu.Unlock()
	if !verified {
		t.Fatal("real MemoryRutClaimRegistry.MarkVerified was not committed")
	}
	if _, ok := declRepo.rows[decl.ID]; ok {
		t.Fatal("declaration row was not purged")
	}
}

func TestDiditRotationClaimFailuresStopBeforeSideEffects(t *testing.T) {
	t.Run("claim", func(t *testing.T) {
		claims := &rotationFailingClaims{MemoryRutClaimRegistry: NewMemoryRutClaimRegistry(), claimErr: ErrRutHMACKeyUnavailable}
		repo := newMemProviderAttemptRepo()
		client := &fakeDiditClient{createResp: CreateSessionResponse{SessionID: rotationSession, SessionURL: "https://verify.didit.me/session/rotation"}}
		svc, declarations, _, _ := rotationFixture(t, claims, repo, client)
		_ = saveRotationDeclaration(t, declarations, rotationAccountA)
		if _, err := startRotation(t, svc, rotationAccountA, "claim-failure"); !errors.Is(err, ErrRutHMACKeyUnavailable) {
			t.Fatalf("start error=%v", err)
		}
		if len(repo.order) != 0 || len(client.created) != 0 {
			t.Fatalf("claim failure had side effects: prepared=%d created=%d", len(repo.order), len(client.created))
		}
	})
	t.Run("resolve", func(t *testing.T) {
		claims := &rotationFailingClaims{MemoryRutClaimRegistry: NewMemoryRutClaimRegistry(), resolveErr: ErrRutHMACKeyUnavailable}
		baseRepo := newMemProviderAttemptRepo()
		repo := &rotationRecordingRepo{memProviderAttemptRepo: baseRepo}
		client := &fakeDiditClient{createResp: CreateSessionResponse{SessionID: rotationSession, SessionURL: "https://verify.didit.me/session/rotation"}}
		svc, declarations, declRepo, _ := rotationFixture(t, claims, repo, client)
		decl := saveRotationDeclaration(t, declarations, rotationAccountA)
		baseRepo.claims, baseRepo.declarations = claims, declRepo
		started, err := startRotation(t, svc, rotationAccountA, "resolve-failure")
		if err != nil {
			t.Fatal(err)
		}
		decision := DiditDecisionResponse{SessionID: rotationSession, WorkflowID: "wf-chile-id", Status: "Approved", IDVerifications: []DiditIDVerificationNode{{Status: "Approved", FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-02", IdentificationNumber: "12.345.670-K", IssuingState: "CHL", DocumentType: "Identity Card", ExpirationDate: "2030-01-01"}}, LivenessChecks: []DiditLivenessNode{{Status: "Approved", Score: 98.2}}, FaceMatches: []DiditFaceMatchNode{{Status: "Approved", Score: 95.4}}}
		if status, _, _ := EvaluateDecision(&IdentityVerificationAttempt{ID: started.ID, AccountID: rotationAccountA, IDCountry: "CHL", DocumentType: "ID", DeclaredFirstName: "Ana", DeclaredLastName: "Pérez", DeclaredDateOfBirth: "1990-01-02", DeclaredRUTNormalized: "12345670-K", DiditSessionID: rotationSession, WorkflowID: "wf-chile-id"}, &decision); status != "APPROVED" {
			t.Fatalf("valid fixture status=%s", status)
		}
		client.decision = decision
		hash := sha256.Sum256([]byte("resolve-failure-event"))
		input := DiditWebhookInput{EventID: "resolve-failure-event", EventType: "status.updated", ProviderSessionID: rotationSession, TopLevelStatus: "Approved", BodySHA256: hash[:]}
		if _, err := svc.ProcessWebhook(context.Background(), input); !errors.Is(err, ErrRutHMACKeyUnavailable) {
			t.Fatalf("webhook error=%v", err)
		}
		event := findEventByID(t, baseRepo, input.EventID)
		attempt := baseRepo.attempts[started.ID]
		if event.ProcessingStatus != "FAILED" || attempt.Status == "APPROVED" || len(repo.commits) != 0 || len(client.deleted) != 0 {
			t.Fatalf("resolution failure side effects: event=%s attempt=%s commits=%d deleted=%v", event.ProcessingStatus, attempt.Status, len(repo.commits), client.deleted)
		}
		if _, ok := declRepo.rows[decl.ID]; !ok {
			t.Fatal("declaration purged despite failed canonical resolution")
		}
	})
}
