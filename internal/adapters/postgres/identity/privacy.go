package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	dbgen "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity/dbgen"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
)

var _ privacy.Repository = (*IdentityRepository)(nil)

func (r *IdentityRepository) GetProfile(ctx context.Context, accountID string) (privacy.Profile, error) {
	row, err := r.queries.GetProfile(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.Profile{}, privacy.ErrNotFound
		}
		return privacy.Profile{}, mapError(err)
	}
	return privacy.Profile{AccountID: row.AccountID, Name: row.NombreVisible, Phone: row.TelefonoNormalizado, UpdatedAt: row.ActualizadoEn.Time}, nil
}

func (r *IdentityRepository) SaveProfile(ctx context.Context, accountID, name string, phone *string) (privacy.Profile, error) {
	row, err := r.queries.UpsertProfile(ctx, dbgen.UpsertProfileParams{AccountID: accountID, DisplayName: name, Phone: phone})
	if err != nil {
		return privacy.Profile{}, mapError(err)
	}
	return privacy.Profile{AccountID: row.AccountID, Name: row.NombreVisible, Phone: row.TelefonoNormalizado, UpdatedAt: row.ActualizadoEn.Time}, nil
}

func (r *IdentityRepository) CreateRightsRequest(ctx context.Context, accountID, kind, channel string) (privacy.RightsRequest, error) {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		return privacy.RightsRequest{}, err
	}
	row, err := r.queries.CreateRightsRequest(ctx, dbgen.CreateRightsRequestParams{ID: id, AccountID: accountID, Kind: kind, Channel: channel})
	if err != nil {
		return privacy.RightsRequest{}, mapError(err)
	}
	return privacy.RightsRequest{ID: row.ID, Kind: row.Kind, State: row.State, CreatedAt: row.SolicitadaEn.Time}, nil
}

func (r *IdentityRepository) ListOwnRightsRequests(ctx context.Context, accountID string) ([]privacy.RightsRequest, error) {
	rows, err := r.queries.ListOwnRightsRequests(ctx, accountID)
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]privacy.RightsRequest, 0, len(rows))
	for _, row := range rows {
		items = append(items, privacy.RightsRequest{ID: row.ID, Kind: row.Kind, State: row.State, CreatedAt: row.SolicitadaEn.Time})
	}
	return items, nil
}

