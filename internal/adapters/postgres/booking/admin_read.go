package bookingpg

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

type adminReservationCursor struct {
	Version   int       `json:"v"`
	Actor     string    `json:"a"`
	Filter    string    `json:"f"`
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
	PageSize  int       `json:"s"`
}

var bookingUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func adminFilterHash(f booking.AdminReservationFilter) string {
	from, to := "", ""
	if f.CreatedFrom != nil {
		from = f.CreatedFrom.UTC().Format(time.RFC3339Nano)
	}
	if f.CreatedTo != nil {
		to = f.CreatedTo.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(f.ID)) + "\x00" + strings.TrimSpace(f.State) + "\x00" + from + "\x00" + to))
	return fmt.Sprintf("%x", sum[:])
}

func decodeAdminCursor(encoded, actor, filter string, pageSize int) (*adminReservationCursor, error) {
	if encoded == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(b) > 2048 {
		return nil, booking.ErrInvalid
	}
	var cursor adminReservationCursor
	if json.Unmarshal(b, &cursor) != nil || cursor.Version != 1 || cursor.Actor != actor || cursor.Filter != filter || cursor.PageSize != pageSize || !bookingUUID.MatchString(cursor.ID) || cursor.CreatedAt.IsZero() {
		return nil, booking.ErrInvalid
	}
	return &cursor, nil
}

