package accountlock

import (
	"context"
	"errors"
	"sort"

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

// LockAccountIDs serializes account-scoped transactions in the same stable
// order used by commercial quotes and privacy suppression. Call it before
// taking account or domain-row locks.
func LockAccountIDs(ctx context.Context, tx pgx.Tx, accountIDs ...string) error {
	ids := append([]string(nil), accountIDs...)
	sort.Strings(ids)
	previous := ""
	for _, id := range ids {
		if id == "" {
			return errors.New("accountlock: empty account id")
		}
		if id == previous {
			continue
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, id); err != nil {
			return err
		}
		previous = id
	}
	return nil
}

// LockActive takes the shared account advisory lock before the row lock, then
// returns active=false when the account is absent or already retired.
func LockActive(ctx context.Context, tx pgx.Tx, accountID string) (bool, error) {
	if err := LockAccountIDs(ctx, tx, accountID); err != nil {
		return false, err
	}
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
