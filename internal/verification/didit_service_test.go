package verification

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

type memProviderAttemptRepo struct {
	attempts     map[string]ProviderAttempt
	events       map[string]ProviderEvent
	order        []string
	claims       RutClaimStore
	outbox       []string
	declarations *declarationMemoryRepository
}

func newMemProviderAttemptRepo() *memProviderAttemptRepo {
	return &memProviderAttemptRepo{
		attempts: make(map[string]ProviderAttempt),
		events:   make(map[string]ProviderEvent),
	}
}

func (r *memProviderAttemptRepo) PrepareAttempt(_ context.Context, in PrepareAttemptInput) (ProviderAttempt, bool, error) {
	cutoff := in.Now.Add(-in.AttemptTTL)
	for id, a := range r.attempts {
		if a.AccountID == in.AccountID && a.Provider == in.Provider && a.Environment == in.Environment && a.ApplicationID == in.ApplicationID {
			stale := (a.Status == "CREATED" || a.Status == "PENDING_USER_INPUT") && !a.CreatedAt.After(cutoff)
			superseded := a.DeclarationID != in.DeclarationID && (a.Status == "CREATED" || a.Status == "PENDING_USER_INPUT" || a.Status == "PROCESSING" || a.Status == "IN_REVIEW")
			if stale || superseded {
				a.Status = "EXPIRED"
				a.UpdatedAt = in.Now
				r.attempts[id] = a
			}
		}
	}
	for _, id := range r.order {
		a := r.attempts[id]
		if a.Provider == in.Provider && a.Environment == in.Environment && a.ApplicationID == in.ApplicationID && a.IdempotencyKey == in.IdempotencyKey {
			if a.AccountID != in.AccountID || a.DeclarationID != in.DeclarationID {
				return ProviderAttempt{}, false, ErrConflict
			}
			if a.Status == "CREATED" || a.Status == "PENDING_USER_INPUT" || a.Status == "PROCESSING" || a.Status == "IN_REVIEW" || a.Status == "APPROVED" {
				return a, true, nil
			}
		}
	}
	for _, id := range r.order {
		a := r.attempts[id]
		if a.AccountID == in.AccountID && a.DeclarationID == in.DeclarationID && a.Provider == in.Provider && a.Environment == in.Environment && a.ApplicationID == in.ApplicationID {
			if a.Status == "CREATED" || a.Status == "PENDING_USER_INPUT" || a.Status == "PROCESSING" || a.Status == "IN_REVIEW" {
				return a, true, nil
			}
		}
	}
	row := ProviderAttempt{
		ID:              in.ID,
		AccountID:       in.AccountID,
		DeclarationID:   in.DeclarationID,
		Provider:        in.Provider,
		Environment:     in.Environment,
		ApplicationID:   in.ApplicationID,
		WorkflowID:      in.WorkflowID,
		WorkflowVersion: in.WorkflowVersion,
		IdempotencyKey:  in.IdempotencyKey,
		CorrelationID:   in.CorrelationID,
		Status:          "CREATED",
		CreatedAt:       in.Now,
		UpdatedAt:       in.Now,
		RetentionAt:     in.RetentionAt,
	}
	r.attempts[row.ID] = row
	r.order = append(r.order, row.ID)
	return row, false, nil
}

func (r *memProviderAttemptRepo) MarkAttemptSessionReady(_ context.Context, attemptID, providerSessionID string, updatedAt time.Time) (ProviderAttempt, error) {
	a, ok := r.attempts[attemptID]
	if !ok {
		return ProviderAttempt{}, ErrNotFound
	}
	a.ProviderSessionID = providerSessionID
	a.Status = "PENDING_USER_INPUT"
	a.UpdatedAt = updatedAt
	r.attempts[attemptID] = a
	return a, nil
}

func (r *memProviderAttemptRepo) MarkAttemptFailed(_ context.Context, attemptID string, updatedAt time.Time) error {
	a, ok := r.attempts[attemptID]
	if !ok {
		return ErrNotFound
	}
	a.Status = "FAILED"
	a.UpdatedAt = updatedAt
	r.attempts[attemptID] = a
	return nil
}

func (r *memProviderAttemptRepo) GetAttemptForAccount(_ context.Context, accountID, attemptID string, now time.Time, attemptTTL time.Duration) (ProviderAttempt, error) {
	a, ok := r.attempts[attemptID]
	if !ok || a.AccountID != accountID || !a.RetentionAt.After(now) {
		return ProviderAttempt{}, ErrNotFound
	}
	if (a.Status == "CREATED" || a.Status == "PENDING_USER_INPUT") && !a.CreatedAt.Add(attemptTTL).After(now) {
		a.Status = "EXPIRED"
		a.UpdatedAt = now
		r.attempts[attemptID] = a
	}
	return a, nil
}

