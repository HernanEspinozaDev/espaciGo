package operationspg

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) ReserveCandidate(ctx context.Context, actor, reservation, kind, fileID string, at time.Time) error {
	var allowed bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local
		WHERE id=$1 AND $2 IN (anfitrion_id::text,arrendatario_id::text))`, reservation, actor).Scan(&allowed); err != nil {
		return mapError(err)
	}
	if !allowed {
		return operation.ErrNotFound
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO public.operacion_arriendo_archivo_candidato_local
		(archivo_id,reserva_id,actor_id,tipo,estado,creada_en,proximo_intento_en)
		VALUES($1,$2,$3,$4,'reservado',$5,$5)`, fileID, reservation, actor, kind, at.UTC())
	return mapError(err)
}

func (r *Repository) QueueCandidateCleanup(ctx context.Context, fileID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.operacion_arriendo_archivo_candidato_local
		SET estado='pendiente_limpieza',proximo_intento_en=$2,lease_iniciado_en=NULL,
		ultimo_codigo_error=CASE WHEN estado='reservado' THEN 'alta_no_confirmada' ELSE ultimo_codigo_error END
		WHERE archivo_id=$1 AND estado IN ('reservado','pendiente_limpieza')`, fileID, at.UTC())
	return mapError(err)
}

func (r *Repository) BeginCandidate(ctx context.Context, actor, reservation, kind, fileID string) (operation.CandidateWriter, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	failed := true
	defer func() {
		if failed {
			_ = tx.Rollback(context.Background())
		}
	}()
	var host, renter string
	err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text
		FROM public.reserva_ensayo_local WHERE id=$1`, reservation).Scan(&host, &renter)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && actor != host && actor != renter) {
		return nil, operation.ErrNotFound
	}
	if err != nil {
		return nil, mapError(err)
	}
	ids := []string{host, renter}
	sort.Strings(ids)
	for i, id := range ids {
		if i > 0 && ids[i-1] == id {
			continue
		}
		var state string
		if err := tx.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
			return nil, mapError(err)
		}
		if state != "activo" {
			return nil, operation.ErrConflict
		}
	}
	var lockedHost, lockedRenter, state, zone string
	var start time.Time
	err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text,estado,inicio,zona_horaria
		FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&lockedHost, &lockedRenter, &state, &start, &zone)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (lockedHost != host || lockedRenter != renter) {
		return nil, operation.ErrNotFound
	}
	if err != nil {
		return nil, mapError(err)
	}
	var candidateState string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.operacion_arriendo_archivo_candidato_local
		WHERE archivo_id=$1 AND reserva_id=$2 AND actor_id=$3 AND tipo=$4 FOR UPDATE`, fileID, reservation, actor, kind).Scan(&candidateState)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && candidateState != "reservado" {
		return nil, operation.ErrConflict
	}
	if err != nil {
		return nil, mapError(err)
	}
	failed = false
	return &candidateWriter{tx: tx, actor: actor, reservation: reservation, kind: kind, state: state, start: start, zone: zone, host: host, renter: renter, fileID: fileID}, nil
}

type candidateWriter struct {
	tx                                                          pgx.Tx
	actor, reservation, kind, state, zone, host, renter, fileID string
	start                                                       time.Time
	done                                                        bool
}

func (w *candidateWriter) Apply(ctx context.Context, opID, key string, input operation.Input, evidence operation.Evidence, fingerprint []byte, clock func() time.Time) (operation.Item, bool, error) {
	if w.done || opID != w.fileID || len(fingerprint) != 32 {
		return operation.Item{}, false, operation.ErrConflict
	}
	prior, priorKey, priorFingerprint, scanErr := scanExistingItem(w.tx.QueryRow(ctx, `SELECT id::text,reserva_id::text,tipo,actor_id::text,ocurrio_en,zona_horaria,ubicacion_sintetica,
		comentarios,observacion,resultado,clave_idempotencia,huella_solicitud
		FROM public.operacion_arriendo_ensayo_local WHERE reserva_id=$1 AND tipo=$2`, w.reservation, w.kind))
	if scanErr == nil {
		cleanup := w.queueCleanupTx(ctx, clock().UTC(), "alta_idempotente_reutilizada")
		if cleanup != nil {
			return operation.Item{}, false, cleanup
		}
		if prior.ActorID != w.actor || priorKey != key || subtle.ConstantTimeCompare(priorFingerprint, fingerprint) != 1 {
			if err := w.finish(ctx); err != nil {
				return operation.Item{}, false, mapError(err)
			}
			return operation.Item{}, false, operation.ErrConflict
		}
		if err := attachEvidence(ctx, w.tx, &prior); err != nil {
			return operation.Item{}, false, err
		}
		if err := w.tx.QueryRow(ctx, `SELECT COALESCE(max(secuencia),0) FROM public.operacion_arriendo_historial_ensayo_local WHERE operacion_id=$1`, prior.ID).Scan(&prior.Sequence); err != nil {
			return operation.Item{}, false, mapError(err)
		}
		prior.Reused = true
		if err := w.finish(ctx); err != nil {
			return operation.Item{}, false, mapError(err)
		}
		return prior, true, nil
	}
	if !errors.Is(scanErr, pgx.ErrNoRows) {
		return operation.Item{}, false, scanErr
	}
	// The actor is authorized before any action-specific detail is evaluated.
	if (w.kind == operation.CheckIn || w.kind == operation.CheckOut) && w.actor != w.renter || w.kind == operation.Receipt && w.actor != w.host {
		return operation.Item{}, false, w.reject(ctx, operation.ErrNotFound)
	}
	// Read the clock only after account and reservation locks have been acquired
	// and after the private image has been generated and stored.
	now := clock().UTC().Truncate(time.Microsecond)
	var from, to, action, result string
	switch w.kind {
	case operation.CheckIn:
		if w.state != "lista_para_checkin" || !sameLocalDate(now, w.start, w.zone) {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		var needsGuarantee, guaranteeReady bool
		if err := w.tx.QueryRow(ctx, `SELECT r.garantia_politica_version IS NOT NULL,COALESCE((SELECT g.estado='autorizada' FROM public.reserva_garantia_ensayo_local g WHERE g.reserva_id=r.id),false) FROM public.reserva_ensayo_local r WHERE r.id=$1`, w.reservation).Scan(&needsGuarantee, &guaranteeReady); err != nil {
			return operation.Item{}, false, w.reject(ctx, mapError(err))
		}
		if needsGuarantee && !guaranteeReady {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		var signed bool
		if err := w.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.contrato_ensayo_local c
			WHERE c.reserva_id=$1 AND c.estado='firmado'
			AND (SELECT count(*) FROM public.contrato_ensayo_firma f WHERE f.contrato_id=c.id AND f.estado='firmada')=2)`, w.reservation).Scan(&signed); err != nil {
			return operation.Item{}, false, w.reject(ctx, mapError(err))
		}
		if !signed {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		from, to, action, result = "lista_para_checkin", "en_curso", "checkin_registrado", "registrada"
	case operation.CheckOut:
		if w.state != "en_curso" {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		from, to, action, result = "en_curso", "finalizada", "checkout_registrado", "registrada"
	case operation.Receipt:
		if w.state != "finalizada" {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		var checkedOut bool
		if err := w.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local WHERE reserva_id=$1 AND tipo='checkout')`, w.reservation).Scan(&checkedOut); err != nil {
			return operation.Item{}, false, w.reject(ctx, mapError(err))
		}
		if !checkedOut {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		if strings.TrimSpace(input.Observations) == "" {
			result, action = "recepcion_conforme", "recepcion_confirmada"
		} else {
			result, action = "recepcion_con_observaciones", "observacion_registrada"
		}
	default:
		return operation.Item{}, false, w.reject(ctx, operation.ErrInvalid)
	}
	location := map[string]any{"source": "synthetic-fixture-v1", "location_code": "santiago-demo-center-v1", "latitude": -33.4560, "longitude": -70.6693}
	locationJSON, err := json.Marshal(location)
	if err != nil {
		return operation.Item{}, false, w.reject(ctx, operation.ErrInvalid)
	}
	var id string
	err = w.tx.QueryRow(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local
		(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,comentarios,observacion,resultado,clave_idempotencia,huella_solicitud,creada_en)
		VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$4) RETURNING id::text`,
		w.reservation, w.kind, w.actor, now, w.zone, locationJSON, input.Comments, input.Observations, result, key, fingerprint).Scan(&id)
	if err != nil {
		return operation.Item{}, false, w.reject(ctx, mapError(err))
	}
	evidence.ID = w.fileID
	evidence.CreatedAt = now
	_, err = w.tx.Exec(ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local
		(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, evidence.ID, id, evidence.Fixture, evidence.MIME, evidence.SHA256, evidence.SizeBytes, now)
	if err != nil {
		return operation.Item{}, false, w.reject(ctx, mapError(err))
	}
	if from != "" {
		res, updateErr := w.tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1 AND estado=$4`, w.reservation, to, now, from)
		if updateErr != nil {
			return operation.Item{}, false, w.reject(ctx, mapError(updateErr))
		}
		if res.RowsAffected() != 1 {
			return operation.Item{}, false, w.reject(ctx, operation.ErrConflict)
		}
		if _, err = w.tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en)
			SELECT gen_random_uuid(),$1,COALESCE(MAX(secuencia),0)+1,$2,$3,$4,$5,$6 FROM public.reserva_ensayo_transicion WHERE reserva_id=$1`, w.reservation, from, to, w.actor, w.kind, now); err != nil {
			return operation.Item{}, false, w.reject(ctx, mapError(err))
		}
	}
	if _, err = w.tx.Exec(ctx, `INSERT INTO public.operacion_arriendo_historial_ensayo_local(reserva_id,operacion_id,accion,actor_id,ocurrida_en)
		VALUES($1,$2,$3,$4,$5)`, w.reservation, id, action, w.actor, now); err != nil {
		return operation.Item{}, false, w.reject(ctx, mapError(err))
	}
	if _, err = w.tx.Exec(ctx, `DELETE FROM public.operacion_arriendo_archivo_candidato_local WHERE archivo_id=$1 AND estado='reservado'`, w.fileID); err != nil {
		return operation.Item{}, false, w.reject(ctx, mapError(err))
	}
	item, err := scanItemWithSequence(w.tx.QueryRow(ctx, `SELECT o.id::text,o.reserva_id::text,o.tipo,o.actor_id::text,o.ocurrio_en,o.zona_horaria,o.ubicacion_sintetica,
		o.comentarios,o.observacion,o.resultado,h.secuencia
		FROM public.operacion_arriendo_ensayo_local o JOIN public.operacion_arriendo_historial_ensayo_local h ON h.operacion_id=o.id WHERE o.id=$1`, id))
	if err != nil {
		return operation.Item{}, false, mapError(err)
	}
	if err := attachEvidence(ctx, w.tx, &item); err != nil {
		return operation.Item{}, false, err
	}
	if item.Kind == operation.CheckOut {
		deadline := item.OccurredAt.Add(24 * time.Hour)
		item.ClaimDeadlineAt = &deadline
	}
	if err = w.finish(ctx); err != nil {
		return operation.Item{}, false, mapError(err)
	}
	return item, false, nil
}

