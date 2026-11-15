package main

import (
	"context"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// runNativeRotationCommitRace proves the runtime SQL candidate waits behind an
// already-running approval, and can proceed when that candidate is rolled back.
// Both candidates are rolled back so the caller's normal repository commit sees
// the original RECEIVED event and pending attempt unchanged.
func runNativeRotationCommitRace(t *testing.T, ctx context.Context, appPool, otherPool *pgxpool.Pool, in verification.CommitWebhookDecisionInput) {
	t.Helper()
	const wait = 5 * time.Second
	bounded, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	a, err := appPool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		t.Fatal("begin first native approval candidate")
	}
	aOpen := true
	defer func() {
		if aOpen {
			_ = a.Rollback(context.Background())
		}
	}()
	var aPID int
	if err := a.QueryRow(bounded, `SELECT pg_backend_pid()`).Scan(&aPID); err != nil {
		t.Fatal("identify first approval transaction")
	}
	var aID string
	var originalDeclaration string
	if err := a.QueryRow(bounded, `SELECT declaration_id::text FROM public.identity_provider_attempt WHERE id=$1::uuid AND status='PENDING_USER_INPUT'`, in.AttemptID).Scan(&originalDeclaration); err != nil || originalDeclaration == "" {
		t.Fatal("capture original native approval declaration")
	}
	if err := a.QueryRow(bounded, `SELECT (public.commit_verified_identity_decision($1::uuid,$2::uuid,$3::text,$4::text,$5::timestamptz)).id::text`,
		in.AttemptID, in.EventRowID, in.RutFingerprint, in.DeleteSessionID, in.ProcessedAt,
	).Scan(&aID); err != nil || aID != in.AttemptID {
		t.Fatal("first native approval candidate did not return its own attempt")
	}

	b, err := otherPool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		t.Fatal("begin second native approval candidate")
	}
	bOpen := true
	defer func() {
		if bOpen {
			_ = b.Rollback(context.Background())
		}
	}()
	var bPID int
	if err := b.QueryRow(bounded, `SELECT pg_backend_pid()`).Scan(&bPID); err != nil {
		t.Fatal("identify second approval transaction")
	}
	if aPID == bPID {
		t.Fatal("native approval candidates unexpectedly share a backend")
	}

	type result struct {
		id  string
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		var id string
		err := b.QueryRow(bounded, `SELECT (public.commit_verified_identity_decision($1::uuid,$2::uuid,$3::text,$4::text,$5::timestamptz)).id::text`,
			in.AttemptID, in.EventRowID, in.RutFingerprint, in.DeleteSessionID, in.ProcessedAt,
		).Scan(&id)
		resultCh <- result{id: id, err: err}
	}()

	blockDeadline := time.Now().Add(wait)
	blocked := false
	for time.Now().Before(blockDeadline) {
		var pids []int
		if err := a.QueryRow(bounded, `SELECT pg_blocking_pids($1)`, bPID).Scan(&pids); err != nil {
			_ = a.Rollback(context.Background())
			aOpen = false
			<-resultCh
			t.Fatal("observe native approval lock wait")
		}
		for _, pid := range pids {
			if pid == aPID {
				blocked = true
				break
			}
		}
		if blocked {
			break
		}
		select {
		case <-resultCh:
			_ = a.Rollback(context.Background())
			aOpen = false
			t.Fatal("second native approval completed before blocking behind the first")
		case <-time.After(20 * time.Millisecond):
		case <-bounded.Done():
		}
	}
	if !blocked {
		_ = a.Rollback(context.Background())
		aOpen = false
		<-resultCh
		t.Fatal("second native approval did not report the first backend as blocker within five seconds")
	}
	if err := a.Rollback(bounded); err != nil {
		aOpen = false
		<-resultCh
		t.Fatal("rollback first native approval candidate")
	}
	aOpen = false

	select {
	case got := <-resultCh:
		if got.err != nil || got.id != in.AttemptID {
			_ = b.Rollback(context.Background())
			bOpen = false
			t.Fatal("second native approval candidate did not proceed successfully after first rollback")
		}
	case <-bounded.Done():
		// pgx transactions are not safe for concurrent calls. Cancellation
		// interrupts the in-flight query; join it before issuing Rollback.
		cancel()
		<-resultCh
		_ = b.Rollback(context.Background())
		bOpen = false
		t.Fatal("second native approval candidate did not finish within five seconds after first rollback")
	}
	if err := b.Rollback(bounded); err != nil {
		bOpen = false
		t.Fatal("rollback second native approval candidate")
	}
	bOpen = false

	var attemptStatus, eventStatus string
	var claimDeclaration *string
	var outboxCount int
	var pendingClaim, snapshotPresent bool
	if err := appPool.QueryRow(bounded, `SELECT a.status,e.processing_status,c.declaration_id::text,
	 (SELECT count(*) FROM public.outbox_evento_local o WHERE o.agregado_id=a.id OR o.clave_deduplicacion='didit_session_deletion:'||$3),
	 COALESCE(c.verified_at IS NULL AND c.expires_at IS NOT NULL AND c.declaration_id=a.declaration_id,false),
	 EXISTS(SELECT 1 FROM public.identity_declaration_revision d WHERE d.id=a.declaration_id AND d.account_id=a.account_id)
	 FROM public.identity_provider_attempt a
	 JOIN public.identity_provider_event e ON e.id=$2::uuid AND e.attempt_id=a.id
	 LEFT JOIN public.identity_rut_claim c ON c.account_id=a.account_id AND c.fingerprint=$4
	 WHERE a.id=$1::uuid`, in.AttemptID, in.EventRowID, in.DeleteSessionID, in.RutFingerprint).Scan(&attemptStatus, &eventStatus, &claimDeclaration, &outboxCount, &pendingClaim, &snapshotPresent); err != nil {
		t.Fatal("read back fixture after rolling back both candidates")
	}
	if attemptStatus != "PENDING_USER_INPUT" || eventStatus != "RECEIVED" || claimDeclaration == nil || *claimDeclaration != originalDeclaration || outboxCount != 0 || !pendingClaim || !snapshotPresent {
		t.Fatalf("native approval candidates did not fully roll back: attempt=%s event=%s outbox=%d", attemptStatus, eventStatus, outboxCount)
	}
	t.Log("PASS: two installed V93 runtime approval calls serialized on the same attempt; both rolled back fully before the ordinary service commit")
}
