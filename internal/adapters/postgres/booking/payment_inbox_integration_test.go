package bookingpg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type countedPaymentAdapter struct {
	inner   *fakebooking.Adapter
	starts  atomic.Int64
	lookups atomic.Int64
}

type gatedPaymentAdapter struct {
	*countedPaymentAdapter
	started chan struct{}
	looked  chan struct{}
	release chan struct{}
}

func seedPaymentReservation(t *testing.T, ctx context.Context, setup *pgx.Conn, spaceID, host, renter, quoteID, reservationID, occupancyID string, now time.Time, offset time.Duration) {
	t.Helper()
	start, end := now.Add(offset), now.Add(offset+time.Hour)
	tx, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,creada_en,vence_en,categoria_codigo,perfil_version,perfil_valores_snapshot)
VALUES($1,$2,$3,$4,1,'hora',8000,'CLP',1,8000,$5,$6,'America/Santiago','Reglas fake',$7,$8,'sala_multiproposito',1,'{}')`, quoteID, spaceID, host, renter, start, end, now, now.Add(15*time.Minute))
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,condiciones_snapshot,politica_cancelacion_version,pago_vence_en,creada_en,actualizada_en)
VALUES($1,$2,$3,$4,$5,$6,decode(repeat('02',32),'hex'),$7,'pendiente_de_pago',8000,1,8000,'hora','CLP',$8,$9,'America/Santiago','Reglas fake','local_flexible_v1',$10,$11,$11)`, reservationID, quoteID, spaceID, host, renter, "request-"+reservationID, occupancyID, start, end, now.Add(15*time.Minute), now)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,expira_en,creada_en)
VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'retencion',true,$6,$7)`, occupancyID, spaceID, reservationID, start, end, now.Add(15*time.Minute), now)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en)
VALUES(gen_random_uuid(),$1,1,NULL,'pendiente_de_pago',$2,'solicitud sintética',$3)`, reservationID, renter, now)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func (a *gatedPaymentAdapter) StartPayment(ctx context.Context, operationID, requested string) (*booking.PaymentEvent, error) {
	a.starts.Add(1)
	select {
	case a.started <- struct{}{}:
	default:
	}
	select {
	case <-a.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return a.inner.StartPayment(ctx, operationID, requested)
}
func (a *gatedPaymentAdapter) LookupPayment(ctx context.Context, operation booking.PaymentOperation) (*booking.PaymentEvent, error) {
	select {
	case a.looked <- struct{}{}:
	default:
	}
	return a.countedPaymentAdapter.LookupPayment(ctx, operation)
}

func (a *countedPaymentAdapter) StartPayment(ctx context.Context, operationID, requested string) (*booking.PaymentEvent, error) {
	a.starts.Add(1)
	return a.inner.StartPayment(ctx, operationID, requested)
}
func (a *countedPaymentAdapter) LookupPayment(ctx context.Context, operation booking.PaymentOperation) (*booking.PaymentEvent, error) {
	a.lookups.Add(1)
	return a.inner.LookupPayment(ctx, operation)
}
func (a *countedPaymentAdapter) VerifyPaymentEvent(event booking.PaymentEvent) bool {
	return a.inner.VerifyPaymentEvent(event)
}

func TestDurableFakePaymentInboxDeduplicatesTimeoutAndRecoversAfterRestart(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	dbName := fmt.Sprintf("payment_inbox_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	dbURLParsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	dbURLParsed.Path = "/" + dbName
	dbURL := dbURLParsed.String()
	var setup *pgx.Conn
	var pool *pgxpool.Pool
	defer func() {
		if pool != nil {
			pool.Close()
		}
		if setup != nil {
			_ = setup.Close(context.Background())
		}
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanCancel()
		_, _ = admin.Exec(cleanCtx, `DROP DATABASE `+pgx.Identifier{dbName}.Sanitize()+` WITH (FORCE)`)
		_, _ = admin.Exec(cleanCtx, `DROP ROLE IF EXISTS espacigo_runtime`)
		_ = admin.Close(context.Background())
	}()
	if _, err = migrator.Run(ctx, dbURL, "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE ROLE espacigo_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD 'payment-inbox-test'`); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bootstrap.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{dbName}.Sanitize()+` TO espacigo_runtime`); err != nil {
		t.Fatal(err)
	}
	if err = dbbootstrap.GrantRuntimePermissions(ctx, bootstrap); err != nil {
		t.Fatal(err)
	}
	_ = bootstrap.Close(ctx)
	runtimeURL, err := url.Parse(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	runtimeURL.User = url.UserPassword("espacigo_runtime", "payment-inbox-test")
	pool, err = pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	setup, err = pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	var operationRead, operationInsert, operationUpdate bool
	var eventRead, eventInsert, eventUpdate, eventDelete bool
	var fakeRead, fakeInsert, fakeUpdate, fakeDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_pago_ensayo_operacion','SELECT'),has_table_privilege(current_user,'public.reserva_pago_ensayo_operacion','INSERT'),has_table_privilege(current_user,'public.reserva_pago_ensayo_operacion','UPDATE')`).Scan(&operationRead, &operationInsert, &operationUpdate); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','SELECT'),has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','INSERT'),has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','UPDATE'),has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','DELETE')`).Scan(&eventRead, &eventInsert, &eventUpdate, &eventDelete); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_pago_fake_resultado_ensayo','SELECT'),has_table_privilege(current_user,'public.reserva_pago_fake_resultado_ensayo','INSERT'),has_table_privilege(current_user,'public.reserva_pago_fake_resultado_ensayo','UPDATE'),has_table_privilege(current_user,'public.reserva_pago_fake_resultado_ensayo','DELETE')`).Scan(&fakeRead, &fakeInsert, &fakeUpdate, &fakeDelete); err != nil {
		t.Fatal(err)
	}
	if !operationRead || !operationInsert || !operationUpdate || !eventRead || !eventInsert || eventUpdate || eventDelete {
		t.Fatalf("runtime payment grants: operation R/I/U=%v/%v/%v immutable event R/I/U/D=%v/%v/%v/%v", operationRead, operationInsert, operationUpdate, eventRead, eventInsert, eventUpdate, eventDelete)
	}
	if !fakeRead || !fakeInsert || !fakeUpdate || fakeDelete {
		t.Fatalf("runtime fake-result grants R/I/U/D=%v/%v/%v/%v", fakeRead, fakeInsert, fakeUpdate, fakeDelete)
	}

	host, renter := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	spaceID, quoteID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc", "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	reservationID, occupancyID := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", "ffffffff-ffff-4fff-8fff-ffffffffffff"
	for index, id := range []string{host, renter} {
		if _, err = setup.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'x','activo')`, id, fmt.Sprintf("payment-%d@example.test", index)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2035, 1, 2, 15, 4, 5, 0, time.UTC)
	start, end := now.Add(24*time.Hour), now.Add(25*time.Hour)
	tx, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria)
