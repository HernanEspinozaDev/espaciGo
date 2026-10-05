package postgres

import (
	"context"
	"strings"
	"time"

	dbgen "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity/dbgen"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

var _ identity.AuthenticationRepository = (*IdentityRepository)(nil)

type authenticationTransaction struct{ *IdentityRepository }

func (r *IdentityRepository) WithLockedAccount(ctx context.Context, lookup identity.AccountLookup, fn func(identity.Account, identity.AuthenticationTransaction) error) error {
	count := 0
	for _, key := range []string{lookup.Email, lookup.VerificationID, lookup.ActionTokenID, lookup.SessionHash} {
		if key != "" {
			count++
		}
	}
	if count != 1 || fn == nil {
		return identity.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	var account identity.Account
	switch {
	case lookup.Email != "":
		row, e := q.LockAccountByEmail(ctx, lookup.Email)
		err = e
		account = accountFrom(row.ID, row.Email, row.NormalizedEmail, row.PasswordHash, row.State, row.CreatedAt, row.UpdatedAt, row.FailedAttempts, row.BlockedUntil)
	case lookup.VerificationID != "":
		row, e := q.LockAccountByVerificationID(ctx, lookup.VerificationID)
		err = e
		account = accountFrom(row.ID, row.Email, row.NormalizedEmail, row.PasswordHash, row.State, row.CreatedAt, row.UpdatedAt, row.FailedAttempts, row.BlockedUntil)
	case lookup.ActionTokenID != "":
		row, e := q.LockAccountByActionTokenID(ctx, lookup.ActionTokenID)
		err = e
		account = accountFrom(row.ID, row.Email, row.NormalizedEmail, row.PasswordHash, row.State, row.CreatedAt, row.UpdatedAt, row.FailedAttempts, row.BlockedUntil)
	default:
		row, e := q.LockAccountBySessionHash(ctx, lookup.SessionHash)
		err = e
		account = accountFrom(row.ID, row.Email, row.NormalizedEmail, row.PasswordHash, row.State, row.CreatedAt, row.UpdatedAt, row.FailedAttempts, row.BlockedUntil)
	}
	if err != nil {
		return mapError(err)
	}
	// Only expose methods that operate on the transaction's generated queries.
	unit := &authenticationTransaction{&IdentityRepository{queries: q}}
	if err := fn(account, unit); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

func (t *authenticationTransaction) Roles(ctx context.Context, id string) ([]identity.Role, error) {
	rows, err := t.queries.GetAccountRoles(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	roles := make([]identity.Role, len(rows))
	for i, role := range rows {
		roles[i] = identity.Role(role)
	}
	return roles, nil
}

func (t *authenticationTransaction) ActionToken(ctx context.Context, id string) (identity.ActionToken, error) {
	row, err := t.queries.GetActionTokenByID(ctx, id)
	if err != nil {
		return identity.ActionToken{}, mapError(err)
	}
	return identity.ActionToken{ID: row.ID, AccountID: row.AccountID, Purpose: row.Purpose, Hash: row.TokenHash, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time, ConsumedAt: nullableTime(row.ConsumedAt), InvalidatedAt: nullableTime(row.InvalidatedAt), Attempts: int(row.Attempts)}, nil
}

func (t *authenticationTransaction) ReplaceActionToken(ctx context.Context, token identity.ActionToken) error {
	if !validNewActionToken(token) || (token.Purpose != "verificar_correo" && token.Purpose != "recuperar_clave") {
		return identity.ErrInvalid
	}
	if _, err := t.queries.InvalidateActiveActionTokens(ctx, dbgen.InvalidateActiveActionTokensParams{AccountID: token.AccountID, Purpose: token.Purpose, InvalidatedAt: dbTime(token.CreatedAt)}); err != nil {
		return mapError(err)
	}
	return mapError(t.queries.CreateActionToken(ctx, dbgen.CreateActionTokenParams{ID: token.ID, AccountID: token.AccountID, Purpose: token.Purpose, TokenHash: strings.ToLower(token.Hash), CreatedAt: dbTime(token.CreatedAt), ExpiresAt: dbTime(token.ExpiresAt)}))
}

func (t *authenticationTransaction) UpdatePasswordHash(ctx context.Context, accountID string, hash identity.Secret) error {
	return mapError(t.queries.UpdatePasswordHash(ctx, dbgen.UpdatePasswordHashParams{AccountID: accountID, PasswordHash: string(hash)}))
}

func (t *authenticationTransaction) RevokeActiveSessions(ctx context.Context, accountID string, at time.Time) error {
	return mapError(t.queries.RevokeActiveSessions(ctx, dbgen.RevokeActiveSessionsParams{AccountID: accountID, RevokedAt: dbTime(at)}))
}
