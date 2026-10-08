package accountlock

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// LockActive serializes owner writes with privacy suppression and returns
// active=false when the account is absent or already retired. Callers must
// acquire this lock before locking domain rows or writing owner-scoped data.
func LockActive(ctx context.Context, tx pgx.Tx, accountID string) (bool, error) {
	var state string
	err := tx.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE`, accountID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return state == "activo", nil
}
