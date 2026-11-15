package migrator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestGalleryCapacityTransitionV68ToV76(t *testing.T) {
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Skip("requires the version-scoped disposable PostgreSQL harness")
	}
	adminURL := os.Getenv("TEST_DATABASE_URL")
	u, err := url.Parse(adminURL)
	if err != nil || u.Hostname() != "localhost" || u.Path != "/postgres" || u.User == nil || u.User.Username() != "postgres" || u.Query().Get("host") == "" {
		t.Fatal("V68→V76 test requires the harness's private PostgreSQL Unix socket")
	}
	if _, err := os.Stat(filepath.Join(u.Query().Get("host"), ".espacigo-migration-ci")); err != nil {
		t.Fatal("disposable migration CI marker is missing")
	}

	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 76 || all[67].Name != "V000068__img_01b_media_schema.sql" || all[75].Name != "V000076__spaces_gallery_capacity_replacement_v2.sql" {
		t.Fatal("migration source inventory does not contain the reviewed V68→V76 prefix")
	}
	prefix := func(last int) string {
		t.Helper()
		if last < 1 || last > len(all) {
			t.Fatalf("invalid migration prefix V1–V%d", last)
		}
		dir := t.TempDir()
		for _, migration := range all[:last] {
			if err := os.WriteFile(filepath.Join(dir, migration.Name), migration.SQL, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		admin := connectTestDB(t, ctx, databaseURL)
		defer admin.Close(context.Background())
		if _, err := admin.Exec(ctx, `CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`); err != nil {
			t.Fatalf("create disposable runtime role: %v", err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			conn, err := pgx.Connect(cleanupCtx, adminURL)
			if err != nil {
				t.Errorf("connect to disposable cluster to remove runtime role: %v", err)
				return
			}
			if _, err := conn.Exec(cleanupCtx, `DROP ROLE IF EXISTS espacigo_runtime`); err != nil {
				t.Errorf("remove disposable runtime role: %v", err)
			}
			if err := conn.Close(cleanupCtx); err != nil {
				t.Errorf("close disposable cluster cleanup connection: %v", err)
			}
		})

		if result, err := Run(ctx, databaseURL, prefix(67)); err != nil || len(result.Applied) != 67 || result.Applied[66] != 67 {
			t.Fatalf("apply disposable prefix V1–V67: applied=%v err=%v", result.Applied, err)
		}
		const ownerID = "10000000-0000-4000-8000-000000000001"
		const spaceID = "20000000-0000-4000-8000-000000000001"
		mustExec := func(sql string, args ...any) {
			t.Helper()
			if _, err := admin.Exec(ctx, sql, args...); err != nil {
				t.Fatalf("execute synthetic gallery fixture: %v", err)
			}
		}
		mustExec(`INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
VALUES($1,'gallery-v68@example.invalid','gallery-v68@example.invalid','synthetic-test-hash','activo')`, ownerID)
		mustExec(`INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,
capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,actualizado_en)
VALUES($1,$2,'oficina','Gallery capacity transition',repeat('Synthetic gallery description. ',5),20,
4,'Synthetic rules','dia',9000,'Synthetic address','borrador',clock_timestamp())`, spaceID, ownerID)

		syntheticID := func(n int) string { return fmt.Sprintf("30000000-0000-4000-8000-%012d", n) }
		for i := 1; i <= 10; i++ {
			createdAt := time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC)
			mustExec(`INSERT INTO public.espacio_galeria_sintetica_local
(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
VALUES($1,$2,$3,$4,'synthetic-png-v1','image/png',repeat('a',64),1,'activa',$5,$6)`,
				syntheticID(i), spaceID, ownerID, fmt.Sprintf("40000000-0000-4000-8000-%012d", i), createdAt, fmt.Sprintf("v68-legacy-%02d", i))
		}

		if result, err := Run(ctx, databaseURL, prefix(68)); err != nil || len(result.Applied) != 1 || result.Applied[0] != 68 {
			t.Fatalf("apply V68 over ten legacy gallery rows: applied=%v err=%v", result.Applied, err)
		}
		assertLegacyBackfill := func(t *testing.T) {
			t.Helper()
			rows, err := admin.Query(ctx, `SELECT synthetic_photo_id::text,position
FROM public.gallery_item_order WHERE espacio_id=$1 ORDER BY position`, spaceID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			count := 0
			for rows.Next() {
				count++
				var id string
				var position int16
				if err := rows.Scan(&id, &position); err != nil {
					t.Fatal(err)
				}
				if id != syntheticID(count) || int(position) != count {
					t.Fatalf("V68 backfill row %d=(%s,%d), want (%s,%d)", count, id, position, syntheticID(count), count)
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if count != 10 {
				t.Fatalf("V68 backfilled %d legacy rows, want 10", count)
			}
			var grant bool
			if err := admin.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.media_asset','INSERT')`).Scan(&grant); err != nil || !grant {
				t.Fatalf("V68 conditional runtime INSERT grant missing: %v", err)
			}
		}
		if !t.Run("v68-backfill-ten-ordered", assertLegacyBackfill) {
			t.FailNow()
		}

		assertLimitError := func(t *testing.T, err error, message string) {
			t.Helper()
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.Message != message {
				t.Fatalf("capacity error=%v, want SQLSTATE 23514 and %q", err, message)
			}
		}
		v68OverflowID := "50000000-0000-4000-8000-000000000099"
		if !t.Run("v68-eleventh-slot-rejected-at-10", func(t *testing.T) {
			_, err := admin.Exec(ctx, `INSERT INTO public.media_asset
(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256)
VALUES($1,$2,$3,'espacio','ready','v68-overflow-11',repeat('b',64))`, v68OverflowID, ownerID, spaceID)
			assertLimitError(t, err, "space media limit of 10 reached")
			var count int
			if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE id=$1`, v68OverflowID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rejected V68 slot left %d asset rows: %v", count, err)
			}
		}) {
			t.FailNow()
		}

		result, err := Run(ctx, databaseURL, prefix(76))
		if err != nil || len(result.Applied) != 8 || result.Applied[7] != 76 {
			t.Fatalf("apply only V69–V76 after V68: applied=%v err=%v", result.Applied, err)
		}
		if !t.Run("v76-upgrade-preserves-legacy-and-runtime-grant", func(t *testing.T) {
			var maxVersion, versionCount int
			if err := admin.QueryRow(ctx, `SELECT max(version),count(*) FROM public.schema_migrations`).Scan(&maxVersion, &versionCount); err != nil || maxVersion != 76 || versionCount != 76 {
				t.Fatalf("ledger max/count=(%d,%d), want (76,76): %v", maxVersion, versionCount, err)
			}
			assertLegacyBackfill(t)
		}) {
			t.FailNow()
		}

		mediaID := func(n int) string { return fmt.Sprintf("50000000-0000-4000-8000-%012d", n) }
		insertAssetSQL := `INSERT INTO public.media_asset
(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256)
VALUES($1,$2,$3,'espacio','ready',$4,repeat('c',64))`
		insertSpaceSlot := func(id string, position int) {
			t.Helper()
			if _, err := admin.Exec(ctx, `INSERT INTO public.space_media(id,espacio_id,propietario_id,media_id)
VALUES($1,$2,$3,$1)`, id, spaceID, ownerID); err != nil {
				t.Fatalf("insert real gallery relation at position %d: %v", position, err)
			}
			orderID := fmt.Sprintf("70000000-0000-4000-8000-%012d", position)
			if _, err := admin.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,space_media_id,position)
VALUES($1,$2,$3,$4,$5)`, orderID, spaceID, ownerID, id, position); err != nil {
				t.Fatalf("insert gallery ordering position %d: %v", position, err)
			}
		}
		for i := 1; i <= 19; i++ {
			id := mediaID(i)
			if _, err := admin.Exec(ctx, insertAssetSQL, id, ownerID, spaceID, fmt.Sprintf("v76-capacity-%02d", i)); err != nil {
				t.Fatalf("V76 real media slot %d: %v", 10+i, err)
			}
			insertSpaceSlot(id, 10+i)
		}

		type outcome struct {
			id       string
			err      error
			closeErr error
		}
		candidateIDs := []string{mediaID(20), mediaID(21)}
		start := make(chan struct{})
		outcomes := make(chan outcome, len(candidateIDs))
		for i, id := range candidateIDs {
			conn, err := pgx.Connect(ctx, databaseURL)
			if err != nil {
				t.Fatalf("connect competing V76 writer %d: %v", i, err)
			}
			go func(conn *pgx.Conn, id string, index int) {
				<-start
				_, execErr := conn.Exec(ctx, insertAssetSQL, id, ownerID, spaceID, fmt.Sprintf("v76-race-%d", index))
				closeErr := conn.Close(context.Background())
				outcomes <- outcome{id: id, err: execErr, closeErr: closeErr}
			}(conn, id, i)
		}
		close(start)
		winnerID := ""
		successes, limitErrors := 0, 0
		for range candidateIDs {
			got := <-outcomes
			if got.closeErr != nil {
				t.Errorf("close V76 competing connection: %v", got.closeErr)
			}
			if got.err == nil {
				successes++
				winnerID = got.id
				continue
			}
			var pgErr *pgconn.PgError
			if errors.As(got.err, &pgErr) && pgErr.Code == "23514" && pgErr.Message == "space media limit of 30 reached" {
				limitErrors++
				continue
			}
			t.Fatalf("competing V76 asset %s failed unexpectedly: %v", got.id, got.err)
		}
		if !t.Run("v76-concurrent-29-to-30-allows-one-writer", func(t *testing.T) {
			if successes != 1 || limitErrors != 1 || winnerID == "" {
				t.Fatalf("concurrent reservations successes=%d limit-errors=%d winner=%q, want 1/1", successes, limitErrors, winnerID)
			}
			var occupied int
			if err := admin.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND estado='activa')+
 (SELECT count(*) FROM public.media_asset WHERE espacio_id=$1 AND proposito='espacio' AND estado IN ('pending','ready'))`, spaceID).Scan(&occupied); err != nil || occupied != 30 {
				t.Fatalf("post-race occupied=%d, want 30: %v", occupied, err)
			}
		}) {
			t.FailNow()
		}
		if !t.Run("v76-thirtieth-order-position-accepted", func(t *testing.T) {
			insertSpaceSlot(winnerID, 30)
			var count, maxPosition int
			if err := admin.QueryRow(ctx, `SELECT count(*),max(position) FROM public.gallery_item_order WHERE espacio_id=$1`, spaceID).Scan(&count, &maxPosition); err != nil || count != 30 || maxPosition != 30 {
				t.Fatalf("gallery count/max position=(%d,%d), want (30,30): %v", count, maxPosition, err)
			}
		}) {
			t.FailNow()
		}
		if !t.Run("v76-thirty-first-slot-rejected", func(t *testing.T) {
			id := mediaID(31)
			_, err := admin.Exec(ctx, insertAssetSQL, id, ownerID, spaceID, "v76-capacity-31")
			assertLimitError(t, err, "space media limit of 30 reached")
			var count int
			if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rejected V76 slot left %d asset rows: %v", count, err)
			}
		}) {
			t.FailNow()
		}
	})
}
