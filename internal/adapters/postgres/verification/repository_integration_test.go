package verificationpg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
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

	// Two requests for the same owner, rejected case, and key can both pass an
	// initial lookup before either inserts. They must converge on one retry row.
	raceParent, err := r.Create(ctx, verification.Case{
		ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", OwnerID: owner, Type: "kyb",
		State: "en_revision", Provider: "local-fixture-v1",
		EvidenceRef: "fixture:ffffffff-ffff-4fff-8fff-ffffffffffff",
		Idempotency: "kyb-race-parent-0001", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Review(ctx, raceParent.ID, reviewer, false, "antecedentes_incompletos"); err != nil {
		t.Fatal(err)
	}
	const concurrentKey = "kyb-retry-race-0001"
	start := make(chan struct{})
	results := make(chan verification.Case, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{
		"88888888-8888-4888-8888-888888888888",
		"99999999-9999-4999-8999-999999999999",
	} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			result, retryErr := r.Retry(ctx, owner, raceParent.ID, verification.Case{
				ID: id, OwnerID: owner, Provider: "local-fixture-v1",
				EvidenceRef: "fixture:" + id, State: "en_revision",
				Idempotency: concurrentKey, CreatedAt: time.Now().UTC(),
			})
			results <- result
			errs <- retryErr
		}(id)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	var retryIDs []string
	for result := range results {
		retryIDs = append(retryIDs, result.ID)
	}
	for retryErr := range errs {
		if retryErr != nil {
			t.Fatalf("concurrent retry err=%v", retryErr)
		}
	}
	if len(retryIDs) != 2 || retryIDs[0] == "" || retryIDs[0] != retryIDs[1] {
		t.Fatalf("concurrent retry IDs=%v; want same non-empty case", retryIDs)
	}
	var retryCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.verificacion WHERE usuario_id=$1 AND reintento_de=$2::text::uuid AND clave_idempotencia=$3`, owner, raceParent.ID, concurrentKey).Scan(&retryCount); err != nil {
		t.Fatal(err)
	}
	if retryCount != 1 {
		t.Fatalf("concurrent retry rows=%d; want exactly one", retryCount)
	}

	// Reusing the same owner/key for a different parent remains a conflict.
	otherCase, err := r.Create(ctx, verification.Case{
		ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab", OwnerID: owner, Type: "kyb",
		State: "en_revision", Provider: "local-fixture-v1",
		EvidenceRef: "fixture:bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbc",
		Idempotency: "kyb-request-0002", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Review(ctx, otherCase.ID, reviewer, false, "antecedentes_incompletos"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Retry(ctx, owner, otherCase.ID, verification.Case{
		ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccd", OwnerID: owner,
		EvidenceRef: "fixture:dddddddd-dddd-4ddd-8ddd-ddddddddddde",
		State:       "en_revision", Idempotency: concurrentKey, CreatedAt: now,
	}); err != verification.ErrConflict {
		t.Fatalf("incompatible retry key reuse err=%v; want conflict", err)
	}
}
