package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Only the existing synthetic decision's workflow binding differs from N06.
// No HTTP client or external provider is instantiated by this fixture.
type nativeRotationProvider struct{ *nativeN06Client }

func (c *nativeRotationProvider) GetSessionDecision(ctx context.Context, id string) (verification.DiditDecisionResponse, error) {
	decision, err := c.nativeN06Client.GetSessionDecision(ctx, id)
	decision.WorkflowID = "local-kyc-test-workflow"
	return decision, err
}

func runNativeRotationApproval(t *testing.T, ctx context.Context, appPool, recoveryPool *pgxpool.Pool, declarations *verification.DeclarationService, keys verification.DeclarationKeyring, attempts *nativeAttemptJournalRepository, accountID, declarationID, legacyFingerprint string) {
	t.Helper()
	sessionID := uuid.NewString()
	provider := &nativeRotationProvider{&nativeN06Client{sessionID: sessionID, deleteFailures: 1}}
	cfg := verification.DiditServiceConfig{Environment: "SANDBOX", ApplicationID: "local-kyc-test-app", WorkflowID: "local-kyc-test-workflow", WorkflowVersion: "1", ConsentVersion: defaultDiditConsentVersion, DeclarationConsentVersion: defaultIdentityDeclarationConsentVersion, AttemptTTL: verification.DefaultDiditAttemptTTL, Retention: identityDeclarationRetentionPolicy, RequireLiveForVerifiedBadge: true, RutHMACKeyring: declarations.RutKeyring()}
	attempts.beforeCommit = func(_ context.Context, in verification.CommitWebhookDecisionInput, _ nativeAttemptIntent) error {
		if in.NextStatus != "APPROVED" || in.RutFingerprint != legacyFingerprint || !in.PurgeDeclaration || in.DeleteSessionID != sessionID {
			return errors.New("synthetic rotation commit did not use its canonical fingerprint")
		}
		if os.Getenv("IDENTITY_KYC_RUT_ROTATION_NATIVE_COMMIT_RACE") == "1" {
			runNativeRotationCommitRace(t, ctx, appPool, recoveryPool, in)
		}
		return nil
	}
	queue := identitydeclaration.NewPostgresSessionDeletionQueue(appPool)
	service, err := verification.NewDiditService(attempts, declarations, provider, keys, credentials.Generator{}, time.Now, cfg, identitydeclaration.NewRutClaimStore(appPool), queue)
	if err != nil {
		t.Fatal("configure native rotation approval service")
	}
	started, err := service.StartOrResume(ctx, accountID, "native-rotation-approval", verification.StartDiditIntentInput{ConsentGiven: true, ConsentVersion: defaultDiditConsentVersion})
	if err != nil || provider.createCalls != 1 || started.Status != "PENDING_USER_INPUT" {
		t.Fatalf("native rotated Start failed: %T", err)
	}
	intent, written := attempts.Snapshot()
	if !written || intent.AttemptID != started.ID || intent.DeclarationID != declarationID || provider.vendorData != intent.CorrelationID {
		t.Fatal("rotated provider intent did not match its pre-write journal")
	}
	control, err := provider.GetSessionDecision(ctx, sessionID)
	if err != nil {
		t.Fatal("synthetic authoritative control unavailable")
	}
	eval := &verification.IdentityVerificationAttempt{ID: started.ID, AccountID: accountID, IDCountry: "CHL", DocumentType: "ID", DeclaredFirstName: "Synthetic", DeclaredLastName: "Person", DeclaredDateOfBirth: "1990-01-02", DeclaredRUTNormalized: "11222333-9", DiditSessionID: sessionID, WorkflowID: cfg.WorkflowID}
	if status, _, _ := verification.EvaluateDecision(eval, &control); status != "APPROVED" {
		t.Fatal("complete synthetic control must independently evaluate approved")
	}
	readsBefore := provider.decisionReads
	hash := sha256.Sum256([]byte("synthetic R2 rotation authoritative event"))
	outcome, err := service.ProcessWebhook(ctx, verification.DiditWebhookInput{EventID: "rotation-event-" + uuid.NewString(), EventType: "status.updated", CorrelationID: intent.CorrelationID, ProviderSessionID: sessionID, TopLevelStatus: "Approved", BodySHA256: hash[:]})
	if err != nil || outcome.AttemptStatus != "APPROVED" || outcome.ProcessingStatus != "PROCESSED" || outcome.RemoteDeleted || provider.deleteCalls != 1 || provider.decisionReads != readsBefore+1 || !attempts.RetentionObserved() {
		t.Fatalf("native canonical approval/initial failed deletion did not complete: %T", err)
	}
	var revisions, claims, pending, ciphertext int
	if err := appPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE id=$1::uuid),(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$2::uuid AND fingerprint=$3 AND declaration_id IS NULL AND verified_at IS NOT NULL AND expires_at IS NULL),(SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id=$4::uuid AND clave_deduplicacion=$5 AND entregada_en IS NULL),(SELECT COALESCE(octet_length(encrypted_decision),0) FROM public.identity_provider_attempt WHERE id=$4::uuid)`, declarationID, accountID, legacyFingerprint, intent.AttemptID, "didit_session_deletion:"+sessionID).Scan(&revisions, &claims, &pending, &ciphertext); err != nil || revisions != 0 || claims != 1 || pending != 1 || ciphertext != 0 {
		t.Fatal("native approval did not atomically purge snapshot and keep canonical historical reservation/outbox")
	}
	recoveryRepo := identitydeclaration.New(recoveryPool)
	recoveredDeclarations, err := verification.NewDeclarationServiceWithRutKeyring(recoveryRepo, keys, credentials.Generator{}, time.Now, identityDeclarationRetentionPolicy, defaultIdentityDeclarationConsentVersion, declarations.RutKeyring())
	if err != nil {
		t.Fatal("reconstruct native rotated declaration service")
	}
	recoveryProvider := &nativeRotationProvider{&nativeN06Client{sessionID: sessionID, vendorData: intent.CorrelationID}}
	recoveryQueue := identitydeclaration.NewPostgresSessionDeletionQueue(recoveryPool)
	recovered, err := verification.NewDiditService(recoveryRepo, recoveredDeclarations, recoveryProvider, keys, credentials.Generator{}, time.Now, cfg, identitydeclaration.NewRutClaimStore(recoveryPool), recoveryQueue)
	if err != nil {
		t.Fatal("reconstruct native rotated provider service")
	}
	pendingIDs, err := recoveryQueue.PendingSessionDeletions(ctx)
	if err != nil || len(pendingIDs) != 1 || pendingIDs[0] != sessionID {
		t.Fatal("recovery must contain exactly this fixture's deletion, never foreign work")
	}
	status, err := recovered.GetStatus(ctx, accountID, intent.AttemptID)
	if err != nil || status.Status != "APPROVED" || status.BadgeResult != "IN_REVIEW" {
		t.Fatal("rotated state did not survive reconstruction or sandbox became eligible")
	}
	deleted, err := recovered.RetryPendingDeletions(ctx)
	if err != nil || len(deleted) != 1 || deleted[0] != sessionID || recoveryProvider.deleteCalls != 1 {
		t.Fatal("native rotated outbox did not recover and acknowledge its own synthetic deletion")
	}
	pendingIDs, err = recoveryQueue.PendingSessionDeletions(ctx)
	if err != nil || len(pendingIDs) != 0 {
		t.Fatal("acknowledged rotated deletion remained pending")
	}
	var ack bool
	if err := recoveryPool.QueryRow(ctx, `SELECT entregada_en IS NOT NULL AND retirar_en>=entregada_en+interval '30 days' FROM public.outbox_evento_local WHERE agregado_id=$1::uuid AND clave_deduplicacion=$2`, intent.AttemptID, "didit_session_deletion:"+sessionID).Scan(&ack); err != nil || !ack {
		t.Fatal("native rotated ACK missing or retention shortened")
	}
	t.Log("PASS: rotated Start, canonical legacy approval, atomic snapshot purge, new service/pool recovery, synthetic deletion retry/ACK and sandbox non-eligibility")
}
