package migrator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRunnerAppliesFromEmptyAndRepeatIsNoop(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__probe.sql", "CREATE TABLE probe (id integer PRIMARY KEY); INSERT INTO probe VALUES (1);\n")

		first, err := Run(ctx, databaseURL, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Applied) != 1 || first.Applied[0] != 1 {
			t.Fatalf("first run applied versions %v", first.Applied)
		}
		second, err := Run(ctx, databaseURL, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(second.Applied) != 0 {
			t.Fatalf("second run reapplied versions %v", second.Applied)
		}

		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var rows, historyRows int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM probe").Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if rows != 1 || historyRows != 1 {
			t.Fatalf("repeat changed state: probe rows=%d history rows=%d", rows, historyRows)
		}
	})
}

func TestRunnerAppliesDDLUnderRestrictedSetRoleMembership(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		group := "mig_group_" + suffix
		login := "mig_login_" + suffix
		adminURL := os.Getenv("TEST_DATABASE_URL")
		admin := connectTestDB(t, ctx, adminURL)
		defer admin.Close(context.Background())
		dbURL, err := url.Parse(databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		databaseName := strings.TrimPrefix(dbURL.Path, "/")
		if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{group}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT"); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{login}.Sanitize()+" LOGIN PASSWORD 'local-integration-only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cleanupAdmin, err := pgx.Connect(cleanupCtx, adminURL)
			if err != nil {
				t.Errorf("connect disposable PostgreSQL for role cleanup: %v", err)
				return
			}
			defer cleanupAdmin.Close(context.Background())
			if _, err := cleanupAdmin.Exec(cleanupCtx, "DROP ROLE IF EXISTS "+pgx.Identifier{login}.Sanitize()+", "+pgx.Identifier{group}.Sanitize()); err != nil {
				t.Errorf("drop disposable restricted roles: %v", err)
			}
		})
		if _, err := admin.Exec(ctx, "GRANT CONNECT ON DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" TO "+pgx.Identifier{login}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		databaseAdmin := connectTestDB(t, ctx, databaseURL)
		if _, err := databaseAdmin.Exec(ctx, "GRANT USAGE, CREATE ON SCHEMA public TO "+pgx.Identifier{group}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		if err := databaseAdmin.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, "GRANT "+pgx.Identifier{group}.Sanitize()+" TO "+pgx.Identifier{login}.Sanitize()+" WITH ADMIN FALSE, INHERIT FALSE, SET TRUE"); err != nil {
			t.Fatal(err)
		}
		loginURL := *dbURL
		loginURL.User = url.UserPassword(login, "local-integration-only")
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__owned_probe.sql", "CREATE TABLE set_role_probe (id integer PRIMARY KEY);\n")
		result, err := RunWithRole(ctx, loginURL.String(), dir, group)
		if err != nil || len(result.Applied) != 1 {
			t.Fatalf("restricted migration applied=%v err=%v", result.Applied, err)
		}
		inspector := connectTestDB(t, ctx, databaseURL)
		var owner string
		if err := inspector.QueryRow(ctx, `SELECT tableowner FROM pg_tables WHERE schemaname='public' AND tablename='set_role_probe'`).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		if err := inspector.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if owner != group {
			t.Fatalf("migration object owner=%q, want SET ROLE group %q", owner, group)
		}
	})
}

