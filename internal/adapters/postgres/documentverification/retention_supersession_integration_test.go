package documentverificationpg

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in test exercises V80's ownership trigger against the authorized
// synthetic F5 database. The fixture is entirely rollback-scoped.
func TestPostgresOwnershipSupersessionPreservesEarlierRetention(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing unexpected F5 integration target")
	}
	if setupRole == "espacigo_runtime" {
		t.Fatal("refusing to seed ownership fixtures through the runtime role")
	}
	var triggerExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='espacio' AND t.tgname='document_verification_space_owner_changed_trg' AND NOT t.tgisinternal)`).Scan(&triggerExists); err != nil || !triggerExists {
		t.Fatal("required V80 ownership trigger is unavailable")
	}

	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback ownership-retention fixture: %v", err)
		}
	})

	oldOwner, newOwner, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	caseID, fileID := uuid.NewString(), uuid.NewString()
	protectedDeadline := time.Now().UTC().Truncate(time.Microsecond).Add(12 * time.Hour)
	hash := [32]byte{9}
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo'),($3,$4,$4,'synthetic','activo')`, oldOwner, oldOwner+"@example.invalid", newOwner, newOwner+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','F5 retention fixture',repeat('Synthetic retention description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseID, oldOwner, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,last_error_code,retention_at,retention_reason_code) VALUES($1,$2,'bloqueado',128,'image/png',1,$3,'malware_detected',$4,'malicious_file')`, fileID, caseID, hash[:], protectedDeadline); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,available_at) VALUES($1,'scan','completado',now()),($1,'retention_delete','pendiente',$2)`, fileID, protectedDeadline); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, newOwner); err != nil {
		t.Fatalf("synthetic ownership transfer failed: %v", err)
	}

	var gotDeadline time.Time
	var gotReason string
	if err := outer.QueryRow(ctx, `SELECT retention_at,retention_reason_code FROM public.document_verification_file WHERE id=$1`, fileID).Scan(&gotDeadline, &gotReason); err != nil {
		t.Fatal(err)
	}
	if !gotDeadline.Equal(protectedDeadline) || gotReason != "malicious_file" {
		t.Fatalf("ownership change extended or replaced the stricter malware retention deadline: deadline=%v reason=%q; want deadline=%v reason=%q", gotDeadline, gotReason, protectedDeadline, "malicious_file")
	}
	var scheduled time.Time
	var state string
	if err := outer.QueryRow(ctx, `SELECT state,available_at FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, fileID).Scan(&state, &scheduled); err != nil {
		t.Fatal(err)
	}
	if state != "pendiente" || !scheduled.Equal(protectedDeadline) {
		t.Fatalf("retention job no longer follows the stricter deadline: state=%q available_at=%v want=%v", state, scheduled, protectedDeadline)
	}
}
