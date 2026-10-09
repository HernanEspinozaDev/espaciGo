package reputation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	damageclaimpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/damageclaim"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	localnoticepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/localnotice"
	"github.com/HernanEspinozaDev/espaciGo/internal/damageclaim"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	localnotice "github.com/HernanEspinozaDev/espaciGo/internal/localnotice"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
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
		t.Fatalf("migration through V37: %v", e)
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
	now := time.Now().UTC().Truncate(time.Microsecond)
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
	markerFixture := seedSuppressionRaceReservation(t, ctx, pool, 30, now)
	if _, e = pool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=NULL WHERE id=$1`, markerFixture.reservation); e != nil {
		t.Fatal(e)
	}
	markerReview, e := svc.Create(ctx, markerFixture.renter, markerFixture.reservation, "marker-review-key-0001", domain.ReviewInput{Rating: 4, Comment: "Synthetic marker test"})
	if e != nil {
		t.Fatalf("create marker test review: %v", e)
	}
	if _, e = pool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='en_disputa' WHERE id=$1`, reservation); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Create(ctx, renter, reservation, "review-new-in-dispute", domain.ReviewInput{Rating: 1}); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("new review during dispute err=%v", e)
	}
	if _, e = pool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='finalizada' WHERE id=$1`, reservation); e != nil {
		t.Fatal(e)
	}
	testReceivedReviewSuppression(t, ctx, pool, adminID, now, reservation, renter, host)
	testCommunicationNoticeTriggers(t, ctx, pool, reservation, secondReservation, host, renter)
	testMailpitLocalCommunicationDelivery(t, ctx, pool, now, adminID)
	testNoticeDispatchSuppressionRace(t, ctx, pool, outsider, adminID, reservation, now)
	var immutable bool
	if e = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.resena_ensayo_local','DELETE') OR has_table_privilege(current_user,'public.reporte_resena_historial_local','UPDATE')`).Scan(&immutable); e != nil || immutable {
		t.Fatalf("runtime can rewrite/delete reputation history: %v err=%v", immutable, e)
	}
	counts, e := r.PurgeExpired(ctx, now.AddDate(0, 25, 0), 10)
	if e != nil || counts.ReviewsPurged != 4 || counts.ReportsPurged != 2 {
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
	futureSvc, e := domain.New(r, credentials.Generator{}, func() time.Time { return now.AddDate(0, 25, 0) })
	if e != nil {
		t.Fatal(e)
	}
	if _, e = futureSvc.Create(ctx, markerFixture.renter, markerFixture.reservation, "marker-review-retry-key", domain.ReviewInput{Rating: 4, Comment: "Synthetic marker test"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("purged review uniqueness was not retained: err=%v review=%s", e, markerReview.ID)
	}
	var retainedMarker bool
	markerDigest := reviewAuthorshipMarker(markerFixture.reservation, markerFixture.renter)
	if e = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.resena_autoria_marca_local WHERE huella_autoria=$1 AND retirar_en IS NULL)`, markerDigest[:]).Scan(&retainedMarker); e != nil || !retainedMarker {
		t.Fatalf("eligible reservation author marker was purged: retained=%t err=%v", retainedMarker, e)
	}
	testDurableNoticeRecovery(t, ctx, pool, now, reservation, adminID, host, outsider)
	testReviewAndReportSuppressionRaces(t, ctx, pool, svc, adminID, now)
	testReviewAndClaimLockOrder(t, ctx, pool, svc, now)
}

func TestV37BackfillsV36ReviewAuthorshipMarker(t *testing.T) {
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
	name := fmt.Sprintf("local_comm_v37_%d", time.Now().UnixNano())
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
		if conn, err := disposable.Connect(c, baseURL); err == nil {
			_, _ = conn.Exec(c, "DROP DATABASE "+disposable.Identifier{name}.Sanitize()+" WITH (FORCE)")
			_ = conn.Close(c)
		}
	})
	fullDir, e := filepath.Abs("../../../../db/migrations")
	if e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(fullDir)
	if e != nil {
		t.Fatal(e)
	}
	v36Dir := t.TempDir()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "V") {
			continue
		}
		versionText := strings.SplitN(strings.TrimPrefix(entry.Name(), "V"), "__", 2)[0]
		version, parseErr := strconv.Atoi(versionText)
		if parseErr != nil || version > 36 {
			continue
		}
		if err := os.Symlink(filepath.Join(fullDir, entry.Name()), filepath.Join(v36Dir, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	if _, e = migrator.Run(ctx, dbURL, v36Dir); e != nil {
		t.Fatalf("apply through V36: %v", e)
	}
	seedLocalReputationReservation(t, ctx, dbURL)
	const renter = "11000000-0000-4000-8000-000000000002"
	const reservation = "22000000-0000-4000-8000-000000000001"
	const legacyReview = "77000000-0000-4000-8000-000000000091"
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	pool, e := pgxpool.New(ctx, dbURL)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if _, e = pool.Exec(ctx, `INSERT INTO public.resena_ensayo_local(id,reserva_id,espacio_id,autor_id,destinatario_tipo,destinatario_id,puntuacion,comentario,estado,creada_en,retirar_en,huella_solicitud,clave_idempotencia) VALUES($1,$2,'33000000-0000-4000-8000-000000000001',$3,'espacio',NULL,5,'legacy V36 review','publicada',$4,$4::timestamptz+interval '24 months',repeat('a',32)::bytea,'legacy-v36-review')`, legacyReview, reservation, renter, createdAt); e != nil {
		t.Fatalf("seed V36 review: %v", e)
	}
	if _, e = migrator.Run(ctx, dbURL, fullDir); e != nil {
		t.Fatalf("upgrade V36 data through V37: %v", e)
	}
	marker := reviewAuthorshipMarker(reservation, renter)
	var markedAt time.Time
	var removeAt *time.Time
	if e = pool.QueryRow(ctx, `SELECT creada_en,retirar_en FROM public.resena_autoria_marca_local WHERE huella_autoria=$1`, marker[:]).Scan(&markedAt, &removeAt); e != nil {
		t.Fatalf("V37 did not backfill legacy authorship marker: %v", e)
	}
	if !markedAt.Equal(createdAt) || removeAt != nil {
		t.Fatalf("legacy marker dates=%s/%s", markedAt, removeAt)
	}
	if _, e = pool.Exec(ctx, `DELETE FROM public.resena_ensayo_local WHERE id=$1`, legacyReview); e != nil {
		t.Fatal(e)
	}
	service, e := domain.New(New(pool), credentials.Generator{}, func() time.Time { return createdAt })
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Create(ctx, renter, reservation, "legacy-v36-review-retry", domain.ReviewInput{Rating: 5, Comment: "legacy V36 review"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("V37 backfilled marker did not prevent duplicate: %v", e)
	}
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
	const strandedID = "66000000-0000-4000-8000-000000000003"
	if _, e = pool.Exec(ctx, `INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,estado,ciclo,intentos_total,intentos_ciclo,creada_en,proximo_intento_en,lease_hasta) VALUES($1,'checkin_registrado','reserva',$2,$3,'comm-test-expired-eighth-lease','procesando',1,8,8,$4,$4,$4::timestamptz-interval '1 second')`, strandedID, reservation, adminID, clock); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en,intentos) VALUES($1,1,'pendiente',$2,8)`, strandedID, clock.Add(-time.Minute)); e != nil {
		t.Fatal(e)
	}
	if claimed, err := restartedService.DispatchOne(ctx); err != nil || claimed || restartedSender.sent != 1 {
		t.Fatalf("expired eighth lease should terminalize without another send: claimed=%t sends=%d err=%v", claimed, restartedSender.sent, err)
	}
	var strandedState, strandedCode, cycleState, cycleCode string
	var terminalAt, removeAt *time.Time
	var lease *time.Time
	if e = pool.QueryRow(ctx, `SELECT estado,COALESCE(codigo_error,''),fallo_terminal_en,retirar_en,lease_hasta FROM public.aviso_local WHERE id=$1`, strandedID).Scan(&strandedState, &strandedCode, &terminalAt, &removeAt, &lease); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `SELECT estado,COALESCE(codigo_resultado,'') FROM public.aviso_local_ciclo WHERE aviso_id=$1 AND ciclo=1`, strandedID).Scan(&cycleState, &cycleCode); e != nil {
		t.Fatal(e)
	}
	if strandedState != "fallo_terminal" || strandedCode != "lease_expirado_tras_intento_8" || terminalAt == nil || !terminalAt.Equal(clock) || removeAt == nil || !removeAt.Equal(clock.Add(30*24*time.Hour)) || lease != nil || cycleState != "fallo_terminal" || cycleCode != strandedCode {
		t.Fatalf("stranded eighth attempt not recovered: event=%s/%s/%v/%v lease=%v cycle=%s/%s", strandedState, strandedCode, terminalAt, removeAt, lease, cycleState, cycleCode)
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

func testMailpitLocalCommunicationDelivery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, at time.Time, adminID string) {
	t.Helper()
	smtpAddr, apiURL := os.Getenv("LOCAL_SMTP_ADDR"), os.Getenv("LOCAL_MAILPIT_API")
	var noticeID, recipient string
	e := pool.QueryRow(ctx, `SELECT a.id::text,u.correo_original FROM public.aviso_local a JOIN public.usuario u ON u.id=a.destinatario_id WHERE a.tipo_evento='resena_reportada' AND a.destinatario_id=$1 AND a.estado='pendiente' ORDER BY a.creada_en,a.id LIMIT 1`, adminID).Scan(&noticeID, &recipient)
	if e != nil {
		t.Fatalf("find review-report notice for Mailpit: %v", e)
	}
	if _, e = pool.Exec(ctx, `UPDATE public.aviso_local SET proximo_intento_en='9999-01-01T00:00:00Z' WHERE estado='pendiente' AND id<>$1`, noticeID); e != nil {
		t.Fatal(e)
	}
	if smtpAddr == "" || apiURL == "" {
		if _, e = pool.Exec(ctx, `UPDATE public.aviso_local SET proximo_intento_en='9999-01-01T00:00:00Z' WHERE id=$1`, noticeID); e != nil {
			t.Fatal(e)
		}
		t.Log("real Mailpit delivery omitted; run scripts/test-local-comm-mailpit.sh to include it")
		return
	}
	sender := devauth.Mailer{Address: smtpAddr}
	service, e := localnotice.New(localnoticepg.New(pool), sender, func() time.Time { return at })
	if e != nil {
		t.Fatal(e)
	}
	claimed, e := service.DispatchOne(ctx)
	if e != nil || !claimed {
		t.Fatalf("dispatch review notice to Mailpit: claimed=%t err=%v", claimed, e)
	}
	var state string
	if e = pool.QueryRow(ctx, `SELECT estado FROM public.aviso_local WHERE id=$1`, noticeID).Scan(&state); e != nil || state != "entregada" {
		t.Fatalf("Mailpit notice state=%s err=%v", state, e)
	}
	searchURL := strings.TrimRight(apiURL, "/") + "/api/v1/search?query=" + url.QueryEscape("to:"+recipient)
	response, e := http.Get(searchURL)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Mailpit search status=%s", response.Status)
	}
	var result struct {
		Messages []struct {
			ID      string
			Subject string
			To      []struct{ Address string }
		} `json:"messages"`
	}
	if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
		t.Fatal(e)
	}
	for _, message := range result.Messages {
		if !strings.Contains(message.Subject, "Reseña reportada") {
			continue
		}
		for _, to := range message.To {
			if strings.EqualFold(to.Address, recipient) {
				return
			}
		}
	}
	t.Fatalf("Mailpit has no review-report message for %s", recipient)
}

