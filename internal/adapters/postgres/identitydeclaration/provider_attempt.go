package identitydeclaration

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
)

const attemptSelectColumns = `
	id::text,
	account_id::text,
	COALESCE(declaration_id::text, ''),
	provider,
	environment,
	application_id,
	workflow_id,
	workflow_version,
	idempotency_key::text,
	correlation_id::text,
	COALESCE(provider_session_id, ''),
	status,
	COALESCE(decision_key_version, ''),
	COALESCE(decision_algorithm, ''),
	encrypted_decision,
	created_at,
	updated_at,
	retention_at`

func scanProviderAttempt(row pgx.Row) (verification.ProviderAttempt, error) {
	var out verification.ProviderAttempt
	err := row.Scan(
		&out.ID,
		&out.AccountID,
		&out.DeclarationID,
		&out.Provider,
		&out.Environment,
		&out.ApplicationID,
		&out.WorkflowID,
		&out.WorkflowVersion,
		&out.IdempotencyKey,
		&out.CorrelationID,
		&out.ProviderSessionID,
		&out.Status,
		&out.DecisionKeyVersion,
		&out.DecisionAlgorithm,
		&out.EncryptedDecision,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.RetentionAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return verification.ProviderAttempt{}, verification.ErrNotFound
	}
	if err != nil {
		return verification.ProviderAttempt{}, safeDBError(err)
	}
	return out, nil
}

