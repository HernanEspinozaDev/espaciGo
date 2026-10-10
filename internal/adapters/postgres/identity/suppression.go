package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type suppressionDetail struct {
	Obligations []string `json:"obligations_detected"`
	Removed     []string `json:"removed"`
	Retained    []string `json:"retained"`
	Decision    string   `json:"decision_code"`
}

func suppressionError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf("privacy suppression constraint %s: %w", pgErr.ConstraintName, mapError(err))
	}
	return mapError(err)
}

func activeSuppressionObligations(ctx context.Context, tx pgx.Tx, subjectID string) ([]string, error) {
	out := make([]string, 0, 3)
	var found bool
	queries := []struct{ code, sql string }{
		{"reserva_activa", `SELECT EXISTS (SELECT 1 FROM public.reserva_ensayo_local WHERE (anfitrion_id=$1 OR arrendatario_id=$1) AND estado IN ('pendiente_de_pago','pagada','aprobada_host','firma_parcial','lista_para_checkin','en_curso'))`},
		{"pago_o_devolucion_pendiente", `SELECT EXISTS (
		 SELECT 1 FROM public.reserva_pago_ensayo_operacion p JOIN public.reserva_ensayo_local r ON r.id=p.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND p.estado='pendiente'
		 UNION ALL SELECT 1 FROM public.reserva_pago_evento_aplicacion_ensayo a JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id JOIN public.reserva_pago_ensayo_operacion p ON p.id=e.operacion_id JOIN public.reserva_ensayo_local r ON r.id=p.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND a.estado='pendiente_conciliacion'
		 UNION ALL SELECT 1 FROM public.reserva_pago_fake_resultado_ensayo f JOIN public.reserva_pago_ensayo_operacion p ON p.id=f.operacion_id JOIN public.reserva_ensayo_local r ON r.id=p.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND p.estado='vencida' AND f.estado='resultado' AND NOT EXISTS (SELECT 1 FROM public.reserva_pago_evento_ensayo e WHERE e.operacion_id=p.id AND e.proveedor_evento_id=f.proveedor_evento_id)
		 UNION ALL SELECT 1 FROM public.reserva_devolucion_ensayo d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND d.estado='pendiente'
		 UNION ALL SELECT 1 FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND g.estado IN ('pendiente_autorizacion','autorizada','por_conciliar','captura_pendiente','captura_por_conciliar','parcialmente_capturada','liberacion_pendiente','liberacion_por_conciliar')
		 UNION ALL SELECT 1 FROM public.reserva_garantia_operacion_ensayo_local o JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND o.estado IN ('pendiente','por_conciliar')
		 UNION ALL SELECT 1 FROM public.reserva_decision_financiera_ensayo_local d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND d.estado IN ('pendiente','por_conciliar'))`},
		{"disputa_abierta", `SELECT EXISTS (
		 SELECT 1 FROM public.disputa_ensayo_local WHERE (anfitrion_id=$1 OR arrendatario_id=$1) AND estado='abierta'
		 UNION ALL SELECT 1 FROM public.reclamo_dano_ensayo_local WHERE (anfitrion_id=$1 OR arrendatario_id=$1) AND estado='abierto')`},
	}
	for _, item := range queries {
		if err := tx.QueryRow(ctx, item.sql, subjectID).Scan(&found); err != nil {
			return nil, mapError(err)
		}
		if found {
			out = append(out, item.code)
		}
	}
	return out, nil
}

func appendEvidenceIDs(ctx context.Context, tx pgx.Tx, ids *[]string, query, subjectID string) error {
	rows, err := tx.Query(ctx, query, subjectID)
	if err != nil {
		return suppressionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return suppressionError(err)
		}
		*ids = append(*ids, id)
	}
	return suppressionError(rows.Err())
}