func (r *memProviderAttemptRepo) FindAttemptForWebhook(_ context.Context, provider, environment, applicationID, correlationID, providerSessionID string) (ProviderAttempt, error) {
	for _, id := range r.order {
		a := r.attempts[id]
		if a.Provider == provider && a.Environment == environment && a.ApplicationID == applicationID {
			if correlationID != "" && a.CorrelationID == correlationID {
				return a, nil
			}
		}
	}
	for _, id := range r.order {
		a := r.attempts[id]
		if a.Provider == provider && a.Environment == environment && a.ApplicationID == applicationID {
			if providerSessionID != "" && a.ProviderSessionID == providerSessionID {
				return a, nil
			}
		}
	}
	return ProviderAttempt{}, ErrNotFound
}

func (r *memProviderAttemptRepo) InsertOrGetWebhookEvent(_ context.Context, in WebhookEventRecordInput) (ProviderEvent, bool, error) {
	key := in.Provider + ":" + in.Environment + ":" + in.ApplicationID + ":" + in.EventID
	if existing, ok := r.events[key]; ok {
		return existing, true, nil
	}
	evt := ProviderEvent{
		ID:               in.ID,
		AttemptID:        in.AttemptID,
		Provider:         in.Provider,
		Environment:      in.Environment,
		ApplicationID:    in.ApplicationID,
		EventID:          in.EventID,
		EventType:        in.EventType,
		BodySHA256:       append([]byte(nil), in.BodySHA256...),
		ProcessingStatus: "RECEIVED",
		ReceivedAt:       in.ReceivedAt,
		RetentionAt:      in.RetentionAt,
	}
	r.events[key] = evt
	return evt, false, nil
}

func (r *memProviderAttemptRepo) SaveAttemptDecisionAndCompleteEvent(_ context.Context, attemptID, eventRowID, nextStatus, decisionKeyVersion, decisionAlgorithm string, encryptedDecision []byte, processedAt time.Time) (ProviderAttempt, error) {
	a, ok := r.attempts[attemptID]
	if !ok {
		return ProviderAttempt{}, ErrNotFound
	}
	a.Status = nextStatus
	a.DecisionKeyVersion = decisionKeyVersion
	a.DecisionAlgorithm = decisionAlgorithm
	a.EncryptedDecision = append([]byte(nil), encryptedDecision...)
	a.UpdatedAt = processedAt
	r.attempts[attemptID] = a
	r.markEvent(eventRowID, "PROCESSED", processedAt)
	return a, nil
}

func (r *memProviderAttemptRepo) CommitWebhookDecision(ctx context.Context, in CommitWebhookDecisionInput) (ProviderAttempt, error) {
	a, ok := r.attempts[in.AttemptID]
	if !ok {
		return ProviderAttempt{}, ErrNotFound
	}
	if in.RutFingerprint != "" && r.claims != nil {
		if err := r.claims.MarkVerified(ctx, a.AccountID, a.DeclarationID, in.RutFingerprint, in.ProcessedAt); err != nil {
			return ProviderAttempt{}, err
		}
	}
	a.Status = in.NextStatus
	a.DecisionKeyVersion = in.DecisionKeyVersion
	a.DecisionAlgorithm = in.DecisionAlgorithm
	a.EncryptedDecision = append([]byte(nil), in.EncryptedDecision...)
	a.UpdatedAt = in.ProcessedAt
	if in.PurgeDeclaration {
		declarationID := a.DeclarationID
		a.DeclarationID = ""
		a.DecisionKeyVersion = ""
		a.DecisionAlgorithm = ""
		a.EncryptedDecision = nil
		if r.declarations != nil {
			delete(r.declarations.rows, declarationID)
		}
	}
	r.attempts[in.AttemptID] = a
	r.markEvent(in.EventRowID, "PROCESSED", in.ProcessedAt)
	if in.DeleteSessionID != "" {
		r.outbox = append(r.outbox, in.DeleteSessionID)
	}
	return a, nil
}

