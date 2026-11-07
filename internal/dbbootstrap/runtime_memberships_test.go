package dbbootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type runtimeMembershipRows struct {
	rows [][]any
	at   int
}

func (r *runtimeMembershipRows) Close()                                       {}
func (r *runtimeMembershipRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *runtimeMembershipRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *runtimeMembershipRows) Err() error                                   { return nil }
func (r *runtimeMembershipRows) Next() bool {
	if r.at >= len(r.rows) {
		return false
	}
	r.at++
	return true
}
func (r *runtimeMembershipRows) Scan(dest ...any) error {
	if len(dest) != 5 {
		return errors.New("unexpected membership column count")
	}
	values := r.rows[r.at-1]
	for i, value := range values {
		switch out := dest[i].(type) {
		case *string:
			v, ok := value.(string)
			if !ok {
				return errors.New("unexpected string value")
			}
			*out = v
		case *bool:
			v, ok := value.(bool)
			if !ok {
				return errors.New("unexpected bool value")
			}
			*out = v
		default:
			return errors.New("unexpected membership destination")
		}
	}
	return nil
}
func (r *runtimeMembershipRows) Conn() *pgx.Conn        { return nil }
func (r *runtimeMembershipRows) Values() ([]any, error) { return nil, nil }
func (r *runtimeMembershipRows) RawValues() [][]byte    { return nil }
func (r *runtimeMembershipRows) TypeMap() *pgtype.Map   { return nil }

type runtimeMembershipRow struct {
	db  *runtimeMembershipDB
	sql string
}

func (r runtimeMembershipRow) Scan(dest ...any) error {
	switch {
	case strings.Contains(r.sql, "pg_roles WHERE rolname"):
		if len(dest) != 7 {
			return errors.New("unexpected role attribute column count")
		}
		values := r.db.attrs
		if values == nil {
			values = make([]bool, 7)
		}
		for i, d := range dest {
			out, ok := d.(*bool)
			if !ok {
				return errors.New("unexpected role attribute destination")
			}
			*out = values[i]
		}
		return nil
	case strings.Contains(r.sql, "SELECT EXISTS") && strings.Contains(r.sql, "pg_auth_members"):
		if len(dest) != 1 {
			return errors.New("unexpected membership existence column count")
		}
		out, ok := dest[0].(*bool)
		if !ok {
			return errors.New("unexpected membership existence destination")
		}
		*out = len(r.db.edges) > 0
		return nil
	case strings.Contains(r.sql, "has_schema_privilege"):
		if len(dest) != 43 {
			return errors.New("unexpected core privilege column count")
		}
		values := r.db.checks
		if values == nil {
			values = make([]bool, 43)
			for i := range values {
				values[i] = true
			}
		}
		for i, d := range dest {
			out, ok := d.(*bool)
			if !ok {
				return errors.New("unexpected core privilege destination")
			}
			*out = values[i]
		}
		return nil
	default:
		return fmt.Errorf("unexpected query row: %s", r.sql)
	}
}

type runtimeMembershipDB struct {
	edges  [][]any
	attrs  []bool
	checks []bool
}

func (d *runtimeMembershipDB) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "pg_auth_members") {
		return nil, errors.New("unexpected query")
	}
	return &runtimeMembershipRows{rows: d.edges}, nil
}
func (d *runtimeMembershipDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	return runtimeMembershipRow{db: d, sql: sql}
}
func (*runtimeMembershipDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func membershipEdge(role, member string, admin, inherit, set bool) []any {
	return []any{role, member, admin, inherit, set}
}

func TestVerifyRuntimePermissionCoreReadbackWithMemberships(t *testing.T) {
	expected := []RuntimeMembership{{Role: "espacigo_runtime", Member: "espacigo_api_login", AdminOption: false, InheritOption: false, SetOption: true}}
	toRows := func(edges ...[]any) *runtimeMembershipDB { return &runtimeMembershipDB{edges: edges} }
	t.Run("accepts expected API membership", func(t *testing.T) {
		db := toRows(membershipEdge("espacigo_runtime", "espacigo_api_login", false, false, true))
		if err := VerifyRuntimePermissionCoreReadbackWithMemberships(context.Background(), db, expected); err != nil {
			t.Fatal(err)
		}
	})
	cases := []struct {
		name     string
		edges    [][]any
		expected []RuntimeMembership
	}{
		{"missing incoming", nil, expected},
		{"extra incoming", [][]any{membershipEdge("unexpected_parent", "espacigo_runtime", false, false, true)}, []RuntimeMembership{}},
		{"extra outgoing", [][]any{membershipEdge("espacigo_runtime", "espacigo_api_login", false, false, true), membershipEdge("espacigo_runtime", "extra_member", false, false, true)}, expected},
		{"wrong admin", [][]any{membershipEdge("espacigo_runtime", "espacigo_api_login", true, false, true)}, expected},
		{"wrong inherit", [][]any{membershipEdge("espacigo_runtime", "espacigo_api_login", false, true, true)}, expected},
		{"wrong set", [][]any{membershipEdge("espacigo_runtime", "espacigo_api_login", false, false, false)}, expected},
		{"duplicate expected", [][]any{membershipEdge("espacigo_runtime", "espacigo_api_login", false, false, true), membershipEdge("espacigo_runtime", "espacigo_api_login", false, false, true)}, append(expected, expected...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyRuntimePermissionCoreReadbackWithMemberships(context.Background(), toRows(tc.edges...), tc.expected); err == nil {
				t.Fatal("expected membership mismatch")
			}
		})
	}
	t.Run("legacy strict verifier rejects valid edge", func(t *testing.T) {
		err := VerifyRuntimePermissionCoreReadback(context.Background(), toRows(membershipEdge("espacigo_runtime", "espacigo_api_login", false, false, true)))
		if err == nil || !strings.Contains(err.Error(), "unexpected memberships") {
			t.Fatalf("expected membership rejection, got %v", err)
		}
	})
}

func TestVerifyRuntimePermissionCoreReadbackRejectsUnsafeAttributes(t *testing.T) {
	for index, name := range []string{"login", "superuser", "createdb", "createrole", "replication", "bypassrls", "inherit"} {
		t.Run(name, func(t *testing.T) {
			attrs := make([]bool, 7)
			attrs[index] = true
			db := &runtimeMembershipDB{attrs: attrs}
			err := VerifyRuntimePermissionCoreReadbackWithMemberships(context.Background(), db, []RuntimeMembership{{Role: "espacigo_runtime", Member: "espacigo_api_login", SetOption: true}})
			if err == nil || !strings.Contains(err.Error(), "unsafe attributes") {
				t.Fatalf("expected unsafe attribute rejection, got %v", err)
			}
		})
	}
}

func TestVerifyRuntimePermissionCoreReadbackChecksEveryPrivilege(t *testing.T) {
	for index := 0; index < 43; index++ {
		t.Run(fmt.Sprintf("check_%02d", index+1), func(t *testing.T) {
			checks := make([]bool, 43)
			for i := range checks {
				checks[i] = true
			}
			checks[index] = false
			db := &runtimeMembershipDB{checks: checks}
			err := VerifyRuntimePermissionCoreReadbackWithMemberships(context.Background(), db, []RuntimeMembership{{Role: "espacigo_runtime", Member: "espacigo_api_login", SetOption: true}})
			if err == nil || !strings.Contains(err.Error(), "core runtime grants") {
				t.Fatalf("expected privilege rejection, got %v", err)
			}
		})
	}
}
