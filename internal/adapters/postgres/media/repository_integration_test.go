package mediapg

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIMG01CMigrationBackedRepositoryQueries(t *testing.T) {
	url := os.Getenv("IMG01C_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IMG01C_TEST_DATABASE_URL to a disposable V70 database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	r := New(pool)
	items, err := r.ListSpace(ctx, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002")
	if err != nil || len(items) != 0 {
		t.Fatalf("empty media list=%v err=%v", items, err)
	}
	cleanup, err := r.ClaimCleanup(ctx, 10, time.Now().UTC())
	if err != nil || len(cleanup) != 0 {
		t.Fatalf("empty cleanup=%v err=%v", cleanup, err)
	}
	if err = r.PlanVariant(ctx, "00000000-0000-4000-8000-000000000003", 1, media.VariantDisplay, "media/v1/spaces/00000000-0000-4000-8000-000000000002/00000000-0000-4000-8000-000000000003/1/display.webp"); !errors.Is(err, media.ErrConflict) {
		t.Fatalf("missing reservation PlanVariant error=%v", err)
	}
}
