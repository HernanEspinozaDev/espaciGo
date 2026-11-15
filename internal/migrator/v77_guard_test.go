package migrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type v77Row struct {
	definition string
	err        error
}

func (r v77Row) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("unexpected V77 scan shape")
	}
	if r.err != nil {
		return r.err
	}
	p, ok := dest[0].(*string)
	if !ok {
		return errors.New("unexpected V77 scan destination")
	}
	*p = r.definition
	return nil
}

type v77Queryer struct {
	row   v77Row
	calls int
}

func (q *v77Queryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.calls++
	if query != `SELECT pg_catalog.pg_get_functiondef('public.img01b_enforce_gallery_slot()'::regprocedure)` || len(args) != 0 {
		return v77Row{err: errors.New("unexpected V77 query shape")}
	}
	return q.row
}

func TestV77PredecessorGuard(t *testing.T) {
	// pg_get_functiondef returns the decoded function body, not the SQL
	// string literal's doubled quotes in the immutable migration source.
	const fragment = "IF TG_TABLE_NAME='media_asset' AND NEW.replacement_target_space_media_id IS NOT NULL THEN"
	base := "BEGIN " + fragment + " RETURN NEW; END IF; END;"
	for _, tc := range []struct {
		name, definition string
		err              error
		want             string
	}{
		{"exactly-one", base, nil, ""},
		{"zero", "BEGIN RETURN NEW; END;", nil, "found 0"},
		{"empty-definition", "", nil, "found 0"},
		{"duplicate", base + " -- " + fragment, nil, "found 2"},
		{"query-error", "", errors.New("query failed"), "could not read"},
		{"missing-function", "", &pgconn.PgError{Code: "42883", Message: "function missing"}, "could not read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &v77Queryer{row: v77Row{definition: tc.definition, err: tc.err}}
			err := guardV77Predecessor(context.Background(), q)
			if q.calls != 1 {
				t.Fatalf("expected one catalog query, got %d", q.calls)
			}
			if tc.want == "" {
				if err != nil {
					t.Fatalf("valid predecessor rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "V77 predecessor-function guard") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted V77 guard %q, got %v", tc.want, err)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("catalog failure was not preserved: %v", err)
			}
		})
	}
}

func TestV77OriginalFragmentMatchesImmutableSQL(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", v77GalleryMigration))
	if err != nil {
		t.Fatal(err)
	}
	literal := "'" + strings.ReplaceAll(v77OriginalFragment, "'", "''") + "'"
	if strings.Count(string(source), literal) != 1 {
		t.Fatal("decoded guard fragment does not match the immutable SQL literal")
	}
}