// PrepareAttempt expires stale CREATED/PENDING_USER_INPUT attempts and attempts bound to superseded
// declaration revisions, returns an existing active/idempotent attempt when present, or inserts a new CREATED attempt.
func (r *Repository) PrepareAttempt(ctx context.Context, in verification.PrepareAttemptInput) (verification.ProviderAttempt, bool, error) {
	if r == nil || r.pool == nil {
		return verification.ProviderAttempt{}, false, verification.ErrInvalid
	}
	if !canonicalUUID(in.ID) || !canonicalUUID(in.AccountID) || !canonicalUUID(in.DeclarationID) || !canonicalUUID(in.IdempotencyKey) || !canonicalUUID(in.CorrelationID) {
		return verification.ProviderAttempt{}, false, verification.ErrInvalid
	}
	if in.Provider != verification.DiditProviderName || (in.Environment != "SANDBOX" && in.Environment != "LIVE") || in.ApplicationID == "" || in.WorkflowID == "" || in.WorkflowVersion == "" || in.Now.IsZero() || !in.RetentionAt.After(in.Now) {
		return verification.ProviderAttempt{}, false, verification.ErrInvalid
	}
	ttl := in.AttemptTTL
	if ttl <= 0 {
		ttl = verification.DefaultDiditAttemptTTL
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return verification.ProviderAttempt{}, false, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 1))`, in.AccountID); err != nil {
		return verification.ProviderAttempt{}, false, err
	}

	cutoff := in.Now.Add(-ttl)
	// Expire stale CREATED/PENDING_USER_INPUT attempts or attempts belonging to older declaration revisions.
	if _, err := tx.Exec(ctx, `
		UPDATE public.identity_provider_attempt
		SET status = 'EXPIRED', updated_at = GREATEST(created_at, $2)
		WHERE account_id = $1
		  AND provider = $3
		  AND environment = $4
		  AND application_id = $5
		  AND (
		    (status IN ('CREATED', 'PENDING_USER_INPUT') AND created_at <= $6)
		    OR (declaration_id <> $7 AND status IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW'))
		  )`,
		in.AccountID, in.Now, in.Provider, in.Environment, in.ApplicationID, cutoff, in.DeclarationID,
	); err != nil {
		return verification.ProviderAttempt{}, false, safeDBError(err)
	}

	// Check idempotent replay first.
	existing, err := scanProviderAttempt(tx.QueryRow(ctx, `
		SELECT `+attemptSelectColumns+`
		FROM public.identity_provider_attempt
		WHERE provider = $1 AND environment = $2 AND application_id = $3 AND idempotency_key = $4`,
		in.Provider, in.Environment, in.ApplicationID, in.IdempotencyKey,
	))
	if err == nil {
		if existing.AccountID != in.AccountID || existing.DeclarationID != in.DeclarationID {
			return verification.ProviderAttempt{}, false, verification.ErrConflict
		}
		if existing.Status == "CREATED" || existing.Status == "PENDING_USER_INPUT" || existing.Status == "PROCESSING" || existing.Status == "IN_REVIEW" || existing.Status == "APPROVED" {
			if err := tx.Commit(ctx); err != nil {
				return verification.ProviderAttempt{}, false, err
			}
			return existing, true, nil
		}
	} else if !errors.Is(err, verification.ErrNotFound) {
		return verification.ProviderAttempt{}, false, err
	}

	// Check if an active attempt already exists for this (account_id, declaration_id, provider, environment, application_id).
	active, err := scanProviderAttempt(tx.QueryRow(ctx, `
		SELECT `+attemptSelectColumns+`
		FROM public.identity_provider_attempt
		WHERE account_id = $1 AND declaration_id = $2 AND provider = $3 AND environment = $4 AND application_id = $5
		  AND status IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW')
		LIMIT 1`,
		in.AccountID, in.DeclarationID, in.Provider, in.Environment, in.ApplicationID,
	))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return verification.ProviderAttempt{}, false, err
		}
		return active, true, nil
	} else if !errors.Is(err, verification.ErrNotFound) {
		return verification.ProviderAttempt{}, false, err
	}

	created, err := scanProviderAttempt(tx.QueryRow(ctx, `
		INSERT INTO public.identity_provider_attempt (
			id, account_id, declaration_id, provider, environment, application_id,
			workflow_id, workflow_version, idempotency_key, correlation_id,
			provider_session_id, status, decision_key_version, decision_algorithm, encrypted_decision,
			created_at, updated_at, retention_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			NULL, 'CREATED', NULL, NULL, NULL,
			$11, $11, $12
		)
		RETURNING `+attemptSelectColumns,
		in.ID, in.AccountID, in.DeclarationID, in.Provider, in.Environment, in.ApplicationID,
		in.WorkflowID, in.WorkflowVersion, in.IdempotencyKey, in.CorrelationID,
		in.Now, in.RetentionAt,
	))
	if err != nil {
		return verification.ProviderAttempt{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return verification.ProviderAttempt{}, false, err
	}
	return created, false, nil
}

func (r *Repository) MarkAttemptSessionReady(ctx context.Context, attemptID, providerSessionID string, updatedAt time.Time) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || !canonicalUUID(attemptID) || strings.TrimSpace(providerSessionID) == "" || updatedAt.IsZero() {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	return scanProviderAttempt(r.pool.QueryRow(ctx, `
		UPDATE public.identity_provider_attempt
		SET provider_session_id = $2, status = 'PENDING_USER_INPUT', updated_at = GREATEST(updated_at, $3)
		WHERE id = $1
		  AND status = 'CREATED'
		  AND declaration_id IS NOT NULL
		  AND provider_session_id IS NULL
		  AND retention_at > $3
		RETURNING `+attemptSelectColumns,
		attemptID, strings.TrimSpace(providerSessionID), updatedAt,
	))
}

func (r *Repository) MarkAttemptFailed(ctx context.Context, attemptID string, updatedAt time.Time) error {
	if r == nil || r.pool == nil || !canonicalUUID(attemptID) || updatedAt.IsZero() {
		return verification.ErrInvalid
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE public.identity_provider_attempt
		SET status = 'FAILED', updated_at = GREATEST(created_at, $2)
		WHERE id = $1 AND status IN ('CREATED', 'PENDING_USER_INPUT')`,
		attemptID, updatedAt,
	)
	return safeDBError(err)
}

