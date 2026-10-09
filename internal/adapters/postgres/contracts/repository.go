package contractspg

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/documents"
	"github.com/HernanEspinozaDev/espaciGo/internal/contract"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	key  []byte
}

func New(pool *pgxpool.Pool, key []byte) (*Repository, error) {
	if pool == nil || len(key) != 32 {
		return nil, contract.ErrInvalid
	}
	return &Repository{pool: pool, key: append([]byte(nil), key...)}, nil
}

// lockReservationAccounts establishes the shared lock order used by M06
// mutations and privacy execution: participant accounts, reservation, then
// contract/signatures. Expiry workers also take participant locks so they do
// not invert order against a concurrent cancellation.
func lockReservationAccounts(ctx context.Context, tx pgx.Tx, reservationID string) (string, string, error) {
	var host, renter string
	if err := tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&host, &renter); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", contract.ErrNotFound
		}
		return "", "", err
	}
	ids := []string{host, renter}
	sort.Strings(ids)
	for i, id := range ids {
		if i > 0 && id == ids[i-1] {
			continue
		}
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
			return "", "", err
		}
	}
	return host, renter, nil
}

// LockReservationAccounts exposes the shared account → reservation lock order
// to other local adapters that mutate reservation-owned records.
func LockReservationAccounts(ctx context.Context, tx pgx.Tx, reservationID string) (string, string, error) {
	return lockReservationAccounts(ctx, tx, reservationID)
}

const columns = `id::text,reserva_id::text,documento_id::text,version,estado,snapshot::text,encode(sha256,'hex'),creada_en,actualizada_en`

func (r *Repository) Create(ctx context.Context, actor, reservationID string, clock func() time.Time) (contract.Contract, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return contract.Contract{}, err
	}
	defer tx.Rollback(ctx)
	if _, _, err = lockReservationAccounts(ctx, tx, reservationID); err != nil {
		return contract.Contract{}, err
	}
	var state, host, renter string
	var start, end time.Time
	var subtotal int64
	var title, notice string
	var conditions, policy, unit, currency, zone string
	err = tx.QueryRow(ctx, `SELECT r.estado,r.anfitrion_id::text,r.arrendatario_id::text,r.inicio,r.termino,r.subtotal_clp,e.titulo,r.condiciones_snapshot,r.politica_cancelacion_version,r.modalidad,r.moneda,r.zona_horaria FROM public.reserva_ensayo_local r JOIN public.espacio e ON e.id=r.espacio_id WHERE r.id=$1 FOR UPDATE OF r`, reservationID).Scan(&state, &host, &renter, &start, &end, &subtotal, &title, &conditions, &policy, &unit, &currency, &zone)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.Contract{}, contract.ErrNotFound
	}
	if err != nil {
		return contract.Contract{}, err
	}
	if actor != host && actor != renter {
		return contract.Contract{}, contract.ErrNotFound
	}
	if prior, e := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM public.contrato_ensayo_local WHERE reserva_id=$1`, reservationID)); e == nil {
		if err = tx.Commit(ctx); err != nil {
			return contract.Contract{}, err
		}
		return load(ctx, r.pool, prior.ID, r.key)
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return contract.Contract{}, e
	}
	if state != "aprobada_host" {
		return contract.Contract{}, contract.ErrConflict
	}
	now := clock().UTC()
	if !now.Before(start) {
		return contract.Contract{}, contract.ErrConflict
	}
	notice = contract.SafetyNotice
	snapshot, _ := json.Marshal(map[string]any{"reservation_id": reservationID, "host_id": host, "renter_id": renter, "space_title": title, "start_at": start.UTC(), "end_at": end.UTC(), "subtotal_clp": subtotal, "currency": currency, "rate_unit": unit, "time_zone": zone, "conditions_snapshot": conditions, "cancellation_policy_version": policy, "notice": notice})
	pdf := syntheticPDF("ENSAYO SINTETICO LOCAL - SIN VALIDEZ JURIDICA", "Reserva: "+reservationID, "Anfitrion (cuenta sintetica): "+host, "Arrendatario (cuenta sintetica): "+renter, "Espacio: "+title, "Inicio UTC: "+start.UTC().Format(time.RFC3339), "Termino UTC: "+end.UTC().Format(time.RFC3339), "Zona IANA: "+zone, fmt.Sprintf("Total snapshot: %d %s (%s)", subtotal, currency, unit), "Condiciones snapshot: "+conditions, "Politica de cancelacion: "+policy)
	sum := sha256.Sum256(pdf)
	encryptedPDF, err := encryptPDF(r.key, pdf)
	if err != nil {
		return contract.Contract{}, err
	}
	documentID, err := documents.InsertPDF(ctx, tx, reservationID, encryptedPDF, now)
	if err != nil {
		return contract.Contract{}, err
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO public.contrato_ensayo_local(id,reserva_id,documento_id,version,estado,snapshot,sha256,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,$2,1,'generado',$3,$4,$5,$5) RETURNING id::text`, reservationID, documentID, snapshot, sum[:], now).Scan(&id)
	if err != nil {
		return contract.Contract{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.contrato_ensayo_firma(contrato_id,firmante_id,rol,estado,actualizada_en) VALUES($1,$2,'anfitrion','pendiente',$3),($1,$4,'arrendatario','pendiente',$3)`, id, host, now, renter)
	if err != nil {
		return contract.Contract{}, err
	}
	if err = event(ctx, tx, id, actor, "generado", "", now); err != nil {
		return contract.Contract{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return contract.Contract{}, err
	}
	return load(ctx, r.pool, id, r.key)
}

func (r *Repository) Get(ctx context.Context, actor, id string) (contract.Contract, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.contrato_ensayo_local c JOIN public.reserva_ensayo_local r ON r.id=c.reserva_id WHERE c.id=$1 AND ($2=r.anfitrion_id OR $2=r.arrendatario_id))`, id, actor).Scan(&allowed)
	if err != nil {
		return contract.Contract{}, err
	}
	if !allowed {
		return contract.Contract{}, contract.ErrNotFound
	}
	return load(ctx, r.pool, id, r.key)
}
func (r *Repository) Sign(ctx context.Context, actor, id string, clock func() time.Time) (contract.Contract, error) {
	return r.transition(ctx, actor, id, "firmar", "", clock)
}
func (r *Repository) Reject(ctx context.Context, actor, id, reason string, clock func() time.Time) (contract.Contract, error) {
	return r.transition(ctx, actor, id, "rechazar", reason, clock)
}