// ExportOwnData reads one consistent, owner-scoped identity snapshot. It never
// selects password hashes, sessions, action tokens, or unrelated domain rows.
func (r *IdentityRepository) ExportOwnData(ctx context.Context, accountID string) (privacy.OwnData, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return privacy.OwnData{}, mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var data privacy.OwnData
	data.Scope = "identidad_local_v1"
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `SELECT correo_original, estado, preferencia_uso, creado_en
		FROM public.usuario WHERE id=$1`, accountID).Scan(
		&data.Account.Email, &data.Account.State, &data.Account.UsePreference, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.OwnData{}, privacy.ErrNotFound
		}
		return privacy.OwnData{}, mapError(err)
	}
	data.Account.CreatedAt = createdAt.UTC()

	data.Roles = []string{}
	rows, err := tx.Query(ctx, `SELECT rol FROM public.rol_usuario WHERE usuario_id=$1 ORDER BY rol`, accountID)
	if err != nil {
		return privacy.OwnData{}, mapError(err)
	}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			rows.Close()
			return privacy.OwnData{}, mapError(err)
		}
		data.Roles = append(data.Roles, role)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return privacy.OwnData{}, mapError(err)
	}
	rows.Close()

	data.Acceptances = []privacy.TermsAcceptance{}
	rows, err = tx.Query(ctx, `SELECT v.codigo, v.tipo, a.aceptada_en
		FROM public.aceptacion_terminos a JOIN public.version_terminos v ON v.id=a.version_id
		WHERE a.usuario_id=$1 ORDER BY a.aceptada_en, v.codigo`, accountID)
	if err != nil {
		return privacy.OwnData{}, mapError(err)
	}
	for rows.Next() {
		var item privacy.TermsAcceptance
		if err := rows.Scan(&item.Code, &item.Type, &item.AcceptedAt); err != nil {
			rows.Close()
			return privacy.OwnData{}, mapError(err)
		}
		item.AcceptedAt = item.AcceptedAt.UTC()
		data.Acceptances = append(data.Acceptances, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return privacy.OwnData{}, mapError(err)
	}
	rows.Close()

	var profile privacy.ExportProfile
	if err := tx.QueryRow(ctx, `SELECT nombre_visible, telefono_normalizado, actualizado_en
		FROM public.perfil_usuario WHERE usuario_id=$1`, accountID).Scan(&profile.Name, &profile.Phone, &profile.UpdatedAt); err == nil {
		profile.UpdatedAt = profile.UpdatedAt.UTC()
		data.Profile = &profile
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return privacy.OwnData{}, mapError(err)
	}

	data.Requests = []privacy.RightsRequest{}
	rows, err = tx.Query(ctx, `SELECT id::text, tipo, estado, solicitada_en
		FROM public.solicitud_titular WHERE usuario_id=$1 ORDER BY solicitada_en, id`, accountID)
	if err != nil {
		return privacy.OwnData{}, mapError(err)
	}
	for rows.Next() {
		var item privacy.RightsRequest
		if err := rows.Scan(&item.ID, &item.Kind, &item.State, &item.CreatedAt); err != nil {
			rows.Close()
			return privacy.OwnData{}, mapError(err)
		}
		item.CreatedAt = item.CreatedAt.UTC()
		data.Requests = append(data.Requests, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return privacy.OwnData{}, mapError(err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return privacy.OwnData{}, mapError(err)
	}
	return data, nil
}

// ReviewSuppression persists a read-only assessment of existing obligations.
// It serializes on the subject account, captures time after the locks and does
// not delete, scrub or change the rights request. A later executor must repeat
// these checks in the same transaction as any suppression effects.
func (r *IdentityRepository) ReviewSuppression(ctx context.Context, reviewerID, requestID, idempotencyKey, correlationID string, now func() time.Time) (privacy.SuppressionReview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return privacy.SuppressionReview{}, mapError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var subjectID, requestKind string
	if err := tx.QueryRow(ctx, `SELECT usuario_id::text, tipo FROM public.solicitud_titular WHERE id=$1`, requestID).Scan(&subjectID, &requestKind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.SuppressionReview{}, privacy.ErrNotFound
		}
		return privacy.SuppressionReview{}, mapError(err)
	}
	if requestKind != "supresion" {
		return privacy.SuppressionReview{}, privacy.ErrNotFound
	}
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE id=$1 FOR UPDATE`, subjectID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.SuppressionReview{}, privacy.ErrNotFound
		}
		return privacy.SuppressionReview{}, mapError(err)
	}
	var state string
	if err := tx.QueryRow(ctx, `SELECT estado FROM public.solicitud_titular WHERE id=$1 AND usuario_id=$2 AND tipo='supresion'`, requestID, subjectID).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return privacy.SuppressionReview{}, privacy.ErrNotFound
		}
		return privacy.SuppressionReview{}, mapError(err)
	}

	var existing privacy.SuppressionReview
	var existingReviewer, existingReason string
	var existingDetails []byte
	err = tx.QueryRow(ctx, `SELECT actor_id::text, motivo_codigo, ocurrido_en, detalle_codigos
		FROM public.evento_auditoria_local
		WHERE recurso_id=$1 AND accion='privacy.suppression.review' AND clave_idempotencia=$2`, requestID, idempotencyKey).Scan(
		&existingReviewer, &existingReason, &existing.ReviewedAt, &existingDetails)
	if err == nil {
		if existingReviewer != reviewerID {
			return privacy.SuppressionReview{}, identity.ErrConflict
		}
		existing.RequestID = requestID
		switch existingReason {
		case "supresion_con_obligaciones":
			existing.Outcome = "bloqueada"
		case "supresion_revision_incompleta":
			existing.Outcome = "revision_incompleta"
		default:
			return privacy.SuppressionReview{}, identity.ErrConflict
		}
		var details struct {
			Obligations   []string `json:"obligations_detected"`
			PendingChecks []string `json:"pending_checks"`
		}
		if err := json.Unmarshal(existingDetails, &details); err != nil {
			return privacy.SuppressionReview{}, mapError(err)
		}
		existing.Obligations = details.Obligations
		existing.PendingChecks = details.PendingChecks
		existing.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return privacy.SuppressionReview{}, mapError(err)
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return privacy.SuppressionReview{}, mapError(err)
	}
	if state != "en_revision" {
		return privacy.SuppressionReview{}, identity.ErrConflict
	}
	if now == nil {
		now = time.Now
	}
	checkedAt := now().UTC().Truncate(time.Microsecond) // Sample post-lock and match PostgreSQL timestamp precision.
	obligations := make([]string, 0, 3)
	pendingChecks := []string{"fuente_disputas_no_modelada", "matriz_retencion_historicos_incompleta"}
	var found bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM public.reserva_ensayo_local
		WHERE (anfitrion_id=$1 OR arrendatario_id=$1)
		  AND estado IN ('pendiente_de_pago','pagada','aprobada_host')
	)`, subjectID).Scan(&found); err != nil {
		return privacy.SuppressionReview{}, mapError(err)
	}
	if found {
		obligations = append(obligations, "reserva_activa")
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM public.reserva_pago_ensayo_operacion p
		JOIN public.reserva_ensayo_local r ON r.id=p.reserva_id
		WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND p.estado='pendiente'
		UNION ALL
		SELECT 1 FROM public.reserva_pago_evento_aplicacion_ensayo a
		JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id
		JOIN public.reserva_pago_ensayo_operacion p ON p.id=e.operacion_id
		JOIN public.reserva_ensayo_local r ON r.id=p.reserva_id
		WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND a.estado='pendiente_conciliacion'
		UNION ALL
		SELECT 1 FROM public.reserva_devolucion_ensayo d
		JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id
		WHERE (r.anfitrion_id=$1 OR r.arrendatario_id=$1) AND d.estado='pendiente'
	)`, subjectID).Scan(&found); err != nil {
		return privacy.SuppressionReview{}, mapError(err)
	}
	if found {
		obligations = append(obligations, "pago_o_devolucion_pendiente")
	}
	outcome := "revision_incompleta"
	if len(obligations) > 0 {
		outcome = "bloqueada"
	}
	persisted := privacy.SuppressionReview{
		RequestID: requestID, Outcome: outcome, Obligations: obligations,
		PendingChecks: pendingChecks, ReviewedAt: checkedAt,
	}
	auditID, err := (credentials.Generator{}).ID()
	if err != nil {
		return privacy.SuppressionReview{}, err
	}
	reason := "supresion_revision_incompleta"
	result := "exito"
	if outcome == "bloqueada" {
		reason = "supresion_con_obligaciones"
	}
	detailJSON, err := json.Marshal(struct {
		Obligations   []string `json:"obligations_detected"`
		PendingChecks []string `json:"pending_checks"`
	}{Obligations: obligations, PendingChecks: pendingChecks})
	if err != nil {
		return privacy.SuppressionReview{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local (
		id, actor_id, recurso_tipo, recurso_id, accion, resultado, motivo_codigo,
		correlacion_id, ocurrido_en, retirar_en, clave_idempotencia, detalle_codigos
	) VALUES ($1,$2,'solicitud_titular',$3,'privacy.suppression.review',$4,$5,$6,$7,$8,$9,$10::jsonb)`,
		auditID, reviewerID, requestID, result, reason, correlationID, checkedAt, identity.AddCalendarMonthsUTC(checkedAt, 60), idempotencyKey, string(detailJSON)); err != nil {
		return privacy.SuppressionReview{}, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return privacy.SuppressionReview{}, mapError(err)
	}
	return persisted, nil
}

func (r *IdentityRepository) ListPendingSuppressions(ctx context.Context) ([]privacy.SuppressionQueueItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, solicitada_en
		FROM public.solicitud_titular
		WHERE tipo='supresion' AND estado='en_revision'
		ORDER BY solicitada_en, id`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]privacy.SuppressionQueueItem, 0)
	for rows.Next() {
		var item privacy.SuppressionQueueItem
		if err := rows.Scan(&item.RequestID, &item.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		item.CreatedAt = item.CreatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}
