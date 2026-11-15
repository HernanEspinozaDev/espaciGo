package identitydeclaration

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
)

const (
	rutClaimLockKeyset        = "espacigo:rut-hmac-claim-keyset:v1"
	rutClaimAccountLockSQL    = `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`
	rutClaimGlobalLockSQL     = `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`
	rutClaimUnknownVersionSQL = `SELECT EXISTS (SELECT 1 FROM public.identity_rut_claim WHERE split_part(fingerprint,':',1) <> ALL($1::text[]))`
	rutClaimLookupSQL         = `SELECT account_id::text,COALESCE(declaration_id::text,''),verified_at,expires_at FROM public.identity_rut_claim WHERE fingerprint=$1 FOR UPDATE`
)

var _ verification.RotatingRutClaimStore = (*RutClaimStore)(nil)

func (s *RutClaimStore) ClaimCandidates(ctx context.Context, accountID, declarationID string, set verification.RutFingerprintSet, expiresAt, now time.Time) (string, error) {
	return s.publicClaimCandidates(ctx, accountID, declarationID, set, expiresAt, now, true)
}
func (s *RutClaimStore) publicClaimCandidates(ctx context.Context, accountID, declarationID string, set verification.RutFingerprintSet, expiresAt, now time.Time, rebindVerified bool) (string, error) {
	if s == nil || s.pool == nil || !canonicalUUID(accountID) || !canonicalUUID(declarationID) || verification.ValidateRutFingerprintSet(set) != nil || now.IsZero() || !expiresAt.After(now) {
		return "", verification.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	canonical, err := claimRutCandidatesInTx(ctx, tx, accountID, declarationID, set, expiresAt, now, rebindVerified)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", rutClaimDBError(err)
	}
	return canonical, nil
}

func claimRutCandidatesInTx(ctx context.Context, tx rutClaimTx, accountID, declarationID string, set verification.RutFingerprintSet, expiresAt, now time.Time, rebindVerified bool) (string, error) {
	if tx == nil || !canonicalUUID(accountID) || !canonicalUUID(declarationID) || verification.ValidateRutFingerprintSet(set) != nil || now.IsZero() || !expiresAt.After(now) {
		return "", verification.ErrInvalid
	}
	fp, claim, err := readCanonicalRutClaim(ctx, tx, accountID, set)
	if err != nil {
		return "", err
	}
	if claim != nil && claim.verified == nil {
		if claim.expires == nil {
			return "", verification.ErrConflict
		}
		if !claim.expires.After(now) {
			tag, err := tx.Exec(ctx, `DELETE FROM public.identity_rut_claim WHERE fingerprint=$1 AND verified_at IS NULL AND expires_at <= $2`, fp, now)
			if err != nil {
				return "", err
			}
			if tag.RowsAffected() != 1 {
				return "", verification.ErrConflict
			}
			claim = nil
		}
	}
	if claim != nil {
		if claim.owner != accountID {
			return "", verification.ErrConflict
		}
		if claim.verified != nil {
			if claim.expires != nil {
				return "", verification.ErrConflict
			}
			if rebindVerified && claim.declaration != declarationID {
				tag, err := tx.Exec(ctx, `UPDATE public.identity_rut_claim SET declaration_id=$2,updated_at=GREATEST(updated_at,$3) WHERE fingerprint=$1 AND account_id=$4 AND verified_at IS NOT NULL AND expires_at IS NULL`, fp, declarationID, now, accountID)
				if err != nil {
					return "", rutClaimDBError(err)
				}
				if tag.RowsAffected() != 1 {
					return "", verification.ErrConflict
				}
			}
			return fp, nil
		}
		tag, err := tx.Exec(ctx, `UPDATE public.identity_rut_claim SET declaration_id=$2,expires_at=$3,updated_at=GREATEST(updated_at,$4) WHERE fingerprint=$1 AND account_id=$5 AND verified_at IS NULL`, fp, declarationID, expiresAt, now, accountID)
		if err != nil {
			return "", rutClaimDBError(err)
		}
		if tag.RowsAffected() != 1 {
			return "", verification.ErrConflict
		}
		return fp, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.identity_rut_claim WHERE account_id=$1 AND verified_at IS NULL`, accountID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.identity_rut_claim(fingerprint,account_id,declaration_id,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)`, set.Active, accountID, declarationID, expiresAt, now); err != nil {
		return "", rutClaimDBError(err)
	}
	return set.Active, nil
}

type canonicalRutClaim struct {
	owner, declaration string
	verified, expires  *time.Time
}

// Complete the entire read phase before mutating any claim. In particular a
// legacy match cannot hide another matching version or a later query failure.
func readCanonicalRutClaim(ctx context.Context, tx rutClaimTx, accountID string, set verification.RutFingerprintSet) (string, *canonicalRutClaim, error) {
	if _, err := tx.Exec(ctx, rutClaimAccountLockSQL, accountID); err != nil {
		return "", nil, err
	}
	if _, err := tx.Exec(ctx, rutClaimGlobalLockSQL, rutClaimLockKeyset); err != nil {
		return "", nil, err
	}
	versions := make([]string, 0, len(set.Candidates))
	for _, fp := range set.Candidates {
		versions = append(versions, strings.SplitN(fp, ":", 2)[0])
	}
	var unknown bool
	if err := tx.QueryRow(ctx, rutClaimUnknownVersionSQL, versions).Scan(&unknown); err != nil {
		return "", nil, err
	}
	if unknown {
		return "", nil, verification.ErrRutHMACKeyUnavailable
	}
	var found *canonicalRutClaim
	canonical := ""
	for _, fp := range set.Candidates {
		var claim canonicalRutClaim
		err := tx.QueryRow(ctx, rutClaimLookupSQL, fp).Scan(&claim.owner, &claim.declaration, &claim.verified, &claim.expires)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if found != nil {
			return "", nil, verification.ErrConflict
		}
		canonical, found = fp, &claim
	}
	return canonical, found, nil
}

func (s *RutClaimStore) ResolveCandidates(ctx context.Context, accountID, declarationID string, set verification.RutFingerprintSet) (string, error) {
	if s == nil || s.pool == nil || !canonicalUUID(accountID) || !canonicalUUID(declarationID) || verification.ValidateRutFingerprintSet(set) != nil {
		return "", verification.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	found, claim, err := readCanonicalRutClaim(ctx, tx, accountID, set)
	if err != nil {
		return "", err
	}
	if claim == nil || claim.owner != accountID || claim.declaration != declarationID {
		return "", verification.ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return "", rutClaimDBError(err)
	}
	return found, nil
}
