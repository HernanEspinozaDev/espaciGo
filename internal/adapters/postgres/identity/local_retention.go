package postgres

import (
	"context"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
)

func (r *IdentityRepository) PrepareExpiredLocalRetentions(ctx context.Context, at time.Time, limit int) (privacy.RetentionPurgeResult, error) {
	var result privacy.RetentionPurgeResult
	if limit < 1 || limit > 500 {
		return result, privacy.ErrInvalid
	}
	err := r.pool.QueryRow(ctx, `SELECT terms_purged,rights_purged,quotes_purged,verifications_purged,files_queued
		FROM public.purge_expired_local_privacy_rows($1,$2)`, at.UTC(), limit).
		Scan(&result.TermsPurged, &result.RightsPurged, &result.QuotesPurged, &result.VerificationsPurged, &result.EvidenceFilesQueued)
	return result, mapError(err)
}

func (r *IdentityRepository) ClaimExpiredVerificationFiles(ctx context.Context, at time.Time, limit int) ([]privacy.SuppressionFile, error) {
	if limit < 1 || limit > 500 {
		return nil, privacy.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, `SELECT evidencia_id::text,archivo_id::text
		FROM public.claim_expired_local_verification_files($1,$2)`, at.UTC(), limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	files := make([]privacy.SuppressionFile, 0)
	for rows.Next() {
		var item privacy.SuppressionFile
		if err := rows.Scan(&item.EvidenceID, &item.StorageID); err != nil {
			return nil, mapError(err)
		}
		files = append(files, item)
	}
	return files, mapError(rows.Err())
}

func (r *IdentityRepository) CompleteExpiredVerificationFile(ctx context.Context, file privacy.SuppressionFile, at time.Time) error {
	var completed bool
	err := r.pool.QueryRow(ctx, `SELECT public.complete_expired_local_verification_file($1,$2)`, file.EvidenceID, at.UTC()).Scan(&completed)
	if err != nil {
		return mapError(err)
	}
	return nil
}

func (r *IdentityRepository) FailExpiredVerificationFile(ctx context.Context, file privacy.SuppressionFile, at time.Time) error {
	var deferred bool
	err := r.pool.QueryRow(ctx, `SELECT public.fail_expired_local_verification_file($1,$2)`, file.EvidenceID, at.UTC()).Scan(&deferred)
	return mapError(err)
}
