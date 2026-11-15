package main

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Observe metadata immediately before the real commit/purge. The observer is
// read-only: it never changes clocks, expiries, policy or the commit payload.
func readNativeApprovalRetention(ctx context.Context, pool *pgxpool.Pool, in verification.CommitWebhookDecisionInput, intent nativeAttemptIntent) error {
	if pool == nil || in.AttemptID != intent.AttemptID || !canonicalNativeUUID(in.EventRowID) || in.NextStatus != "APPROVED" || !in.PurgeDeclaration {
		return errors.New("retention observation requires the trusted approval commit")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var declaration30Days, attemptSame, eventSame, claimSame, claimPending bool
	err = tx.QueryRow(ctx, `SELECT d.retention_at=d.consent_given_at+interval '30 days',a.retention_at=d.retention_at,e.retention_at=a.retention_at,c.expires_at=d.retention_at,c.verified_at IS NULL
 FROM public.identity_declaration_revision d
 JOIN public.identity_provider_attempt a ON a.declaration_id=d.id AND a.account_id=d.account_id
 JOIN public.identity_provider_event e ON e.attempt_id=a.id
 JOIN public.identity_rut_claim c ON c.declaration_id=d.id AND c.account_id=d.account_id
 WHERE d.id=$1::uuid AND d.account_id=$2::uuid AND a.id=$3::uuid AND a.correlation_id=$4::uuid AND e.id=$5::uuid`, intent.DeclarationID, intent.AccountID, intent.AttemptID, intent.CorrelationID, in.EventRowID).Scan(&declaration30Days, &attemptSame, &eventSame, &claimSame, &claimPending)
	if err != nil {
		return err
	}
	if !declaration30Days || !attemptSame || !eventSame || !claimSame || !claimPending {
		return errors.New("native retention metadata differs from the configured candidate policy")
	}
	return tx.Commit(ctx)
}