func testReceivedReviewSuppression(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adminID string, at time.Time, reservation, renter, host string) {
	t.Helper()
	privacyService, e := privacy.NewService(identitypg.NewIdentityRepository(pool))
	if e != nil {
		t.Fatal(e)
	}
	request, e := privacyService.RequestRight(ctx, renter, "supresion", "api")
	if e != nil {
		t.Fatal(e)
	}
	assessment, e := privacyService.ReviewSuppression(ctx, adminID, request.ID, "comm-received-review-assess", "comm-received-review", func() time.Time { return at })
	if e != nil || assessment.Outcome != "elegible" {
		t.Fatalf("review suppression assessment=%+v err=%v", assessment, e)
	}
	result, e := privacyService.ExecuteSuppression(ctx, adminID, request.ID, "comm-received-review-exec", "comm-received-review", func() time.Time { return at }, nil)
	if e != nil || result.Status == "bloqueada" {
		t.Fatalf("real suppression with received review: %+v err=%v", result, e)
	}
	var state, targetType, comment string
	var targetID *string
	if e = pool.QueryRow(ctx, `SELECT u.estado,r.destinatario_tipo,r.destinatario_id::text,r.comentario FROM public.usuario u JOIN public.resena_ensayo_local r ON r.reserva_id=$2 AND r.autor_id=$3 WHERE u.id=$1`, renter, reservation, host).Scan(&state, &targetType, &targetID, &comment); e != nil {
		t.Fatal(e)
	}
	if state != "desidentificado" || targetType != "arrendatario_retirado" || targetID != nil || comment != "" {
		t.Fatalf("received review minimization state=%s type=%s target=%v comment=%q", state, targetType, targetID, comment)
	}
}

