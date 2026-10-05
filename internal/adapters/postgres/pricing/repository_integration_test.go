package pricingpg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresPrivateSimulationSnapshotsRateAndDoesNotOccupyCalendar(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	dbName := fmt.Sprintf("private_price_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + dbName
	databaseURL := u.String()
	if _, err = migrator.Run(ctx, databaseURL, "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	const runtimePassword = "runtime-integration-only"
	if _, err = admin.Exec(ctx, `CREATE ROLE espacigo_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD 'runtime-integration-only'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		cleanupAdmin, cleanupErr := pgx.Connect(cleanupCtx, adminURL)
		if cleanupErr != nil {
			t.Errorf("connect to disposable PostgreSQL for cleanup: %v", cleanupErr)
			return
		}
		if _, cleanupErr = cleanupAdmin.Exec(cleanupCtx, `DROP DATABASE `+pgx.Identifier{dbName}.Sanitize()+` WITH (FORCE)`); cleanupErr != nil {
			t.Errorf("drop disposable database: %v", cleanupErr)
		}
		if _, cleanupErr = cleanupAdmin.Exec(cleanupCtx, `DROP ROLE IF EXISTS espacigo_runtime`); cleanupErr != nil {
			t.Errorf("drop disposable runtime role: %v", cleanupErr)
		}
		_ = cleanupAdmin.Close(context.Background())
	})
	bootstrap, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bootstrap.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{dbName}.Sanitize()+` TO espacigo_runtime`); err != nil {
		_ = bootstrap.Close(context.Background())
		t.Fatal(err)
	}
	if err = dbbootstrap.GrantRuntimePermissions(ctx, bootstrap); err != nil {
		_ = bootstrap.Close(context.Background())
		t.Fatal(err)
	}
	if err = bootstrap.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtimeURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	runtimeURL.User = url.UserPassword("espacigo_runtime", runtimePassword)
	pool, err := pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	var roleName string
	if err = pool.QueryRow(ctx, `SELECT current_user`).Scan(&roleName); err != nil || roleName != "espacigo_runtime" {
		t.Fatalf("integration connection user=%q err=%v", roleName, err)
	}
	var rateSelect, rateInsert, rateUpdate, rateDelete bool
	var simulationSelect, simulationInsert, simulationUpdate, simulationDelete bool
	if err = pool.QueryRow(ctx, `SELECT
 has_table_privilege(current_user,'public.tarifa_espacio','SELECT'),
 has_table_privilege(current_user,'public.tarifa_espacio','INSERT'),
 has_table_privilege(current_user,'public.tarifa_espacio','UPDATE'),
 has_table_privilege(current_user,'public.tarifa_espacio','DELETE'),
 has_table_privilege(current_user,'public.simulacion_precio_privada','SELECT'),
 has_table_privilege(current_user,'public.simulacion_precio_privada','INSERT'),
 has_table_privilege(current_user,'public.simulacion_precio_privada','UPDATE'),
 has_table_privilege(current_user,'public.simulacion_precio_privada','DELETE')`).Scan(&rateSelect, &rateInsert, &rateUpdate, &rateDelete, &simulationSelect, &simulationInsert, &simulationUpdate, &simulationDelete); err != nil {
		t.Fatal(err)
	}
	if !rateSelect || !rateInsert || rateUpdate || rateDelete || !simulationSelect || !simulationInsert || simulationUpdate || simulationDelete {
		t.Fatalf("runtime privileges: rates select/insert/update/delete=%v/%v/%v/%v simulations=%v/%v/%v/%v", rateSelect, rateInsert, rateUpdate, rateDelete, simulationSelect, simulationInsert, simulationUpdate, simulationDelete)
	}
	defer pool.Close()
	owner := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'x','activo')`, owner, "price@example.test"); err != nil {
		t.Fatal(err)
	}
	spaceRepo := spacespg.New(pool, credentials.Generator{})
	draft, err := spaceRepo.Create(ctx, owner, spaces.Input{CategoryCode: "oficina", Title: "Borrador privado", Description: strings.Repeat("Espacio privado sintético. ", 5), AreaM2: 25, Capacity: 4, UsageRules: "Respetar horarios", RateUnit: "hora", BasePriceCLP: 8000, Address: "Dirección sintética", AttributeSchemaVersion: 1, Attributes: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	draftInput := spaces.Input{CategoryCode: "oficina", Title: "Borrador privado editado", Description: strings.Repeat("Espacio privado sintético editado. ", 5), AreaM2: 25, Capacity: 4, UsageRules: "Respetar horarios", RateUnit: "hora", BasePriceCLP: 8000, Address: "Dirección sintética", AttributeSchemaVersion: 1, Attributes: map[string]any{}}
	if _, err = spaceRepo.UpdateOwn(ctx, owner, draft.ID, draftInput); err != nil {
		t.Fatalf("runtime role could not edit draft: %v", err)
	}
	calendar, err := occupancy.NewService(occupancypg.New(pool), credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	if err = calendar.SetTimeZone(ctx, owner, draft.ID, "America/Santiago"); err != nil {
		t.Fatal(err)
	}
	service, err := pricing.NewService(New(pool), calendar, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	start, end := "2030-04-01T12:00:00Z", "2030-04-01T13:30:00Z"
	current, err := service.CurrentRate(ctx, owner, draft.ID)
	if err != nil || current.Version != 1 || current.Amount != 8000 {
		t.Fatalf("runtime current rate=%+v err=%v", current, err)
	}
	history, err := service.RateHistory(ctx, owner, draft.ID)
	if err != nil || len(history) != 1 || history[0].Version != 1 {
		t.Fatalf("runtime initial rate history=%+v err=%v", history, err)
	}
	first, err := service.Simulate(ctx, owner, draft.ID, pricing.SimulationInput{StartAt: start, EndAt: end})
	if err != nil {
		t.Fatal(err)
	}
	if first.RateVersion != 1 || first.BasePrice != 8000 || first.BilledUnits != 2 || first.Subtotal != 16000 || !first.Private {
		t.Fatalf("unexpected simulation snapshot: %+v", first)
	}
	var occupancyCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1`, draft.ID).Scan(&occupancyCount); err != nil || occupancyCount != 0 {
		t.Fatalf("price simulation changed occupancy count=%d err=%v", occupancyCount, err)
	}
	if _, err = service.UpdateRate(ctx, owner, draft.ID, pricing.RateInput{Unit: "hora", Amount: 10000}); err != nil {
		t.Fatal(err)
	}
	previous, err := service.GetSimulation(ctx, owner, draft.ID, first.ID)
	if err != nil || previous.RateVersion != 1 || previous.BasePrice != 8000 || previous.Subtotal != 16000 {
		t.Fatalf("previous simulation changed: %+v err=%v", previous, err)
	}
	history, err = service.RateHistory(ctx, owner, draft.ID)
	if err != nil || len(history) != 2 || history[0].Version != 1 || history[1].Version != 2 {
		t.Fatalf("rate history=%+v err=%v", history, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.tarifa_espacio SET precio_base_clp=1 WHERE espacio_id=$1 AND version=1`, draft.ID); err == nil {
		t.Fatal("runtime role unexpectedly modified immutable rate history")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM public.tarifa_espacio WHERE espacio_id=$1 AND version=1`, draft.ID); err == nil {
		t.Fatal("runtime role unexpectedly deleted immutable rate history")
	}
	if _, err = pool.Exec(ctx, `UPDATE public.simulacion_precio_privada SET subtotal_clp=1 WHERE id=$1`, first.ID); err == nil {
		t.Fatal("runtime role unexpectedly modified immutable simulation snapshot")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM public.simulacion_precio_privada WHERE id=$1`, first.ID); err == nil {
		t.Fatal("runtime role unexpectedly deleted immutable simulation snapshot")
	}
	if _, err = calendar.CreateBlock(ctx, owner, draft.ID, occupancy.BlockInput{StartAt: start, EndAt: end, Reason: "bloqueo sintético"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Simulate(ctx, owner, draft.ID, pricing.SimulationInput{StartAt: start, EndAt: end}); err != pricing.ErrConflict {
		t.Fatalf("unavailable interval error=%v", err)
	}
	foreign, err := service.GetSimulation(ctx, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", draft.ID, first.ID)
	if err != pricing.ErrNotFound {
		t.Fatalf("foreign snapshot=%+v err=%v", foreign, err)
	}
}
