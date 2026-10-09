package bookingpg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

// closeGuaranteeOnCancellation never treats an authorization as a capture.
// Uncertain authorization remains pending for reconciliation; a confirmed
// balance receives its own durable release operation.
func closeGuaranteeOnCancellation(ctx context.Context, tx pgx.Tx, reservationID string, now time.Time) error {
	var gid, state string
	var authorized, captured, released int64
	err := tx.QueryRow(ctx, `SELECT g.id::text,g.estado,g.autorizado_clp,g.capturado_clp,g.liberado_clp FROM public.reserva_garantia_ensayo_local g WHERE g.reserva_id=$1 FOR UPDATE`, reservationID).Scan(&gid, &state, &authorized, &captured, &released)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state == "autorizada" || state == "parcialmente_capturada" {
		remaining := authorized - captured - released
		if remaining > 0 {
			fp, _ := json.Marshal([]any{"liberacion", remaining, "exito"})
			sum := sha256.Sum256(fp)
			_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,'liberacion',$2,$3,$4,'exito','pendiente',$5,NULL,NULL,$5,$5) ON CONFLICT(garantia_id,tipo) DO NOTHING`, gid, "cancelacion-libera:"+reservationID, sum[:], remaining, now)
			if err != nil {
				return err
			}
			state = "liberacion_pendiente"
		}
	} else if state == "pendiente_pago" || state == "pendiente_autorizacion" {
		var uncertain bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_garantia_operacion_ensayo_local WHERE garantia_id=$1 AND tipo='autorizacion' AND estado='por_conciliar')`, gid).Scan(&uncertain); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_operacion_ensayo_local SET estado='vencida',ultimo_resultado='cancelada',completada_en=$2,actualizada_en=$2 WHERE garantia_id=$1 AND tipo='autorizacion' AND estado='pendiente'`, gid, now); err != nil {
			return err
		}
		if uncertain {
			state = "por_conciliar"
		} else {
			state = "cancelada"
		}
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1`, gid, state, now)
	return err
}

func cancelPaidReservationForGuarantee(ctx context.Context, tx pgx.Tx, reservationID string, actor any, reason string, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, reservationID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_por_pago',actualizada_en=$2 WHERE id=$1 AND estado='pagada'`, reservationID, now); err != nil {
		return err
	}
	if err := appendTransition(ctx, tx, reservationID, ptrString("pagada"), "cancelada_por_pago", actor, reason, now); err != nil {
		return err
	}
	var paid int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(importe_clp),0) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'`, reservationID).Scan(&paid); err != nil {
		return err
	}
	if paid > 0 {
		_, err := tx.Exec(ctx, `INSERT INTO public.reserva_devolucion_ensayo(id,reserva_id,operacion_id,importe_clp,moneda,estado,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,gen_random_uuid(),$2,'CLP','pendiente',$3,$3) ON CONFLICT(reserva_id) DO NOTHING`, reservationID, paid, now)
		return err
	}
	return nil
}

func guaranteeHistoryType(kind, state string) string {
	if kind == "autorizacion" {
		if state == "rechazada" {
			return "garantia_rechazada"
		}
		if state == "vencida" {
			return "garantia_vencida"
		}
		if state == "por_conciliar" || state == "pendiente" {
			return "operacion_pendiente_conciliacion"
		}
		return "garantia_autorizada"
	}
	if kind == "captura" {
		return "garantia_capturada"
	}
	if kind == "liberacion" {
		return "garantia_liberada"
	}
	return "operacion_pendiente_conciliacion"
}

func (r *Repository) Guarantee(ctx context.Context, actor, reservationID string) (booking.GuaranteeSnapshot, error) {
	return r.getGuarantee(ctx, actor, reservationID, false)
}

func (r *Repository) GuaranteeForAdministrator(ctx context.Context, reservationID string) (booking.GuaranteeSnapshot, error) {
	// The shared query binds the participant ID as uuid even for its admin
	// branch; use a valid sentinel UUID rather than an empty string.
	return r.getGuarantee(ctx, "00000000-0000-0000-0000-000000000000", reservationID, true)
}