type raceFixture struct{ host, renter, space, quote, reservation, occupancy string }

func seedSuppressionRaceReservation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix int, at time.Time) raceFixture {
	t.Helper()
	id := func(group uint32, n uint64) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", group, n) }
	f := raceFixture{
		host: id(0x12000000, uint64(suffix*10+1)), renter: id(0x12000000, uint64(suffix*10+2)),
		space: id(0x34000000, uint64(suffix)), quote: id(0x45000000, uint64(suffix)),
		reservation: id(0x23000000, uint64(suffix)), occupancy: id(0x56000000, uint64(suffix)),
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-hash','activo'),($3,$4,$4,'synthetic-hash','activo')`, f.host, fmt.Sprintf("comm-race-%d-host@example.invalid", suffix), f.renter, fmt.Sprintf("comm-race-%d-renter@example.invalid", suffix)); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','Race review space',repeat('Synthetic local race fixture description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, f.space, f.host); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, f.space); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, f.space); e != nil {
		t.Fatal(e)
	}
	start, end := at.Add(-48*time.Hour), at.Add(-24*time.Hour)
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, e = tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en,condiciones_snapshot,categoria_codigo,perfil_version,perfil_valores_snapshot) VALUES($1,$2,$3,$4,1,'hora',8000,'CLP',24,192000,$5,$6,'America/Santiago',$7,$7::timestamptz+interval '15 minutes','Synthetic terms','sala_multiproposito',1,'{}'::jsonb)`, f.quote, f.space, f.host, f.renter, start, end, at); e != nil {
		t.Fatal(e)
	}
	if _, e := tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,creada_en) VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'reserva',true,$6)`, f.occupancy, f.space, f.reservation, start, end, at); e != nil {
		t.Fatal(e)
	}
	if _, e := tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,pago_vence_en,creada_en,actualizada_en,condiciones_snapshot,politica_cancelacion_version,vinculos_retirar_en) VALUES($1,$2,$3,$4,$5,$6,repeat('b',32)::bytea,$7,'finalizada',8000,24,192000,'hora','CLP',$8,$9,'America/Santiago',$10,$11,$11,'Synthetic terms','local_flexible_v1',$12)`, f.reservation, f.quote, f.space, f.host, f.renter, fmt.Sprintf("comm-race-reservation-%d", suffix), f.occupancy, start, end, at.Add(time.Hour), at, at.AddDate(0, 24, 0)); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return f
}

func prepareSuppression(t *testing.T, ctx context.Context, pool *pgxpool.Pool, adminID, subject string, at time.Time, suffix string) (*privacy.Service, privacy.RightsRequest) {
	t.Helper()
	s, e := privacy.NewService(identitypg.NewIdentityRepository(pool))
	if e != nil {
		t.Fatal(e)
	}
	req, e := s.RequestRight(ctx, subject, "supresion", "api")
	if e != nil {
		t.Fatal(e)
	}
	assessment, e := s.ReviewSuppression(ctx, adminID, req.ID, "comm-assess-"+suffix, "comm-corr-"+suffix, func() time.Time { return at })
	if e != nil || assessment.Outcome != "elegible" {
		t.Fatalf("suppression assessment=%+v err=%v", assessment, e)
	}
	return s, req
}

func waitForRuntimeLockWaits(t *testing.T, ctx context.Context, pool *pgxpool.Pool, minimum int) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		e := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE usename=current_user AND wait_event_type='Lock' AND pid<>pg_backend_pid()`).Scan(&waiting)
		if e != nil {
			t.Fatal(e)
		}
		if waiting >= minimum {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d PostgreSQL lock waiters", minimum)
}