func (r *memProviderAttemptRepo) UpdateAttemptStatusAndCompleteEvent(_ context.Context, attemptID, eventRowID, nextStatus, eventProcessingStatus string, processedAt time.Time) (ProviderAttempt, error) {
	a, ok := r.attempts[attemptID]
	if !ok {
		return ProviderAttempt{}, ErrNotFound
	}
	a.Status = nextStatus
	a.UpdatedAt = processedAt
	r.attempts[attemptID] = a
	r.markEvent(eventRowID, eventProcessingStatus, processedAt)
	return a, nil
}

func (r *memProviderAttemptRepo) CompleteWebhookEvent(_ context.Context, eventRowID, processingStatus string, processedAt time.Time) error {
	r.markEvent(eventRowID, processingStatus, processedAt)
	return nil
}

func findEventByID(t *testing.T, repo *memProviderAttemptRepo, eventID string) ProviderEvent {
	t.Helper()
	for _, event := range repo.events {
		if event.EventID == eventID {
			return event
		}
	}
	t.Fatalf("webhook event %q not found", eventID)
	return ProviderEvent{}
}

func (r *memProviderAttemptRepo) markEvent(eventRowID, status string, at time.Time) {
	for k, evt := range r.events {
		if evt.ID == eventRowID {
			evt.ProcessingStatus = status
			cp := at
			evt.ProcessedAt = &cp
			r.events[k] = evt
		}
	}
}

type fakeDiditClient struct {
	createResp   CreateSessionResponse
	createErr    error
	decision     DiditDecisionResponse
	decErr       error
	deleteErr    error
	created      []CreateSessionRequest
	deleted      []string
	decisionGets int
}

func (f *fakeDiditClient) CreateSession(_ context.Context, req CreateSessionRequest) (CreateSessionResponse, error) {
	f.created = append(f.created, req)
	if f.createErr != nil {
		return CreateSessionResponse{}, f.createErr
	}
	return f.createResp, nil
}

func (f *fakeDiditClient) GetSessionDecision(_ context.Context, _ string) (DiditDecisionResponse, error) {
	f.decisionGets++
	if f.decErr != nil {
		return DiditDecisionResponse{}, f.decErr
	}
	return f.decision, nil
}

func (f *fakeDiditClient) VerifyWebhookSignature(string, []byte) bool { return true }

func (f *fakeDiditClient) DeleteSession(_ context.Context, sessionID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, sessionID)
	return nil
}

type counterUUIDs struct {
	n int
}

func (c *counterUUIDs) ID() (string, error) {
	c.n++
	return fmt.Sprintf("85000000-0000-4000-8000-%012d", c.n), nil
}