func (r *Repository) getGuarantee(ctx context.Context, actor, reservationID string, administrator bool) (booking.GuaranteeSnapshot, error) {
	var v booking.GuaranteeSnapshot
	var deadline *time.Time
	err := r.pool.QueryRow(ctx, `SELECT g.politica_version,g.moneda,g.previsto_clp,g.autorizado_clp,g.capturado_clp,g.liberado_clp,g.estado,
 (SELECT min(o.vence_en) FROM public.reserva_garantia_operacion_ensayo_local o WHERE o.garantia_id=g.id AND o.tipo='autorizacion'),g.id::text
 FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id
	 WHERE r.id=$1 AND (r.anfitrion_id=$2 OR r.arrendatario_id=$2 OR $3::boolean)`, reservationID, actor, administrator).
		Scan(&v.PolicyVersion, &v.Currency, &v.ExpectedCLP, &v.AuthorizedCLP, &v.CapturedCLP, &v.ReleasedCLP, &v.State, &deadline, new(string))
	if err != nil {
		return v, mapErr(err)
	}
	v.AuthorizationDeadline = deadline
	v.Operations = []booking.GuaranteeOperation{}
	rows, err := r.pool.Query(ctx, `SELECT o.id::text,o.tipo,o.importe_clp,o.estado,COALESCE(o.ultimo_resultado,''),o.creada_en,o.actualizada_en
 FROM public.reserva_garantia_operacion_ensayo_local o JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id
 JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE r.id=$1 AND (r.anfitrion_id=$2 OR r.arrendatario_id=$2 OR $3::boolean) ORDER BY o.creada_en,o.id`, reservationID, actor, administrator)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var o booking.GuaranteeOperation
		if err = rows.Scan(&o.ID, &o.Kind, &o.AmountCLP, &o.State, &o.LastResult, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return v, err
		}
		v.Operations = append(v.Operations, o)
	}
	if err = rows.Err(); err != nil {
		return v, err
	}
	var d booking.FinancialDecision
	err = r.pool.QueryRow(ctx, `SELECT id::text,COALESCE(reclamo_id::text,''),resultado_reclamo,deduccion_clp,motivo_codigo,evidencia_id::text,estado,creada_en,actualizada_en
 FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1`, reservationID).Scan(&d.ID, &d.ClaimID, &d.Outcome, &d.DeductionCLP, &d.ReasonCode, &d.EvidenceID, &d.State, &d.CreatedAt, &d.UpdatedAt)
	if err == nil {
		v.Decision = &d
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	return v, nil
}

