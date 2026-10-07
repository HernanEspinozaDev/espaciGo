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

func (a *gatedPaymentAdapter) StartPayment(ctx context.Context, operationID, requested string) (*booking.PaymentEvent, error) {
	a.starts.Add(1)
	a.started <- struct{}{}
	select {
	case <-a.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return a.inner.StartPayment(ctx, operationID, requested)
}
func (a *gatedPaymentAdapter) LookupPayment(ctx context.Context, operation booking.PaymentOperation) (*booking.PaymentEvent, error) {
	a.looked <- struct{}{}
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
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_pago_ensayo_operacion','SELECT'),has_table_privilege(current_user,'public.reserva_pago_ensayo_operacion','INSERT'),has_table_privilege(current_user,'public.reserva_pago_ensayo_operacion','UPDATE')`).Scan(&operationRead, &operationInsert, &operationUpdate); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','SELECT'),has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','INSERT'),has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','UPDATE'),has_table_privilege(current_user,'public.reserva_pago_evento_ensayo','DELETE')`).Scan(&eventRead, &eventInsert, &eventUpdate, &eventDelete); err != nil {
		t.Fatal(err)
	}
	if !operationRead || !operationInsert || !operationUpdate || !eventRead || !eventInsert || eventUpdate || eventDelete {
		t.Fatalf("runtime payment grants: operation R/I/U=%v/%v/%v immutable event R/I/U/D=%v/%v/%v/%v", operationRead, operationInsert, operationUpdate, eventRead, eventInsert, eventUpdate, eventDelete)
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
	clock := func() time.Time { return now }
	inner1, err := fakebooking.New([]byte(eventSecret))
	if err != nil {
		t.Fatal(err)
	}
	adapter1 := &gatedPaymentAdapter{countedPaymentAdapter: &countedPaymentAdapter{inner: inner1}, started: make(chan struct{}, 1), looked: make(chan struct{}, 2), release: make(chan struct{})}
	repo := New(pool)
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
	if got := adapter1.starts.Load(); got != 1 {
		t.Fatalf("fake charge start calls=%d; timeout/retry must not initiate another charge", got)
	}
	var operationID string
	if err = pool.QueryRow(ctx, `SELECT id::text FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 AND clave_idempotencia='payment-key-stable'`, reservationID).Scan(&operationID); err != nil {
		t.Fatal(err)
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
	inner2, err := fakebooking.New([]byte(eventSecret))
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
}
