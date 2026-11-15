package identitydeclaration

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RevisionInput = verification.DeclarationRevisionInput
type Revision = verification.DeclarationRevision
type Sealer = verification.DeclarationSealer
type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var keyVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func canonicalUUID(s string) bool { id, err := uuid.Parse(s); return err == nil && id.String() == s }
func validInput(in RevisionInput, seal Sealer) bool {
	if !canonicalUUID(in.ID) || (in.RutFingerprint != "" && !validRutClaimInput(in.AccountID, in.ID, in.RutFingerprint)) || !canonicalUUID(in.AccountID) || !utf8.ValidString(in.CorrelationID) || utf8.RuneCountInString(in.CorrelationID) < 1 || utf8.RuneCountInString(in.CorrelationID) > 120 || strings.TrimSpace(in.CorrelationID) != in.CorrelationID || in.SchemaVersion != "1" || len(in.ConsentVersion) < 1 || len(in.ConsentVersion) > 32 || seal == nil || in.ConsentGivenAt.IsZero() || in.RetentionAt.IsZero() || !in.RetentionAt.After(in.ConsentGivenAt) {
		return false
	}
	if len(in.RutFingerprintCandidates) != 0 {
		return verification.ValidateRutFingerprintSet(verification.RutFingerprintSet{Active: in.RutFingerprint, Candidates: in.RutFingerprintCandidates}) == nil
	}
	return true
}
func safeDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return verification.ErrConflict
		}
		if pgErr.Code[:2] == "23" {
			return verification.ErrInvalid
		}
	}
	return err
}
func (r *Repository) PurgeExpiredRevisions(ctx context.Context) (int64, error) {
	if r == nil || r.pool == nil {
		return 0, verification.ErrInvalid
	}
	var deleted int64
	err := r.pool.QueryRow(ctx, `SELECT public.purge_expired_identity_declaration_revisions()`).Scan(&deleted)
	return deleted, err
}

func (r *Repository) PurgeDeliveredSessionOutbox(ctx context.Context) (int64, error) {
	if r == nil || r.pool == nil {
		return 0, verification.ErrInvalid
	}
	var deleted int64
	err := r.pool.QueryRow(ctx, `SELECT public.purge_delivered_identity_session_outbox()`).Scan(&deleted)
	return deleted, err
}