func (r *Repository) GetAttemptForAccount(ctx context.Context, accountID, attemptID string, now time.Time, attemptTTL time.Duration) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || !canonicalUUID(accountID) || !canonicalUUID(attemptID) || now.IsZero() {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	if attemptTTL <= 0 {
		attemptTTL = verification.DefaultDiditAttemptTTL
	}
	attempt, err := scanProviderAttempt(r.pool.QueryRow(ctx, `
		SELECT `+attemptSelectColumns+`
		FROM public.identity_provider_attempt
		WHERE id = $1 AND account_id = $2`,
		attemptID, accountID,
	))
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	if !attempt.RetentionAt.After(now) {
		return verification.ProviderAttempt{}, verification.ErrNotFound
	}
	if (attempt.Status == "CREATED" || attempt.Status == "PENDING_USER_INPUT") && !attempt.CreatedAt.Add(attemptTTL).After(now) {
		expired, expErr := scanProviderAttempt(r.pool.QueryRow(ctx, `
			UPDATE public.identity_provider_attempt
			SET status = 'EXPIRED', updated_at = GREATEST(created_at, $2)
			WHERE id = $1 AND account_id = $3 AND status IN ('CREATED', 'PENDING_USER_INPUT')
			RETURNING `+attemptSelectColumns,
			attemptID, now, accountID,
		))
		if expErr == nil {
			return expired, nil
		}
	}
	return attempt, nil
}

