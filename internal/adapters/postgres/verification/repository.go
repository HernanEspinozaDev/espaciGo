package verificationpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
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

// lockActiveOwner is the shared serialization point with local suppression.
// Every owner-side verification/evidence mutation acquires this row before any
// verification row, then revalidates the account state inside its transaction.
func lockActiveOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return mapDBError(err)
	}
	if !active {
		return verification.ErrNotFound
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, item verification.Case) (verification.Case, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockActiveOwner(ctx, tx, item.OwnerID); err != nil {
		return verification.Case{}, err
	}
	queries := r.queries.WithTx(tx)
	row, err := queries.CreateVerification(ctx, dbgen.CreateVerificationParams{
		ID: item.ID, OwnerID: item.OwnerID, Type: item.Type, EvidenceRef: item.EvidenceRef,
		IdempotencyKey: item.Idempotency, CreatedAt: dbTime(item.CreatedAt),
	})
	if err == nil {
		if _, err = tx.Exec(ctx, `INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_nuevo,correlacion_id,ocurrida_en)
			VALUES($1,$2,'solicitada','en_revision',$3,$4)`, item.ID, item.OwnerID, "verification:"+item.ID, dbTime(item.CreatedAt)); err != nil {
			return verification.Case{}, mapDBError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return verification.Case{}, mapDBError(err)
		}
		return fromCreate(row), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, mapDBError(err)
	}
	prior, err := queries.GetVerificationByIdempotency(ctx, dbgen.GetVerificationByIdempotencyParams{OwnerID: item.OwnerID, IdempotencyKey: item.Idempotency})
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
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

func (r *Repository) ListRejected(ctx context.Context) ([]verification.Case, error) {
	rows, err := r.queries.ListRejectedVerifications(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	items := make([]verification.Case, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromRejected(row))
	}
	return items, nil
}

func (r *Repository) Review(ctx context.Context, id, reviewer string, approved bool, reason string, clock func() time.Time) (verification.Case, error) {
	var owner string
	if err := r.pool.QueryRow(ctx, `SELECT usuario_id::text FROM public.verificacion WHERE id=$1`, id).Scan(&owner); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockActiveOwner(ctx, tx, owner); err != nil {
		return verification.Case{}, err
	}
	var currentState, kind string
	if err := tx.QueryRow(ctx, `SELECT estado,tipo FROM public.verificacion WHERE id=$1 FOR UPDATE`, id).Scan(&currentState, &kind); errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, verification.ErrNotFound
	} else if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if currentState != "en_revision" {
		return verification.Case{}, verification.ErrConflict
	}
	newState := "rechazada"
	if approved {
		newState = "aprobada"
	}
	resolvedAt := clock().UTC()
	_, err = tx.Exec(ctx, `UPDATE public.verificacion SET estado=$1,revisor_id=$2,motivo_codigo=NULLIF($3,''),resuelta_en=$4 WHERE id=$5 AND estado='en_revision'`, newState, reviewer, reason, dbTime(resolvedAt), id)
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	action := "revision_rechazada"
	if approved {
		action = "revision_aprobada"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_anterior,estado_nuevo,motivo_codigo,correlacion_id,ocurrida_en)
		VALUES($1,$2,$3,'en_revision',$4,NULLIF($5,''),$6,$7)`, id, reviewer, action, newState, reason, "verification-review:"+id, dbTime(resolvedAt)); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if approved {
		if _, err = tx.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en)
			VALUES($1,$2,$3,'elegible',$4) ON CONFLICT(usuario_id,tipo) DO UPDATE SET verificacion_id=EXCLUDED.verificacion_id,
			estado='elegible',concedida_en=EXCLUDED.concedida_en,revocada_en=NULL,revocada_por=NULL,motivo_revocacion_codigo=NULL`, owner, kind, id, dbTime(resolvedAt)); err != nil {
			return verification.Case{}, mapDBError(err)
		}
	}
	auditReason := reason
	if approved {
		auditReason = "aprobacion_fixture"
	}
	if err = recordVerificationAudit(ctx, tx, reviewer, id, "kyc.synthetic.review", auditReason, "verification-review:"+id, resolvedAt); err != nil {
		return verification.Case{}, err
	}
	item, err := scanCase(tx.QueryRow(ctx, caseSelect+` WHERE id=$1`, id))
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return item, nil
}

