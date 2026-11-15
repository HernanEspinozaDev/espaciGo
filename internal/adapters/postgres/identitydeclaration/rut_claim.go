package identitydeclaration

import (
	"context"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RutClaimStore is the durable, HMAC-only implementation of verification.RutClaimStore.
type RutClaimStore struct{ pool *pgxpool.Pool }

// rutClaimTx is the transaction surface needed by Claim's claim decision.
type rutClaimTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
	Rollback(context.Context) error
}

var _ verification.RutClaimStore = (*RutClaimStore)(nil)

func NewRutClaimStore(pool *pgxpool.Pool) *RutClaimStore { return &RutClaimStore{pool: pool} }

var rutFingerprintVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

func validRutClaimInput(accountID, declarationID, fingerprint string) bool {
	account, accountErr := uuid.Parse(accountID)
	declaration, declarationErr := uuid.Parse(declarationID)
	parts := strings.SplitN(fingerprint, ":", 2)
	if accountErr != nil || account.String() != accountID || declarationErr != nil || declaration.String() != declarationID || len(parts) != 2 || !rutFingerprintVersionPattern.MatchString(parts[0]) || len(parts[1]) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(parts[1])
	return err == nil && len(decoded) == 32
}

func rutClaimDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return verification.ErrConflict
	}
	return err
}

func (s *RutClaimStore) Claim(ctx context.Context, accountID, declarationID, fingerprint string, expiresAt, now time.Time) error {
	if s == nil || s.pool == nil || !validRutClaimInput(accountID, declarationID, fingerprint) || now.IsZero() || !expiresAt.After(now) {
		return verification.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	return claimRutInTx(ctx, tx, accountID, declarationID, fingerprint, expiresAt, now)
}

func claimRutInTx(ctx context.Context, tx rutClaimTx, accountID, declarationID, fingerprint string, expiresAt, now time.Time) error {
	defer tx.Rollback(ctx)
	var err error
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, accountID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.identity_rut_claim WHERE verified_at IS NULL AND expires_at <= $1`, now); err != nil {
		return err
	}
	var owner, decl string
	var verifiedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT account_id::text,COALESCE(declaration_id::text,''),verified_at FROM public.identity_rut_claim WHERE fingerprint=$1 FOR UPDATE`, fingerprint).Scan(&owner, &decl, &verifiedAt)
	lookupErr := err
	if err == nil && owner != accountID {
		return verification.ErrConflict
	}
	if !errors.Is(lookupErr, pgx.ErrNoRows) && lookupErr != nil {
		return lookupErr
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.identity_rut_claim WHERE account_id=$1 AND fingerprint<>$2 AND verified_at IS NULL`, accountID, fingerprint); err != nil {
		return err
	}
	if lookupErr == nil {
		if verifiedAt == nil {
			tag, updateErr := tx.Exec(ctx, `UPDATE public.identity_rut_claim SET declaration_id=$2,expires_at=$3,updated_at=$4 WHERE fingerprint=$1 AND account_id=$5`, fingerprint, declarationID, expiresAt, now, accountID)
			if updateErr != nil {
				return rutClaimDBError(updateErr)
			}
			if tag.RowsAffected() != 1 {
				return verification.ErrConflict
			}
		} else if decl != declarationID {
			// The marker is a historical anti-reuse reservation, not the new
			// declaration's approval. Rebinding must never make it expirable.
			tag, updateErr := tx.Exec(ctx, `UPDATE public.identity_rut_claim SET declaration_id=$2,updated_at=GREATEST(updated_at,$3) WHERE fingerprint=$1 AND account_id=$4 AND verified_at IS NOT NULL AND expires_at IS NULL`, fingerprint, declarationID, now, accountID)
			if updateErr != nil {
				return rutClaimDBError(updateErr)
			}
			if tag.RowsAffected() != 1 {
				return verification.ErrConflict
			}
		}
	} else if errors.Is(lookupErr, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO public.identity_rut_claim(fingerprint,account_id,declaration_id,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)`, fingerprint, accountID, declarationID, expiresAt, now)
		if err != nil {
			return rutClaimDBError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return rutClaimDBError(err)
	}
	return nil
}

type rutClaimDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *RutClaimStore) MarkVerified(ctx context.Context, accountID, declarationID, fingerprint string, now time.Time) error {
	if s == nil || s.pool == nil || !validRutClaimInput(accountID, declarationID, fingerprint) || now.IsZero() {
		return verification.ErrInvalid
	}
	return markVerifiedInDB(ctx, s.pool, accountID, declarationID, fingerprint, now)
}

func markVerifiedInDB(ctx context.Context, db rutClaimDB, accountID, declarationID, fingerprint string, now time.Time) error {
	tag, err := db.Exec(ctx, `UPDATE public.identity_rut_claim SET verified_at=$4,expires_at=NULL,updated_at=$4 WHERE fingerprint=$1 AND account_id=$2 AND declaration_id=$3 AND verified_at IS NULL AND expires_at>$4`, fingerprint, accountID, declarationID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		var owner, declaration string
		var verifiedAt *time.Time
		err = db.QueryRow(ctx, `SELECT account_id::text,COALESCE(declaration_id::text,''),verified_at FROM public.identity_rut_claim WHERE fingerprint=$1`, fingerprint).Scan(&owner, &declaration, &verifiedAt)
		if err == nil && owner == accountID && (declaration == declarationID || declaration == "") && verifiedAt != nil {
			return nil
		}
		return verification.ErrConflict
	}
	return nil
}

// RevokeVerified follows the approved lifetime rule: verified fingerprints remain
// reserved to prevent historical reuse. The legacy interface cannot report DB errors.
func (s *RutClaimStore) RevokeVerified(ctx context.Context, accountID string, now time.Time) {
	// The approved policy keeps verified claims permanently; this legacy operation
	// therefore does not mutate state. It cannot report storage errors by contract.
	_ = s
	_ = ctx
	_ = accountID
	_ = now
}
