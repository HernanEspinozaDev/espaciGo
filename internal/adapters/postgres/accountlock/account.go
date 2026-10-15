package accountlock

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const globalAdministratorLockClass int32 = 0x4741444d  // "GADM"
const globalAdministratorLockObject int32 = 0x53455431 // "SET1"

// LockGlobalAdministratorSet serializes every transition that could remove or
// restore a global administrator, including suppression and administrative blocks.
func LockGlobalAdministratorSet(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1,$2)`, globalAdministratorLockClass, globalAdministratorLockObject)
	return err
}

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
