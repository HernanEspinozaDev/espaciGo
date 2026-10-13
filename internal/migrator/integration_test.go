package migrator

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
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
