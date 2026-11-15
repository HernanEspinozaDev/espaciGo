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

type rotationBehaviorRow struct {
	owner, decl       string
	verified, expires *time.Time
	err               error
}

func (r rotationBehaviorRow) Scan(v ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(v) != 4 {
		return errors.New("unexpected candidate row shape")
	}
	*v[0].(*string) = r.owner
	*v[1].(*string) = r.decl
	*v[2].(**time.Time) = r.verified
	*v[3].(**time.Time) = r.expires
	return nil
}

type rotationBehaviorBool struct {
	unknown bool
	err     error
}

func (r rotationBehaviorBool) Scan(v ...any) error {
	if r.err != nil {
		return r.err
	}
	*v[0].(*bool) = r.unknown
	return nil
}

type rotationBehaviorTx struct {
	rows               map[string]rotationBehaviorRow
	unknown            bool
	versionErr         error
	queries            int
	log                []string
	writes             []string
	args               [][]any
	updateRows         int64
	commits, rollbacks int
}

func (f *rotationBehaviorTx) QueryRow(_ context.Context, q string, args ...any) pgx.Row {
	if strings.Contains(q, "split_part(fingerprint") {
		f.log = append(f.log, "versions")
		return rotationBehaviorBool{f.unknown, f.versionErr}
	}
	if strings.Contains(q, "WHERE fingerprint=$1 FOR UPDATE") {
		f.queries++
		fp := args[0].(string)
		f.log = append(f.log, "lookup")
		if r, ok := f.rows[fp]; ok {
			return r
		}
		return rotationBehaviorRow{err: pgx.ErrNoRows}
	}
	return rotationBehaviorRow{err: errors.New("unexpected rotation query")}
}
func (f *rotationBehaviorTx) Exec(_ context.Context, q string, args ...any) (pgconn.CommandTag, error) {
	if q == `SELECT pg_advisory_xact_lock(hashtextextended($1,0))` {
		tag := "account-advisory"
		if len(args) == 1 && args[0] == "espacigo:rut-hmac-claim-keyset:v1" {
			tag = "global-advisory"
		}
		f.log = append(f.log, tag)
		return pgconn.NewCommandTag("SELECT 1"), nil
	}
	if strings.HasPrefix(q, "SELECT id FROM public.usuario") {
		f.log = append(f.log, "account-row")
		return pgconn.NewCommandTag("SELECT 1"), nil
	}
	f.writes = append(f.writes, q)
	f.args = append(f.args, args)
	switch {
	case strings.HasPrefix(q, "UPDATE "):
		return pgconn.NewCommandTag("UPDATE " + map[bool]string{true: "0", false: "1"}[f.updateRows == 0]), nil
	case strings.HasPrefix(q, "INSERT "):
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.HasPrefix(q, "DELETE "):
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	return pgconn.CommandTag{}, errors.New("unexpected rotation statement")
}
func (f *rotationBehaviorTx) Commit(context.Context) error   { f.commits++; return nil }
func (f *rotationBehaviorTx) Rollback(context.Context) error { f.rollbacks++; return nil }

func TestRotatingClaimsResolveAllCandidatesBeforeMutation(t *testing.T) {
	const account = "85000000-0000-4000-8000-000000000001"
	const decl = "85000000-0000-4000-8000-000000000002"
	now := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	verified := now.Add(-time.Hour)
	expires := now.Add(time.Hour)
	legacy := "v1:" + strings.Repeat("a", 64)
	active := "v2:" + strings.Repeat("b", 64)
	set := verification.RutFingerprintSet{Active: active, Candidates: []string{legacy, active}}
	failure := errors.New("synthetic lookup failure")
	for _, tc := range []struct {
		name    string
		rows    map[string]rotationBehaviorRow
		unknown bool
		wantErr error
		want    string
	}{
		{name: "new active", rows: map[string]rotationBehaviorRow{}, want: active},
		{name: "legacy canonical", rows: map[string]rotationBehaviorRow{legacy: {owner: account, decl: decl, verified: &verified}}, want: legacy},
		{name: "two versions reject", rows: map[string]rotationBehaviorRow{legacy: {owner: account, decl: decl, expires: &expires}, active: {owner: account, decl: decl, expires: &expires}}, wantErr: verification.ErrConflict},
		{name: "late lookup error aborts", rows: map[string]rotationBehaviorRow{legacy: {owner: account, decl: decl, verified: &verified}, active: {err: failure}}, wantErr: failure},
		{name: "missing retained key", rows: map[string]rotationBehaviorRow{}, unknown: true, wantErr: verification.ErrRutHMACKeyUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &rotationBehaviorTx{rows: tc.rows, unknown: tc.unknown, updateRows: 1}
			got, err := claimRutCandidatesInTx(context.Background(), f, account, decl, set, expires, now, true)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("canonical=%q err=%v wanted=%q/%v", got, err, tc.want, tc.wantErr)
			}
			if len(f.log) < 3 || f.log[0] != "account-advisory" || f.log[1] != "global-advisory" || f.log[2] != "versions" {
				t.Fatalf("wrong literal account/global lock protocol: %v", f.log)
			}
			if tc.wantErr != nil && len(f.writes) != 0 {
				t.Fatal("claim resolution failure made writes")
			}
			if !tc.unknown && f.queries != 2 {
				t.Fatalf("checked %d candidates instead of both", f.queries)
			}
			if f.commits != 0 || f.rollbacks != 0 {
				t.Fatal("nested helper took over its caller transaction")
			}
		})
	}
}

func TestRotatingClaimsPreserveHistoricalMarkerOnRebind(t *testing.T) {
	const account = "85000000-0000-4000-8000-000000000001"
	const decl = "85000000-0000-4000-8000-000000000002"
	const oldDecl = "85000000-0000-4000-8000-000000000003"
	now := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	verified := now.Add(-time.Hour)
	legacy := "v1:" + strings.Repeat("a", 64)
	active := "v2:" + strings.Repeat("b", 64)
	set := verification.RutFingerprintSet{Active: active, Candidates: []string{legacy, active}}
	for _, bind := range []bool{false, true} {
		f := &rotationBehaviorTx{rows: map[string]rotationBehaviorRow{legacy: {owner: account, decl: oldDecl, verified: &verified}}, updateRows: 1}
		got, err := claimRutCandidatesInTx(context.Background(), f, account, decl, set, now.Add(time.Hour), now, bind)
		if err != nil || got != legacy {
			t.Fatalf("same-account historical claim rejected: %v", err)
		}
		updates := 0
		for _, q := range f.writes {
			if strings.HasPrefix(q, "UPDATE ") {
				updates++
				if strings.Contains(q, "verified_at=NULL") || strings.Contains(q, "expires_at=") {
					t.Fatal("historical reservation downgraded to expiring pending")
				}
			}
		}
		if bind && updates != 1 || !bind && updates != 0 {
			t.Fatal("save/start historical rebinding semantics differ from contract")
		}
	}
}