func (r *Repository) transition(ctx context.Context, actor, id, action, reason string, clock func() time.Time) (contract.Contract, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return contract.Contract{}, err
	}
	defer tx.Rollback(ctx)
	var reservation, contractState, reservationState, host, renter string
	var start time.Time
	err = tx.QueryRow(ctx, `SELECT reserva_id::text FROM public.contrato_ensayo_local WHERE id=$1`, id).Scan(&reservation)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.Contract{}, contract.ErrNotFound
	}
	if err != nil {
		return contract.Contract{}, err
	}
	if _, _, err = lockReservationAccounts(ctx, tx, reservation); err != nil {
		return contract.Contract{}, err
	}
	// Every M06/M07 operation locks the reservation before the contract. Avoid
	// a join-level FOR UPDATE whose row acquisition order could invert against
	// cancellation (reservation then contract).
	err = tx.QueryRow(ctx, `SELECT estado,anfitrion_id::text,arrendatario_id::text,inicio FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&reservationState, &host, &renter, &start)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.Contract{}, contract.ErrNotFound
	}
	if err != nil {
		return contract.Contract{}, err
	}
	err = tx.QueryRow(ctx, `SELECT estado FROM public.contrato_ensayo_local WHERE id=$1 AND reserva_id=$2 FOR UPDATE`, id, reservation).Scan(&contractState)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.Contract{}, contract.ErrNotFound
	}
	if err != nil {
		return contract.Contract{}, err
	}
	if actor != host && actor != renter {
		return contract.Contract{}, contract.ErrNotFound
	}
	// Consult Backend time only after the shared reservation lock is held.
	now := clock().UTC()
	var signed int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.contrato_ensayo_firma WHERE contrato_id=$1 AND estado='firmada'`, id).Scan(&signed); err != nil {
		return contract.Contract{}, err
	}
	if contractState == "firmado" {
		_ = tx.Commit(ctx)
		return load(ctx, r.pool, id, r.key)
	}
	if contractState == "anulado" || strings.HasPrefix(reservationState, "cancelada_") {
		return contract.Contract{}, contract.ErrConflict
	}
	if !now.Before(start) {
		if signed < 2 {
			if err = expireLocked(ctx, tx, id, reservation, actor, now); err != nil {
				return contract.Contract{}, err
			}
			if err = tx.Commit(ctx); err != nil {
				return contract.Contract{}, err
			}
			return contract.Contract{}, contract.ErrConflict
		}
	}
	var signatureState string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.contrato_ensayo_firma WHERE contrato_id=$1 AND firmante_id=$2 FOR UPDATE`, id, actor).Scan(&signatureState)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.Contract{}, contract.ErrNotFound
	}
	if err != nil {
		return contract.Contract{}, err
	}
	if action == "firmar" && signatureState == "firmada" {
		if err = tx.Commit(ctx); err != nil {
			return contract.Contract{}, err
		}
		return load(ctx, r.pool, id, r.key)
	}
	if signatureState != "pendiente" {
		return contract.Contract{}, contract.ErrConflict
	}
	newSignature := "firmada"
	eventAction := "firmado"
	if action == "rechazar" {
		newSignature = "rechazada"
		eventAction = "rechazado"
	}
	if _, err = tx.Exec(ctx, `UPDATE public.contrato_ensayo_firma SET estado=$3,motivo=$4,actualizada_en=$5 WHERE contrato_id=$1 AND firmante_id=$2`, id, actor, newSignature, reason, now); err != nil {
		return contract.Contract{}, err
	}
	if err = event(ctx, tx, id, actor, eventAction, reason, now); err != nil {
		return contract.Contract{}, err
	}
	var completed int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.contrato_ensayo_firma WHERE contrato_id=$1 AND estado='firmada'`, id).Scan(&completed); err != nil {
		return contract.Contract{}, err
	}
	if completed > 0 || action == "rechazar" {
		if _, err = tx.Exec(ctx, `UPDATE public.contrato_ensayo_local SET estado=CASE WHEN $2=2 THEN 'firmado' ELSE 'firma_parcial' END,actualizada_en=$3::timestamptz,firmado_en=CASE WHEN $2=2 THEN $3::timestamptz ELSE NULL::timestamptz END WHERE id=$1`, id, completed, now); err != nil {
			return contract.Contract{}, err
		}
		nextBooking := "firma_parcial"
		if completed == 2 {
			nextBooking = "lista_para_checkin"
			if _, err = tx.Exec(ctx, `UPDATE public.ocupacion SET expira_en=NULL WHERE reserva_id=$1 AND activo`, reservation); err != nil {
				return contract.Contract{}, err
			}
			if err = event(ctx, tx, id, actor, "firmado_completo", "", now); err != nil {
				return contract.Contract{}, err
			}
		}
		if reservationState != "firma_parcial" || completed == 2 {
			if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1`, reservation, nextBooking, now); err != nil {
				return contract.Contract{}, err
			}
			if err = bookingEvent(ctx, tx, reservation, reservationState, nextBooking, actor, "contrato sintético firmado", now); err != nil {
				return contract.Contract{}, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return contract.Contract{}, err
	}
	return load(ctx, r.pool, id, r.key)
}

func (r *Repository) ExpireDue(ctx context.Context, clock func() time.Time) (int, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.id::text,c.reserva_id::text FROM public.contrato_ensayo_local c WHERE c.estado IN ('generado','firma_parcial') ORDER BY c.creada_en`)
	if err != nil {
		return 0, err
	}
	type pair struct{ c, r string }
	items := []pair{}
	for rows.Next() {
		var p pair
		if err = rows.Scan(&p.c, &p.r); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, p)
	}
	rows.Close()
	n := 0
	for _, p := range items {
		tx, e := r.pool.Begin(ctx)
		if e != nil {
			return n, e
		}
		if _, _, e = lockReservationAccounts(ctx, tx, p.r); e != nil {
			_ = tx.Rollback(ctx)
			return n, e
		}
		var state string
		var start time.Time
		e = tx.QueryRow(ctx, `SELECT estado,inicio FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, p.r).Scan(&state, &start)
		if e == nil {
			now := clock().UTC()
			var contractState string
			e = tx.QueryRow(ctx, `SELECT estado FROM public.contrato_ensayo_local WHERE id=$1`, p.c).Scan(&contractState)
			if e == nil && contractState != "firmado" && contractState != "anulado" {
				if state != "aprobada_host" && state != "firma_parcial" {
					_, e = tx.Exec(ctx, `UPDATE public.contrato_ensayo_local SET estado='anulado',actualizada_en=$2 WHERE id=$1`, p.c, now)
					if e == nil {
						e = event(ctx, tx, p.c, "", "reserva_terminal", "reserva terminó antes de completar el contrato", now)
					}
				} else if !now.Before(start) {
					var signed int
					e = tx.QueryRow(ctx, `SELECT count(*) FROM public.contrato_ensayo_firma WHERE contrato_id=$1 AND estado='firmada'`, p.c).Scan(&signed)
					if e == nil && signed < 2 {
						e = expireLocked(ctx, tx, p.c, p.r, "", now)
						if e == nil {
							n++
						}
					}
				}
			}
		}
		if e != nil {
			tx.Rollback(ctx)
			return n, e
		}
		if e = tx.Commit(ctx); e != nil {
			return n, e
		}
	}
	// Approval and contract generation are separate owner operations. If the
	// API was unavailable through start_at, M06 still expires the approved
	// reservation once, under its reservation lock.
	rows, err = r.pool.Query(ctx, `SELECT r.id::text FROM public.reserva_ensayo_local r WHERE r.estado='aprobada_host' AND NOT EXISTS(SELECT 1 FROM public.contrato_ensayo_local c WHERE c.reserva_id=r.id)`)
	if err != nil {
		return n, err
	}
	missing := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return n, err
		}
		missing = append(missing, id)
	}
	rows.Close()
	for _, rid := range missing {
		tx, e := r.pool.Begin(ctx)
		if e != nil {
			return n, e
		}
		if _, _, e = lockReservationAccounts(ctx, tx, rid); e != nil {
			_ = tx.Rollback(ctx)
			return n, e
		}
		var state string
		var start time.Time
		e = tx.QueryRow(ctx, `SELECT estado,inicio FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, rid).Scan(&state, &start)
		if e == nil {
			now := clock().UTC()
			if state == "aprobada_host" && !now.Before(start) {
				e = expireApprovedWithoutContract(ctx, tx, rid, now)
				if e == nil {
					n++
				}
			}
		}
		if e != nil {
			_ = tx.Rollback(ctx)
			return n, e
		}
		if e = tx.Commit(ctx); e != nil {
			return n, e
		}
	}
	return n, nil
}

