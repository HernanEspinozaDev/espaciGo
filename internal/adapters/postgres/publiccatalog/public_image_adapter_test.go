package publiccatalogpg

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/jackc/pgx/v5"
)

type publicImageAdapterPool struct {
	row   []any
	err   error
	args  []any
	query string
}

func (*publicImageAdapterPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (p *publicImageAdapterPool) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	p.query = query
	p.args = append([]any(nil), args...)
	return publicImageAdapterRow{row: p.row, err: p.err}
}

type publicImageAdapterRow struct {
	row []any
	err error
}

func (r publicImageAdapterRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.row) != len(dest) {
		return fmt.Errorf("fixture columns=%d scan destinations=%d", len(r.row), len(dest))
	}
	for i, destination := range dest {
		switch out := destination.(type) {
		case *string:
			value, ok := r.row[i].(string)
			if !ok {
				return fmt.Errorf("column %d expected string, got %T", i, r.row[i])
			}
			*out = value
		case *int64:
			value, ok := r.row[i].(int64)
			if !ok {
				return fmt.Errorf("column %d expected int64, got %T", i, r.row[i])
			}
			*out = value
		default:
			return fmt.Errorf("column %d unsupported scan destination %T", i, destination)
		}
	}
	return nil
}

func TestPublicImageAdaptersScanMetadataAndForwardIDs(t *testing.T) {
	const (
		spaceID = "11111111-1111-4111-8111-111111111111"
		imageID = "22222222-2222-4222-8222-222222222222"
	)
	cases := []struct {
		name  string
		row   []any
		args  []any
		want  publiccatalog.PublicImage
		fetch func(*Repository) (publiccatalog.PublicImage, error)
	}{
		{
			name: "synthetic PNG",
			row:  []any{"image/png", "a1b2c3", "file-synthetic-01", int64(128)},
			args: []any{spaceID, imageID},
			want: publiccatalog.PublicImage{MIME: "image/png", SHA256: "a1b2c3", FileID: "file-synthetic-01", Size: 128},
			fetch: func(repo *Repository) (publiccatalog.PublicImage, error) {
				return repo.PublicImage(context.Background(), spaceID, imageID)
			},
		},
		{
			name: "media WebP variant",
			row:  []any{"image/webp", "d4e5f6", int64(256), "synthetic/space/image/display.webp", "fixture-bucket", int64(7)},
			args: []any{spaceID, imageID, "display"},
			want: publiccatalog.PublicImage{MIME: "image/webp", SHA256: "d4e5f6", Size: 256, ObjectKey: "synthetic/space/image/display.webp", Bucket: "fixture-bucket", Generation: 7},
			fetch: func(repo *Repository) (publiccatalog.PublicImage, error) {
				return repo.PublicMediaVariant(context.Background(), spaceID, imageID, "display")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := &publicImageAdapterPool{row: tc.row}
			got, err := tc.fetch(&Repository{pool: pool})
			if err != nil {
				t.Fatalf("fetch image metadata: %v", err)
			}
			if got != tc.want {
				t.Errorf("metadata=%+v, want %+v", got, tc.want)
			}
			if !reflect.DeepEqual(pool.args, tc.args) {
				t.Errorf("query args=%v, want %v", pool.args, tc.args)
			}
			if pool.query == "" {
				t.Fatal("adapter did not execute a metadata query")
			}
		})
	}
}

func TestPublicImageAdaptersMapMissingRowsToNotFound(t *testing.T) {
	const (
		spaceID = "11111111-1111-4111-8111-111111111111"
		imageID = "22222222-2222-4222-8222-222222222222"
	)
	cases := []struct {
		name  string
		fetch func(*Repository) error
	}{
		{
			name: "synthetic PNG",
			fetch: func(repo *Repository) error {
				_, err := repo.PublicImage(context.Background(), spaceID, imageID)
				return err
			},
		},
		{
			name: "media WebP variant",
			fetch: func(repo *Repository) error {
				_, err := repo.PublicMediaVariant(context.Background(), spaceID, imageID, "display")
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := &publicImageAdapterPool{err: pgx.ErrNoRows}
			err := tc.fetch(&Repository{pool: pool})
			if !errors.Is(err, publiccatalog.ErrNotFound) {
				t.Fatalf("error=%v, want ErrNotFound", err)
			}
		})
	}
}