func TestGADM03MigrationWorksFromEmptyAndV56WithoutChangingHistory(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 60 || migrations[58].Version != 59 || migrations[59].Version != 60 {
		t.Fatalf("expected V60 after V59, got %d migrations", len(migrations))
	}
	migrations = migrations[:59]
	latest := migrations[len(migrations)-1]
	if latest.Name != "V000059__gadm_03_audit_scope_and_query_indexes.sql" {
		t.Fatalf("unexpected latest migration %q", latest.Name)
	}
	v59Dir := t.TempDir()
	for _, migration := range migrations {
		if err := os.WriteFile(filepath.Join(v59Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("fresh base", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			result, err := Run(ctx, databaseURL, v59Dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Applied) != len(migrations) || result.Applied[len(result.Applied)-1] != 59 {
				t.Fatalf("fresh migration result=%v", result.Applied)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			var allowlisted, typed, indexExists, targetColumn, registeredColumn int
			if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.atributo_filtro_publico`).Scan(&allowlisted); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.atributo_filtro_publico WHERE tipo IN ('integer','number','enum','enum_list')`).Scan(&typed); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='espacio_caracteristicas_public_filter_keys_idx'`).Scan(&indexExists); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='evento_auditoria_local' AND column_name='cuenta_objetivo_id'`).Scan(&targetColumn); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='evento_auditoria_local' AND column_name='registrado_en'`).Scan(&registeredColumn); err != nil {
				t.Fatal(err)
			}
			if allowlisted < 50 || typed < 30 || indexExists != 1 || targetColumn != 1 || registeredColumn != 1 {
				t.Fatalf("fresh base mismatch: allowlisted=%d typed=%d index=%d target=%d registered=%d", allowlisted, typed, indexExists, targetColumn, registeredColumn)
			}
		})
	})

	t.Run("upgrade from V56", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			v56Dir := t.TempDir()
			for _, migration := range migrations {
				if migration.Version > 56 {
					continue
				}
				if err := os.WriteFile(filepath.Join(v56Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Run(ctx, databaseURL, v56Dir)
			if err != nil || len(result.Applied) != 56 || result.Applied[55] != 56 {
				t.Fatalf("apply V1-V56: %v %v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			before := map[int64]string{}
			rows, err := conn.Query(ctx, `SELECT version,checksum FROM public.schema_migrations ORDER BY version`)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var version int64
				var checksum string
				if err := rows.Scan(&version, &checksum); err != nil {
					t.Fatal(err)
				}
				before[version] = checksum
			}
			rows.Close()
			if err := conn.Close(ctx); err != nil {
				t.Fatal(err)
			}
			result, err = Run(ctx, databaseURL, v59Dir)
			if err != nil || len(result.Applied) != 3 || result.Applied[0] != 57 || result.Applied[1] != 58 || result.Applied[2] != 59 {
				t.Fatalf("upgrade from V56: %v %v", result.Applied, err)
			}
			conn = connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			rows, err = conn.Query(ctx, `SELECT version,checksum FROM public.schema_migrations WHERE version<=56 ORDER BY version`)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for rows.Next() {
				var version int64
				var checksum string
				if err := rows.Scan(&version, &checksum); err != nil {
					t.Fatal(err)
				}
				count++
				if before[version] != checksum {
					t.Fatalf("V%d checksum changed: before=%q after=%q", version, before[version], checksum)
				}
			}
			rows.Close()
			if count != 56 {
				t.Fatalf("preserved migration rows=%d, want 56", count)
			}
		})
	})

	t.Run("upgrade from V58", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			v58Dir := t.TempDir()
			for _, migration := range migrations {
				if migration.Version > 58 {
					continue
				}
				if err := os.WriteFile(filepath.Join(v58Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if result, err := Run(ctx, databaseURL, v58Dir); err != nil || len(result.Applied) != 58 || result.Applied[57] != 58 {
				t.Fatalf("apply V1-V58: %v %v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			before := map[int64]string{}
			rows, err := conn.Query(ctx, `SELECT version,checksum FROM public.schema_migrations ORDER BY version`)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var version int64
				var checksum string
				if err := rows.Scan(&version, &checksum); err != nil {
					t.Fatal(err)
				}
				before[version] = checksum
			}
			rows.Close()
			result, err := Run(ctx, databaseURL, v59Dir)
			if err != nil || len(result.Applied) != 1 || result.Applied[0] != 59 {
				t.Fatalf("upgrade from V58: %v %v", result.Applied, err)
			}
			rows, err = conn.Query(ctx, `SELECT version,checksum FROM public.schema_migrations ORDER BY version`)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for rows.Next() {
				var version int64
				var checksum string
				if err := rows.Scan(&version, &checksum); err != nil {
					t.Fatal(err)
				}
				count++
				if version <= 58 && before[version] != checksum {
					t.Fatalf("V%d checksum changed: before=%q after=%q", version, before[version], checksum)
				}
			}
			rows.Close()
			if count != 59 {
				t.Fatalf("preserved migration rows=%d, want 59", count)
			}
		})
	})
}

func TestGADM04RestrictedBootstrapRoleMigrationFromV59(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 61 || migrations[59].Name != "V000060__gadm_local_bootstrap_restricted_role.sql" {
		t.Fatalf("unexpected latest migration: count=%d latest=%q", len(migrations), migrations[len(migrations)-1].Name)
	}
	v59Dir := t.TempDir()
	for _, migration := range migrations[:59] {
		if err := os.WriteFile(filepath.Join(v59Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	v60Dir := t.TempDir()
	for _, migration := range migrations[:60] {
		if err := os.WriteFile(filepath.Join(v60Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		if result, err := Run(ctx, databaseURL, v59Dir); err != nil || len(result.Applied) != 59 || result.Applied[58] != 59 {
			t.Fatalf("apply V1-V59: applied=%v err=%v", result.Applied, err)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(ctx)
		var before string
		if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		result, err := Run(ctx, databaseURL, v60Dir)
		if err != nil || len(result.Applied) != 1 || result.Applied[0] != 60 {
			t.Fatalf("upgrade V59 to V60: applied=%v err=%v", result.Applied, err)
		}
		var after string
		if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations WHERE version<60`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatal("V60 changed an applied V1-V59 migration checksum or history row")
		}
		var canLogin, super, createDB, createRole, replication, bypassRLS, inherit bool
		if err := conn.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls,rolinherit
			FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'`).Scan(&canLogin, &super, &createDB, &createRole, &replication, &bypassRLS, &inherit); err != nil {
			t.Fatal(err)
		}
		if canLogin || super || createDB || createRole || replication || bypassRLS || inherit {
			t.Fatalf("bootstrap group has unsafe role attributes: login=%v super=%v createdb=%v createrole=%v replication=%v bypassrls=%v inherit=%v", canLogin, super, createDB, createRole, replication, bypassRLS, inherit)
		}
		var memberships int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_auth_members m
			JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member
			WHERE r.rolname='espacigo_gadm_bootstrap' OR member.rolname='espacigo_gadm_bootstrap'`).Scan(&memberships); err != nil {
			t.Fatal(err)
		}
		if memberships != 0 {
			t.Fatalf("bootstrap group starts with %d role memberships", memberships)
		}
		var connect, noCreateDB, schemaUsage, noSchemaCreate bool
		if err := conn.QueryRow(ctx, `SELECT has_database_privilege('espacigo_gadm_bootstrap',current_database(),'CONNECT'),
			NOT has_database_privilege('espacigo_gadm_bootstrap',current_database(),'CREATE'),
			has_schema_privilege('espacigo_gadm_bootstrap','public','USAGE'),
			NOT has_schema_privilege('espacigo_gadm_bootstrap','public','CREATE')`).Scan(&connect, &noCreateDB, &schemaUsage, &noSchemaCreate); err != nil {
			t.Fatal(err)
		}
		if !connect || !noCreateDB || !schemaUsage || !noSchemaCreate {
			t.Fatalf("bootstrap group database/schema ACLs are too broad or incomplete: connect=%v noCreateDB=%v usage=%v noCreate=%v", connect, noCreateDB, schemaUsage, noSchemaCreate)
		}
		rows, err := conn.Query(ctx, `SELECT table_name,column_name,privilege_type FROM information_schema.column_privileges
			WHERE grantee='espacigo_gadm_bootstrap' ORDER BY table_name,column_name,privilege_type`)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var table, column, privilege string
			if err := rows.Scan(&table, &column, &privilege); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			got[table+"."+column+":"+privilege] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		want := []string{
			"usuario.id:SELECT", "usuario.estado:SELECT", "usuario.bloqueado_hasta:SELECT", "usuario.id:UPDATE",
			"bloqueo_cuenta_administrativo_local.cuenta_id:SELECT",
			"administrador_global_bootstrap.singleton:SELECT", "administrador_global_bootstrap.usuario_id:SELECT", "administrador_global_bootstrap.singleton:UPDATE",
			"administrador_global_bootstrap.singleton:INSERT", "administrador_global_bootstrap.usuario_id:INSERT", "administrador_global_bootstrap.creado_en:INSERT",
			"administrador_global.usuario_id:INSERT", "administrador_global.concedido_en:INSERT", "administrador_global.motivo_codigo:INSERT",
			"administrador_global.usuario_id:SELECT",
			"administrador_global_bootstrap_evento.id:INSERT", "administrador_global_bootstrap_evento.usuario_objetivo_id:INSERT",
			"administrador_global_bootstrap_evento.actor_tipo:INSERT", "administrador_global_bootstrap_evento.accion:INSERT",
			"administrador_global_bootstrap_evento.resultado:INSERT", "administrador_global_bootstrap_evento.motivo_codigo:INSERT",
			"administrador_global_bootstrap_evento.correlacion_id:INSERT", "administrador_global_bootstrap_evento.ocurrida_en:INSERT",
		}
		if len(got) != len(want) {
			t.Fatalf("bootstrap group column grants=%d, want exact allowlist of %d", len(got), len(want))
		}
		for _, privilege := range want {
			if !got[privilege] {
				t.Errorf("required or unexpected bootstrap column privilege missing: %s", privilege)
			}
		}
	})
}

func TestMPV212HourlyScheduleMigrationFromFreshAndV60(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 61 || migrations[60].Name != "V000061__mpv2_12_hourly_publication_schedules.sql" {
		t.Fatalf("unexpected latest migration: count=%d latest=%q", len(migrations), migrations[len(migrations)-1].Name)
	}
	v60Dir := t.TempDir()
	for _, migration := range migrations[:60] {
		if err := os.WriteFile(filepath.Join(v60Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	v61Dir := t.TempDir()
	for _, migration := range migrations[:61] {
		if err := os.WriteFile(filepath.Join(v61Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("fresh V1-V61", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			result, err := Run(ctx, databaseURL, v61Dir)
			if err != nil || len(result.Applied) != 61 || result.Applied[60] != 61 {
				t.Fatalf("fresh migration result=%v err=%v", result.Applied, err)
			}
			assertHourlyScheduleMigrationBehavior(t, ctx, databaseURL)
		})
	})

	t.Run("upgrade V60 to V61", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			result, err := Run(ctx, databaseURL, v60Dir)
			if err != nil || len(result.Applied) != 60 || result.Applied[59] != 60 {
				t.Fatalf("apply historical V1-V60: applied=%v err=%v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			var before string
			if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations`).Scan(&before); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			result, err = Run(ctx, databaseURL, v61Dir)
			if err != nil || len(result.Applied) != 1 || result.Applied[0] != 61 {
				_ = conn.Close(ctx)
				t.Fatalf("upgrade V60 to V61: applied=%v err=%v", result.Applied, err)
			}
			var after string
			if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations WHERE version<61`).Scan(&after); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if err := conn.Close(ctx); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("V61 changed an applied V1-V60 migration checksum or history row")
			}
			assertHourlyScheduleMigrationBehavior(t, ctx, databaseURL)
		})
	})
}

func TestGCP04DurableReplayRegistryFromFreshAndV61(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 62 || migrations[61].Name != "V000062__gcp04_durable_privacy_replay_registry.sql" {
		t.Fatalf("unexpected latest migration: count=%d latest=%q", len(migrations), migrations[len(migrations)-1].Name)
	}
	v61Dir := t.TempDir()
	for _, migration := range migrations[:61] {
		if err := os.WriteFile(filepath.Join(v61Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	v62Dir := t.TempDir()
	for _, migration := range migrations[:62] {
		if err := os.WriteFile(filepath.Join(v62Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("fresh V1-V62", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			result, err := Run(ctx, databaseURL, v62Dir)
			if err != nil || len(result.Applied) != 62 || result.Applied[61] != 62 {
				t.Fatalf("fresh migration result=%v err=%v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			var exists int
			if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='registro_replay_baja_local'`).Scan(&exists); err != nil || exists != 1 {
				t.Fatalf("fresh replay registry exists=%d err=%v", exists, err)
			}
		})
	})

	t.Run("upgrade V61 and seed completed rows", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			result, err := Run(ctx, databaseURL, v61Dir)
			if err != nil || len(result.Applied) != 61 || result.Applied[60] != 61 {
				t.Fatalf("apply V1-V61: applied=%v err=%v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			const accountID = "06200000-0000-4000-8000-000000000001"
			const requestID = "06200000-0000-4000-8000-000000000002"
			const executionID = "06200000-0000-4000-8000-000000000003"
			requestedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			completedAt := requestedAt.Add(time.Hour)
			if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,'gcp04-migration@example.invalid','gcp04-migration@example.invalid','synthetic','desidentificado')`, accountID); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if _, err := conn.Exec(ctx, `INSERT INTO public.solicitud_titular(id,usuario_id,tipo,canal,estado,solicitada_en) VALUES($1,$2,'supresion','web','resuelta',$3)`, requestID, accountID, requestedAt); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if _, err := conn.Exec(ctx, `INSERT INTO public.ejecucion_baja_local(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,completada_en,retirar_en,detalle)
				VALUES($1,$2,$3,$3,'gcp04-migration','completada','baja_local_minimizada',$4,$5::timestamptz,$5::timestamptz+interval '5 years','{}'::jsonb)`, executionID, requestID, accountID, requestedAt, completedAt); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			var before string
			if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations`).Scan(&before); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if err := conn.Close(ctx); err != nil {
				t.Fatal(err)
			}
			result, err = Run(ctx, databaseURL, v62Dir)
			if err != nil || len(result.Applied) != 1 || result.Applied[0] != 62 {
				t.Fatalf("upgrade V61 to V62: applied=%v err=%v", result.Applied, err)
			}
			conn = connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			var after, gotRequest, gotAccount string
			var gotRequested, gotCompleted time.Time
			if err := conn.QueryRow(ctx, `SELECT solicitud_id::text,usuario_id::text,solicitada_en,completada_en FROM public.registro_replay_baja_local WHERE ejecucion_origen_id=$1`, executionID).Scan(&gotRequest, &gotAccount, &gotRequested, &gotCompleted); err != nil {
				t.Fatal(err)
			}
			if gotRequest != requestID || gotAccount != accountID || !gotRequested.Equal(requestedAt) || !gotCompleted.Equal(completedAt) {
				t.Fatalf("backfilled registry row=%s/%s/%s/%s", gotRequest, gotAccount, gotRequested, gotCompleted)
			}
			if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations WHERE version<62`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("V62 changed an applied V1-V61 migration checksum or history row")
			}
		})
	})

	t.Run("runtime grants remain compatible with V61", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			if result, err := Run(ctx, databaseURL, v61Dir); err != nil || len(result.Applied) != 61 {
				t.Fatalf("apply historical V1-V61: applied=%v err=%v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			if _, err := conn.Exec(ctx, `CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT`); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if err := dbbootstrap.GrantRuntimePermissions(ctx, conn); err != nil {
				_, _ = conn.Exec(context.Background(), `DROP OWNED BY espacigo_runtime; DROP ROLE espacigo_runtime`)
				_ = conn.Close(ctx)
				t.Fatalf("runtime grants on historical V61 database: %v", err)
			}
			var registryAbsent, suppressionSelect bool
			if err := conn.QueryRow(ctx, `SELECT to_regclass('public.registro_replay_baja_local') IS NULL,
				has_table_privilege('espacigo_runtime','public.ejecucion_baja_local','SELECT')`).Scan(&registryAbsent, &suppressionSelect); err != nil || !registryAbsent || !suppressionSelect {
				_, _ = conn.Exec(context.Background(), `DROP OWNED BY espacigo_runtime; DROP ROLE espacigo_runtime`)
				_ = conn.Close(ctx)
				t.Fatalf("V61 runtime privilege compatibility: registry_absent=%v suppression_select=%v err=%v", registryAbsent, suppressionSelect, err)
			}
			if _, err := conn.Exec(ctx, `DROP OWNED BY espacigo_runtime; DROP ROLE espacigo_runtime`); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if err := conn.Close(ctx); err != nil {
				t.Fatal(err)
			}
		})
	})
}

func TestGCP04SuppressionClaimFencingMigrationFromFreshAndV62(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 63 || migrations[62].Name != "V000063__gcp04_privacy_suppression_claim_fencing.sql" {
		t.Fatalf("unexpected latest migration: count=%d latest=%q", len(migrations), migrations[len(migrations)-1].Name)
	}
	v62Dir, v63Dir := t.TempDir(), t.TempDir()
	for _, migration := range migrations[:62] {
		if err := os.WriteFile(filepath.Join(v62Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, migration := range migrations[:63] {
		if err := os.WriteFile(filepath.Join(v63Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	assertClaimSchema := func(t *testing.T, ctx context.Context, conn *pgx.Conn) {
		t.Helper()
		var ownerNullable, leaseNullable, generationDefault string
		if err := conn.QueryRow(ctx, `SELECT
			(SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='baja_archivo_pendiente_local' AND column_name='claim_owner'),
			(SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='baja_archivo_pendiente_local' AND column_name='claim_hasta'),
			(SELECT column_default FROM information_schema.columns WHERE table_schema='public' AND table_name='baja_archivo_pendiente_local' AND column_name='claim_generacion')`).Scan(&ownerNullable, &leaseNullable, &generationDefault); err != nil {
			t.Fatal(err)
		}
		if ownerNullable != "YES" || leaseNullable != "YES" || !strings.Contains(generationDefault, "0") {
			t.Fatalf("claim schema owner/lease/default=%s/%s/%q", ownerNullable, leaseNullable, generationDefault)
		}
		var indexDef string
		if err := conn.QueryRow(ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname='public' AND indexname='baja_archivo_pendiente_claim_idx'`).Scan(&indexDef); err != nil || !strings.Contains(indexDef, "claim_hasta") {
			t.Fatalf("claim due index definition=%q err=%v", indexDef, err)
		}
	}

	t.Run("fresh V1-V63", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			result, err := Run(ctx, databaseURL, v63Dir)
			if err != nil || len(result.Applied) != 63 || result.Applied[62] != 63 {
				t.Fatalf("fresh migration result=%v err=%v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			assertClaimSchema(t, ctx, conn)
		})
	})

	t.Run("upgrade V62 preserves history and queued rows", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			if result, err := Run(ctx, databaseURL, v62Dir); err != nil || len(result.Applied) != 62 || result.Applied[61] != 62 {
				t.Fatalf("apply V1-V62: applied=%v err=%v", result.Applied, err)
			}
			conn := connectTestDB(t, ctx, databaseURL)
			const accountID = "06300000-0000-4000-8000-000000000001"
			const requestID = "06300000-0000-4000-8000-000000000002"
			const executionID = "06300000-0000-4000-8000-000000000003"
			const evidenceID = "06300000-0000-4000-8000-000000000004"
			now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
			if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,'gcp04-v63@example.invalid','gcp04-v63@example.invalid','synthetic','activo')`, accountID); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if _, err := conn.Exec(ctx, `INSERT INTO public.solicitud_titular(id,usuario_id,tipo,canal,estado,solicitada_en) VALUES($1,$2,'supresion','web','en_revision',$3)`, requestID, accountID, now); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if _, err := conn.Exec(ctx, `INSERT INTO public.ejecucion_baja_local(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,detalle)
				VALUES($1,$2,$3,$3,'gcp04-v63','limpieza_pendiente','baja_local_minimizada',$4,'{}'::jsonb)`, executionID, requestID, accountID, now); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if _, err := conn.Exec(ctx, `INSERT INTO public.baja_archivo_pendiente_local(ejecucion_id,evidencia_id,disponible_en) VALUES($1,$2,$3)`, executionID, evidenceID, now); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			var before string
			if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations`).Scan(&before); err != nil {
				_ = conn.Close(ctx)
				t.Fatal(err)
			}
			if err := conn.Close(ctx); err != nil {
				t.Fatal(err)
			}
			result, err := Run(ctx, databaseURL, v63Dir)
			if err != nil || len(result.Applied) != 1 || result.Applied[0] != 63 {
				t.Fatalf("upgrade V62 to V63: applied=%v err=%v", result.Applied, err)
			}
			conn = connectTestDB(t, ctx, databaseURL)
			defer conn.Close(ctx)
			var claimOwner *string
			var claimUntil *time.Time
			var claimGeneration int64
			if err := conn.QueryRow(ctx, `SELECT claim_owner::text,claim_hasta,claim_generacion FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND evidencia_id=$2`, executionID, evidenceID).Scan(&claimOwner, &claimUntil, &claimGeneration); err != nil || claimOwner != nil || claimUntil != nil || claimGeneration != 0 {
				t.Fatalf("existing pending row after V63 owner=%v lease=%v generation=%d err=%v", claimOwner, claimUntil, claimGeneration, err)
			}
			assertClaimSchema(t, ctx, conn)
			var after string
			if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations WHERE version<63`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("V63 changed an applied V1-V62 migration checksum or history row")
			}
		})
	})
}

func TestGCP04WorkerRoleMigrationV64LeastPrivilege(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	admin, err := pgx.Connect(context.Background(), adminURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), `DROP ROLE IF EXISTS espacigo_privacy_cleanup_worker; DROP ROLE IF EXISTS espacigo_runtime; CREATE ROLE espacigo_runtime NOLOGIN`); err != nil {
		_ = admin.Close(context.Background())
		t.Fatal(err)
	}
	if err := admin.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, err := pgx.Connect(context.Background(), adminURL)
		if err != nil {
			t.Errorf("connect to clean disposable roles: %v", err)
			return
		}
		defer cleanup.Close(context.Background())
		if _, err := cleanup.Exec(context.Background(), `DROP ROLE IF EXISTS espacigo_privacy_cleanup_worker; DROP ROLE IF EXISTS espacigo_runtime`); err != nil {
			t.Errorf("drop disposable cluster roles: %v", err)
		}
	})
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 64 || migrations[63].Name != "V000064__gcp04_privacy_cleanup_worker_role.sql" {
		t.Fatalf("unexpected V64 migration: count=%d v64=%q", len(migrations), migrations[63].Name)
	}
	v64Dir := t.TempDir()
	for _, migration := range migrations[:64] {
		if err := os.WriteFile(filepath.Join(v64Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		result, err := Run(ctx, databaseURL, v64Dir)
		if err != nil || len(result.Applied) != 64 || result.Applied[63] != 64 {
			t.Fatalf("fresh V1-V64 migration: applied=%v err=%v", result.Applied, err)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(ctx)
		var login, superuser, inherit, runtimeCanReadAccounts, runtimeCanExecute, workerCanReadQueue, workerCanDeleteEvidence bool
		if err := conn.QueryRow(ctx, `SELECT r.rolcanlogin,r.rolsuper,r.rolinherit,
			has_table_privilege('espacigo_privacy_cleanup_worker','public.usuario','SELECT'),
			has_function_privilege('espacigo_runtime','public.finalize_pending_suppression_cleanup(uuid,timestamptz)','EXECUTE'),
			has_column_privilege('espacigo_privacy_cleanup_worker','public.baja_archivo_pendiente_local','claim_owner','SELECT'),
			has_table_privilege('espacigo_privacy_cleanup_worker','public.verificacion_evidencia_sintetica','DELETE')
			FROM pg_roles r WHERE r.rolname='espacigo_privacy_cleanup_worker'`).Scan(&login, &superuser, &inherit, &runtimeCanReadAccounts, &runtimeCanExecute, &workerCanReadQueue, &workerCanDeleteEvidence); err != nil {
			t.Fatal(err)
		}
		if login || superuser || inherit || runtimeCanReadAccounts || runtimeCanExecute || !workerCanReadQueue || !workerCanDeleteEvidence {
			t.Fatalf("least privilege mismatch login=%v superuser=%v inherit=%v account_read=%v runtime_execute=%v queue_read=%v evidence_delete=%v", login, superuser, inherit, runtimeCanReadAccounts, runtimeCanExecute, workerCanReadQueue, workerCanDeleteEvidence)
		}
		const accountID = "06400000-0000-4000-8000-000000000101"
		const requestID = "06400000-0000-4000-8000-000000000102"
		const executionID = "06400000-0000-4000-8000-000000000103"
		now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
		if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,'worker-role@example.invalid','worker-role@example.invalid','synthetic','activo')`, accountID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.solicitud_titular(id,usuario_id,tipo,canal,estado,solicitada_en) VALUES($1,$2,'supresion','web','en_revision',$3)`, requestID, accountID, now); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.ejecucion_baja_local(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,detalle) VALUES($1,$2,$3,$3,'gcp04-v64-role-test','limpieza_pendiente','baja_local_minimizada',$4,'{}'::jsonb)`, executionID, requestID, accountID, now); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_cleanup_worker`); err != nil {
			t.Fatal(err)
		}
		var finalized bool
		if err := conn.QueryRow(ctx, `SELECT public.finalize_pending_suppression_cleanup($1,$2)`, requestID, now).Scan(&finalized); err != nil || !finalized {
			t.Fatalf("worker finalizer did not complete queue: finalized=%v err=%v", finalized, err)
		}
		if _, err := conn.Exec(ctx, `RESET ROLE`); err != nil {
			t.Fatal(err)
		}
		var executionState, requestState string
		var replayRows int
		if err := conn.QueryRow(ctx, `SELECT x.estado,s.estado,(SELECT count(*) FROM public.registro_replay_baja_local WHERE ejecucion_origen_id=x.id) FROM public.ejecucion_baja_local x JOIN public.solicitud_titular s ON s.id=x.solicitud_id WHERE x.id=$1`, executionID).Scan(&executionState, &requestState, &replayRows); err != nil || executionState != "completada" || requestState != "resuelta" || replayRows != 1 {
			t.Fatalf("worker finalizer state execution=%s request=%s replay=%d err=%v", executionState, requestState, replayRows, err)
		}
		if _, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT public.finalize_pending_suppression_cleanup($1,$2)`, requestID, now).Scan(&finalized); err == nil {
			t.Fatal("broad runtime role invoked worker finalizer")
		}
	})
	v63Dir := t.TempDir()
	for _, migration := range migrations[:63] {
		if err := os.WriteFile(filepath.Join(v63Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		if result, err := Run(ctx, databaseURL, v63Dir); err != nil || len(result.Applied) != 63 || result.Applied[62] != 63 {
			t.Fatalf("apply V1-V63: applied=%v err=%v", result.Applied, err)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		const accountID = "06400000-0000-4000-8000-000000000001"
		const requestID = "06400000-0000-4000-8000-000000000002"
		const executionID = "06400000-0000-4000-8000-000000000003"
		const evidenceID = "06400000-0000-4000-8000-000000000004"
		now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
		if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,'gcp04-v64@example.invalid','gcp04-v64@example.invalid','synthetic','activo')`, accountID); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.solicitud_titular(id,usuario_id,tipo,canal,estado,solicitada_en) VALUES($1,$2,'supresion','web','en_revision',$3)`, requestID, accountID, now); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.ejecucion_baja_local(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,detalle) VALUES($1,$2,$3,$3,'gcp04-v64','limpieza_pendiente','baja_local_minimizada',$4,'{}'::jsonb)`, executionID, requestID, accountID, now); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.baja_archivo_pendiente_local(ejecucion_id,evidencia_id,disponible_en) VALUES($1,$2,$3)`, executionID, evidenceID, now); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		var before string
		if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations`).Scan(&before); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if err := conn.Close(ctx); err != nil {
			t.Fatal(err)
		}
		result, err := Run(ctx, databaseURL, v64Dir)
		if err != nil || len(result.Applied) != 1 || result.Applied[0] != 64 {
			t.Fatalf("upgrade V63 to V64: applied=%v err=%v", result.Applied, err)
		}
		conn = connectTestDB(t, ctx, databaseURL)
		defer conn.Close(ctx)
		var claimGeneration int64
		var completedAt *time.Time
		if err := conn.QueryRow(ctx, `SELECT claim_generacion,completada_en FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND evidencia_id=$2`, executionID, evidenceID).Scan(&claimGeneration, &completedAt); err != nil || claimGeneration != 0 || completedAt != nil {
			t.Fatalf("V63 pending row after V64 generation=%d completed=%v err=%v", claimGeneration, completedAt, err)
		}
		var after string
		if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum, ',' ORDER BY version) FROM public.schema_migrations WHERE version<64`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatal("V64 changed an applied V1-V63 migration checksum or history row")
		}
	})
}