func expireApprovedWithoutContract(ctx context.Context, tx pgx.Tx, rid string, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, rid, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_por_firma',actualizada_en=$2 WHERE id=$1`, rid, now); err != nil {
		return err
	}
	if err := bookingEvent(ctx, tx, rid, "aprobada_host", "cancelada_por_firma", "", "vencimiento sin firmas antes del inicio", now); err != nil {
		return err
	}
	var amount int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(importe_clp),0) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'`, rid).Scan(&amount); err != nil {
		return err
	}
	if amount > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO public.reserva_devolucion_ensayo(id,reserva_id,operacion_id,importe_clp,moneda,estado,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,gen_random_uuid(),$2,'CLP','pendiente',$3,$3) ON CONFLICT(reserva_id) DO NOTHING`, rid, amount, now); err != nil {
			return err
		}
	}
	return nil
}

// EnsureApproved recovers contracts whose M06 host approval committed before
// the process stopped. Create reuses the reservation row lock and unique key.
func (r *Repository) EnsureApproved(ctx context.Context, clock func() time.Time) (int, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,anfitrion_id::text FROM public.reserva_ensayo_local r WHERE estado='aprobada_host' AND NOT EXISTS(SELECT 1 FROM public.contrato_ensayo_local c WHERE c.reserva_id=r.id) ORDER BY actualizada_en,id`)
	if err != nil {
		return 0, err
	}
	type pending struct{ id, host string }
	items := []pending{}
	for rows.Next() {
		var item pending
		if err = rows.Scan(&item.id, &item.host); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	rows.Close()
	count := 0
	for _, item := range items {
		_, createErr := r.Create(ctx, item.host, item.id, clock)
		if createErr == nil {
			count++
			continue
		}
		if errors.Is(createErr, contract.ErrConflict) || errors.Is(createErr, contract.ErrNotFound) {
			continue
		}
		return count, createErr
	}
	return count, nil
}

