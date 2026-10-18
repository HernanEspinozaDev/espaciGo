package postgres

import (
	"context"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuppressionCleanupWorkerRepository exposes the narrow V64 finalizer only
// to the dedicated worker process. API repositories retain FinishSuppression.
type SuppressionCleanupWorkerRepository interface {
	privacy.Repository
	privacy.SuppressionCleanupFinalizer
}

type suppressionCleanupWorkerRepository struct {
	*IdentityRepository
}

func NewSuppressionCleanupWorkerRepository(pool *pgxpool.Pool) SuppressionCleanupWorkerRepository {
	return &suppressionCleanupWorkerRepository{IdentityRepository: NewIdentityRepository(pool)}
}

func (r *suppressionCleanupWorkerRepository) FinalizeSuppressionCleanup(ctx context.Context, requestID string, at time.Time) error {
	if !suppressionClaimUUIDPattern.MatchString(requestID) {
		return privacy.ErrInvalid
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	var finalized bool
	if err := r.pool.QueryRow(ctx, `SELECT public.finalize_pending_suppression_cleanup($1,$2)`, requestID, at.UTC()).Scan(&finalized); err != nil {
		return mapError(err)
	}
	return nil
}