func TestMigratorSerializesIndependentProcesses(t *testing.T) {
	if os.Getenv("MIGRATOR_PROCESS_HELPER") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := Run(ctx, os.Getenv("TEST_DATABASE_URL"), os.Getenv("MIGRATOR_PROCESS_MIGRATIONS")); err != nil {
			t.Fatalf("subprocess migrator failed: %v", err)
		}
		return
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		migrationDir := t.TempDir()
		writeMigration(t, migrationDir, "V000001__process_lock.sql", `CREATE TABLE public.process_migration_once (id integer PRIMARY KEY); INSERT INTO public.process_migration_once VALUES (1);`)
		env := make([]string, 0, len(os.Environ())+3)
		for _, item := range os.Environ() {
			if strings.HasPrefix(item, "MIGRATOR_PROCESS_HELPER=") || strings.HasPrefix(item, "TEST_DATABASE_URL=") || strings.HasPrefix(item, "MIGRATOR_PROCESS_MIGRATIONS=") {
				continue
			}
			env = append(env, item)
		}
		commands := make([]*exec.Cmd, 2)
		var outputs [2]bytes.Buffer
		for index := range commands {
			commands[index] = exec.Command(os.Args[0], "-test.run=^TestMigratorSerializesIndependentProcesses$")
			commands[index].Env = append(env, "MIGRATOR_PROCESS_HELPER=1", "TEST_DATABASE_URL="+databaseURL, "MIGRATOR_PROCESS_MIGRATIONS="+migrationDir)
			commands[index].Stdout = &outputs[index]
			commands[index].Stderr = &outputs[index]
			if err := commands[index].Start(); err != nil {
				t.Fatalf("start migrator subprocess %d: %v", index, err)
			}
		}
		for index, command := range commands {
			if err := command.Wait(); err != nil {
				t.Fatalf("migrator subprocess %d: %v: %s", index, err, outputs[index].String())
			}
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(ctx)
		var historyRows, ddlRows int
		if err := conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.schema_migrations),(SELECT count(*) FROM public.process_migration_once)`).Scan(&historyRows, &ddlRows); err != nil {
			t.Fatal(err)
		}
		if historyRows != 1 || ddlRows != 1 {
			t.Fatalf("concurrent process migration duplicated work: history=%d ddl=%d", historyRows, ddlRows)
		}
	})
}

func assertHourlyScheduleMigrationBehavior(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	conn := connectTestDB(t, ctx, databaseURL)
	defer conn.Close(ctx)
	ownerID := "00000000-0000-4000-8000-000000000101"
	if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
		VALUES ($1,'mpv212-migration@example.invalid','mpv212-migration@example.invalid','test-hash','activo')`, ownerID); err != nil {
		t.Fatal(err)
	}
	spaces := []struct {
		id    string
		unit  string
		state string
	}{
		{"00000000-0000-4000-8000-000000000111", "hora", "activa"},
		{"00000000-0000-4000-8000-000000000112", "dia", "activa"},
		{"00000000-0000-4000-8000-000000000113", "mes", "activa"},
		{"00000000-0000-4000-8000-000000000114", "hora", "eliminada"},
	}
	for i, space := range spaces {
		if _, err := conn.Exec(ctx, `INSERT INTO public.espacio
			(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,
			 modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
			VALUES ($1,$2,'oficina',$3,repeat('Synthetic description ',6),20,2,'Synthetic rules',
			$4,12000,$5,$6,'UTC')`, space.id, ownerID, fmt.Sprintf("MPV2-12 schedule case %d", i), space.unit,
			fmt.Sprintf("Migration address %d", i), space.state); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,$2,12000)`, space.id, space.unit); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo) VALUES($1,true)`, spaces[0].id); err != nil {
		t.Fatalf("active schedule for ordinary hourly publication was rejected: %v", err)
	}
	for _, space := range []struct{ id, unit string }{{spaces[1].id, "day"}, {spaces[2].id, "month"}, {spaces[3].id, "deleted hourly"}} {
		_, err := conn.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo) VALUES($1,true)`, space.id)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "horario_semanal_publicacion_hora_ck" {
			t.Errorf("active schedule for %s publication error=%v; want check violation horario_semanal_publicacion_hora_ck", space.unit, err)
		}
	}
}

func TestUpgradeFromExistingV46BasePreservesMigrationHistory(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		_, sourceFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("resolve migration source path")
		}
		sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
		migrations, err := DiscoverMigrations(sourceDir)
		if err != nil {
			t.Fatal(err)
		}
		v46Dir := t.TempDir()
		for _, migration := range migrations {
			if migration.Version > 46 {
				continue
			}
			if err := os.WriteFile(filepath.Join(v46Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
				t.Fatal(err)
			}
			if migration.Version == 46 && migration.Checksum != "74e70bad1a0fb0d99dfb25a66e5dae169d957c62914e879fda18489501ab5736" {
				t.Fatalf("V46 differs from the established base migration checksum: %s", migration.Checksum)
			}
		}
		if result, err := Run(ctx, databaseURL, v46Dir); err != nil || len(result.Applied) != 46 || result.Applied[45] != 46 {
			t.Fatalf("apply established V1-V46 base: applied=%v err=%v", result.Applied, err)
		}

		conn := connectTestDB(t, ctx, databaseURL)
		var beforeChecksum, beforeName string
		var beforeAppliedAt time.Time
		if err := conn.QueryRow(ctx, `SELECT name,checksum,applied_at FROM public.schema_migrations WHERE version=46`).Scan(&beforeName, &beforeChecksum, &beforeAppliedAt); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if err := conn.Close(ctx); err != nil {
			t.Fatal(err)
		}

		result, err := Run(ctx, databaseURL, sourceDir)
		if err != nil {
			t.Fatalf("upgrade a previously migrated V46 database: %v", err)
		}
		if len(result.Applied) != len(migrations)-46 {
			t.Fatalf("upgrade applied versions %v; want all versions after V46", result.Applied)
		}
		conn = connectTestDB(t, ctx, databaseURL)
		defer conn.Close(ctx)
		var afterChecksum, afterName string
		var afterAppliedAt time.Time
		var historyRows int
		if err := conn.QueryRow(ctx, `SELECT name,checksum,applied_at FROM public.schema_migrations WHERE version=46`).Scan(&afterName, &afterChecksum, &afterAppliedAt); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.schema_migrations`).Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if beforeName != afterName || beforeChecksum != afterChecksum || !beforeAppliedAt.Equal(afterAppliedAt) {
			t.Fatalf("upgrade rewrote V46 history: before=(%s,%s,%s) after=(%s,%s,%s)", beforeName, beforeChecksum, beforeAppliedAt, afterName, afterChecksum, afterAppliedAt)
		}
		if historyRows != len(migrations) {
			t.Fatalf("migration history rows=%d, want %d", historyRows, len(migrations))
		}
	})
}

