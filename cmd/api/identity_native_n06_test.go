package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	nativeN06ApplicationID = "espacigo-native-n06-test"
	nativeN06WorkflowID    = "native-n06-approved-fixture"
	nativeN06HMACVersion   = "n06-native-v1"
)

var nativeN06HMACSecret = []byte("n06-native-synthetic-hmac-key-2026")

type nativeN06IDs struct {
	values []string
	next   int
}

func (g *nativeN06IDs) ID() (string, error) {
	if g == nil || g.next >= len(g.values) {
		return "", errors.New("native N06 synthetic ID fixture exhausted")
	}
	id := g.values[g.next]
	g.next++
	return id, nil
}

type nativeN06CommitRepo struct {
	*identitydeclaration.Repository
	commitCalls int
	lastCommit  verification.CommitWebhookDecisionInput
}

func (r *nativeN06CommitRepo) CommitWebhookDecision(ctx context.Context, in verification.CommitWebhookDecisionInput) (verification.ProviderAttempt, error) {
	r.commitCalls++
	r.lastCommit = in
	return r.Repository.CommitWebhookDecision(ctx, in)
}

type nativeN06Client struct {
	vendorData     string
	sessionID      string
	deleteFailures int
	deleteCalls    int
	createCalls    int
	decisionReads  int
}

func TestNativeN06DecisionFixtureEvaluatesApproved(t *testing.T) {
	client := &nativeN06Client{sessionID: "n06-session-synthetic", vendorData: "n06-correlation-synthetic"}
	decision, err := client.GetSessionDecision(context.Background(), client.sessionID)
	if err != nil {
		t.Fatal("synthetic authoritative decision fixture was rejected")
	}
	attempt := &verification.IdentityVerificationAttempt{
		ID: "123e4567-e89b-42d3-a456-426614174000", AccountID: "123e4567-e89b-42d3-a456-426614174001",
		Status: "PENDING_USER_INPUT", IDCountry: "CHL", DocumentType: "ID",
		DeclaredFirstName: "Synthetic", DeclaredLastName: "Person", DeclaredDateOfBirth: "1990-01-02",
		DeclaredRUTNormalized: "11222333-9", DiditSessionID: client.sessionID, WorkflowID: nativeN06WorkflowID,
	}
	status, flags, warnings := verification.EvaluateDecision(attempt, &decision)
	if status != "APPROVED" || len(flags) != 0 || len(warnings) != 0 {
		t.Fatalf("synthetic control decision must evaluate APPROVED: status=%s flags=%v warnings=%v", status, flags, warnings)
	}
	if _, err := verification.ComputeRutFingerprint("11.222.333-9", nativeN06HMACSecret, nativeN06HMACVersion); err != nil {
		t.Fatal("synthetic RUT fixture must be valid for the durable claim")
	}
}

func (c *nativeN06Client) CreateSession(_ context.Context, in verification.CreateSessionRequest) (verification.CreateSessionResponse, error) {
	c.createCalls++
	c.vendorData = in.VendorData
	return verification.CreateSessionResponse{SessionID: c.sessionID, SessionURL: "https://verification.didit.me/session/" + c.sessionID}, nil
}

func (c *nativeN06Client) GetSessionDecision(_ context.Context, sessionID string) (verification.DiditDecisionResponse, error) {
	c.decisionReads++
	if sessionID != c.sessionID || c.vendorData == "" {
		return verification.DiditDecisionResponse{}, errors.New("native N06 authoritative decision binding mismatch")
	}
	return verification.DiditDecisionResponse{
		SessionID: c.sessionID, WorkflowID: nativeN06WorkflowID, VendorData: c.vendorData, Status: "Approved",
		IDVerifications: []verification.DiditIDVerificationNode{{
			Status: "Approved", FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02",
			IdentificationNumber: "11.222.333-9", IssuingState: "CHL", DocumentType: "Identity Card", ExpirationDate: "2030-01-01",
		}},
		LivenessChecks: []verification.DiditLivenessNode{{Status: "Approved", Score: 98.2}},
		FaceMatches:    []verification.DiditFaceMatchNode{{Status: "Approved", Score: 95.4}},
	}, nil
}

func (c *nativeN06Client) VerifyWebhookSignature(string, []byte) bool { return true }

func (c *nativeN06Client) DeleteSession(_ context.Context, sessionID string) error {
	c.deleteCalls++
	if sessionID != c.sessionID {
		return errors.New("native N06 deletion binding mismatch")
	}
	if c.deleteFailures > 0 {
		c.deleteFailures--
		return errors.New("synthetic provider deletion failure")
	}
	return nil
}

