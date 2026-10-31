package documentverificationpg

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Opt-in regression for the V82 ownership trigger. Fixtures are inserted
// directly (not through V81 helpers) and are always rolled back.
func TestV82OwnershipRetentionPrecedence(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	expectedAddress := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_EXPECTED_SERVER_ADDRESS")
	if net.ParseIP(expectedAddress) == nil || expectedAddress == "" {
		t.Fatal("ESPACIGO_DOCUMENT_VERIFICATION_EXPECTED_SERVER_ADDRESS must be a separately supplied IP address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	cc := cfg.ConnConfig
	if cc.Host != "127.0.0.1" || cc.Port != 15433 || cc.Database != "espacigo_f5_synth_20261029" || len(cc.Fallbacks) != 0 {
		t.Fatal("refusing integration target outside the exact loopback F5 endpoint or with fallback hosts")
	}
	if cc.Config.TLSConfig != nil {
		t.Fatal("F5 integration DSN must explicitly disable TLS")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot create F5 pool")
	}
	t.Cleanup(pool.Close)
	var database string
	var address net.IP
	if err := pool.QueryRow(ctx, `SELECT current_database(),inet_server_addr()`).Scan(&database, &address); err != nil {
		t.Fatal("cannot verify PostgreSQL target")
	}
	if database != "espacigo_f5_synth_20261029" || address == nil || address.String() != expectedAddress {
		t.Fatalf("connected PostgreSQL target mismatch: database=%q server_address=%v", database, address)
	}
	var setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&setupRole); err != nil || setupRole == "espacigo_runtime" {
		t.Fatal("fixture setup requires the authorized non-runtime setup role")
	}
	var bound bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid JOIN pg_catalog.pg_namespace pn ON pn.oid=p.pronamespace
		WHERE n.nspname='public' AND c.relname='espacio' AND NOT t.tgisinternal
		AND pn.nspname='public' AND p.proname='document_verification_space_owner_changed'
	)`).Scan(&bound); err != nil || !bound {
		t.Fatal("ownership trigger is not bound to document_verification_space_owner_changed()")
	}

	type scenario struct {
		name, fileState, lastError, reason, wantReason string
		scan, retention, deadline, want                string
		inspected                                      string
		completed, alsoCompleted                       bool
	}
	cases := []scenario{
		{name: "no-deadline", fileState: "pendiente", scan: "pendiente", retention: "absent", deadline: "none", want: "replacement", wantReason: "rejected_or_replaced"},
		{name: "earlier-null-reason", fileState: "pendiente", scan: "procesando", retention: "pendiente", deadline: "earlier", want: "existing", wantReason: ""},
		{name: "inspection-error-seven-day-retention", fileState: "error", lastError: "scanner_unavailable", reason: "inspection_error", wantReason: "inspection_error", scan: "reintentar", retention: "procesando", deadline: "inspection-error", want: "existing"},
		{name: "earlier-malware-deadline-preserves-deadline-and-malware-reason", fileState: "bloqueado", lastError: "malware_detected", reason: "abandoned_draft", wantReason: "malicious_file", inspected: "recent", scan: "completado", retention: "reintentar", deadline: "malware-earlier", want: "existing"},
		{name: "malware-error-caps-later-deadline", fileState: "bloqueado", lastError: "malware_detected", inspected: "recent", wantReason: "malicious_file", scan: "completado", retention: "pendiente", deadline: "malware-later", want: "malware-cap"},
		{name: "malware-reason-caps-later-deadline", fileState: "bloqueado", reason: "malicious_file", wantReason: "malicious_file", inspected: "recent", scan: "completado", retention: "pendiente", deadline: "later", want: "malware-cap"},
		{name: "malware-without-inspected-at-is-immediate", fileState: "bloqueado", lastError: "malware_detected", wantReason: "malicious_file", scan: "completado", retention: "absent", deadline: "none", want: "malware-now"},
		{name: "malware-future-inspected-at-is-immediate", fileState: "bloqueado", lastError: "malware_detected", inspected: "future", wantReason: "malicious_file", scan: "completado", retention: "absent", deadline: "later", want: "malware-now"},
		{name: "later-replacement-deadline", fileState: "pendiente", reason: "abandoned_draft", wantReason: "rejected_or_replaced", scan: "completado", retention: "absent", deadline: "later", want: "replacement"},
		{name: "completed-retention-history", fileState: "pendiente", wantReason: "rejected_or_replaced", retention: "completado", deadline: "none", want: "replacement", completed: true},
		{name: "active-and-completed-retention-history", fileState: "pendiente", wantReason: "rejected_or_replaced", scan: "procesando", retention: "pendiente", deadline: "none", want: "replacement", completed: true, alsoCompleted: true},
	}
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	owner, newOwner := uuid.NewString(), uuid.NewString()
	spaceIDs, caseIDs, fileIDs := make([]string, len(cases)), make([]string, len(cases)), make([]string, len(cases))
	initialDeadlines := make([]time.Time, len(cases))
	inspectionTimes := make([]time.Time, len(cases))
	for i := range cases {
		spaceIDs[i], caseIDs[i], fileIDs[i] = uuid.NewString(), uuid.NewString(), uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		switch cases[i].inspected {
		case "recent":
			inspectionTimes[i] = now.Add(-12 * time.Hour)
		case "future":
			inspectionTimes[i] = now.Add(12 * time.Hour)
		}
		switch cases[i].deadline {
		case "earlier":
			initialDeadlines[i] = now.Add(24 * time.Hour)
		case "inspection-error":
			initialDeadlines[i] = now.Add(7 * 24 * time.Hour)
		case "malware-earlier":
			initialDeadlines[i] = inspectionTimes[i].Add(6 * time.Hour)
		case "malware-later":
			initialDeadlines[i] = inspectionTimes[i].Add(48 * time.Hour)
		case "later":
			initialDeadlines[i] = now.Add(90 * 24 * time.Hour)
		}
	}
	rolledBack := false
	t.Cleanup(func() {
		if !rolledBack {
			if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
				t.Errorf("rollback fixture transaction: %v", err)
			}
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT
			(SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+
			(SELECT count(*) FROM public.espacio WHERE id=ANY($3::uuid[]))+
			(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=ANY($3::uuid[]))+
			(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=ANY($3::uuid[]))+
			(SELECT count(*) FROM public.document_verification_case WHERE id=ANY($4::uuid[]))+
			(SELECT count(*) FROM public.document_verification_file WHERE id=ANY($5::uuid[]))+
			(SELECT count(*) FROM public.document_verification_job WHERE file_id=ANY($5::uuid[]))+
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=ANY($4::uuid[]))`, owner, newOwner, spaceIDs, caseIDs, fileIDs).Scan(&remaining); err != nil {
			t.Errorf("verify rollback cleanup: %v", err)
		} else if remaining != 0 {
			t.Errorf("rollback left %d fixture rows", remaining)
		}
	})

	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo'),($3,$4,$4,'synthetic','activo')`, owner, owner+"@example.invalid", newOwner, newOwner+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	for i, s := range cases {
		if _, err := outer.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','V82 retention fixture',repeat('Synthetic. ',12),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceIDs[i], owner); err != nil {
			t.Fatalf("insert space %s: %v", s.name, err)
		}
		if _, err := outer.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceIDs[i]); err != nil {
			t.Fatalf("insert rate %s: %v", s.name, err)
		}
		if _, err := outer.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceIDs[i]); err != nil {
			t.Fatalf("insert characteristics %s: %v", s.name, err)
		}
		if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseIDs[i], owner, spaceIDs[i]); err != nil {
			t.Fatalf("insert case %s: %v", s.name, err)
		}
		var reason, inspected any
		if s.reason != "" {
			reason = s.reason
		}
		if !inspectionTimes[i].IsZero() {
			inspected = inspectionTimes[i]
		}
		var deadline any
		switch s.deadline {
		case "earlier", "inspection-error", "malware-earlier", "malware-later":
			deadline = initialDeadlines[i]
		case "later":
			deadline = initialDeadlines[i]
		}
		if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,inspected_at,last_error_code,retention_at,retention_reason_code) VALUES($1,$2,$3,128,'image/png',1,$4,$5,NULLIF($6,''),$7,NULLIF($8,''))`, fileIDs[i], caseIDs[i], s.fileState, []byte(fmt.Sprintf("%032d", i)), inspected, s.lastError, deadline, reason); err != nil {
			t.Fatalf("insert file %s: %v", s.name, err)
		}
		if s.scan != "" {
			lease := any(nil)
			if s.scan == "procesando" {
				lease = time.Now().UTC().Add(time.Hour)
			}
			if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,lease_until) VALUES($1,'scan',$2,$3)`, fileIDs[i], s.scan, lease); err != nil {
				t.Fatalf("insert scan job %s: %v", s.name, err)
			}
		}
		if s.retention != "absent" {
			lease := any(nil)
			attempts := 0
			if s.retention == "procesando" {
				lease = time.Now().UTC().Add(time.Hour)
			}
			if s.retention == "procesando" || s.retention == "reintentar" {
				attempts = 3
			}
			if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,lease_until,available_at) VALUES($1,'retention_delete',$2,$3,$4,clock_timestamp())`, fileIDs[i], s.retention, attempts, lease); err != nil {
				t.Fatalf("insert retention job %s: %v", s.name, err)
			}
		}
		if s.alsoCompleted {
			if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state) VALUES($1,'retention_delete','completado')`, fileIDs[i]); err != nil {
				t.Fatalf("insert completed retention history %s: %v", s.name, err)
			}
		}
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatalf("set runtime role: %v", err)
	}
	var role string
	var superuser, bypass bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&role, &superuser, &bypass); err != nil {
		t.Fatal(err)
	}
	if role != "espacigo_runtime" || superuser || bypass {
		t.Fatalf("unexpected runtime role flags: role=%q superuser=%t bypassRLS=%t", role, superuser, bypass)
	}
	for _, spaceID := range spaceIDs {
		if _, err := outer.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, newOwner); err != nil {
			t.Fatalf("ownership transfer: %v", err)
		}
	}
	for i, s := range cases {
		var actualDeadline *time.Time
		var actualReason *string
		if err := outer.QueryRow(ctx, `SELECT retention_at,retention_reason_code FROM public.document_verification_file WHERE id=$1`, fileIDs[i]).Scan(&actualDeadline, &actualReason); err != nil {
			t.Fatal(err)
		}
		var superseded time.Time
		if err := outer.QueryRow(ctx, `SELECT superseded_at FROM public.document_verification_case WHERE id=$1`, caseIDs[i]).Scan(&superseded); err != nil {
			t.Fatal(err)
		}
		var wantDeadline time.Time
		switch s.want {
		case "existing":
			wantDeadline = initialDeadlines[i]
		case "replacement":
			wantDeadline = superseded.Add(30 * 24 * time.Hour)
		case "malware-cap":
			wantDeadline = inspectionTimes[i].Add(24 * time.Hour)
		case "malware-now":
			wantDeadline = superseded
		default:
			t.Fatalf("unknown expected deadline policy %q", s.want)
		}
		if actualDeadline == nil || !actualDeadline.Equal(wantDeadline) {
			t.Errorf("%s deadline=%v want=%v policy=%s", s.name, actualDeadline, wantDeadline, s.want)
		}
		gotReason := "<nil>"
		if actualReason != nil {
			gotReason = *actualReason
		}
		if s.wantReason == "" {
			if actualReason != nil {
				t.Errorf("%s expected NULL retention reason, got %q", s.name, gotReason)
			}
		} else if actualReason == nil || *actualReason != s.wantReason {
			t.Errorf("%s retention reason=%q want=%q", s.name, gotReason, s.wantReason)
		}
		var scanJobs, completedScans, activeScans, activeRetention, pendingRetention, completedRetention, retentionJobs, retentionAttempts int
		var retentionLeasesClear bool
		var scheduled *time.Time
		if err := outer.QueryRow(ctx, `SELECT count(*) FILTER(WHERE kind='scan'), count(*) FILTER(WHERE kind='scan' AND state='completado'), count(*) FILTER(WHERE kind='scan' AND state IN('pendiente','procesando','reintentar')), count(*) FILTER(WHERE kind='retention_delete' AND state IN('pendiente','procesando','reintentar')), count(*) FILTER(WHERE kind='retention_delete' AND state='pendiente'), count(*) FILTER(WHERE kind='retention_delete' AND state='completado'), count(*) FILTER(WHERE kind='retention_delete'), COALESCE(sum(attempts) FILTER(WHERE kind='retention_delete'),0), COALESCE(bool_and(lease_until IS NULL) FILTER(WHERE kind='retention_delete'),true), max(available_at) FILTER(WHERE kind='retention_delete' AND state='pendiente') FROM public.document_verification_job WHERE file_id=$1`, fileIDs[i]).Scan(&scanJobs, &completedScans, &activeScans, &activeRetention, &pendingRetention, &completedRetention, &retentionJobs, &retentionAttempts, &retentionLeasesClear, &scheduled); err != nil {
			t.Fatal(err)
		}
		if s.scan == "" {
			if scanJobs != 0 {
				t.Errorf("%s scan jobs=%d want none", s.name, scanJobs)
			}
		} else if scanJobs != 1 || completedScans != 1 || activeScans != 0 {
			t.Errorf("%s scan state after transfer: jobs=%d completed=%d active=%d; want one completed and no active", s.name, scanJobs, completedScans, activeScans)
		}
		if s.completed {
			wantJobs := 1
			if s.alsoCompleted {
				wantJobs = 2
			}
			if activeRetention != 0 || completedRetention != wantJobs || retentionJobs != wantJobs {
				t.Errorf("completed retention history was rescheduled: active=%d completed=%d jobs=%d want=%d", activeRetention, completedRetention, retentionJobs, wantJobs)
			}
		} else if activeRetention != 1 || pendingRetention != 1 || retentionJobs != 1 || retentionAttempts != 0 || !retentionLeasesClear || scheduled == nil || actualDeadline == nil || !scheduled.Equal(*actualDeadline) {
			t.Errorf("%s retention scheduling active=%d pending=%d jobs=%d attempts=%d leases_clear=%t scheduled=%v deadline=%v", s.name, activeRetention, pendingRetention, retentionJobs, retentionAttempts, retentionLeasesClear, scheduled, actualDeadline)
		}
	}
	if err := outer.Rollback(ctx); err != nil {
		t.Fatalf("rollback fixtures: %v", err)
	}
	rolledBack = true
}