func (w *candidateWriter) reject(_ context.Context, cause error) error {
	if !w.done {
		w.done = true
		_ = w.tx.Rollback(context.Background())
	}
	return cause
}

func (w *candidateWriter) queueCleanupTx(ctx context.Context, at time.Time, code string) error {
	_, err := w.tx.Exec(ctx, `UPDATE public.operacion_arriendo_archivo_candidato_local
		SET estado='pendiente_limpieza',proximo_intento_en=$2,lease_iniciado_en=NULL,ultimo_codigo_error=$3
		WHERE archivo_id=$1 AND estado='reservado'`, w.fileID, at.UTC(), code)
	return mapError(err)
}

func (w *candidateWriter) finish(ctx context.Context) error {
	if w.done {
		return nil
	}
	w.done = true
	return w.tx.Commit(ctx)
}

func (w *candidateWriter) QueueCleanup(ctx context.Context, at time.Time) error {
	if w.done {
		return nil
	}
	if err := w.queueCleanupTx(ctx, at, "alta_no_confirmada"); err != nil {
		return err
	}
	return w.finish(ctx)
}

func (w *candidateWriter) Close() error {
	if w.done {
		return nil
	}
	w.done = true
	return w.tx.Rollback(context.Background())
}

const itemColumns = `o.id::text,o.reserva_id::text,o.tipo,o.actor_id::text,o.ocurrio_en,o.zona_horaria,o.ubicacion_sintetica,o.comentarios,o.observacion,o.resultado`

