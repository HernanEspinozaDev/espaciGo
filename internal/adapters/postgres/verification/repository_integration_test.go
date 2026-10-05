package verificationpg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestVerificationRepositoryOwnershipIdempotencyReviewAndRetry(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	database := fmt.Sprintf("kyc_repo_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize())
		if err != nil {
			t.Errorf("drop disposable database: %v", err)
		}
	}()
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + database
	testURL := parsed.String()
	if _, err := migrator.Run(ctx, testURL, "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	owner := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	other := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	reviewer := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	for _, id := range []string{owner, other, reviewer} {
		if _, err := pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-hash','activo')`, id, id+"@ejemplo.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	r := New(pool)
	now := time.Now().UTC()
	first := verification.Case{ID: "11111111-1111-4111-8111-111111111111", OwnerID: owner, Type: "kyb", State: "en_revision", Provider: "local-fixture-v1", EvidenceRef: "fixture:22222222-2222-4222-8222-222222222222", Idempotency: "kyb-request-0001", CreatedAt: now}
	created, err := r.Create(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	replayed := first
	replayed.ID = "33333333-3333-4333-8333-333333333333"
	replayed.EvidenceRef = "fixture:44444444-4444-4444-8444-444444444444"
	duplicate, err := r.Create(ctx, replayed)
	if err != nil || duplicate.ID != created.ID {
		t.Fatalf("idempotent create=%+v err=%v", duplicate, err)
	}
	if _, err := r.GetOwn(ctx, other, created.ID); err != verification.ErrNotFound {
		t.Fatalf("foreign owner read err=%v", err)
	}
	if items, err := r.ListPending(ctx); err != nil || len(items) != 1 {
		t.Fatalf("pending count=%d err=%v", len(items), err)
	}
	rejected, err := r.Review(ctx, created.ID, reviewer, false, "antecedentes_incompletos")
	if err != nil || rejected.State != "rechazada" || rejected.ResolvedAt == nil {
		t.Fatalf("reject=%+v err=%v", rejected, err)
	}
	if _, err := r.Review(ctx, created.ID, reviewer, true, ""); err != verification.ErrConflict {
		t.Fatalf("second review err=%v", err)
	}
	retry := verification.Case{ID: "55555555-5555-4555-8555-555555555555", OwnerID: owner, Provider: "local-fixture-v1", EvidenceRef: "fixture:66666666-6666-4666-8666-666666666666", State: "en_revision", Idempotency: "kyb-retry-0001", CreatedAt: now}
	newCase, err := r.Retry(ctx, owner, created.ID, retry)
	if err != nil || newCase.RetryOf != created.ID || newCase.Type != "kyb" {
		t.Fatalf("retry=%+v err=%v", newCase, err)
	}
	retry.ID = "77777777-7777-4777-8777-777777777777"
	again, err := r.Retry(ctx, owner, created.ID, retry)
	if err != nil || again.ID != newCase.ID {
		t.Fatalf("idempotent retry=%+v err=%v", again, err)
	}
}
