package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

func TestHTTPDiditClientBoundaries(t *testing.T) {
	var lastMethod, lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		switch r.URL.Path {
		case "/v3/session/sess-123/decision/":
			if r.Method != http.MethodGet {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_ = json.NewEncoder(w).Encode(DiditDecisionResponse{
				SessionID: "sess-123",
				Status:    "Approved",
			})
		case "/v3/session/sess-123/delete/":
			if r.Method != http.MethodDelete {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/v3/session/err-500/decision/":
			http.Error(w, "internal server error", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewHTTPDiditClient("api-key", "test-secret", false, time.Second)
	client.BaseURL = srv.URL
	ctx := context.Background()

	// 1. GetSessionDecision
	dec, err := client.GetSessionDecision(ctx, "sess-123")
	if err != nil || dec.SessionID != "sess-123" || dec.Status != "Approved" {
		t.Fatalf("unexpected decision: %+v, err: %v", dec, err)
	}
	if lastMethod != http.MethodGet || lastPath != "/v3/session/sess-123/decision/" {
		t.Fatalf("unexpected path/method: %s %s", lastMethod, lastPath)
	}

	// 2. GetSessionDecision server error
	_, err = client.GetSessionDecision(ctx, "err-500")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	// 3. DeleteSession validation boundaries
	if err := client.DeleteSession(ctx, "   "); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty/whitespace session id in DeleteSession")
	}

	// 4. DeleteSession success
	if err := client.DeleteSession(ctx, "  sess-123  "); err != nil {
		t.Fatalf("unexpected DeleteSession error: %v", err)
	}
	if lastMethod != http.MethodDelete || lastPath != "/v3/session/sess-123/delete/" {
		t.Fatalf("unexpected DeleteSession path/method: %s %s", lastMethod, lastPath)
	}
}

func TestMemorySessionDeletionQueueBoundaries(t *testing.T) {
	q := NewMemorySessionDeletionQueue()
	ctx := context.Background()

	// Initially empty
	pending, err := q.PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatalf("expected empty queue initially, got: %+v err: %v", pending, err)
	}

	// Enqueue unique sessions
	_ = q.EnqueueSessionDeletion(ctx, "sess-1")
	_ = q.EnqueueSessionDeletion(ctx, "sess-2")
	// Duplicate enqueue is idempotent
	_ = q.EnqueueSessionDeletion(ctx, "sess-1")

	pending, err = q.PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 2 {
		t.Fatalf("expected exactly 2 deduplicated sessions, got %d", len(pending))
	}

	// Acknowledge one
	_ = q.AcknowledgeSessionDeletion(ctx, "sess-1")
	pending, _ = q.PendingSessionDeletions(ctx)
	if len(pending) != 1 || pending[0] != "sess-2" {
		t.Fatalf("expected only sess-2 after acknowledge, got: %+v", pending)
	}

	// Acknowledge nonexistent is safe no-op
	_ = q.AcknowledgeSessionDeletion(ctx, "nonexistent")
	pending, _ = q.PendingSessionDeletions(ctx)
	if len(pending) != 1 {
		t.Fatalf("expected 1 session remaining, got: %d", len(pending))
	}

	// Concurrent duplicate enqueues settle to exactly one pending session.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := q.EnqueueSessionDeletion(ctx, "c-sess"); err != nil {
				t.Errorf("enqueue: %v", err)
			}
		}()
	}
	wg.Wait()
	pending, err = q.PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 2 || pending[0] != "sess-2" || pending[1] != "c-sess" {
		t.Fatalf("after concurrent duplicate enqueue, pending = %v, err = %v; want [sess-2 c-sess]", pending, err)
	}

	// Concurrent acknowledgements settle to no c-sess, preserving sess-2.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := q.AcknowledgeSessionDeletion(ctx, "c-sess"); err != nil {
				t.Errorf("acknowledge: %v", err)
			}
		}()
	}
	wg.Wait()
	pending, err = q.PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 1 || pending[0] != "sess-2" {
		t.Fatalf("after concurrent acknowledge, pending = %v, err = %v; want [sess-2]", pending, err)
	}
}

func TestDiditServiceDecryptAttemptDecisionBoundaries(t *testing.T) {
	accountID := "a0000000-0000-4000-8000-000000000001"
	attemptID := "b0000000-0000-4000-8000-000000000002"
	provider := "didit"
	env := "SANDBOX"

	keyring, err := declarationcrypto.New("k1", map[string][]byte{
		"k1": bytes.Repeat([]byte("a"), 32),
	})
	if err != nil {
		t.Fatal(err)
	}

	svc := &DiditService{
		keys: keyring,
	}

	// 1. Nil service
	var nilSvc *DiditService
	if _, err := nilSvc.DecryptAttemptDecision(ProviderAttempt{}); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for nil service")
	}

	// 2. Empty encrypted decision
	if _, err := svc.DecryptAttemptDecision(ProviderAttempt{DecisionAlgorithm: DecisionAlgorithmV1}); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for empty encrypted decision")
	}

	// 3. Wrong decision algorithm
	if _, err := svc.DecryptAttemptDecision(ProviderAttempt{EncryptedDecision: []byte("data"), DecisionAlgorithm: "legacy"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound for wrong decision algorithm")
	}

	// 4. Invalid AAD (non-canonical accountID)
	if _, err := svc.DecryptAttemptDecision(ProviderAttempt{
		AccountID:         "bad-account",
		ID:                attemptID,
		Provider:          provider,
		Environment:       env,
		EncryptedDecision: []byte("ciphertext"),
		DecisionAlgorithm: DecisionAlgorithmV1,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for invalid AAD accountID")
	}

	// 5. Successful encryption and decryption
	snapshot := EncryptedDecisionSnapshot{
		ProviderStatus:  "Approved",
		EvaluatedStatus: "Approved",
		LivenessPassed:  true,
		FaceMatchPassed: true,
		EvaluatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	plain, _ := json.Marshal(snapshot)
	aad, err := BuildDecisionAAD(accountID, attemptID, provider, env, DecisionSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	keyVer, ciphertext, err := keyring.Seal(aad, plain)
	if err != nil {
		t.Fatal(err)
	}

	attempt := ProviderAttempt{
		ID:                 attemptID,
		AccountID:          accountID,
		Provider:           provider,
		Environment:        env,
		DecisionKeyVersion: keyVer,
		DecisionAlgorithm:  DecisionAlgorithmV1,
		EncryptedDecision:  ciphertext,
	}

	decrypted, err := svc.DecryptAttemptDecision(attempt)
	if err != nil {
		t.Fatalf("unexpected decryption error: %v", err)
	}
	if decrypted.ProviderStatus != "Approved" || decrypted.EvaluatedStatus != "Approved" || !decrypted.LivenessPassed || !decrypted.FaceMatchPassed {
		t.Fatalf("decrypted snapshot mismatch: %+v", decrypted)
	}

	// 6. Corrupted ciphertext returns open error
	badAttempt := attempt
	badAttempt.EncryptedDecision = append([]byte(nil), ciphertext...)
	badAttempt.EncryptedDecision[len(badAttempt.EncryptedDecision)-1] ^= 0xff
	if _, err := svc.DecryptAttemptDecision(badAttempt); err == nil {
		t.Fatal("expected decryption error for tampered ciphertext")
	}
}

func TestMemoryRutClaimRegistryRevokeVerifiedIsSafeNoop(t *testing.T) {
	reg := NewMemoryRutClaimRegistry()
	reg.RevokeVerified(context.Background(), "acc-1", time.Now())
}
