package rentalflowpg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func validateDisposablePostgresTarget(adminURL, token string) (*url.URL, error) {
	adminConfig, err := url.Parse(adminURL)
	if err != nil || adminConfig.Hostname() != "localhost" || adminConfig.Path != "/postgres" || adminConfig.User == nil || adminConfig.User.Username() != "postgres" || adminConfig.Query().Get("host") == "" {
		return nil, errors.New("disposable PostgreSQL integration requires the harness's local Unix-socket target")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("disposable PostgreSQL integration token is missing")
	}
	marker, err := os.ReadFile(filepath.Join(adminConfig.Query().Get("host"), ".espacigo-migration-ci"))
	if err != nil || strings.TrimSpace(string(marker)) != token {
		return nil, errors.New("disposable PostgreSQL integration marker does not match the harness token")
	}
	return adminConfig, nil
}

func TestDisposablePostgresTargetRejectsMismatchedToken(t *testing.T) {
	socketDir := t.TempDir()
	marker := filepath.Join(socketDir, ".espacigo-migration-ci")
	if err := os.WriteFile(marker, []byte("actual-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	adminURL := "postgres://postgres@localhost/postgres?host=" + url.QueryEscape(socketDir)
	if _, err := validateDisposablePostgresTarget(adminURL, "actual-token"); err != nil {
		t.Fatalf("matching harness token rejected: %v", err)
	}
	if _, err := validateDisposablePostgresTarget(adminURL, "different-token"); err == nil {
		t.Fatal("target accepted a marker whose token does not match")
	}
	if _, err := validateDisposablePostgresTarget(adminURL, ""); err == nil {
		t.Fatal("target accepted a missing harness token")
	}
}

func TestPostgresPublicIntervalsUsesWeeklyScheduleAndOccupancy(t *testing.T) {
	if os.Getenv("TEST_RENTALFLOW_POSTGRES_ISOLATED") != "1" {
		t.Skip("TEST_RENTALFLOW_POSTGRES_ISOLATED=1 is required for disposable PostgreSQL integration")
	}
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for disposable PostgreSQL integration")
	}
	adminConfig, err := validateDisposablePostgresTarget(adminURL, os.Getenv("TEST_RENTALFLOW_POSTGRES_ISOLATION_TOKEN"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Errorf("close disposable PostgreSQL admin connection: %v", err)
		}
	})

	dbName := fmt.Sprintf("rentalflow_availability_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("drop owned disposable database: %v", err)
		}
	})
	dbURL := adminConfig
	dbURL.Path = "/" + dbName
	migrations, err := migrator.DiscoverMigrations("../../../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrationPrefix := t.TempDir()
	for _, migration := range migrations {
		if migration.Version > 61 {
			break
		}
		if err := os.WriteFile(filepath.Join(migrationPrefix, migration.Name), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = migrator.Run(ctx, dbURL.String(), migrationPrefix); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dbURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	ownerID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	spaceID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if _, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-test-hash','activo')`, ownerID, ownerID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria) VALUES($1,$2,'oficina','Oficina de prueba',$3,20,4,'No fumar','hora',9000,'Dirección sintética','borrador','UTC')`, spaceID, ownerID, strings.Repeat("Descripción sintética. ", 6)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',9000)`, spaceID); err != nil {
		t.Fatal(err)
	}

	const dateText = "2030-04-01"
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		t.Fatal(err)
	}
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo) VALUES($1,true)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,$2,1,780,900)`, spaceID, weekday); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,intervalo,tipo,motivo) VALUES('dddddddd-dddd-4ddd-8ddd-dddddddddddd',$1,tstzrange($2::timestamptz,$3::timestamptz,'[)'),'bloqueo_manual','Bloqueo sintético de integración')`, spaceID, dateText+"T13:00:00Z", dateText+"T14:00:00Z"); err != nil {
		t.Fatal(err)
	}

	repo, err := NewRepository(pool, pool, func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.PublicIntervals(ctx, rentalflow.Listing{
		ID: spaceID, RateUnit: "hora", TimeZone: "UTC",
	}, dateText, 1)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Fatalf("PublicIntervals PostgreSQL error code=%s position=%d message=%s", pgErr.Code, pgErr.Position, pgErr.Message)
		}
		t.Fatalf("PublicIntervals: %v", err)
	}
	if len(got) != 1 || !got[0].StartAt.Equal(date.Add(14*time.Hour)) || !got[0].EndAt.Equal(date.Add(15*time.Hour)) {
		t.Fatalf("public intervals = %#v, want only 14:00-15:00 after schedule and occupied range filtering", got)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,6,1,1380,1440)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.espacio_horario_semanal SET activo=false WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.espacio SET zona_horaria='America/Santiago' WHERE id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.espacio_horario_semanal SET activo=true WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	var foldTimeZone string
	if err := pool.QueryRow(ctx, `SELECT zona_horaria FROM public.espacio WHERE id=$1`, spaceID).Scan(&foldTimeZone); err != nil {
		t.Fatal(err)
	}
	if foldTimeZone != "America/Santiago" {
		t.Fatalf("DST fold listing timezone=%q, want America/Santiago", foldTimeZone)
	}
	dstIntervals, err := repo.PublicIntervals(ctx, rentalflow.Listing{
		ID: spaceID, RateUnit: "hora", TimeZone: "America/Santiago",
	}, "2026-04-04", 1)
	if err != nil {
		t.Fatalf("PublicIntervals on Santiago DST fold: %v", err)
	}
	wantDSTStarts := []time.Time{
		time.Date(2026, 4, 5, 2, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 5, 2, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 5, 3, 0, 0, 0, time.UTC),
	}
	if len(dstIntervals) != len(wantDSTStarts) {
		t.Fatalf("Santiago repeated-hour intervals=%d, want %d: %#v", len(dstIntervals), len(wantDSTStarts), dstIntervals)
	}
	for i, wantStart := range wantDSTStarts {
		if !dstIntervals[i].StartAt.Equal(wantStart) || dstIntervals[i].EndAt.Sub(dstIntervals[i].StartAt) != time.Hour {
			t.Fatalf("Santiago repeated-hour interval[%d]=[%s,%s), want start %s and one elapsed hour", i, dstIntervals[i].StartAt, dstIntervals[i].EndAt, wantStart)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,7,1,0,120)`, spaceID); err != nil {
		t.Fatal(err)
	}
	gapIntervals, err := repo.PublicIntervals(ctx, rentalflow.Listing{
		ID: spaceID, RateUnit: "hora", TimeZone: "America/Santiago",
	}, "2026-09-06", 1)
	if err != nil {
		t.Fatalf("PublicIntervals on Santiago DST gap: %v", err)
	}
	wantGapStart := time.Date(2026, 9, 6, 4, 0, 0, 0, time.UTC)
	if len(gapIntervals) != 1 || !gapIntervals[0].StartAt.Equal(wantGapStart) || gapIntervals[0].EndAt.Sub(gapIntervals[0].StartAt) != time.Hour {
		t.Fatalf("Santiago DST-gap intervals=%#v, want only the valid 01:00-02:00 local slot starting %s", gapIntervals, wantGapStart)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.espacio_horario_semanal SET activo=false WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.espacio SET zona_horaria='UTC' WHERE id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.ocupacion WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,intervalo,tipo,motivo) VALUES('eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee',$1,tstzrange($2::timestamptz,$3::timestamptz,'[)'),'bloqueo_manual','Bloqueo sintético del segundo día')`, spaceID, "2030-04-02T12:00:00Z", "2030-04-02T13:00:00Z"); err != nil {
		t.Fatal(err)
	}
	dailyIntervals, err := repo.PublicIntervals(ctx, rentalflow.Listing{
		ID: spaceID, RateUnit: "dia", TimeZone: "UTC",
	}, dateText, 2)
	if err != nil {
		t.Fatalf("PublicIntervals for a two-day rental: %v", err)
	}
	if len(dailyIntervals) != 0 {
		t.Fatalf("two-day interval survived occupancy on its second day: %#v", dailyIntervals)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.ocupacion WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,intervalo,tipo,motivo) VALUES('ffffffff-ffff-4fff-8fff-ffffffffffff',$1,tstzrange($2::timestamptz,$3::timestamptz,'[)'),'bloqueo_manual','Bloqueo sintético al final del mes')`, spaceID, "2030-04-30T12:00:00Z", "2030-04-30T13:00:00Z"); err != nil {
		t.Fatal(err)
	}
	monthlyIntervals, err := repo.PublicIntervals(ctx, rentalflow.Listing{
		ID: spaceID, RateUnit: "mes", TimeZone: "UTC",
	}, dateText, 1)
	if err != nil {
		t.Fatalf("PublicIntervals for a monthly rental: %v", err)
	}
	if len(monthlyIntervals) != 0 {
		t.Fatalf("monthly interval survived occupancy near its end: %#v", monthlyIntervals)
	}
}