func TestUpgradeFromV46BackfillsVerificationEvidenceFileReferences(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		_, sourceFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("resolve migration test source path")
		}
		sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
		migrations, err := DiscoverMigrations(sourceDir)
		if err != nil {
			t.Fatal(err)
		}
		v46Dir := t.TempDir()
		for _, migration := range migrations {
			if migration.Version > 46 {
				continue
			}
			if err := os.WriteFile(filepath.Join(v46Dir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if result, err := Run(ctx, databaseURL, v46Dir); err != nil || len(result.Applied) != 46 || result.Applied[len(result.Applied)-1] != 46 {
			t.Fatalf("apply V1-V46: applied=%v err=%v", result.Applied, err)
		}

		ownerID, reviewerID := "a4700000-0000-4000-8000-000000000001", "a4700000-0000-4000-8000-000000000002"
		caseID, evidenceID := "a4700000-0000-4000-8000-000000000003", "a4700000-0000-4000-8000-000000000004"
		resolvedAt := time.Date(2024, 3, 10, 14, 15, 16, 0, time.UTC)
		conn := connectTestDB(t, ctx, databaseURL)
		for _, user := range []struct{ id, email string }{{ownerID, "retention-owner@example.invalid"}, {reviewerID, "retention-reviewer@example.invalid"}} {
			if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'test-hash','activo')`, user.id, user.email); err != nil {
				_ = conn.Close(context.Background())
				t.Fatal(err)
			}
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,resuelta_en)
			VALUES($1,$2,'kyc','aprobada',$3,'upgrade-v46-retention-1',$4,$5)`, caseID, ownerID, "fixture:"+caseID, reviewerID, resolvedAt); err != nil {
			_ = conn.Close(context.Background())
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.verificacion_evidencia_sintetica(id,verificacion_id,codigo_fixture,mime_type,tamano_bytes,sha256,creada_en)
			VALUES($1,$2,'synthetic-png-v1','image/png',16,repeat('a',64),$3)`, evidenceID, caseID, resolvedAt.Add(-time.Hour)); err != nil {
			_ = conn.Close(context.Background())
			t.Fatal(err)
		}
		if err := conn.Close(ctx); err != nil {
			t.Fatal(err)
		}

		blob, err := verification.SyntheticPNG()
		if err != nil {
			t.Fatal(err)
		}
		privateRoot := t.TempDir()
		if err := os.Chmod(privateRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		store, err := evidencefs.New(privateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put(ctx, evidenceID, blob); err != nil {
			t.Fatal(err)
		}

		v47, err := os.ReadFile(filepath.Join(sourceDir, "V000047__local_privacy_retention_purgers.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(v46Dir, "V000047__local_privacy_retention_purgers.sql"), v47, 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := Run(ctx, databaseURL, v46Dir)
		if err != nil || len(result.Applied) != 1 || result.Applied[0] != 47 {
			t.Fatalf("upgrade from V46: applied=%v err=%v", result.Applied, err)
		}

		conn = connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var fileID string
		var removeAt time.Time
		var removedAt *time.Time
		var constraintValidated bool
		if err := conn.QueryRow(ctx, `SELECT archivo_id::text,retirar_archivo_en,archivo_retirado_en FROM public.verificacion_evidencia_sintetica WHERE id=$1`, evidenceID).Scan(&fileID, &removeAt, &removedAt); err != nil {
			t.Fatal(err)
		}
		if fileID != evidenceID || !removeAt.Equal(resolvedAt.AddDate(0, 0, 90)) || removedAt != nil {
			t.Fatalf("backfilled evidence reference/deadline changed: file_id=%q remove_at=%s removed_at=%v", fileID, removeAt, removedAt)
		}
		if err := conn.QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname='verificacion_evidencia_archivo_retencion_ck'`).Scan(&constraintValidated); err != nil || !constraintValidated {
			t.Fatalf("retention constraint validation=%v err=%v", constraintValidated, err)
		}
		storedBlob, err := store.Get(ctx, evidenceID)
		if err != nil || !bytes.Equal(storedBlob, blob) {
			t.Fatalf("migration changed/deleted existing private file: bytes_match=%v err=%v", bytes.Equal(storedBlob, blob), err)
		}
	})
}