func expireLocked(ctx context.Context, tx pgx.Tx, cid, rid, actor string, now time.Time) error {
	var state string
	var start time.Time
	if err := tx.QueryRow(ctx, `SELECT estado,inicio FROM public.reserva_ensayo_local WHERE id=$1`, rid).Scan(&state, &start); err != nil {
		return err
	}
	if now.Before(start) {
		return nil
	}
	if state == "cancelada_por_firma" {
		return nil
	}
	var signed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.contrato_ensayo_firma WHERE contrato_id=$1 AND estado='firmada'`, cid).Scan(&signed); err != nil {
		return err
	}
	if signed == 2 {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, rid, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_por_firma',actualizada_en=$2 WHERE id=$1`, rid, now); err != nil {
		return err
	}
	if err := bookingEvent(ctx, tx, rid, state, "cancelada_por_firma", actor, "vencimiento de firmas del contrato de ensayo", now); err != nil {
		return err
	}
	var paid int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(importe_clp),0) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'`, rid).Scan(&paid); err != nil {
		return err
	}
	if paid > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO public.reserva_devolucion_ensayo(id,reserva_id,operacion_id,importe_clp,moneda,estado,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,gen_random_uuid(),$2,'CLP','pendiente',$3,$3) ON CONFLICT(reserva_id) DO NOTHING`, rid, paid, now); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE public.contrato_ensayo_local SET estado='anulado',actualizada_en=$2 WHERE id=$1`, cid, now); err != nil {
		return err
	}
	return event(ctx, tx, cid, actor, "vencido", "faltaban firmas al inicio de la reserva", now)
}

