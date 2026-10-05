package occupancy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresDraftAvailabilityAndManualBlocksAreOwnerScoped(t *testing.T) {
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
	dbName := fmt.Sprintf("occupancy_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+pgx.Identifier{dbName}.Sanitize()+` WITH (FORCE)`)
	})
	dbURL, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	dbURL.Path = "/" + dbName
	if _, err = migrator.Run(ctx, dbURL.String(), "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dbURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ownerA, ownerB := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	for _, id := range []string{ownerA, ownerB} {
		if _, err := pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'x','activo')`, id, id+"@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	spaceRepo := spacespg.New(pool, credentials.Generator{})
	draftInput := spaces.Input{CategoryCode: "oficina", Title: "Borrador privado", Description: strings.Repeat("Espacio sintético de prueba. ", 5), AreaM2: 20, Capacity: 4, UsageRules: "No fumar", RateUnit: "dia", BasePriceCLP: 9000, Address: "Dirección sintética", AttributeSchemaVersion: 1, Attributes: map[string]any{}}
	draftA, err := spaceRepo.Create(ctx, ownerA, draftInput)
	if err != nil {
		t.Fatal(err)
	}
	draftB, err := spaceRepo.Create(ctx, ownerB, draftInput)
	if err != nil {
		t.Fatal(err)
	}
	service, err := occupancy.NewService(New(pool), credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Availability(ctx, ownerA, draftA.ID, "2030-04-01T09:00:00Z", "2030-04-01T10:00:00Z"); !errors.Is(err, occupancy.ErrTimezoneRequired) {
		t.Fatalf("availability without configured zone err=%v", err)
	}
	if err := service.SetTimeZone(ctx, ownerB, draftA.ID, "America/Santiago"); !errors.Is(err, occupancy.ErrNotFound) {
		t.Fatalf("foreign timezone update err=%v", err)
	}
	if err := service.SetTimeZone(ctx, ownerA, draftA.ID, "America/Santiago"); err != nil {
		t.Fatal(err)
	}
	if err := service.SetTimeZone(ctx, ownerA, draftA.ID, "Mars/Olympus_Mons"); !errors.Is(err, occupancy.ErrInvalid) {
		t.Fatalf("invalid IANA zone err=%v", err)
	}
	free, err := service.Availability(ctx, ownerA, draftA.ID, "2030-04-01T12:00:00Z", "2030-04-01T13:00:00Z")
	if err != nil || !free.Available || free.TimeZone != "America/Santiago" {
		t.Fatalf("initial availability=%+v err=%v", free, err)
	}
	block, err := service.CreateBlock(ctx, ownerA, draftA.ID, occupancy.BlockInput{StartAt: "2030-04-01T12:00:00Z", EndAt: "2030-04-01T13:00:00Z", Reason: "Mantención sintética"})
	if err != nil || block.TimeZone != "America/Santiago" {
		t.Fatalf("create block=%+v err=%v", block, err)
	}
	blocked, err := service.Availability(ctx, ownerA, draftA.ID, "2030-04-01T12:30:00Z", "2030-04-01T13:30:00Z")
	if err != nil || blocked.Available {
		t.Fatalf("overlap availability=%+v err=%v", blocked, err)
	}
	adjacent, err := service.Availability(ctx, ownerA, draftA.ID, "2030-04-01T13:00:00Z", "2030-04-01T14:00:00Z")
	if err != nil || !adjacent.Available {
		t.Fatalf("adjacent availability=%+v err=%v", adjacent, err)
	}
	if _, err := service.CreateBlock(ctx, ownerA, draftA.ID, occupancy.BlockInput{StartAt: "2030-04-01T12:30:00Z", EndAt: "2030-04-01T13:30:00Z", Reason: "Conflicto sintético"}); !errors.Is(err, occupancy.ErrConflict) {
		t.Fatalf("overlapping block err=%v", err)
	}
	if _, err := service.CreateBlock(ctx, ownerA, draftA.ID, occupancy.BlockInput{StartAt: "2030-04-01T10:00:00Z", EndAt: "2030-04-01T10:00:00Z", Reason: "Intervalo vacío"}); !errors.Is(err, occupancy.ErrInvalid) {
		t.Fatalf("empty block err=%v", err)
	}
	calendar, err := service.ListBlocks(ctx, ownerA, draftA.ID, "2030-04-01T08:00:00Z", "2030-04-01T15:00:00Z")
	if err != nil || len(calendar.Items) != 1 || calendar.Items[0].ID != block.ID {
		t.Fatalf("calendar=%+v err=%v", calendar, err)
	}
	if _, err := service.ListBlocks(ctx, ownerB, draftA.ID, "2030-04-01T08:00:00Z", "2030-04-01T15:00:00Z"); !errors.Is(err, occupancy.ErrNotFound) {
		t.Fatalf("foreign list err=%v", err)
	}
	if err := service.DeleteBlock(ctx, ownerB, draftA.ID, block.ID); !errors.Is(err, occupancy.ErrNotFound) {
		t.Fatalf("foreign delete err=%v", err)
	}
	if err := service.DeleteBlock(ctx, ownerA, draftB.ID, block.ID); !errors.Is(err, occupancy.ErrNotFound) {
		t.Fatalf("wrong-space delete err=%v", err)
	}
	if err := service.DeleteBlock(ctx, ownerA, draftA.ID, block.ID); err != nil {
		t.Fatal(err)
	}
	free, err = service.Availability(ctx, ownerA, draftA.ID, "2030-04-01T09:30:00Z", "2030-04-01T09:45:00Z")
	if err != nil || !free.Available {
		t.Fatalf("availability after delete=%+v err=%v", free, err)
	}
	calendar, err = service.ListBlocks(ctx, ownerA, draftA.ID, "2030-04-01T08:00:00Z", "2030-04-01T15:00:00Z")
	if err != nil || len(calendar.Items) != 0 {
		t.Fatalf("active block list after delete=%+v err=%v", calendar, err)
	}
	var active, retained int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE activo), count(*) FROM public.ocupacion WHERE id=$1`, block.ID).Scan(&active, &retained); err != nil || active != 0 || retained != 1 {
		t.Fatalf("soft deletion active=%d retained=%d err=%v", active, retained, err)
	}
}
