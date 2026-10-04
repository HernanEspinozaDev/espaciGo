package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	dbgen "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity/dbgen"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IdentityRepository implements identity-owned persistence through generated sqlc queries.
type IdentityRepository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

var (
	_ identity.AccountRepository     = (*IdentityRepository)(nil)
	_ identity.ActionTokenRepository = (*IdentityRepository)(nil)
)

func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool, queries: dbgen.New(pool)}
}

func (r *IdentityRepository) CreateWithTerms(ctx context.Context, account identity.Account, acceptances []identity.TermsAcceptance) error {
	if err := account.Validate(); err != nil {
		return err
	}
	for _, acceptance := range acceptances {
		if acceptance.AccountID != account.ID || !validTermsAcceptance(acceptance) {
			return identity.ErrInvalid
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()

	queries := r.queries.WithTx(tx)
	if err := queries.CreateAccount(ctx, dbgen.CreateAccountParams{
		ID: account.ID, Email: account.Email, NormalizedEmail: account.NormalizedEmail,
		PasswordHash: string(account.PasswordHash), State: string(account.State),
		CreatedAt: dbTime(account.CreatedAt), UpdatedAt: dbTime(account.UpdatedAt),
	}); err != nil {
		return mapError(err)
	}
	if err := queries.CreateTenantRole(ctx, account.ID); err != nil {
		return mapError(err)
	}
	for _, acceptance := range acceptances {
		if err := queries.CreateTermsAcceptance(ctx, dbgen.CreateTermsAcceptanceParams{
			ID: acceptance.ID, AccountID: acceptance.AccountID, VersionID: acceptance.VersionID,
			AcceptedAt: dbTime(acceptance.AcceptedAt), Channel: acceptance.Channel,
		}); err != nil {
			return mapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(err)
	}
	committed = true
	return nil
}

func (r *IdentityRepository) AccountByNormalizedEmail(ctx context.Context, email string) (identity.Account, error) {
	row, err := r.queries.GetAccountByNormalizedEmail(ctx, email)
	if err != nil {
		return identity.Account{}, mapError(err)
	}
	return accountFrom(row.ID, row.Email, row.NormalizedEmail, row.PasswordHash, row.State,
		row.CreatedAt, row.UpdatedAt, row.FailedAttempts, row.BlockedUntil), nil
}

func (r *IdentityRepository) AccountByID(ctx context.Context, id string) (identity.Account, error) {
	row, err := r.queries.GetAccountByID(ctx, id)
	if err != nil {
		return identity.Account{}, mapError(err)
	}
	return accountFrom(row.ID, row.Email, row.NormalizedEmail, row.PasswordHash, row.State,
		row.CreatedAt, row.UpdatedAt, row.FailedAttempts, row.BlockedUntil), nil
}

func (r *IdentityRepository) SaveLoginState(ctx context.Context, id string, state identity.AccountState, attempts int, blocked *time.Time) error {
	if !state.Valid() || attempts < 0 || int64(attempts) > 2_147_483_647 {
		return identity.ErrInvalid
	}
	rows, err := r.queries.UpdateLoginState(ctx, dbgen.UpdateLoginStateParams{
		ID: id, State: string(state), FailedAttempts: int32(attempts), BlockedUntil: dbNullableTime(blocked),
	})
	if err != nil {
		return mapError(err)
	}
	if rows == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (r *IdentityRepository) CreateSession(ctx context.Context, session identity.Session) error {
	if session.ID == "" || session.AccountID == "" || session.TokenHash == "" ||
		!session.ExpiresAt.After(session.CreatedAt) || session.ExpiresAt.After(session.CreatedAt.Add(8*time.Hour)) ||
		session.LastActivityAt.Before(session.CreatedAt) || session.LastActivityAt.After(session.ExpiresAt) ||
		(session.RevokedAt != nil && session.RevokedAt.Before(session.CreatedAt)) {
		return identity.ErrInvalid
	}
	clientSummary := session.ClientSummary
	return mapError(r.queries.CreateSession(ctx, dbgen.CreateSessionParams{
		ID: session.ID, AccountID: session.AccountID, TokenHash: session.TokenHash,
		CreatedAt: dbTime(session.CreatedAt), LastActivityAt: dbTime(session.LastActivityAt),
		ExpiresAt: dbTime(session.ExpiresAt), RevokedAt: dbNullableTime(session.RevokedAt),
		ClientSummary: &clientSummary,
	}))
}

func (r *IdentityRepository) SessionByTokenHash(ctx context.Context, hash string) (identity.Session, error) {
	row, err := r.queries.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return identity.Session{}, mapError(err)
	}
	return identity.Session{
		ID: row.ID, AccountID: row.AccountID, TokenHash: row.TokenHash,
		CreatedAt: row.CreatedAt.Time, LastActivityAt: row.LastActivityAt.Time,
		ExpiresAt: row.ExpiresAt.Time, RevokedAt: nullableTime(row.RevokedAt),
		ClientSummary: row.ClientSummary,
	}, nil
}

func (r *IdentityRepository) RevokeSession(ctx context.Context, id string, at time.Time) error {
	rows, err := r.queries.RevokeSession(ctx, dbgen.RevokeSessionParams{ID: id, RevokedAt: dbTime(at)})
	if err != nil {
		return mapError(err)
	}
	if rows == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (r *IdentityRepository) TouchSession(ctx context.Context, id string, at time.Time) (bool, error) {
	if at.IsZero() {
		return false, identity.ErrInvalid
	}
	rows, err := r.queries.TouchSession(ctx, dbgen.TouchSessionParams{ID: id, ActivityAt: dbTime(at)})
	if err != nil {
		return false, mapError(err)
	}
	return rows == 1, nil
}

func (r *IdentityRepository) TermsVersion(ctx context.Context, id string) (identity.TermsVersion, error) {
	row, err := r.queries.GetTermsVersion(ctx, id)
	if err != nil {
		return identity.TermsVersion{}, mapError(err)
	}
	return identity.TermsVersion{
		ID: row.ID, Code: row.Code, Type: row.Type, SHA256: row.Sha256, PublishedAt: row.PublishedAt.Time,
	}, nil
}

func (r *IdentityRepository) AcceptTerms(ctx context.Context, acceptance identity.TermsAcceptance) error {
	if !validTermsAcceptance(acceptance) {
		return identity.ErrInvalid
	}
	return mapError(r.queries.CreateTermsAcceptance(ctx, dbgen.CreateTermsAcceptanceParams{
		ID: acceptance.ID, AccountID: acceptance.AccountID, VersionID: acceptance.VersionID,
		AcceptedAt: dbTime(acceptance.AcceptedAt), Channel: acceptance.Channel,
	}))
}

func (r *IdentityRepository) ReplaceActiveActionToken(ctx context.Context, token identity.ActionToken) error {
	if !validNewActionToken(token) {
		return identity.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	queries := r.queries.WithTx(tx)
	if _, err := queries.LockAccountForActionToken(ctx, token.AccountID); err != nil {
		return mapError(err)
	}
	_, err = queries.InvalidateActiveActionTokens(ctx, dbgen.InvalidateActiveActionTokensParams{
		AccountID: token.AccountID, Purpose: token.Purpose, InvalidatedAt: dbTime(token.CreatedAt),
	})
	if err != nil {
		return mapError(err)
	}
	if err := queries.CreateActionToken(ctx, dbgen.CreateActionTokenParams{
		ID: token.ID, AccountID: token.AccountID, Purpose: token.Purpose, TokenHash: token.Hash,
		CreatedAt: dbTime(token.CreatedAt), ExpiresAt: dbTime(token.ExpiresAt),
		ConsumedAt: dbNullableTime(token.ConsumedAt), InvalidatedAt: dbNullableTime(token.InvalidatedAt),
		Attempts: int32(token.Attempts),
	}); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (r *IdentityRepository) ActionTokenByHash(ctx context.Context, hash string) (identity.ActionToken, error) {
	row, err := r.queries.GetActionTokenByHash(ctx, hash)
	if err != nil {
		return identity.ActionToken{}, mapError(err)
	}
	return identity.ActionToken{
		ID: row.ID, AccountID: row.AccountID, Purpose: row.Purpose, Hash: row.TokenHash,
		CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time,
		ConsumedAt: nullableTime(row.ConsumedAt), InvalidatedAt: nullableTime(row.InvalidatedAt),
		Attempts: int(row.Attempts),
	}, nil
}

func (r *IdentityRepository) RecordActionTokenFailure(ctx context.Context, hash string, at time.Time) (bool, error) {
	rows, err := r.queries.RecordActionTokenFailure(ctx, dbgen.RecordActionTokenFailureParams{
		TokenHash: hash, AttemptedAt: dbTime(at),
	})
	if err != nil {
		return false, mapError(err)
	}
	return rows == 1, nil
}

func (r *IdentityRepository) ConsumeActionToken(ctx context.Context, hash string, at time.Time) (bool, error) {
	rows, err := r.queries.ConsumeActionToken(ctx, dbgen.ConsumeActionTokenParams{
		TokenHash: hash, ConsumedAt: dbTime(at),
	})
	if err != nil {
		return false, mapError(err)
	}
	return rows == 1, nil
}

func (r *IdentityRepository) CountActionTokenEmissions(ctx context.Context, accountID, purpose string, since time.Time) (int64, error) {
	count, err := r.queries.CountActionTokenEmissions(ctx, dbgen.CountActionTokenEmissionsParams{
		AccountID: accountID, Purpose: purpose, Since: dbTime(since),
	})
	return count, mapError(err)
}

func accountFrom(id, email, normalizedEmail, passwordHash, state string, createdAt, updatedAt pgtype.Timestamptz, failedAttempts int32, blockedUntil pgtype.Timestamptz) identity.Account {
	return identity.Account{
		ID: id, Email: email, NormalizedEmail: normalizedEmail, PasswordHash: identity.Secret(passwordHash),
		State: identity.AccountState(state), CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
		FailedAttempts: int(failedAttempts), BlockedUntil: nullableTime(blockedUntil),
	}
}

func validTermsAcceptance(acceptance identity.TermsAcceptance) bool {
	if acceptance.ID == "" || acceptance.AccountID == "" || acceptance.VersionID == "" || acceptance.AcceptedAt.IsZero() {
		return false
	}
	switch acceptance.Channel {
	case "web", "api", "administrado":
		return true
	default:
		return false
	}
}

func validNewActionToken(token identity.ActionToken) bool {
	if token.ID == "" || token.AccountID == "" || token.CreatedAt.IsZero() || token.ExpiresAt.IsZero() ||
		token.Attempts != 0 || token.ConsumedAt != nil || token.InvalidatedAt != nil ||
		!token.ExpiresAt.After(token.CreatedAt) || len(token.Hash) != 64 || token.Hash != strings.ToLower(token.Hash) {
		return false
	}
	if token.Purpose != "verificar_correo" && token.Purpose != "recuperar_clave" {
		return false
	}
	_, err := hex.DecodeString(token.Hash)
	return err == nil
}

func dbTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func dbNullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return dbTime(*value)
}

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return identity.ErrConflict
		case "23502", "23503", "23514", "22P02", "22007":
			return identity.ErrInvalid
		}
	}
	return err
}
