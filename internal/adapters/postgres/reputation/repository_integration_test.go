package reputation

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	localnoticepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/localnotice"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	localnotice "github.com/HernanEspinozaDev/espaciGo/internal/localnotice"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	domain "github.com/HernanEspinozaDev/espaciGo/internal/reputation"
	disposable "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSyntheticReviewReciprocityModerationRetentionAndRuntimePermissions(t *testing.T) {
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, e := disposable.Connect(ctx, baseURL)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(context.Background())
	name := fmt.Sprintf("local_comm_%d", time.Now().UnixNano())
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+disposable.Identifier{name}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(baseURL)
	if e != nil {
		t.Fatal(e)
	}
	u.Path = "/" + name
	dbURL := u.String()
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 20*time.Second)
		defer cc()
		if conn, x := disposable.Connect(c, dbURL); x == nil {
			_, _ = conn.Exec(c, `DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN DROP OWNED BY espacigo_runtime; END IF; END $$`)
			_ = conn.Close(c)
		}
		if conn, x := disposable.Connect(c, baseURL); x == nil {
			_, _ = conn.Exec(c, "DROP DATABASE "+disposable.Identifier{name}.Sanitize()+" WITH (FORCE)")
			_, _ = conn.Exec(c, "DROP ROLE IF EXISTS espacigo_runtime")
			_ = conn.Close(c)
		}
	})
	if _, e = migrator.Run(ctx, dbURL, "../../../../db/migrations"); e != nil {
		t.Fatalf("migration through V36: %v", e)
	}
	if _, e = admin.Exec(ctx, `CREATE ROLE espacigo_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD 'local-comm-test'`); e != nil {
		t.Fatal(e)
	}
	bootstrap, e := disposable.Connect(ctx, dbURL)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = bootstrap.Exec(ctx, "GRANT CONNECT ON DATABASE "+disposable.Identifier{name}.Sanitize()+" TO espacigo_runtime"); e == nil {
		e = dbbootstrap.GrantRuntimePermissions(ctx, bootstrap)
	}
	_ = bootstrap.Close(ctx)
	if e != nil {
		t.Fatal(e)
	}
	runtimeURL, e := url.Parse(dbURL)
	if e != nil {
		t.Fatal(e)
	}
	runtimeURL.User = url.UserPassword("espacigo_runtime", "local-comm-test")
	pool, e := pgxpool.New(ctx, runtimeURL.String())
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	seedLocalReputationReservation(t, ctx, dbURL)
	var runtime string
	if e = pool.QueryRow(ctx, "SELECT current_user").Scan(&runtime); e != nil || runtime != "espacigo_runtime" {
		t.Fatalf("runtime user=%q err=%v", runtime, e)
	}
	r := New(pool)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	svc, e := domain.New(r, credentials.Generator{}, func() time.Time { return now })
	if e != nil {
		t.Fatal(e)
	}
	const host = "11000000-0000-4000-8000-000000000001"
	const renter = "11000000-0000-4000-8000-000000000002"
	const outsider = "11000000-0000-4000-8000-000000000003"
	const adminID = "11000000-0000-4000-8000-000000000004"
	const reservation = "22000000-0000-4000-8000-000000000001"
	const space = "33000000-0000-4000-8000-000000000001"
	first, e := svc.Create(ctx, renter, reservation, "review-renter-key-01", domain.ReviewInput{Rating: 5, Comment: "Espacio agradable. contacto test@example.invalid id 44000000-0000-4000-8000-000000000001 teléfono +56 9 1234 5678 RUT 12.345.678-9"})
	if e != nil {
		t.Fatal(e)
	}
	retry, e := svc.Create(ctx, renter, reservation, "review-renter-key-02", domain.ReviewInput{Rating: 5, Comment: "Espacio agradable. contacto test@example.invalid id 44000000-0000-4000-8000-000000000001 teléfono +56 9 1234 5678 RUT 12.345.678-9"})
	if e != nil || !retry.Reused || retry.ID != first.ID {
		t.Fatalf("identical retry=%+v err=%v", retry, e)
	}
	if _, e = svc.Create(ctx, outsider, reservation, "review-outsider-key", domain.ReviewInput{Rating: 1}); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("outsider review err=%v", e)
	}
	if _, e = svc.Create(ctx, host, reservation, "review-host-key-01", domain.ReviewInput{Rating: 4}); e != nil {
		t.Fatalf("host reciprocal review: %v", e)
	}
	spaceReviews, e := svc.ListSpace(ctx, space)
	if e != nil || spaceReviews.Count != 1 || spaceReviews.Average != 5 || len(spaceReviews.Items) != 1 {
		t.Fatalf("space reviews=%+v err=%v", spaceReviews, e)
	}
	if spaceReviews.Items[0].Comment == "" || containsReviewPrivateData(spaceReviews.Items[0].Comment) {
		t.Fatalf("public review was not minimized: %q", spaceReviews.Items[0].Comment)
	}
	private, e := svc.ListReservation(ctx, renter, reservation)
	if e != nil || len(private) != 2 {
		t.Fatalf("participant reviews=%+v err=%v", private, e)
	}
	renterScore, e := svc.MyReputation(ctx, renter)
	if e != nil || renterScore.Count != 1 || renterScore.Average != 4 {
		t.Fatalf("authenticated renter reputation=%+v err=%v", renterScore, e)
	}
	report, e := svc.ReportReview(ctx, host, reservation, first.ID, "review-report-key-01", "datos_personales")
	if e != nil {
		t.Fatalf("report review: %v", e)
	}
	reportRetry, e := svc.ReportReview(ctx, host, reservation, first.ID, "review-report-key-02", "datos_personales")
	if e != nil || !reportRetry.Reused || reportRetry.ID != report.ID {
		t.Fatalf("report retry=%+v err=%v", reportRetry, e)
	}
	var noticeCount int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.aviso_local WHERE tipo_evento='resena_reportada' AND agregado_id=$1 AND destinatario_id=$2`, report.ID, adminID).Scan(&noticeCount); e != nil || noticeCount != 1 {
		t.Fatalf("report notice count=%d err=%v", noticeCount, e)
	}
	if _, e = svc.ReportReview(ctx, outsider, reservation, first.ID, "review-report-outsider", "spam"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("outsider report err=%v", e)
	}
	spaceReviews, e = svc.ListSpace(ctx, space)
	if e != nil || spaceReviews.Count != 1 || spaceReviews.Average != 5 || spaceReviews.Items[0].TargetType != "espacio" || spaceReviews.Items[0].State != "" {
		t.Fatalf("report changed public rating: %+v err=%v", spaceReviews, e)
	}
	queue, e := svc.Reports(ctx)
	if e != nil || len(queue) != 1 || queue[0].Comment == "" {
		t.Fatalf("admin report queue=%+v err=%v", queue, e)
	}
	moderated, e := svc.Moderate(ctx, adminID, report.ID, "ocultar", "contenido_inadecuado", "moderation-key-0001", "local-comm-test")
	if e != nil || moderated.State != "oculta" {
		t.Fatalf("moderation=%+v err=%v", moderated, e)
	}
	spaceReviews, e = svc.ListSpace(ctx, space)
	if e != nil || spaceReviews.Count != 0 || len(spaceReviews.Items) != 0 {
		t.Fatalf("hidden review remains public/averaged: %+v err=%v", spaceReviews, e)
	}
	const secondReservation = "22000000-0000-4000-8000-000000000002"
	secondReview, e := svc.Create(ctx, renter, secondReservation, "review-renter-key-02", domain.ReviewInput{Rating: 3, Comment: "Experiencia sintética"})
	if e != nil {
		t.Fatal(e)
	}
	secondReport, e := svc.ReportReview(ctx, host, secondReservation, secondReview.ID, "review-report-key-02", "spam")
	if e != nil {
		t.Fatal(e)
	}
	dismissed, e := svc.Moderate(ctx, adminID, secondReport.ID, "desestimar", "sin_infraccion", "moderation-key-0002", "local-comm-dismiss")
	if e != nil || dismissed.State != "desestimada" {
		t.Fatalf("dismiss moderation=%+v err=%v", dismissed, e)
	}
	spaceReviews, e = svc.ListSpace(ctx, space)
	if e != nil || spaceReviews.Count != 1 || spaceReviews.Average != 3 {
		t.Fatalf("dismissal should restore the review to average: %+v err=%v", spaceReviews, e)
	}
	var audit int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.evento_auditoria_local WHERE accion='reputation.review.moderate' AND recurso_id=$1`, report.ID).Scan(&audit); e != nil || audit != 1 {
		t.Fatalf("moderation audit=%d err=%v", audit, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='en_disputa' WHERE id=$1`, reservation); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Create(ctx, renter, reservation, "review-new-in-dispute", domain.ReviewInput{Rating: 1}); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("new review during dispute err=%v", e)
	}
	testCommunicationNoticeTriggers(t, ctx, pool, reservation, secondReservation, host, renter)
	var immutable bool
	if e = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.resena_ensayo_local','DELETE') OR has_table_privilege(current_user,'public.reporte_resena_historial_local','UPDATE')`).Scan(&immutable); e != nil || immutable {
		t.Fatalf("runtime can rewrite/delete reputation history: %v err=%v", immutable, e)
	}
	counts, e := r.PurgeExpired(ctx, now.AddDate(0, 24, 0), 10)
	if e != nil || counts.ReviewsPurged != 3 || counts.ReportsPurged != 2 {
		var pgErr *pgconn.PgError
		if errors.As(e, &pgErr) {
			t.Logf("postgres error: schema=%s table=%s column=%s constraint=%s detail=%s where=%s internal_query=%s", pgErr.SchemaName, pgErr.TableName, pgErr.ColumnName, pgErr.ConstraintName, pgErr.Detail, pgErr.Where, pgErr.InternalQuery)
		}
		var definer bool
		var owner string
		_ = pool.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner)::text FROM pg_proc p WHERE p.oid='public.purge_expired_local_communication(timestamptz,integer)'::regprocedure`).Scan(&definer, &owner)
		var del bool
		_ = pool.QueryRow(ctx, `SELECT has_table_privilege($1,'public.reporte_resena_ensayo_local','DELETE')`, owner).Scan(&del)
		t.Fatalf("due retention counts=%+v err=%v security_definer=%t owner=%s owner_delete=%v", counts, e, definer, owner, del)
	}
	testDurableNoticeRecovery(t, ctx, pool, now, reservation, adminID, host, outsider)
}

func testCommunicationNoticeTriggers(t *testing.T, ctx context.Context, pool *pgxpool.Pool, reservation, secondReservation, host, renter string) {
	t.Helper()
	location := `'{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}'::jsonb`
	_, e := pool.Exec(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en) VALUES('77000000-0000-4000-8000-000000000001',$1,'checkin',$2,now(),'America/Santiago',`+location+`,'registrada','comm-checkin-key',repeat('c',32)::bytea,now())`, secondReservation, renter)
	if e != nil {
		t.Fatal(e)
	}
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.aviso_local WHERE tipo_evento='checkin_registrado' AND agregado_id=$1 AND destinatario_id=$2`, secondReservation, host).Scan(&count); e != nil || count != 1 {
		t.Fatalf("check-in notice count=%d err=%v", count, e)
	}
	_, e = pool.Exec(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en) VALUES('77000000-0000-4000-8000-000000000002',$1,'checkout',$2,now(),'America/Santiago',`+location+`,'registrada','comm-checkout-key',repeat('d',32)::bytea,now())`, reservation, renter)
	if e != nil {
		t.Fatal(e)
	}
	_, e = pool.Exec(ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en) VALUES('77000000-0000-4000-8000-000000000003','77000000-0000-4000-8000-000000000002','synthetic-png-v1','image/png',repeat('a',64),128,now())`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = pool.Exec(ctx, `INSERT INTO public.reclamo_dano_ensayo_local(id,reserva_id,anfitrion_id,arrendatario_id,checkout_operacion_id,checkout_evidencia_id,descripcion,estado,clave_idempotencia,huella_solicitud,abierto_en,plazo_reclamo_hasta) VALUES('88000000-0000-4000-8000-000000000001',$1,$2,$3,'77000000-0000-4000-8000-000000000002','77000000-0000-4000-8000-000000000003','Ensayo sintético','abierto','comm-claim-key',repeat('e',32)::bytea,now(),now()+interval '24 hours')`, reservation, host, renter)
	if e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.aviso_local WHERE tipo_evento='reclamo_abierto' AND agregado_id=$1 AND destinatario_id=$2`, reservation, renter).Scan(&count); e != nil || count != 1 {
		t.Fatalf("claim notice count=%d err=%v", count, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_arrendatario',actualizada_en=now() WHERE id=$1`, secondReservation); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.aviso_local WHERE tipo_evento='reserva_cancelada' AND agregado_id=$1`, secondReservation).Scan(&count); e != nil || count != 2 {
		t.Fatalf("participant cancellation notices=%d err=%v", count, e)
	}
}

