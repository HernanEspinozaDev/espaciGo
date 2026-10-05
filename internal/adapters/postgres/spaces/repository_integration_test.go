package spacespg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresDraftCRUDIsOwnerScopedAndOnlyDrafts(t *testing.T) {
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
	name := fmt.Sprintf("space_draft_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
	})
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	if _, err = migrator.Run(ctx, u.String(), "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, u.String())
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
	repo := New(pool, credentials.Generator{})
	categories, err := repo.Categories(ctx)
	if err != nil || len(categories) != 8 {
		t.Fatalf("category catalog count=%d err=%v", len(categories), err)
	}
	in := spaces.Input{CategoryCode: "oficina", Title: "Oficina", Description: strings.Repeat("Espacio privado sintético. ", 5), AreaM2: 20, Capacity: 6, UsageRules: "No fumar", RateUnit: "dia", BasePriceCLP: 9000, Address: "Dirección privada de prueba"}
	created, err := repo.Create(ctx, ownerA, in)
	if err != nil {
		t.Fatal(err)
	}
	if created.State != "borrador" {
		t.Fatalf("state=%q", created.State)
	}
	items, err := repo.ListOwn(ctx, ownerA)
	if err != nil || len(items) != 1 {
		t.Fatalf("owner list=%v err=%v", items, err)
	}
	items, err = repo.ListOwn(ctx, ownerB)
	if err != nil || len(items) != 0 {
		t.Fatalf("foreign list leaked: %v err=%v", items, err)
	}
	if _, err = repo.GetOwn(ctx, ownerB, created.ID); err != spaces.ErrNotFound {
		t.Fatalf("foreign read err=%v", err)
	}
	if _, err = repo.UpdateOwn(ctx, ownerB, created.ID, in); err != spaces.ErrNotFound {
		t.Fatalf("foreign update err=%v", err)
	}
	in.Title = "Oficina editada"
	updated, err := repo.UpdateOwn(ctx, ownerA, created.ID, in)
	if err != nil || updated.Title != in.Title {
		t.Fatalf("update=%v err=%v", updated, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.espacio SET estado='publicado' WHERE id=$1`, created.ID); err == nil {
		t.Fatal("database accepted commercial state")
	}
}
