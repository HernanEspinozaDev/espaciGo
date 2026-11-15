package migrator

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type nativeV86IdentityRow struct {
	database, version, session, current string
	oid                                 int64
	readOnly, canCreate                 bool
	err                                 error
}

func (r nativeV86IdentityRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 7 {
		return errors.New("unexpected native target identity scan")
	}
	*dest[0].(*string) = r.database
	*dest[1].(*int64) = r.oid
	*dest[2].(*string) = r.version
	*dest[3].(*string) = r.session
	*dest[4].(*string) = r.current
	*dest[5].(*bool) = r.readOnly
	*dest[6].(*bool) = r.canCreate
	return nil
}

type nativeV86IdentityQuery struct {
	row   nativeV86IdentityRow
	calls int
}

func (q *nativeV86IdentityQuery) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if sql != nativeV86TargetSQL {
		panic("unexpected native identity query")
	}
	q.calls++
	return q.row
}
func TestNativeV86ExecutionTargetFailsClosed(t *testing.T) {
	valid := nativeV86IdentityRow{database: "espacigo_dms_stage_20261020", oid: 16388, version: "180007", session: "espacigo_migrator_login", current: "espacigo_schema_owner", canCreate: true}
	q := &nativeV86IdentityQuery{row: valid}
	if err := guardNativeV86ExecutionTarget(context.Background(), q, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"database", "oid", "version", "session", "role", "read-only transaction", "missing CREATE", "query failure"} {
		t.Run(name, func(t *testing.T) {
			r := valid
			switch name {
			case "database":
				r.database = "other"
			case "oid":
				r.oid = 42
			case "version":
				r.version = "170000"
			case "session":
				r.session = "postgres"
			case "role":
				r.current = "postgres"
			case "read-only transaction":
				r.readOnly = true
			case "missing CREATE":
				r.canCreate = false
			case "query failure":
				r.err = errors.New("unavailable")
			}
			if guardNativeV86ExecutionTarget(context.Background(), &nativeV86IdentityQuery{row: r}, true) == nil {
				t.Fatal("unsafe execution target accepted")
			}
		})
	}
	withoutCreate := valid
	withoutCreate.canCreate = false
	if err := guardNativeV86ExecutionTarget(context.Background(), &nativeV86IdentityQuery{row: withoutCreate}, false); err != nil {
		t.Fatalf("non-CREATE migration rejected without a CREATE prerequisite: %v", err)
	}
}

func TestNativeKYCCreatePrerequisiteByVersion(t *testing.T) {
	for version, want := range map[int64]bool{
		86: true, 87: false, 88: true, 89: true, 90: true, 91: false,
		92: true, 93: true, 94: true, 95: true, 96: true, 97: false,
	} {
		if got := NativeKYCMigrationRequiresCreate(version); got != want {
			t.Fatalf("V%d CREATE prerequisite=%t, want %t", version, got, want)
		}
	}
}