type localNoticeTestSender struct {
	failures int
	sent     int
}

func (s *localNoticeTestSender) SendLocalBookingNotice(context.Context, string, string, string) error {
	s.sent++
	if s.failures > 0 {
		s.failures--
		return errors.New("synthetic Mailpit failure")
	}
	return nil
}

func testDurableNoticeRecovery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, now time.Time, reservation, adminID, host, inactiveRecipient string) {
	const noticeID = "66000000-0000-4000-8000-000000000001"
	// Resolve the report-triggered intent without contacting SMTP; this focused test exercises one isolated event.
	if _, e := pool.Exec(ctx, `UPDATE public.aviso_local SET estado='entregada',entregada_en=$1,retirar_en=$1::timestamptz+interval '30 days' WHERE estado='pendiente'`, now); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `UPDATE public.aviso_local_ciclo SET estado='entregada',finalizada_en=$1,codigo_resultado='test_fixture_resolved' WHERE estado='pendiente'`, now); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,estado,ciclo,intentos_total,intentos_ciclo,creada_en,proximo_intento_en) VALUES($1,'checkin_registrado','reserva',$2,$3,'comm-test-recovery-notice','pendiente',1,0,0,$4,$4)`, noticeID, reservation, adminID, now); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en) VALUES($1,1,'pendiente',$2)`, noticeID, now); e != nil {
		t.Fatal(e)
	}
	sender := &localNoticeTestSender{failures: 8}
	clock := now
	svc, e := localnotice.New(localnoticepg.New(pool), sender, func() time.Time { return clock })
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 8; i++ {
		claimed, err := svc.DispatchOne(ctx)
		if err != nil || !claimed {
			t.Fatalf("dispatch failure attempt %d claimed=%t err=%v", i+1, claimed, err)
		}
		if i < 7 {
			clock = clock.Add(time.Duration(1<<i) * time.Second)
		}
	}
	var state string
	var cycle, cycleAttempts, totalAttempts int
	if e = pool.QueryRow(ctx, `SELECT estado,ciclo,intentos_ciclo,intentos_total FROM public.aviso_local WHERE id=$1`, noticeID).Scan(&state, &cycle, &cycleAttempts, &totalAttempts); e != nil || state != "fallo_terminal" || cycle != 1 || cycleAttempts != 8 || totalAttempts != 8 {
		t.Fatalf("terminal state=%s cycle=%d cycle_attempts=%d total=%d err=%v", state, cycle, cycleAttempts, totalAttempts, e)
	}
	clock = clock.Add(time.Minute)
	if _, err := svc.Reopen(ctx, noticeID, host, "reintento_operativo", "comm-test-unauthorized", "comm-reopen-denied"); !errors.Is(err, localnotice.ErrInvalid) {
		t.Fatalf("non-admin reopened notice: %v", err)
	}
	restartedSender := &localNoticeTestSender{}
	restartedService, e := localnotice.New(localnoticepg.New(pool), restartedSender, func() time.Time { return clock })
	if e != nil {
		t.Fatal(e)
	}
	type result struct {
		reused bool
		err    error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := restartedService.Reopen(ctx, noticeID, adminID, "reintento_operativo", "comm-test-reopen-key-01", "comm-reopen-correlation")
			results <- result{v.Reused, err}
		}()
	}
	wg.Wait()
	close(results)
	reused := 0
	for v := range results {
		if v.err != nil {
			t.Fatal(v.err)
		}
		if v.reused {
			reused++
		}
	}
	if reused != 1 {
		t.Fatalf("concurrent reopen reused=%d, want one persisted reuse", reused)
	}
	claimed, err := restartedService.DispatchOne(ctx)
	if err != nil || !claimed {
		t.Fatalf("dispatch after recovery claimed=%t err=%v", claimed, err)
	}
	if sender.sent != 8 || restartedSender.sent != 1 {
		t.Fatalf("deliveries before restart=%d after=%d, want eight failures then one success", sender.sent, restartedSender.sent)
	}
	if e = pool.QueryRow(ctx, `SELECT estado,ciclo,intentos_ciclo,intentos_total FROM public.aviso_local WHERE id=$1`, noticeID).Scan(&state, &cycle, &cycleAttempts, &totalAttempts); e != nil || state != "entregada" || cycle != 2 || cycleAttempts != 1 || totalAttempts != 9 {
		t.Fatalf("recovered state=%s cycle=%d cycle_attempts=%d total=%d err=%v", state, cycle, cycleAttempts, totalAttempts, e)
	}
	const inactiveNotice = "66000000-0000-4000-8000-000000000002"
	if _, e = pool.Exec(ctx, `INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,estado,ciclo,intentos_total,intentos_ciclo,creada_en,proximo_intento_en,fallo_terminal_en,codigo_error,retirar_en) VALUES($1,'checkin_registrado','reserva',$2,$3,'comm-test-inactive-notice','fallo_terminal',1,8,8,$4,$4,$4,'mailpit_delivery_failed',$4::timestamptz+interval '30 days')`, inactiveNotice, reservation, inactiveRecipient, clock); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en,finalizada_en,intentos,codigo_resultado) VALUES($1,1,'fallo_terminal',$2,$2,8,'mailpit_delivery_failed')`, inactiveNotice, clock); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `UPDATE public.usuario SET estado='desidentificado' WHERE id=$1`, inactiveRecipient); e != nil {
		t.Fatal(e)
	}
	if _, e = restartedService.Reopen(ctx, inactiveNotice, adminID, "reintento_operativo", "comm-test-inactive-reopen", "comm-reopen-inactive"); !errors.Is(e, localnotice.ErrInvalid) {
		t.Fatalf("reopened notice for inactive recipient: %v", e)
	}
}

func seedLocalReputationReservation(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	pool, e := pgxpool.New(ctx, databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	const host = "11000000-0000-4000-8000-000000000001"
	const renter = "11000000-0000-4000-8000-000000000002"
	const outsider = "11000000-0000-4000-8000-000000000003"
	const adminID = "11000000-0000-4000-8000-000000000004"
	const space = "33000000-0000-4000-8000-000000000001"
	const reservation = "22000000-0000-4000-8000-000000000001"
	const quote = "44000000-0000-4000-8000-000000000002"
	const occupancy = "55000000-0000-4000-8000-000000000001"
	for i, id := range []string{host, renter, outsider, adminID} {
		if _, e = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-hash','activo')`, id, fmt.Sprintf("comm-%d@example.invalid", i)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','Comm test space',repeat('Synthetic local review fixture description. ',4),30,4,'Reglas sintéticas','hora',8000,'Dirección local sintética','America/Santiago','activa')`, space, host); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, space); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, space); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	start := now.Add(-48 * time.Hour)
	end := now.Add(-24 * time.Hour)
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en,condiciones_snapshot,categoria_codigo,perfil_version,perfil_valores_snapshot) VALUES($1,$2,$3,$4,1,'hora',8000,'CLP',24,192000,$5,$6,'America/Santiago',$7,$7::timestamptz+interval '15 minutes','Reglas snapshot','sala_multiproposito',1,'{}'::jsonb)`, quote, space, host, renter, start, end, now); e != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,creada_en) VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'reserva',true,$6)`, occupancy, space, reservation, start, end, now); e != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,pago_vence_en,creada_en,actualizada_en,condiciones_snapshot,politica_cancelacion_version) VALUES($1,$2,$3,$4,$5,'comm-seed-key',repeat('a',32)::bytea,$6,'finalizada',8000,24,192000,'hora','CLP',$7,$8,'America/Santiago',$9,$10,$10,'Reglas snapshot','local_flexible_v1')`, reservation, quote, space, host, renter, occupancy, start, end, now.Add(15*time.Minute), now); e != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	seedSecondLocalReputationReservation(t, ctx, pool, host, renter, space, time.Now().UTC().Truncate(time.Microsecond))
}

func seedSecondLocalReputationReservation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, host, renter, space string, now time.Time) {
	t.Helper()
	const reservation = "22000000-0000-4000-8000-000000000002"
	const quote = "44000000-0000-4000-8000-000000000003"
	const occupancy = "55000000-0000-4000-8000-000000000002"
	start, end := now.Add(-96*time.Hour), now.Add(-72*time.Hour)
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, e = tx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en,condiciones_snapshot,categoria_codigo,perfil_version,perfil_valores_snapshot) VALUES($1,$2,$3,$4,1,'hora',8000,'CLP',24,192000,$5,$6,'America/Santiago',$7,$7::timestamptz+interval '15 minutes','Reglas snapshot','sala_multiproposito',1,'{}'::jsonb)`, quote, space, host, renter, start, end, now); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,creada_en) VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'reserva',true,$6)`, occupancy, space, reservation, start, end, now); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,pago_vence_en,creada_en,actualizada_en,condiciones_snapshot,politica_cancelacion_version) VALUES($1,$2,$3,$4,$5,'comm-seed-key-2',repeat('b',32)::bytea,$6,'finalizada',8000,24,192000,'hora','CLP',$7,$8,'America/Santiago',$9,$10,$10,'Reglas snapshot','local_flexible_v1')`, reservation, quote, space, host, renter, occupancy, start, end, now.Add(15*time.Minute), now); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
}

func containsReviewPrivateData(s string) bool {
	return strings.Contains(s, "test@example.invalid") || strings.Contains(s, "44000000-0000-4000-8000-000000000001") || strings.Contains(s, "1234 5678") || strings.Contains(s, "12.345.678-9")
}
