package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func cleanupNativeApprovalHarness(t *testing.T, pool *pgxpool.Pool, repos []*nativeDeclarationIntentRepository, attemptRepo *nativeAttemptJournalRepository, attemptPath, dir string, baseline int64) {
	t.Helper()
	var declarations []nativeDeclarationIntent
	var paths []string
	for _, r := range repos {
		if !r.intentWritten {
			continue
		}
		if validateNativeIdentityPrivateFile(r.journalPath) != nil {
			t.Error("approval declaration journal metadata invalid; preserve artifacts")
			return
		}
		data, err := os.ReadFile(r.journalPath)
		var d nativeDeclarationIntent
		if err != nil || json.Unmarshal(data, &d) != nil || d != r.trustedIntent {
			t.Error("approval declaration journal differs from trusted input; preserve artifacts")
			return
		}
		declarations = append(declarations, d)
		paths = append(paths, r.journalPath)
	}
	var attempt *nativeAttemptIntent
	if attemptRepo != nil {
		if saved, ok := attemptRepo.Snapshot(); ok {
			disk, err := readNativeAttemptIntent(attemptPath)
			if err != nil || disk != saved {
				t.Error("approval attempt journal differs from trusted input; preserve artifacts")
				return
			}
			attempt = &saved
			paths = append(paths, attemptPath)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if len(declarations) > 0 {
		if err := cleanupNativeApprovalFixture(ctx, pool, declarations, attempt); err != nil {
			t.Error("approval fixture cleanup failed; preserve all journals")
			return
		}
	} else if attempt != nil {
		t.Error("approval attempt exists without trusted declaration")
		return
	}
	var rows int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND tipo='didit.session_remote_deletion'`).Scan(&rows); err != nil || rows != baseline {
		t.Error("approval outbox baseline differs; preserve journals")
		return
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			t.Error("approval journal retirement failed")
			return
		}
	}
	f, err := os.Open(dir)
	if err == nil {
		err = f.Sync()
		_ = f.Close()
	}
	if err != nil {
		t.Error("approval journal retirement sync failed")
	}
}

func verifyNativeApprovalHarness(t *testing.T, pool *pgxpool.Pool, repos []*nativeDeclarationIntentRepository, attempts *nativeAttemptJournalRepository, mode string, baseline int64) {
	t.Helper()
	intent, ok := attempts.Snapshot()
	if !attempts.RetentionObserved() {
		t.Error("native retention metadata was not observed before approval purge")
		return
	}
	if !ok || len(repos) == 0 || !repos[0].intentWritten {
		t.Error("approval gate missing journaled attempt/declaration")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Error("approval verification read-only transaction failed")
		return
	}
	defer tx.Rollback(ctx)
	var matching, allAttempts, decls, audits, events, processed, ignored, claims, outboxes, globalRows int64
	var state, claimDecl string
	var detached, noCipher, verified, acked, retained bool
	err = tx.QueryRow(ctx, `SELECT count(*) FROM public.identity_provider_attempt WHERE id=$1::uuid AND account_id=$2::uuid AND correlation_id=$3::uuid AND provider=$4 AND environment=$5 AND application_id=$6 AND workflow_id=$7 AND workflow_version=$8`, intent.AttemptID, intent.AccountID, intent.CorrelationID, intent.Provider, intent.Environment, intent.ApplicationID, intent.WorkflowID, intent.WorkflowVersion).Scan(&matching)
	if err != nil || matching != 1 {
		t.Error("approval attempt binding verification failed")
		return
	}
	err = tx.QueryRow(ctx, `SELECT status,declaration_id IS NULL,decision_key_version IS NULL AND decision_algorithm IS NULL AND encrypted_decision IS NULL FROM public.identity_provider_attempt WHERE id=$1::uuid AND account_id=$2::uuid`, intent.AttemptID, intent.AccountID).Scan(&state, &detached, &noCipher)
	if err != nil || !detached || !noCipher {
		t.Error("approval snapshot detachment/purge invariant failed")
		return
	}
	expectedState := "REJECTED"
	expectedDecls := int64(0)
	expectedAudits := int64(1)
	expectedEvents := int64(3)
	expectedProcessed := int64(2)
	expectedIgnored := int64(1)
	if mode == "supersession" {
		expectedState = "APPROVED"
		expectedDecls = 1
		expectedAudits = 2
		expectedEvents = 2
		expectedIgnored = 0
	}
	if state != expectedState {
		t.Error("approval terminal state differs from gate")
		return
	}
	err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid),(SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid),(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration'),(SELECT count(*) FROM public.identity_provider_event WHERE attempt_id=$2::uuid),(SELECT count(*) FROM public.identity_provider_event WHERE attempt_id=$2::uuid AND processing_status='PROCESSED' AND octet_length(body_sha256)=32),(SELECT count(*) FROM public.identity_provider_event WHERE attempt_id=$2::uuid AND processing_status='IGNORED' AND octet_length(body_sha256)=32),(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid)`, intent.AccountID, intent.AttemptID).Scan(&allAttempts, &decls, &audits, &events, &processed, &ignored, &claims)
	if err != nil || allAttempts != 1 || decls != expectedDecls || audits != expectedAudits || events != expectedEvents || processed != expectedProcessed || ignored != expectedIgnored || claims != 1 {
		t.Error("approval exact artifact/event count verification failed")
		return
	}
	var original int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.identity_declaration_revision WHERE id=$1::uuid`, intent.DeclarationID).Scan(&original); err != nil || original != 0 {
		t.Error("approved declaration snapshot remains")
		return
	}
	err = tx.QueryRow(ctx, `SELECT COALESCE(declaration_id::text,''),verified_at IS NOT NULL FROM public.identity_rut_claim WHERE account_id=$1::uuid`, intent.AccountID).Scan(&claimDecl, &verified)
	if err != nil {
		t.Error("approval claim readback failed")
		return
	}
	if mode == "supersession" {
		if len(repos) != 2 || !repos[1].intentWritten {
			t.Error("supersession requires its second trusted declaration")
			return
		}
		var newRetentionExact bool
		if err := tx.QueryRow(ctx, `SELECT retention_at=consent_given_at+interval '30 days' FROM public.identity_declaration_revision WHERE account_id=$1::uuid AND id=$2::uuid`, intent.AccountID, repos[1].trustedIntent.DeclarationID).Scan(&newRetentionExact); err != nil || !newRetentionExact {
			t.Error("superseding declaration retention differs from the candidate policy")
			return
		}
	}
	// A new declaration invalidates the effective approval in GetStatus, not the
	// permanent anti-reuse reservation. Saving must preserve that minimal claim.
	if claimDecl != "" || !verified {
		t.Error("approval transition unexpectedly changed retained minimal claim")
		return
	}
	var auditMatches int64
	for _, r := range repos {
		if !r.intentWritten {
			t.Error("approval expected declaration journal missing")
			return
		}
		d := r.trustedIntent
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration' AND recurso_id=$2::uuid AND accion='kyc.identity_declaration.save' AND correlacion_id=$3`, d.AccountID, d.DeclarationID, d.CorrelationID).Scan(&auditMatches); err != nil || auditMatches != 1 {
			t.Error("approval declaration audit binding failed")
			return
		}
	}
	err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_and(o.entregada_en IS NOT NULL),false),COALESCE(bool_and(o.retirar_en>=o.entregada_en+interval '30 days'),false) FROM public.outbox_evento_local o JOIN public.identity_provider_attempt a ON a.id=o.agregado_id WHERE a.id=$1::uuid AND a.account_id=$2::uuid AND o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion' AND o.clave_deduplicacion='didit_session_deletion:'||a.provider_session_id`, intent.AttemptID, intent.AccountID).Scan(&outboxes, &acked, &retained)
	if err != nil || outboxes != 1 || !acked || !retained {
		t.Error("approval remote deletion ACK/retention verification failed")
		return
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND tipo='didit.session_remote_deletion'`).Scan(&globalRows); err != nil || globalRows != baseline+1 {
		t.Error("approval global outbox delta is not exactly one")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		t.Error("approval read-only verification could not finish")
	}
}

func TestNativeApprovalDetachedCleanupBounds(t *testing.T) {
	for _, status := range []string{"APPROVED", "REJECTED", "EXPIRED"} {
		if !nativeApprovalDetachedStateAllowed(status, "", "", false) {
			t.Fatal("allowed detached terminal state rejected")
		}
		if nativeApprovalDetachedStateAllowed(status, "v1", "", false) || nativeApprovalDetachedStateAllowed(status, "", "AES-256-GCM", false) || nativeApprovalDetachedStateAllowed(status, "", "", true) {
			t.Fatal("detached ciphertext metadata accepted")
		}
	}
	for _, status := range []string{"", "CREATED", "PENDING_USER_INPUT", "IN_REVIEW", "PROCESSING", "FAILED"} {
		if nativeApprovalDetachedStateAllowed(status, "", "", false) {
			t.Fatal("unsafe detached status accepted")
		}
	}
	known := map[string]bool{"first": true, "second": true}
	if !nativeApprovalClaimScopeAllowed("second", false, known, true) || !nativeApprovalClaimScopeAllowed("", true, known, true) {
		t.Fatal("known supersession/retained claim rejected")
	}
	if nativeApprovalClaimScopeAllowed("other", true, known, true) || nativeApprovalClaimScopeAllowed("", false, known, true) || nativeApprovalClaimScopeAllowed("", true, known, false) {
		t.Fatal("untrusted claim accepted")
	}
	if cleanupNativeApprovalFixture(context.Background(), nil, nil, nil) == nil {
		t.Fatal("nil cleanup authority accepted")
	}
}
