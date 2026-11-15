package identitydeclaration

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSessionDeletionQueueInputValidation(t *testing.T) {
	ctx := context.Background()
	var q *PostgresSessionDeletionQueue

	if err := q.EnqueueSessionDeletion(ctx, "sess-1"); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("EnqueueSessionDeletion nil queue err=%v, want ErrInvalid", err)
	}
	if _, err := q.PendingSessionDeletions(ctx); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("PendingSessionDeletions nil queue err=%v, want ErrInvalid", err)
	}
	if err := q.AcknowledgeSessionDeletion(ctx, "sess-1"); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("AcknowledgeSessionDeletion nil queue err=%v, want ErrInvalid", err)
	}

	queue := NewPostgresSessionDeletionQueue(nil)
	if err := queue.EnqueueSessionDeletion(ctx, "   "); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("EnqueueSessionDeletion empty session err=%v, want ErrInvalid", err)
	}
	if err := queue.EnqueueSessionDeletion(ctx, strings.Repeat("x", 101)); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("EnqueueSessionDeletion overlong session err=%v, want ErrInvalid", err)
	}
	if err := queue.AcknowledgeSessionDeletion(ctx, "   "); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("AcknowledgeSessionDeletion empty session err=%v, want ErrInvalid", err)
	}
}

func resolveRuntimeDSN(t *testing.T) string {
	t.Helper()
	return nativeTestDSNFromEnv(os.Getenv, "IDENTITY_DECLARATION_NATIVE_TEST_DSN")
}

func TestSessionDeletionQueuePostgresLifecycle(t *testing.T) {
	dsn := resolveRuntimeDSN(t)
	if dsn == "" {
		t.Skip("native outbox lifecycle test not explicitly enabled")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("failed to parse pool config: %v", err)
	}
	config.MaxConns = 2
	config.AfterConnect = func(connectCtx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(connectCtx, "SET ROLE espacigo_runtime")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	sessA := "didit-sess-unit-a-" + uuid.New().String()
	sessB := "didit-sess-unit-b-" + uuid.New().String()

	queue := NewPostgresSessionDeletionQueue(pool)

	// 1. Enqueue session A
	if err := queue.EnqueueSessionDeletion(ctx, sessA); err != nil {
		t.Fatalf("failed to enqueue sessA: %v", err)
	}

	// 2. Enqueue session A again (idempotent replay)
	if err := queue.EnqueueSessionDeletion(ctx, sessA); err != nil {
		t.Fatalf("failed to re-enqueue sessA idempotently: %v", err)
	}

	// 3. Enqueue session B
	if err := queue.EnqueueSessionDeletion(ctx, sessB); err != nil {
		t.Fatalf("failed to enqueue sessB: %v", err)
	}

	// 4. Query pending sessions: must contain both sessA and sessB
	pending, err := queue.PendingSessionDeletions(ctx)
	if err != nil {
		t.Fatalf("failed to query pending sessions: %v", err)
	}
	hasA, hasB := false, false
	for _, s := range pending {
		if s == sessA {
			hasA = true
		}
		if s == sessB {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Fatalf("expected both sessions pending, got %v (hasA=%v, hasB=%v)", pending, hasA, hasB)
	}

	// 5. Acknowledge session A
	if err := queue.AcknowledgeSessionDeletion(ctx, sessA); err != nil {
		t.Fatalf("failed to acknowledge sessA: %v", err)
	}

	// 6. Query pending sessions: sessA must no longer be pending, sessB must remain
	pendingAfter, err := queue.PendingSessionDeletions(ctx)
	if err != nil {
		t.Fatalf("failed to query pending sessions after ack: %v", err)
	}
	for _, s := range pendingAfter {
		if s == sessA {
			t.Fatalf("acknowledged session A still appears in pending list: %v", pendingAfter)
		}
	}
	hasBAfter := false
	for _, s := range pendingAfter {
		if s == sessB {
			hasBAfter = true
		}
	}
	if !hasBAfter {
		t.Fatalf("session B must remain pending after session A ack, got %v", pendingAfter)
	}

	// 7. Acknowledge session B
	if err := queue.AcknowledgeSessionDeletion(ctx, sessB); err != nil {
		t.Fatalf("failed to acknowledge sessB: %v", err)
	}

	// 8. Re-acknowledging session A or B is idempotent
	if err := queue.AcknowledgeSessionDeletion(ctx, sessA); err != nil {
		t.Fatalf("re-acknowledging sessA failed: %v", err)
	}
}
