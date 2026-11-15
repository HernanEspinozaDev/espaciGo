package identitydeclaration

import (
	"context"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type rotationRow struct {
	owner, declaration string
	verified, expires  *time.Time
	err                error
}

func (r rotationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 4 {
		panic("unexpected rotation scan")
	}
	*dest[0].(*string) = r.owner
	*dest[1].(*string) = r.declaration
	*dest[2].(**time.Time) = r.verified
	*dest[3].(**time.Time) = r.expires
	return nil
}

type rotationRowBool struct{ value bool }

func (r rotationRowBool) Scan(dest ...any) error { *dest[0].(*bool) = r.value; return nil }

type rotationFake struct {
	log        []string
	row        rotationRow
	updateRows int64
}

func (f *rotationFake) Commit(context.Context) error   { return nil }
func (f *rotationFake) Rollback(context.Context) error { return nil }

func (f *rotationFake) Exec(_ context.Context, q string, _ ...any) (pgconn.CommandTag, error) {
	f.log = append(f.log, q)
	switch {
	case q == rutClaimGlobalLockSQL:
		return pgconn.NewCommandTag("SELECT 1"), nil
	case q == rutClaimAccountLockSQL:
		return pgconn.NewCommandTag("SELECT 1"), nil
	case q == rutClaimUnknownVersionSQL:
		return pgconn.NewCommandTag("SELECT 1"), nil
	case q[:min(len(q), 7)] == "UPDATE ":
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case q[:min(len(q), 7)] == "DELETE ":
		return pgconn.NewCommandTag("DELETE 0"), nil
	case q[:min(len(q), 7)] == "INSERT ":
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}
	panic("unexpected SQL")
}
func (f *rotationFake) QueryRow(_ context.Context, q string, _ ...any) pgx.Row {
	f.log = append(f.log, q)
	if q == rutClaimUnknownVersionSQL {
		return rotationRowBool{value: false}
	}
	return f.row
}

func TestClaimRutCandidatesHelperHasNoTransactionBoundary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	set := verification.RutFingerprintSet{Active: "v2:" + stringsRepeat("b", 64), Candidates: []string{"v1:" + stringsRepeat("a", 64), "v2:" + stringsRepeat("b", 64)}}
	f := &rotationFake{row: rotationRow{err: pgx.ErrNoRows}}
	_, err := claimRutCandidatesInTx(ctx, f, "85000000-0000-4000-8000-000000000001", "85000000-0000-4000-8000-000000000002", set, now.Add(time.Hour), now, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.log) < 3 || f.log[0] != rutClaimAccountLockSQL || f.log[1] != rutClaimGlobalLockSQL {
		t.Fatalf("lock order: %#v", f.log)
	}
}
