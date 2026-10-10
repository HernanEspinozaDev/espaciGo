package bookingpg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

const settlementSelect = `SELECT s.id::text,s.reserva_id::text,COALESCE(s.anfitrion_id::text,''),s.politica_version,s.moneda,
 s.arriendo_confirmado_clp,s.devoluciones_confirmadas_clp,s.comision_bps,s.comision_clp,s.transferencia_clp,s.estado,COALESCE(s.ultimo_resultado,''),s.creada_en,s.actualizada_en,COALESCE(s.completada_en,'epoch'::timestamptz),o.id::text,o.resultado_solicitado
 FROM public.reserva_liquidacion_ensayo_local s JOIN public.reserva_liquidacion_operacion_ensayo_local o ON o.liquidacion_id=s.id`

func scanSettlement(row pgx.Row) (booking.Settlement, error) {
	var v booking.Settlement
	var completed time.Time
	err := row.Scan(&v.ID, &v.ReservationID, &v.HostID, &v.PolicyVersion, &v.Currency, &v.GrossRentCLP, &v.ConfirmedRefundsCLP, &v.CommissionBPS, &v.CommissionCLP, &v.TransferCLP, &v.State, &v.LastResult, &v.CreatedAt, &v.UpdatedAt, &completed, &v.OperationID, &v.RequestedOutcome)
	if err != nil {
		return booking.Settlement{}, mapErr(err)
	}
	return v, nil
}

func calculateLocalCommission(gross int64, basisPoints int) (int64, error) {
	if gross < 0 || basisPoints < 0 || basisPoints > 10000 {
		return 0, booking.ErrInvalid
	}
	amount := new(big.Int).Mul(big.NewInt(gross), big.NewInt(int64(basisPoints)))
	amount.Quo(amount, big.NewInt(10000))
	if !amount.IsInt64() {
		return 0, booking.ErrInvalid
	}
	return amount.Int64(), nil
}

