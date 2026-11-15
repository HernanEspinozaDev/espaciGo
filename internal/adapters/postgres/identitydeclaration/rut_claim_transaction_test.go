package identitydeclaration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type claimDecisionFake struct {
	lookupErr                   error
	owner                       string
	declaration                 string
	verified                    *time.Time
	insertErr                   error
	execs                       []string
	queries, commits, rollbacks int
}
type claimDecisionRow struct{ tx *claimDecisionFake }

func (r claimDecisionRow) Scan(dest ...any) error {
	if r.tx.lookupErr != nil {
		return r.tx.lookupErr
	}
	if len(dest) != 3 {
		return errors.New("unexpected scan shape")
	}
	*dest[0].(*string) = r.tx.owner
	*dest[1].(*string) = r.tx.declaration
	*dest[2].(**time.Time) = r.tx.verified
	return nil
}
func (f *claimDecisionFake) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if !strings.HasPrefix(sql, "SELECT account_id::text,COALESCE(declaration_id::text,''),verified_at FROM public.identity_rut_claim WHERE fingerprint=$1 FOR UPDATE") {
		panic("unexpected claim query")
	}
	f.queries++
	return claimDecisionRow{f}
}
func (f *claimDecisionFake) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.execs = append(f.execs, sql)
	switch {
	case strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock("):
		return pgconn.NewCommandTag("SELECT 1"), nil
	case strings.HasPrefix(sql, "DELETE FROM public.identity_rut_claim WHERE verified_at"), strings.HasPrefix(sql, "DELETE FROM public.identity_rut_claim WHERE account_id"):
		return pgconn.NewCommandTag("DELETE 0"), nil
	case strings.HasPrefix(sql, "INSERT INTO public.identity_rut_claim("):
		return pgconn.NewCommandTag("INSERT 0 1"), f.insertErr
	case strings.HasPrefix(sql, "UPDATE public.identity_rut_claim SET declaration_id"):
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		panic("unexpected claim statement")
	}
}
func (f *claimDecisionFake) Commit(context.Context) error   { f.commits++; return nil }
func (f *claimDecisionFake) Rollback(context.Context) error { f.rollbacks++; return nil }
func (f *claimDecisionFake) count(prefix string) int {
	n := 0
	for _, q := range f.execs {
		if strings.HasPrefix(q, prefix) {
			n++
		}
	}
	return n
}

func TestClaimRutInTxSelectResultControlsInsertAndRollback(t *testing.T) {
	account := "85000000-0000-4000-8000-000000000001"
	declaration := "85000000-0000-4000-8000-000000000002"
	fingerprint := "v1:" + strings.Repeat("a", 64)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	lookupFailure := errors.New("lookup unavailable")
	verified := now
	for _, tc := range []struct {
		name                      string
		tx                        claimDecisionFake
		wantErr                   error
		inserts, updates, commits int
	}{
		{"missing claim inserts", claimDecisionFake{lookupErr: pgx.ErrNoRows}, nil, 1, 0, 1},
		{"lookup error aborts", claimDecisionFake{lookupErr: lookupFailure}, lookupFailure, 0, 0, 0},
		{"owned pending updates", claimDecisionFake{owner: account}, nil, 0, 1, 1},
		{"owned verified unchanged", claimDecisionFake{owner: account, declaration: declaration, verified: &verified}, nil, 0, 0, 1},
		{"detached verified claim binds new revision retaining historical marker", claimDecisionFake{owner: account, verified: &verified}, nil, 0, 1, 1},
		{"other owner conflicts", claimDecisionFake{owner: "85000000-0000-4000-8000-000000000003"}, verification.ErrConflict, 0, 0, 0},
		{"race duplicate conflicts", claimDecisionFake{lookupErr: pgx.ErrNoRows, insertErr: &pgconn.PgError{Code: "23505"}}, verification.ErrConflict, 1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.tx
			err := claimRutInTx(context.Background(), &f, account, declaration, fingerprint, now.Add(time.Hour), now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v want=%v", err, tc.wantErr)
			}
			if f.count("INSERT") != tc.inserts || f.count("UPDATE") != tc.updates || f.commits != tc.commits {
				t.Fatalf("inserts=%d updates=%d commits=%d want=%d/%d/%d", f.count("INSERT"), f.count("UPDATE"), f.commits, tc.inserts, tc.updates, tc.commits)
			}
			if f.rollbacks == 0 {
				t.Fatal("transaction cleanup was not registered")
			}
		})
	}
}

func TestLegacyClaimRebindPreservesVerifiedReservation(t *testing.T) {
	now := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	verified := now.Add(-time.Hour)
	f := &claimDecisionFake{owner: "85000000-0000-4000-8000-000000000001", verified: &verified}
	err := claimRutInTx(context.Background(), f, f.owner, "85000000-0000-4000-8000-000000000002", "v1:"+strings.Repeat("a", 64), now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range f.execs {
		if strings.HasPrefix(query, "UPDATE ") && (strings.Contains(query, "verified_at=NULL") || strings.Contains(query, "expires_at=")) {
			t.Fatal("reverification converted a historical reservation into an expiring pending claim")
		}
	}
}

type markVerifiedFake struct {
	updateErr error
	rows      int64
	lookupErr error
	owner     string
	decl      string
	verified  *time.Time
}
type markVerifiedRow struct{ fake *markVerifiedFake }

func (r markVerifiedRow) Scan(dest ...any) error {
	if r.fake.lookupErr != nil {
		return r.fake.lookupErr
	}
	if len(dest) != 3 {
		return errors.New("unexpected scan shape")
	}
	*dest[0].(*string) = r.fake.owner
	*dest[1].(*string) = r.fake.decl
	*dest[2].(**time.Time) = r.fake.verified
	return nil
}
func (f *markVerifiedFake) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(strings.TrimSpace(strings.Join([]string{"UPDATE", ""}, " ")) + " " + strings.TrimSpace(string(rune('0'+f.rows)))), f.updateErr
}
func (f *markVerifiedFake) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return markVerifiedRow{f}
}

func TestMarkVerifiedInDB(t *testing.T) {
	account := "85000000-0000-4000-8000-000000000001"
	declaration := "85000000-0000-4000-8000-000000000002"
	fingerprint := "v1:" + strings.Repeat("a", 64)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	verified := now
	for _, tc := range []struct {
		name    string
		fake    markVerifiedFake
		wantErr error
	}{
		{"normal update succeeds", markVerifiedFake{rows: 1}, nil},
		{"already verified succeeds idempotently", markVerifiedFake{rows: 0, owner: account, decl: declaration, verified: &verified}, nil},
		{"detached verified claim remains idempotent for same account", markVerifiedFake{rows: 0, owner: account, decl: "", verified: &verified}, nil},
		{"different owner returns conflict", markVerifiedFake{rows: 0, owner: "85000000-0000-4000-8000-000000000003", decl: declaration, verified: &verified}, verification.ErrConflict},
		{"unverified lookup returns conflict", markVerifiedFake{rows: 0, owner: account, decl: declaration, verified: nil}, verification.ErrConflict},
		{"lookup missing returns conflict", markVerifiedFake{rows: 0, lookupErr: pgx.ErrNoRows}, verification.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := markVerifiedInDB(context.Background(), &tc.fake, account, declaration, fingerprint, now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want=%v", err, tc.wantErr)
			}
		})
	}
}