func scanItem(row pgx.Row) (operation.Item, error) {
	var item operation.Item
	var location []byte
	var comments, observations string
	err := row.Scan(&item.ID, &item.ReservationID, &item.Kind, &item.ActorID, &item.OccurredAt, &item.TimeZone, &location, &comments, &observations, &item.Result)
	if err != nil {
		return item, mapError(err)
	}
	item.Comments, item.Observations = comments, observations
	item.OccurredAt = item.OccurredAt.UTC()
	if err := json.Unmarshal(location, &item.SyntheticLocation); err != nil {
		return operation.Item{}, operation.ErrInvalid
	}
	return item, nil
}

func scanExistingItem(row pgx.Row) (operation.Item, string, []byte, error) {
	var item operation.Item
	var location []byte
	var comments, observations, key string
	var fingerprint []byte
	err := row.Scan(&item.ID, &item.ReservationID, &item.Kind, &item.ActorID, &item.OccurredAt, &item.TimeZone, &location,
		&comments, &observations, &item.Result, &key, &fingerprint)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return item, "", nil, pgx.ErrNoRows
		}
		return item, "", nil, mapError(err)
	}
	item.Comments, item.Observations = comments, observations
	item.OccurredAt = item.OccurredAt.UTC()
	if err := json.Unmarshal(location, &item.SyntheticLocation); err != nil {
		return operation.Item{}, "", nil, operation.ErrInvalid
	}
	return item, key, fingerprint, nil
}

