package spaceversion

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Bump advances the shared listing version for any owner or administrative
// mutation of publication content, gallery, or weekly hours. The row must be
// locked by the caller before changing its associated resource.
func Bump(ctx context.Context, tx pgx.Tx, spaceID string) (time.Time, error) {
	var updated time.Time
	err := tx.QueryRow(ctx, `UPDATE public.espacio
		SET actualizado_en=GREATEST(clock_timestamp(), actualizado_en+interval '1 microsecond')
		WHERE id=$1
		RETURNING actualizado_en`, spaceID).Scan(&updated)
	return updated, err
}