func newTestKeyring(t *testing.T) *declarationcrypto.Keyring {
	t.Helper()
	kr, err := declarationcrypto.New("v1", map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	return kr
}

func TestDiditServiceFullLifecycleAndSecurityInvariants(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	ids := &counterUUIDs{}
	keys := newTestKeyring(t)
	declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	declSvc, err := NewDeclarationService(declRepo, keys, ids, clock, func(t time.Time) time.Time {
		return t.Add(30 * 24 * time.Hour)
	}, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}

	attemptRepo := newMemProviderAttemptRepo()
	attemptRepo.declarations = declRepo
	client := &fakeDiditClient{
		createResp: CreateSessionResponse{
			SessionID:  "didit-sess-001",
			SessionURL: "https://verify.didit.me/session/didit-sess-001",
		},
	}
	rutClaims := NewMemoryRutClaimRegistry()
	attemptRepo.claims = rutClaims
	hmacSecret := []byte("0123456789abcdef0123456789abcdef")

	diditSvc, err := NewDiditService(attemptRepo, declSvc, client, keys, ids, clock, DiditServiceConfig{
		Environment:                 "SANDBOX",
		ApplicationID:               "espacigo-test-app",
		WorkflowID:                  "wf-chile-id",
		WorkflowVersion:             "1",
		ConsentVersion:              "2026-11-v1",
		AttemptTTL:                  30 * time.Minute,
		RequireLiveForVerifiedBadge: false,
		RutHMACSecret:               hmacSecret,
		RutHMACKeyVersion:           "v1",
	}, rutClaims)
	if err != nil {
		t.Fatal(err)
	}

	accountA := "11111111-1111-4111-8111-111111111111"
	accountB := "22222222-2222-4222-8222-222222222222"

	// 1. Starting without a saved declaration fails closed with ErrConflict.
	_, err = diditSvc.StartOrResume(ctx, accountA, "idemp-1", StartDiditIntentInput{
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when declaration is missing, got %v", err)
	}

	// 2. Save declaration for account A.
	savedDecl, err := declSvc.Save(ctx, accountA, DeclarationPayload{
		FirstName:    "Ana",
		LastName:     "Pérez",
		DateOfBirth:  "1990-01-02",
		RUT:          "12.345.670-K",
		IDCountry:    "CHL",
		DocumentType: "ID",
	}, true, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}

	// 3. Mismatched client payload vs server declaration is rejected.
	_, err = diditSvc.StartOrResume(ctx, accountA, "idemp-mismatch", StartDiditIntentInput{
		FirstName:      "Otra",
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when client payload mismatches saved declaration, got %v", err)
	}

	// 4. StartOrResume uses server declaration and returns redirect_url.
	started, err := diditSvc.StartOrResume(ctx, accountA, "idemp-1", StartDiditIntentInput{
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != "PENDING_USER_INPUT" || started.RedirectURL != "https://verify.didit.me/session/didit-sess-001" {
		t.Fatalf("unexpected started intent: %+v", started)
	}
	if len(client.created) != 1 || client.created[0].ExpectedDetails.IdentificationNumber != "12345670-K" {
		t.Fatalf("expected 1 Didit session with canonical RUT, got %+v", client.created)
	}

	// 5. Idempotent replay reuses the active attempt without calling Didit again.
	replayed, err := diditSvc.StartOrResume(ctx, accountA, "idemp-1", StartDiditIntentInput{
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != started.ID || len(client.created) != 1 {
		t.Fatalf("expected idempotent reuse without second CreateSession call, got %+v (calls=%d)", replayed, len(client.created))
	}

	// 6. Account B attempting to claim the same RUT while Account A holds an active claim is blocked with ErrConflict.
	_, err = declSvc.Save(ctx, accountB, DeclarationPayload{
		FirstName:    "Pedro",
		LastName:     "Soto",
		DateOfBirth:  "1988-05-10",
		RUT:          "12.345.670-K",
		IDCountry:    "CHL",
		DocumentType: "ID",
	}, true, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = diditSvc.StartOrResume(ctx, accountB, "idemp-b1", StartDiditIntentInput{
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when second account claims active RUT, got %v", err)
	}

	// 7. Cross-account GetStatus is hidden as ErrNotFound.
	if _, err := diditSvc.GetStatus(ctx, accountB, started.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-account GetStatus, got %v", err)
	}

	// A complete, independently valid decision is required for the fallback regression.
	webhookApproval := &DiditDecisionResponse{
		SessionID: "didit-sess-001", WorkflowID: "wf-chile-id", Status: "Approved",
		IDVerifications: []DiditIDVerificationNode{{Status: "Approved", FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-02", IdentificationNumber: "12.345.670-K", IssuingState: "CHL", DocumentType: "Identity Card", ExpirationDate: "2030-01-01"}},
		LivenessChecks:  []DiditLivenessNode{{Status: "Approved", Score: 98.2}},
		FaceMatches:     []DiditFaceMatchNode{{Status: "Approved", Score: 95.4}},
	}
	evalAttempt := &IdentityVerificationAttempt{ID: started.ID, AccountID: accountA, IDCountry: "CHL", DocumentType: "ID", DeclaredFirstName: "Ana", DeclaredLastName: "Pérez", DeclaredDateOfBirth: "1990-01-02", DeclaredRUTNormalized: "12345670-K", DiditSessionID: "didit-sess-001", WorkflowID: "wf-chile-id"}
	if status, _, _ := EvaluateDecision(evalAttempt, webhookApproval); status != "APPROVED" {
		t.Fatalf("control decision fixture must independently evaluate APPROVED, got %s", status)
	}
	assertUnverifiedClaim := func() {
		t.Helper()
		rutClaims.mu.Lock()
		defer rutClaims.mu.Unlock()
		found := 0
		for _, claim := range rutClaims.byDigest {
			if claim.AccountID == accountA {
				found++
				if claim.Verified {
					t.Fatal("failed or incomplete reconciliation verified the RUT claim")
				}
			}
		}
		if found != 1 {
			t.Fatalf("expected one pending account claim, found %d", found)
		}
	}

	// Failed authoritative lookup leaves the exact event retryable and does not approve.
	client.decErr = errors.New("temporary decision lookup failure")
	failedHash := sha256.Sum256([]byte(`{"event_id":"evt-authority-failure"}`))
	eventInput := DiditWebhookInput{EventID: "evt-authority-failure", EventType: "status.updated", ProviderSessionID: "didit-sess-001", TopLevelStatus: "Approved", Decision: webhookApproval, BodySHA256: failedHash[:]}
	_, err = diditSvc.ProcessWebhook(ctx, eventInput)
	if err == nil {
		t.Fatal("expected recoverable error when authoritative decision lookup fails")
	}
	failedAttempt := attemptRepo.attempts[started.ID]
	if failedAttempt.Status == "APPROVED" || len(failedAttempt.EncryptedDecision) != 0 || len(client.deleted) != 0 {
		t.Fatalf("webhook decision must not approve, save decision, or delete session: attempt=%+v deleted=%v", failedAttempt, client.deleted)
	}
	failedEventCount := 0
	for _, event := range attemptRepo.events {
		if event.EventID == eventInput.EventID {
			failedEventCount++
			if event.ProcessingStatus != "FAILED" {
				t.Fatalf("expected event to remain retryable as FAILED, got %s", event.ProcessingStatus)
			}
		}
	}
	if failedEventCount != 1 {
		t.Fatalf("expected exactly one stored FAILED event, found %d", failedEventCount)
	}
	assertUnverifiedClaim()

	// A genuinely session-less persisted attempt must fail before any GET.
	client.decErr = nil
	noSessionHash := sha256.Sum256([]byte(`{"event_id":"evt-no-provider-session"}`))
	noSessionInput := DiditWebhookInput{EventID: "evt-no-provider-session", EventType: "status.updated", CorrelationID: attemptRepo.attempts[started.ID].CorrelationID, TopLevelStatus: "Approved", Decision: webhookApproval, BodySHA256: noSessionHash[:]}
	savedAttempt := attemptRepo.attempts[started.ID]
	withoutSession := savedAttempt
	withoutSession.Status = "PENDING_USER_INPUT"
	withoutSession.ProviderSessionID = ""
	attemptRepo.attempts[started.ID] = withoutSession
	getCountBefore := client.decisionGets
	_, noSessionErr := diditSvc.ProcessWebhook(ctx, noSessionInput)
	if noSessionErr == nil {
		t.Fatal("expected failure for persisted attempt without provider session")
	}
	if client.decisionGets != getCountBefore {
		t.Fatalf("session-less webhook made decision GET: before=%d after=%d", getCountBefore, client.decisionGets)
	}
	noSessionAttempt := attemptRepo.attempts[started.ID]
	if noSessionAttempt.Status == "APPROVED" || len(noSessionAttempt.EncryptedDecision) != 0 || len(client.deleted) != 0 {
		t.Fatalf("session-less webhook had forbidden side effects: attempt=%+v deleted=%v", noSessionAttempt, client.deleted)
	}
	noSessionEvent := findEventByID(t, attemptRepo, noSessionInput.EventID)
	if noSessionEvent.ProcessingStatus != "FAILED" {
		t.Fatalf("session-less event should be FAILED: %+v", noSessionEvent)
	}
	assertUnverifiedClaim()
	attemptRepo.attempts[started.ID] = savedAttempt

	// With the authoritative GET recovered, the same FAILED event can be retried.
	client.decErr = nil
	client.decision = *webhookApproval
	client.decision.Status = "In Review"
	retryOutcome, retryErr := diditSvc.ProcessWebhook(ctx, eventInput)
	if retryErr != nil {
		t.Fatalf("retry same FAILED event after GET recovery: %v", retryErr)
	}
	if !retryOutcome.Duplicate {
		t.Fatalf("retry of previously stored FAILED event should report Duplicate=true, got %+v", retryOutcome)
	}
	retriedEvent := findEventByID(t, attemptRepo, eventInput.EventID)
	if retriedEvent.ProcessingStatus != "PROCESSED" {
		t.Fatalf("same event did not recover from FAILED: %+v", retriedEvent)
	}
	assertUnverifiedClaim()
	if attemptRepo.attempts[started.ID].Status == "APPROVED" || len(client.deleted) != 0 {
		t.Fatal("authoritative In Review must override webhook Approved without deleting the session")
	}

	// 8. ProcessWebhook with V3 nested arrays evaluates APPROVED, encrypts decision with AEAD, and deletes remote session.
	client.decision = DiditDecisionResponse{
		SessionID:  "didit-sess-001",
		WorkflowID: "wf-chile-id",
		Status:     "Approved",
		IDVerifications: []DiditIDVerificationNode{{
			Status:               "Approved",
			FirstName:            "Ana",
			LastName:             "Pérez",
			DateOfBirth:          "1990-01-02",
			IdentificationNumber: "12.345.670-K",
			IssuingState:         "CHL",
			DocumentType:         "Identity Card",
			ExpirationDate:       "2030-01-01",
		}},
		LivenessChecks: []DiditLivenessNode{{
			Status: "Approved",
			Score:  98.2,
		}},
		FaceMatches: []DiditFaceMatchNode{{
			Status: "Approved",
			Score:  95.4,
		}},
	}
	bodyHash := sha256.Sum256([]byte(`{"event_id":"evt-1"}`))
	outcome, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-1",
		EventType:         "status.updated",
		ProviderSessionID: "didit-sess-001",
		TopLevelStatus:    "Approved",
		BodySHA256:        bodyHash[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.AttemptStatus != "APPROVED" || !outcome.RemoteDeleted || len(client.deleted) != 1 || client.deleted[0] != "didit-sess-001" {
		t.Fatalf("expected APPROVED and immediate remote session deletion, got outcome=%+v deleted=%v", outcome, client.deleted)
	}

	// The authoritative commit keeps only status/claim/outbox metadata and purges the encrypted snapshot.
	storedAttempt := attemptRepo.attempts[started.ID]
	if len(storedAttempt.EncryptedDecision) != 0 || storedAttempt.DeclarationID != "" {
		t.Fatalf("verified result retained declaration/ciphertext: %+v", storedAttempt)
	}
	if len(attemptRepo.outbox) != 1 || attemptRepo.outbox[0] != "didit-sess-001" {
		t.Fatalf("verified result outbox = %v", attemptRepo.outbox)
	}
	if _, err := diditSvc.DecryptAttemptDecision(storedAttempt); !errors.Is(err, ErrNotFound) {
		t.Fatalf("verified result should not retain an encrypted decision; err=%v", err)
	}

	// 9. Duplicate webhook delivery is deduplicated without re-running.
	replayOutcome, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-1",
		EventType:         "status.updated",
		ProviderSessionID: "didit-sess-001",
		TopLevelStatus:    "Approved",
		BodySHA256:        bodyHash[:],
	})
	if err != nil || !replayOutcome.Duplicate {
		t.Fatalf("expected duplicate webhook to be flagged Duplicate=true, got %+v err=%v", replayOutcome, err)
	}

	// 10. Status view is APPROVED / VERIFIED.
	statusView, err := diditSvc.GetStatus(ctx, accountA, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if statusView.Status != "APPROVED" || statusView.BadgeResult != "VERIFIED" {
		t.Fatalf("expected APPROVED/VERIFIED, got %+v", statusView)
	}

	// 11. Saving a new declaration revision supersedes the previous attempt's badge result.
	newDecl, err := declSvc.Save(ctx, accountA, DeclarationPayload{
		FirstName:    "Ana María",
		LastName:     "Pérez",
		DateOfBirth:  "1990-01-02",
		RUT:          "12.345.670-K",
		IDCountry:    "CHL",
		DocumentType: "ID",
	}, true, "2026-11-v1")
	if err != nil || newDecl.ID == savedDecl.ID {
		t.Fatalf("expected new declaration revision, got %+v err=%v", newDecl, err)
	}
	supersededView, err := diditSvc.GetStatus(ctx, accountA, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if supersededView.BadgeResult != "NOT_VERIFIED" {
		t.Fatalf("expected superseded attempt to report NOT_VERIFIED badge, got %+v", supersededView)
	}
}

func TestDiditServiceRevocationAndRemoteDeleteRetryQueue(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	ids := &counterUUIDs{}
	keys := newTestKeyring(t)
	declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	declSvc, err := NewDeclarationService(declRepo, keys, ids, clock, func(t time.Time) time.Time {
		return t.Add(30 * 24 * time.Hour)
	}, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	attemptRepo := newMemProviderAttemptRepo()
	attemptRepo.declarations = declRepo
	client := &fakeDiditClient{
		createResp: CreateSessionResponse{
			SessionID:  "didit-sess-retry",
			SessionURL: "https://verification.didit.me/session/didit-sess-retry",
		},
		deleteErr: errors.New("temporary network error"),
	}
	rutClaims := NewMemoryRutClaimRegistry()
	attemptRepo.claims = rutClaims
	hmacSecret := []byte("0123456789abcdef0123456789abcdef")
	diditSvc, err := NewDiditService(attemptRepo, declSvc, client, keys, ids, clock, DiditServiceConfig{
		Environment:                 "SANDBOX",
		ApplicationID:               "espacigo-test-app",
		WorkflowID:                  "wf-chile-id",
		WorkflowVersion:             "1",
		ConsentVersion:              "2026-11-v1",
		AttemptTTL:                  30 * time.Minute,
		RequireLiveForVerifiedBadge: true,
		RutHMACSecret:               hmacSecret,
		RutHMACKeyVersion:           "v1",
	}, rutClaims)
	if err != nil {
		t.Fatal(err)
	}

	accountA := "33333333-3333-4333-8333-333333333333"
	_, err = declSvc.Save(ctx, accountA, DeclarationPayload{
		FirstName:    "Carlos",
		LastName:     "Díaz",
		DateOfBirth:  "1985-04-12",
		RUT:          "11.222.333-9",
		IDCountry:    "CHL",
		DocumentType: "ID",
	}, true, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	started, err := diditSvc.StartOrResume(ctx, accountA, "idemp-r1", StartDiditIntentInput{
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if err != nil {
		t.Fatal(err)
	}

	client.decision = DiditDecisionResponse{
		SessionID:  "didit-sess-retry",
		WorkflowID: "wf-chile-id",
		Status:     "Approved",
		IDVerifications: []DiditIDVerificationNode{{
			Status:               "Approved",
			FirstName:            "Carlos",
			LastName:             "Díaz",
			DateOfBirth:          "1985-04-12",
			IdentificationNumber: "11222333-9",
			IssuingState:         "CHL",
			DocumentType:         "ID",
		}},
		LivenessChecks: []DiditLivenessNode{{Status: "Approved"}},
		FaceMatches:    []DiditFaceMatchNode{{Status: "Approved"}},
	}
	hash1 := sha256.Sum256([]byte(`{"event_id":"evt-r1"}`))
	outcome, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-r1",
		EventType:         "status.updated",
		ProviderSessionID: "didit-sess-retry",
		TopLevelStatus:    "Approved",
		BodySHA256:        hash1[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.RemoteDeleted || len(diditSvc.PendingRemoteDeletions()) != 1 {
		t.Fatalf("expected failed remote delete to enqueue session for retry, got outcome=%+v pending=%v", outcome, diditSvc.PendingRemoteDeletions())
	}

	// Because RequireLiveForVerifiedBadge=true and environment=SANDBOX, badge is IN_REVIEW.
	stView, err := diditSvc.GetStatus(ctx, accountA, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stView.Status != "APPROVED" || stView.BadgeResult != "IN_REVIEW" {
		t.Fatalf("expected SANDBOX with RequireLiveForVerifiedBadge to return IN_REVIEW badge, got %+v", stView)
	}

	// Revocation webhook withdraws approval.
	hash2 := sha256.Sum256([]byte(`{"event_id":"evt-r2"}`))
	revOutcome, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-r2",
		EventType:         "user.status.updated",
		ProviderSessionID: "didit-sess-retry",
		TopLevelStatus:    "Revoked",
		BodySHA256:        hash2[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if revOutcome.AttemptStatus != "REJECTED" {
		t.Fatalf("expected REJECTED after Revoked webhook, got %+v", revOutcome)
	}

	// Recover provider deletion and verify retry removes the queued session.
	client.deleteErr = nil
	deleted, err := diditSvc.RetryPendingDeletions(ctx)
	if err != nil {
		t.Fatalf("RetryPendingDeletions after provider recovery: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "didit-sess-retry" || len(diditSvc.PendingRemoteDeletions()) != 0 {
		t.Fatalf("expected successful retry and empty queue, deleted=%v pending=%v", deleted, diditSvc.PendingRemoteDeletions())
	}
}

func TestDiditServiceLateEventsOnTerminalAttemptsAreIgnored(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	ids := &counterUUIDs{}
	keys := newTestKeyring(t)
	declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	declSvc, err := NewDeclarationService(declRepo, keys, ids, clock, func(t time.Time) time.Time {
		return t.Add(30 * 24 * time.Hour)
	}, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	attemptRepo := newMemProviderAttemptRepo()
	attemptRepo.declarations = declRepo
	client := &fakeDiditClient{
		createResp: CreateSessionResponse{
			SessionID:  "didit-sess-term",
			SessionURL: "https://verification.didit.me/session/didit-sess-term",
		},
	}
	rutClaims := NewMemoryRutClaimRegistry()
	attemptRepo.claims = rutClaims
	hmacSecret := []byte("0123456789abcdef0123456789abcdef")
	diditSvc, err := NewDiditService(attemptRepo, declSvc, client, keys, ids, clock, DiditServiceConfig{
		Environment:                 "SANDBOX",
		ApplicationID:               "espacigo-test-app",
		WorkflowID:                  "wf-chile-id",
		WorkflowVersion:             "1",
		ConsentVersion:              "2026-11-v1",
		AttemptTTL:                  30 * time.Minute,
		RequireLiveForVerifiedBadge: false,
		RutHMACSecret:               hmacSecret,
		RutHMACKeyVersion:           "v1",
	}, rutClaims)
	if err != nil {
		t.Fatal(err)
	}

	accountID := "44444444-4444-4444-8444-444444444444"
	_, err = declSvc.Save(ctx, accountID, DeclarationPayload{
		FirstName:    "Valeria",
		LastName:     "Morales",
		DateOfBirth:  "1992-07-20",
		RUT:          "12.345.670-K",
		IDCountry:    "CHL",
		DocumentType: "ID",
	}, true, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	started, err := diditSvc.StartOrResume(ctx, accountID, "idemp-term1", StartDiditIntentInput{
		ConsentGiven:   true,
		ConsentVersion: "2026-11-v1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Manually mark the attempt as REJECTED (e.g. from an earlier declined evaluation).
	saved := attemptRepo.attempts[started.ID]
	saved.Status = "REJECTED"
	attemptRepo.attempts[started.ID] = saved

	// 2. Deliver a late "Approved" webhook event with full approved decision.
	lateApprovedHash := sha256.Sum256([]byte(`{"event_id":"evt-late-approved"}`))
	outcome, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-late-approved",
		EventType:         "status.updated",
		ProviderSessionID: "didit-sess-term",
		TopLevelStatus:    "Approved",
		BodySHA256:        lateApprovedHash[:],
	})
	if err != nil {
		t.Fatalf("ProcessWebhook failed on late event: %v", err)
	}
	if outcome.AttemptStatus != "REJECTED" || outcome.ProcessingStatus != "IGNORED" {
		t.Fatalf("expected attempt to remain REJECTED and event IGNORED, got outcome: %+v", outcome)
	}
	evt := findEventByID(t, attemptRepo, "evt-late-approved")
	if evt.ProcessingStatus != "IGNORED" {
		t.Fatalf("expected stored event status IGNORED, got %s", evt.ProcessingStatus)
	}
	if len(client.deleted) != 0 {
		t.Fatalf("late event on REJECTED attempt must not delete remote session: %v", client.deleted)
	}
	if client.decisionGets != 0 {
		t.Fatalf("late event on REJECTED attempt must not make decision GETs: %d", client.decisionGets)
	}

	// 3. Test EXPIRED attempt ignores late intermediate events.
	saved.Status = "EXPIRED"
	attemptRepo.attempts[started.ID] = saved
	lateProgressHash := sha256.Sum256([]byte(`{"event_id":"evt-late-progress"}`))
	outcomeProgress, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-late-progress",
		EventType:         "status.updated",
		ProviderSessionID: "didit-sess-term",
		TopLevelStatus:    "In Progress",
		BodySHA256:        lateProgressHash[:],
	})
	if err != nil {
		t.Fatalf("ProcessWebhook failed on late In Progress event: %v", err)
	}
	if outcomeProgress.AttemptStatus != "EXPIRED" || outcomeProgress.ProcessingStatus != "IGNORED" {
		t.Fatalf("expected attempt to remain EXPIRED and event IGNORED, got %+v", outcomeProgress)
	}

	// 4. Test FAILED attempt ignores late events.
	saved.Status = "FAILED"
	attemptRepo.attempts[started.ID] = saved
	lateFailedHash := sha256.Sum256([]byte(`{"event_id":"evt-late-failed-test"}`))
	outcomeFailed, err := diditSvc.ProcessWebhook(ctx, DiditWebhookInput{
		EventID:           "evt-late-failed-test",
		EventType:         "status.updated",
		ProviderSessionID: "didit-sess-term",
		TopLevelStatus:    "Approved",
		BodySHA256:        lateFailedHash[:],
	})
	if err != nil {
		t.Fatalf("ProcessWebhook failed on late event for FAILED attempt: %v", err)
	}
	if outcomeFailed.AttemptStatus != "FAILED" || outcomeFailed.ProcessingStatus != "IGNORED" {
		t.Fatalf("expected attempt to remain FAILED and event IGNORED, got %+v", outcomeFailed)
	}
}