func claimRutForRevision(ctx context.Context, tx pgx.Tx, accountID, declarationID, fingerprint string, expiresAt, now time.Time) error {
	var owner, declaration string
	var verifiedAt *time.Time
	err := tx.QueryRow(ctx, `SELECT account_id::text,COALESCE(declaration_id::text,''),verified_at FROM public.identity_rut_claim WHERE fingerprint=$1 FOR UPDATE`, fingerprint).Scan(&owner, &declaration, &verifiedAt)
	if err == nil {
		if owner != accountID {
			return verification.ErrConflict
		}
		if verifiedAt != nil {
			return nil
		}
		tag, execErr := tx.Exec(ctx, `UPDATE public.identity_rut_claim SET declaration_id=$2,expires_at=$3,updated_at=$4 WHERE fingerprint=$1 AND account_id=$5 AND verified_at IS NULL`, fingerprint, declarationID, expiresAt, now, accountID)
		if execErr != nil {
			return execErr
		}
		if tag.RowsAffected() != 1 {
			return verification.ErrConflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// The per-account unique key protects against silently replacing a different claim.
	_, err = tx.Exec(ctx, `INSERT INTO public.identity_rut_claim(fingerprint,account_id,declaration_id,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)`, fingerprint, accountID, declarationID, expiresAt, now)
	return err
}

func createRevisionInTx(ctx context.Context, tx pgx.Tx, in RevisionInput, seal Sealer) (Revision, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, in.AccountID); err != nil {
		return Revision{}, err
	}
	var next int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision),0)+1 FROM public.identity_declaration_revision WHERE account_id=$1`, in.AccountID).Scan(&next); err != nil {
		return Revision{}, err
	}
	keyVersion, ciphertext, err := seal(next)
	if err != nil {
		return Revision{}, err
	}
	if !keyVersionPattern.MatchString(keyVersion) || len(ciphertext) < 29 || len(ciphertext) > 65564 {
		return Revision{}, verification.ErrInvalid
	}
	var out Revision
	err = tx.QueryRow(ctx, `INSERT INTO public.identity_declaration_revision (id,account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text,account_id::text,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at`, in.ID, in.AccountID, next, keyVersion, ciphertext, in.SchemaVersion, in.ConsentVersion, in.ConsentGivenAt, in.RetentionAt).Scan(&out.ID, &out.AccountID, &out.Revision, &out.KeyVersion, &out.Ciphertext, &out.SchemaVersion, &out.ConsentVersion, &out.ConsentGivenAt, &out.RetentionAt, &out.CreatedAt)
	if err != nil {
		return Revision{}, safeDBError(err)
	}
	if in.RutFingerprint != "" {
		if len(in.RutFingerprintCandidates) > 0 {
			_, err = claimRutCandidatesInTx(ctx, tx, in.AccountID, in.ID, verification.RutFingerprintSet{Active: in.RutFingerprint, Candidates: in.RutFingerprintCandidates}, in.RetentionAt, in.ConsentGivenAt, false)
		} else {
			err = claimRutForRevision(ctx, tx, in.AccountID, in.ID, in.RutFingerprint, in.RetentionAt, in.ConsentGivenAt)
		}
		if err != nil {
			return Revision{}, safeDBError(err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en,detalle_codigos) VALUES(gen_random_uuid(),$1,'identity_declaration',$2,'kyc.identity_declaration.save','exito','consent_accepted',$3,$4,$4::timestamptz+interval '5 years','{"obligations_detected":[],"pending_checks":[]}'::jsonb)`, in.AccountID, in.ID, in.CorrelationID, in.ConsentGivenAt)
	if err != nil {
		return Revision{}, safeDBError(err)
	}
	return out, nil
}

func (r *Repository) CreateRevision(ctx context.Context, in RevisionInput, seal Sealer) (Revision, error) {
	if !validInput(in, seal) {
		return Revision{}, verification.ErrInvalid
	}
	if r == nil || r.pool == nil {
		return Revision{}, verification.ErrInvalid
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Revision{}, err
	}
	defer tx.Rollback(ctx)
	out, err := createRevisionInTx(ctx, tx, in, seal)
	if err != nil {
		return Revision{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Revision{}, err
	}
	return out, nil
}
func (r *Repository) GetRevision(ctx context.Context, accountID, id string) (Revision, error) {
	if !canonicalUUID(accountID) || !canonicalUUID(id) || r == nil || r.pool == nil {
		return Revision{}, verification.ErrInvalid
	}
	var out Revision
	err := r.pool.QueryRow(ctx, `SELECT id::text,account_id::text,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at FROM public.identity_declaration_revision WHERE account_id=$1 AND id=$2`, accountID, id).Scan(&out.ID, &out.AccountID, &out.Revision, &out.KeyVersion, &out.Ciphertext, &out.SchemaVersion, &out.ConsentVersion, &out.ConsentGivenAt, &out.RetentionAt, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, verification.ErrNotFound
	}
	if err != nil {
		return Revision{}, safeDBError(err)
	}
	return out, nil
}

func (r *Repository) GetCurrent(ctx context.Context, accountID string) (Revision, error) {
	if !canonicalUUID(accountID) || r == nil || r.pool == nil {
		return Revision{}, verification.ErrInvalid
	}
	var out Revision
	err := r.pool.QueryRow(ctx, `SELECT id::text,account_id::text,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at FROM public.identity_declaration_revision WHERE account_id=$1 ORDER BY revision DESC LIMIT 1`, accountID).Scan(&out.ID, &out.AccountID, &out.Revision, &out.KeyVersion, &out.Ciphertext, &out.SchemaVersion, &out.ConsentVersion, &out.ConsentGivenAt, &out.RetentionAt, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, verification.ErrNotFound
	}
	if err != nil {
		return Revision{}, safeDBError(err)
	}
	return out, nil
}
