package documentverificationpg

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"testing"
	"time"
)

// This is a rollback-only metadata predicate trial, not an approval or scan.
func TestLocalIdentityCurrentDoesNotResurrectPriorApproval(t *testing.T) {
	if os.Getenv("ESPACIGO_IDENTITY_CURRENT_SQL_TRIAL") != "1" {
		t.Skip("explicit same-target metadata trial required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=espacigo_f5_gate_20261101 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(context.Background())
	var ok bool
	if err = c.QueryRow(ctx, `SELECT current_database()='espacigo_f5_gate_20261101' AND current_user='postgres' AND current_setting('server_version_num')='180006' AND (SELECT count(*) FROM public.usuario)=2 AND (SELECT count(*) FROM public.schema_migrations)=83`).Scan(&ok); err != nil || !ok {
		t.Fatal("exact target preflight failed")
	}
	var owner, reviewer string
	if err = c.QueryRow(ctx, `SELECT a.id::text,b.id::text FROM public.usuario a JOIN public.usuario b ON b.id<>a.id WHERE a.estado='activo' AND b.estado='activo' ORDER BY a.id LIMIT 1`).Scan(&owner, &reviewer); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mutation string
		want           bool
	}{
		{"current_approved", "", true},
		{"latest_revoked", `UPDATE public.document_verification_case SET state='revocada',revoked_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1`, false},
		{"latest_expired_without_worker", `UPDATE public.document_verification_case SET verified_until=now()-interval '1 second' WHERE id=$1`, false},
		{"latest_document_expired", `UPDATE public.document_verification_case SET document_expires_at=now()-interval '1 second' WHERE id=$1`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, e := c.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			var latest string
			for range 2 {
				e = tx.QueryRow(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,state,reviewer_id,verified_at,verified_until,document_expires_at,updated_at) VALUES(gen_random_uuid(),'identity',$1,'aprobada',$2,now()-interval '1 day',now()+interval '7 days',now()+interval '30 days',clock_timestamp()) RETURNING id::text`, owner, reviewer).Scan(&latest)
				if e != nil {
					t.Fatal(e)
				}
			}
			if _, e = tx.Exec(ctx, `SAVEPOINT identity_scope_guard`); e != nil {
				t.Fatal(e)
			}
			_, e = tx.Exec(ctx, `UPDATE public.document_verification_case SET superseded_at=clock_timestamp() WHERE id=$1`, latest)
			var scopeErr *pgconn.PgError
			if !errors.As(e, &scopeErr) || scopeErr.Code != "23514" || scopeErr.ConstraintName != "document_verification_case_superseded_scope_ck" {
				t.Fatal("identity supersession must be rejected by the applied schema")
			}
			if _, e = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT identity_scope_guard`); e != nil {
				t.Fatal(e)
			}
			if tc.mutation != "" {
				if _, e = tx.Exec(ctx, tc.mutation, latest); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); e != nil {
				t.Fatal(e)
			}
			current, e := identityCurrent(ctx, tx, owner)
			if e != nil || current != tc.want {
				t.Fatalf("current=%t want=%t err=%v", current, tc.want, e)
			}
			if e = tx.Rollback(ctx); e != nil {
				t.Fatal(e)
			}
			var remains bool
			if e = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.document_verification_case WHERE id=$1)`, latest).Scan(&remains); e != nil || remains {
				t.Fatal("fixture rollback readback failed")
			}
		})
	}
}
