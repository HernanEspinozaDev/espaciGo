package bookingpg

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	bookinghttp "github.com/HernanEspinozaDev/espaciGo/internal/booking/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type localSettlementAPITestAuth struct{ host, renter, administrator string }

type gatedSettlementAdapter struct {
	inner   booking.LocalSettlementAdapter
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (a *gatedSettlementAdapter) StartTransfer(ctx context.Context, operationID, requested string) (*booking.SettlementEvent, error) {
	a.once.Do(func() { close(a.entered) })
	select {
	case <-a.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return a.inner.StartTransfer(ctx, operationID, requested)
}
func (a *gatedSettlementAdapter) LookupTransfer(ctx context.Context, value booking.Settlement) (*booking.SettlementEvent, error) {
	return a.inner.LookupTransfer(ctx, value)
}
func (a *gatedSettlementAdapter) VerifyTransferEvent(event booking.SettlementEvent) bool {
	return a.inner.VerifyTransferEvent(event)
}
func (a *gatedSettlementAdapter) ResolveTransfer(ctx context.Context, operationID, outcome string) (booking.SettlementEvent, error) {
	return a.inner.ResolveTransfer(ctx, operationID, outcome)
}

func (a localSettlementAPITestAuth) Authorize(_ context.Context, raw identity.Secret, role identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	accounts := map[identity.Secret]string{"settlement-host-session": a.host, "settlement-renter-session": a.renter, "settlement-admin-session": a.administrator}
	account, ok := accounts[raw]
	if !ok {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	if role == identity.RoleAdministrator && raw != "settlement-admin-session" {
		return identity.Principal{}, identity.ErrForbidden
	}
	return identity.Principal{AccountID: account}, nil
}

func seedFinalizedSettlementReservation(t *testing.T, ctx context.Context, setup *pgx.Conn, host, renter, reservation, quote, occupancy string, now, checkout time.Time, offset time.Duration) {
	t.Helper()
	spaceID := "cccccccc-cccc-4ccc-8ccc-" + reservation[len(reservation)-12:]
	if _, err := setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria)
VALUES($1,$2,'sala_multiproposito','Liquidación sintética',repeat('Fixture local. ',10),20,3,'Reglas fake','hora',8000,'Privada','America/Santiago')`, spaceID, host); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	seedPaymentReservation(t, ctx, setup, spaceID, host, renter, quote, reservation, occupancy, now, offset)
	start, end := checkout.Add(-time.Hour), checkout
	if _, err := setup.Exec(ctx, `UPDATE public.cotizacion_reserva_ensayo SET inicio=$2,termino=$3,liquidacion_politica_version='liquidacion_local_0pct_v1',liquidacion_comision_bps=0 WHERE id=$1`, quote, start, end); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='finalizada',inicio=$2,termino=$3,actualizada_en=$4,liquidacion_politica_version='liquidacion_local_0pct_v1',liquidacion_comision_bps=0 WHERE id=$1`, reservation, start, end, now); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `UPDATE public.ocupacion SET intervalo=tstzrange($2,$3,'[)'),activo=false,expira_en=$4,desactivada_en=$4 WHERE reserva_id=$1`, reservation, start, end, now); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en) VALUES(gen_random_uuid(),$1,2,'en_curso','finalizada',$2,'check-out sintético',$3)`, reservation, renter, checkout); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en)
VALUES(gen_random_uuid(),$1,'checkout',$2,$3,'America/Santiago','{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.4489,"longitude":-70.6693}'::jsonb,'registrada','settlement-checkout',decode(repeat('04',32),'hex'),$3)`, reservation, renter, checkout); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en,importe_clp) VALUES(gen_random_uuid(),$1,'exito_simulado','rent-confirmed',$2,10000)`, reservation, now.Add(-30*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `INSERT INTO public.reserva_garantia_ensayo_local(reserva_id,politica_version,moneda,previsto_clp,autorizado_clp,capturado_clp,liberado_clp,estado,creada_en,actualizada_en)
VALUES($1,'garantia_local_fija_v1','CLP',50000,0,0,0,'liberada',$2,$2)`, reservation, now.Add(-30*time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	dbName := fmt.Sprintf("local_settlement_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + dbName
	dbURL := parsed.String()
	var setup *pgx.Conn
	var pool *pgxpool.Pool
	defer func() {
		if pool != nil {
			pool.Close()
		}
		if setup != nil {
			_ = setup.Close(context.Background())
		}
		cleanCtx, done := context.WithTimeout(context.Background(), 20*time.Second)
		defer done()
		if conn, e := pgx.Connect(cleanCtx, dbURL); e == nil {
			_, _ = conn.Exec(cleanCtx, `DO $$ BEGIN IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN DROP OWNED BY espacigo_runtime; END IF; END $$`)
			_ = conn.Close(cleanCtx)
		}
		_, _ = admin.Exec(cleanCtx, "DROP DATABASE "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		_, _ = admin.Exec(cleanCtx, "DROP ROLE IF EXISTS espacigo_runtime")
		_ = admin.Close(context.Background())
	}()
	if _, err = migrator.Run(ctx, dbURL, "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE ROLE espacigo_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD 'settlement-test-runtime'`); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bootstrap.Exec(ctx, "GRANT CONNECT ON DATABASE "+pgx.Identifier{dbName}.Sanitize()+" TO espacigo_runtime"); err != nil {
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
	runtimeURL.User = url.UserPassword("espacigo_runtime", "settlement-test-runtime")
	pool, err = pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	setup, err = pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	host, renter, adminID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	for i, id := range []string{host, renter, adminID} {
		if _, err = setup.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'x','activo')`, id, fmt.Sprintf("settlement-%d@example.test", i)); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2035, 1, 2, 15, 4, 5, 0, time.UTC)
	clockNow := base
	clock := func() time.Time { return clockNow }
	repo := New(pool)
	id1, q1, o1 := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	checkout1 := base.Add(-24*time.Hour + time.Second)
	seedFinalizedSettlementReservation(t, ctx, setup, host, renter, id1, q1, o1, base, checkout1, 72*time.Hour)
	// Before the strict checkout+24h threshold the host cannot create an intent.
	key1 := "settlement-success-key"
	adapter, err := fakebooking.NewWithStore([]byte("stable-test-settlement-event-key-with-at-least-32-bytes"), repo)
	if err != nil {
		t.Fatal(err)
	}
	service, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, adapter, time.Minute, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.SetLocalSettlementAdapter(adapter)
	if _, err = service.StartSettlement(ctx, host, id1, key1, "exito"); !errors.Is(err, booking.ErrConflict) {
		t.Fatalf("before 24h boundary err=%v", err)
	}
	var intents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_liquidacion_ensayo_local WHERE reserva_id=$1`, id1).Scan(&intents); err != nil || intents != 0 {
		t.Fatalf("pre-boundary settlement intent count=%d err=%v", intents, err)
	}
	// Hold the reservation row so the request starts before the deadline but
	// samples its injected clock only after it acquires the lock.
	blocker, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err = blocker.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, id1); err != nil {
		t.Fatal(err)
	}
	var blockerPID int
	if err = blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}
	type result struct {
		v   booking.Settlement
		err error
	}
	done := make(chan result, 1)
	go func() { v, e := service.StartSettlement(ctx, host, id1, key1, "exito"); done <- result{v, e} }()
	waitUntil := time.Now().Add(5 * time.Second)
	waiting := false
	for time.Now().Before(waitUntil) {
		err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity a WHERE a.datname=$1 AND $2=ANY(pg_blocking_pids(a.pid)))`, dbName, blockerPID).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("settlement did not block on the reservation row")
	}
	clockNow = checkout1.Add(24 * time.Hour)
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	call := <-done
	if call.err != nil || call.v.State != "liquidada" || call.v.TransferCLP != 10000 || call.v.CommissionCLP != 0 {
		t.Fatalf("boundary settlement=%+v err=%v", call.v, call.err)
	}
	// A concurrent retry with the same key reuses the one transfer, receipt,
	// ledger confirmation and immutable local snapshot.
	call1, call2 := make(chan result, 1), make(chan result, 1)
	start := make(chan struct{})
	for _, ch := range []chan result{call1, call2} {
		go func(ch chan result) {
			<-start
			v, e := service.StartSettlement(ctx, host, id1, key1, "exito")
			ch <- result{v, e}
		}(ch)
	}
	close(start)
	for _, ch := range []chan result{call1, call2} {
		v := <-ch
		if v.err != nil || v.v.State != "liquidada" {
			t.Fatalf("concurrent replay=%+v err=%v", v.v, v.err)
		}
	}
	var receiptCount, confirmHistory, fakeSuccess int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_comprobante_ensayo_local c JOIN public.reserva_liquidacion_ensayo_local s ON s.id=c.liquidacion_id WHERE s.reserva_id=$1`, id1).Scan(&receiptCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_liquidacion_historial_ensayo_local h JOIN public.reserva_liquidacion_ensayo_local s ON s.id=h.liquidacion_id WHERE s.reserva_id=$1 AND h.accion='liquidacion_confirmada'`, id1).Scan(&confirmHistory); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_liquidacion_resultado_fake_ensayo_local f JOIN public.reserva_liquidacion_operacion_ensayo_local o ON o.id=f.operacion_id JOIN public.reserva_liquidacion_ensayo_local s ON s.id=o.liquidacion_id WHERE s.reserva_id=$1 AND f.resultado='exito_simulado'`, id1).Scan(&fakeSuccess); err != nil {
		t.Fatal(err)
	}
	if receiptCount != 1 || confirmHistory != 1 || fakeSuccess != 1 {
		t.Fatalf("success side effects receipt/history/fake=%d/%d/%d", receiptCount, confirmHistory, fakeSuccess)
	}
	receipt, err := service.GetLocalReceipt(ctx, host, id1)
	if err != nil || receipt.Kind != "comprobante_ensayo" || receipt.Banner != booking.LocalSettlementBanner || receipt.TransferCLP != 10000 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	third := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if _, err = service.GetLocalReceipt(ctx, third, id1); !errors.Is(err, booking.ErrNotFound) {
		t.Fatalf("third-party receipt err=%v", err)
	}

	// Timeout remains uncertain across a new adapter/service instance. Recovery
	// resolves the persisted fake operation and does not start a second transfer.
	id2, q2, o2 := "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555"
	seedFinalizedSettlementReservation(t, ctx, setup, host, renter, id2, q2, o2, base, base.Add(-25*time.Hour), 80*time.Hour)
	timeout, timeoutErr := service.StartSettlement(ctx, host, id2, "settlement-timeout-key", "sin_respuesta")
	if !errors.Is(timeoutErr, booking.ErrSimulatedNoResponse) || timeout.State != "por_conciliar" {
		t.Fatalf("timeout=%+v err=%v", timeout, timeoutErr)
	}
	adapterRestart, err := fakebooking.NewWithStore([]byte("stable-test-settlement-event-key-with-at-least-32-bytes"), repo)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, adapterRestart, time.Minute, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetLocalSettlementAdapter(adapterRestart)
	if err = restarted.ReconcileSettlements(ctx); err != nil {
		t.Fatal(err)
	}
	still, err := restarted.GetSettlement(ctx, host, id2)
	if err != nil || still.State != "por_conciliar" {
		t.Fatalf("restart timeout settlement=%+v err=%v", still, err)
	}
	auth := localSettlementAPITestAuth{host: host, renter: renter, administrator: adminID}
	handler := bookinghttp.NewHandler(auth, restarted, nil)
	unauthenticatedReq := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+id1+"/settlement", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticatedReq)
	requestID := unauthenticatedResponse.Header().Get("X-Request-ID")
	if unauthenticatedResponse.Code != http.StatusUnauthorized || unauthenticatedResponse.Header().Get("WWW-Authenticate") != "Bearer" || requestID == "" || !strings.Contains(unauthenticatedResponse.Body.String(), `"request_id":"`+requestID+`"`) {
		t.Fatalf("unauthenticated settlement response status=%d headers=%v body=%s", unauthenticatedResponse.Code, unauthenticatedResponse.Header(), unauthenticatedResponse.Body.String())
	}
	reconcilePath := "/api/v1/local/booking-trial/reservations/" + id2 + "/settlement-reconcile"
	requestReconcile := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, reconcilePath, strings.NewReader(`{"outcome":"exito"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if forbidden := requestReconcile("settlement-host-session"); forbidden.Code != http.StatusForbidden {
		t.Fatalf("host reconciliation status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	if reconciled := requestReconcile("settlement-admin-session"); reconciled.Code != http.StatusOK || !strings.Contains(reconciled.Body.String(), `"state":"liquidada"`) {
		t.Fatalf("admin reconciliation status=%d body=%s", reconciled.Code, reconciled.Body.String())
	}
	still, err = restarted.GetSettlement(ctx, host, id2)
	if err != nil || still.State != "liquidada" {
		t.Fatalf("HTTP-reconciled settlement=%+v err=%v", still, err)
	}
	if err = restarted.ReconcileSettlements(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_liquidacion_resultado_fake_ensayo_local f JOIN public.reserva_liquidacion_operacion_ensayo_local o ON o.id=f.operacion_id JOIN public.reserva_liquidacion_ensayo_local s ON s.id=o.liquidacion_id WHERE s.reserva_id=$1`, id2).Scan(&fakeSuccess); err != nil {
		t.Fatal(err)
	}
	if fakeSuccess != 1 {
		t.Fatalf("timeout recovery fake result rows=%d", fakeSuccess)
	}
	// Rejection is terminal and never emits a receipt.
	id3, q3, o3 := "66666666-6666-4666-8666-666666666666", "77777777-7777-4777-8777-777777777777", "88888888-8888-4888-8888-888888888888"
	seedFinalizedSettlementReservation(t, ctx, setup, host, renter, id3, q3, o3, base, base.Add(-26*time.Hour), 88*time.Hour)
	rejected, err := service.StartSettlement(ctx, host, id3, "settlement-reject-key", "rechazo")
	if err != nil || rejected.State != "rechazada" {
		t.Fatalf("rejected settlement=%+v err=%v", rejected, err)
	}
	if _, err = service.GetLocalReceipt(ctx, host, id3); !errors.Is(err, booking.ErrNotFound) {
		t.Fatalf("rejected transfer created receipt, err=%v", err)
	}
	// A pending refund blocks settlement; after it is confirmed, the net fake
	// transfer subtracts exactly that refund and does not include the guarantee.
	id4, q4, o4 := "99999999-9999-4999-8999-999999999999", "aaaaaaaa-1111-4111-8111-111111111111", "bbbbbbbb-1111-4111-8111-111111111111"
	seedFinalizedSettlementReservation(t, ctx, setup, host, renter, id4, q4, o4, base, base.Add(-25*time.Hour), 96*time.Hour)
	refundID, refundOp := "cccccccc-1111-4111-8111-111111111111", "dddddddd-1111-4111-8111-111111111111"
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_devolucion_ensayo(id,reserva_id,operacion_id,importe_clp,moneda,estado,creada_en,actualizada_en) VALUES($1,$2,$3,2500,'CLP','pendiente',$4,$4)`, refundID, id4, refundOp, base); err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartSettlement(ctx, host, id4, "settlement-after-refund", "exito"); !errors.Is(err, booking.ErrConflict) {
		t.Fatalf("pending refund should block settlement, err=%v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_liquidacion_ensayo_local WHERE reserva_id=$1`, id4).Scan(&intents); err != nil || intents != 0 {
		t.Fatalf("pending refund created settlement intents=%d err=%v", intents, err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.reserva_devolucion_ensayo SET estado='completada',ultimo_resultado='exito_simulado',actualizada_en=$2,completada_en=$2 WHERE id=$1`, refundID, base); err != nil {
		t.Fatal(err)
	}
	refundCall1, refundCall2 := make(chan result, 1), make(chan result, 1)
	refundStart := make(chan struct{})
	for _, ch := range []chan result{refundCall1, refundCall2} {
		go func(ch chan result) {
			<-refundStart
			v, e := service.StartSettlement(ctx, host, id4, "settlement-after-refund", "exito")
			ch <- result{v, e}
		}(ch)
	}
	close(refundStart)
	for _, ch := range []chan result{refundCall1, refundCall2} {
		value := <-ch
		if value.err != nil || value.v.GrossRentCLP != 10000 || value.v.ConfirmedRefundsCLP != 2500 || value.v.TransferCLP != 7500 {
			t.Fatalf("concurrent refund-adjusted settlement=%+v err=%v", value.v, value.err)
		}
	}
	var refundSettlementRows int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_liquidacion_ensayo_local WHERE reserva_id=$1`, id4).Scan(&refundSettlementRows); err != nil || refundSettlementRows != 1 {
		t.Fatalf("concurrent requests created settlement rows=%d err=%v", refundSettlementRows, err)
	}
	// The participant endpoint supports idempotent HTTP replays and exposes the
	// generated receipt only to a reservation participant.
	startPath := "/api/v1/local/booking-trial/reservations/" + id4 + "/settlement"
	startReq := httptest.NewRequest(http.MethodPost, startPath, strings.NewReader(`{"outcome":"exito"}`))
	startReq.Header.Set("Authorization", "Bearer settlement-host-session")
	startReq.Header.Set("Content-Type", "application/json")
	startReq.Header.Set("Idempotency-Key", "settlement-after-refund")
	startResponse := httptest.NewRecorder()
	handler.ServeHTTP(startResponse, startReq)
	if startResponse.Code != http.StatusAccepted || !strings.Contains(startResponse.Body.String(), `"state":"liquidada"`) {
		t.Fatalf("host settlement replay status=%d body=%s", startResponse.Code, startResponse.Body.String())
	}
	receiptReq := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+id4+"/receipt", nil)
	receiptReq.Header.Set("Authorization", "Bearer settlement-renter-session")
	receiptResponse := httptest.NewRecorder()
	handler.ServeHTTP(receiptResponse, receiptReq)
	if receiptResponse.Code != http.StatusOK || !strings.Contains(receiptResponse.Body.String(), booking.LocalSettlementBanner) || !strings.Contains(receiptResponse.Body.String(), `"kind":"comprobante_ensayo"`) {
		t.Fatalf("participant receipt status=%d body=%s", receiptResponse.Code, receiptResponse.Body.String())
	}
	if nonHost, err := service.StartSettlement(ctx, renter, id3, "non-host-key", "exito"); !errors.Is(err, booking.ErrNotFound) {
		t.Fatalf("non-host settlement=%+v err=%v", nonHost, err)
	}
	var activeOccupancy int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, id3).Scan(&activeOccupancy); err != nil || activeOccupancy != 0 {
		t.Fatalf("settlement modified occupancy count=%d err=%v", activeOccupancy, err)
	}

	var receiptMutable, historyMutable bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_comprobante_ensayo_local','UPDATE') OR has_table_privilege(current_user,'public.reserva_comprobante_ensayo_local','DELETE'),has_table_privilege(current_user,'public.reserva_liquidacion_historial_ensayo_local','UPDATE') OR has_table_privilege(current_user,'public.reserva_liquidacion_historial_ensayo_local','DELETE')`).Scan(&receiptMutable, &historyMutable); err != nil {
		t.Fatal(err)
	}
	if receiptMutable || historyMutable {
		t.Fatalf("runtime can mutate immutable receipt/history: %v/%v", receiptMutable, historyMutable)
	}

	// Gate a request after its fake-result lookup returned empty. Another caller
	// persists and confirms the same event before the first request resumes.
	id5, q5, o5 := "eeeeeeee-1111-4111-8111-111111111111", "eeeeeeee-2222-4222-8222-222222222222", "eeeeeeee-3333-4333-8333-333333333333"
	seedFinalizedSettlementReservation(t, ctx, setup, host, renter, id5, q5, o5, base, base.Add(-25*time.Hour), 104*time.Hour)
	baseAdapter, err := fakebooking.NewWithStore([]byte("stable-test-settlement-event-key-with-at-least-32-bytes"), repo)
	if err != nil {
		t.Fatal(err)
	}
	gated := &gatedSettlementAdapter{inner: baseAdapter, entered: make(chan struct{}), release: make(chan struct{})}
	concurrentService, err := booking.NewServiceWithTTLs(repo, credentials.Generator{}, clock, baseAdapter, time.Minute, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	concurrentService.SetLocalSettlementAdapter(gated)
	type settlementCall struct {
		value booking.Settlement
		err   error
	}
	startCall := make(chan settlementCall, 1)
	go func() {
		value, startErr := concurrentService.StartSettlement(ctx, host, id5, "settlement-race-key", "exito")
		startCall <- settlementCall{value: value, err: startErr}
	}()
	select {
	case <-gated.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first transfer did not reach the deterministic post-query gate")
	}
	intent, err := repo.SettlementForAdministrator(ctx, id5)
	if err != nil {
		t.Fatal(err)
	}
	event, err := baseAdapter.StartTransfer(ctx, intent.OperationID, "exito")
	if err != nil || event == nil {
		t.Fatalf("concurrent fake result registration event=%+v err=%v", event, err)
	}
	fingerprint := sha256.Sum256([]byte(event.EventID + "\n" + event.OperationID + "\n" + event.Outcome))
	confirmed, _, err := repo.ApplySettlementResult(ctx, host, *event, fingerprint[:], clock)
	if err != nil || confirmed.State != "liquidada" {
		t.Fatalf("concurrent fake result confirmation=%+v err=%v", confirmed, err)
	}
	close(gated.release)
	call5 := <-startCall
	if call5.err != nil || call5.value.State != "liquidada" || !call5.value.Reused || call5.value.TransferCLP != confirmed.TransferCLP {
		t.Fatalf("racing request did not reuse persisted confirmation: value=%+v err=%v", call5.value, call5.err)
	}
	conflictingEvent := *baseAdapter.SignSettlementEvent(intent.OperationID, "rechazo_simulado")
	if _, _, err = repo.SaveFakeSettlementResult(ctx, conflictingEvent, clockNow); !errors.Is(err, booking.ErrConflict) {
		t.Fatalf("different concurrent fake result must conflict, err=%v", err)
	}
	var raceReceipts, raceFinals int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_comprobante_ensayo_local c JOIN public.reserva_liquidacion_ensayo_local s ON s.id=c.liquidacion_id WHERE s.reserva_id=$1),(SELECT count(*) FROM public.reserva_liquidacion_historial_ensayo_local h JOIN public.reserva_liquidacion_ensayo_local s ON s.id=h.liquidacion_id WHERE s.reserva_id=$1 AND h.accion='liquidacion_confirmada')`, id5).Scan(&raceReceipts, &raceFinals); err != nil {
		t.Fatal(err)
	}
	if raceReceipts != 1 || raceFinals != 1 {
		t.Fatalf("racing confirmation duplicated effects receipt/history=%d/%d", raceReceipts, raceFinals)
	}
}