func testReviewAndReportSuppressionRaces(t *testing.T, ctx context.Context, pool *pgxpool.Pool, svc *domain.Service, adminID string, at time.Time) {
	t.Helper()
	createFixture := seedSuppressionRaceReservation(t, ctx, pool, 10, at)
	privacyService, req := prepareSuppression(t, ctx, pool, adminID, createFixture.renter, at, "create-race")
	blocker, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = blocker.Exec(ctx, `SELECT id FROM public.usuario WHERE id=$1 FOR UPDATE`, createFixture.renter); e != nil {
		t.Fatal(e)
	}
	type suppressionResult struct {
		result privacy.SuppressionExecution
		err    error
	}
	suppressed := make(chan suppressionResult, 1)
	go func() {
		v, err := privacyService.ExecuteSuppression(ctx, adminID, req.ID, "comm-execute-create-race", "comm-create-race", func() time.Time { return at }, nil)
		suppressed <- suppressionResult{v, err}
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 1)
	created := make(chan error, 1)
	go func() {
		_, err := svc.Create(ctx, createFixture.renter, createFixture.reservation, "comm-review-create-race-key", domain.ReviewInput{Rating: 5})
		created <- err
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 2)
	if e = blocker.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if result := <-suppressed; result.err != nil || result.result.Status == "bloqueada" {
		t.Fatalf("suppression in review race=%+v err=%v", result.result, result.err)
	}
	if err := <-created; !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("review inserted after suppression won lock: %v", err)
	}
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.resena_ensayo_local WHERE reserva_id=$1`, createFixture.reservation).Scan(&count); e != nil || count != 0 {
		t.Fatalf("racing review rows=%d err=%v", count, e)
	}

	reportFixture := seedSuppressionRaceReservation(t, ctx, pool, 20, at)
	review, e := svc.Create(ctx, reportFixture.renter, reportFixture.reservation, "comm-report-race-review", domain.ReviewInput{Rating: 4})
	if e != nil {
		t.Fatal(e)
	}
	reportPrivacy, reportReq := prepareSuppression(t, ctx, pool, adminID, reportFixture.host, at, "report-race")
	blocker, e = pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = blocker.Exec(ctx, `SELECT id FROM public.usuario WHERE id=$1 FOR UPDATE`, reportFixture.host); e != nil {
		t.Fatal(e)
	}
	suppressed = make(chan suppressionResult, 1)
	go func() {
		v, err := reportPrivacy.ExecuteSuppression(ctx, adminID, reportReq.ID, "comm-execute-report-race", "comm-report-race", func() time.Time { return at }, nil)
		suppressed <- suppressionResult{v, err}
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 1)
	reported := make(chan error, 1)
	go func() {
		_, err := svc.ReportReview(ctx, reportFixture.host, reportFixture.reservation, review.ID, "comm-report-race-key", "spam")
		reported <- err
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 2)
	if e = blocker.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if result := <-suppressed; result.err != nil || result.result.Status == "bloqueada" {
		t.Fatalf("suppression in report race=%+v err=%v", result.result, result.err)
	}
	if err := <-reported; !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("report inserted after suppression won lock: %v", err)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM public.reporte_resena_ensayo_local WHERE resena_id=$1`, review.ID).Scan(&count); e != nil || count != 0 {
		t.Fatalf("racing report rows=%d err=%v", count, e)
	}
}

func testReviewAndClaimLockOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, svc *domain.Service, at time.Time) {
	t.Helper()
	f := seedSuppressionRaceReservation(t, ctx, pool, 40, at)
	const operationID = "66000000-0000-4000-8000-000000000040"
	const evidenceID = "67000000-0000-4000-8000-000000000040"
	location := `{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.4489,"longitude":-70.6693}`
	if _, err := pool.Exec(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en) VALUES($1,$2,'checkout',$3,$4,'America/Santiago',$5::jsonb,'registrada','review-claim-checkout',repeat('c',32)::bytea,$4)`, operationID, f.reservation, f.renter, at.Add(-30*time.Minute), location); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en) VALUES($1,$2,'synthetic-png-v1','image/png',repeat('a',64),128,$3)`, evidenceID, operationID, at.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	claimService, err := damageclaim.New(damageclaimpg.New(pool), func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	// Hold the later-sorted participant. Claim opening first locks the host,
	// then waits for the renter; review creation must wait on the host before it
	// can lock the reservation. The old reservation-first order deadlocked here.
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(context.Background()) }()
	if _, err = blocker.Exec(ctx, `SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE`, f.renter); err != nil {
		t.Fatal(err)
	}
	claimResult := make(chan error, 1)
	go func() {
		_, e := claimService.Open(ctx, f.host, f.reservation, "review-claim-deadlock-key", damageclaim.Input{Description: "Synthetic concurrent claim."})
		claimResult <- e
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 1)
	reviewResult := make(chan error, 1)
	go func() {
		_, e := svc.Create(ctx, f.renter, f.reservation, "review-claim-review-key", domain.ReviewInput{Rating: 5})
		reviewResult <- e
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 2)
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-claimResult:
		if err != nil {
			t.Fatalf("claim operation deadlocked or failed: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("claim operation did not finish after releasing account lock")
	}
	select {
	case err = <-reviewResult:
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("review should observe the claim transition, got %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("review operation did not finish after claim committed")
	}
	var reviewCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.resena_ensayo_local WHERE reserva_id=$1`, f.reservation).Scan(&reviewCount); err != nil || reviewCount != 0 {
		t.Fatalf("review inserted despite claim transition: count=%d err=%v", reviewCount, err)
	}
}

func testNoticeDispatchSuppressionRace(t *testing.T, ctx context.Context, pool *pgxpool.Pool, subject, adminID, reservation string, at time.Time) {
	t.Helper()
	const noticeID = "66000000-0000-4000-8000-000000000091"
	if _, e := pool.Exec(ctx, `INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,creada_en,proximo_intento_en) VALUES($1,'checkin_registrado','reserva',$2,$3,'comm-dispatch-race',$4,$4)`, noticeID, reservation, subject, at); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en) VALUES($1,1,'pendiente',$2)`, noticeID, at); e != nil {
		t.Fatal(e)
	}
	privacyService, request := prepareSuppression(t, ctx, pool, adminID, subject, at, "dispatch-race")
	sender := &blockingNoticeSender{entered: make(chan struct{}), release: make(chan struct{})}
	service, e := localnotice.New(localnoticepg.New(pool), sender, func() time.Time { return at })
	if e != nil {
		t.Fatal(e)
	}
	dispatched := make(chan error, 1)
	go func() { _, err := service.DispatchOne(ctx); dispatched <- err }()
	select {
	case <-sender.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	suppressed := make(chan error, 1)
	go func() {
		_, err := privacyService.ExecuteSuppression(ctx, adminID, request.ID, "comm-execute-dispatch-race", "comm-dispatch-race", func() time.Time { return at }, nil)
		suppressed <- err
	}()
	waitForRuntimeLockWaits(t, ctx, pool, 1)
	close(sender.release)
	if e = <-dispatched; e != nil {
		t.Fatalf("dispatch before baja commit: %v", e)
	}
	if e = <-suppressed; e != nil {
		t.Fatalf("baja after terminal dispatch: %v", e)
	}
	var state string
	if e = pool.QueryRow(ctx, `SELECT estado FROM public.aviso_local WHERE id=$1`, noticeID).Scan(&state); e != nil || state != "entregada" {
		t.Fatalf("notice state after dispatch/baja=%s err=%v", state, e)
	}
	const afterID = "66000000-0000-4000-8000-000000000092"
	if _, e = pool.Exec(ctx, `INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,creada_en,proximo_intento_en) VALUES($1,'checkin_registrado','reserva',$2,$3,'comm-inactive-recipient',$4,$4)`, afterID, reservation, subject, at); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en) VALUES($1,1,'pendiente',$2)`, afterID, at); e != nil {
		t.Fatal(e)
	}
	quietSender := &localNoticeTestSender{}
	quiet, e := localnotice.New(localnoticepg.New(pool), quietSender, func() time.Time { return at })
	if e != nil {
		t.Fatal(e)
	}
	if claimed, err := quiet.DispatchOne(ctx); err != nil || claimed || quietSender.sent != 0 {
		t.Fatalf("inactive recipient dispatch claimed=%v sends=%d err=%v", claimed, quietSender.sent, err)
	}
	if e = pool.QueryRow(ctx, `SELECT estado FROM public.aviso_local WHERE id=$1`, afterID).Scan(&state); e != nil || state != "cancelada" {
		t.Fatalf("post-baja notice state=%s err=%v", state, e)
	}
}

type blockingNoticeSender struct{ entered, release chan struct{} }

func (s *blockingNoticeSender) SendLocalBookingNotice(ctx context.Context, _, _, _ string) error {
	close(s.entered)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return nil
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