// GetLatestAttemptForAccount reads the newest retained attempt within the exact provider scope.
func (r *Repository) GetLatestAttemptForAccount(ctx context.Context, accountID, provider, environment, applicationID string, now time.Time) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || !canonicalUUID(accountID) || provider == "" || environment == "" || applicationID == "" || now.IsZero() {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	return scanProviderAttempt(r.pool.QueryRow(ctx, `
		SELECT `+attemptSelectColumns+`
		FROM public.identity_provider_attempt
		WHERE account_id = $1 AND provider = $2 AND environment = $3 AND application_id = $4
		  AND retention_at > $5
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, accountID, provider, environment, applicationID, now))
}

func (r *Repository) FindAttemptForWebhook(ctx context.Context, provider, environment, applicationID, correlationID, providerSessionID string) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || provider == "" || environment == "" || applicationID == "" {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	trimmedCorr := strings.TrimSpace(correlationID)
	trimmedSess := strings.TrimSpace(providerSessionID)
	if canonicalUUID(trimmedCorr) {
		attempt, err := scanProviderAttempt(r.pool.QueryRow(ctx, `
			SELECT `+attemptSelectColumns+`
			FROM public.identity_provider_attempt
			WHERE provider = $1 AND environment = $2 AND application_id = $3 AND correlation_id = $4`,
			provider, environment, applicationID, trimmedCorr,
		))
		if err == nil {
			return attempt, nil
		}
		if !errors.Is(err, verification.ErrNotFound) {
			return verification.ProviderAttempt{}, err
		}
	}
	if trimmedSess != "" {
		return scanProviderAttempt(r.pool.QueryRow(ctx, `
			SELECT `+attemptSelectColumns+`
			FROM public.identity_provider_attempt
			WHERE provider = $1 AND environment = $2 AND application_id = $3 AND provider_session_id = $4`,
			provider, environment, applicationID, trimmedSess,
		))
	}
	return verification.ProviderAttempt{}, verification.ErrNotFound
}

func (r *Repository) InsertOrGetWebhookEvent(ctx context.Context, in verification.WebhookEventRecordInput) (verification.ProviderEvent, bool, error) {
	if r == nil || r.pool == nil || !canonicalUUID(in.ID) || !canonicalUUID(in.AttemptID) || len(in.BodySHA256) != 32 || in.EventID == "" || in.ReceivedAt.IsZero() || !in.RetentionAt.After(in.ReceivedAt) {
		return verification.ProviderEvent{}, false, verification.ErrInvalid
	}
	var out verification.ProviderEvent
	err := r.pool.QueryRow(ctx, `
		INSERT INTO public.identity_provider_event (
			id, attempt_id, provider, environment, application_id,
			event_id, event_type, body_sha256, processing_status,
			received_at, processed_at, retention_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'RECEIVED', $9, NULL, $10)
		ON CONFLICT (provider, environment, application_id, event_id) DO NOTHING
		RETURNING id::text, attempt_id::text, provider, environment, application_id, event_id, event_type, body_sha256, processing_status, received_at, processed_at, retention_at`,
		in.ID, in.AttemptID, in.Provider, in.Environment, in.ApplicationID,
		in.EventID, in.EventType, in.BodySHA256, in.ReceivedAt, in.RetentionAt,
	).Scan(
		&out.ID, &out.AttemptID, &out.Provider, &out.Environment, &out.ApplicationID,
		&out.EventID, &out.EventType, &out.BodySHA256, &out.ProcessingStatus,
		&out.ReceivedAt, &out.ProcessedAt, &out.RetentionAt,
	)
	if err == nil {
		return out, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return verification.ProviderEvent{}, false, safeDBError(err)
	}
	err = r.pool.QueryRow(ctx, `
		SELECT id::text, attempt_id::text, provider, environment, application_id, event_id, event_type, body_sha256, processing_status, received_at, processed_at, retention_at
		FROM public.identity_provider_event
		WHERE provider = $1 AND environment = $2 AND application_id = $3 AND event_id = $4`,
		in.Provider, in.Environment, in.ApplicationID, in.EventID,
	).Scan(
		&out.ID, &out.AttemptID, &out.Provider, &out.Environment, &out.ApplicationID,
		&out.EventID, &out.EventType, &out.BodySHA256, &out.ProcessingStatus,
		&out.ReceivedAt, &out.ProcessedAt, &out.RetentionAt,
	)
	if err != nil {
		return verification.ProviderEvent{}, false, safeDBError(err)
	}
	return out, true, nil
}

func (r *Repository) SaveAttemptDecisionAndCompleteEvent(ctx context.Context, attemptID, eventRowID, nextStatus, decisionKeyVersion, decisionAlgorithm string, encryptedDecision []byte, processedAt time.Time) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || !canonicalUUID(attemptID) || !canonicalUUID(eventRowID) || !keyVersionPattern.MatchString(decisionKeyVersion) || decisionAlgorithm != verification.DecisionAlgorithmV1 || len(encryptedDecision) < 29 || len(encryptedDecision) > 65564 || processedAt.IsZero() {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	defer tx.Rollback(ctx)

	updated, err := scanProviderAttempt(tx.QueryRow(ctx, `
		UPDATE public.identity_provider_attempt
		SET status = $2,
		    decision_key_version = $3,
		    decision_algorithm = $4,
		    encrypted_decision = $5,
		    updated_at = GREATEST(created_at, $6)
		WHERE id = $1
		  AND (status IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW') OR status = $2)
		RETURNING `+attemptSelectColumns,
		attemptID, nextStatus, decisionKeyVersion, decisionAlgorithm, encryptedDecision, processedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		var currentStatus string
		if checkErr := tx.QueryRow(ctx, `SELECT status FROM public.identity_provider_attempt WHERE id = $1`, attemptID).Scan(&currentStatus); checkErr == nil {
			return verification.ProviderAttempt{}, verification.ErrConflict
		}
		return verification.ProviderAttempt{}, verification.ErrNotFound
	}
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE public.identity_provider_event
		SET processing_status = 'PROCESSED', processed_at = GREATEST(received_at, $2)
		WHERE id = $1`,
		eventRowID, processedAt,
	); err != nil {
		return verification.ProviderAttempt{}, safeDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return verification.ProviderAttempt{}, err
	}
	return updated, nil
}

func (r *Repository) CommitWebhookDecision(ctx context.Context, in verification.CommitWebhookDecisionInput) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || !canonicalUUID(in.AttemptID) || !canonicalUUID(in.EventRowID) || in.ProcessedAt.IsZero() {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	if in.NextStatus == "APPROVED" {
		if !in.PurgeDeclaration || in.RutFingerprint == "" || strings.TrimSpace(in.DeleteSessionID) == "" {
			return verification.ProviderAttempt{}, verification.ErrInvalid
		}
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return verification.ProviderAttempt{}, err
		}
		defer tx.Rollback(ctx)
		// Read one field from the VOLATILE composite-returning function. Expanding
		// (function()).* may evaluate it once per output column and repeat mutations.
		var committedID string
		if err := tx.QueryRow(ctx, `SELECT (public.commit_verified_identity_decision($1, $2, $3, $4, $5)).id::text`,
			in.AttemptID, in.EventRowID, in.RutFingerprint, strings.TrimSpace(in.DeleteSessionID), in.ProcessedAt,
		).Scan(&committedID); err != nil {
			return verification.ProviderAttempt{}, safeDBError(err)
		}
		if !canonicalUUID(committedID) {
			return verification.ProviderAttempt{}, verification.ErrInvalid
		}
		attempt, err := scanProviderAttempt(tx.QueryRow(ctx, `SELECT `+attemptSelectColumns+`
			FROM public.identity_provider_attempt WHERE id=$1::uuid`, committedID))
		if err != nil {
			return verification.ProviderAttempt{}, safeDBError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return verification.ProviderAttempt{}, err
		}
		return attempt, nil
	}
	if in.PurgeDeclaration || in.RutFingerprint != "" || in.DeleteSessionID != "" {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	return r.SaveAttemptDecisionAndCompleteEvent(ctx, in.AttemptID, in.EventRowID, in.NextStatus, in.DecisionKeyVersion, in.DecisionAlgorithm, in.EncryptedDecision, in.ProcessedAt)
}

func (r *Repository) UpdateAttemptStatusAndCompleteEvent(ctx context.Context, attemptID, eventRowID, nextStatus, eventProcessingStatus string, processedAt time.Time) (verification.ProviderAttempt, error) {
	if r == nil || r.pool == nil || !canonicalUUID(attemptID) || !canonicalUUID(eventRowID) || processedAt.IsZero() {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	defer tx.Rollback(ctx)

	updated, err := scanProviderAttempt(tx.QueryRow(ctx, `
		UPDATE public.identity_provider_attempt
		SET status = $2, updated_at = GREATEST(created_at, $3)
		WHERE id = $1
		  AND (
		      ($2 IN ('EXPIRED', 'REJECTED') AND status NOT IN ('EXPIRED', 'FAILED'))
		      OR
		      (status IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW') OR status = $2)
		  )
		RETURNING `+attemptSelectColumns,
		attemptID, nextStatus, processedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		var currentStatus string
		if checkErr := tx.QueryRow(ctx, `SELECT status FROM public.identity_provider_attempt WHERE id = $1`, attemptID).Scan(&currentStatus); checkErr == nil {
			return verification.ProviderAttempt{}, verification.ErrConflict
		}
		return verification.ProviderAttempt{}, verification.ErrNotFound
	}
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE public.identity_provider_event
		SET processing_status = $2, processed_at = GREATEST(received_at, $3)
		WHERE id = $1`,
		eventRowID, eventProcessingStatus, processedAt,
	); err != nil {
		return verification.ProviderAttempt{}, safeDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return verification.ProviderAttempt{}, err
	}
	return updated, nil
}

func (r *Repository) CompleteWebhookEvent(ctx context.Context, eventRowID, processingStatus string, processedAt time.Time) error {
	if r == nil || r.pool == nil || !canonicalUUID(eventRowID) || processedAt.IsZero() {
		return verification.ErrInvalid
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE public.identity_provider_event
		SET processing_status = $2, processed_at = GREATEST(received_at, $3)
		WHERE id = $1`,
		eventRowID, processingStatus, processedAt,
	)
	return safeDBError(err)
}
