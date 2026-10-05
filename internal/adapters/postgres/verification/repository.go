package verificationpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbgen "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification/dbgen"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: dbgen.New(pool)}
}

func (r *Repository) Create(ctx context.Context, item verification.Case) (verification.Case, error) {
	row, err := r.queries.CreateVerification(ctx, dbgen.CreateVerificationParams{
		ID: item.ID, OwnerID: item.OwnerID, Type: item.Type, EvidenceRef: item.EvidenceRef,
		IdempotencyKey: item.Idempotency, CreatedAt: dbTime(item.CreatedAt),
	})
	if err == nil {
		return fromCreate(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, mapDBError(err)
	}
	prior, err := r.queries.GetVerificationByIdempotency(ctx, dbgen.GetVerificationByIdempotencyParams{OwnerID: item.OwnerID, IdempotencyKey: item.Idempotency})
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return fromIdempotency(prior), nil
}

func (r *Repository) GetOwn(ctx context.Context, owner, id string) (verification.Case, error) {
	row, err := r.queries.GetOwnVerification(ctx, dbgen.GetOwnVerificationParams{OwnerID: owner, ID: id})
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return fromOwn(row), nil
}

func (r *Repository) ListOwn(ctx context.Context, owner string) ([]verification.Case, error) {
	rows, err := r.queries.ListOwnVerifications(ctx, owner)
	if err != nil {
		return nil, err
	}
	items := make([]verification.Case, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromOwnList(row))
	}
	return items, nil
}

func (r *Repository) ListPending(ctx context.Context) ([]verification.Case, error) {
	rows, err := r.queries.ListPendingVerifications(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]verification.Case, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromPending(row))
	}
	return items, nil
}

func (r *Repository) Review(ctx context.Context, id, reviewer string, approved bool, reason string) (verification.Case, error) {
	state := "rechazada"
	if approved {
		state = "aprobada"
	}
	row, err := r.queries.ReviewVerification(ctx, dbgen.ReviewVerificationParams{State: state, ReviewerID: reviewer, ReasonCode: reason, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, verification.ErrConflict
	}
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return fromReview(row), nil
}

func (r *Repository) Retry(ctx context.Context, owner, priorID string, item verification.Case) (verification.Case, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return verification.Case{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := r.queries.WithTx(tx)

	// Serialize retries for this case before checking the key. A concurrent retry
	// may have passed its first idempotency check while waiting on this row lock.
	prior, err := queries.LockPriorVerificationForRetry(ctx, dbgen.LockPriorVerificationForRetryParams{ID: priorID, OwnerID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, verification.ErrNotFound
	}
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if prior.State != "rechazada" {
		return verification.Case{}, verification.ErrConflict
	}
	// Recheck after acquiring the parent lock so same-case requests that raced
	// on the initial lookup return the one row committed by the first request.
	existing, err := queries.GetVerificationByIdempotency(ctx, dbgen.GetVerificationByIdempotencyParams{OwnerID: owner, IdempotencyKey: item.Idempotency})
	if err == nil {
		if retryParent(existing.RetryOf) != priorID {
			return verification.Case{}, verification.ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return verification.Case{}, mapDBError(err)
		}
		return fromIdempotency(existing), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, mapDBError(err)
	}
	row, err := queries.CreateVerificationRetry(ctx, dbgen.CreateVerificationRetryParams{
		ID: item.ID, OwnerID: owner, Type: prior.Type, EvidenceRef: item.EvidenceRef,
		IdempotencyKey: item.Idempotency, RetryOf: priorID, CreatedAt: dbTime(item.CreatedAt),
	})
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return fromRetry(row), nil
}

func dbTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func retryParent(value interface{}) string {
	switch result := value.(type) {
	case nil:
		return ""
	case string:
		return result
	case []byte:
		return string(result)
	default:
		return fmt.Sprint(result)
	}
}

func fromCreate(row dbgen.CreateVerificationRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}
func fromIdempotency(row dbgen.GetVerificationByIdempotencyRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}
func fromOwn(row dbgen.GetOwnVerificationRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}
func fromOwnList(row dbgen.ListOwnVerificationsRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}
func fromPending(row dbgen.ListPendingVerificationsRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}
func fromReview(row dbgen.ReviewVerificationRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}
func fromRetry(row dbgen.CreateVerificationRetryRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey}
}

func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return verification.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && (pgerr.Code == "23505" || pgerr.Code == "23514") {
		return verification.ErrConflict
	}
	return err
}