func (r *Repository) RunGuaranteeOperation(ctx context.Context, actor, reservationID, kind, key string, amount int64, outcome, operationID string, administrator bool, clock func() time.Time) (booking.GuaranteeOperation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.GuaranteeOperation{}, false, err
	}
	defer tx.Rollback(ctx)
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return booking.GuaranteeOperation{}, false, err
	}
	var v booking.Reservation
	v, err = scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND (arrendatario_id=$2 OR anfitrion_id=$2 OR $3::boolean) FOR UPDATE`, reservationID, actor, administrator))
	if err != nil {
		return booking.GuaranteeOperation{}, false, err
	}
	var gid, guaranteeState string
	var authorized, captured, released int64
	err = tx.QueryRow(ctx, `SELECT id::text,estado,autorizado_clp,capturado_clp,liberado_clp FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&gid, &guaranteeState, &authorized, &captured, &released)
	if err != nil {
		return booking.GuaranteeOperation{}, false, mapErr(err)
	}
	if kind == "autorizacion" && actor != v.RenterID {
		return booking.GuaranteeOperation{}, false, booking.ErrNotFound
	}
	var old booking.GuaranteeOperation
	var oldHash []byte
	err = tx.QueryRow(ctx, `SELECT id::text,tipo,importe_clp,estado,COALESCE(ultimo_resultado,''),creada_en,actualizada_en,huella_solicitud FROM public.reserva_garantia_operacion_ensayo_local WHERE garantia_id=$1 AND tipo=$2 FOR UPDATE`, gid, kind).Scan(&old.ID, &old.Kind, &old.AmountCLP, &old.State, &old.LastResult, &old.CreatedAt, &old.UpdatedAt, &oldHash)
	if err == nil {
		fingerprint, _ := json.Marshal([]any{kind, amount, outcome})
		sum := sha256.Sum256(fingerprint)
		if !equalBytes(oldHash, sum[:]) {
			return old, false, booking.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return old, false, err
		}
		return old, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return old, false, err
	}
	now := clock().UTC()
	if kind == "autorizacion" {
		if v.State != "pagada" || guaranteeState != "pendiente_pago" {
			return old, false, booking.ErrConflict
		}
	} else {
		// Mutating guarantee operations are available only after an approved financial decision.
		if v.State != "finalizada" && v.State != "en_disputa" && v.State != "en_curso" {
			return old, false, booking.ErrConflict
		}
		var deduction int64
		var decisionState, outcomeDecision string
		var decisionFound bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1),COALESCE((SELECT deduccion_clp FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1),0),COALESCE((SELECT estado FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1),''),COALESCE((SELECT resultado_reclamo FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1),'')`, reservationID).Scan(&decisionFound, &deduction, &decisionState, &outcomeDecision)
		if err != nil {
			return old, false, err
		}
		if kind == "captura" && (!decisionFound || deduction <= 0 || amount != deduction || decisionState != "pendiente") {
			return old, false, booking.ErrConflict
		}
		if kind == "liberacion" {
			if !decisionFound { // No claim: release only after the persisted 24-hour checkout window.
				var eligible bool
				if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=$1 AND o.tipo='checkout' AND o.ocurrio_en+interval '24 hours'<=$2) AND NOT EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=$1 AND c.estado='abierto')`, reservationID, now).Scan(&eligible); err != nil {
					return old, false, err
				}
				if !eligible {
					return old, false, booking.ErrConflict
				}
			} else if deduction > 0 && decisionState != "aplicada" {
				return old, false, booking.ErrConflict
			}
			if outcomeDecision == "rechazado" && deduction != 0 {
				return old, false, booking.ErrConflict
			}
			if amount != authorized-captured-released {
				return old, false, booking.ErrConflict
			}
		}
	}
	fingerprint, _ := json.Marshal([]any{kind, amount, outcome})
	sum := sha256.Sum256(fingerprint)
	var deadline any
	if kind == "autorizacion" {
		d := now.Add(15 * time.Minute)
		if v.StartAt.Before(d) {
			d = v.StartAt
		}
		deadline = d
	}
	state, last := "confirmada", "exito_simulado"
	if outcome == "sin_respuesta" {
		state, last = "por_conciliar", "sin_respuesta_simulada"
	} else if outcome == "rechazo" {
		state, last = "rechazada", "rechazo_simulado"
	}
	// The fake outcome remains authoritative even when it arrives at the
	// reservation start. A timeout is still uncertain and must stay reconcilable;
	// a confirmed success is recorded as late and compensated with a release.
	lateAuthorization := kind == "autorizacion" && !now.Before(v.StartAt)
	if lateAuthorization {
		switch outcome {
		case "exito":
			state, last = "vencida", "resultado_tardio"
		case "rechazo":
			state, last = "rechazada", "rechazo_simulado"
		default:
			state, last = "por_conciliar", "sin_respuesta_simulada"
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,vence_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::timestamptz,$10::timestamptz,$11,CASE WHEN $8 IN ('confirmada','rechazada','vencida') THEN $9::timestamptz END,$9::timestamptz,$9::timestamptz)`, operationID, gid, kind, key, sum[:], amount, outcome, state, now, deadline, last)
	if err != nil {
		return old, false, mapErr(err)
	}
	if outcome == "sin_respuesta" {
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_resultado_fake_ensayo_local(operacion_id,resultado,registrada_en) VALUES($1,'sin_respuesta_simulada',$2)`, operationID, now); err != nil {
			return old, false, err
		}
	} else {
		fakeResult := "exito_simulado"
		if outcome == "rechazo" {
			fakeResult = "rechazo_simulado"
		}
		eventID := operationID
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_resultado_fake_ensayo_local(operacion_id,evento_id,resultado,registrada_en) VALUES($1,$2,$3,$4)`, operationID, eventID, fakeResult, now); err != nil {
			return old, false, err
		}
		var eventRow string
		if err = tx.QueryRow(ctx, `INSERT INTO public.reserva_garantia_evento_ensayo_local(operacion_id,evento_id,resultado,autenticado_en) VALUES($1,$2,$3,$4) RETURNING id::text`, operationID, eventID, fakeResult, now).Scan(&eventRow); err != nil {
			return old, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_evento_aplicacion_ensayo_local(evento_id,estado,codigo_resultado,creada_en,procesada_en) VALUES($1,'aplicada','aplicado',$2,$2)`, eventRow, now); err != nil {
			return old, false, err
		}
	}
	if kind == "autorizacion" {
		switch state {
		case "confirmada":
			guaranteeState = "autorizada"
			authorized = 50000
		case "rechazada":
			guaranteeState = "no_disponible"
		case "vencida":
			guaranteeState = "liberacion_pendiente"
			fp, _ := json.Marshal([]any{"liberacion", int64(50000), "exito"})
			sum := sha256.Sum256(fp)
			_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
VALUES(gen_random_uuid(),$1,'liberacion',$2,$3,50000,'exito','pendiente',$4,NULL,NULL,$4,$4) ON CONFLICT(garantia_id,tipo) DO NOTHING`, gid, "libera-tardia:"+operationID, sum[:], now)
			if err != nil {
				return old, false, err
			}
		default:
			guaranteeState = "por_conciliar"
		}
	} else if state == "confirmada" {
		if kind == "captura" {
			if amount > authorized-captured-released {
				return old, false, booking.ErrConflict
			}
			captured += amount
			guaranteeState = "parcialmente_capturada"
			if _, err = tx.Exec(ctx, `UPDATE public.reserva_decision_financiera_ensayo_local SET estado='aplicada',actualizada_en=$2 WHERE reserva_id=$1 AND estado='pendiente'`, reservationID, now); err != nil {
				return old, false, err
			}
		}
		if kind == "liberacion" {
			if amount > authorized-captured-released {
				return old, false, booking.ErrConflict
			}
			released += amount
			guaranteeState = "liberada"
		}
	} else {
		switch kind {
		case "autorizacion":
			guaranteeState = "por_conciliar"
		case "captura":
			guaranteeState = "captura_por_conciliar"
		case "liberacion":
			guaranteeState = "liberacion_por_conciliar"
		}
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado=$2,autorizado_clp=$3,capturado_clp=$4,liberado_clp=$5,actualizada_en=$6 WHERE id=$1`, gid, guaranteeState, authorized, captured, released, now)
	if err != nil {
		return old, false, err
	}
	if kind == "autorizacion" && (lateAuthorization || state == "rechazada") && v.State == "pagada" {
		reason := "cancelación por resultado de garantía al inicio; ocupación liberada"
		if state == "rechazada" {
			reason = "cancelación por rechazo confirmado de garantía; ocupación liberada"
		}
		if err = cancelPaidReservationForGuarantee(ctx, tx, reservationID, actor, reason, now); err != nil {
			return old, false, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,$2,$3,$4,$5)`, reservationID, guaranteeHistoryType(kind, state), actor, last, now)
	if err != nil {
		return old, false, err
	}
	var result booking.GuaranteeOperation
	err = tx.QueryRow(ctx, `SELECT id::text,tipo,importe_clp,estado,COALESCE(ultimo_resultado,''),creada_en,actualizada_en FROM public.reserva_garantia_operacion_ensayo_local WHERE id=$1`, operationID).Scan(&result.ID, &result.Kind, &result.AmountCLP, &result.State, &result.LastResult, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return old, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, false, mapErr(err)
	}
	return result, false, nil
}

func (r *Repository) DecideGuarantee(ctx context.Context, admin, reservationID string, input booking.FinancialDecisionInput, key string, fingerprint []byte, clock func() time.Time) (booking.FinancialDecision, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.FinancialDecision{}, false, err
	}
	defer tx.Rollback(ctx)
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return booking.FinancialDecision{}, false, err
	}
	var gid string
	var authorized int64
	var state string
	err = tx.QueryRow(ctx, `SELECT g.id::text,g.autorizado_clp,g.estado FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE r.id=$1 FOR UPDATE OF r,g`, reservationID).Scan(&gid, &authorized, &state)
	if err != nil {
		return booking.FinancialDecision{}, false, mapErr(err)
	}
	var existing booking.FinancialDecision
	var oldHash []byte
	err = tx.QueryRow(ctx, `SELECT id::text,COALESCE(reclamo_id::text,''),resultado_reclamo,deduccion_clp,motivo_codigo,evidencia_id::text,estado,creada_en,actualizada_en,huella_solicitud FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&existing.ID, &existing.ClaimID, &existing.Outcome, &existing.DeductionCLP, &existing.ReasonCode, &existing.EvidenceID, &existing.State, &existing.CreatedAt, &existing.UpdatedAt, &oldHash)
	if err == nil {
		if !equalBytes(oldHash, fingerprint) {
			return existing, false, booking.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return existing, false, err
		}
		existing.Reused = true
		return existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return existing, false, err
	}
	if input.DeductionCLP < 0 || input.DeductionCLP > authorized || input.Outcome != "acogido" && input.Outcome != "rechazado" || input.Outcome == "rechazado" && input.DeductionCLP != 0 {
		return existing, false, booking.ErrInvalid
	}
	if input.DeductionCLP == 0 {
		input.ReasonCode = "sin_deduccion"
		input.EvidenceID = ""
	} else if input.ReasonCode != "dano_acreditado" && input.ReasonCode != "faltante_acreditado" || input.EvidenceID == "" {
		return existing, false, booking.ErrInvalid
	}
	if input.ClaimID != "" {
		var outcome string
		err = tx.QueryRow(ctx, `SELECT x.resultado
		FROM public.reclamo_dano_resolucion_ensayo_local x
		JOIN public.reclamo_dano_ensayo_local c ON c.id=x.reclamo_id
		WHERE x.reclamo_id=$1 AND c.reserva_id=$2`, input.ClaimID, reservationID).Scan(&outcome)
		if err != nil || outcome != input.Outcome {
			return existing, false, booking.ErrConflict
		}
	}
	if input.EvidenceID != "" {
		var ok bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM public.operacion_arriendo_evidencia_ensayo_local e
			JOIN public.operacion_arriendo_ensayo_local o ON o.id=e.operacion_id
			JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id
			WHERE e.id=$1 AND r.id=$2
		)`, input.EvidenceID, reservationID).Scan(&ok)
		if err != nil {
			return existing, false, err
		}
		if !ok {
			return existing, false, booking.ErrInvalid
		}
	}
	now := clock().UTC()
	id := ""
	err = tx.QueryRow(ctx, `INSERT INTO public.reserva_decision_financiera_ensayo_local(id,garantia_id,reserva_id,reclamo_id,resultado_reclamo,deduccion_clp,motivo_codigo,evidencia_id,administrador_id,clave_idempotencia,huella_solicitud,estado,creada_en,actualizada_en)
 VALUES(gen_random_uuid(),$1,$2,NULLIF($3,'')::uuid,$4,$5,$6,NULLIF($7,'')::uuid,$8,$9,$10,'pendiente',$11,$11) RETURNING id::text`, gid, reservationID, input.ClaimID, input.Outcome, input.DeductionCLP, input.ReasonCode, input.EvidenceID, admin, key, fingerprint, now).Scan(&id)
	if err != nil {
		return existing, false, mapErr(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,'decision_financiera_registrada',$2,$3,$4)`, reservationID, admin, input.ReasonCode, now)
	if err != nil {
		return existing, false, err
	}
	if input.DeductionCLP == 0 {
		_, err = tx.Exec(ctx, `UPDATE public.reserva_decision_financiera_ensayo_local SET estado='sin_deduccion' WHERE id=$1`, id)
		if err != nil {
			return existing, false, err
		}
	}
	_ = state
	result := booking.FinancialDecision{ID: id, ClaimID: input.ClaimID, Outcome: input.Outcome, DeductionCLP: input.DeductionCLP, ReasonCode: input.ReasonCode, State: "pendiente", CreatedAt: now, UpdatedAt: now}
	if input.EvidenceID != "" {
		result.EvidenceID = &input.EvidenceID
	}
	if input.DeductionCLP == 0 {
		result.State = "sin_deduccion"
	}
	if err = tx.Commit(ctx); err != nil {
		return result, false, mapErr(err)
	}
	return result, false, nil
}

