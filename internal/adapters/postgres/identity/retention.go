package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const terminalReservationStates = `('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')`

type reservationRetentionCandidate struct {
	id, host, renter string
	due              time.Time
}

func (r *IdentityRepository) PurgeExpiredReservationLinks(ctx context.Context, now time.Time, limit int) (privacy.RetentionPurgeResult, error) {
	result := privacy.RetentionPurgeResult{}
	if limit < 1 || limit > 500 {
		return result, privacy.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, `SELECT r.id::text,COALESCE(r.anfitrion_id::text,''),COALESCE(r.arrendatario_id::text,''),r.vinculos_retirar_en
		FROM public.reserva_ensayo_local r
		WHERE r.estado IN `+terminalReservationStates+`
		  AND NOT EXISTS (SELECT 1 FROM public.reserva_vinculo_purgado_local x WHERE x.reserva_id=r.id)
		  AND r.vinculos_retirar_en <= $1
		ORDER BY r.vinculos_retirar_en,r.id LIMIT $2`, now.UTC(), limit)
	if err != nil {
		return result, mapError(err)
	}
	var candidates []reservationRetentionCandidate
	for rows.Next() {
		var item reservationRetentionCandidate
		if err := rows.Scan(&item.id, &item.host, &item.renter, &item.due); err != nil {
			rows.Close()
			return result, mapError(err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, mapError(err)
	}
	rows.Close()
	for _, candidate := range candidates {
		result.Scanned++
		purged, deferred, err := r.purgeReservationLinks(ctx, candidate, now.UTC())
		if err != nil {
			return result, err
		}
		if purged {
			result.Purged++
		} else if deferred {
			result.Deferred++
		}
	}
	return result, nil
}

func (r *IdentityRepository) purgeReservationLinks(ctx context.Context, candidate reservationRetentionCandidate, now time.Time) (bool, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, false, mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	ids := []string{}
	for _, id := range []string{candidate.host, candidate.renter} {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `SELECT id FROM public.usuario WHERE id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, ids); err != nil {
			return false, false, mapError(err)
		}
	}
	var host, renter pgtype.Text
	var state string
	var storedDeadline pgtype.Timestamptz
	if err := tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text,estado,vinculos_retirar_en
		FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, candidate.id).Scan(&host, &renter, &state, &storedDeadline); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, mapError(err)
	}
	if state != "cancelada_por_pago" && state != "rechazada_arrendador" && state != "vencida_pago" && state != "vencida_host" && state != "cancelada_arrendatario" {
		return false, false, tx.Commit(ctx)
	}
	currentHost, currentRenter := "", ""
	if host.Valid {
		currentHost = host.String
	}
	if renter.Valid {
		currentRenter = renter.String
	}
	if currentHost != candidate.host || currentRenter != candidate.renter {
		return false, false, tx.Commit(ctx)
	}
	var deadline time.Time
	if err := tx.QueryRow(ctx, `SELECT GREATEST(r.actualizada_en,
		COALESCE((SELECT max(p.actualizada_en) FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=r.id),r.actualizada_en),
		COALESCE((SELECT max(d.actualizada_en) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),r.actualizada_en),
		COALESCE((SELECT max(x.cerrada_en) FROM public.disputa_ensayo_local x WHERE x.reserva_id=r.id),r.actualizada_en)
		)+interval '24 months' FROM public.reserva_ensayo_local r WHERE r.id=$1`, candidate.id).Scan(&deadline); err != nil {
		return false, false, mapError(err)
	}
	if storedDeadline.Valid && storedDeadline.Time.After(deadline) {
		deadline = storedDeadline.Time
	}
	if deadline.After(now) {
		if _, err := tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2 WHERE id=$1`, candidate.id, deadline); err != nil {
			return false, false, mapError(err)
		}
		return false, true, tx.Commit(ctx)
	}
	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=$1 AND p.estado='pendiente')
		OR EXISTS (SELECT 1 FROM public.reserva_pago_evento_aplicacion_ensayo a JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id WHERE e.operacion_id IN (SELECT id FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1) AND a.estado='pendiente_conciliacion')
		OR EXISTS (SELECT 1 FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=$1 AND d.estado='pendiente')
		OR EXISTS (SELECT 1 FROM public.disputa_ensayo_local d WHERE d.reserva_id=$1 AND d.estado='abierta')`, candidate.id).Scan(&blocked); err != nil {
		return false, false, mapError(err)
	}
	if blocked {
		return false, true, tx.Commit(ctx)
	}
	fields := []string{"reserva.anfitrion_id", "reserva.arrendatario_id", "cotizacion.anfitrion_id", "cotizacion.arrendatario_id", "pago.arrendatario_id", "cancelacion.arrendatario_id", "disputa.participantes_y_actores", "historial_transicion.actor_id", "historial_disputa.actor_id", "mensaje.autor_id", "fixture.participantes", "cursor_lectura"}
	statements := []struct {
		sql  string
		args []any
	}{
		{`UPDATE public.cotizacion_reserva_ensayo q SET anfitrion_id=NULL,arrendatario_id=NULL WHERE q.id=(SELECT cotizacion_id FROM public.reserva_ensayo_local WHERE id=$1)`, []any{candidate.id}},
		{`UPDATE public.reserva_pago_ensayo_operacion SET arrendatario_id=NULL WHERE reserva_id=$1`, []any{candidate.id}},
		{`UPDATE public.reserva_cancelacion_ensayo SET arrendatario_id=NULL WHERE reserva_id=$1`, []any{candidate.id}},
		{`UPDATE public.disputa_ensayo_local SET anfitrion_id=NULL,arrendatario_id=NULL,abierta_por=NULL,cerrada_por=CASE WHEN cerrada_por IN ($2::uuid,$3::uuid) THEN NULL ELSE cerrada_por END WHERE reserva_id=$1`, []any{candidate.id, nullableUUID(currentHost), nullableUUID(currentRenter)}},
		{`UPDATE public.disputa_ensayo_historial SET actor_id=NULL WHERE actor_id IN ($2::uuid,$3::uuid) AND disputa_id IN (SELECT id FROM public.disputa_ensayo_local WHERE reserva_id=$1)`, []any{candidate.id, nullableUUID(currentHost), nullableUUID(currentRenter)}},
		{`UPDATE public.reserva_ensayo_transicion SET actor_id=NULL WHERE actor_id IN ($2::uuid,$3::uuid) AND reserva_id=$1`, []any{candidate.id, nullableUUID(currentHost), nullableUUID(currentRenter)}},
		{`UPDATE public.mensaje_reserva_ensayo SET autor_id=NULL WHERE autor_id IN ($2::uuid,$3::uuid) AND reserva_id=$1`, []any{candidate.id, nullableUUID(currentHost), nullableUUID(currentRenter)}},
		{`DELETE FROM public.reserva_mensaje_lectura WHERE reserva_id=$1`, []any{candidate.id}},
		{`UPDATE public.reserva_ensayo_local_fixture f SET anfitrion_id=CASE WHEN f.anfitrion_id IN ($2::uuid,$3::uuid) THEN NULL ELSE f.anfitrion_id END,
			arrendatario_id=CASE WHEN f.arrendatario_id IN ($2::uuid,$3::uuid) THEN NULL ELSE f.arrendatario_id END
			WHERE f.espacio_id=(SELECT espacio_id FROM public.reserva_ensayo_local WHERE id=$1)
			AND NOT EXISTS (SELECT 1 FROM public.reserva_ensayo_local other WHERE other.espacio_id=f.espacio_id AND other.id<>$1 AND NOT EXISTS (SELECT 1 FROM public.reserva_vinculo_purgado_local p WHERE p.reserva_id=other.id))`, []any{candidate.id, nullableUUID(currentHost), nullableUUID(currentRenter)}},
		{`UPDATE public.reserva_ensayo_local SET anfitrion_id=NULL,arrendatario_id=NULL WHERE id=$1`, []any{candidate.id}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.sql, statement.args...); err != nil {
			return false, false, mapError(err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.reserva_vinculo_purgado_local(reserva_id,vencio_en,purgado_en,campos_retirados) VALUES($1,$2,$3,$4) ON CONFLICT (reserva_id) DO NOTHING`, candidate.id, deadline, now, fields); err != nil {
		return false, false, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, false, mapError(err)
	}
	return true, false, nil
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *IdentityRepository) ExportCompletedSuppressions(ctx context.Context) (privacy.SuppressionReplayManifest, error) {
	rows, err := r.pool.Query(ctx, `SELECT x.id::text,x.solicitud_id::text,x.usuario_id::text,s.solicitada_en,x.completada_en
		FROM public.ejecucion_baja_local x JOIN public.solicitud_titular s ON s.id=x.solicitud_id
		WHERE x.estado='completada' AND x.completada_en IS NOT NULL ORDER BY x.completada_en,x.id`)
	if err != nil {
		return privacy.SuppressionReplayManifest{}, mapError(err)
	}
	defer rows.Close()
	manifest := privacy.SuppressionReplayManifest{Version: 1, Entries: []privacy.SuppressionReplayEntry{}}
	for rows.Next() {
		var item privacy.SuppressionReplayEntry
		if err := rows.Scan(&item.ExecutionID, &item.RequestID, &item.AccountID, &item.RequestedAt, &item.CompletedAt); err != nil {
			return privacy.SuppressionReplayManifest{}, mapError(err)
		}
		manifest.Entries = append(manifest.Entries, item)
	}
	return manifest, rows.Err()
}