// ExpireForReservation applies the M07 signature deadline while the caller
// already holds participant-account and reservation locks. It is used by a
// new message send so a stale partial-contract state cannot accept a message
// before a read or background sweep materializes the expiry.
func ExpireForReservation(ctx context.Context, tx pgx.Tx, reservationID string, now time.Time) (bool, error) {
	var reservationState string
	var start time.Time
	if err := tx.QueryRow(ctx, `SELECT estado,inicio FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&reservationState, &start); err != nil {
		return false, err
	}
	if reservationState != "firma_parcial" || now.Before(start) {
		return false, nil
	}
	var contractID, contractState string
	err := tx.QueryRow(ctx, `SELECT id::text,estado FROM public.contrato_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&contractID, &contractState)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, contract.ErrConflict
	}
	if err != nil {
		return false, err
	}
	if contractState != "firma_parcial" {
		return false, contract.ErrConflict
	}
	if err = expireLocked(ctx, tx, contractID, reservationID, "", now); err != nil {
		return false, err
	}
	return true, nil
}

func event(ctx context.Context, tx pgx.Tx, id, actor, action, reason string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.contrato_ensayo_historial(id,contrato_id,secuencia,actor_id,accion,motivo,creada_en) SELECT gen_random_uuid(),$1,COALESCE(MAX(secuencia),0)+1,NULLIF($2,'')::uuid,$3,$4,$5 FROM public.contrato_ensayo_historial WHERE contrato_id=$1`, id, actor, action, reason, at)
	return err
}
func bookingEvent(ctx context.Context, tx pgx.Tx, id, from, to, actor, reason string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en) SELECT gen_random_uuid(),$1,COALESCE(MAX(secuencia),0)+1,$2,$3,NULLIF($4,'')::uuid,$5,$6 FROM public.reserva_ensayo_transicion WHERE reserva_id=$1`, id, from, to, actor, reason, at)
	return err
}

