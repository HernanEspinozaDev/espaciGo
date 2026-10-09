// Package documents owns private local document metadata and bytes (M09).
// Callers decide authorization and provide their transaction for atomic writes.
package documents

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func InsertPDF(ctx context.Context, tx pgx.Tx, reservationID string, data []byte, createdAt time.Time) (string, error) {
	if len(data) == 0 || len(data) > 2<<20 {
		return "", errors.New("invalid private PDF size")
	}
	sha := sha256.Sum256(data)
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO public.documento_privado_sintetico_local(id,reserva_id,mime_type,encryption_algorithm,sha256,size_bytes,contenido,creada_en) VALUES(gen_random_uuid(),$1,'application/pdf','aes-256-gcm',$2,$3,$4,$5) RETURNING id::text`, reservationID, sha[:], len(data), data, createdAt).Scan(&id)
	return id, err
}

func ReadPDF(ctx context.Context, pool *pgxpool.Pool, documentID string) ([]byte, error) {
	var data []byte
	err := pool.QueryRow(ctx, `SELECT contenido FROM public.documento_privado_sintetico_local WHERE id=$1`, documentID).Scan(&data)
	return data, err
}
