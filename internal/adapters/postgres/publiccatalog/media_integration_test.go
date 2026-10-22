package publiccatalogpg

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIMG01CPublicMediaQueryUsesEligibleMediaProjection(t *testing.T) {
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
	images, err := r.publicImages(ctx, "00000000-0000-4000-8000-000000000001")
	if err != nil || len(images) != 0 {
		t.Fatalf("empty public images=%v err=%v", images, err)
	}
	_, err = r.PublicMediaVariant(ctx, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "display")
	if !errors.Is(err, publiccatalog.ErrNotFound) {
		t.Fatalf("missing public media query error=%v", err)
	}
}