func replayKey(restoreID, executionID string, attempt int) string {
	return fmt.Sprintf("restore:%s:%s:%d", restoreID, executionID, attempt)
}

func (r *IdentityRepository) PrepareSuppressionReplay(ctx context.Context, entry privacy.SuppressionReplayEntry, restoreID, actorID string, now time.Time) (string, string, string, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", "", "", false, mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var actorRole string
	if err := tx.QueryRow(ctx, `SELECT ru.rol FROM public.usuario u JOIN public.rol_usuario ru ON ru.usuario_id=u.id WHERE u.id=$1 AND u.estado='activo' AND ru.rol='administrador' FOR UPDATE OF u`, actorID).Scan(&actorRole); err != nil {
		return "", "", "", false, mapError(err)
	}
	var state string
	var attempts int
	err = tx.QueryRow(ctx, `SELECT estado,intentos FROM public.reaplicacion_baja_local WHERE restore_id=$1 AND ejecucion_origen_id=$2 FOR UPDATE`, restoreID, entry.ExecutionID).Scan(&state, &attempts)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", false, mapError(err)
	}
	replayExists := err == nil
	if err == nil && (state == "reaplicada" || state == "ya_presente") {
		return entry.RequestID, "", state, true, tx.Commit(ctx)
	}
	var lockedAccount string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE id=$1 FOR UPDATE`, entry.AccountID).Scan(&lockedAccount); err != nil {
		return "", "", "", false, mapError(err)
	}
	var priorRequestAccount string
	err = tx.QueryRow(ctx, `SELECT usuario_id::text FROM public.solicitud_titular WHERE id=$1 AND tipo='supresion'`, entry.RequestID).Scan(&priorRequestAccount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", false, mapError(err)
	}
	requestExists := err == nil
	if err == nil && priorRequestAccount != entry.AccountID {
		return "", "", "", false, privacy.ErrInvalid
	}
	if !requestExists {
		if _, err = tx.Exec(ctx, `INSERT INTO public.solicitud_titular(id,usuario_id,tipo,canal,estado,solicitada_en,resuelta_en,motivo_resolucion_codigo,retirar_en) VALUES($1,$2,'supresion','api','en_revision',$3,NULL,NULL,NULL)`, entry.RequestID, entry.AccountID, entry.RequestedAt); err != nil {
			return "", "", "", false, mapError(err)
		}
	}
	var alreadySuppressed bool
	if err := tx.QueryRow(ctx, `SELECT baja_iniciada_en IS NOT NULL FROM public.usuario WHERE id=$1`, entry.AccountID).Scan(&alreadySuppressed); err != nil {
		return "", "", "", false, mapError(err)
	}
	if alreadySuppressed {
		if _, err := tx.Exec(ctx, `INSERT INTO public.reaplicacion_baja_local(restore_id,ejecucion_origen_id,usuario_id,estado,intentos,aplicada_en) VALUES($1,$2,$3,'ya_presente',1,$4) ON CONFLICT(restore_id,ejecucion_origen_id) DO UPDATE SET estado='ya_presente',aplicada_en=EXCLUDED.aplicada_en`, restoreID, entry.ExecutionID, entry.AccountID, now); err != nil {
			return "", "", "", false, mapError(err)
		}
		return entry.RequestID, "", "ya_presente", false, tx.Commit(ctx)
	}
	if replayExists && state == "pendiente" {
		var previousState string
		previousKey := replayKey(restoreID, entry.ExecutionID, attempts)
		lookupErr := tx.QueryRow(ctx, `SELECT estado FROM public.ejecucion_baja_local WHERE solicitud_id=$1 AND clave_idempotencia=$2`, entry.RequestID, previousKey).Scan(&previousState)
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return "", "", "", false, mapError(lookupErr)
		}
		if lookupErr == nil && previousState == "bloqueada" {
			attempts++
		}
	} else if !replayExists {
		attempts = 1
	}
	if attempts < 1 {
		attempts = 1
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.reaplicacion_baja_local(restore_id,ejecucion_origen_id,usuario_id,estado,intentos,aplicada_en) VALUES($1,$2,$3,'pendiente',$4,$5) ON CONFLICT(restore_id,ejecucion_origen_id) DO UPDATE SET estado='pendiente',intentos=EXCLUDED.intentos,aplicada_en=EXCLUDED.aplicada_en`, restoreID, entry.ExecutionID, entry.AccountID, attempts, now); err != nil {
		return "", "", "", false, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", "", false, mapError(err)
	}
	return entry.RequestID, replayKey(restoreID, entry.ExecutionID, attempts), "pendiente", false, nil
}

func (r *IdentityRepository) FinishSuppressionReplay(ctx context.Context, entry privacy.SuppressionReplayEntry, restoreID, operationKey, status string, now time.Time) error {
	if status != "reaplicada" && status != "pendiente" {
		return privacy.ErrInvalid
	}
	_, err := r.pool.Exec(ctx, `UPDATE public.reaplicacion_baja_local SET estado=$3,aplicada_en=$4 WHERE restore_id=$1 AND ejecucion_origen_id=$2 AND estado='pendiente'`, restoreID, entry.ExecutionID, status, now)
	return mapError(err)
}