func (r *Repository) ResolveGuaranteeOperation(ctx context.Context, admin, reservationID, kind, outcome string, clock func() time.Time) (booking.GuaranteeOperation, error) {
	if kind != "autorizacion" && kind != "captura" && kind != "liberacion" || outcome != "exito" && outcome != "rechazo" {
		return booking.GuaranteeOperation{}, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.GuaranteeOperation{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return booking.GuaranteeOperation{}, err
	}
	v, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID))
	if err != nil {
		return booking.GuaranteeOperation{}, err
	}
	var gid, gs string
	var authorized int64
	if err = tx.QueryRow(ctx, `SELECT id::text,estado,autorizado_clp FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&gid, &gs, &authorized); err != nil {
		return booking.GuaranteeOperation{}, mapErr(err)
	}
	var op booking.GuaranteeOperation
	var requested, key string
	var deadline *time.Time
	err = tx.QueryRow(ctx, `SELECT id::text,tipo,importe_clp,estado,COALESCE(ultimo_resultado,''),creada_en,actualizada_en,resultado_solicitado,clave_idempotencia,vence_en FROM public.reserva_garantia_operacion_ensayo_local WHERE garantia_id=$1 AND tipo=$2 FOR UPDATE`, gid, kind).Scan(&op.ID, &op.Kind, &op.AmountCLP, &op.State, &op.LastResult, &op.CreatedAt, &op.UpdatedAt, &requested, &key, &deadline)
	if err != nil {
		return booking.GuaranteeOperation{}, mapErr(err)
	}
	var priorResult bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_garantia_resultado_fake_ensayo_local WHERE operacion_id=$1 AND evento_id IS NOT NULL)`, op.ID).Scan(&priorResult); err != nil {
		return op, err
	}
	if op.State == "confirmada" || op.State == "rechazada" || op.State == "vencida" && priorResult {
		if err = tx.Commit(ctx); err != nil {
			return op, err
		}
		return op, nil
	}
	if op.State != "por_conciliar" && op.State != "pendiente" && !(op.State == "vencida" && !priorResult) {
		return op, booking.ErrConflict
	}
	now := clock().UTC()
	providerResult := "exito_simulado"
	if outcome == "rechazo" {
		providerResult = "rechazo_simulado"
	}
	eventID := op.ID + ":" + providerResult
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_resultado_fake_ensayo_local(operacion_id,evento_id,resultado,registrada_en) VALUES($1,$2,$3,$4) ON CONFLICT(operacion_id) DO UPDATE SET evento_id=EXCLUDED.evento_id,resultado=EXCLUDED.resultado,registrada_en=EXCLUDED.registrada_en`, op.ID, eventID, providerResult, now)
	if err != nil {
		return op, err
	}
	var eventRow string
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_evento_ensayo_local(operacion_id,evento_id,resultado,autenticado_en) VALUES($1,$2,$3,$4) ON CONFLICT(evento_id) DO NOTHING`, op.ID, eventID, providerResult, now)
	if err != nil {
		return op, err
	}
	err = tx.QueryRow(ctx, `SELECT id::text FROM public.reserva_garantia_evento_ensayo_local WHERE evento_id=$1`, eventID).Scan(&eventRow)
	if err != nil {
		return op, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_evento_aplicacion_ensayo_local(evento_id,estado,codigo_resultado,creada_en,procesada_en) VALUES($1,'pendiente',NULL,$2,NULL) ON CONFLICT(evento_id) DO NOTHING`, eventRow, now)
	if err != nil {
		return op, err
	}
	late := kind == "autorizacion" && (deadline != nil && !now.Before(*deadline) || !now.Before(v.StartAt) || v.State != "pagada")
	finalState, lastResult := "confirmada", "exito_simulado"
	if outcome == "rechazo" {
		finalState, lastResult = "rechazada", "rechazo_simulado"
	}
	if late {
		finalState, lastResult = "vencida", "resultado_tardio"
	}
	var captured, released int64
	if err = tx.QueryRow(ctx, `SELECT capturado_clp,liberado_clp FROM public.reserva_garantia_ensayo_local WHERE id=$1`, gid).Scan(&captured, &released); err != nil {
		return op, err
	}
	if outcome == "exito" && kind == "autorizacion" {
		authorized = 50000
		if late {
			gs = "liberacion_pendiente"
			// Persist the late authorization before creating its compensating
			// release intent. Both writes remain atomic in this transaction.
			if _, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado=$2,autorizado_clp=$3,actualizada_en=$4 WHERE id=$1`, gid, gs, authorized, now); err != nil {
				return op, err
			}
			// A late authorization is never allowed to revive the booking. Persist
			// its compensating release as a separate durable fake operation.
			fp, _ := json.Marshal([]any{"liberacion", int64(50000), "exito"})
			sum := sha256.Sum256(fp)
			_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,'liberacion',$2,$3,50000,'exito','pendiente',$4,NULL,NULL,$4,$4) ON CONFLICT(garantia_id,tipo) DO NOTHING`, gid, "libera-tardia:"+op.ID, sum[:], now)
			if err != nil {
				return op, err
			}
		}
	}
	if kind == "autorizacion" && (outcome == "rechazo" || late) && v.State == "pagada" {
		if _, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, reservationID, now); err != nil {
			return op, err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_por_pago',actualizada_en=$2 WHERE id=$1`, reservationID, now); err != nil {
			return op, err
		}
		if err = appendTransition(ctx, tx, reservationID, ptrString("pagada"), "cancelada_por_pago", admin, "cancelación por garantía; resultado fake no permite continuar", now); err != nil {
			return op, err
		}
		var paid int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(importe_clp),0) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'`, reservationID).Scan(&paid); err != nil {
			return op, err
		}
		if paid > 0 {
			if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_devolucion_ensayo(id,reserva_id,operacion_id,importe_clp,moneda,estado,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,gen_random_uuid(),$2,'CLP','pendiente',$3,$3) ON CONFLICT(reserva_id) DO NOTHING`, reservationID, paid, now); err != nil {
				return op, err
			}
		}
	}
	if kind == "autorizacion" && outcome == "rechazo" {
		gs = "no_disponible"
	}
	if kind == "autorizacion" && outcome == "exito" && !late {
		gs = "autorizada"
	}
	if kind == "captura" {
		if outcome == "exito" {
			captured += op.AmountCLP
			gs = "parcialmente_capturada"
			if _, err = tx.Exec(ctx, `UPDATE public.reserva_decision_financiera_ensayo_local SET estado='aplicada',actualizada_en=$2 WHERE reserva_id=$1 AND estado='pendiente'`, reservationID, now); err != nil {
				return op, err
			}
		} else {
			gs = "captura_por_conciliar"
		}
	}
	if kind == "liberacion" {
		if outcome == "exito" {
			released += op.AmountCLP
			if authorized-captured-released == 0 {
				gs = "liberada"
			} else {
				gs = "parcialmente_capturada"
			}
		} else {
			gs = "liberacion_por_conciliar"
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado=$2,autorizado_clp=$3,capturado_clp=$4,liberado_clp=$5,actualizada_en=$6 WHERE id=$1`, gid, gs, authorized, captured, released, now); err != nil {
		return op, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_operacion_ensayo_local SET estado=$2,ultimo_resultado=$3,completada_en=$4,actualizada_en=$4 WHERE id=$1`, op.ID, finalState, lastResult, now); err != nil {
		return op, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_evento_aplicacion_ensayo_local SET estado='aplicada',codigo_resultado=$2,procesada_en=$3 WHERE evento_id=$1`, eventRow, lastResult, now); err != nil {
		return op, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,$2,$3,$4,$5)`, reservationID, guaranteeHistoryType(kind, finalState), admin, lastResult, now); err != nil {
		return op, err
	}
	op.State, op.LastResult, op.UpdatedAt = finalState, lastResult, now
	if err = tx.Commit(ctx); err != nil {
		return op, mapErr(err)
	}
	return op, nil
}

func (r *Repository) ExpireDueGuarantees(ctx context.Context, clock func() time.Time) error {
	if clock == nil {
		clock = time.Now
	}
	scanAt := clock().UTC()
	rows, err := r.pool.Query(ctx, `SELECT reservation_id FROM (
SELECT DISTINCT g.reserva_id::text AS reservation_id FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_garantia_operacion_ensayo_local o ON o.garantia_id=g.id JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE o.tipo='autorizacion' AND o.estado IN ('pendiente','por_conciliar') AND o.ultimo_resultado IS DISTINCT FROM 'autorizacion_vencida' AND (o.vence_en<=$1 OR r.inicio<=$1)
UNION
SELECT DISTINCT g.reserva_id::text AS reservation_id FROM public.reserva_garantia_ensayo_local g JOIN public.operacion_arriendo_ensayo_local checkout ON checkout.reserva_id=g.reserva_id AND checkout.tipo='checkout' WHERE g.estado IN ('autorizada','parcialmente_capturada') AND checkout.ocurrio_en+interval '24 hours'<=$1 AND NOT EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=g.reserva_id)
) candidates ORDER BY reservation_id`, scanAt)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		tx, e := r.pool.Begin(ctx)
		if e != nil {
			return e
		}
		if e = lockReservationParties(ctx, tx, id); e != nil {
			_ = tx.Rollback(ctx)
			if errors.Is(e, booking.ErrNotFound) {
				continue
			}
			return e
		}
		v, e := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		var gid, gs, opID string
		var deadline *time.Time
		var auth, capture, release int64
		var opState string
		e = tx.QueryRow(ctx, `SELECT g.id::text,g.estado,g.autorizado_clp,g.capturado_clp,g.liberado_clp,o.id::text,o.estado,o.vence_en FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_garantia_operacion_ensayo_local o ON o.garantia_id=g.id AND o.tipo='autorizacion' WHERE g.reserva_id=$1 FOR UPDATE OF g,o`, id).Scan(&gid, &gs, &auth, &capture, &release, &opID, &opState, &deadline)
		if e != nil {
			_ = tx.Rollback(ctx)
			if errors.Is(e, pgx.ErrNoRows) {
				continue
			}
			return e
		}
		now := clock().UTC()
		due := deadline != nil && !now.Before(*deadline) || !now.Before(v.StartAt)
		if !due || opState != "pendiente" && opState != "por_conciliar" {
			_ = tx.Rollback(ctx)
			continue
		}
		if _, e = tx.Exec(ctx, `UPDATE public.reserva_garantia_operacion_ensayo_local SET estado='por_conciliar',ultimo_resultado='autorizacion_vencida',completada_en=NULL,actualizada_en=$2 WHERE id=$1 AND estado IN ('pendiente','por_conciliar')`, opID, now); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if v.State == "pagada" {
			if e = cancelPaidReservationForGuarantee(ctx, tx, id, nil, "garantía fake no confirmada dentro del plazo; ocupación liberada", now); e != nil {
				_ = tx.Rollback(ctx)
				return e
			}
		}
		if _, e = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado='por_conciliar',actualizada_en=$2 WHERE id=$1 AND estado IN ('pendiente_pago','pendiente_autorizacion','por_conciliar')`, gid, now); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if v.State == "pagada" {
			if _, e = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,detalle_codigo,ocurrida_en) VALUES($1,'garantia_vencida','autorizacion_no_confirmada', $2)`, id, now); e != nil {
				_ = tx.Rollback(ctx)
				return e
			}
		}
		if e = tx.Commit(ctx); e != nil {
			return mapErr(e)
		}
	}
	if err = r.releaseNoClaimGuarantees(ctx, scanAt, clock); err != nil {
		return err
	}
	return nil
}

func (r *Repository) releaseNoClaimGuarantees(ctx context.Context, scanAt time.Time, clock func() time.Time) error {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT g.reserva_id::text
FROM public.reserva_garantia_ensayo_local g
JOIN public.operacion_arriendo_ensayo_local checkout ON checkout.reserva_id=g.reserva_id AND checkout.tipo='checkout'
WHERE g.estado IN ('autorizada','parcialmente_capturada')
  AND checkout.ocurrio_en+interval '24 hours'<=$1
  AND NOT EXISTS (SELECT 1 FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=g.reserva_id)
ORDER BY 1`, scanAt)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		tx, e := r.pool.Begin(ctx)
		if e != nil {
			return e
		}
		if e = lockReservationParties(ctx, tx, id); e != nil {
			_ = tx.Rollback(ctx)
			if errors.Is(e, booking.ErrNotFound) {
				continue
			}
			return e
		}
		if _, e = tx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, id); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		var gid, state string
		var authorized, captured, released int64
		e = tx.QueryRow(ctx, `SELECT id::text,estado,autorizado_clp,capturado_clp,liberado_clp FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, id).
			Scan(&gid, &state, &authorized, &captured, &released)
		if e != nil {
			_ = tx.Rollback(ctx)
			if errors.Is(e, pgx.ErrNoRows) {
				continue
			}
			return e
		}
		now := clock().UTC()
		var checkoutAt time.Time
		e = tx.QueryRow(ctx, `SELECT ocurrio_en FROM public.operacion_arriendo_ensayo_local WHERE reserva_id=$1 AND tipo='checkout'`, id).Scan(&checkoutAt)
		if errors.Is(e, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			continue
		}
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		var claimExists bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local WHERE reserva_id=$1)`, id).Scan(&claimExists); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if now.Before(checkoutAt.Add(24*time.Hour)) || claimExists || state != "autorizada" && state != "parcialmente_capturada" {
			_ = tx.Rollback(ctx)
			continue
		}
		amount := authorized - captured - released
		if amount <= 0 {
			_ = tx.Rollback(ctx)
			continue
		}
		fp, _ := json.Marshal([]any{"liberacion", amount, "exito"})
		sum := sha256.Sum256(fp)
		var opID string
		e = tx.QueryRow(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
VALUES(gen_random_uuid(),$1,'liberacion',$2,$3,$4,'exito','confirmada',$5,'exito_simulado',$5,$5,$5)
ON CONFLICT(garantia_id,tipo) DO NOTHING RETURNING id::text`, gid, "checkout-24h:"+id, sum[:], amount, now).Scan(&opID)
		if errors.Is(e, pgx.ErrNoRows) {
			// A pending or uncertain release already owns the unique operation slot.
			// It remains visible for explicit reconciliation; never overwrite it.
			_ = tx.Rollback(ctx)
			continue
		}
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_resultado_fake_ensayo_local(operacion_id,evento_id,resultado,registrada_en) VALUES($1,$2,'exito_simulado',$3)`, opID, "checkout-24h:"+id, now); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		var eventID string
		if e = tx.QueryRow(ctx, `INSERT INTO public.reserva_garantia_evento_ensayo_local(operacion_id,evento_id,resultado,autenticado_en) VALUES($1,$2,'exito_simulado',$3) RETURNING id::text`, opID, "checkout-24h:"+id, now).Scan(&eventID); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_evento_aplicacion_ensayo_local(evento_id,estado,codigo_resultado,creada_en,procesada_en) VALUES($1,'aplicada','liberacion_checkout_24h',$2,$2)`, eventID, now); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET liberado_clp=liberado_clp+$2,estado='liberada',actualizada_en=$3 WHERE id=$1`, gid, amount, now); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,detalle_codigo,ocurrida_en) VALUES($1,'garantia_liberada','checkout_24h',$2)`, id, now); e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return mapErr(e)
		}
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