func encodeAdminCursor(cursor adminReservationCursor) string {
	b, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (r *Repository) ListAdminReservations(ctx context.Context, actor, correlation string, filter booking.AdminReservationFilter, pageSize int, encodedCursor string) (booking.AdminReservationPage, error) {
	if pageSize < 1 || pageSize > 100 || !bookingUUID.MatchString(actor) || (filter.ID != "" && !bookingUUID.MatchString(filter.ID)) || correlation == "" || len(correlation) > 120 {
		return booking.AdminReservationPage{}, booking.ErrInvalid
	}
	hash := adminFilterHash(filter)
	cursor, err := decodeAdminCursor(encodedCursor, actor, hash, pageSize)
	if err != nil {
		return booking.AdminReservationPage{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.AdminReservationPage{}, err
	}
	defer tx.Rollback(ctx)
	if err = insertAdminReadAudit(ctx, tx, actor, "coleccion_reservas_ensayo_local", auditCollectionID, "booking.admin.reservations.list", correlation); err != nil {
		return booking.AdminReservationPage{}, err
	}
	query := `SELECT id::text,anfitrion_id::text,arrendatario_id::text,estado,inicio,termino,subtotal_clp,moneda,creada_en,actualizada_en FROM public.reserva_ensayo_local WHERE ($1='' OR id::text=lower($1)) AND ($2='' OR estado=$2) AND ($3::timestamptz IS NULL OR creada_en >= $3) AND ($4::timestamptz IS NULL OR creada_en < $4) AND ($5::timestamptz IS NULL OR (creada_en,id)<($5,$6::uuid)) ORDER BY creada_en DESC,id DESC LIMIT $7`
	var from, to *time.Time
	if filter.CreatedFrom != nil {
		t := filter.CreatedFrom.UTC()
		from = &t
	}
	if filter.CreatedTo != nil {
		t := filter.CreatedTo.UTC()
		to = &t
	}
	var lastAt *time.Time
	var lastID any
	if cursor != nil {
		t := cursor.CreatedAt.UTC()
		lastAt = &t
		lastID = cursor.ID
	}
	rows, err := tx.Query(ctx, query, strings.TrimSpace(filter.ID), strings.TrimSpace(filter.State), from, to, lastAt, lastID, pageSize+1)
	if err != nil {
		return booking.AdminReservationPage{}, err
	}
	items := make([]booking.AdminReservationSummary, 0, pageSize+1)
	for rows.Next() {
		var item booking.AdminReservationSummary
		var host, renter sql.NullString
		if err = rows.Scan(&item.ID, &host, &renter, &item.State, &item.StartAt, &item.EndAt, &item.Subtotal, &item.Currency, &item.CreatedAt, &item.UpdatedAt); err != nil {
			rows.Close()
			return booking.AdminReservationPage{}, err
		}
		item.HostID, item.RenterID = nullableString(host), nullableString(renter)
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return booking.AdminReservationPage{}, err
	}
	rows.Close()
	page := booking.AdminReservationPage{Items: make([]booking.AdminReservationSummary, 0, pageSize)}
	if len(items) > pageSize {
		items = items[:pageSize]
		last := items[len(items)-1]
		page.NextCursor = encodeAdminCursor(adminReservationCursor{Version: 1, Actor: actor, Filter: hash, CreatedAt: last.CreatedAt, ID: last.ID, PageSize: pageSize})
	}
	page.Items = append(page.Items, items...)
	if err = tx.Commit(ctx); err != nil {
		return booking.AdminReservationPage{}, err
	}
	return page, nil
}

func (r *Repository) GetAdminReservation(ctx context.Context, actor, reservationID, correlation string) (booking.AdminReservationDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.AdminReservationDetail{}, err
	}
	defer tx.Rollback(ctx)
	if err = insertAdminReadAudit(ctx, tx, actor, "reserva_ensayo_local", reservationID, "booking.admin.reservations.read", correlation); err != nil {
		return booking.AdminReservationDetail{}, err
	}
	var out booking.AdminReservationDetail
	var host, renter sql.NullString
	err = tx.QueryRow(ctx, `SELECT id::text,anfitrion_id::text,arrendatario_id::text,estado,inicio,termino,subtotal_clp,moneda,creada_en,actualizada_en,espacio_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(
		&out.ID, &host, &renter, &out.State, &out.StartAt, &out.EndAt, &out.Subtotal, &out.Currency, &out.CreatedAt, &out.UpdatedAt, &out.SpaceID)
	if err != nil {
		return booking.AdminReservationDetail{}, mapBookingAdminErr(err)
	}
	out.HostID, out.RenterID = nullableString(host), nullableString(renter)
	out.History = []booking.AdminTransition{}
	history, err := tx.Query(ctx, `SELECT secuencia,estado_anterior,estado_nuevo,actor_id::text,creada_en FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 ORDER BY secuencia`, reservationID)
	if err != nil {
		return booking.AdminReservationDetail{}, err
	}
	for history.Next() {
		var item booking.AdminTransition
		var from, actor sql.NullString
		if err = history.Scan(&item.Sequence, &from, &item.To, &actor, &item.At); err != nil {
			history.Close()
			return booking.AdminReservationDetail{}, err
		}
		item.From = nullableString(from)
		item.Actor = nullableString(actor)
		out.History = append(out.History, item)
	}
	if err = history.Err(); err != nil {
		history.Close()
		return booking.AdminReservationDetail{}, err
	}
	history.Close()
	out.Payments = []booking.AdminPaymentFact{}
	payments, err := tx.Query(ctx, `SELECT resultado,importe_clp,creada_en FROM public.reserva_pago_ensayo WHERE reserva_id=$1 ORDER BY creada_en,id`, reservationID)
	if err != nil {
		return booking.AdminReservationDetail{}, err
	}
	for payments.Next() {
		var item booking.AdminPaymentFact
		if err = payments.Scan(&item.Result, &item.AmountCLP, &item.At); err != nil {
			payments.Close()
			return booking.AdminReservationDetail{}, err
		}
		out.Payments = append(out.Payments, item)
	}
	if err = payments.Err(); err != nil {
		payments.Close()
		return booking.AdminReservationDetail{}, err
	}
	payments.Close()
	out.PaymentOperations = []booking.AdminPaymentOperation{}
	paymentOperations, err := tx.Query(ctx, `SELECT id::text,estado,resultado_solicitado,creada_en,actualizada_en FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 ORDER BY creada_en,id`, reservationID)
	if err != nil {
		return booking.AdminReservationDetail{}, err
	}
	for paymentOperations.Next() {
		var item booking.AdminPaymentOperation
		if err = paymentOperations.Scan(&item.ID, &item.State, &item.Requested, &item.CreatedAt, &item.UpdatedAt); err != nil {
			paymentOperations.Close()
			return booking.AdminReservationDetail{}, err
		}
		out.PaymentOperations = append(out.PaymentOperations, item)
	}
	if err = paymentOperations.Err(); err != nil {
		paymentOperations.Close()
		return booking.AdminReservationDetail{}, err
	}
	paymentOperations.Close()
	var refund booking.AdminRefundFact
	var refundResult sql.NullString
	err = tx.QueryRow(ctx, `SELECT importe_clp,moneda,estado,ultimo_resultado,creada_en,actualizada_en FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1`, reservationID).Scan(&refund.AmountCLP, &refund.Currency, &refund.State, &refundResult, &refund.CreatedAt, &refund.UpdatedAt)
	if err == nil {
		refund.LastResult = nullableString(refundResult)
		out.Refund = &refund
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return booking.AdminReservationDetail{}, err
	}
	var guarantee booking.AdminGuaranteeSnapshot
	err = readAdminGuarantee(ctx, tx, reservationID, &guarantee)
	if err == nil {
		out.Guarantee = &guarantee
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return booking.AdminReservationDetail{}, err
	}
	var claim booking.AdminClaimFact
	var claimOutcome sql.NullString
	var resolved sql.NullTime
	err = tx.QueryRow(ctx, `SELECT c.id::text,c.estado,c.abierto_en,x.resuelta_en,x.resultado FROM public.reclamo_dano_ensayo_local c LEFT JOIN public.reclamo_dano_resolucion_ensayo_local x ON x.reclamo_id=c.id WHERE c.reserva_id=$1`, reservationID).Scan(&claim.ID, &claim.State, &claim.OpenedAt, &resolved, &claimOutcome)
	if err == nil {
		claim.ResolvedAt = nullableTime(resolved)
		claim.Outcome = nullableString(claimOutcome)
		out.Claim = &claim
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return booking.AdminReservationDetail{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.AdminReservationDetail{}, err
	}
	return out, nil
}

func insertAdminReadAudit(ctx context.Context, tx pgx.Tx, actor, resourceType, resource, action, correlation string) error {
	if resource == "" || correlation == "" || len(correlation) > 120 {
		return booking.ErrInvalid
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en) VALUES(gen_random_uuid(),$1,$2,$3,$4,'exito','consulta_administrativa',$5,statement_timestamp(),statement_timestamp()+interval '5 years')`, actor, resourceType, resource, action, correlation)
	return err
}

func readAdminGuarantee(ctx context.Context, tx pgx.Tx, reservationID string, out *booking.AdminGuaranteeSnapshot) error {
	var deadline sql.NullTime
	err := tx.QueryRow(ctx, `SELECT politica_version,moneda,previsto_clp,autorizado_clp,capturado_clp,liberado_clp,estado,(SELECT min(o.vence_en) FROM public.reserva_garantia_operacion_ensayo_local o WHERE o.garantia_id=g.id AND o.tipo='autorizacion') FROM public.reserva_garantia_ensayo_local g WHERE g.reserva_id=$1`, reservationID).Scan(&out.PolicyVersion, &out.Currency, &out.ExpectedCLP, &out.AuthorizedCLP, &out.CapturedCLP, &out.ReleasedCLP, &out.State, &deadline)
	if err != nil {
		return err
	}
	out.AuthorizationDeadline = nullableTime(deadline)
	out.Operations = []booking.GuaranteeOperation{}
	rows, err := tx.Query(ctx, `SELECT id::text,tipo,importe_clp,estado,COALESCE(ultimo_resultado,''),creada_en,actualizada_en FROM public.reserva_garantia_operacion_ensayo_local WHERE garantia_id=(SELECT id FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1) ORDER BY creada_en,id`, reservationID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var op booking.GuaranteeOperation
		if err = rows.Scan(&op.ID, &op.Kind, &op.AmountCLP, &op.State, &op.LastResult, &op.CreatedAt, &op.UpdatedAt); err != nil {
			return err
		}
		out.Operations = append(out.Operations, op)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	var decision booking.AdminFinancialDecision
	err = tx.QueryRow(ctx, `SELECT resultado_reclamo,deduccion_clp,motivo_codigo,estado,creada_en,actualizada_en FROM public.reserva_decision_financiera_ensayo_local WHERE reserva_id=$1`, reservationID).Scan(&decision.Outcome, &decision.DeductionCLP, &decision.ReasonCode, &decision.State, &decision.CreatedAt, &decision.UpdatedAt)
	if err == nil {
		out.Decision = &decision
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}
func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time.UTC()
	return &v
}
func mapBookingAdminErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return booking.ErrNotFound
	}
	return err
}
