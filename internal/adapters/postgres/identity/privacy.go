package postgres

import (
	"context"
	"errors"

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