// CancelForReservation coordinates M06's local_flexible_v1 cancellation with
// M07 while the caller holds the reservation row lock. Incomplete contracts
// become terminal before another signer can proceed; a fully signed contract
// remains a historical signed artifact.
func CancelForReservation(ctx context.Context, tx pgx.Tx, reservationID, actor string, at time.Time) error {
	var id, state string
	err := tx.QueryRow(ctx, `SELECT id::text,estado FROM public.contrato_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).Scan(&id, &state)
	if errors.Is(err, pgx.ErrNoRows) || state == "anulado" || state == "firmado" {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.contrato_ensayo_local SET estado='anulado',actualizada_en=$2 WHERE id=$1`, id, at); err != nil {
		return err
	}
	return event(ctx, tx, id, actor, "reserva_cancelada", "local_flexible_v1", at)
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func load(ctx context.Context, q rowQuerier, id string, key []byte) (contract.Contract, error) {
	var v contract.Contract
	err := q.QueryRow(ctx, `SELECT `+columns+` FROM public.contrato_ensayo_local WHERE id=$1`, id).Scan(&v.ID, &v.ReservationID, &v.DocumentID, &v.Version, &v.State, &v.Snapshot, &v.SHA256, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, contract.ErrNotFound
	}
	if err != nil {
		return v, err
	}
	if pool, ok := q.(*pgxpool.Pool); ok {
		stored, readErr := documents.ReadPDF(ctx, pool, v.DocumentID)
		err = readErr
		if err != nil {
			return v, err
		}
		v.Artifact, err = decryptPDF(key, stored)
		if err != nil {
			return v, err
		}
		if Hash(v.Artifact) != v.SHA256 {
			return v, errors.New("synthetic contract document checksum mismatch")
		}
	}
	sigRows, err := q.(interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}).Query(ctx, `SELECT firmante_id::text,rol,estado,motivo,actualizada_en FROM public.contrato_ensayo_firma WHERE contrato_id=$1 ORDER BY rol`, id)
	if err != nil {
		return v, err
	}
	defer sigRows.Close()
	v.Signatures = []contract.Signature{}
	for sigRows.Next() {
		var s contract.Signature
		if err = sigRows.Scan(&s.SignerID, &s.Role, &s.State, &s.Reason, &s.UpdatedAt); err != nil {
			return v, err
		}
		v.Signatures = append(v.Signatures, s)
	}
	if err = sigRows.Err(); err != nil {
		return v, err
	}
	histRows, err := q.(interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}).Query(ctx, `SELECT secuencia,COALESCE(actor_id::text,''),accion,motivo,creada_en FROM public.contrato_ensayo_historial WHERE contrato_id=$1 ORDER BY secuencia`, id)
	if err != nil {
		return v, err
	}
	defer histRows.Close()
	v.History = []contract.Event{}
	for histRows.Next() {
		var e contract.Event
		if err = histRows.Scan(&e.Sequence, &e.ActorID, &e.Action, &e.Reason, &e.At); err != nil {
			return v, err
		}
		v.History = append(v.History, e)
	}
	return v, histRows.Err()
}

func scan(row pgx.Row) (contract.Contract, error) {
	var v contract.Contract
	err := row.Scan(&v.ID, &v.ReservationID, &v.DocumentID, &v.Version, &v.State, &v.Snapshot, &v.SHA256, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func syntheticPDF(lines ...string) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 10 Tf 40 760 Td\n")
	lineNo := 0
	for _, line := range lines {
		line = strings.ToValidUTF8(line, "?")
		line = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(line)
		// Standard Helvetica/WinAnsi supports the synthetic fixture's Latin text.
		var encoded strings.Builder
		for _, r := range line {
			if r > 255 {
				encoded.WriteByte('?')
			} else {
				encoded.WriteByte(byte(r))
			}
		}
		parts := []string{encoded.String()}
		for len(parts[len(parts)-1]) > 88 {
			tail := parts[len(parts)-1]
			cut := strings.LastIndex(tail[:89], " ")
			if cut < 1 {
				cut = 88
			}
			parts[len(parts)-1] = tail[:cut]
			parts = append(parts, strings.TrimSpace(tail[cut:]))
		}
		for _, part := range parts {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "\\", "\\\\"), "(", "\\(")
			part = strings.ReplaceAll(part, ")", "\\)")
			if lineNo > 0 {
				content.WriteString("0 -14 Td\n")
			}
			content.WriteString("(" + part + ") Tj\n")
			lineNo++
		}
	}
	content.WriteString("ET")
	stream := []byte(content.String())
	objs := [][]byte{[]byte("<< /Type /Catalog /Pages 2 0 R >>"), []byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"), []byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"), []byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"), []byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, o := range objs {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}
func Hash(blob []byte) string { v := sha256.Sum256(blob); return hex.EncodeToString(v[:]) }

func encryptPDF(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, []byte("LOCAL-CONT-01")), nil
}
func decryptPDF(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("invalid encrypted synthetic document")
	}
	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], []byte("LOCAL-CONT-01"))
}