func (r *Repository) BeginSettlement(ctx context.Context, host, reservationID, key string, fingerprint []byte, settlementID, operationID, outcome string, clock func() time.Time) (booking.Settlement, bool, error) {
	if len(fingerprint) != 32 || key == "" || len(key) > 200 || settlementID == "" || operationID == "" || (outcome != "exito" && outcome != "rechazo" && outcome != "sin_respuesta") {
		return booking.Settlement{}, false, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Settlement{}, false, err
	}
	defer tx.Rollback(ctx)
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return booking.Settlement{}, false, err
	}
	var state, storedHost, renter, snapshotPolicy string
	var snapshotBPS sql.NullInt32
	if err = tx.QueryRow(ctx, `SELECT estado,anfitrion_id::text,arrendatario_id::text,COALESCE(liquidacion_politica_version,''),liquidacion_comision_bps FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID).Scan(&state, &storedHost, &renter, &snapshotPolicy, &snapshotBPS); err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	if storedHost != host {
		return booking.Settlement{}, false, booking.ErrNotFound
	}
	var prior booking.Settlement
	prior, err = scanSettlement(tx.QueryRow(ctx, settlementSelect+` WHERE s.reserva_id=$1 FOR UPDATE OF s,o`, reservationID))
	if err == nil {
		var oldKey string
		var oldHash []byte
		if err = tx.QueryRow(ctx, `SELECT clave_idempotencia,huella_solicitud FROM public.reserva_liquidacion_ensayo_local WHERE id=$1`, prior.ID).Scan(&oldKey, &oldHash); err != nil {
			return booking.Settlement{}, false, err
		}
		if oldKey != key || !equalBytes(oldHash, fingerprint) {
			return booking.Settlement{}, false, booking.ErrConflict
		}
		prior.Reused = true
		if err = tx.Commit(ctx); err != nil {
			return booking.Settlement{}, false, err
		}
		return prior, true, nil
	}
	if !errors.Is(err, booking.ErrNotFound) {
		return booking.Settlement{}, false, err
	}
	if snapshotPolicy != booking.LocalSettlementPolicyVersion || !snapshotBPS.Valid {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	var guaranteeState string
	var authorized, captured, released int64
	var hasGuarantee bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1),COALESCE((SELECT estado FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1),''),COALESCE((SELECT autorizado_clp FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1),0),COALESCE((SELECT capturado_clp FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1),0),COALESCE((SELECT liberado_clp FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1),0)`, reservationID).Scan(&hasGuarantee, &guaranteeState, &authorized, &captured, &released)
	if err != nil {
		return booking.Settlement{}, false, err
	}
	if hasGuarantee {
		if _, err = tx.Exec(ctx, `SELECT id FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID); err != nil {
			return booking.Settlement{}, false, err
		}
	}
	var checkoutAt time.Time
	if err = tx.QueryRow(ctx, `SELECT ocurrio_en FROM public.operacion_arriendo_ensayo_local WHERE reserva_id=$1 AND tipo='checkout' FOR SHARE`, reservationID).Scan(&checkoutAt); err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	// The Backend clock is sampled only after the account, reservation and guarantee rows are locked.
	now := clock().UTC()
	if state != "finalizada" || now.Before(checkoutAt.Add(24*time.Hour)) {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	var openClaim, pending, unsettledDecision bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local WHERE reserva_id=$1 AND estado='abierto'),
 EXISTS(SELECT 1 FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1 AND estado='pendiente')
 OR EXISTS(SELECT 1 FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 AND estado IN ('pendiente','por_conciliar'))
 OR EXISTS(SELECT 1 FROM public.reserva_pago_evento_ensayo e JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id JOIN public.reserva_pago_ensayo_operacion o ON o.id=e.operacion_id WHERE o.reserva_id=$1 AND a.estado IN ('pendiente','pendiente_conciliacion'))
 OR EXISTS(SELECT 1 FROM public.reserva_devolucion_intento_ensayo i JOIN public.reserva_devolucion_ensayo d ON d.id=i.devolucion_id WHERE d.reserva_id=$1 AND i.resultado='sin_respuesta_simulada' AND d.estado='pendiente')
 OR EXISTS(SELECT 1 FROM public.reserva_garantia_operacion_ensayo_local o JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id WHERE g.reserva_id=$1 AND o.estado IN ('pendiente','por_conciliar'))
 OR EXISTS(SELECT 1 FROM public.reserva_garantia_evento_aplicacion_ensayo_local a JOIN public.reserva_garantia_evento_ensayo_local e ON e.id=a.evento_id JOIN public.reserva_garantia_operacion_ensayo_local o ON o.id=e.operacion_id JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id WHERE g.reserva_id=$1 AND a.estado IN ('pendiente','por_conciliar')),
 EXISTS(SELECT 1 FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1 AND estado IN ('pendiente','por_conciliar'))`, reservationID).Scan(&openClaim, &pending, &unsettledDecision); err != nil {
		return booking.Settlement{}, false, err
	}
	if openClaim || pending || unsettledDecision {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	if hasGuarantee && ((guaranteeState != "liberada" && guaranteeState != "parcialmente_capturada") || authorized != captured+released) {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	var gross, refunds int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT sum(importe_clp) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'),0),COALESCE((SELECT sum(importe_clp) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1 AND estado='completada'),0)`, reservationID).Scan(&gross, &refunds); err != nil {
		return booking.Settlement{}, false, err
	}
	if gross <= 0 || refunds > gross {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	bps := int(snapshotBPS.Int32)
	commission, err := calculateLocalCommission(gross-refunds, bps)
	if err != nil {
		return booking.Settlement{}, false, err
	}
	transfer := gross - refunds - commission
	if transfer < 0 {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_ensayo_local(id,reserva_id,anfitrion_id,politica_version,moneda,arriendo_confirmado_clp,devoluciones_confirmadas_clp,comision_bps,comision_clp,transferencia_clp,estado,clave_idempotencia,huella_solicitud,creada_en,actualizada_en)
VALUES($1,$2,$3,$4,'CLP',$5,$6,$7,$8,$9,'pendiente',$10,$11,$12,$12)`, settlementID, reservationID, host, snapshotPolicy, gross, refunds, bps, commission, transfer, key, fingerprint, now)
	if err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_operacion_ensayo_local(id,liquidacion_id,resultado_solicitado,creada_en,actualizada_en) VALUES($1,$2,$3,$4,$4)`, operationID, settlementID, outcome, now)
	if err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_historial_ensayo_local(liquidacion_id,accion,actor_id,codigo,ocurrida_en) VALUES($1,'liquidacion_solicitada',$2,'transferencia_fake_persistida',$3)`, settlementID, host, now); err != nil {
		return booking.Settlement{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,'liquidacion_solicitada',$2,'transferencia_fake_persistida',$3)`, reservationID, host, now); err != nil {
		return booking.Settlement{}, false, err
	}
	if err = appendFinancialAdminAudit(ctx, tx, host, reservationID, "local.finance.settlement", "liquidacion_local_solicitada", key, now); err != nil {
		return booking.Settlement{}, false, err
	}
	value, err := scanSettlement(tx.QueryRow(ctx, settlementSelect+` WHERE s.id=$1`, settlementID))
	if err != nil {
		return booking.Settlement{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.Settlement{}, false, err
	}
	return value, false, nil
}

func (r *Repository) MarkSettlementUncertain(ctx context.Context, actor, reservationID string, clock func() time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return err
	}
	var host string
	if err = tx.QueryRow(ctx, `SELECT anfitrion_id::text FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID).Scan(&host); err != nil {
		return mapErr(err)
	}
	var sid, state string
	if err = tx.QueryRow(ctx, `SELECT id::text,estado FROM public.reserva_liquidacion_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&sid, &state); err != nil {
		return mapErr(err)
	}
	if state != "pendiente" {
		return tx.Commit(ctx)
	}
	now := clock().UTC()
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_liquidacion_ensayo_local SET estado='por_conciliar',ultimo_resultado='sin_respuesta_simulada',actualizada_en=$2 WHERE id=$1`, sid, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_historial_ensayo_local(liquidacion_id,accion,actor_id,codigo,ocurrida_en) VALUES($1,'liquidacion_por_conciliar',$2,'resultado_fake_incierto',$3)`, sid, actor, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,'operacion_pendiente_conciliacion',$2,'liquidacion_fake_incierta',$3)`, reservationID, actor, now); err != nil {
		return err
	}
	if err = appendFinancialAdminAudit(ctx, tx, actor, reservationID, "local.finance.settlement", "liquidacion_local_por_conciliar", sid, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ApplySettlementResult(ctx context.Context, actor string, event booking.SettlementEvent, fingerprint []byte, clock func() time.Time) (booking.Settlement, bool, error) {
	if len(fingerprint) != 32 || event.EventID == "" || event.OperationID == "" || (event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return booking.Settlement{}, false, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Settlement{}, false, err
	}
	defer tx.Rollback(ctx)
	var reservationID string
	if err = tx.QueryRow(ctx, `SELECT s.reserva_id::text FROM public.reserva_liquidacion_operacion_ensayo_local o JOIN public.reserva_liquidacion_ensayo_local s ON s.id=o.liquidacion_id WHERE o.id=$1`, event.OperationID).Scan(&reservationID); err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return booking.Settlement{}, false, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID); err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	value, err := scanSettlement(tx.QueryRow(ctx, settlementSelect+` WHERE o.id=$1 FOR UPDATE OF s,o`, event.OperationID))
	if err != nil {
		return booking.Settlement{}, false, err
	}
	var recordedID, recordedResult string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(event_id,''),resultado FROM public.reserva_liquidacion_resultado_fake_ensayo_local WHERE operacion_id=$1`, event.OperationID).Scan(&recordedID, &recordedResult); err != nil {
		return booking.Settlement{}, false, mapErr(err)
	}
	if recordedID != event.EventID || recordedResult != event.Outcome {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	if value.State == "liquidada" || value.State == "rechazada" {
		if value.LastResult != event.Outcome {
			return booking.Settlement{}, false, booking.ErrConflict
		}
		value.Reused = true
		if err = tx.Commit(ctx); err != nil {
			return booking.Settlement{}, false, err
		}
		return value, true, nil
	}
	if value.RequestedOutcome == "sin_respuesta" && value.LastResult == "" { /* admin reconciliation supplies the terminal outcome */
	} else if value.RequestedOutcome == "exito" && event.Outcome != "exito_simulado" || value.RequestedOutcome == "rechazo" && event.Outcome != "rechazo_simulado" {
		return booking.Settlement{}, false, booking.ErrConflict
	}
	now := clock().UTC()
	finalState, action, ledger, reason := "rechazada", "liquidacion_rechazada", "liquidacion_rechazada", "liquidacion_fake_rechazada"
	if event.Outcome == "exito_simulado" {
		finalState, action, ledger, reason = "liquidada", "liquidacion_confirmada", "liquidacion_confirmada", "liquidacion_fake_confirmada"
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_liquidacion_ensayo_local SET estado=$2,ultimo_resultado=$3,actualizada_en=$4,completada_en=$4 WHERE id=$1`, value.ID, finalState, event.Outcome, now)
	if err != nil {
		return booking.Settlement{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_historial_ensayo_local(liquidacion_id,accion,actor_id,codigo,ocurrida_en) VALUES($1,$2,$3,$4,$5)`, value.ID, action, actor, reason, now); err != nil {
		return booking.Settlement{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,$2,$3,$4,$5)`, reservationID, ledger, actor, reason, now); err != nil {
		return booking.Settlement{}, false, err
	}
	if event.Outcome == "exito_simulado" {
		receipt := booking.LocalReceipt{Version: 1, Kind: "comprobante_ensayo", Banner: booking.LocalSettlementBanner, ReceiptID: "", ReservationID: reservationID, PolicyVersion: value.PolicyVersion, Currency: value.Currency, GrossRentCLP: value.GrossRentCLP, RefundsCLP: value.ConfirmedRefundsCLP, CommissionCLP: value.CommissionCLP, TransferCLP: value.TransferCLP, ConfirmedAt: now}
		var receiptID string
		if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&receiptID); err != nil {
			return booking.Settlement{}, false, err
		}
		receipt.ReceiptID = receiptID
		content, marshalErr := json.Marshal(receipt)
		if marshalErr != nil {
			return booking.Settlement{}, false, marshalErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_comprobante_ensayo_local(id,liquidacion_id,version,contenido,emitido_en) VALUES($1,$2,1,$3::jsonb,$4)`, receiptID, value.ID, string(content), now); err != nil {
			return booking.Settlement{}, false, err
		}
	}
	if actor != "" {
		if err = appendFinancialAdminAudit(ctx, tx, actor, reservationID, "local.finance.settlement", reason, event.EventID, now); err != nil {
			return booking.Settlement{}, false, err
		}
	}
	value, err = scanSettlement(tx.QueryRow(ctx, settlementSelect+` WHERE s.id=$1`, value.ID))
	if err != nil {
		return booking.Settlement{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.Settlement{}, false, err
	}
	return value, false, nil
}

