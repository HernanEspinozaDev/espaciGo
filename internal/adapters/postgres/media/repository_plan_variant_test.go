package mediapg

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type planRow struct {
	id  string
	err error
}

func (r planRow) Scan(dst ...any) error {
	if r.err != nil {
		return r.err
	}
	*dst[0].(*string) = r.id
	return nil
}

type planTxFake struct {
	events []string
	row    planRow
}

func (f *planTxFake) QueryRow(_ context.Context, q string, _ ...any) pgx.Row {
	f.events = append(f.events, q)
	return f.row
}
func (f *planTxFake) Exec(_ context.Context, q string, _ ...any) (pgconn.CommandTag, error) {
	f.events = append(f.events, q)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (f *planTxFake) Commit(context.Context) error   { f.events = append(f.events, "commit"); return nil }
func (f *planTxFake) Rollback(context.Context) error { return nil }

func TestPlanVariantLocksMatchingPendingAssetBeforeIntentWrite(t *testing.T) {
	f := &planTxFake{row: planRow{id: "asset"}}
	if err := planVariant(context.Background(), f, "asset", 7, media.VariantAvatar, "key"); err != nil {
		t.Fatal(err)
	}
	if len(f.events) != 2 || !strings.Contains(f.events[0], "fencing_token=$2 AND estado='pending' FOR UPDATE") || !strings.Contains(f.events[1], "INSERT INTO public.media_variant_intent") {
		t.Fatalf("unexpected operations: %#v", f.events)
	}
}

func TestPlanVariantDoesNotWriteIntentForStaleOrNonpendingAsset(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{{"stale", pgx.ErrNoRows}, {"nonpending", pgx.ErrNoRows}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &planTxFake{row: planRow{err: tc.err}}
			err := planVariant(context.Background(), f, "asset", 6, media.VariantAvatar, "key")
			if !errors.Is(err, media.ErrConflict) {
				t.Fatalf("error=%v", err)
			}
			if len(f.events) != 1 {
				t.Fatalf("intent write occurred: %#v", f.events)
			}
		})
	}
}

func TestRecordVariantLocksAssetBeforeIntentAndVariantWrites(t *testing.T) {
	f := &planTxFake{row: planRow{id: "asset"}}
	variant := media.Variant{Name: media.VariantAvatar, Provider: "gcs", Bucket: "bucket", ObjectKey: "key", Generation: 1}
	if err := recordVariant(context.Background(), f, "asset", 7, variant); err != nil {
		t.Fatal(err)
	}
	if len(f.events) != 4 || !strings.Contains(f.events[0], "fencing_token=$2 AND estado='pending' FOR UPDATE") || !strings.Contains(f.events[1], "UPDATE public.media_variant_intent") || !strings.Contains(f.events[2], "INSERT INTO public.media_variant(") {
		t.Fatalf("unexpected operations: %#v", f.events)
	}
}

func TestRecordVariantRejectsStaleAssetBeforeIntentWrite(t *testing.T) {
	f := &planTxFake{row: planRow{err: pgx.ErrNoRows}}
	variant := media.Variant{Name: media.VariantAvatar, Provider: "gcs", Bucket: "bucket", ObjectKey: "key", Generation: 1}
	if err := recordVariant(context.Background(), f, "asset", 6, variant); !errors.Is(err, media.ErrConflict) {
		t.Fatalf("error=%v, want conflict", err)
	}
	if len(f.events) != 1 {
		t.Fatalf("writes occurred before stale asset was rejected: %#v", f.events)
	}
}

func TestReplacementReplayRejectsLegacyTargetWithoutExpectedRevision(t *testing.T) {
	legacy := pgtype.Timestamptz{}
	if replacementReplayMatches("target", legacy, "target", "") {
		t.Fatal("legacy replacement with unknown ExpectedRevision was accepted")
	}
	if replacementReplayMatches("target", legacy, "target", "2026-10-27T22:18:16Z") {
		t.Fatal("legacy replacement matched a revision that was never persisted")
	}
}
