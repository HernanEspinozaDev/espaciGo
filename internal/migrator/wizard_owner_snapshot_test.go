package migrator

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type ownerRows struct {
	rows [][]any
	i    int
	err  error
}

func (r *ownerRows) Close()                                       {}
func (r *ownerRows) Err() error                                   { return r.err }
func (r *ownerRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *ownerRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *ownerRows) Next() bool {
	if r.i >= len(r.rows) {
		return false
	}
	r.i++
	return true
}
func (r *ownerRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	if len(dest) != len(row) {
		return errors.New("scan arity")
	}
	for i, v := range row {
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case *int64:
			*d = v.(int64)
		case *bool:
			*d = v.(bool)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
func (r *ownerRows) Values() ([]any, error) { return nil, io.EOF }
func (r *ownerRows) RawValues() [][]byte    { return nil }
func (r *ownerRows) Conn() *pgx.Conn        { return nil }
func (r *ownerRows) TypeMap() *pgtype.Map   { return pgtype.NewMap() }

type ownerQuery struct {
	rows pgx.Rows
	err  error
}

func (q ownerQuery) Query(context.Context, string, ...any) (pgx.Rows, error) { return q.rows, q.err }

func TestWizardOwnerSnapshotCapture(t *testing.T) {
	rows := &ownerRows{rows: [][]any{{"pg_class", int64(1), int64(20), int64(0), false}, {"pg_proc", int64(2), int64(21), int64(0), true}}}
	got, err := captureWizardOwnerSnapshot(context.Background(), ownerQuery{rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[wizardObjectKey{"pg_proc", 2}] != (wizardOwnerState{owner: 21, securityDefiner: true}) {
		t.Fatalf("unexpected snapshot: %#v", got)
	}
	if _, err := captureWizardOwnerSnapshot(context.Background(), ownerQuery{err: errors.New("query failed")}); err == nil {
		t.Fatal("expected query error")
	}
	if _, err := captureWizardOwnerSnapshot(context.Background(), ownerQuery{rows: &ownerRows{rows: [][]any{{"pg_class", int64(1), int64(20), int64(0), false}}, err: errors.New("iteration failed")}}); err == nil {
		t.Fatal("expected rows.Err failure")
	}
}

func TestWizardOwnerSnapshotVerify(t *testing.T) {
	tbl, idx := wizardObjectKey{"pg_class", 10}, wizardObjectKey{"pg_class", 11}
	f1, f2, f3 := wizardObjectKey{"pg_proc", 20}, wizardObjectKey{"pg_proc", 21}, wizardObjectKey{"pg_proc", 22}
	base := wizardOwnerSnapshot{{"pg_class", 1}: {owner: 7}, {"pg_proc", 2}: {owner: 8}}
	beforeV60 := wizardOwnerSnapshot{}
	afterV60 := wizardOwnerSnapshot{{"pg_proc", 60}: {owner: 99, securityDefiner: true}}
	if err := verifyWizardOwnerSnapshot(beforeV60, afterV60, map[wizardObjectKey]bool{{"pg_proc", 60}: true}, 99); err != nil {
		t.Fatalf("V60 function: %v", err)
	}
	before := base
	after := wizardOwnerSnapshot{{"pg_class", 1}: {owner: 7}, {"pg_proc", 2}: {owner: 8}, tbl: {owner: 99}, idx: {owner: 99, parentTable: 10}, f1: {owner: 99, securityDefiner: true}, f2: {owner: 99}, f3: {owner: 99, securityDefiner: true}}
	expected := map[wizardObjectKey]bool{tbl: false, f1: true, f2: false, f3: true}
	if err := verifyWizardOwnerSnapshot(before, after, expected, 99); err != nil {
		t.Fatalf("V66 table/index/functions: %v", err)
	}

	clone := func() wizardOwnerSnapshot {
		out := wizardOwnerSnapshot{}
		for k, v := range after {
			out[k] = v
		}
		return out
	}
	cases := []struct {
		name string
		b, a wizardOwnerSnapshot
		e    map[wizardObjectKey]bool
	}{
		{"missing prior", before, wizardOwnerSnapshot{}, expected},
		{"old owner changed", before, func() wizardOwnerSnapshot {
			a := clone()
			s := a[wizardObjectKey{"pg_class", 1}]
			s.owner++
			a[wizardObjectKey{"pg_class", 1}] = s
			return a
		}(), expected},
		{"old definer changed", wizardOwnerSnapshot{{"pg_proc", 2}: {owner: 8, securityDefiner: true}}, func() wizardOwnerSnapshot {
			a := clone()
			s := a[wizardObjectKey{"pg_proc", 2}]
			s.securityDefiner = true
			a[wizardObjectKey{"pg_proc", 2}] = s
			return a
		}(), expected},
		{"missing expected", before, wizardOwnerSnapshot{{"pg_class", 1}: {owner: 7}, {"pg_proc", 2}: {owner: 8}}, expected},
		{"expected existed before", after, after, expected},
		{"wrong owner", before, func() wizardOwnerSnapshot { a := clone(); s := a[tbl]; s.owner = 1; a[tbl] = s; return a }(), expected},
		{"wrong definer", before, func() wizardOwnerSnapshot { a := clone(); s := a[f1]; s.securityDefiner = false; a[f1] = s; return a }(), expected},
		{"unexpected object", before, func() wizardOwnerSnapshot {
			a := clone()
			a[wizardObjectKey{"pg_proc", 30}] = wizardOwnerState{owner: 99}
			return a
		}(), expected},
		{"orphan index", before, func() wizardOwnerSnapshot { a := clone(); s := a[idx]; s.parentTable = 777; a[idx] = s; return a }(), expected},
		{"unallowed index parent", before, func() wizardOwnerSnapshot {
			a := clone()
			a[wizardObjectKey{"pg_class", 12}] = wizardOwnerState{owner: 99, parentTable: 1}
			return a
		}(), expected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyWizardOwnerSnapshot(tc.b, tc.a, tc.e, 99); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