VALUES($1,$2,'sala_multiproposito','Pago sintético',repeat('Fixture local. ',10),20,3,'Reglas fake','hora',8000,'Privada','America/Santiago')`, spaceID, host)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,creada_en,vence_en,categoria_codigo,perfil_version,perfil_valores_snapshot)
VALUES($1,$2,$3,$4,1,'hora',8000,'CLP',1,8000,$5,$6,'America/Santiago','Reglas fake',$7,$8,'sala_multiproposito',1,'{}')`, quoteID, spaceID, host, renter, start, end, now, now.Add(15*time.Minute))
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,condiciones_snapshot,politica_cancelacion_version,pago_vence_en,creada_en,actualizada_en)
VALUES($1,$2,$3,$4,$5,'request-key',decode(repeat('01',32),'hex'),$6,'pendiente_de_pago',8000,1,8000,'hora','CLP',$7,$8,'America/Santiago','Reglas fake','local_flexible_v1',$9,$10,$10)`, reservationID, quoteID, spaceID, host, renter, occupancyID, start, end, now.Add(15*time.Minute), now)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,expira_en,creada_en)
VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'retencion',true,$6,$7)`, occupancyID, spaceID, reservationID, start, end, now.Add(15*time.Minute), now)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en)
VALUES(gen_random_uuid(),$1,1,NULL,'pendiente_de_pago',$2,'solicitud sintética',$3)`, reservationID, renter, now)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	const eventSecret = "stable-test-local-payment-webhook-key-at-least-32-bytes"
	currentTime := now
	clock := func() time.Time { return currentTime }
	repo := New(pool)
	inner1, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	adapter1 := &gatedPaymentAdapter{countedPaymentAdapter: &countedPaymentAdapter{inner: inner1}, started: make(chan struct{}, 1), looked: make(chan struct{}, 2), release: make(chan struct{})}
	if _, _, err = repo.BeginPayment(ctx, host, reservationID, "exito", "foreign-payment-key", make([]byte, 32), "11111111-1111-4111-8111-111111111111", clock); !errors.Is(err, booking.ErrNotFound) {
		t.Fatalf("foreign reservation BeginPayment err=%v; want not found", err)
	}
	if _, _, err = repo.BeginPayment(ctx, renter, "22222222-2222-4222-8222-222222222222", "exito", "missing-payment-key", make([]byte, 32), "33333333-3333-4333-8333-333333333333", clock); !errors.Is(err, booking.ErrNotFound) {
		t.Fatalf("missing reservation BeginPayment err=%v; want not found", err)
	}
	service1, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, adapter1, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	type paymentCall struct {
		reservation booking.Reservation
		err         error
	}
	firstCall, replayCall := make(chan paymentCall, 1), make(chan paymentCall, 1)
	go func() {
		reservation, callErr := service1.Pay(ctx, renter, reservationID, "sin_respuesta", "payment-key-stable")
		firstCall <- paymentCall{reservation: reservation, err: callErr}
	}()
	select {
	case <-adapter1.started:
	case <-ctx.Done():
		t.Fatal("fake payment start did not reach its gate")
	}
	go func() {
		reservation, callErr := service1.Pay(ctx, renter, reservationID, "sin_respuesta", "payment-key-stable")
		replayCall <- paymentCall{reservation: reservation, err: callErr}
	}()
	select {
	case <-adapter1.looked:
	case <-ctx.Done():
		t.Fatal("concurrent idempotent request did not query the existing operation")
	}
	close(adapter1.release)
	if call := <-firstCall; !errors.Is(call.err, booking.ErrSimulatedNoResponse) || call.reservation.State != "pendiente_de_pago" {
		t.Fatalf("first fake timeout reservation=%+v err=%v", call.reservation, call.err)
	}
	if call := <-replayCall; !errors.Is(call.err, booking.ErrSimulatedNoResponse) || call.reservation.State != "pendiente_de_pago" {
		t.Fatalf("concurrent idempotent timeout replay reservation=%+v err=%v", call.reservation, call.err)
	}
	if _, err = service1.Pay(ctx, renter, reservationID, "exito", "different-charge-key"); !errors.Is(err, booking.ErrConflict) {
		t.Fatalf("second payment key after timeout err=%v; want conflict", err)
	}
	var operationID string
	if err = pool.QueryRow(ctx, `SELECT id::text FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 AND clave_idempotencia='payment-key-stable'`, reservationID).Scan(&operationID); err != nil {
		t.Fatal(err)
	}
	var fakeResultRows int
	var fakeResultState string
	if err = pool.QueryRow(ctx, `SELECT count(*),min(estado) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1`, operationID).Scan(&fakeResultRows, &fakeResultState); err != nil || fakeResultRows != 1 || fakeResultState != "sin_respuesta" {
		t.Fatalf("timeout fake records=%d state=%q err=%v; retries must not create a charge", fakeResultRows, fakeResultState, err)
	}
	lateEvent := *inner1.SignPaymentEvent(operationID, "exito_simulado")
	firstReceipt, err := service1.IngestPaymentEvent(ctx, lateEvent)
	if err != nil || !firstReceipt.Accepted || firstReceipt.Reused {
		t.Fatalf("first authenticated event receipt=%+v err=%v", firstReceipt, err)
	}
	replayReceipt, err := service1.IngestPaymentEvent(ctx, lateEvent)
	if err != nil || !replayReceipt.Accepted || !replayReceipt.Reused {
		t.Fatalf("event replay receipt=%+v err=%v", replayReceipt, err)
	}
	unauthenticated := lateEvent
	unauthenticated.Signature = "invalid"
	if _, err = service1.IngestPaymentEvent(ctx, unauthenticated); !errors.Is(err, booking.ErrUnauthenticatedPaymentEvent) {
		t.Fatalf("invalid callback signature err=%v", err)
	}
	var inboxCount, applicationCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationID).Scan(&inboxCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_aplicacion_ensayo a JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id WHERE e.operacion_id=$1 AND a.estado='pendiente'`, operationID).Scan(&applicationCount); err != nil {
		t.Fatal(err)
	}
	if inboxCount != 1 || applicationCount != 1 {
		t.Fatalf("inbox unique event/pending processing rows=%d/%d", inboxCount, applicationCount)
	}

	// Model a process restart: a new Service and adapter instance read the
	// authenticated inbox from PostgreSQL and apply it without another charge.
	inner2, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	adapter2 := &countedPaymentAdapter{inner: inner2}
	service2, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, adapter2, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = service2.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	if adapter2.starts.Load() != 0 || adapter2.lookups.Load() != 0 {
		t.Fatalf("inbox recovery made adapter calls start/lookup=%d/%d", adapter2.starts.Load(), adapter2.lookups.Load())
	}
	var state, operationState, eventState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_pago_ensayo_operacion WHERE id=$1`, operationID).Scan(&operationState); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT a.estado FROM public.reserva_pago_evento_aplicacion_ensayo a JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id WHERE e.operacion_id=$1`, operationID).Scan(&eventState); err != nil {
		t.Fatal(err)
	}
	if state != "pagada" || operationState != "aplicada" || eventState != "aplicada" {
		t.Fatalf("recovered states reservation/operation/event=%s/%s/%s", state, operationState, eventState)
	}
	if replayAfterRestart, replayErr := service2.IngestPaymentEvent(ctx, lateEvent); replayErr != nil || !replayAfterRestart.Reused {
		t.Fatalf("signed event replay after restart=%+v err=%v", replayAfterRestart, replayErr)
	}
	var payments, transitions, activeOccupancy int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'`, reservationID).Scan(&payments); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 AND estado_nuevo='pagada'`, reservationID).Scan(&transitions); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo AND tipo='reserva'`, reservationID).Scan(&activeOccupancy); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationID).Scan(&inboxCount); err != nil {
		t.Fatal(err)
	}
	if payments != 1 || transitions != 1 || activeOccupancy != 1 || inboxCount != 1 {
		t.Fatalf("replay duplicated side effects payment/history/occupancy/events=%d/%d/%d/%d", payments, transitions, activeOccupancy, inboxCount)
	}
	if replayPayment, replayErr := service2.Pay(ctx, renter, reservationID, "sin_respuesta", "payment-key-stable"); replayErr != nil || replayPayment.State != "pagada" || adapter2.starts.Load() != 0 {
		t.Fatalf("payment replay after settlement=%+v err=%v starts=%d", replayPayment, replayErr, adapter2.starts.Load())
	}

	// A durable intent with no fake result is not a success. On restart the
	// reconciler starts the fake with the operation key and then applies it.
	quoteBefore, reservationBefore, occupancyBefore := "12121212-1212-4121-8121-121212121212", "13131313-1313-4131-8131-131313131313", "14141414-1414-4141-8141-141414141414"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteBefore, reservationBefore, occupancyBefore, now, 72*time.Hour)
	operationBeforeID := "15151515-1515-4151-8151-151515151515"
	operationBefore, created, err := repo.BeginPayment(ctx, renter, reservationBefore, "exito", "before-start-key", make([]byte, 32), operationBeforeID, clock)
	if err != nil || !created {
		t.Fatalf("create intent before simulated crash=%+v created=%v err=%v", operationBefore, created, err)
	}
	if event, lookupErr := inner2.LookupPayment(ctx, operationBefore); lookupErr != nil || event != nil {
		t.Fatalf("unknown fake operation lookup fabricated event=%+v err=%v", event, lookupErr)
	}
	if err = service2.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var stateBefore string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationBefore).Scan(&stateBefore); err != nil || stateBefore != "pagada" {
		t.Fatalf("recovery before fake start reservation state=%q err=%v", stateBefore, err)
	}
	var fakeResultsBefore int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1 AND estado='resultado'`, operationBeforeID).Scan(&fakeResultsBefore); err != nil || fakeResultsBefore != 1 {
		t.Fatalf("recovery before start fake results=%d err=%v", fakeResultsBefore, err)
	}

	// Simulate a crash after the fake durably records its result but before the
	// Backend inserts the authenticated event into its inbox.
	quoteAfter, reservationAfter, occupancyAfter := "21212121-2121-4212-8212-212121212121", "23232323-2323-4232-8232-232323232323", "24242424-2424-4242-8242-242424242424"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteAfter, reservationAfter, occupancyAfter, now, 74*time.Hour)
	operationAfterID := "25252525-2525-4252-8252-252525252525"
	operationAfter, created, err := repo.BeginPayment(ctx, renter, reservationAfter, "exito", "after-start-key", make([]byte, 32), operationAfterID, clock)
	if err != nil || !created {
		t.Fatalf("create post-start intent=%+v created=%v err=%v", operationAfter, created, err)
	}
	producedEvent, err := inner2.StartPayment(ctx, operationAfter.ID, operationAfter.Requested)
	if err != nil || producedEvent == nil {
		t.Fatalf("fake result before simulated crash=%+v err=%v", producedEvent, err)
	}
	innerAfterRestart, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	adapterAfterRestart := &countedPaymentAdapter{inner: innerAfterRestart}
	serviceAfterRestart, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, adapterAfterRestart, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = serviceAfterRestart.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	if adapterAfterRestart.starts.Load() != 0 {
		t.Fatalf("recovery after fake result started it again: starts=%d", adapterAfterRestart.starts.Load())
	}
	var stateAfter string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationAfter).Scan(&stateAfter); err != nil || stateAfter != "pagada" {
		t.Fatalf("recovery after fake start reservation state=%q err=%v", stateAfter, err)
	}
	var resultRowsAfter, inboxRowsAfter int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1`, operationAfterID).Scan(&resultRowsAfter); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationAfterID).Scan(&inboxRowsAfter); err != nil {
		t.Fatal(err)
	}
	if resultRowsAfter != 1 || inboxRowsAfter != 1 {
		t.Fatalf("post-start restart duplicated fake result/inbox rows=%d/%d", resultRowsAfter, inboxRowsAfter)
	}

	// An authenticated callback arriving after cancellation is retained for
	// reconciliation and cannot reactivate the reservation or its occupancy.
	quoteCancelled, reservationCancelled, occupancyCancelled := "31313131-3131-4313-8313-313131313131", "32323232-3232-4323-8323-323232323232", "34343434-3434-4434-8434-343434343434"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteCancelled, reservationCancelled, occupancyCancelled, now, 76*time.Hour)
	operationCancelledID := "35353535-3535-4353-8353-353535353535"
	operationCancelled, _, err := repo.BeginPayment(ctx, renter, reservationCancelled, "exito", "cancelled-late-key", make([]byte, 32), operationCancelledID, clock)
	if err != nil {
		t.Fatal(err)
	}
	cancelledEvent, err := inner2.StartPayment(ctx, operationCancelled.ID, operationCancelled.Requested)
	if err != nil || cancelledEvent == nil {
		t.Fatalf("register fake result before cancellation=%+v err=%v", cancelledEvent, err)
	}
	if _, err = service2.Cancel(ctx, renter, reservationCancelled, "cancel-before-event", "test"); err != nil {
		t.Fatal(err)
	}
	// Simulate a restart with the fake result persisted but no callback saved
	// by the Backend. Reconciliation must retrieve and authenticate it itself.
	cancelRestartInner, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	cancelRestartAdapter := &countedPaymentAdapter{inner: cancelRestartInner}
	cancelRestartService, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, cancelRestartAdapter, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = cancelRestartService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	if cancelRestartAdapter.starts.Load() != 0 || cancelRestartAdapter.lookups.Load() == 0 {
		t.Fatalf("cancel recovery start/lookup=%d/%d; want lookup only", cancelRestartAdapter.starts.Load(), cancelRestartAdapter.lookups.Load())
	}
	var cancelledState, cancelledApplication string
	var cancelledActive bool
	if err = pool.QueryRow(ctx, `SELECT r.estado,o.activo,a.estado FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id JOIN public.reserva_pago_ensayo_operacion p ON p.reserva_id=r.id JOIN public.reserva_pago_evento_ensayo e ON e.operacion_id=p.id JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE r.id=$1`, reservationCancelled).Scan(&cancelledState, &cancelledActive, &cancelledApplication); err != nil {
		t.Fatal(err)
	}
	if cancelledState != "cancelada_arrendatario" || cancelledActive || cancelledApplication != "pendiente_conciliacion" {
		t.Fatalf("cancelled late-event result state/occupancy/application=%s/%v/%s", cancelledState, cancelledActive, cancelledApplication)
	}
	var cancelledInbox, cancelledFakeResults int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationCancelledID).Scan(&cancelledInbox); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1 AND estado='resultado'`, operationCancelledID).Scan(&cancelledFakeResults); err != nil {
		t.Fatal(err)
	}
	if err = cancelRestartService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var cancelledInboxAfter int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationCancelledID).Scan(&cancelledInboxAfter); err != nil {
		t.Fatal(err)
	}
	if cancelledInbox != 1 || cancelledInboxAfter != cancelledInbox || cancelledFakeResults != 1 || cancelRestartAdapter.starts.Load() != 0 {
		t.Fatalf("cancel replay duplicated effects inbox=%d/%d fakeResults=%d starts=%d", cancelledInbox, cancelledInboxAfter, cancelledFakeResults, cancelRestartAdapter.starts.Load())
	}

	// If the Backend crashed before the fake recorded any result, cancellation
	// must not cause reconciliation to start a new payment.
	quoteCancelledEmpty, reservationCancelledEmpty, occupancyCancelledEmpty := "51515151-5151-4515-8515-515151515151", "52525252-5252-4525-8525-525252525252", "53535353-5353-4535-8535-535353535353"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteCancelledEmpty, reservationCancelledEmpty, occupancyCancelledEmpty, now, 79*time.Hour)
	operationCancelledEmptyID := "54545454-5454-4545-8545-545454545454"
	if _, _, err = repo.BeginPayment(ctx, renter, reservationCancelledEmpty, "exito", "cancelled-no-result-key", make([]byte, 32), operationCancelledEmptyID, clock); err != nil {
		t.Fatal(err)
	}
	if _, err = service2.Cancel(ctx, renter, reservationCancelledEmpty, "cancel-without-result", "test"); err != nil {
		t.Fatal(err)
	}
	noResultRestartInner, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	noResultRestartAdapter := &countedPaymentAdapter{inner: noResultRestartInner}
	noResultRestartService, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, noResultRestartAdapter, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = noResultRestartService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var cancelledNoResultState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationCancelledEmpty).Scan(&cancelledNoResultState); err != nil || cancelledNoResultState != "cancelada_arrendatario" || noResultRestartAdapter.starts.Load() != 0 {
		t.Fatalf("cancelled no-result recovery state=%q starts=%d err=%v", cancelledNoResultState, noResultRestartAdapter.starts.Load(), err)
	}

	// The same rule applies after the payment deadline without an expiry sweep:
	// the absent result is not permission to start a new fake payment.
	quoteExpiredEmpty, reservationExpiredEmpty, occupancyExpiredEmpty := "61616161-6161-4616-8616-616161616161", "62626262-6262-4626-8626-626262626262", "63636363-6363-4636-8636-636363636363"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteExpiredEmpty, reservationExpiredEmpty, occupancyExpiredEmpty, now, 80*time.Hour)
	operationExpiredEmptyID := "64646464-6464-4646-8646-646464646464"
	if _, _, err = repo.BeginPayment(ctx, renter, reservationExpiredEmpty, "exito", "expired-no-result-key", make([]byte, 32), operationExpiredEmptyID, clock); err != nil {
		t.Fatal(err)
	}
	currentTime = now.Add(16 * time.Minute)
	if err = noResultRestartService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var expiredNoResultState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationExpiredEmpty).Scan(&expiredNoResultState); err != nil || expiredNoResultState != "vencida_pago" || noResultRestartAdapter.starts.Load() != 0 {
		t.Fatalf("expired no-result recovery state=%q starts=%d err=%v", expiredNoResultState, noResultRestartAdapter.starts.Load(), err)
	}
	currentTime = now

	// A fake result may be durable at the provider before the Backend stores
	// its event, but only become observable to recovery after the deadline.
	quoteAfterDeadline, reservationAfterDeadline, occupancyAfterDeadline := "71717171-7171-4717-8717-717171717171", "72727272-7272-4727-8727-727272727272", "73737373-7373-4737-8737-737373737373"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteAfterDeadline, reservationAfterDeadline, occupancyAfterDeadline, now, 81*time.Hour)
	operationAfterDeadlineID := "74747474-7474-4747-8747-747474747474"
	operationAfterDeadline, _, err := repo.BeginPayment(ctx, renter, reservationAfterDeadline, "exito", "after-deadline-result-key", make([]byte, 32), operationAfterDeadlineID, clock)
	if err != nil {
		t.Fatal(err)
	}
	currentTime = now.Add(16 * time.Minute)
	if event, startErr := inner2.StartPayment(ctx, operationAfterDeadline.ID, operationAfterDeadline.Requested); startErr != nil || event == nil {
		t.Fatalf("persist fake result after deadline=%+v err=%v", event, startErr)
	}
	afterDeadlineInner, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	afterDeadlineAdapter := &countedPaymentAdapter{inner: afterDeadlineInner}
	afterDeadlineService, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, afterDeadlineAdapter, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = afterDeadlineService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var afterDeadlineState, afterDeadlineApplication string
	var afterDeadlineActive bool
	if err = pool.QueryRow(ctx, `SELECT r.estado,o.activo,a.estado FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id JOIN public.reserva_pago_ensayo_operacion p ON p.reserva_id=r.id JOIN public.reserva_pago_evento_ensayo e ON e.operacion_id=p.id JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE r.id=$1`, reservationAfterDeadline).Scan(&afterDeadlineState, &afterDeadlineActive, &afterDeadlineApplication); err != nil {
		t.Fatal(err)
	}
	var afterDeadlineInbox, afterDeadlineResults int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationAfterDeadlineID).Scan(&afterDeadlineInbox); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1 AND estado='resultado'`, operationAfterDeadlineID).Scan(&afterDeadlineResults); err != nil {
		t.Fatal(err)
	}
	if afterDeadlineState != "vencida_pago" || afterDeadlineActive || afterDeadlineApplication != "pendiente_conciliacion" || afterDeadlineInbox != 1 || afterDeadlineResults != 1 || afterDeadlineAdapter.starts.Load() != 0 {
		t.Fatalf("after-deadline recovery state/active/application/inbox/results/starts=%s/%v/%s/%d/%d/%d", afterDeadlineState, afterDeadlineActive, afterDeadlineApplication, afterDeadlineInbox, afterDeadlineResults, afterDeadlineAdapter.starts.Load())
	}
	if err = afterDeadlineService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var afterDeadlineInboxAgain int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationAfterDeadlineID).Scan(&afterDeadlineInboxAgain); err != nil {
		t.Fatal(err)
	}
	if afterDeadlineInboxAgain != afterDeadlineInbox || afterDeadlineAdapter.starts.Load() != 0 {
		t.Fatalf("after-deadline replay duplicated inbox or started payment: inbox=%d/%d starts=%d", afterDeadlineInbox, afterDeadlineInboxAgain, afterDeadlineAdapter.starts.Load())
	}
	currentTime = now
	quoteExpired, reservationExpired, occupancyExpired := "36363636-3636-4363-8363-363636363636", "37373737-3737-4373-8373-373737373737", "38383838-3838-4383-8383-383838383838"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteExpired, reservationExpired, occupancyExpired, now, 77*time.Hour)
	operationExpiredID := "39393939-3939-4393-8393-393939393939"
	operationExpired, _, err := repo.BeginPayment(ctx, renter, reservationExpired, "exito", "expired-late-key", make([]byte, 32), operationExpiredID, clock)
	if err != nil {
		t.Fatal(err)
	}
	expiredEvent, err := inner2.StartPayment(ctx, operationExpired.ID, operationExpired.Requested)
	if err != nil || expiredEvent == nil {
		t.Fatalf("expired late-event fake result=%+v err=%v", expiredEvent, err)
	}
	currentTime = now.Add(16 * time.Minute)
	if err = repo.Expire(ctx, currentTime); err != nil {
		t.Fatal(err)
	}
	var operationExpiredState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_pago_ensayo_operacion WHERE id=$1`, operationExpiredID).Scan(&operationExpiredState); err != nil || operationExpiredState != "vencida" {
		t.Fatalf("expiry sweep operation state=%q err=%v", operationExpiredState, err)
	}
	expiredRestartInner, err := fakebooking.NewWithStore([]byte(eventSecret), repo)
	if err != nil {
		t.Fatal(err)
	}
	expiredRestartAdapter := &countedPaymentAdapter{inner: expiredRestartInner}
	expiredRestartService, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, expiredRestartAdapter, 15*time.Minute, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = expiredRestartService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	if expiredRestartAdapter.starts.Load() != 0 || expiredRestartAdapter.lookups.Load() == 0 {
		t.Fatalf("expired recovery start/lookup=%d/%d; want lookup only", expiredRestartAdapter.starts.Load(), expiredRestartAdapter.lookups.Load())
	}
	var expiredState, expiredApplication string
	var expiredActive bool
	if err = pool.QueryRow(ctx, `SELECT r.estado,o.activo,a.estado FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id JOIN public.reserva_pago_ensayo_operacion p ON p.reserva_id=r.id JOIN public.reserva_pago_evento_ensayo e ON e.operacion_id=p.id JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE r.id=$1`, reservationExpired).Scan(&expiredState, &expiredActive, &expiredApplication); err != nil {
		t.Fatal(err)
	}
	if expiredState != "vencida_pago" || expiredActive || expiredApplication != "pendiente_conciliacion" {
		t.Fatalf("expired late-event result state/occupancy/application=%s/%v/%s", expiredState, expiredActive, expiredApplication)
	}
	var expiredInbox, expiredFakeResults int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationExpiredID).Scan(&expiredInbox); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1 AND estado='resultado'`, operationExpiredID).Scan(&expiredFakeResults); err != nil {
		t.Fatal(err)
	}
	if err = expiredRestartService.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var expiredInboxAfter int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_evento_ensayo WHERE operacion_id=$1`, operationExpiredID).Scan(&expiredInboxAfter); err != nil {
		t.Fatal(err)
	}
	if expiredInbox != 1 || expiredInboxAfter != expiredInbox || expiredFakeResults != 1 || expiredRestartAdapter.starts.Load() != 0 {
		t.Fatalf("expired replay duplicated effects inbox=%d/%d fakeResults=%d starts=%d", expiredInbox, expiredInboxAfter, expiredFakeResults, expiredRestartAdapter.starts.Load())
	}

	// Events authenticated before expiry are applied before the reconciliation
	// sweep even when processing occurs after the payment deadline.
	quoteTimely, reservationTimely, occupancyTimely := "41414141-4141-4414-8414-414141414141", "42424242-4242-4424-8424-424242424242", "43434343-4343-4434-8434-434343434343"
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quoteTimely, reservationTimely, occupancyTimely, now, 78*time.Hour)
	currentTime = now
	operationTimelyID := "44444444-4444-4444-8444-444444444444"
	operationTimely, _, err := repo.BeginPayment(ctx, renter, reservationTimely, "exito", "timely-event-key", make([]byte, 32), operationTimelyID, clock)
	if err != nil {
		t.Fatal(err)
	}
	timelyEvent, err := inner2.StartPayment(ctx, operationTimely.ID, operationTimely.Requested)
	if err != nil || timelyEvent == nil {
		t.Fatalf("timely fake result=%+v err=%v", timelyEvent, err)
	}
	if _, err = service2.IngestPaymentEvent(ctx, *timelyEvent); err != nil {
		t.Fatal(err)
	}
	currentTime = now.Add(16 * time.Minute)
	if err = repo.Expire(ctx, currentTime); err != nil {
		t.Fatal(err)
	}
	var beforeReconcile string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationTimely).Scan(&beforeReconcile); err != nil || beforeReconcile != "pendiente_de_pago" {
		t.Fatalf("expiry sweep discarded pre-deadline pending callback: state=%q err=%v", beforeReconcile, err)
	}
	if err = service2.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	var timelyState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, reservationTimely).Scan(&timelyState); err != nil || timelyState != "pagada" {
		t.Fatalf("pre-deadline event processed late reservation state=%q err=%v", timelyState, err)
	}
}
