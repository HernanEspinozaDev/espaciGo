package publiccatalogpg

import (
	"context"
	"github.com/jackc/pgx/v5"
	"os"
	"testing"
	"time"
)

// Metadata-only adversarial fixtures, each rolled back. This does not claim
// scanner inspection or a product transition; the native trial covers those.
func TestLocalPublicVerificationPredicatesRollback(t *testing.T) {
	if os.Getenv("ESPACIGO_PUBLIC_VERIFICATION_SQL_TRIAL") != "1" {
		t.Skip("explicit same-target synthetic SQL trial required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=espacigo_f5_gate_20261101 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var db, user string
	var version, accounts, ledger int
	err = conn.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int,(SELECT count(*) FROM public.usuario),(SELECT count(*) FROM public.schema_migrations)`).Scan(&db, &user, &version, &accounts, &ledger)
	if err != nil || db != "espacigo_f5_gate_20261101" || user != "postgres" || version != 180006 || accounts != 2 || ledger != 83 {
		t.Fatal("exact synthetic target preflight failed")
	}
	var space, owner, reviewer string
	err = conn.QueryRow(ctx, `SELECT e.id::text,e.propietario_id::text,(SELECT id::text FROM public.usuario WHERE id<>e.propietario_id AND estado='activo' LIMIT 1) FROM public.espacio e JOIN public.usuario u ON u.id=e.propietario_id AND u.estado='activo' WHERE e.estado='activa' AND EXISTS(SELECT 1 FROM public.tarifa_espacio WHERE espacio_id=e.id) AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local_fixture WHERE espacio_id=e.id AND habilitada) AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=e.propietario_id) ORDER BY e.id LIMIT 1`).Scan(&space, &owner, &reviewer)
	if err != nil {
		t.Fatal("existing active synthetic fixture required")
	}
	cases := []struct {
		name, mutation  string
		identity, space bool
	}{
		{"approved_current_owner", "", true, true},
		{"latest_identity_revoked_no_old_approval_resurrection", `UPDATE public.document_verification_case SET state='revocada',revoked_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1`, false, false},
		{"latest_space_revoked_no_old_approval_resurrection", `UPDATE public.document_verification_case SET state='revocada',revoked_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$2`, true, false},
		{"identity_expired_without_worker", `UPDATE public.document_verification_case SET verified_until=now()-interval '1 second' WHERE id=$1`, false, false},
		{"space_expired_without_worker", `UPDATE public.document_verification_case SET verified_until=now()-interval '1 second' WHERE id=$2`, true, false},
		{"identity_document_expired", `UPDATE public.document_verification_case SET document_expires_at=now()-interval '1 second' WHERE id=$1`, false, false},
		{"space_document_expired", `UPDATE public.document_verification_case SET document_expires_at=now()-interval '1 second' WHERE id=$2`, true, false},
		{"future_identity_approval", `UPDATE public.document_verification_case SET verified_at=now()+interval '1 hour' WHERE id=$1`, false, false},
		{"future_space_approval", `UPDATE public.document_verification_case SET verified_at=now()+interval '1 hour' WHERE id=$2`, true, false},
		{"superseded_space", `UPDATE public.document_verification_case SET superseded_at=clock_timestamp() WHERE id=$2`, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, e := conn.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			insert := func(purpose string, spaceID any) string {
				t.Helper()
				var id string
				e := tx.QueryRow(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id,state,reviewer_id,verified_at,verified_until,document_expires_at,updated_at) VALUES(gen_random_uuid(),$1,$2,$3,'aprobada',$4,now()-interval '1 day',now()+interval '7 days',now()+interval '30 days',clock_timestamp()) RETURNING id::text`, purpose, owner, spaceID, reviewer).Scan(&id)
				if e != nil {
					t.Fatal(e)
				}
				return id
			}
			// The older valid decision must not resurrect when the newer one is revoked.
			_ = insert("identity", nil)
			_ = insert("space", space)
			identityID, spaceID := insert("identity", nil), insert("space", space)
			if tc.mutation != "" {
				// Both placeholders are bound even for one-case mutations.
				query := `WITH inputs AS(SELECT $1::uuid AS identity_id,$2::uuid AS space_id) ` + tc.mutation
				if _, e = tx.Exec(ctx, query, identityID, spaceID); e != nil {
					t.Fatal(e)
				}
			}
			// Legacy KYC is intentionally absent: publication and badges are independent.
			if _, e = tx.Exec(ctx, `DELETE FROM public.elegibilidad_verificacion_local WHERE usuario_id=$1`, owner); e != nil {
				t.Fatal(e)
			}
			if _, e = tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); e != nil {
				t.Fatal(e)
			}
			var role string
			var elevated bool
			if e = tx.QueryRow(ctx, `SELECT current_user,(SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname=current_user)`).Scan(&role, &elevated); e != nil || role != "espacigo_runtime" || elevated {
				t.Fatal("limited SQL reader not established")
			}
			var identityVerified, spaceVerified bool
			e = tx.QueryRow(ctx, `SELECT verification.identity_verified,verification.space_verified`+eligiblePublicSQL+` AND e.id=$1`, space).Scan(&identityVerified, &spaceVerified)
			if e != nil {
				t.Fatal(e)
			}
			if identityVerified != tc.identity || spaceVerified != tc.space {
				t.Fatalf("flags=(%t,%t), want=(%t,%t)", identityVerified, spaceVerified, tc.identity, tc.space)
			}
			if e = tx.Rollback(ctx); e != nil {
				t.Fatal(e)
			}
			var remains bool
			if e = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.document_verification_case WHERE id=ANY($1::uuid[]))`, []string{identityID, spaceID}).Scan(&remains); e != nil || remains {
				t.Fatal("metadata fixture rollback not independently confirmed")
			}
		})
	}
}