func scanItemWithSequence(row pgx.Row) (operation.Item, error) {
	var item operation.Item
	var location []byte
	var comments, observations string
	err := row.Scan(&item.ID, &item.ReservationID, &item.Kind, &item.ActorID, &item.OccurredAt, &item.TimeZone, &location,
		&comments, &observations, &item.Result, &item.Sequence)
	if err != nil {
		return item, mapError(err)
	}
	item.Comments, item.Observations = comments, observations
	item.OccurredAt = item.OccurredAt.UTC()
	if err := json.Unmarshal(location, &item.SyntheticLocation); err != nil {
		return operation.Item{}, operation.ErrInvalid
	}
	return item, nil
}

func attachEvidence(ctx context.Context, tx pgx.Tx, item *operation.Item) error {
	rows, err := tx.Query(ctx, `SELECT id::text,fixture_code,mime_type,sha256,size_bytes,creada_en
		FROM public.operacion_arriendo_evidencia_ensayo_local WHERE operacion_id=$1 ORDER BY id`, item.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	item.Evidence = make([]operation.Evidence, 0, 1)
	for rows.Next() {
		var evidence operation.Evidence
		if err := rows.Scan(&evidence.ID, &evidence.Fixture, &evidence.MIME, &evidence.SHA256, &evidence.SizeBytes, &evidence.CreatedAt); err != nil {
			return mapError(err)
		}
		evidence.CreatedAt = evidence.CreatedAt.UTC()
		item.Evidence = append(item.Evidence, evidence)
	}
	return mapError(rows.Err())
}

func attachEvidencePool(ctx context.Context, pool *pgxpool.Pool, item *operation.Item) error {
	rows, err := pool.Query(ctx, `SELECT id::text,fixture_code,mime_type,sha256,size_bytes,creada_en
		FROM public.operacion_arriendo_evidencia_ensayo_local WHERE operacion_id=$1 ORDER BY id`, item.ID)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	item.Evidence = make([]operation.Evidence, 0, 1)
	for rows.Next() {
		var evidence operation.Evidence
		if err := rows.Scan(&evidence.ID, &evidence.Fixture, &evidence.MIME, &evidence.SHA256, &evidence.SizeBytes, &evidence.CreatedAt); err != nil {
			return mapError(err)
		}
		evidence.CreatedAt = evidence.CreatedAt.UTC()
		item.Evidence = append(item.Evidence, evidence)
	}
	return mapError(rows.Err())
}

func sameLocalDate(now, start time.Time, zoneName string) bool {
	loc, err := time.LoadLocation(zoneName)
	if err != nil {
		return false
	}
	return now.In(loc).Format("2006-01-02") == start.In(loc).Format("2006-01-02")
}

func (r *Repository) List(ctx context.Context, actor, reservation string) ([]operation.Item, error) {
	var participant bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2))`, reservation, actor).Scan(&participant); err != nil {
		return nil, mapError(err)
	}
	if !participant {
		return nil, operation.ErrNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT `+itemColumns+`,h.secuencia FROM public.operacion_arriendo_ensayo_local o
		JOIN public.operacion_arriendo_historial_ensayo_local h ON h.operacion_id=o.id
		WHERE o.reserva_id=$1 ORDER BY h.secuencia`, reservation)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]operation.Item, 0)
	for rows.Next() {
		item, err := scanItemWithSequence(rows)
		if err != nil {
			return nil, err
		}
		if err = attachEvidencePool(ctx, r.pool, &item); err != nil {
			return nil, err
		}
		if item.Kind == operation.CheckOut {
			deadline := item.OccurredAt.Add(24 * time.Hour)
			item.ClaimDeadlineAt = &deadline
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) Evidence(ctx context.Context, actor, reservation, evidenceID string) (operation.Evidence, error) {
	var item operation.Evidence
	err := r.pool.QueryRow(ctx, `SELECT e.id::text,e.fixture_code,e.mime_type,e.sha256,e.size_bytes,e.creada_en
		FROM public.operacion_arriendo_evidencia_ensayo_local e JOIN public.operacion_arriendo_ensayo_local o ON o.id=e.operacion_id
		JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id
		WHERE e.id=$1 AND r.id=$2 AND $3 IN (r.anfitrion_id::text,r.arrendatario_id::text)`, evidenceID, reservation, actor).
		Scan(&item.ID, &item.Fixture, &item.MIME, &item.SHA256, &item.SizeBytes, &item.CreatedAt)
	if err != nil {
		return operation.Evidence{}, mapError(err)
	}
	item.CreatedAt = item.CreatedAt.UTC()
	return item, nil
}

func (r *Repository) ClaimCandidateCleanup(ctx context.Context, now time.Time, limit int) ([]string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	cutoff := now.UTC().Add(-operation.CandidateRecoveryDelay)
	rows, err := tx.Query(ctx, `SELECT archivo_id::text FROM public.operacion_arriendo_archivo_candidato_local
		WHERE (estado='pendiente_limpieza' AND proximo_intento_en<=$1)
		 OR (estado='reservado' AND creada_en<=$2)
		 OR (estado='limpiando' AND lease_iniciado_en<=$2)
		ORDER BY proximo_intento_en,archivo_id FOR UPDATE SKIP LOCKED LIMIT $3`, now.UTC(), cutoff, limit)
	if err != nil {
		return nil, mapError(err)
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, mapError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, mapError(err)
	}
	rows.Close()
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE public.operacion_arriendo_archivo_candidato_local SET estado='limpiando',lease_iniciado_en=$2,intentos_limpieza=intentos_limpieza+1 WHERE archivo_id=$1`, id, now.UTC()); err != nil {
			return nil, mapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return ids, nil
}

func (r *Repository) CompleteCandidateCleanup(ctx context.Context, fileID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.operacion_arriendo_archivo_candidato_local WHERE archivo_id=$1 AND estado='limpiando'`, fileID)
	return mapError(err)
}

func (r *Repository) FailCandidateCleanup(ctx context.Context, fileID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.operacion_arriendo_archivo_candidato_local SET estado='pendiente_limpieza',lease_iniciado_en=NULL,
		proximo_intento_en=$2+LEAST(interval '10 minutes',interval '15 seconds'*power(2,LEAST(intentos_limpieza,5))),ultimo_codigo_error='private_file_delete_failed'
		WHERE archivo_id=$1 AND estado='limpiando'`, fileID, at.UTC())
	return mapError(err)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return operation.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23P01", "40001", "40P01":
			return operation.ErrConflict
		case "23503":
			return operation.ErrNotFound
		case "23514", "22P02", "22001":
			return operation.ErrInvalid
		}
	}
	return err
}