func TestUpgradeFromV52BackfillsLocalDomainRetention(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		_, sourceFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("resolve migration test source path")
		}
		sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
		migrations, err := DiscoverMigrations(sourceDir)
		if err != nil {
			t.Fatal(err)
		}
		upgradeDir := t.TempDir()
		for _, migration := range migrations {
			if migration.Version > 52 {
				continue
			}
			if err := os.WriteFile(filepath.Join(upgradeDir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if result, err := Run(ctx, databaseURL, upgradeDir); err != nil || len(result.Applied) != 52 || result.Applied[51] != 52 {
			t.Fatalf("prepare V52 database: applied=%v err=%v", result.Applied, err)
		}
		actorID, spaceID := "a5300000-0000-4000-8000-000000000001", "a5300000-0000-4000-8000-000000000002"
		deletedAt := time.Date(2024, 2, 29, 18, 30, 0, 0, time.UTC)
		conn := connectTestDB(t, ctx, databaseURL)
		if _, err := conn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES ($1,'retention-v52@example.invalid','retention-v52@example.invalid','test-hash','activo')`, actorID); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria) VALUES ($1,$2,'oficina','Historical listing',repeat('Synthetic description ',6),20,2,'Synthetic rules','hora',12000,'Retired address','eliminada','UTC')`, spaceID, actorID); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.espacio_publicacion_historial_local(espacio_id,actor_id,estado_anterior,estado_nuevo,ocurrida_en,correlacion_id) VALUES ($1,$2,'borrador','activa',$3,'v52-publication')`, spaceID, actorID, deletedAt.Add(-time.Hour)); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.espacio_eliminacion_historial_local(espacio_id,actor_id,ocurrida_en,correlacion_id,archivos_pendientes,resumen) VALUES ($1,$2,$3,'v52-deletion',0,'{"removed":[],"preserved":[]}')`, spaceID, actorID, deletedAt); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		if err := conn.Close(ctx); err != nil {
			t.Fatal(err)
		}
		var v53, v54 Migration
		for _, migration := range migrations {
			if migration.Version == 53 {
				v53 = migration
			}
			if migration.Version == 54 {
				v54 = migration
			}
		}
		if v53.Version != 53 || v54.Version != 54 {
			t.Fatalf("V53/V54 migrations not discovered: V53=%d V54=%d", v53.Version, v54.Version)
		}
		for _, migration := range []Migration{v53, v54} {
			if err := os.WriteFile(filepath.Join(upgradeDir, filepath.Base(migration.Path)), migration.SQL, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		result, err := Run(ctx, databaseURL, upgradeDir)
		if err != nil || len(result.Applied) != 2 || result.Applied[0] != 53 || result.Applied[1] != 54 {
			t.Fatalf("upgrade from V52: applied=%v err=%v", result.Applied, err)
		}
		conn = connectTestDB(t, ctx, databaseURL)
		defer conn.Close(ctx)
		var publicationDeadline, deletionDeadline time.Time
		if err := conn.QueryRow(ctx, `SELECT p.retirar_en,d.retirar_en FROM public.espacio_publicacion_historial_local p JOIN public.espacio_eliminacion_historial_local d ON d.espacio_id=p.espacio_id WHERE p.espacio_id=$1`, spaceID).Scan(&publicationDeadline, &deletionDeadline); err != nil {
			t.Fatal(err)
		}
		// PostgreSQL calendar-month interval resolves an invalid anniversary to
		// the last day of February, not March 1.
		want := time.Date(2026, 2, 28, 18, 30, 0, 0, time.UTC)
		if !publicationDeadline.Equal(want) || !deletionDeadline.Equal(want) {
			t.Fatalf("backfilled retention did not use calendar months from definitive deletion: publication=%s deletion=%s want=%s", publicationDeadline, deletionDeadline, want)
		}
		var documentNullable, purgeDenied bool
		if err := conn.QueryRow(ctx, `SELECT is_nullable='YES' FROM information_schema.columns WHERE table_schema='public' AND table_name='contrato_ensayo_local' AND column_name='documento_id'`).Scan(&documentNullable); err != nil || !documentNullable {
			t.Fatalf("contract PDF reference cannot represent retired artifact: nullable=%v err=%v", documentNullable, err)
		}
		if err := conn.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_proc p CROSS JOIN LATERAL aclexplode(p.proacl) a WHERE p.oid=to_regprocedure('public.purge_expired_local_contracts_and_publication_history(timestamptz,integer)') AND a.grantee=0 AND a.privilege_type='EXECUTE')`).Scan(&purgeDenied); err != nil || !purgeDenied {
			t.Fatalf("public can execute retention purge: denied=%v err=%v", purgeDenied, err)
		}
		var signatureExpiryIndexed bool
		if err := conn.QueryRow(ctx, `SELECT indexdef LIKE '%cancelada_por_firma%' FROM pg_indexes WHERE schemaname='public' AND indexname='reserva_ensayo_retencion_local_idx'`).Scan(&signatureExpiryIndexed); err != nil || !signatureExpiryIndexed {
			t.Fatalf("V54 retention index omits signature-expiry reservations: included=%v err=%v", signatureExpiryIndexed, err)
		}
	})
}

func TestRunnerStoresExactMigrationAndRunnerChecksums(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		const migrationSQL = "CREATE TABLE checksum_probe (id integer PRIMARY KEY);\n"
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__checksum_probe.sql", migrationSQL)
		if _, err := Run(ctx, databaseURL, dir); err != nil {
			t.Fatal(err)
		}

		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var gotMigrationChecksum, gotRunnerChecksum string
		var appliedAt time.Time
		if err := conn.QueryRow(ctx, `SELECT checksum, runner_checksum, applied_at
			FROM public.schema_migrations WHERE version = 1`).Scan(
			&gotMigrationChecksum, &gotRunnerChecksum, &appliedAt); err != nil {
			t.Fatal(err)
		}
		if want := sha256Hex([]byte(migrationSQL)); gotMigrationChecksum != want {
			t.Fatalf("stored migration checksum=%s, want %s", gotMigrationChecksum, want)
		}
		wantRunnerChecksum, err := executableChecksum()
		if err != nil {
			t.Fatal(err)
		}
		if gotRunnerChecksum != wantRunnerChecksum {
			t.Fatalf("stored runner checksum=%s, want %s", gotRunnerChecksum, wantRunnerChecksum)
		}
		if appliedAt.IsZero() {
			t.Fatal("applied_at was not recorded")
		}
	})
}

func TestRunnerRejectsChecksumDriftBeforeDDL(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		path := writeMigration(t, dir, "V000001__probe.sql", "CREATE TABLE probe (id integer PRIMARY KEY);\n")
		if _, err := Run(ctx, databaseURL, dir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("CREATE TABLE drift_must_not_run (id integer);\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(ctx, databaseURL, dir); err == nil {
			t.Fatal("expected checksum drift to fail")
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var relation *string
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.drift_must_not_run')::text").Scan(&relation); err != nil {
			t.Fatal(err)
		}
		if relation != nil {
			t.Fatalf("DDL ran despite checksum drift: %v", *relation)
		}
	})
}

func TestRunnerRollsBackMigrationAndHistoryTogether(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__failing_probe.sql", "CREATE TABLE rolled_back_probe (id integer); SELECT 1 / 0;\n")
		if _, err := Run(ctx, databaseURL, dir); err == nil {
			t.Fatal("expected the invalid migration to fail")
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var relation *string
		var historyRows int
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.rolled_back_probe')::text").Scan(&relation); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if relation != nil || historyRows != 0 {
			t.Fatalf("failed migration was not atomic: relation=%v history=%d", relation, historyRows)
		}
	})
}

func TestRunnerSerializesConcurrentExecutions(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__concurrent_probe.sql", "SELECT pg_sleep(0.3); CREATE TABLE concurrent_probe (id integer PRIMARY KEY);\n")

		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := Run(ctx, databaseURL, dir)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var historyRows int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if historyRows != 1 {
			t.Fatalf("concurrent runs applied migration %d times", historyRows)
		}
	})
}

func TestRunnerRejectsVersionGapBeforeDDL(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__first.sql", "CREATE TABLE first_probe (id integer);\n")
		writeMigration(t, dir, "V000003__third.sql", "CREATE TABLE third_probe (id integer);\n")
		if _, err := Run(ctx, databaseURL, dir); err == nil {
			t.Fatal("expected version gap to fail")
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var historyExists bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&historyExists); err != nil {
			t.Fatal(err)
		}
		if historyExists {
			var historyRows int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
				t.Fatal(err)
			}
			if historyRows != 0 {
				t.Fatalf("invalid sequence changed history: %d rows", historyRows)
			}
		}
	})
}

func TestGCP06TestAccessMigrationV66LeastPrivilege(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration source path")
	}
	sourceDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	migrations, err := DiscoverMigrations(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 66 || migrations[65].Name != "V000066__gcp06_test_access_reset.sql" {
		t.Fatalf("unexpected GCP-06 migration tail: count=%d latest=%q", len(migrations), migrations[len(migrations)-1].Name)
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		result, err := Run(ctx, databaseURL, sourceDir)
		if err != nil || len(result.Applied) != 66 || result.Applied[65] != 66 {
			t.Fatalf("fresh V1-V66 migration: applied=%v err=%v", result.Applied, err)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var canLogin, super, createRole, inherit, hashRead, hashWrite, eventDelete, publicExecute bool
		if err := conn.QueryRow(ctx, `SELECT r.rolcanlogin,r.rolsuper,r.rolcreaterole,r.rolinherit,
			has_column_privilege('espacigo_gcp06_test_access','public.usuario','hash_clave','SELECT'),
			has_column_privilege('espacigo_gcp06_test_access','public.usuario','hash_clave','UPDATE'),
			has_table_privilege('espacigo_gcp06_test_access','public.gcp06_test_access_batch_audit','DELETE'),
			EXISTS(SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) a
				WHERE p.oid='public.gcp06_test_access_target_ids()'::regprocedure AND a.grantee=0 AND a.privilege_type='EXECUTE')
			FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_gcp06_test_access'`).Scan(&canLogin, &super, &createRole, &inherit, &hashRead, &hashWrite, &eventDelete, &publicExecute); err != nil {
			t.Fatal(err)
		}
		if canLogin || super || createRole || inherit || hashRead || !hashWrite || eventDelete || publicExecute {
			t.Fatalf("GCP-06 reset role privilege mismatch: login=%v super=%v create_role=%v inherit=%v hash_read=%v hash_write=%v audit_delete=%v public_execute=%v", canLogin, super, createRole, inherit, hashRead, hashWrite, eventDelete, publicExecute)
		}
		var targetIDs int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.gcp06_test_access_target_ids()`).Scan(&targetIDs); err != nil || targetIDs != 0 {
			t.Fatalf("empty candidate target function returned unexpected rows: count=%d err=%v", targetIDs, err)
		}
	})
	t.Cleanup(func() {
		admin, err := pgx.Connect(context.Background(), adminURL)
		if err != nil {
			t.Errorf("connect to clean disposable GCP-06 role: %v", err)
			return
		}
		defer admin.Close(context.Background())
		if _, err := admin.Exec(context.Background(), "DROP ROLE IF EXISTS espacigo_gcp06_test_access"); err != nil {
			t.Errorf("drop disposable GCP-06 test role: %v", err)
		}
	})
}

func withTestDatabase(t *testing.T, test func(context.Context, string)) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to disposable PostgreSQL: %v", err)
	}
	defer admin.Close(context.Background())
	databaseName := fmt.Sprintf("migrator_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		t.Fatalf("create disposable test database: %v", err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
			t.Errorf("drop disposable test database: %v", err)
		}
	}()
	databaseURLParsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseURLParsed.Path = "/" + databaseName
	testURL := databaseURLParsed.String()
	testConn := connectTestDB(t, ctx, testURL)
	if err := testConn.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	test(ctx, testURL)
}

func connectTestDB(t *testing.T, ctx context.Context, databaseURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func writeMigration(t *testing.T, dir, name, sql string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