func (r *Repository) Settlement(ctx context.Context, actor, reservationID string) (booking.Settlement, error) {
	return scanSettlement(r.pool.QueryRow(ctx, settlementSelect+` JOIN public.reserva_ensayo_local r ON r.id=s.reserva_id WHERE s.reserva_id=$1 AND (r.anfitrion_id=$2 OR r.arrendatario_id=$2)`, reservationID, actor))
}
func (r *Repository) SettlementForAdministrator(ctx context.Context, reservationID string) (booking.Settlement, error) {
	return scanSettlement(r.pool.QueryRow(ctx, settlementSelect+` WHERE s.reserva_id=$1`, reservationID))
}
func (r *Repository) Receipt(ctx context.Context, actor, reservationID string) (booking.LocalReceipt, error) {
	var receipt booking.LocalReceipt
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT c.contenido FROM public.reserva_comprobante_ensayo_local c JOIN public.reserva_liquidacion_ensayo_local s ON s.id=c.liquidacion_id JOIN public.reserva_ensayo_local r ON r.id=s.reserva_id WHERE r.id=$1 AND (r.anfitrion_id=$2 OR r.arrendatario_id=$2)`, reservationID, actor).Scan(&raw)
	if err != nil {
		return booking.LocalReceipt{}, mapErr(err)
	}
	if err = json.Unmarshal(raw, &receipt); err != nil {
		return booking.LocalReceipt{}, err
	}
	return receipt, nil
}
func (r *Repository) PendingSettlements(ctx context.Context, limit int) ([]booking.Settlement, error) {
	if limit < 1 || limit > 500 {
		return nil, booking.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, settlementSelect+` WHERE s.estado IN ('pendiente','por_conciliar') ORDER BY s.creada_en,s.id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []booking.Settlement{}
	for rows.Next() {
		var v booking.Settlement
		if err = rows.Scan(&v.ID, &v.ReservationID, &v.HostID, &v.PolicyVersion, &v.Currency, &v.GrossRentCLP, &v.ConfirmedRefundsCLP, &v.CommissionBPS, &v.CommissionCLP, &v.TransferCLP, &v.State, &v.LastResult, &v.CreatedAt, &v.UpdatedAt, new(time.Time), &v.OperationID, &v.RequestedOutcome); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// The fake result store is separate from application of the result, so a
// process restart between those transactions is recoverable without replaying
// a second transfer.
func (r *Repository) SaveFakeSettlementResult(ctx context.Context, event booking.SettlementEvent, at time.Time) (booking.SettlementEvent, bool, error) {
	if event.OperationID == "" || event.EventID == "" || (event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return booking.SettlementEvent{}, false, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.SettlementEvent{}, false, err
	}
	defer tx.Rollback(ctx)
	var reservationID, requested string
	if err = tx.QueryRow(ctx, `SELECT s.reserva_id::text,o.resultado_solicitado FROM public.reserva_liquidacion_operacion_ensayo_local o JOIN public.reserva_liquidacion_ensayo_local s ON s.id=o.liquidacion_id WHERE o.id=$1`, event.OperationID).Scan(&reservationID, &requested); err != nil {
		return booking.SettlementEvent{}, false, mapErr(err)
	}
	if err = lockReservationParties(ctx, tx, reservationID); err != nil {
		return booking.SettlementEvent{}, false, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID); err != nil {
		return booking.SettlementEvent{}, false, mapErr(err)
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_liquidacion_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&state); err != nil {
		return booking.SettlementEvent{}, false, mapErr(err)
	}
	if requested == "exito" && event.Outcome != "exito_simulado" || requested == "rechazo" && event.Outcome != "rechazo_simulado" {
		return booking.SettlementEvent{}, false, booking.ErrConflict
	}
	cmd, err := tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_resultado_fake_ensayo_local(operacion_id,event_id,resultado,registrada_en) VALUES($1,$2,$3,$4) ON CONFLICT(operacion_id) DO NOTHING`, event.OperationID, event.EventID, event.Outcome, at.UTC())
	if err != nil {
		return booking.SettlementEvent{}, false, mapErr(err)
	}
	var oldID, oldResult string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(event_id,''),resultado FROM public.reserva_liquidacion_resultado_fake_ensayo_local WHERE operacion_id=$1`, event.OperationID).Scan(&oldID, &oldResult); err != nil {
		return booking.SettlementEvent{}, false, err
	}
	if oldID != event.EventID || oldResult != event.Outcome {
		return booking.SettlementEvent{}, false, booking.ErrConflict
	}
	if state == "liquidada" || state == "rechazada" {
		// A concurrent request may have persisted the same fake result and
		// applied it before this worker acquired the reservation lock. Reuse
		// the durable event in that case; mismatched events were rejected above.
		if err = tx.Commit(ctx); err != nil {
			return booking.SettlementEvent{}, false, err
		}
		return booking.SettlementEvent{EventID: oldID, OperationID: event.OperationID, Outcome: oldResult}, false, nil
	}
	if state != "pendiente" && state != "por_conciliar" {
		return booking.SettlementEvent{}, false, booking.ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.SettlementEvent{}, false, err
	}
	return event, cmd.RowsAffected() == 1, nil
}
func (r *Repository) RecordFakeSettlementTimeout(ctx context.Context, operationID string, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var rid, requested string
	if err = tx.QueryRow(ctx, `SELECT s.reserva_id::text,o.resultado_solicitado FROM public.reserva_liquidacion_operacion_ensayo_local o JOIN public.reserva_liquidacion_ensayo_local s ON s.id=o.liquidacion_id WHERE o.id=$1`, operationID).Scan(&rid, &requested); err != nil {
		return mapErr(err)
	}
	if err = lockReservationParties(ctx, tx, rid); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, rid); err != nil {
		return mapErr(err)
	}
	if requested != "sin_respuesta" {
		return booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_liquidacion_resultado_fake_ensayo_local(operacion_id,resultado,registrada_en) VALUES($1,'sin_respuesta_simulada',$2) ON CONFLICT(operacion_id) DO NOTHING`, operationID, at.UTC())
	if err != nil {
		return mapErr(err)
	}
	var result string
	if err = tx.QueryRow(ctx, `SELECT resultado FROM public.reserva_liquidacion_resultado_fake_ensayo_local WHERE operacion_id=$1`, operationID).Scan(&result); err != nil {
		return err
	}
	if result != "sin_respuesta_simulada" {
		return booking.ErrConflict
	}
	return tx.Commit(ctx)
}
func (r *Repository) FindFakeSettlementResult(ctx context.Context, operationID string) (*booking.SettlementEvent, error) {
	var id, outcome string
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(event_id,''),resultado FROM public.reserva_liquidacion_resultado_fake_ensayo_local WHERE operacion_id=$1`, operationID).Scan(&id, &outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if outcome == "sin_respuesta_simulada" {
		return nil, booking.ErrSimulatedNoResponse
	}
	if id == "" {
		return nil, booking.ErrConflict
	}
	return &booking.SettlementEvent{EventID: id, OperationID: operationID, Outcome: outcome}, nil
}
func (r *Repository) ResolveFakeSettlementResult(ctx context.Context, operationID, outcome string, at time.Time) (booking.SettlementEvent, error) {
	if outcome != "exito_simulado" && outcome != "rechazo_simulado" {
		return booking.SettlementEvent{}, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.SettlementEvent{}, err
	}
	defer tx.Rollback(ctx)
	var rid string
	if err = tx.QueryRow(ctx, `SELECT s.reserva_id::text FROM public.reserva_liquidacion_operacion_ensayo_local o JOIN public.reserva_liquidacion_ensayo_local s ON s.id=o.liquidacion_id WHERE o.id=$1`, operationID).Scan(&rid); err != nil {
		return booking.SettlementEvent{}, mapErr(err)
	}
	if err = lockReservationParties(ctx, tx, rid); err != nil {
		return booking.SettlementEvent{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, rid); err != nil {
		return booking.SettlementEvent{}, mapErr(err)
	}
	var result string
	if err = tx.QueryRow(ctx, `SELECT resultado FROM public.reserva_liquidacion_resultado_fake_ensayo_local WHERE operacion_id=$1 FOR UPDATE`, operationID).Scan(&result); err != nil {
		return booking.SettlementEvent{}, mapErr(err)
	}
	eventID := "localfake:settlement:" + operationID + ":" + outcome
	if result == "sin_respuesta_simulada" {
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_liquidacion_resultado_fake_ensayo_local SET event_id=$2,resultado=$3,registrada_en=$4 WHERE operacion_id=$1`, operationID, eventID, outcome, at.UTC()); err != nil {
			return booking.SettlementEvent{}, err
		}
	} else {
		var existing string
		if err = tx.QueryRow(ctx, `SELECT COALESCE(event_id,'') FROM public.reserva_liquidacion_resultado_fake_ensayo_local WHERE operacion_id=$1`, operationID).Scan(&existing); err != nil {
			return booking.SettlementEvent{}, err
		}
		if result != outcome || existing != eventID {
			return booking.SettlementEvent{}, booking.ErrConflict
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.SettlementEvent{}, err
	}
	return booking.SettlementEvent{EventID: eventID, OperationID: operationID, Outcome: outcome}, nil
}