func (r *IdentityRepository) ExecuteSuppression(ctx context.Context, actorID, requestID, key, correlationID string, now func() time.Time) (privacy.SuppressionExecution, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var subjectID, requestState string
	if err := tx.QueryRow(ctx, `SELECT usuario_id::text,estado FROM public.solicitud_titular WHERE id=$1 AND tipo='supresion'`, requestID).Scan(&subjectID, &requestState); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.SuppressionExecution{}, privacy.ErrNotFound
		}
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	var accountID string
	// Serialize suppression with local outbox delivery. The notice worker holds
	// the matching session advisory lock from claim through SMTP result commit.
	var advisoryKey int64
	if err := tx.QueryRow(ctx, `SELECT hashtextextended($1::text,0)`, subjectID).Scan(&advisoryKey); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryKey); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if err := tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE id=$1 FOR UPDATE`, subjectID).Scan(&accountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.SuppressionExecution{}, privacy.ErrNotFound
		}
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	var prior privacy.SuppressionExecution
	var priorActor string
	var detailJSON []byte
	var priorID string
	err = tx.QueryRow(ctx, `SELECT id::text,actor_id::text,estado,iniciada_en,completada_en,detalle FROM public.ejecucion_baja_local WHERE solicitud_id=$1 AND clave_idempotencia=$2`, requestID, key).Scan(&priorID, &priorActor, &prior.Status, &prior.StartedAt, &prior.CompletedAt, &detailJSON)
	if err == nil {
		if priorActor != actorID {
			return privacy.SuppressionExecution{}, identity.ErrConflict
		}
		var detail suppressionDetail
		if err := json.Unmarshal(detailJSON, &detail); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		prior.RequestID = requestID
		prior.Outcome = prior.Status
		if prior.Status == "completada" {
			prior.Outcome = "baja_local_con_minimizacion_y_retencion_residual"
		}
		prior.Obligations = detail.Obligations
		prior.Removed = detail.Removed
		prior.Retained = detail.Retained
		prior.DecisionCode = detail.Decision
		prior.Reused = true
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND completada_en IS NULL`, priorID).Scan(&prior.PendingFiles); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		return prior, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if requestState != "en_revision" {
		return privacy.SuppressionExecution{}, identity.ErrConflict
	}
	if now == nil {
		now = time.Now
	}
	checkedAt := now().UTC().Truncate(time.Microsecond)
	obligations, err := activeSuppressionObligations(ctx, tx, subjectID)
	if err != nil {
		return privacy.SuppressionExecution{}, err
	}
	executionID, err := (credentials.Generator{}).ID()
	if err != nil {
		return privacy.SuppressionExecution{}, err
	}
	if len(obligations) > 0 {
		detail := suppressionDetail{Obligations: obligations, Removed: []string{}, Retained: []string{"solicitud_titular", "auditoria_append_only"}, Decision: "supresion_bloqueada_por_obligaciones"}
		encoded, _ := json.Marshal(detail)
		if _, err = tx.Exec(ctx, `INSERT INTO public.ejecucion_baja_local(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,detalle) VALUES($1,$2,$3,$4,$5,'bloqueada','supresion_con_obligaciones',$6,$7::jsonb)`, executionID, requestID, subjectID, actorID, key, checkedAt, string(encoded)); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		if err = r.auditSuppression(ctx, tx, actorID, requestID, key, correlationID, checkedAt, "rechazo", "supresion_con_obligaciones", map[string]any{"obligations_detected": obligations}); err != nil {
			return privacy.SuppressionExecution{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		return privacy.SuppressionExecution{RequestID: requestID, Status: "bloqueada", Outcome: "bloqueada", DecisionCode: detail.Decision, Obligations: obligations, Removed: []string{}, Retained: detail.Retained, StartedAt: checkedAt}, nil
	}
	var alreadyStarted bool
	if err = tx.QueryRow(ctx, `SELECT baja_iniciada_en IS NOT NULL OR estado='desidentificado' FROM public.usuario WHERE id=$1`, subjectID).Scan(&alreadyStarted); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if alreadyStarted {
		return privacy.SuppressionExecution{}, identity.ErrConflict
	}
	var evidenceIDs []string
	rows, err := tx.Query(ctx, `SELECT e.id::text FROM public.verificacion_evidencia_sintetica e JOIN public.verificacion v ON v.id=e.verificacion_id WHERE v.usuario_id=$1 ORDER BY e.id`, subjectID)
	if err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		evidenceIDs = append(evidenceIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	rows.Close()
	photoRows, err := tx.Query(ctx, `SELECT id::text FROM public.foto_perfil_sintetica_local WHERE usuario_id=$1 AND archivo_id IS NOT NULL ORDER BY id`, subjectID)
	if err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	for photoRows.Next() {
		var id string
		if err := photoRows.Scan(&id); err != nil {
			photoRows.Close()
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		evidenceIDs = append(evidenceIDs, id)
	}
	if err := photoRows.Err(); err != nil {
		photoRows.Close()
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	photoRows.Close()
	galleryRows, err := tx.Query(ctx, `SELECT archivo_id::text FROM public.espacio_galeria_sintetica_local WHERE propietario_id=$1 AND archivo_id IS NOT NULL ORDER BY id`, subjectID)
	if err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	for galleryRows.Next() {
		var id string
		if err := galleryRows.Scan(&id); err != nil {
			galleryRows.Close()
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
		evidenceIDs = append(evidenceIDs, id)
	}
	if err := galleryRows.Err(); err != nil {
		galleryRows.Close()
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	galleryRows.Close()
	if err = appendEvidenceIDs(ctx, tx, &evidenceIDs, `SELECT e.id::text
		FROM public.operacion_arriendo_evidencia_ensayo_local e
		JOIN public.operacion_arriendo_ensayo_local o ON o.id=e.operacion_id
		JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id
		WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1 ORDER BY e.id`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, err
	}
	if err = appendEvidenceIDs(ctx, tx, &evidenceIDs, `SELECT c.archivo_id::text
		FROM public.operacion_arriendo_archivo_candidato_local c
		JOIN public.reserva_ensayo_local r ON r.id=c.reserva_id
		WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1 ORDER BY c.archivo_id`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, err
	}
	uniqueEvidenceIDs := make([]string, 0, len(evidenceIDs))
	seenEvidenceIDs := make(map[string]struct{}, len(evidenceIDs))
	for _, id := range evidenceIDs {
		if _, seen := seenEvidenceIDs[id]; !seen {
			seenEvidenceIDs[id] = struct{}{}
			uniqueEvidenceIDs = append(uniqueEvidenceIDs, id)
		}
	}
	evidenceIDs = uniqueEvidenceIDs
	removed := []string{"sesiones_y_tokens", "hash_vigente_e_historial_claves", "roles_activos", "perfil_y_preferencia", "foto_sintetica_con_limpieza_recuperable", "galeria_sintetica_privada_con_limpieza_recuperable", "referencias_cuenta_cobro_fake", "contenido_de_borradores", "fixtures_del_titular", "cotizaciones_no_convertidas", "mensajes_de_reservas_terminales_sinteticas", "simulaciones_privadas", "avisos_de_credenciales_sin_finalidad", "claves_idempotentes_m02", "texto_libre_reclamo_descargo_y_operaciones_sinteticas", "blobs_evidencia_operativa_sintetica_con_limpieza_recuperable", "estado_vigente_bloqueo_administrativo_del_titular"}
	retained := []string{"ancla_tecnica_usuario", "metadata_minima_de_galeria_sintetica_sin_binario", "aceptaciones_terminos_5_anios", "solicitud_y_decision_5_anios", "metadata_verificacion_2_anios", "reservas_pagos_devoluciones_y_reclamos_24_meses_desde_ultimo_cierre", "historiales_transaccionales_minimizados", "historial_bloqueo_cuenta_motivo_fecha_sin_actor_personal_plazo_pendiente_de_ratificacion", "auditoria_5_anios", "copias_locales_no_eliminadas"}
	detail := suppressionDetail{Obligations: []string{}, Removed: removed, Retained: retained, Decision: "baja_elegible_privacidad_local_v1"}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return privacy.SuppressionExecution{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.ejecucion_baja_local(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,detalle) VALUES($1,$2,$3,$4,$5,'limpieza_pendiente','baja_local_minimizada',$6,$7::jsonb)`, executionID, requestID, subjectID, actorID, key, checkedAt, string(encoded)); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	for _, evidenceID := range evidenceIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO public.baja_archivo_pendiente_local(ejecucion_id,evidencia_id,disponible_en) VALUES($1,$2,$3)`, executionID, evidenceID, checkedAt); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public.foto_perfil_sintetica_local SET estado='retirada_baja',retirada_en=COALESCE(retirada_en,$2),proximo_intento_en=CASE WHEN archivo_id IS NULL THEN NULL ELSE $2 END WHERE usuario_id=$1 AND estado<>'retirada_baja'`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET estado='retirada_baja',retirada_en=COALESCE(retirada_en,$2),proximo_intento_en=CASE WHEN archivo_id IS NULL THEN NULL ELSE $2 END WHERE propietario_id=$1 AND estado<>'retirada_baja'`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.cuenta_cobro_sintetica_local SET estado='retirada_baja',referencia_ficticia=NULL,revocada_en=COALESCE(revocada_en,$2),actualizada_en=$2 WHERE usuario_id=$1 AND estado IN ('activa','reemplazada','revocada')`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.cuenta_cobro_sintetica_historial_local(cuenta_id,usuario_id,accion,ocurrida_en,correlacion_id,clave_idempotencia) SELECT id,usuario_id,'retirada_baja',$2,$3,'privacy-suppression:'||$3 FROM public.cuenta_cobro_sintetica_local WHERE usuario_id=$1 ON CONFLICT DO NOTHING`, subjectID, checkedAt, requestID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	// Retain synthetic reputation facts until their own ratified deadlines while
	// removing direct account links and free-form text during eligible erasure.
	if _, err = tx.Exec(ctx, `UPDATE public.resena_ensayo_local
		SET comentario=CASE WHEN autor_id=$1 OR destinatario_id=$1 THEN '' ELSE comentario END,
		    autor_id=CASE WHEN autor_id=$1 THEN NULL ELSE autor_id END,
		    destinatario_tipo=CASE WHEN destinatario_id=$1 THEN 'arrendatario_retirado' ELSE destinatario_tipo END,
		    destinatario_id=CASE WHEN destinatario_id=$1 THEN NULL ELSE destinatario_id END
		WHERE autor_id=$1 OR destinatario_id=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reporte_resena_ensayo_local
		SET anfitrion_id=CASE WHEN anfitrion_id=$1 THEN NULL ELSE anfitrion_id END,
		    resuelta_por=CASE WHEN resuelta_por=$1 THEN NULL ELSE resuelta_por END
		WHERE anfitrion_id=$1 OR resuelta_por=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reporte_resena_historial_local SET actor_id=NULL WHERE actor_id=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reclamo_dano_ensayo_local
		SET descripcion='Texto libre retirado por baja local de privacidad.'
		WHERE (anfitrion_id=$1 OR arrendatario_id=$1) AND estado='resuelta'`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reclamo_dano_descargo_ensayo_local d
		SET descripcion='Texto libre retirado por baja local de privacidad.'
		FROM public.reclamo_dano_ensayo_local c
		WHERE d.reclamo_id=c.id AND c.estado='resuelta' AND (c.anfitrion_id=$1 OR c.arrendatario_id=$1)`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.operacion_arriendo_ensayo_local o
		SET comentarios='',observacion=''
		FROM public.reserva_ensayo_local r
		WHERE o.reserva_id=r.id AND (r.anfitrion_id=$1 OR r.arrendatario_id=$1)`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='cancelada',cancelada_en=$2,retirar_en=$2::timestamptz+interval '30 days',lease_hasta=NULL,codigo_error='destinatario_retirado'
		WHERE destinatario_id=$1 AND estado IN ('pendiente','procesando')`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.aviso_local_ciclo c SET estado='cancelada',finalizada_en=$2,codigo_resultado='destinatario_retirado'
		FROM public.aviso_local a WHERE c.aviso_id=a.id AND c.ciclo=a.ciclo AND a.destinatario_id=$1 AND a.estado='cancelada' AND c.estado='pendiente'`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.m02_operacion_idempotente_local WHERE usuario_id=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if err = r.auditSuppression(ctx, tx, actorID, requestID, key, correlationID, checkedAt, "exito", "baja_local_minimizada", map[string]any{"removed": removed, "retained": retained, "file_jobs": len(evidenceIDs)}); err != nil {
		return privacy.SuppressionExecution{}, err
	}
	// Preserve the account anchor for restricted historical foreign keys. No
	// usable credential or contact address remains after this update.
	if _, err = tx.Exec(ctx, `UPDATE public.usuario SET estado='desidentificado',correo_original='Cuenta retirada',correo_normalizado='retirada+'||id::text||'@invalid.local',hash_clave='',preferencia_uso=NULL,baja_iniciada_en=$2,actualizado_en=$2 WHERE id=$1`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	for _, statement := range []string{
		`DELETE FROM public.sesion WHERE usuario_id=$1`,
		`DELETE FROM public.token_accion WHERE usuario_id=$1`,
		`DELETE FROM public.historial_clave_local WHERE usuario_id=$1`,
		`DELETE FROM public.perfil_usuario WHERE usuario_id=$1`,
		`UPDATE public.rol_usuario SET concedido_por=NULL WHERE concedido_por=$1`,
		`DELETE FROM public.rol_usuario WHERE usuario_id=$1`,
	} {
		if _, err = tx.Exec(ctx, statement, subjectID); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
	}
	// Blocking is current administrative state, not a privacy-retention record;
	// the account's inactivity is the effective gate after suppression. Preserve
	// the structured history while removing any actor link to the retired account.
	if _, err = tx.Exec(ctx, `DELETE FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.bloqueo_cuenta_administrativo_local SET bloqueada_por=NULL WHERE bloqueada_por=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.bloqueo_cuenta_historial_local SET actor_id=NULL WHERE actor_id=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	// Keep a terminal case's original resolution/deadline. Pending cases are
	// closed by the privacy action and retain from that terminal event instead.
	if _, err = tx.Exec(ctx, `UPDATE public.verificacion SET estado='retirada_privacidad',
		revisor_id=NULL,motivo_codigo='baja_privacidad',resuelta_en=COALESCE(resuelta_en,$2::timestamptz),
		retirada_privacidad_en=$2::timestamptz WHERE usuario_id=$1`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	// End derived KYC/KYB eligibility in the same protected transaction. Keep
	// the source cases, append-only history and their original retention dates.
	if _, err = tx.Exec(ctx, `INSERT INTO public.verificacion_historial_local(
		verificacion_id,actor_id,accion,estado_anterior,estado_nuevo,motivo_codigo,correlacion_id,clave_idempotencia,ocurrida_en)
		SELECT e.verificacion_id,$2,'elegibilidad_retirada_privacidad','aprobada','retirada_privacidad','baja_privacidad',
			$3||':'||e.tipo,$4,$5
		FROM public.elegibilidad_verificacion_local e
		WHERE e.usuario_id=$1 AND e.estado='elegible'
		ON CONFLICT (verificacion_id,clave_idempotencia) DO NOTHING`, subjectID, actorID, "privacy-suppression:"+requestID, "privacy-suppression:"+requestID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.elegibilidad_verificacion_local SET estado='retirada_privacidad',
		revocada_en=$2,revocada_por=$3,motivo_revocacion_codigo='baja_privacidad'
		WHERE usuario_id=$1 AND estado='elegible'`, subjectID, checkedAt, actorID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	termExpiry := identity.AddCalendarMonthsUTC(checkedAt, 60)
	if _, err = tx.Exec(ctx, `UPDATE public.aceptacion_terminos SET retirar_en=$2 WHERE usuario_id=$1`, subjectID, termExpiry); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.cotizacion_reserva_ensayo q WHERE (q.arrendatario_id=$1 OR q.anfitrion_id=$1) AND NOT EXISTS (SELECT 1 FROM public.reserva_ensayo_local r WHERE r.cotizacion_id=q.id)`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local r SET vinculos_retirar_en=GREATEST(COALESCE(r.vinculos_retirar_en,$2::timestamptz),(
		SELECT GREATEST(r.actualizada_en,
			COALESCE((SELECT max(p.actualizada_en) FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=r.id),r.actualizada_en),
			COALESCE((SELECT max(d.actualizada_en) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),r.actualizada_en),
			COALESCE((SELECT max(x.cerrada_en) FROM public.disputa_ensayo_local x WHERE x.reserva_id=r.id),r.actualizada_en),
			COALESCE((SELECT max(x.resuelta_en) FROM public.reclamo_dano_resolucion_ensayo_local x JOIN public.reclamo_dano_ensayo_local c ON c.id=x.reclamo_id WHERE c.reserva_id=r.id),r.actualizada_en)
		) + interval '24 months'
	)) WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1)
	 AND (r.estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','en_disputa')
	      OR EXISTS (SELECT 1 FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=r.id))`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `WITH cancelled AS (
		UPDATE public.outbox_evento_local AS event
		SET cancelada_en=$2::timestamptz,motivo_cancelacion_codigo='baja_local_sin_finalidad',
			lease_hasta=NULL,retirar_en=$2::timestamptz + interval '30 days',ultimo_error='account_suppressed'
		WHERE event.agregado_tipo='usuario' AND event.agregado_id=$1
		  AND event.tipo='identidad.credencial_cambiada'
		  AND event.entregada_en IS NULL AND event.cancelada_en IS NULL
		  AND event.fallo_terminal_en IS NULL
		  AND EXISTS (
			SELECT 1 FROM public.outbox_evento_ciclo_local AS cycle
			WHERE cycle.evento_id=event.id AND cycle.numero_ciclo=event.ciclo_actual AND cycle.estado='pendiente'
		  )
		RETURNING event.id,event.ciclo_actual,event.cancelada_en
	)
	UPDATE public.outbox_evento_ciclo_local AS cycle
	SET estado='cancelada',finalizada_en=cancelled.cancelada_en,codigo_resultado='baja_local_sin_finalidad'
	FROM cancelled
	WHERE cycle.evento_id=cancelled.id AND cycle.numero_ciclo=cancelled.ciclo_actual`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local_fixture SET habilitada=false WHERE anfitrion_id=$1 OR arrendatario_id=$1`, subjectID); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	for _, statement := range []string{
		`DELETE FROM public.reserva_mensaje_lectura c USING public.reserva_ensayo_local r WHERE c.reserva_id=r.id AND (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND r.estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')`,
		`DELETE FROM public.mensaje_reserva_ensayo m USING public.reserva_ensayo_local r WHERE m.reserva_id=r.id AND (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND r.estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')`,
		`DELETE FROM public.simulacion_precio_privada WHERE propietario_id=$1`,
		`DELETE FROM public.espacio_horario_semanal h USING public.espacio e WHERE h.espacio_id=e.id AND e.propietario_id=$1`,
		`DELETE FROM public.espacio_caracteristicas c USING public.espacio e WHERE c.espacio_id=e.id AND e.propietario_id=$1`,
	} {
		if _, err = tx.Exec(ctx, statement, subjectID); err != nil {
			return privacy.SuppressionExecution{}, suppressionError(err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET titulo='Borrador retirado',descripcion='Contenido retirado por una baja local de privacidad. Este texto sintético sustituye el contenido libre del borrador.',reglas_uso='Contenido retirado',direccion='retirado:'||id::text,actualizado_en=$2 WHERE propietario_id=$1`, subjectID, checkedAt); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.ejecucion_baja_local SET detalle=detalle || jsonb_build_object('accepted_terms_retain_until',$2::timestamptz,'outbox_cancel_reason','baja_local_sin_finalidad') WHERE id=$1`, executionID, termExpiry); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return privacy.SuppressionExecution{}, suppressionError(err)
	}
	return privacy.SuppressionExecution{RequestID: requestID, Status: "limpieza_pendiente", Outcome: "limpieza_pendiente", DecisionCode: detail.Decision, Obligations: []string{}, Removed: removed, Retained: retained, PendingFiles: len(evidenceIDs), StartedAt: checkedAt}, nil
}

func (r *IdentityRepository) auditSuppression(ctx context.Context, tx pgx.Tx, actorID, requestID, key, correlation string, at time.Time, result, reason string, detail any) error {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		return err
	}
	// Keep the append-only audit payload within the existing structured code contract.
	var obligations any = []string{}
	if codes, ok := detail.(map[string]any); ok {
		if value, exists := codes["obligations_detected"]; exists {
			obligations = value
		}
	}
	compact := map[string]any{"obligations_detected": obligations, "pending_checks": []string{}}
	encoded, _ := json.Marshal(compact)
	_, err = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en,clave_idempotencia,detalle_codigos) VALUES($1,$2,'solicitud_titular',$3,'privacy.suppression.execute',$4,$5,$6,$7::timestamptz,$7::timestamptz + interval '5 years',$8,$9::jsonb)`, id, actorID, requestID, result, reason, correlation, at, key, string(encoded))
	return mapError(err)
}

func (r *IdentityRepository) PendingSuppressionFiles(ctx context.Context, requestID string, now time.Time) ([]privacy.SuppressionFile, error) {
	rows, err := r.pool.Query(ctx, `SELECT j.ejecucion_id::text,j.evidencia_id::text FROM public.baja_archivo_pendiente_local j JOIN public.ejecucion_baja_local x ON x.id=j.ejecucion_id WHERE x.solicitud_id=$1 AND x.estado='limpieza_pendiente' AND j.completada_en IS NULL AND j.disponible_en<=$2 ORDER BY j.ejecucion_id,j.evidencia_id`, requestID, now.UTC())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := []privacy.SuppressionFile{}
	for rows.Next() {
		var v privacy.SuppressionFile
		if err := rows.Scan(&v.ExecutionID, &v.EvidenceID); err != nil {
			return nil, mapError(err)
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *IdentityRepository) PendingSuppressionRequests(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT solicitud_id::text FROM public.ejecucion_baja_local WHERE estado='limpieza_pendiente' ORDER BY iniciada_en,id`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapError(err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *IdentityRepository) CompleteSuppressionFile(ctx context.Context, file privacy.SuppressionFile, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var done *time.Time
	if err = tx.QueryRow(ctx, `SELECT completada_en FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND evidencia_id=$2 FOR UPDATE`, file.ExecutionID, file.EvidenceID).Scan(&done); err != nil {
		return mapError(err)
	}
	if done == nil {
		if _, err = tx.Exec(ctx, `DELETE FROM public.verificacion_evidencia_sintetica WHERE id=$1`, file.EvidenceID); err != nil {
			return mapError(err)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM public.operacion_arriendo_archivo_candidato_local WHERE archivo_id=$1`, file.EvidenceID); err != nil {
			return mapError(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE public.foto_perfil_sintetica_local SET archivo_id=NULL,mime_type=NULL,sha256=NULL,size_bytes=NULL,limpia_en=$2,proximo_intento_en=NULL,ultimo_codigo_error=NULL WHERE id=$1 AND estado='retirada_baja'`, file.EvidenceID, at.UTC()); err != nil {
			return mapError(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET archivo_id=NULL,limpia_en=$2,proximo_intento_en=NULL,ultimo_codigo_error=NULL WHERE id=$1 AND estado='retirada_baja'`, file.EvidenceID, at.UTC()); err != nil {
			return mapError(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE public.baja_archivo_pendiente_local SET completada_en=$3,ultimo_codigo_error=NULL WHERE ejecucion_id=$1 AND evidencia_id=$2`, file.ExecutionID, file.EvidenceID, at.UTC()); err != nil {
			return mapError(err)
		}
	}
	return mapError(tx.Commit(ctx))
}
func (r *IdentityRepository) FailSuppressionFile(ctx context.Context, file privacy.SuppressionFile, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.baja_archivo_pendiente_local SET intentos=intentos+1,disponible_en=$3 + LEAST(interval '5 minutes', interval '5 seconds' * (intentos+1)),ultimo_codigo_error='archivo_sintetico_no_eliminado' WHERE ejecucion_id=$1 AND evidencia_id=$2 AND completada_en IS NULL`, file.ExecutionID, file.EvidenceID, at.UTC())
	return mapError(err)
}

func (r *IdentityRepository) FinishSuppression(ctx context.Context, requestID string, at time.Time) (privacy.SuppressionExecution, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return privacy.SuppressionExecution{}, mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var exec privacy.SuppressionExecution
	var execID, subjectID, requestState string
	var key string
	var detailJSON []byte
	err = tx.QueryRow(ctx, `SELECT x.id::text,x.usuario_id::text,x.clave_idempotencia,x.estado,x.iniciada_en,x.completada_en,x.detalle,s.estado FROM public.ejecucion_baja_local x JOIN public.solicitud_titular s ON s.id=x.solicitud_id WHERE x.solicitud_id=$1 AND x.estado IN ('limpieza_pendiente','completada') ORDER BY x.iniciada_en DESC LIMIT 1 FOR UPDATE OF x,s`, requestID).Scan(&execID, &subjectID, &key, &exec.Status, &exec.StartedAt, &exec.CompletedAt, &detailJSON, &requestState)
	if err != nil {
		return privacy.SuppressionExecution{}, mapError(err)
	}
	if requestState == "resuelta" {
		exec.RequestID = requestID
		exec.Outcome = "baja_local_con_minimizacion_y_retencion_residual"
		exec.Status = "completada"
		exec.Reused = true
		var detail suppressionDetail
		if err := json.Unmarshal(detailJSON, &detail); err != nil {
			return exec, err
		}
		exec.Obligations, exec.Removed, exec.Retained, exec.DecisionCode = detail.Obligations, detail.Removed, detail.Retained, detail.Decision
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND completada_en IS NULL`, execID).Scan(&exec.PendingFiles); err != nil {
			return exec, mapError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return exec, mapError(err)
		}
		return exec, nil
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND completada_en IS NULL`, execID).Scan(&exec.PendingFiles); err != nil {
		return exec, mapError(err)
	}
	if exec.PendingFiles > 0 {
		exec.RequestID = requestID
		exec.Outcome = "limpieza_pendiente"
		var detail suppressionDetail
		if err := json.Unmarshal(detailJSON, &detail); err != nil {
			return exec, err
		}
		exec.Obligations, exec.Removed, exec.Retained, exec.DecisionCode = detail.Obligations, detail.Removed, detail.Retained, detail.Decision
		if err := tx.Commit(ctx); err != nil {
			return exec, mapError(err)
		}
		return exec, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	retireAt := identity.AddCalendarMonthsUTC(at.UTC(), 60)
	if _, err = tx.Exec(ctx, `UPDATE public.solicitud_titular SET estado='resuelta',resuelta_en=$2,motivo_resolucion_codigo='baja_local_minimizada',retirar_en=$3 WHERE id=$1 AND estado='en_revision'`, requestID, at.UTC(), retireAt); err != nil {
		return privacy.SuppressionExecution{}, mapError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.ejecucion_baja_local SET estado='completada',completada_en=$2,retirar_en=$3 WHERE id=$1 AND estado='limpieza_pendiente'`, execID, at.UTC(), retireAt); err != nil {
		return privacy.SuppressionExecution{}, mapError(err)
	}
	if err = tx.QueryRow(ctx, `SELECT estado,iniciada_en,completada_en,detalle FROM public.ejecucion_baja_local WHERE id=$1`, execID).Scan(&exec.Status, &exec.StartedAt, &exec.CompletedAt, &detailJSON); err != nil {
		return privacy.SuppressionExecution{}, mapError(err)
	}
	var detail suppressionDetail
	if err = json.Unmarshal(detailJSON, &detail); err != nil {
		return privacy.SuppressionExecution{}, err
	}
	exec.RequestID = requestID
	exec.Status = "completada"
	exec.Outcome = "baja_local_con_minimizacion_y_retencion_residual"
	exec.Obligations = detail.Obligations
	exec.Removed = detail.Removed
	exec.Retained = detail.Retained
	exec.DecisionCode = detail.Decision
	if err = tx.Commit(ctx); err != nil {
		return privacy.SuppressionExecution{}, mapError(err)
	}
	return exec, nil
}
