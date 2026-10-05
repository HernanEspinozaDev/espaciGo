package migrator

import (
	"context"
	"path/filepath"
	"testing"
)

func TestM02MigrationAddsOwnerScopedProfileAndRightsWithoutCascade(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := filepath.Join("..", "..", "db", "migrations")
		first, err := Run(ctx, databaseURL, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Applied) != 2 || first.Applied[0] != 1 || first.Applied[1] != 2 {
			t.Fatalf("applied versions=%v", first.Applied)
		}
		second, err := Run(ctx, databaseURL, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(second.Applied) != 0 {
			t.Fatalf("repeat run applied %v", second.Applied)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		for _, table := range []string{"perfil_usuario", "solicitud_titular"} {
			var exists bool
			if err := conn.QueryRow(ctx, "SELECT to_regclass('public.' || $1) IS NOT NULL", table).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if !exists {
				t.Errorf("missing M02 table %s", table)
			}
		}
		var badDeleteActions int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE connamespace='public'::regnamespace
			AND conrelid IN ('public.perfil_usuario'::regclass,'public.solicitud_titular'::regclass)
			AND contype='f' AND confdeltype NOT IN ('a','r')`).Scan(&badDeleteActions); err != nil {
			t.Fatal(err)
		}
		if badDeleteActions != 0 {
			t.Fatalf("M02 introduces %d cascading foreign keys", badDeleteActions)
		}
		var migrationRows int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&migrationRows); err != nil {
			t.Fatal(err)
		}
		if migrationRows != 2 {
			t.Fatalf("migration history rows=%d", migrationRows)
		}
	})
}
