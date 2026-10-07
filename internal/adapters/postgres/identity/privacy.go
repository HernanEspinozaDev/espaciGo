package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	dbgen "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity/dbgen"
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
