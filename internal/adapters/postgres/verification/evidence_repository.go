package verificationpg

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Repository) CreateEvidence(ctx context.Context, item verification.Evidence) (verification.Evidence, error) {
	var created verification.Evidence
	err := r.pool.QueryRow(ctx, `INSERT INTO public.verificacion_evidencia_sintetica
		(id, verificacion_id, codigo_fixture, mime_type, tamano_bytes, sha256, creada_en)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, verificacion_id::text, codigo_fixture, mime_type, tamano_bytes, sha256, creada_en`,
		item.ID, item.VerificationID, item.FixtureCode, item.MIMEType, item.SizeBytes, item.SHA256, dbTime(item.CreatedAt)).
		Scan(&created.ID, &created.VerificationID, &created.FixtureCode, &created.MIMEType, &created.SizeBytes, &created.SHA256, &created.CreatedAt)
	if err != nil {
		return verification.Evidence{}, mapEvidenceDBError(err)
	}
	return created, nil
}

func (r *Repository) ListOwnEvidence(ctx context.Context, owner, caseID string) ([]verification.Evidence, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.id::text, e.verificacion_id::text, e.codigo_fixture,
		e.mime_type, e.tamano_bytes, e.sha256, e.creada_en
		FROM public.verificacion_evidencia_sintetica e
		JOIN public.verificacion v ON v.id = e.verificacion_id
		WHERE v.usuario_id = $1 AND v.id = $2
		ORDER BY e.creada_en DESC, e.id`, owner, caseID)
	if err != nil {
		return nil, mapEvidenceDBError(err)
	}
	defer rows.Close()
	items := make([]verification.Evidence, 0)
	for rows.Next() {
		var item verification.Evidence
		if err := rows.Scan(&item.ID, &item.VerificationID, &item.FixtureCode, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListReviewEvidence(ctx context.Context, caseID string) ([]verification.Evidence, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.id::text, e.verificacion_id::text, e.codigo_fixture,
		e.mime_type, e.tamano_bytes, e.sha256, e.creada_en
		FROM public.verificacion_evidencia_sintetica e
		WHERE e.verificacion_id = $1
		ORDER BY e.creada_en DESC, e.id`, caseID)
	if err != nil {
		return nil, mapEvidenceDBError(err)
	}
	defer rows.Close()
	items := make([]verification.Evidence, 0)
	for rows.Next() {
		var item verification.Evidence
		if err := rows.Scan(&item.ID, &item.VerificationID, &item.FixtureCode, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetOwnEvidence(ctx context.Context, owner, caseID, evidenceID string) (verification.Evidence, error) {
	return r.getEvidence(ctx, `SELECT e.id::text, e.verificacion_id::text, e.codigo_fixture, e.mime_type, e.tamano_bytes, e.sha256, e.creada_en
		FROM public.verificacion_evidencia_sintetica e JOIN public.verificacion v ON v.id=e.verificacion_id
		WHERE v.usuario_id=$1 AND v.id=$2 AND e.id=$3`, owner, caseID, evidenceID)
}

func (r *Repository) GetReviewEvidence(ctx context.Context, caseID, evidenceID string) (verification.Evidence, error) {
	return r.getEvidence(ctx, `SELECT e.id::text, e.verificacion_id::text, e.codigo_fixture, e.mime_type, e.tamano_bytes, e.sha256, e.creada_en
		FROM public.verificacion_evidencia_sintetica e JOIN public.verificacion v ON v.id=e.verificacion_id
		WHERE v.id=$1 AND e.id=$2`, caseID, evidenceID)
}

func (r *Repository) getEvidence(ctx context.Context, query string, args ...any) (verification.Evidence, error) {
	var item verification.Evidence
	err := r.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.VerificationID, &item.FixtureCode, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.CreatedAt)
	if err != nil {
		return verification.Evidence{}, mapEvidenceDBError(err)
	}
	return item, nil
}

func (r *Repository) DeleteOwnEvidence(ctx context.Context, owner, caseID, evidenceID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.verificacion_evidencia_sintetica e USING public.verificacion v
		WHERE e.verificacion_id=v.id AND v.usuario_id=$1 AND v.id=$2 AND e.id=$3`, owner, caseID, evidenceID)
	return mapEvidenceDBError(err)
}

func (r *Repository) DeleteReviewEvidence(ctx context.Context, caseID, evidenceID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.verificacion_evidencia_sintetica e USING public.verificacion v
		WHERE e.verificacion_id=v.id AND v.id=$1 AND e.id=$2`, caseID, evidenceID)
	return mapEvidenceDBError(err)
}

func mapEvidenceDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return verification.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && (pgerr.Code == "23505" || pgerr.Code == "23514" || pgerr.Code == "23503") {
		return verification.ErrConflict
	}
	return err
}
