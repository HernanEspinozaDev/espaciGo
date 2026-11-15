package identitydeclaration

import (
	"context"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	diditSessionAggregateType = "didit_session"
	diditSessionDeletionType  = "didit.session_remote_deletion"
	diditSessionDedupPrefix   = "didit_session_deletion:"
)

// PostgresSessionDeletionQueue persists pending remote session deletions in public.outbox_evento_local
// under the runtime role, providing a crash-durable outbox that survives process restarts.
type PostgresSessionDeletionQueue struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPostgresSessionDeletionQueue(pool *pgxpool.Pool, now ...func() time.Time) *PostgresSessionDeletionQueue {
	n := func() time.Time { return time.Now().UTC() }
	if len(now) > 0 && now[0] != nil {
		n = now[0]
	}
	return &PostgresSessionDeletionQueue{
		pool: pool,
		now:  n,
	}
}

func (q *PostgresSessionDeletionQueue) EnqueueSessionDeletion(ctx context.Context, sessionID string) error {
	if q == nil || q.pool == nil {
		return verification.ErrInvalid
	}
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" || len(trimmed) > 100 {
		return verification.ErrInvalid
	}
	return enqueueSessionDeletion(ctx, q.pool, trimmed, q.now().UTC())
}

type sessionDeletionExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func enqueueSessionDeletion(ctx context.Context, exec sessionDeletionExecutor, sessionID string, now time.Time) error {
	trimmed := strings.TrimSpace(sessionID)
	if exec == nil || trimmed == "" || len(trimmed) > 100 || now.IsZero() {
		return verification.ErrInvalid
	}
	dedupKey := diditSessionDedupPrefix + trimmed
	_, err := exec.Exec(ctx, `
		INSERT INTO public.outbox_evento_local (
			id, agregado_tipo, agregado_id, tipo, clave_deduplicacion,
			version, creada_en, disponible_en
		) VALUES ($1, $2, $3, $4, $5, 1, $6, $6)
		ON CONFLICT (clave_deduplicacion) DO NOTHING`,
		uuid.New(), diditSessionAggregateType, uuid.New(), diditSessionDeletionType, dedupKey, now,
	)
	return safeDBError(err)
}

func (q *PostgresSessionDeletionQueue) PendingSessionDeletions(ctx context.Context) ([]string, error) {
	if q == nil || q.pool == nil {
		return nil, verification.ErrInvalid
	}
	rows, err := q.pool.Query(ctx, `
		SELECT clave_deduplicacion
		FROM public.outbox_evento_local
		WHERE agregado_tipo = $1
		  AND tipo = $2
		  AND entregada_en IS NULL
		  AND cancelada_en IS NULL
		  AND fallo_terminal_en IS NULL
		ORDER BY creada_en ASC
		LIMIT 500`,
		diditSessionAggregateType, diditSessionDeletionType,
	)
	if err != nil {
		return nil, safeDBError(err)
	}
	defer rows.Close()

	var sessions []string
	for rows.Next() {
		var dedupKey string
		if err := rows.Scan(&dedupKey); err != nil {
			return nil, safeDBError(err)
		}
		sessionID := strings.TrimPrefix(dedupKey, diditSessionDedupPrefix)
		if sessionID != "" {
			sessions = append(sessions, sessionID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, safeDBError(err)
	}
	return sessions, nil
}

func (q *PostgresSessionDeletionQueue) AcknowledgeSessionDeletion(ctx context.Context, sessionID string) error {
	if q == nil || q.pool == nil {
		return verification.ErrInvalid
	}
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return verification.ErrInvalid
	}
	dedupKey := diditSessionDedupPrefix + trimmed
	now := q.now().UTC()

	_, err := q.pool.Exec(ctx, `
		UPDATE public.outbox_evento_local
		SET entregada_en = $2::timestamptz,
		    retirar_en = $2::timestamptz + interval '30 days'
		WHERE agregado_tipo = $3
		  AND tipo = $4
		  AND clave_deduplicacion = $1
		  AND entregada_en IS NULL`,
		dedupKey, now, diditSessionAggregateType, diditSessionDeletionType,
	)
	return safeDBError(err)
}
