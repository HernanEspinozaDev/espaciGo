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
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+pgx.Identifier{dbName}.Sanitize()+` WITH (FORCE)`)
	})
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + dbName
	if _, err = migrator.Run(ctx, u.String(), "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
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
	first, err := service.Simulate(ctx, owner, draft.ID, pricing.SimulationInput{StartAt: start, EndAt: end})
	if err != nil {
		t.Fatal(err)
	}
	if first.RateVersion != 1 || first.BasePrice != 8000 || first.BilledUnits != 2 || first.Subtotal != 16000 || !first.Private {
		t.Fatalf("unexpected simulation snapshot: %+v", first)
	}
	if _, err = service.UpdateRate(ctx, owner, draft.ID, pricing.RateInput{Unit: "hora", Amount: 10000}); err != nil {
		t.Fatal(err)
	}
	previous, err := service.GetSimulation(ctx, owner, draft.ID, first.ID)
	if err != nil || previous.RateVersion != 1 || previous.BasePrice != 8000 || previous.Subtotal != 16000 {
		t.Fatalf("previous simulation changed: %+v err=%v", previous, err)
	}
	history, err := service.RateHistory(ctx, owner, draft.ID)
	if err != nil || len(history) != 2 || history[0].Version != 1 || history[1].Version != 2 {
		t.Fatalf("rate history=%+v err=%v", history, err)
	}
	var occupancyCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1`, draft.ID).Scan(&occupancyCount); err != nil || occupancyCount != 0 {
		t.Fatalf("simulation mutated occupancy count=%d err=%v", occupancyCount, err)
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