func runNativeN06AtomicRecoveryTest(t *testing.T, ctx context.Context, appPool, cleanupPool *pgxpool.Pool, newPool func() *pgxpool.Pool, values map[string]string, accountID, declarationID string) {
	t.Helper()
	attemptID, correlationID, eventRowID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	retryEventRowID := uuid.NewString()
	eventID := "n06-event-" + attemptID
	sessionID := "n06-session-" + attemptID
	fingerprint, err := verification.ComputeRutFingerprint("11.222.333-9", nativeN06HMACSecret, nativeN06HMACVersion)
	if err != nil {
		t.Fatal("native N06 synthetic RUT fingerprint could not be prepared")
	}
	var preexistingAttempts, preexistingClaims, preexistingOutbox int64
	if err := appPool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid AND application_id=$2 AND workflow_id=$3),
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid AND fingerprint=$4),
 (SELECT count(*) FROM public.outbox_evento_local WHERE clave_deduplicacion=$5)`,
		accountID, nativeN06ApplicationID, nativeN06WorkflowID, fingerprint, "didit_session_deletion:"+sessionID,
	).Scan(&preexistingAttempts, &preexistingClaims, &preexistingOutbox); err != nil || preexistingAttempts != 0 || preexistingClaims != 0 || preexistingOutbox != 0 {
		t.Fatal("native N06 synthetic scope is occupied or unreadable; preserve prior rows and do not start")
	}
	pendingBefore, err := identitydeclaration.NewPostgresSessionDeletionQueue(appPool).PendingSessionDeletions(ctx)
	if err != nil || len(pendingBefore) != 0 {
		t.Fatal("native N06 requires an empty durable deletion queue; preserve existing provider work and do not retry it")
	}

	repo := identitydeclaration.New(appPool)
	declarationService, err := newRuntimeIdentityDeclarationService(env(values), repo, time.Now)
	if err != nil || declarationService == nil {
		t.Fatal("native N06 declaration service could not be configured")
	}
	keyring, err := loadIdentityDeclarationKeyring(env(values))
	if err != nil || keyring == nil {
		t.Fatal("native N06 decision keyring could not be configured")
	}
	client := &nativeN06Client{sessionID: sessionID, deleteFailures: 1}
	ids := &nativeN06IDs{values: []string{attemptID, correlationID, eventRowID, retryEventRowID}}
	cfg := verification.DiditServiceConfig{
		Environment: "SANDBOX", ApplicationID: nativeN06ApplicationID,
		WorkflowID: nativeN06WorkflowID, WorkflowVersion: "1",
		ConsentVersion: "didit-consent-v1", DeclarationConsentVersion: "id-consent-20261108",
		AttemptTTL: 30 * time.Minute, Retention: identityDeclarationRetentionPolicy,
		RutHMACSecret: nativeN06HMACSecret, RutHMACKeyVersion: nativeN06HMACVersion,
	}
	queue := identitydeclaration.NewPostgresSessionDeletionQueue(appPool)
	commitRepo := &nativeN06CommitRepo{Repository: repo}
	service, err := verification.NewDiditService(commitRepo, declarationService, client, keyring, ids, time.Now, cfg,
		identitydeclaration.NewRutClaimStore(appPool), queue)
	if err != nil || service == nil {
		t.Fatal("native N06 Didit service could not be configured with durable stores")
	}

	started, err := service.StartOrResume(ctx, accountID, "n06-idempotency-"+attemptID, verification.StartDiditIntentInput{
		FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-9",
		IDCountry: "CHL", DocumentType: "ID", ConsentGiven: true, ConsentVersion: "didit-consent-v1",
	})
	if err != nil || started.ID != attemptID || started.Status != "PENDING_USER_INPUT" || client.createCalls != 1 || client.vendorData != correlationID {
		t.Fatalf("native N06 synthetic intent did not start from the saved declaration: %v", err)
	}

	if cleanupPool == nil {
		t.Fatal("native N06 rollback fixture requires the separate authorized cleanup pool")
	}
	if _, err := cleanupPool.Exec(ctx, `CREATE OR REPLACE FUNCTION public.n06_test_abort_snapshot_purge()
RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN RAISE EXCEPTION 'synthetic N06 rollback failpoint'; END
$$`); err != nil {
		t.Fatal("native N06 rollback failpoint function could not be created")
	}
	t.Cleanup(func() {
		_, _ = cleanupPool.Exec(context.Background(), `DROP TRIGGER IF EXISTS n06_test_abort_snapshot_purge ON public.identity_declaration_revision`)
		_, _ = cleanupPool.Exec(context.Background(), `DROP FUNCTION IF EXISTS public.n06_test_abort_snapshot_purge()`)
	})
	if _, err := cleanupPool.Exec(ctx, `CREATE TRIGGER n06_test_abort_snapshot_purge BEFORE DELETE ON public.identity_declaration_revision FOR EACH ROW EXECUTE FUNCTION public.n06_test_abort_snapshot_purge()`); err != nil {
		t.Fatal("native N06 rollback failpoint trigger could not be created")
	}

	bodyHash := sha256.Sum256([]byte("synthetic N06 approved webhook body"))
	webhook := verification.DiditWebhookInput{
		EventID: eventID, EventType: "status.updated", CorrelationID: client.vendorData,
		ProviderSessionID: sessionID, TopLevelStatus: "Approved", BodySHA256: bodyHash[:],
	}
	if _, err := service.ProcessWebhook(ctx, webhook); err == nil {
		t.Fatal("native N06 rollback failpoint did not abort the authoritative commit")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
			t.Fatalf("native N06 rollback was not caused by the snapshot-purge failpoint: %T", err)
		}
	}
	if _, err := cleanupPool.Exec(ctx, `DROP TRIGGER n06_test_abort_snapshot_purge ON public.identity_declaration_revision`); err != nil {
		t.Fatal("native N06 rollback failpoint trigger could not be removed")
	}
	probeTx, err := appPool.Begin(ctx)
	if err != nil {
		t.Fatal("native N06 transactional commit probe could not start")
	}
	var probeAttemptID string
	probeErr := probeTx.QueryRow(ctx, `SELECT (public.commit_verified_identity_decision($1::uuid,$2::uuid,$3::text,$4::text,$5::timestamptz)).id::text`,
		attemptID, eventRowID, fingerprint, sessionID, time.Now().UTC()).Scan(&probeAttemptID)
	rollbackErr := probeTx.Rollback(ctx)
	if rollbackErr != nil {
		t.Fatal("native N06 transactional commit probe could not roll back")
	}
	if probeErr != nil {
		var pgErr *pgconn.PgError
		if errors.As(probeErr, &pgErr) {
			t.Fatalf("native N06 atomic commit probe failed: SQLSTATE=%s constraint=%s", pgErr.Code, pgErr.ConstraintName)
		}
		t.Fatalf("native N06 atomic commit probe failed: %T", probeErr)
	}
	if probeAttemptID != attemptID {
		t.Fatal("native N06 atomic commit probe returned a different attempt")
	}
	var status, currentDeclaration, eventStatus, claimDeclaration string
	var ciphertextBytes, revisionCount, verifiedClaims, pendingOutbox int64
	if err := appPool.QueryRow(ctx, `SELECT
 (SELECT status FROM public.identity_provider_attempt WHERE id=$1::uuid),
 (SELECT COALESCE(declaration_id::text,'') FROM public.identity_provider_attempt WHERE id=$1::uuid),
 (SELECT COALESCE(octet_length(encrypted_decision),0) FROM public.identity_provider_attempt WHERE id=$1::uuid),
 (SELECT processing_status FROM public.identity_provider_event WHERE provider='didit' AND environment='SANDBOX' AND application_id=$2 AND event_id=$3),
 (SELECT count(*) FROM public.identity_declaration_revision WHERE id=$4::uuid AND account_id=$5::uuid),
 (SELECT COALESCE(declaration_id::text,'') FROM public.identity_rut_claim WHERE account_id=$5::uuid AND fingerprint=$6),
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$5::uuid AND fingerprint=$6 AND verified_at IS NOT NULL),
 (SELECT count(*) FROM public.outbox_evento_local WHERE clave_deduplicacion=$7 AND entregada_en IS NULL)`,
		attemptID, nativeN06ApplicationID, eventID, declarationID, accountID, fingerprint, "didit_session_deletion:"+sessionID,
	).Scan(&status, &currentDeclaration, &ciphertextBytes, &eventStatus, &revisionCount, &claimDeclaration, &verifiedClaims, &pendingOutbox); err != nil {
		t.Fatal("native N06 rollback state could not be read back")
	}
	if status != "PENDING_USER_INPUT" || currentDeclaration != declarationID || ciphertextBytes != 0 || eventStatus != "RECEIVED" || revisionCount != 1 || claimDeclaration != declarationID || verifiedClaims != 0 || pendingOutbox != 0 || client.deleteCalls != 0 {
		t.Fatalf("native N06 failed commit was not fully rolled back: status=%s event=%s revisions=%d verified_claims=%d outbox=%d", status, eventStatus, revisionCount, verifiedClaims, pendingOutbox)
	}

	outcome, err := service.ProcessWebhook(ctx, webhook)
	if err != nil || outcome.AttemptStatus != "APPROVED" || outcome.ProcessingStatus != "PROCESSED" || outcome.RemoteDeleted || !outcome.Duplicate || client.deleteCalls != 1 || client.decisionReads != 2 {
		in := commitRepo.lastCommit
		t.Fatalf("native N06 approved retry/outbox persistence failed: outcome=%+v err=%v commits=%d next=%s purge=%t fingerprint=%t delete_session=%t ciphertext_bytes=%d decision_reads=%d delete_calls=%d", outcome, err, commitRepo.commitCalls, in.NextStatus, in.PurgeDeclaration, in.RutFingerprint != "", in.DeleteSessionID != "", len(in.EncryptedDecision), client.decisionReads, client.deleteCalls)
	}
	if err := appPool.QueryRow(ctx, `SELECT
 (SELECT status FROM public.identity_provider_attempt WHERE id=$1::uuid),
 (SELECT COALESCE(declaration_id::text,'') FROM public.identity_provider_attempt WHERE id=$1::uuid),
 (SELECT COALESCE(octet_length(encrypted_decision),0) FROM public.identity_provider_attempt WHERE id=$1::uuid),
 (SELECT processing_status FROM public.identity_provider_event WHERE provider='didit' AND environment='SANDBOX' AND application_id=$2 AND event_id=$3),
 (SELECT count(*) FROM public.identity_declaration_revision WHERE id=$4::uuid AND account_id=$5::uuid),
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$5::uuid AND fingerprint=$6 AND declaration_id IS NULL AND verified_at IS NOT NULL AND expires_at IS NULL),
 (SELECT count(*) FROM public.outbox_evento_local WHERE clave_deduplicacion=$7 AND entregada_en IS NULL)`,
		attemptID, nativeN06ApplicationID, eventID, declarationID, accountID, fingerprint, "didit_session_deletion:"+sessionID,
	).Scan(&status, &currentDeclaration, &ciphertextBytes, &eventStatus, &revisionCount, &verifiedClaims, &pendingOutbox); err != nil {
		t.Fatal("native N06 committed state could not be read back")
	}
	if status != "APPROVED" || currentDeclaration != "" || ciphertextBytes != 0 || eventStatus != "PROCESSED" || revisionCount != 0 || verifiedClaims != 1 || pendingOutbox != 1 {
		t.Fatalf("native N06 successful commit violated atomic VERIFIED/purge/outbox state: status=%s event=%s revisions=%d verified_claims=%d outbox=%d", status, eventStatus, revisionCount, verifiedClaims, pendingOutbox)
	}

	// Reconstruct the service and queue on a fresh pool, with no process-local retry state.
	recoveryPool := newPool()
	t.Cleanup(recoveryPool.Close)
	recoveryRepo := identitydeclaration.New(recoveryPool)
	recoveryDeclarations, err := newRuntimeIdentityDeclarationService(env(values), recoveryRepo, time.Now)
	if err != nil || recoveryDeclarations == nil {
		t.Fatal("native N06 recovery declaration service could not be configured")
	}
	recoveryKeyring, err := loadIdentityDeclarationKeyring(env(values))
	if err != nil || recoveryKeyring == nil {
		t.Fatal("native N06 recovery keyring could not be configured")
	}
	recoveryClient := &nativeN06Client{sessionID: sessionID}
	recoveryQueue := identitydeclaration.NewPostgresSessionDeletionQueue(recoveryPool)
	recoveryService, err := verification.NewDiditService(recoveryRepo, recoveryDeclarations, recoveryClient, recoveryKeyring,
		&nativeN06IDs{}, time.Now, cfg, identitydeclaration.NewRutClaimStore(recoveryPool), recoveryQueue)
	if err != nil || recoveryService == nil {
		t.Fatal("native N06 recovery service could not be configured with durable stores")
	}
	pending, err := recoveryQueue.PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 1 || pending[0] != sessionID {
		t.Fatalf("native N06 pending deletion did not survive service restart: pending=%v err=%v", pending, err)
	}
	verifiedStatus, err := recoveryService.GetStatus(ctx, accountID, attemptID)
	if err != nil || verifiedStatus.Status != "APPROVED" {
		t.Fatalf("native N06 verified status did not survive service restart: status=%+v err=%v", verifiedStatus, err)
	}
	deleted, err := recoveryService.RetryPendingDeletions(ctx)
	if err != nil || len(deleted) != 1 || deleted[0] != sessionID || recoveryClient.deleteCalls != 1 {
		t.Fatalf("native N06 durable remote deletion retry failed: deleted=%v err=%v", deleted, err)
	}
	pending, err = recoveryQueue.PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatalf("native N06 acknowledged deletion remained pending after restart: pending=%v err=%v", pending, err)
	}
	var acknowledged bool
	if err := recoveryPool.QueryRow(ctx, `SELECT entregada_en IS NOT NULL AND retirar_en >= entregada_en + interval '30 days'
 FROM public.outbox_evento_local WHERE clave_deduplicacion=$1`, "didit_session_deletion:"+sessionID).Scan(&acknowledged); err != nil || !acknowledged {
		t.Fatal("native N06 acknowledgement did not retain the outbox row for the configured period")
	}
}

func cleanupNativeN06Artifacts(ctx context.Context, tx pgx.Tx, accountID, declarationID string) error {
	fingerprint, err := verification.ComputeRutFingerprint("11.222.333-9", nativeN06HMACSecret, nativeN06HMACVersion)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, provider_session_id, COALESCE(declaration_id::text,''), status
 FROM public.identity_provider_attempt
 WHERE account_id=$1::uuid AND provider='didit' AND environment='SANDBOX'
   AND application_id=$2 AND workflow_id=$3
 ORDER BY created_at FOR UPDATE`, accountID, nativeN06ApplicationID, nativeN06WorkflowID)
	if err != nil {
		return err
	}
	type attemptScope struct{ id, session, declaration, status string }
	var attempts []attemptScope
	for rows.Next() {
		var attempt attemptScope
		if err := rows.Scan(&attempt.id, &attempt.session, &attempt.declaration, &attempt.status); err != nil {
			rows.Close()
			return err
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(attempts) > 1 {
		return errors.New("native KYC N06 cleanup matched multiple provider attempts")
	}
	if len(attempts) == 1 {
		attempt := attempts[0]
		if !canonicalNativeUUID(attempt.id) || attempt.session != "n06-session-"+attempt.id ||
			(attempt.declaration != "" && attempt.declaration != declarationID) {
			return errors.New("native KYC N06 attempt does not match the trusted synthetic scope")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.identity_provider_event WHERE attempt_id=$1::uuid`, attempt.id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.outbox_evento_local
 WHERE agregado_tipo='didit_session' AND agregado_id=$1::uuid
   AND tipo='didit.session_remote_deletion' AND clave_deduplicacion=$2`, attempt.id, "didit_session_deletion:"+attempt.session); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.identity_provider_attempt
 WHERE id=$1::uuid AND account_id=$2::uuid AND application_id=$3 AND workflow_id=$4`, attempt.id, accountID, nativeN06ApplicationID, nativeN06WorkflowID); err != nil {
			return err
		}
	} else {
		var orphanOutbox int64
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE clave_deduplicacion LIKE 'didit_session_deletion:n06-session-%'`).Scan(&orphanOutbox); err != nil {
			return err
		}
		if orphanOutbox != 0 {
			return errors.New("native KYC N06 outbox exists without its scoped attempt; preserve for read-only reconciliation")
		}
	}
	var claims int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid AND fingerprint=$2`, accountID, fingerprint).Scan(&claims); err != nil {
		return err
	}
	if claims > 1 {
		return errors.New("native KYC N06 fingerprint matched multiple claims")
	}
	if claims == 1 {
		var claimDeclaration string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(declaration_id::text,'') FROM public.identity_rut_claim
 WHERE account_id=$1::uuid AND fingerprint=$2 FOR UPDATE`, accountID, fingerprint).Scan(&claimDeclaration); err != nil {
			return err
		}
		if claimDeclaration != "" && claimDeclaration != declarationID {
			return errors.New("native KYC N06 fingerprint is attached to another declaration")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.identity_rut_claim
 WHERE account_id=$1::uuid AND fingerprint=$2 AND (declaration_id=$3::uuid OR declaration_id IS NULL)`, accountID, fingerprint, declarationID); err != nil {
			return err
		}
	}
	return nil
}