func (r *Repository) Retry(ctx context.Context, owner, priorID string, item verification.Case, clock func() time.Time) (verification.Case, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return verification.Case{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := r.queries.WithTx(tx)
	if err := lockActiveOwner(ctx, tx, owner); err != nil {
		return verification.Case{}, err
	}

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
	if !validRetryCorrection(prior.ReasonCode, item.CorrectionCode) {
		return verification.Case{}, verification.ErrInvalid
	}
	// Recheck after acquiring the parent lock so same-case requests that raced
	// on the initial lookup return the one row committed by the first request.
	existing, err := queries.GetVerificationByIdempotency(ctx, dbgen.GetVerificationByIdempotencyParams{OwnerID: owner, IdempotencyKey: item.Idempotency})
	if err == nil {
		if retryParent(existing.RetryOf) != priorID {
			return verification.Case{}, verification.ErrConflict
		}
		if existing.CorrectionCode != item.CorrectionCode {
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
	item.CreatedAt = clock().UTC()
	row, err := queries.CreateVerificationRetry(ctx, dbgen.CreateVerificationRetryParams{
		ID: item.ID, OwnerID: owner, Type: prior.Type, EvidenceRef: item.EvidenceRef,
		IdempotencyKey: item.Idempotency, RetryOf: priorID, CreatedAt: dbTime(item.CreatedAt), CorrectionCode: &item.CorrectionCode,
	})
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_nuevo,correlacion_id,ocurrida_en)
		VALUES($1,$2,'solicitada','en_revision',$3,$4)`, item.ID, owner, "verification:"+item.ID, dbTime(item.CreatedAt)); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_anterior,estado_nuevo,motivo_codigo,correlacion_id,clave_idempotencia,ocurrida_en)
		VALUES($1,$2,'subsanacion_solicitada','rechazada','rechazada',$3,$4,$5,$6)`, priorID, owner, prior.ReasonCode, "verification-retry:"+priorID, item.Idempotency, dbTime(item.CreatedAt)); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return fromRetry(row), nil
}

func validRetryCorrection(reason, correction string) bool {
	switch reason {
	case "documento_vencido":
		return correction == "fixture_vigente_actualizado"
	case "antecedentes_incompletos":
		return correction == "antecedentes_fixture_actualizados"
	case "inicio_actividades_no_confirmado":
		return correction == "inicio_actividades_fixture_actualizadas"
	default:
		return false
	}
}

func (r *Repository) ListHistory(ctx context.Context, owner, id string) ([]verification.HistoryEntry, error) {
	rows, err := r.queries.ListVerificationHistory(ctx, dbgen.ListVerificationHistoryParams{OwnerID: owner, ID: id})
	if err != nil {
		return nil, mapDBError(err)
	}
	items := make([]verification.HistoryEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, verification.HistoryEntry{Sequence: row.Sequence, Action: row.Action, FromState: row.FromState, ToState: row.ToState, ReasonCode: row.ReasonCode, OccurredAt: row.OccurredAt.Time})
	}
	if len(items) == 0 {
		if _, err := r.GetOwn(ctx, owner, id); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *Repository) ListEligibility(ctx context.Context, owner string) ([]verification.Eligibility, error) {
	rows, err := r.pool.Query(ctx, `SELECT kinds.tipo,
       COALESCE(e.estado='elegible',false), COALESCE(e.verificacion_id::text,''),
       e.concedida_en, e.revocada_en, COALESCE(e.motivo_revocacion_codigo,'')
FROM (SELECT 'kyc'::text AS tipo UNION ALL SELECT 'kyb'::text) kinds
LEFT JOIN public.elegibilidad_verificacion_local e ON e.usuario_id=$1 AND e.tipo=kinds.tipo
ORDER BY kinds.tipo`, owner)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]verification.Eligibility, 0, 2)
	for rows.Next() {
		var item verification.Eligibility
		var granted, revoked pgtype.Timestamptz
		if err := rows.Scan(&item.Type, &item.Eligible, &item.VerificationID, &granted, &revoked, &item.RevocationReason); err != nil {
			return nil, mapDBError(err)
		}
		if granted.Valid {
			item.GrantedAt = granted.Time
		}
		if revoked.Valid {
			at := revoked.Time
			item.RevokedAt = &at
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return items, nil
}

func (r *Repository) Revoke(ctx context.Context, id, reviewer, reason, key, correlation string, clock func() time.Time) (verification.Case, error) {
	var owner string
	if err := r.pool.QueryRow(ctx, `SELECT usuario_id::text FROM public.verificacion WHERE id=$1`, id).Scan(&owner); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockActiveOwner(ctx, tx, owner); err != nil {
		return verification.Case{}, err
	}
	var kind, state string
	if err = tx.QueryRow(ctx, `SELECT tipo,estado FROM public.verificacion WHERE id=$1 FOR UPDATE`, id).Scan(&kind, &state); errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, verification.ErrNotFound
	} else if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	var priorKey string
	err = tx.QueryRow(ctx, `SELECT COALESCE(clave_idempotencia,'') FROM public.verificacion_historial_local WHERE verificacion_id=$1 AND accion='elegibilidad_revocada' ORDER BY id DESC LIMIT 1`, id).Scan(&priorKey)
	if err == nil {
		if priorKey != key {
			return verification.Case{}, verification.ErrConflict
		}
		item, e := scanCase(tx.QueryRow(ctx, caseSelect+` WHERE id=$1`, id))
		if e != nil {
			return verification.Case{}, mapDBError(e)
		}
		if e = tx.Commit(ctx); e != nil {
			return verification.Case{}, mapDBError(e)
		}
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, mapDBError(err)
	}
	if state != "aprobada" {
		return verification.Case{}, verification.ErrConflict
	}
	var eligibilityState string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.elegibilidad_verificacion_local WHERE usuario_id=$1 AND tipo=$2 AND verificacion_id=$3 FOR UPDATE`, owner, kind, id).Scan(&eligibilityState); errors.Is(err, pgx.ErrNoRows) {
		return verification.Case{}, verification.ErrConflict
	} else if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if eligibilityState != "elegible" {
		return verification.Case{}, verification.ErrConflict
	}
	at := clock().UTC()
	if _, err = tx.Exec(ctx, `UPDATE public.verificacion SET estado='revocada',revocada_en=$2,revocada_por=$3,motivo_revocacion_codigo=$4 WHERE id=$1`, id, dbTime(at), reviewer, reason); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.elegibilidad_verificacion_local SET estado='revocada',revocada_en=$4,revocada_por=$3,motivo_revocacion_codigo=$5 WHERE usuario_id=$1 AND tipo=$2 AND verificacion_id=$6`, owner, kind, reviewer, dbTime(at), reason, id); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_anterior,estado_nuevo,motivo_codigo,correlacion_id,clave_idempotencia,ocurrida_en)
		VALUES($1,$2,'elegibilidad_revocada','aprobada','revocada',$3,$4,$5,$6)`, id, reviewer, reason, correlation, key, dbTime(at)); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if err = recordVerificationAudit(ctx, tx, reviewer, id, "kyc.eligibility.revoke", reason, correlation, at); err != nil {
		return verification.Case{}, err
	}
	item, err := scanCase(tx.QueryRow(ctx, caseSelect+` WHERE id=$1`, id))
	if err != nil {
		return verification.Case{}, mapDBError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return verification.Case{}, mapDBError(err)
	}
	return item, nil
}

const caseSelect = `SELECT id::text,usuario_id::text,tipo,estado,proveedor_ref,referencia_evidencia,COALESCE(reintento_de::text,''),COALESCE(motivo_codigo,''),creada_en,resuelta_en,clave_idempotencia,COALESCE(correccion_codigo,''),revocada_en,COALESCE(motivo_revocacion_codigo,'') FROM public.verificacion`

type rowScanner interface{ Scan(...any) error }

func scanCase(row rowScanner) (verification.Case, error) {
	var c verification.Case
	var retry string
	var created, resolved, revoked pgtype.Timestamptz
	var revokeReason string
	err := row.Scan(&c.ID, &c.OwnerID, &c.Type, &c.State, &c.Provider, &c.EvidenceRef, &retry, &c.ReasonCode, &created, &resolved, &c.Idempotency, &c.CorrectionCode, &revoked, &revokeReason)
	if err != nil {
		return verification.Case{}, err
	}
	c.RetryOf = retry
	c.CreatedAt = created.Time
	c.ResolvedAt = nullableTime(resolved)
	c.RevokedAt = nullableTime(revoked)
	c.RevocationReason = revokeReason
	return c, nil
}

func recordVerificationAudit(ctx context.Context, tx pgx.Tx, actor, id, action, reason, correlation string, at time.Time) error {
	auditID, err := (credentials.Generator{}).ID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en,detalle_codigos)
		VALUES($1,$2,'verificacion',$3,$4,'exito',$5,$6,$7,$8,'{"obligations_detected":[],"pending_checks":[]}'::jsonb)`, auditID, actor, id, action, reason, correlation, dbTime(at), dbTime(at.AddDate(5, 0, 0)))
	return mapDBError(err)
}

func dbTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromIdempotency(row dbgen.GetVerificationByIdempotencyRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromOwn(row dbgen.GetOwnVerificationRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromOwnList(row dbgen.ListOwnVerificationsRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromPending(row dbgen.ListPendingVerificationsRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromRejected(row dbgen.ListRejectedVerificationsRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromReview(row dbgen.ReviewVerificationRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: row.CorrectionCode, RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
}
func fromRetry(row dbgen.CreateVerificationRetryRow) verification.Case {
	return verification.Case{ID: row.ID, OwnerID: row.OwnerID, Type: row.Type, State: row.State, Provider: row.Provider, EvidenceRef: row.EvidenceRef, RetryOf: retryParent(row.RetryOf), ReasonCode: row.ReasonCode, CreatedAt: row.CreatedAt.Time, ResolvedAt: nullableTime(row.ResolvedAt), Idempotency: row.IdempotencyKey, CorrectionCode: nullableString(row.CorrectionCode), RevokedAt: nullableTime(row.RevokedAt), RevocationReason: row.RevocationReason}
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
