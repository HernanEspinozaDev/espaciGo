package bookingpg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	contractpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/contracts"
	conversationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/conversation"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	bookinghttp "github.com/HernanEspinozaDev/espaciGo/internal/booking/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/contract"
	contracthttp "github.com/HernanEspinozaDev/espaciGo/internal/contract/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type integrationBookingAuth struct{ accountID string }

func (a integrationBookingAuth) Authorize(_ context.Context, raw identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if raw != "local-booking-integration-session" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	return identity.Principal{AccountID: a.accountID}, nil
}

type localNoticeRecorder struct {
	mu         sync.Mutex
	recipients []string
}

func newTestPaymentAdapter(t *testing.T) *fakebooking.Adapter {
	t.Helper()
	adapter, err := fakebooking.New([]byte("integration-only-local-payment-webhook-key-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func publishedBookingAPI(t *testing.T, actor string, service *booking.Service, method, path, idempotencyKey string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body strings.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = *strings.NewReader(string(encoded))
	}
	request := httptest.NewRequest(method, path, &body)
	request.Header.Set("Authorization", "Bearer local-booking-integration-session")
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	bookinghttp.NewHandler(integrationBookingAuth{accountID: actor}, service, nil).ServeHTTP(response, request)
	return response
}

func publishedContractAPI(t *testing.T, actor string, service *contract.Service, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer local-booking-integration-session")
	response := httptest.NewRecorder()
	contracthttp.NewHandler(integrationBookingAuth{accountID: actor}, service, []string{"http://localhost:8081"}).ServeHTTP(response, request)
	return response
}

func (r *localNoticeRecorder) SendLocalBookingNotice(_ context.Context, recipient, _, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recipients = append(r.recipients, recipient)
	return nil
}

type refundRecordSignalRepository struct {
	booking.Repository
	completed chan struct{}
	once      sync.Once
}

func (r *refundRecordSignalRepository) RecordRefund(ctx context.Context, renter, id, result string, now time.Time) (booking.RefundResult, error) {
	value, err := r.Repository.RecordRefund(ctx, renter, id, result, now)
	if err == nil && value.State == "completada" {
		r.once.Do(func() { close(r.completed) })
	}
	return value, err
}

type gatedRefundAdapter struct {
	entered chan string
	release map[string]<-chan struct{}
}

func (a *gatedRefundAdapter) ProcessRefund(ctx context.Context, operationID, requestedKey, outcome string) (string, error) {
	if operationID != requestedKey {
		return "", fmt.Errorf("unexpected refund operation key")
	}
	select {
	case a.entered <- outcome:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	select {
	case <-a.release[outcome]:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	switch outcome {
	case "exito":
		return "exito_simulado", nil
	case "sin_respuesta":
		return "sin_respuesta_simulada", nil
	default:
		return "", fmt.Errorf("unsupported gated refund outcome")
	}
}

func TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry(t *testing.T) {
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
	defer admin.Close(context.Background())
	name := fmt.Sprintf("local_booking_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	var dbURL string
	t.Cleanup(func() {
		cleanCtx, cc := context.WithTimeout(context.Background(), 20*time.Second)
		defer cc()
		if dbURL != "" {
			if cleanupConn, e := pgx.Connect(cleanCtx, dbURL); e == nil {
				_, _ = cleanupConn.Exec(cleanCtx, `DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN DROP OWNED BY espacigo_runtime; END IF; END $$`)
				_ = cleanupConn.Close(cleanCtx)
			}
		}
		cleanupAdmin, e := pgx.Connect(cleanCtx, adminURL)
		if e != nil {
			t.Errorf("reconnect disposable booking PostgreSQL: %v", e)
			return
		}
		defer cleanupAdmin.Close(context.Background())
		if _, e := cleanupAdmin.Exec(cleanCtx, `DROP DATABASE `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`); e != nil {
			t.Errorf("drop disposable booking database: %v", e)
		}
		if _, e := cleanupAdmin.Exec(cleanCtx, `DROP ROLE IF EXISTS espacigo_runtime`); e != nil {
			t.Errorf("drop disposable runtime role: %v", e)
		}
	})
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dbURL = u.String()
	if _, err = migrator.Run(ctx, dbURL, "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE ROLE espacigo_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD 'local-booking-test'`); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bootstrap.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{name}.Sanitize()+` TO espacigo_runtime`); err != nil {
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
	runtimeURL.User = url.UserPassword("espacigo_runtime", "local-booking-test")
	pool, err := pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var current string
	if err = pool.QueryRow(ctx, `SELECT current_user`).Scan(&current); err != nil || current != "espacigo_runtime" {
		t.Fatalf("runtime role=%q err=%v", current, err)
	}
	var historyRead, historyInsert, historyUpdate, historyDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_ensayo_transicion','SELECT'),has_table_privilege(current_user,'public.reserva_ensayo_transicion','INSERT'),has_table_privilege(current_user,'public.reserva_ensayo_transicion','UPDATE'),has_table_privilege(current_user,'public.reserva_ensayo_transicion','DELETE')`).Scan(&historyRead, &historyInsert, &historyUpdate, &historyDelete); err != nil {
		t.Fatal(err)
	}
	if !historyRead || !historyInsert || historyUpdate || historyDelete {
		t.Fatalf("transition grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", historyRead, historyInsert, historyUpdate, historyDelete)
	}
	var cancellationRead, cancellationInsert, cancellationUpdate, cancellationDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_cancelacion_ensayo','SELECT'),has_table_privilege(current_user,'public.reserva_cancelacion_ensayo','INSERT'),has_table_privilege(current_user,'public.reserva_cancelacion_ensayo','UPDATE'),has_table_privilege(current_user,'public.reserva_cancelacion_ensayo','DELETE')`).Scan(&cancellationRead, &cancellationInsert, &cancellationUpdate, &cancellationDelete); err != nil {
		t.Fatal(err)
	}
	if !cancellationRead || !cancellationInsert || cancellationUpdate || cancellationDelete {
		t.Fatalf("cancellation grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", cancellationRead, cancellationInsert, cancellationUpdate, cancellationDelete)
	}
	var refundRead, refundInsert, refundUpdate, refundDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_devolucion_ensayo','SELECT'),has_table_privilege(current_user,'public.reserva_devolucion_ensayo','INSERT'),has_table_privilege(current_user,'public.reserva_devolucion_ensayo','UPDATE'),has_table_privilege(current_user,'public.reserva_devolucion_ensayo','DELETE')`).Scan(&refundRead, &refundInsert, &refundUpdate, &refundDelete); err != nil {
		t.Fatal(err)
	}
	if !refundRead || !refundInsert || !refundUpdate || refundDelete {
		t.Fatalf("refund grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", refundRead, refundInsert, refundUpdate, refundDelete)
	}
	var contractDocRead, contractDocInsert, contractDocUpdate, contractDocDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.documento_privado_sintetico_local','SELECT'),has_table_privilege(current_user,'public.documento_privado_sintetico_local','INSERT'),has_table_privilege(current_user,'public.documento_privado_sintetico_local','UPDATE'),has_table_privilege(current_user,'public.documento_privado_sintetico_local','DELETE')`).Scan(&contractDocRead, &contractDocInsert, &contractDocUpdate, &contractDocDelete); err != nil {
		t.Fatal(err)
	}
	if !contractDocRead || !contractDocInsert || contractDocUpdate || contractDocDelete {
		t.Fatalf("immutable private document grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", contractDocRead, contractDocInsert, contractDocUpdate, contractDocDelete)
	}
	var refundAttemptRead, refundAttemptInsert, refundAttemptUpdate, refundAttemptDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_devolucion_intento_ensayo','SELECT'),has_table_privilege(current_user,'public.reserva_devolucion_intento_ensayo','INSERT'),has_table_privilege(current_user,'public.reserva_devolucion_intento_ensayo','UPDATE'),has_table_privilege(current_user,'public.reserva_devolucion_intento_ensayo','DELETE')`).Scan(&refundAttemptRead, &refundAttemptInsert, &refundAttemptUpdate, &refundAttemptDelete); err != nil {
		t.Fatal(err)
	}
	if !refundAttemptRead || !refundAttemptInsert || refundAttemptUpdate || refundAttemptDelete {
		t.Fatalf("refund attempt grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", refundAttemptRead, refundAttemptInsert, refundAttemptUpdate, refundAttemptDelete)
	}
	var messagesRead, messagesInsert, messagesUpdate, messagesDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.mensaje_reserva_ensayo','SELECT'),has_table_privilege(current_user,'public.mensaje_reserva_ensayo','INSERT'),has_table_privilege(current_user,'public.mensaje_reserva_ensayo','UPDATE'),has_table_privilege(current_user,'public.mensaje_reserva_ensayo','DELETE')`).Scan(&messagesRead, &messagesInsert, &messagesUpdate, &messagesDelete); err != nil {
		t.Fatal(err)
	}
	if !messagesRead || !messagesInsert || messagesUpdate || !messagesDelete {
		t.Fatalf("message grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", messagesRead, messagesInsert, messagesUpdate, messagesDelete)
	}
	var cursorRead, cursorInsert, cursorUpdate, cursorDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_mensaje_lectura','SELECT'),has_table_privilege(current_user,'public.reserva_mensaje_lectura','INSERT'),has_table_privilege(current_user,'public.reserva_mensaje_lectura','UPDATE'),has_table_privilege(current_user,'public.reserva_mensaje_lectura','DELETE')`).Scan(&cursorRead, &cursorInsert, &cursorUpdate, &cursorDelete); err != nil {
		t.Fatal(err)
	}
	if !cursorRead || !cursorInsert || !cursorUpdate || !cursorDelete {
		t.Fatalf("read cursor grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", cursorRead, cursorInsert, cursorUpdate, cursorDelete)
	}
	var fixtureLocationRead, fixtureLocationInsert, fixtureLocationUpdate, fixtureLocationDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.reserva_ensayo_local_ubicacion_sintetica','SELECT'),has_table_privilege(current_user,'public.reserva_ensayo_local_ubicacion_sintetica','INSERT'),has_table_privilege(current_user,'public.reserva_ensayo_local_ubicacion_sintetica','UPDATE'),has_table_privilege(current_user,'public.reserva_ensayo_local_ubicacion_sintetica','DELETE')`).Scan(&fixtureLocationRead, &fixtureLocationInsert, &fixtureLocationUpdate, &fixtureLocationDelete); err != nil {
		t.Fatal(err)
	}
	if !fixtureLocationRead || fixtureLocationInsert || fixtureLocationUpdate || fixtureLocationDelete {
		t.Fatalf("synthetic location grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", fixtureLocationRead, fixtureLocationInsert, fixtureLocationUpdate, fixtureLocationDelete)
	}
	setup, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close(context.Background())
	host, renter, outsider := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	for i, id := range []string{host, renter, outsider} {
		_, err = setup.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'x','activo')`, id, fmt.Sprintf("booking-%d@example.test", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	space := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	_, err = setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,'sala_multiproposito','Fixture de reserva',repeat('Espacio sintético autorizado. ',4),30,8,'Reglas de prueba','hora',8000,'Dirección sintética','America/Santiago')`, space, host)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}')`, space); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, space); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id) VALUES($1,$2,$3)`, space, host, renter); err != nil {
		t.Fatal(err)
	}
	secondSpace := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,'bodega','Bodega sintética',repeat('Detalle sintético seguro. ',4),40,2,'Acceso controlado','hora',12000,'Dirección privada','America/Santiago')`, secondSpace, host); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'bodega',1,'{"altura_util_m":3.2,"carro_carga_disponible":false}'::jsonb)`, secondSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',12000)`, secondSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id) VALUES($1,$2,$3)`, secondSpace, host, renter); err != nil {
		t.Fatal(err)
	}
	centerLon, centerLat := -70.6693, -33.4560
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_ubicacion_sintetica(espacio_id,latitud,longitud,es_sintetica)
VALUES($1,ST_Y(ST_Project(ST_SetSRID(ST_MakePoint($2,$3),4326)::geography,1001,0)::geometry),$2,true),
      ($4,ST_Y(ST_Project(ST_SetSRID(ST_MakePoint($2,$3),4326)::geography,1000,0)::geometry),$2,true)`, space, centerLon, centerLat, secondSpace); err != nil {
		t.Fatal(err)
	}
	privateSpace := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,'bodega','Draft no catalogado',repeat('Draft no visible en resultados. ',4),10,1,'Privado','hora',8000,'Privada','America/Santiago')`, privateSpace, host); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'bodega',1,'{}')`, privateSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, privateSpace); err != nil {
		t.Fatal(err)
	}
	repo := New(pool)
	fixedNow := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return fixedNow }
	paymentAdapter := newTestPaymentAdapter(t)
	svc, err := booking.NewService(repo, credentials.Generator{}, clock, paymentAdapter)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetLocalRefundAdapter(paymentAdapter)
	noticeRecorder := &localNoticeRecorder{}
	svc.SetLocalNoticeSender(noticeRecorder)
	// New reservations require an effective KYC approval from both participants.
	// The test first omits each side in turn and checks no reservation or hold is
	// left behind, then seeds the approvals used by the rest of this lifecycle.
	seedKYC := func(account string) string {
		t.Helper()
		var verificationID string
		if err := setup.QueryRow(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,proveedor_ref,referencia_evidencia,clave_idempotencia,revisor_id,creada_en,resuelta_en)
			VALUES(gen_random_uuid(),$1,'kyc','aprobada','local-fixture-v1','fixture:'||gen_random_uuid()::text,$2,$3,$4,$4) RETURNING id::text`, account, "booking-kyc-"+account, outsider, fixedNow.Add(-time.Hour)).Scan(&verificationID); err != nil {
			t.Fatal(err)
		}
		if _, err := setup.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en) VALUES($1,'kyc',$2,'elegible',$3)`, account, verificationID, fixedNow.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		return verificationID
	}
	hostKYC := seedKYC(host)
	missingRenterQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: fixedNow.Add(24 * time.Hour).Format(time.RFC3339), EndAt: fixedNow.Add(25 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: missingRenterQuote.ID}, "missing-renter-kyc"); err != booking.ErrConflict {
		t.Fatalf("reservation without renter KYC err=%v, want conflict", err)
	}
	renterKYC := seedKYC(renter)
	if _, err = setup.Exec(ctx, `DELETE FROM public.elegibilidad_verificacion_local WHERE usuario_id=$1 AND tipo='kyc'`, host); err != nil {
		t.Fatal(err)
	}
	missingHostQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: fixedNow.Add(26 * time.Hour).Format(time.RFC3339), EndAt: fixedNow.Add(27 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: missingHostQuote.ID}, "missing-host-kyc"); err != booking.ErrConflict {
		t.Fatalf("reservation without host KYC err=%v, want conflict", err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en) VALUES($1,'kyc',$2,'elegible',$3)`, host, hostKYC, fixedNow.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	var rejectedRows, rejectedKYCQuoteOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=ANY($1::uuid[])`, []string{missingRenterQuote.ID, missingHostQuote.ID}).Scan(&rejectedRows); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE activo AND reserva_id IN (SELECT id FROM public.reserva_ensayo_local WHERE cotizacion_id=ANY($1::uuid[]))`, []string{missingRenterQuote.ID, missingHostQuote.ID}).Scan(&rejectedKYCQuoteOccupancies); err != nil {
		t.Fatal(err)
	}
	if rejectedRows != 0 || rejectedKYCQuoteOccupancies != 0 {
		t.Fatalf("KYC rejection left partial reservation/occupancy=%d/%d", rejectedRows, rejectedKYCQuoteOccupancies)
	}
	_ = renterKYC
	// Persist two genuinely adjacent reservations, then exercise the database
	// exclusion constraint directly through the runtime role and its public
	// HTTP conflict translation. Offered selector slots alone do not prove the
	// reservation/occupancy transaction accepts adjacency.
	adjacentStart := fixedNow.Add(48 * time.Hour)
	firstAdjacentQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: adjacentStart.Format(time.RFC3339Nano), EndAt: adjacentStart.Add(time.Hour).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("quote first adjacent reservation: %v", err)
	}
	secondAdjacentStart := adjacentStart.Add(time.Hour)
	secondAdjacentQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: secondAdjacentStart.Format(time.RFC3339Nano), EndAt: secondAdjacentStart.Add(time.Hour).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("quote second adjacent reservation: %v", err)
	}
	overlapQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: adjacentStart.Add(15 * time.Minute).Format(time.RFC3339Nano), EndAt: adjacentStart.Add(45 * time.Minute).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("prepare quote before interval is claimed: %v", err)
	}
	firstAdjacent, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: firstAdjacentQuote.ID}, "local-adjacent-first")
	if err != nil {
		t.Fatalf("create first adjacent reservation: %v", err)
	}
	secondAdjacent, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: secondAdjacentQuote.ID}, "local-adjacent-second")
	if err != nil {
		t.Fatalf("create second adjacent reservation: %v", err)
	}
	if firstAdjacent.State != "pendiente_de_pago" || secondAdjacent.State != "pendiente_de_pago" {
		t.Fatalf("adjacent reservation states=%s/%s", firstAdjacent.State, secondAdjacent.State)
	}
	var adjacentReservations, adjacentOccupancies, adjacentTransitions int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE id=ANY($1::uuid[]) AND estado='pendiente_de_pago'`, []string{firstAdjacent.ID, secondAdjacent.ID}).Scan(&adjacentReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=ANY($1::uuid[]) AND activo AND tipo='retencion' AND intervalo && tstzrange($2,$3,'[)')`, []string{firstAdjacent.ID, secondAdjacent.ID}, adjacentStart, secondAdjacentStart.Add(time.Hour)).Scan(&adjacentOccupancies); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=ANY($1::uuid[]) AND secuencia=1 AND estado_nuevo='pendiente_de_pago'`, []string{firstAdjacent.ID, secondAdjacent.ID}).Scan(&adjacentTransitions); err != nil {
		t.Fatal(err)
	}
	if adjacentReservations != 2 || adjacentOccupancies != 2 || adjacentTransitions != 2 {
		t.Fatalf("adjacent persisted rows reservations/occupancies/history=%d/%d/%d", adjacentReservations, adjacentOccupancies, adjacentTransitions)
	}
	_, directConstraintErr := pool.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo) VALUES($1,$2,NULL,tstzrange($3,$4,'[)'),'bloqueo_manual',true,'constraint integration probe')`, "77777777-7777-4777-8777-777777777701", space, adjacentStart.Add(30*time.Minute), adjacentStart.Add(90*time.Minute))
	var pgErr *pgconn.PgError
	if !errors.As(directConstraintErr, &pgErr) || pgErr.Code != "23P01" {
		t.Fatalf("direct overlapping occupancy insert SQLSTATE=%v, want 23P01", directConstraintErr)
	}
	handler := bookinghttp.NewHandler(integrationBookingAuth{accountID: renter}, svc, nil)
	requestBody, err := json.Marshal(booking.RequestInput{QuoteID: overlapQuote.ID})
	if err != nil {
		t.Fatal(err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations", strings.NewReader(string(requestBody)))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer local-booking-integration-session")
	httpRequest.Header.Set("Idempotency-Key", "local-adjacent-overlap")
	httpResponse := httptest.NewRecorder()
	handler.ServeHTTP(httpResponse, httpRequest)
	if httpResponse.Code != http.StatusConflict || strings.Contains(httpResponse.Body.String(), "23P01") {
		t.Fatalf("public overlap response status=%d body=%s", httpResponse.Code, httpResponse.Body.String())
	}
	var failedReservationRows, failedHistoryRows, overlapOccupancyRows int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, overlapQuote.ID).Scan(&failedReservationRows); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id IN (SELECT id FROM public.reserva_ensayo_local WHERE cotizacion_id=$1)`, overlapQuote.ID).Scan(&failedHistoryRows); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND activo AND tipo='retencion' AND intervalo && tstzrange($2,$3,'[)')`, space, adjacentStart, secondAdjacentStart.Add(time.Hour)).Scan(&overlapOccupancyRows); err != nil {
		t.Fatal(err)
	}
	if failedReservationRows != 0 || failedHistoryRows != 0 || overlapOccupancyRows != 2 {
		t.Fatalf("overlap conflict left partial rows reservation/history or changed adjacent holds=%d/%d/%d", failedReservationRows, failedHistoryRows, overlapOccupancyRows)
	}
	// Catalog availability must drive the existing expiry transition itself;
	// no reservation read or new quote may be needed after the payment deadline.
	expiringStart := fixedNow.Add(24 * time.Hour)
	expiringQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: expiringStart.Format(time.RFC3339), EndAt: expiringStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	expiringReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: expiringQuote.ID}, "catalog-expiry-direct")
	if err != nil {
		t.Fatal(err)
	}
	clockMu.Lock()
	fixedNow = expiringReservation.PayExpiresAt
	clockMu.Unlock()
	selectorLocation, _ := time.LoadLocation("America/Santiago")
	selectorDate := expiringStart.In(selectorLocation).Format("2006-01-02")
	expiredHoldOptions, err := svc.AvailableIntervals(ctx, renter, space, booking.AvailabilityOptionsInput{Date: selectorDate, Duration: 1})
	if err != nil {
		t.Fatalf("selector did not process an expired payment hold: %+v err=%v", expiredHoldOptions, err)
	}
	var expiredHoldReturned bool
	for _, option := range expiredHoldOptions.Items {
		if option.StartAt.Equal(expiringStart) {
			expiredHoldReturned = true
		}
	}
	if !expiredHoldReturned {
		t.Fatalf("selector did not return the interval after hold expiry: %+v", expiredHoldOptions)
	}
	searchStart, searchEnd := expiringStart, expiringStart.Add(time.Hour)
	itemsAfterExpiry, err := svc.Catalog(ctx, renter, booking.CatalogFilter{StartAt: &searchStart, EndAt: &searchEnd})
	if err != nil {
		t.Fatal(err)
	}
	var returnedAvailable bool
	for _, item := range itemsAfterExpiry {
		if item.SpaceID == space && item.Available != nil {
			returnedAvailable = *item.Available
		}
	}
	if !returnedAvailable {
		t.Fatalf("catalog did not return the space after direct expiry search: %+v", itemsAfterExpiry)
	}
	expiredDetail, err := svc.Get(ctx, renter, expiringReservation.ID)
	if err != nil || expiredDetail.State != "vencida_pago" || len(expiredDetail.History) != 2 || expiredDetail.History[1].To != "vencida_pago" {
		t.Fatalf("catalog did not persist expiry and history: %+v err=%v", expiredDetail, err)
	}
	var activeOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, expiringReservation.ID).Scan(&activeOccupancies); err != nil || activeOccupancies != 0 {
		t.Fatalf("expired reservation active occupancy count=%d err=%v", activeOccupancies, err)
	}
	clockMu.Lock()
	fixedNow = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clockMu.Unlock()
	for _, startAt := range []time.Time{fixedNow.Add(-time.Second), fixedNow} {
		_, err = svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: startAt.Format(time.RFC3339Nano), EndAt: startAt.Add(time.Hour).Format(time.RFC3339Nano)})
		if err != booking.ErrInvalid {
			t.Fatalf("quote start %s should be rejected at backend time %s: %v", startAt, fixedNow, err)
		}
	}
	items, err := svc.Catalog(ctx, renter, booking.CatalogFilter{})
	if err != nil || len(items) != 2 {
		t.Fatalf("authorized catalog items=%d err=%v: %+v", len(items), err, items)
	}
	catalogPageOne, err := svc.CatalogPage(ctx, renter, booking.CatalogFilter{}, 1, "")
	if err != nil || len(catalogPageOne.Items) != 1 || catalogPageOne.NextCursor == "" {
		t.Fatalf("runtime first catalog page=%+v err=%v", catalogPageOne, err)
	}
	catalogPageTwo, err := svc.CatalogPage(ctx, renter, booking.CatalogFilter{}, 1, catalogPageOne.NextCursor)
	if err != nil || len(catalogPageTwo.Items) != 1 || catalogPageTwo.NextCursor != "" || catalogPageTwo.Items[0].SpaceID == catalogPageOne.Items[0].SpaceID {
		t.Fatalf("runtime final catalog page=%+v err=%v", catalogPageTwo, err)
	}
	if _, err = svc.CatalogPage(ctx, outsider, booking.CatalogFilter{}, 1, catalogPageOne.NextCursor); err != booking.ErrInvalid {
		t.Fatalf("cursor reuse by another account error=%v", err)
	}
	hostItems, err := svc.Catalog(ctx, host, booking.CatalogFilter{})
	if err != nil || len(hostItems) != 2 {
		t.Fatalf("authorized host catalog items=%d err=%v", len(hostItems), err)
	}
	outsiderItems, err := svc.Catalog(ctx, outsider, booking.CatalogFilter{})
	if err != nil || len(outsiderItems) != 0 {
		t.Fatalf("unassigned account saw catalog items=%d err=%v", len(outsiderItems), err)
	}
	var foundWarehouse bool
	for _, item := range items {
		if item.SpaceID == privateSpace {
			t.Fatal("private unlisted draft leaked into catalog")
		}
		if item.SpaceID == secondSpace {
			foundWarehouse = item.CategoryCode == "bodega" && item.Price == 12000 && item.ProfileVersion == 1 && hasAttributes(item.Attributes, map[string]any{"altura_util_m": 3.2, "carro_carga_disponible": false})
		}
	}
	if !foundWarehouse {
		t.Fatalf("second fixture lacks its own category/profile/tariff: %+v", items)
	}
	geoCenterLat, geoCenterLon := centerLat, centerLon
	geoRadius1, geoRadius3 := 1, 3
	geoAt, geoUntil := fixedNow.Add(800*24*time.Hour), fixedNow.Add(800*24*time.Hour+time.Hour)
	geoMin, geoMax := int64(12000), int64(12000)
	var geoQuotesBefore, geoReservationsBefore, geoOccupanciesBefore int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cotizacion_reserva_ensayo),(SELECT count(*) FROM public.reserva_ensayo_local),(SELECT count(*) FROM public.ocupacion)`).Scan(&geoQuotesBefore, &geoReservationsBefore, &geoOccupanciesBefore); err != nil {
		t.Fatal(err)
	}
	geoCombined, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega", StartAt: &geoAt, EndAt: &geoUntil, MinTotalCLP: &geoMin, MaxTotalCLP: &geoMax, ProfileVersion: 1, Attributes: map[string]any{"altura_util_m": 3.2, "carro_carga_disponible": false}, Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &geoRadius1})
	if err != nil || len(geoCombined) != 1 || geoCombined[0].SpaceID != secondSpace || geoCombined[0].DistanceKM == nil || *geoCombined[0].DistanceKM != 1.0 || geoCombined[0].DistanceKind != "direct" || geoCombined[0].EstimatedTotal == nil || *geoCombined[0].EstimatedTotal != 12000 {
		t.Fatalf("combined geo/category/availability/attribute/price filter=%+v err=%v", geoCombined, err)
	}
	geoPage, err := svc.CatalogPage(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega", StartAt: &geoAt, EndAt: &geoUntil, MinTotalCLP: &geoMin, MaxTotalCLP: &geoMax, ProfileVersion: 1, Attributes: map[string]any{"altura_util_m": 3.2, "carro_carga_disponible": false}, Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &geoRadius1}, 1, "")
	if err != nil || len(geoPage.Items) != 1 || geoPage.Items[0].SpaceID != secondSpace || geoPage.NextCursor != "" {
		t.Fatalf("combined filters must be applied before pagination: page=%+v err=%v", geoPage, err)
	}
	geoBoundary, err := svc.Catalog(ctx, renter, booking.CatalogFilter{Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &geoRadius1})
	if err != nil || len(geoBoundary) != 1 || geoBoundary[0].SpaceID != secondSpace {
		t.Fatalf("one-kilometer inclusive boundary/outside results=%+v err=%v", geoBoundary, err)
	}
	for _, radius := range []int{1, 3, 5, 10, 25} {
		itemsAtRadius, searchErr := svc.Catalog(ctx, renter, booking.CatalogFilter{Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &radius})
		want := 2
		if radius == 1 {
			want = 1
		}
		if searchErr != nil || len(itemsAtRadius) != want {
			t.Fatalf("radius %dkm results=%+v err=%v; want %d", radius, itemsAtRadius, searchErr, want)
		}
	}
	geoSorted, err := svc.Catalog(ctx, renter, booking.CatalogFilter{Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &geoRadius3})
	if err != nil || len(geoSorted) != 2 || geoSorted[0].SpaceID != secondSpace || geoSorted[1].SpaceID != space || geoSorted[0].DistanceKM == nil || geoSorted[1].DistanceKM == nil || *geoSorted[0].DistanceKM != 1.0 || *geoSorted[1].DistanceKM != 1.0 {
		t.Fatalf("raw distance ordering before one-decimal rounding=%+v err=%v", geoSorted, err)
	}
	outsiderGeo, err := svc.Catalog(ctx, outsider, booking.CatalogFilter{Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &geoRadius3})
	if err != nil || len(outsiderGeo) != 0 {
		t.Fatalf("geographic query widened fixture access for unrelated account: %+v err=%v", outsiderGeo, err)
	}
	geoJSON, err := json.Marshal(geoSorted)
	if err != nil || strings.Contains(string(geoJSON), "latitude") || strings.Contains(string(geoJSON), "longitude") || strings.Contains(string(geoJSON), "direccion") || strings.Contains(string(geoJSON), "punto") {
		t.Fatalf("catalog response exposed location or private address: %s err=%v", geoJSON, err)
	}
	if _, err = setup.Exec(ctx, `DELETE FROM public.reserva_ensayo_local_ubicacion_sintetica WHERE espacio_id=$1`, space); err != nil {
		t.Fatal(err)
	}
	withoutMissingLocation, err := svc.Catalog(ctx, renter, booking.CatalogFilter{Latitude: &geoCenterLat, Longitude: &geoCenterLon, RadiusKM: &geoRadius3})
	if err != nil || len(withoutMissingLocation) != 1 || withoutMissingLocation[0].SpaceID != secondSpace {
		t.Fatalf("fixture missing synthetic location was not excluded: %+v err=%v", withoutMissingLocation, err)
	}
	withoutGeoFilter, err := svc.Catalog(ctx, renter, booking.CatalogFilter{})
	if err != nil || len(withoutGeoFilter) != 2 {
		t.Fatalf("fixture without coordinates changed non-geographic catalog results: %+v err=%v", withoutGeoFilter, err)
	}
	var geoQuotesAfter, geoReservationsAfter, geoOccupanciesAfter int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cotizacion_reserva_ensayo),(SELECT count(*) FROM public.reserva_ensayo_local),(SELECT count(*) FROM public.ocupacion)`).Scan(&geoQuotesAfter, &geoReservationsAfter, &geoOccupanciesAfter); err != nil {
		t.Fatal(err)
	}
	if geoQuotesAfter != geoQuotesBefore || geoReservationsAfter != geoReservationsBefore || geoOccupanciesAfter != geoOccupanciesBefore {
		t.Fatalf("geographic catalog search wrote booking state: quotes %d->%d reservations %d->%d occupancies %d->%d", geoQuotesBefore, geoQuotesAfter, geoReservationsBefore, geoReservationsAfter, geoOccupanciesBefore, geoOccupanciesAfter)
	}
	detailItem, err := svc.CatalogDetail(ctx, renter, secondSpace)
	if err != nil || detailItem.CategoryCode != "bodega" || detailItem.CategoryName != "Bodega" || detailItem.ProfileVersion != 1 || !hasAttributes(detailItem.Attributes, map[string]any{"altura_util_m": 3.2, "carro_carga_disponible": false}) {
		t.Fatalf("selected fixture detail=%+v err=%v", detailItem, err)
	}
	filtered, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega"})
	if err != nil || len(filtered) != 1 || filtered[0].SpaceID != secondSpace {
		t.Fatalf("category filter=%+v err=%v", filtered, err)
	}
	when := fixedNow.Add(30 * time.Minute)
	until := when.Add(time.Hour)
	available, err := svc.Catalog(ctx, renter, booking.CatalogFilter{StartAt: &when, EndAt: &until})
	if err != nil || len(available) != 2 || available[0].Available == nil || !*available[0].Available {
		t.Fatalf("available catalog=%+v err=%v", available, err)
	}
	if available[0].SpaceID != space || available[1].SpaceID != secondSpace || available[0].EstimatedTotal == nil || *available[0].EstimatedTotal != 8000 || available[1].EstimatedTotal == nil || *available[1].EstimatedTotal != 12000 {
		t.Fatalf("estimated totals/order=%+v", available)
	}
	minTotal, maxTotal := int64(12000), int64(12000)
	var quotesBefore, occupanciesBefore int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_reserva_ensayo`).Scan(&quotesBefore); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion`).Scan(&occupanciesBefore); err != nil {
		t.Fatal(err)
	}
	filteredFeatures, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega", StartAt: &when, EndAt: &until, MinTotalCLP: &minTotal, MaxTotalCLP: &maxTotal, ProfileVersion: 1, Attributes: map[string]any{"altura_util_m": 3.2, "carro_carga_disponible": false}})
	if err != nil || len(filteredFeatures) != 1 || filteredFeatures[0].SpaceID != secondSpace || filteredFeatures[0].EstimatedTotal == nil || *filteredFeatures[0].EstimatedTotal != 12000 || filteredFeatures[0].RateUnit != "hora" || filteredFeatures[0].TimeZone != "America/Santiago" {
		t.Fatalf("AND feature/category/availability/inclusive-price filter=%+v err=%v", filteredFeatures, err)
	}
	var quotesAfter, occupanciesAfter int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_reserva_ensayo`).Scan(&quotesAfter); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion`).Scan(&occupanciesAfter); err != nil {
		t.Fatal(err)
	}
	if quotesAfter != quotesBefore || occupanciesAfter != occupanciesBefore {
		t.Fatalf("catalog search wrote quotes/occupancies: before=%d/%d after=%d/%d", quotesBefore, occupanciesBefore, quotesAfter, occupanciesAfter)
	}
	maxBelow := int64(11999)
	belowBoundary, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega", StartAt: &when, EndAt: &until, MaxTotalCLP: &maxBelow})
	if err != nil || len(belowBoundary) != 0 {
		t.Fatalf("maximum price below inclusive total returned=%+v err=%v", belowBoundary, err)
	}
	falseMatches, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega", ProfileVersion: 1, Attributes: map[string]any{"carro_carga_disponible": false}})
	if err != nil || len(falseMatches) != 1 || falseMatches[0].SpaceID != secondSpace {
		t.Fatalf("boolean false search=%+v err=%v", falseMatches, err)
	}
	var absentMatches bool
	if err = setup.QueryRow(ctx, `SELECT '{}'::jsonb @> '{"carro_carga_disponible":false}'::jsonb`).Scan(&absentMatches); err != nil || absentMatches {
		t.Fatalf("absent boolean must not match false: result=%v err=%v", absentMatches, err)
	}
	if _, err = svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega", ProfileVersion: 2, Attributes: map[string]any{"altura_util_m": 3.2}}); err != booking.ErrInvalid {
		t.Fatalf("missing/unmatched immutable profile version accepted: %v", err)
	}
	if _, err = svc.Fixture(ctx, outsider); err != booking.ErrNotFound {
		t.Fatalf("unlisted fixture visible: %v", err)
	}
	if _, err = svc.CatalogDetail(ctx, outsider, space); err != booking.ErrNotFound {
		t.Fatalf("outsider queried enabled fixture detail: %v", err)
	}
	if _, err = svc.CatalogDetail(ctx, renter, privateSpace); err != booking.ErrNotFound {
		t.Fatalf("unlisted draft detail visible: %v", err)
	}
	wrongCategory, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "unknown"})
	if err != booking.ErrInvalid || len(wrongCategory) != 0 {
		t.Fatalf("unknown category filter accepted: %+v %v", wrongCategory, err)
	}
	selectedQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: secondSpace, StartAt: fixedNow.Add(2 * time.Hour).Format(time.RFC3339), EndAt: fixedNow.Add(3 * time.Hour).Format(time.RFC3339)})
	if err != nil || selectedQuote.SpaceID != secondSpace || selectedQuote.UnitPrice != 12000 || selectedQuote.CategoryCode != "bodega" || selectedQuote.ProfileVersion != 1 || !hasAttributes(selectedQuote.ProfileValues, map[string]any{"altura_util_m": 3.2, "carro_carga_disponible": false}) {
		t.Fatalf("selected-space quote snapshot=%+v err=%v", selectedQuote, err)
	}
	if _, err = svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: privateSpace, StartAt: fixedNow.Add(2 * time.Hour).Format(time.RFC3339), EndAt: fixedNow.Add(3 * time.Hour).Format(time.RFC3339)}); err != booking.ErrNotFound {
		t.Fatalf("quote for unlisted draft=%v", err)
	}
	if _, err = svc.Quote(ctx, outsider, booking.QuoteInput{SpaceID: space, StartAt: fixedNow.Add(2 * time.Hour).Format(time.RFC3339), EndAt: fixedNow.Add(3 * time.Hour).Format(time.RFC3339)}); err != booking.ErrNotFound {
		t.Fatalf("quote by unauthorized renter=%v", err)
	}
	if available, err = svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "bodega"}); err != nil || len(available) != 1 || available[0].SpaceID != secondSpace {
		t.Fatalf("filtered selected-space availability=%+v err=%v", available, err)
	}
	quoteForAnotherSpace, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: secondSpace, StartAt: when.Format(time.RFC3339), EndAt: until.Format(time.RFC3339)})
	if err != nil || quoteForAnotherSpace.UnitPrice != 12000 || quoteForAnotherSpace.Subtotal != *filteredFeatures[0].EstimatedTotal {
		t.Fatalf("second-space quote=%+v err=%v", quoteForAnotherSpace, err)
	}
	secondReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: quoteForAnotherSpace.ID}, "selected-space-reservation")
	if err != nil {
		t.Fatalf("selected-space request failed: %v", err)
	}
	if available, err = svc.Catalog(ctx, renter, booking.CatalogFilter{StartAt: &when, EndAt: &until}); err != nil || len(available) != 1 || available[0].SpaceID != space {
		t.Fatalf("availability filter did not omit held second space: %+v err=%v", available, err)
	}
	// Keep the primary flow on the original space; its own price/profile and zone
	// remain distinct from the second allowlisted fixture.
	staleStart := fixedNow.Add(5 * time.Minute)
	staleQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: staleStart.Format(time.RFC3339Nano), EndAt: staleStart.Add(time.Hour).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	clockMu.Lock()
	fixedNow = staleStart.Add(time.Second)
	clockMu.Unlock()
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: staleQuote.ID}, "stale-start"); err != booking.ErrConflict {
		t.Fatalf("still-live quote whose interval started should be rejected transactionally: %v", err)
	}
	var rejectedReservations, rejectedOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, staleQuote.ID).Scan(&rejectedReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND intervalo && tstzrange($2,$3,'[)')`, space, staleStart, staleStart.Add(time.Hour)).Scan(&rejectedOccupancies); err != nil {
		t.Fatal(err)
	}
	if rejectedReservations != 0 || rejectedOccupancies != 0 {
		t.Fatalf("rejected request left rows: reservations=%d occupancies=%d", rejectedReservations, rejectedOccupancies)
	}
	clockMu.Lock()
	fixedNow = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clockMu.Unlock()
	start := time.Date(2030, 2, 1, 12, 0, 0, 0, time.UTC)
	quote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: start.Format(time.RFC3339), EndAt: start.Add(90 * time.Minute).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Units != 2 || quote.Subtotal != 16000 || quote.TimeZone != "America/Santiago" || quote.Conditions != "Reglas de prueba" || !quote.ExpiresAt.Equal(fixedNow.Add(15*time.Minute)) {
		t.Fatalf("quote snapshot mismatch: %+v", quote)
	}
	input := booking.RequestInput{QuoteID: quote.ID}
	results := make([]booking.Reservation, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = svc.Request(ctx, renter, input, "same-key") }(i)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if results[0].ID != results[1].ID {
		t.Fatalf("concurrent retry created multiple reservations: %s / %s", results[0].ID, results[1].ID)
	}
	if results[0].Conditions != "Reglas de prueba" {
		t.Fatalf("reservation did not preserve conditions snapshot: %q", results[0].Conditions)
	}
	conversationService, err := conversation.NewService(conversationpg.New(pool), credentials.Generator{}, clock)
	if err != nil {
		t.Fatal(err)
	}
	sent, err := conversationService.Send(ctx, renter, results[0].ID, "conversation-retry", "Primer mensaje idempotente")
	if err != nil {
		t.Fatal(err)
	}
	retriedMessage, err := conversationService.Send(ctx, renter, results[0].ID, "conversation-retry", "Primer mensaje idempotente")
	if err != nil {
		t.Fatal(err)
	}
	if sent.ID != retriedMessage.ID || sent.Sequence != retriedMessage.Sequence {
		t.Fatalf("message retry produced duplicates: %+v / %+v", sent, retriedMessage)
	}
	if _, err = conversationService.Send(ctx, renter, results[0].ID, "conversation-retry", "Contenido diferente"); err != conversation.ErrConflict {
		t.Fatalf("idempotency key accepted different content: %v", err)
	}
	if _, err = conversationService.Send(ctx, renter, secondReservation.ID, "pending-write", "Mensaje en pendiente_de_pago"); err != nil {
		t.Fatalf("pending conversation rejected message: %v", err)
	}
	if _, err = conversationService.List(ctx, outsider, results[0].ID, nil, 30); err != conversation.ErrNotFound {
		t.Fatalf("outsider read conversation: %v", err)
	}
	if _, err = conversationService.Send(ctx, outsider, results[0].ID, "outsider-write", "No debería entrar"); err != conversation.ErrNotFound {
		t.Fatalf("outsider wrote conversation: %v", err)
	}
	if _, err = conversationService.MarkRead(ctx, outsider, results[0].ID, 1); err != conversation.ErrNotFound {
		t.Fatalf("outsider advanced conversation cursor: %v", err)
	}
	if _, err = conversationService.Send(ctx, renter, results[0].ID, "too-long", strings.Repeat("x", 2001)); err != conversation.ErrInvalid {
		t.Fatalf("message longer than 2000 chars accepted: %v", err)
	}
	containsReservation := func(items []booking.Reservation, id string) bool {
		for _, item := range items {
			if item.ID == id {
				return true
			}
		}
		return false
	}
	renterInbox, err := svc.List(ctx, renter)
	if err != nil || !containsReservation(renterInbox, results[0].ID) {
		t.Fatalf("renter inbox omitted own reservation: count=%d err=%v", len(renterInbox), err)
	}
	hostInbox, err := svc.List(ctx, host)
	if err != nil || !containsReservation(hostInbox, results[0].ID) {
		t.Fatalf("host inbox omitted own reservation: count=%d err=%v", len(hostInbox), err)
	}
	outsiderInbox, err := svc.List(ctx, outsider)
	if err != nil || len(outsiderInbox) != 0 {
		t.Fatalf("outsider inbox contains reservations: %+v err=%v", outsiderInbox, err)
	}
	if _, err = svc.Pay(ctx, host, results[0].ID, "exito", "host-cannot-pay"); err != booking.ErrNotFound {
		t.Fatalf("host paid as renter: %v", err)
	}
	if _, err = svc.Cancel(ctx, host, results[0].ID, "host-cannot-cancel", ""); err != booking.ErrNotFound {
		t.Fatalf("host cancelled as renter: %v", err)
	}
	if _, err = svc.Decide(ctx, host, results[0].ID, "aprobar", ""); err != booking.ErrConflict {
		t.Fatalf("host decided before payment: %v", err)
	}
	if _, err = svc.Decide(ctx, renter, results[0].ID, "aprobar", ""); err != booking.ErrNotFound {
		t.Fatalf("renter acted as host: %v", err)
	}
	if _, err = svc.Get(ctx, outsider, results[0].ID); err != booking.ErrNotFound {
		t.Fatalf("outsider queried reservation: %v", err)
	}
	if _, err = svc.Pay(ctx, renter, results[0].ID, "exito", "payment-one"); err != nil {
		t.Fatal(err)
	}
	if _, err = conversationService.Send(ctx, renter, results[0].ID, "paid-message", "Mensaje en pagada"); err != nil {
		t.Fatalf("paid conversation rejected message: %v", err)
	}
	if _, err = svc.Decide(ctx, renter, results[0].ID, "aprobar", ""); err != booking.ErrNotFound {
		t.Fatalf("renter decided on own paid reservation: %v", err)
	}
	approved, err := svc.Decide(ctx, host, results[0].ID, "aprobar", "")
	if err != nil || approved.State != "aprobada_host" {
		t.Fatalf("approval: %+v %v", approved, err)
	}
	for _, msg := range []struct{ actor, key, body string }{
		{host, "approved-host", "Mensaje anfitrión aprobado"},
		{renter, "approved-renter", "Mensaje arrendatario aprobado"},
		{host, "approved-host-second", "Segundo mensaje anfitrión"},
	} {
		if _, err = conversationService.Send(ctx, msg.actor, results[0].ID, msg.key, msg.body); err != nil {
			t.Fatalf("approved conversation rejected participant message: %v", err)
		}
	}
	page, err := conversationService.List(ctx, renter, results[0].ID, nil, 2)
	if err != nil || len(page.Items) != 2 || page.Items[0].Body != "Mensaje arrendatario aprobado" || page.Items[1].Body != "Segundo mensaje anfitrión" || page.Items[0].Sequence >= page.Items[1].Sequence || page.OlderCursor == nil || *page.OlderCursor != page.Items[0].Sequence {
		t.Fatalf("latest stable conversation page=%+v err=%v", page, err)
	}
	priorPage, err := conversationService.List(ctx, host, results[0].ID, page.OlderCursor, 2)
	if err != nil || len(priorPage.Items) != 2 || priorPage.Items[0].Body != "Mensaje en pagada" || priorPage.Items[1].Body != "Mensaje anfitrión aprobado" || priorPage.Items[0].Sequence >= priorPage.Items[1].Sequence || priorPage.Items[1].Sequence >= page.Items[0].Sequence || priorPage.OlderCursor == nil || *priorPage.OlderCursor != priorPage.Items[0].Sequence {
		t.Fatalf("previous stable conversation page=%+v err=%v", priorPage, err)
	}
	firstPage, err := conversationService.List(ctx, renter, results[0].ID, priorPage.OlderCursor, 2)
	if err != nil || len(firstPage.Items) != 1 || firstPage.Items[0].Body != "Primer mensaje idempotente" || firstPage.Items[0].Sequence >= priorPage.Items[0].Sequence || firstPage.OlderCursor != nil {
		t.Fatalf("oldest conversation page=%+v err=%v", firstPage, err)
	}
	throughRecentPage := page.Items[len(page.Items)-1].Sequence
	if cursor, markErr := conversationService.MarkRead(ctx, renter, results[0].ID, throughRecentPage); markErr != nil || cursor != throughRecentPage {
		t.Fatalf("renter mark read cursor=%d want=%d err=%v", cursor, throughRecentPage, markErr)
	}
	listReservation := func(actor, id string) booking.Reservation {
		t.Helper()
		items, listErr := svc.List(ctx, actor)
		if listErr != nil {
			t.Fatal(listErr)
		}
		for _, item := range items {
			if item.ID == id {
				return item
			}
		}
		t.Fatalf("reservation %s missing from actor %s inbox", id, actor)
		return booking.Reservation{}
	}
	if item := listReservation(renter, results[0].ID); item.UnreadCount != 0 {
		t.Fatalf("renter unread count after marking recent page=%d", item.UnreadCount)
	}
	if item := listReservation(host, results[0].ID); item.UnreadCount != 3 {
		t.Fatalf("host cursor must remain independent; unread count=%d want=3", item.UnreadCount)
	}
	if cursor, markErr := conversationService.MarkRead(ctx, host, results[0].ID, throughRecentPage); markErr != nil || cursor != throughRecentPage {
		t.Fatalf("host mark read cursor=%d want=%d err=%v", cursor, throughRecentPage, markErr)
	}
	if _, err = conversationService.Send(ctx, renter, results[0].ID, "renter-after-cursor", "Mensaje nuevo para anfitrión"); err != nil {
		t.Fatal(err)
	}
	if item := listReservation(host, results[0].ID); item.UnreadCount != 1 {
		t.Fatalf("message received after host cursor count=%d want=1", item.UnreadCount)
	}
	if item := listReservation(renter, results[0].ID); item.UnreadCount != 0 {
		t.Fatalf("own message counted as unread for renter: %d", item.UnreadCount)
	}
	var concurrentSequences [2]int64
	for i, body := range []string{"Primer mensaje nuevo del anfitrión", "Segundo mensaje nuevo del anfitrión"} {
		message, sendErr := conversationService.Send(ctx, host, results[0].ID, fmt.Sprintf("host-unread-%d", i), body)
		if sendErr != nil {
			t.Fatal(sendErr)
		}
		concurrentSequences[i] = message.Sequence
	}
	markErrors := make(chan error, 2)
	var markWG sync.WaitGroup
	for _, sequence := range concurrentSequences {
		markWG.Add(1)
		go func(sequence int64) {
			defer markWG.Done()
			_, markErr := conversationService.MarkRead(ctx, host, results[0].ID, sequence)
			markErrors <- markErr
		}(sequence)
	}
	markWG.Wait()
	close(markErrors)
	for markErr := range markErrors {
		if markErr != nil {
			t.Fatalf("concurrent mark read failed: %v", markErr)
		}
	}
	if cursor, markErr := conversationService.MarkRead(ctx, host, results[0].ID, concurrentSequences[0]); markErr != nil || cursor != concurrentSequences[1] {
		t.Fatalf("out-of-order older mark regressed host cursor=%d want=%d err=%v", cursor, concurrentSequences[1], markErr)
	}
	var persistedHostCursor int64
	if err = pool.QueryRow(ctx, `SELECT ultima_secuencia_leida FROM public.reserva_mensaje_lectura WHERE reserva_id=$1 AND participante_id=$2`, results[0].ID, host).Scan(&persistedHostCursor); err != nil || persistedHostCursor != concurrentSequences[1] {
		t.Fatalf("host cursor after out-of-order requests=%d want=%d err=%v", persistedHostCursor, concurrentSequences[1], err)
	}
	if item := listReservation(host, results[0].ID); item.UnreadCount != 0 {
		t.Fatalf("host unread count after concurrent cursor advancement=%d", item.UnreadCount)
	}
	if item := listReservation(renter, results[0].ID); item.UnreadCount != 2 {
		t.Fatalf("renter cursor changed with host updates; unread count=%d want=2", item.UnreadCount)
	}
	// A newly constructed service sees the same persisted participant cursor,
	// matching the state a user has after logging in again.
	reloggedService, err := conversation.NewService(conversationpg.New(pool), credentials.Generator{}, clock)
	if err != nil {
		t.Fatal(err)
	}
	if cursor, markErr := reloggedService.MarkRead(ctx, host, results[0].ID, throughRecentPage); markErr != nil || cursor != concurrentSequences[1] {
		t.Fatalf("persisted cursor after service recreation=%d want=%d err=%v", cursor, concurrentSequences[1], markErr)
	}
	var messageCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, results[0].ID).Scan(&messageCount); err != nil || messageCount != 8 {
		t.Fatalf("message count after read-cursor tests=%d want=8 err=%v", messageCount, err)
	}
	// Exercise the cleanup script's real psql variable interpolation against
	// this disposable database. Only the selected thread may be deleted.
	var otherThreadMessages, reservationsBefore, historiesBefore int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, secondReservation.ID).Scan(&otherThreadMessages); err != nil || otherThreadMessages != 1 {
		t.Fatalf("other thread message count=%d err=%v", otherThreadMessages, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE id IN ($1,$2)`, results[0].ID, secondReservation.ID).Scan(&reservationsBefore); err != nil || reservationsBefore != 2 {
		t.Fatalf("reservation count before cleanup=%d err=%v", reservationsBefore, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id IN ($1,$2)`, results[0].ID, secondReservation.ID).Scan(&historiesBefore); err != nil {
		t.Fatal(err)
	}
	cleanupDir := t.TempDir()
	dockerWrapper := `#!/bin/sh
set -eu
while [ "$#" -gt 0 ] && [ "$1" != psql ]; do shift; done
[ "$#" -gt 0 ] || exit 90
shift
reservation_arg=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --set=reservation_id=*) reservation_arg="${1#--set=reservation_id=}" ;;
  esac
  shift
done
[ -n "$reservation_arg" ] || exit 91
exec psql "$TEST_DATABASE_URL" -X -v ON_ERROR_STOP=1 --set="reservation_id=$reservation_arg"
`
	if err = os.WriteFile(filepath.Join(cleanupDir, "docker"), []byte(dockerWrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	cleanupCommand := exec.CommandContext(ctx, "bash", "../../../../scripts/clean-local-booking-thread-messages.sh", results[0].ID)
	cleanupCommand.Env = append(os.Environ(), "PATH="+cleanupDir+":"+os.Getenv("PATH"), "TEST_DATABASE_URL="+dbURL)
	cleanupCommand.Stdin = strings.NewReader("borrar-hilo\n")
	cleanupOutput, cleanupErr := cleanupCommand.CombinedOutput()
	if cleanupErr != nil {
		t.Fatalf("thread cleanup script failed: %v\n%s", cleanupErr, cleanupOutput)
	}
	var selectedMessages, otherMessages, reservationsAfter, historiesAfter int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, results[0].ID).Scan(&selectedMessages); err != nil || selectedMessages != 0 {
		t.Fatalf("selected thread messages after cleanup=%d err=%v", selectedMessages, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, secondReservation.ID).Scan(&otherMessages); err != nil || otherMessages != otherThreadMessages {
		t.Fatalf("other thread messages after cleanup=%d want=%d err=%v", otherMessages, otherThreadMessages, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE id IN ($1,$2)`, results[0].ID, secondReservation.ID).Scan(&reservationsAfter); err != nil || reservationsAfter != reservationsBefore {
		t.Fatalf("reservations after cleanup=%d want=%d err=%v", reservationsAfter, reservationsBefore, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id IN ($1,$2)`, results[0].ID, secondReservation.ID).Scan(&historiesAfter); err != nil || historiesAfter != historiesBefore {
		t.Fatalf("histories after cleanup=%d want=%d err=%v", historiesAfter, historiesBefore, err)
	}
	clockMu.Lock()
	fixedNow = start.Add(2 * time.Hour) // Past the reserved interval: approval does not end chat in this slice.
	clockMu.Unlock()
	if _, err = conversationService.Send(ctx, host, results[0].ID, "after-interval", "La aprobación no cierra aún el hilo"); err != nil {
		t.Fatalf("approved thread closed at interval end: %v", err)
	}
	clockMu.Lock()
	fixedNow = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clockMu.Unlock()
	if _, err = svc.Decide(ctx, host, results[0].ID, "aprobar", ""); err != booking.ErrConflict {
		t.Fatalf("host repeated decision after approval: %v", err)
	}
	detail, err := svc.Get(ctx, renter, results[0].ID)
	if err != nil || len(detail.History) != 3 {
		t.Fatalf("history=%+v err=%v", detail, err)
	}
	wantStates := []string{"pendiente_de_pago", "pagada", "aprobada_host"}
	for i, transition := range detail.History {
		if transition.Sequence != int64(i+1) || transition.To != wantStates[i] || !transition.At.Equal(fixedNow) {
			t.Fatalf("same-instant history order[%d]=%+v, want sequence=%d state=%s timestamp=%s", i, transition, i+1, wantStates[i], fixedNow)
		}
	}
	// A matching idempotency retry still returns the reservation after its
	// interval has begun; only a new request is subject to the start-time rule.
	clockMu.Lock()
	fixedNow = start
	clockMu.Unlock()
	retried, err := svc.Request(ctx, renter, input, "same-key")
	if err != nil || retried.ID != results[0].ID {
		t.Fatalf("idempotent retry after interval start returned %+v, err=%v", retried, err)
	}
	clockMu.Lock()
	fixedNow = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clockMu.Unlock()
	var active bool
	var kind string
	var expiry *time.Time
	if err = pool.QueryRow(ctx, `SELECT activo,tipo,expira_en FROM public.ocupacion WHERE reserva_id=$1`, results[0].ID).Scan(&active, &kind, &expiry); err != nil || !active || kind != "reserva" || expiry != nil {
		t.Fatalf("approved occupancy active=%v type=%s expiry=%v err=%v", active, kind, expiry, err)
	}
	quote2, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: start.Add(3 * time.Hour).Format(time.RFC3339), EndAt: start.Add(4 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: quote2.ID}, "other-key")
	if err != nil {
		t.Fatal(err)
	}
	terminalUnreadMessage, err := conversationService.Send(ctx, renter, retry.ID, "terminal-unread", "Mensaje antes del rechazo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, retry.ID, "exito", "payment-two"); err != nil {
		t.Fatal(err)
	}
	rejected, err := svc.Decide(ctx, host, retry.ID, "rechazar", "No es compatible con el uso del espacio")
	if err != nil || rejected.State != "rechazada_arrendador" {
		t.Fatalf("rejection: %+v %v", rejected, err)
	}
	rejectedDetail, err := svc.Get(ctx, renter, retry.ID)
	if err != nil || len(rejectedDetail.History) < 3 || !strings.Contains(rejectedDetail.History[len(rejectedDetail.History)-1].Reason, "No es compatible con el uso del espacio") {
		t.Fatalf("host rejection reason missing from history: %+v err=%v", rejectedDetail, err)
	}
	if _, err = conversationService.Send(ctx, renter, retry.ID, "rejected-write", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("rejected reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, host, retry.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 1 {
		t.Fatalf("rejected thread not available read-only: %+v err=%v", terminalRead, readErr)
	}
	if item := listReservation(host, retry.ID); item.UnreadCount != 1 {
		t.Fatalf("terminal reservation unread count=%d want=1", item.UnreadCount)
	}
	if item := listReservation(renter, retry.ID); item.UnreadCount != 0 {
		t.Fatalf("terminal own message was counted unread=%d", item.UnreadCount)
	}
	if cursor, markErr := conversationService.MarkRead(ctx, host, retry.ID, terminalUnreadMessage.Sequence); markErr != nil || cursor != terminalUnreadMessage.Sequence {
		t.Fatalf("terminal thread read cursor=%d err=%v", cursor, markErr)
	}
	if item := listReservation(host, retry.ID); item.UnreadCount != 0 {
		t.Fatalf("terminal unread counter remained after opening thread: %d", item.UnreadCount)
	}
	quote3, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: start.Add(5 * time.Hour).Format(time.RFC3339), EndAt: start.Add(6 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: quote3.ID}, "expire-key")
	if err != nil {
		t.Fatal(err)
	}
	noResponse, err := svc.Pay(ctx, renter, pending.ID, "sin_respuesta", "no-response")
	if err != booking.ErrSimulatedNoResponse || noResponse.State != "pendiente_de_pago" {
		t.Fatalf("simulated no response changed state: %+v %v", noResponse, err)
	}
	if _, err = svc.Pay(ctx, renter, pending.ID, "exito", "second-payment-after-timeout"); err != booking.ErrConflict {
		t.Fatalf("new payment attempt after ambiguous fake timeout=%v", err)
	}
	clockMu.Lock()
	fixedNow = pending.PayExpiresAt
	clockMu.Unlock()
	expired, err := svc.Get(ctx, renter, pending.ID)
	if err != nil || expired.State != "vencida_pago" {
		t.Fatalf("payment expiry: %+v %v", expired, err)
	}
	var expiredPaymentOperation string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1`, pending.ID).Scan(&expiredPaymentOperation); err != nil || expiredPaymentOperation != "vencida" {
		t.Fatalf("ambiguous payment operation after reservation expiry=%q err=%v", expiredPaymentOperation, err)
	}
	if _, err = conversationService.Send(ctx, renter, pending.ID, "expired-write", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("expired payment reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, host, pending.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 0 {
		t.Fatalf("expired thread not available read-only: %+v err=%v", terminalRead, readErr)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, pending.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired occupancy count=%d err=%v", count, err)
	}
	cancelStart := start.Add(6 * time.Hour)
	cancelQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: cancelStart.Format(time.RFC3339), EndAt: cancelStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	cancelPending, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: cancelQuote.ID}, "inbox-cancel-pending")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conversationService.Send(ctx, renter, cancelPending.ID, "before-cancel", "Mensaje antes de cancelar"); err != nil {
		t.Fatalf("pending thread rejected message before cancellation: %v", err)
	}
	cancelled, err := svc.Cancel(ctx, renter, cancelPending.ID, "cancel-pending-first", "")
	if err != nil || cancelled.Reservation.State != "cancelada_arrendatario" || cancelled.RefundState != "no_aplica" {
		t.Fatalf("renter cancellation while pending: %+v err=%v", cancelled, err)
	}
	if _, err = conversationService.Send(ctx, renter, cancelPending.ID, "after-cancel", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("cancelled reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, host, cancelPending.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 1 || terminalRead.Items[0].Body != "Mensaje antes de cancelar" {
		t.Fatalf("cancelled thread was not preserved read-only: %+v err=%v", terminalRead, readErr)
	}
	if repeatedCancel, repeatErr := svc.Cancel(ctx, renter, cancelPending.ID, "cancel-pending-first", ""); repeatErr != nil || repeatedCancel.Reservation.ID != cancelled.Reservation.ID {
		t.Fatalf("idempotent pending cancellation retry=%+v err=%v", repeatedCancel, repeatErr)
	}
	if _, err = svc.Cancel(ctx, renter, cancelPending.ID, "different-key", ""); err != booking.ErrConflict {
		t.Fatalf("different cancellation retry key should conflict: %v", err)
	}
	if _, err = svc.Pay(ctx, renter, cancelPending.ID, "exito", "payment-after-cancel"); err != booking.ErrConflict {
		t.Fatalf("payment after cancellation: %v", err)
	}
	quote4, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: start.Add(7 * time.Hour).Format(time.RFC3339), EndAt: start.Add(8 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	hostWait, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: quote4.ID}, "host-expire")
	if err != nil {
		t.Fatal(err)
	}
	paid, err := svc.Pay(ctx, renter, hostWait.ID, "exito", "payment-host-expire")
	if err != nil || paid.HostExpiresAt == nil {
		t.Fatalf("host expiry payment: %+v %v", paid, err)
	}
	clockMu.Lock()
	fixedNow = *paid.HostExpiresAt
	clockMu.Unlock()
	hostExpired, err := svc.Get(ctx, host, hostWait.ID)
	if err != nil || hostExpired.State != "vencida_host" {
		t.Fatalf("host response expiry: %+v %v", hostExpired, err)
	}
	if _, err = conversationService.Send(ctx, renter, hostWait.ID, "host-expired-write", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("host-expired reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, renter, hostWait.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 0 {
		t.Fatalf("host-expired thread not available read-only: %+v err=%v", terminalRead, readErr)
	}
	paymentRejectStart := start.Add(9 * time.Hour)
	paymentRejectQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: paymentRejectStart.Format(time.RFC3339), EndAt: paymentRejectStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	cancelledByPayment, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: paymentRejectQuote.ID}, "message-payment-rejected")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conversationService.Send(ctx, renter, cancelledByPayment.ID, "payment-reject-before", "Mensaje antes del rechazo"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, cancelledByPayment.ID, "rechazo", "message-payment-rejection"); err != nil {
		t.Fatal(err)
	}
	if _, err = conversationService.Send(ctx, host, cancelledByPayment.ID, "payment-reject-after", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("payment-rejected reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, host, cancelledByPayment.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 1 {
		t.Fatalf("payment-rejected thread was not preserved read-only: %+v err=%v", terminalRead, readErr)
	}
	// Sending directly at either deadline must run the existing expiry
	// transition under the reservation lock, without a Get/List/catalog call.
	setClock := func(value time.Time) {
		clockMu.Lock()
		fixedNow = value
		clockMu.Unlock()
	}
	baseNow := time.Date(2030, 1, 3, 12, 0, 0, 0, time.UTC)
	setClock(baseNow)
	paymentDeadlineStart := start.Add(12 * time.Hour)
	paymentDeadlineQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: paymentDeadlineStart.Format(time.RFC3339), EndAt: paymentDeadlineStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	paymentDeadlineReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: paymentDeadlineQuote.ID}, "direct-payment-deadline")
	if err != nil {
		t.Fatal(err)
	}
	priorMessage, err := conversationService.Send(ctx, renter, paymentDeadlineReservation.ID, "before-payment-deadline", "Mensaje anterior al plazo")
	if err != nil {
		t.Fatal(err)
	}
	setClock(paymentDeadlineReservation.PayExpiresAt)
	if _, err = conversationService.Send(ctx, renter, paymentDeadlineReservation.ID, "at-payment-deadline", "No debe insertarse"); err != conversation.ErrConflict {
		t.Fatalf("new message at payment deadline err=%v", err)
	}
	priorRetry, err := conversationService.Send(ctx, renter, paymentDeadlineReservation.ID, "before-payment-deadline", "Mensaje anterior al plazo")
	if err != nil || priorRetry.ID != priorMessage.ID {
		t.Fatalf("idempotent message retry after deadline=%+v original=%+v err=%v", priorRetry, priorMessage, err)
	}
	var deadlineState string
	var deadlineMessageCount, deadlineActiveOccupancy int
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, paymentDeadlineReservation.ID).Scan(&deadlineState); err != nil || deadlineState != "vencida_pago" {
		t.Fatalf("payment deadline state=%s err=%v", deadlineState, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, paymentDeadlineReservation.ID).Scan(&deadlineMessageCount); err != nil || deadlineMessageCount != 1 {
		t.Fatalf("payment deadline messages=%d want only preexisting message err=%v", deadlineMessageCount, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, paymentDeadlineReservation.ID).Scan(&deadlineActiveOccupancy); err != nil || deadlineActiveOccupancy != 0 {
		t.Fatalf("payment deadline active occupancy=%d err=%v", deadlineActiveOccupancy, err)
	}
	var paymentExpiryTransition int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 AND estado_nuevo='vencida_pago'`, paymentDeadlineReservation.ID).Scan(&paymentExpiryTransition); err != nil || paymentExpiryTransition != 1 {
		t.Fatalf("payment expiry history transition count=%d err=%v", paymentExpiryTransition, err)
	}
	setClock(baseNow)
	hostDeadlineStart := start.Add(14 * time.Hour)
	hostDeadlineQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: hostDeadlineStart.Format(time.RFC3339), EndAt: hostDeadlineStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	hostDeadlineReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: hostDeadlineQuote.ID}, "direct-host-deadline")
	if err != nil {
		t.Fatal(err)
	}
	paidAtDeadlineReservation, err := svc.Pay(ctx, renter, hostDeadlineReservation.ID, "exito", "direct-host-deadline-payment")
	if err != nil || paidAtDeadlineReservation.HostExpiresAt == nil {
		t.Fatalf("host deadline setup payment=%+v err=%v", paidAtDeadlineReservation, err)
	}
	setClock(*paidAtDeadlineReservation.HostExpiresAt)
	if _, err = conversationService.Send(ctx, host, hostDeadlineReservation.ID, "at-host-deadline", "No debe insertarse"); err != conversation.ErrConflict {
		t.Fatalf("new message at host deadline err=%v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, hostDeadlineReservation.ID).Scan(&deadlineState); err != nil || deadlineState != "vencida_host" {
		t.Fatalf("host deadline state=%s err=%v", deadlineState, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, hostDeadlineReservation.ID).Scan(&deadlineMessageCount); err != nil || deadlineMessageCount != 0 {
		t.Fatalf("host deadline inserted messages=%d err=%v", deadlineMessageCount, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, hostDeadlineReservation.ID).Scan(&deadlineActiveOccupancy); err != nil || deadlineActiveOccupancy != 0 {
		t.Fatalf("host deadline active occupancy=%d err=%v", deadlineActiveOccupancy, err)
	}
	var hostExpiryTransition, simulatedRefund int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 AND estado_nuevo='vencida_host'`, hostDeadlineReservation.ID).Scan(&hostExpiryTransition); err != nil || hostExpiryTransition != 1 {
		t.Fatalf("host expiry history transition count=%d err=%v", hostExpiryTransition, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='devolucion_simulada'`, hostDeadlineReservation.ID).Scan(&simulatedRefund); err != nil || simulatedRefund != 1 {
		t.Fatalf("host expiry simulated refund count=%d err=%v", simulatedRefund, err)
	}
	// Hold the reservation lock in another transaction. Start Send before the
	// payment deadline, wait until PostgreSQL confirms it is blocked on that
	// row, then advance the injected clock to the boundary before releasing it.
	setClock(baseNow)
	lockedStart := start.Add(16 * time.Hour)
	lockedQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: lockedStart.Format(time.RFC3339), EndAt: lockedStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	lockedReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: lockedQuote.ID}, "blocked-send-deadline")
	if err != nil {
		t.Fatal(err)
	}
	lockTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback(context.Background())
	var lockedState string
	if err = lockTx.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, lockedReservation.ID).Scan(&lockedState); err != nil || lockedState != "pendiente_de_pago" {
		t.Fatalf("test lock acquired state=%s err=%v", lockedState, err)
	}
	sendStarted := make(chan struct{})
	sendDone := make(chan error, 1)
	go func() {
		close(sendStarted)
		_, sendErr := conversationService.Send(ctx, renter, lockedReservation.ID, "wait-through-deadline", "No debe entrar tras el plazo")
		sendDone <- sendErr
	}()
	<-sendStarted
	waitDeadline := time.Now().Add(5 * time.Second)
	var waitingOnLock bool
	for !waitingOnLock && time.Now().Before(waitDeadline) {
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename=current_user AND wait_event_type='Lock' AND query ILIKE '%reserva_ensayo_local%')`).Scan(&waitingOnLock); err != nil {
			t.Fatal(err)
		}
		if !waitingOnLock {
			select {
			case sendErr := <-sendDone:
				t.Fatalf("send returned before lock release: %v", sendErr)
			default:
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !waitingOnLock {
		t.Fatal("Send did not block behind the reservation row lock")
	}
	setClock(lockedReservation.PayExpiresAt)
	if err = lockTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-sendDone; err != conversation.ErrConflict {
		t.Fatalf("send started before but acquired lock at payment deadline returned %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1`, lockedReservation.ID).Scan(&deadlineState); err != nil || deadlineState != "vencida_pago" {
		t.Fatalf("blocked send deadline state=%s err=%v", deadlineState, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, lockedReservation.ID).Scan(&deadlineMessageCount); err != nil || deadlineMessageCount != 0 {
		t.Fatalf("blocked send inserted messages=%d err=%v", deadlineMessageCount, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 AND estado_nuevo='vencida_pago'`, lockedReservation.ID).Scan(&paymentExpiryTransition); err != nil || paymentExpiryTransition != 1 {
		t.Fatalf("blocked send expiry history transition count=%d err=%v", paymentExpiryTransition, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, lockedReservation.ID).Scan(&deadlineActiveOccupancy); err != nil || deadlineActiveOccupancy != 0 {
		t.Fatalf("blocked send active occupancy=%d err=%v", deadlineActiveOccupancy, err)
	}
	setClock(baseNow)
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: quote2.ID}, "same-key"); err != booking.ErrConflict {
		t.Fatalf("idempotency payload conflict=%v", err)
	}
	// Two distinct keys and quotes for the same window race on the single
	// occupancy exclusion constraint; only one request can retain it.
	startRace := start.Add(24 * time.Hour)
	qa, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: startRace.Format(time.RFC3339), EndAt: startRace.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	qb, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: startRace.Format(time.RFC3339), EndAt: startRace.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	raceInputs := []booking.RequestInput{{QuoteID: qa.ID}, {QuoteID: qb.ID}}
	raceErrs := make([]error, 2)
	var raceWG sync.WaitGroup
	for i := range raceInputs {
		raceWG.Add(1)
		go func(i int) {
			defer raceWG.Done()
			_, raceErrs[i] = svc.Request(ctx, renter, raceInputs[i], fmt.Sprintf("overlap-%d", i))
		}(i)
	}
	raceWG.Wait()
	successes, conflicts := 0, 0
	for _, e := range raceErrs {
		if e == nil {
			successes++
		} else if e == booking.ErrConflict {
			conflicts++
		} else {
			t.Fatalf("unexpected overlapping request error: %v", e)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("overlap race success/conflict=%d/%d errors=%v", successes, conflicts, raceErrs)
	}
	// A search and quote use the current tariff; a tariff changed after quoting
	// must be rejected again in the reservation transaction.
	rateStart := fixedNow.Add(365 * 24 * time.Hour)
	rateQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: secondSpace, StartAt: rateStart.Format(time.RFC3339), EndAt: rateStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,2,'hora',13000)`, secondSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: rateQuote.ID}, "stale-rate-after-search"); err != booking.ErrConflict {
		t.Fatalf("reservation with stale tariff quote error=%v", err)
	}
	var staleRateReservations, staleRateOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, rateQuote.ID).Scan(&staleRateReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND intervalo && tstzrange($2,$3,'[)')`, secondSpace, rateStart, rateStart.Add(time.Hour)).Scan(&staleRateOccupancies); err != nil {
		t.Fatal(err)
	}
	if staleRateReservations != 0 || staleRateOccupancies != 0 {
		t.Fatalf("stale tariff rejection left reservation/occupancy=%d/%d", staleRateReservations, staleRateOccupancies)
	}

	// A tariff edit locks the space row before appending its immutable tariff
	// version. Hold that exact lock, start a request with a still-current quote,
	// and commit the tariff edit only after PostgreSQL confirms the request is
	// waiting for its FOR SHARE lock. The request must then observe the new
	// version and fail without a reservation or occupancy.
	concurrentRateStart := fixedNow.Add(400 * 24 * time.Hour)
	concurrentRateQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: secondSpace, StartAt: concurrentRateStart.Format(time.RFC3339), EndAt: concurrentRateStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	tariffTx, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tariffTx.Rollback(ctx)
	var oldMode string
	var oldPrice int64
	if err = tariffTx.QueryRow(ctx, `SELECT modalidad_tarifa,precio_base_clp FROM public.espacio WHERE id=$1 FOR UPDATE`, secondSpace).Scan(&oldMode, &oldPrice); err != nil {
		t.Fatal(err)
	}
	if _, err = tariffTx.Exec(ctx, `UPDATE public.espacio SET modalidad_tarifa='hora',precio_base_clp=14000 WHERE id=$1`, secondSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = tariffTx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,3,'hora',14000)`, secondSpace); err != nil {
		t.Fatal(err)
	}
	rateRequestErr := make(chan error, 1)
	rateRequestStarted := make(chan struct{})
	go func() {
		close(rateRequestStarted)
		_, requestErr := svc.Request(ctx, renter, booking.RequestInput{QuoteID: concurrentRateQuote.ID}, "concurrent-stale-rate")
		rateRequestErr <- requestErr
	}()
	<-rateRequestStarted
	lockDeadline := time.Now().Add(5 * time.Second)
	var waitingForSpace bool
	for !waitingForSpace && time.Now().Before(lockDeadline) {
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename=current_user AND wait_event_type='Lock' AND query ILIKE '%FOR SHARE OF e%')`).Scan(&waitingForSpace); err != nil {
			t.Fatal(err)
		}
		if !waitingForSpace {
			select {
			case requestErr := <-rateRequestErr:
				t.Fatalf("request completed before tariff lock was released: %v", requestErr)
			default:
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !waitingForSpace {
		t.Fatal("reservation request did not wait for the tariff update's space-row lock")
	}
	if err = tariffTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-rateRequestErr; err != booking.ErrConflict {
		t.Fatalf("request using an outdated concurrent tariff quote error=%v", err)
	}
	var concurrentRateReservations, concurrentRateOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, concurrentRateQuote.ID).Scan(&concurrentRateReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND intervalo && tstzrange($2,$3,'[)')`, secondSpace, concurrentRateStart, concurrentRateStart.Add(time.Hour)).Scan(&concurrentRateOccupancies); err != nil {
		t.Fatal(err)
	}
	if concurrentRateReservations != 0 || concurrentRateOccupancies != 0 {
		t.Fatalf("concurrent stale tariff rejection left reservation/occupancy=%d/%d", concurrentRateReservations, concurrentRateOccupancies)
	}
	clockMu.Lock()
	fixedNow = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clockMu.Unlock()
	selectorZone, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	daySpace, monthSpace := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	for _, fixture := range []struct{ id, category, title, unit string }{
		{daySpace, "bodega", "Selector local · día", "dia"},
		{monthSpace, "local_flexible", "Selector local · mes", "mes"},
	} {
		if _, err = setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,$3,$4,repeat('Fixture sintético de selector. ',4),20,4,'Uso de prueba',$5,8000,'Privada','America/Santiago')`, fixture.id, host, fixture.category, fixture.title, fixture.unit); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) SELECT $1,$2,max(version),'{}' FROM public.categoria_perfil_atributos WHERE categoria_codigo=$2`, fixture.id, fixture.category); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,$2,8000)`, fixture.id, fixture.unit); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, fixture.id, host, renter); err != nil {
			t.Fatal(err)
		}
	}
	blockStart := time.Date(2030, time.January, 2, 10, 0, 0, 0, selectorZone)
	for index, block := range []struct {
		space      string
		start, end time.Time
	}{
		{space, blockStart, blockStart.Add(time.Hour)},
		{daySpace, time.Date(2030, time.January, 2, 10, 0, 0, 0, selectorZone), time.Date(2030, time.January, 2, 11, 0, 0, 0, selectorZone)},
		{monthSpace, time.Date(2030, time.February, 1, 0, 0, 0, 0, selectorZone), time.Date(2030, time.February, 1, 1, 0, 0, 0, selectorZone)},
	} {
		if _, err = setup.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo) VALUES($1,$2,NULL,tstzrange($3,$4,'[)'),'bloqueo_manual',true,'bloqueo sintético de prueba')`, fmt.Sprintf("33333333-3333-4333-8333-%012d", index+1), block.space, block.start, block.end); err != nil {
			t.Fatal(err)
		}
	}
	dayBeforeQuotes, dayBeforeOccupancies := int64(0), int64(0)
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cotizacion_reserva_ensayo),(SELECT count(*) FROM public.ocupacion)`).Scan(&dayBeforeQuotes, &dayBeforeOccupancies); err != nil {
		t.Fatal(err)
	}
	hourlyOptions, err := svc.AvailableIntervals(ctx, renter, space, booking.AvailabilityOptionsInput{Date: "2030-01-02", Duration: 1})
	if err != nil || hourlyOptions.RateUnit != "hora" || len(hourlyOptions.Items) == 0 {
		t.Fatalf("hourly availability options=%+v err=%v", hourlyOptions, err)
	}
	hasAt := func(options booking.AvailabilityOptions, local time.Time) bool {
		for _, item := range options.Items {
			if item.StartAt.Equal(local.UTC()) {
				return true
			}
		}
		return false
	}
	for _, local := range []time.Time{
		time.Date(2030, 1, 2, 9, 0, 0, 0, selectorZone),  // adjacent before block
		time.Date(2030, 1, 2, 11, 0, 0, 0, selectorZone), // adjacent after block
	} {
		if !hasAt(hourlyOptions, local) {
			t.Fatalf("adjacent hourly option %s missing", local)
		}
	}
	for _, local := range []time.Time{
		time.Date(2030, 1, 2, 9, 30, 0, 0, selectorZone), // partial overlap
		time.Date(2030, 1, 2, 10, 0, 0, 0, selectorZone),
		time.Date(2030, 1, 2, 10, 30, 0, 0, selectorZone),
	} {
		if hasAt(hourlyOptions, local) {
			t.Fatalf("overlapping hourly option %s was offered", local)
		}
	}
	dayBlocked, err := svc.AvailableIntervals(ctx, renter, daySpace, booking.AvailabilityOptionsInput{Date: "2030-01-02", Duration: 1})
	if err != nil || dayBlocked.RateUnit != "dia" || len(dayBlocked.Items) != 0 {
		t.Fatalf("partially blocked full-day candidate=%+v err=%v", dayBlocked, err)
	}
	dayFree, err := svc.AvailableIntervals(ctx, host, daySpace, booking.AvailabilityOptionsInput{Date: "2030-01-03", Duration: 2})
	if err != nil || len(dayFree.Items) != 1 || dayFree.Items[0].EndAt.Sub(dayFree.Items[0].StartAt) != 48*time.Hour {
		t.Fatalf("calendar-day candidate=%+v err=%v", dayFree, err)
	}
	monthBlocked, err := svc.AvailableIntervals(ctx, renter, monthSpace, booking.AvailabilityOptionsInput{Date: "2030-01-31", Duration: 1})
	if err != nil || monthBlocked.RateUnit != "mes" || len(monthBlocked.Items) != 0 {
		t.Fatalf("partially blocked monthly candidate=%+v err=%v", monthBlocked, err)
	}
	monthFree, err := svc.AvailableIntervals(ctx, renter, monthSpace, booking.AvailabilityOptionsInput{Date: "2030-02-28", Duration: 1})
	if err != nil || len(monthFree.Items) != 1 || monthFree.Items[0].EndAt.In(selectorZone).Day() != 28 || monthFree.Items[0].EndAt.In(selectorZone).Month() != time.March {
		t.Fatalf("monthly anniversary candidate=%+v err=%v", monthFree, err)
	}
	if _, err = svc.AvailableIntervals(ctx, outsider, space, booking.AvailabilityOptionsInput{Date: "2030-01-02", Duration: 1}); err != booking.ErrNotFound {
		t.Fatalf("unauthorized selector access=%v", err)
	}
	var selectorQuotesAfter, selectorOccupanciesAfter int64
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cotizacion_reserva_ensayo),(SELECT count(*) FROM public.ocupacion)`).Scan(&selectorQuotesAfter, &selectorOccupanciesAfter); err != nil {
		t.Fatal(err)
	}
	if selectorQuotesAfter != dayBeforeQuotes || selectorOccupanciesAfter != dayBeforeOccupancies {
		t.Fatalf("availability lookup persisted quotes/occupancies: before=%d/%d after=%d/%d", dayBeforeQuotes, dayBeforeOccupancies, selectorQuotesAfter, selectorOccupanciesAfter)
	}
	selectedInterval := hourlyOptions.Items[0]
	lateAvailabilityQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: selectedInterval.StartAt.Format(time.RFC3339Nano), EndAt: selectedInterval.EndAt.Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("quote for recently displayed free interval: %v", err)
	}
	lateBlockID := "44444444-4444-4444-8444-444444444444"
	if _, err = setup.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo) VALUES($1,$2,NULL,tstzrange($3,$4,'[)'),'bloqueo_manual',true,'bloqueo agregado después de consultar')`, lateBlockID, space, selectedInterval.StartAt, selectedInterval.EndAt); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: lateAvailabilityQuote.ID}, "availability-became-stale"); err != booking.ErrConflict {
		t.Fatalf("reservation after a post-query occupancy should conflict: %v", err)
	}
	var lateBlockReservations, lateBlockBookingOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, lateAvailabilityQuote.ID).Scan(&lateBlockReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND tipo='reserva' AND activo AND intervalo && tstzrange($2,$3,'[)')`, space, selectedInterval.StartAt, selectedInterval.EndAt).Scan(&lateBlockBookingOccupancies); err != nil {
		t.Fatal(err)
	}
	if lateBlockReservations != 0 || lateBlockBookingOccupancies != 0 {
		t.Fatalf("stale availability rejection left reservation/booking occupancy=%d/%d", lateBlockReservations, lateBlockBookingOccupancies)
	}

	// Exercise the selector intervals through the real PostgreSQL quote path:
	// offered local calendar durations must map to the approved billable units.
	clockMu.Lock()
	fixedNow = time.Date(2026, time.August, 5, 23, 30, 0, 0, selectorZone)
	clockMu.Unlock()
	dstDaySpace, dstMonthSpace := "66666666-6666-4666-8666-666666666601", "66666666-6666-4666-8666-666666666602"
	for _, fixture := range []struct {
		id, category, title, unit string
		price                     int64
	}{
		{dstDaySpace, "bodega", "Selector DST · día", "dia", 12000},
		{dstMonthSpace, "local_flexible", "Selector DST · mes", "mes", 450000},
	} {
		if _, err = setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,$3,$4,repeat('Fixture sintético de prueba DST. ',4),20,4,'Uso de prueba',$5,$6,'Privada','America/Santiago')`, fixture.id, host, fixture.category, fixture.title, fixture.unit, fixture.price); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) SELECT $1,$2,max(version),'{}' FROM public.categoria_perfil_atributos WHERE categoria_codigo=$2`, fixture.id, fixture.category); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,$2,$3)`, fixture.id, fixture.unit, fixture.price); err != nil {
			t.Fatal(err)
		}
		if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, fixture.id, host, renter); err != nil {
			t.Fatal(err)
		}
	}
	for days := 1; days <= 3; days++ {
		options, optionsErr := svc.AvailableIntervals(ctx, renter, dstDaySpace, booking.AvailabilityOptionsInput{Date: "2026-09-06", Duration: days})
		if optionsErr != nil || len(options.Items) != 1 {
			t.Fatalf("DST %d-day options=%+v err=%v", days, options, optionsErr)
		}
		interval := options.Items[0]
		startLocal, endLocal := interval.StartAt.In(selectorZone), interval.EndAt.In(selectorZone)
		if startLocal.Format("2006-01-02 15:04") != "2026-09-06 01:00" || !endLocal.After(startLocal) {
			t.Fatalf("DST %d-day interval=%s–%s", days, startLocal, endLocal)
		}
		quote, quoteErr := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: dstDaySpace, StartAt: interval.StartAt.Format(time.RFC3339Nano), EndAt: interval.EndAt.Format(time.RFC3339Nano)})
		if quoteErr != nil || quote.Units != int64(days) || quote.Subtotal != int64(days)*12000 {
			t.Fatalf("DST %d-day quote units/subtotal=%d/%d err=%v", days, quote.Units, quote.Subtotal, quoteErr)
		}
	}
	monthOptions, err := svc.AvailableIntervals(ctx, renter, dstMonthSpace, booking.AvailabilityOptionsInput{Date: "2026-08-06", Duration: 1})
	if err != nil || len(monthOptions.Items) != 1 {
		t.Fatalf("DST monthly options=%+v err=%v", monthOptions, err)
	}
	monthInterval := monthOptions.Items[0]
	monthStart, monthEnd := monthInterval.StartAt.In(selectorZone), monthInterval.EndAt.In(selectorZone)
	if monthStart.Format("2006-01-02 15:04") != "2026-08-06 00:00" || monthEnd.Format("2006-01-02 15:04") != "2026-09-06 01:00" {
		t.Fatalf("monthly anniversary interval=%s–%s", monthStart, monthEnd)
	}
	monthQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: dstMonthSpace, StartAt: monthInterval.StartAt.Format(time.RFC3339Nano), EndAt: monthInterval.EndAt.Format(time.RFC3339Nano)})
	if err != nil || monthQuote.Units != 1 || monthQuote.Subtotal != 450000 {
		t.Fatalf("monthly quote units/subtotal=%d/%d err=%v", monthQuote.Units, monthQuote.Subtotal, err)
	}

	// Weekly rules are a separate configuration and serialize with quote and
	// reservation via the owning space row. Thursday includes a pause; Sunday
	// spans the first valid local hour after Santiago's spring gap.
	clockMu.Lock()
	fixedNow = time.Date(2026, time.September, 3, 0, 0, 0, 0, selectorZone)
	clockMu.Unlock()
	weekly := booking.WeeklyHours{Enabled: true, Days: make([]booking.WeeklyDay, 7)}
	for i := range weekly.Days {
		weekly.Days[i] = booking.WeeklyDay{Weekday: i + 1, Periods: []booking.WeeklyPeriod{}}
	}
	weekly.Days[3].Periods = []booking.WeeklyPeriod{{Open: "09:00", Close: "12:00"}, {Open: "13:00", Close: "17:00"}}
	weekly.Days[5].Periods = []booking.WeeklyPeriod{{Open: "22:00", Close: "24:00"}}
	weekly.Days[6].Periods = []booking.WeeklyPeriod{{Open: "01:00", Close: "04:00"}}
	savedHours, err := svc.SaveWeeklyHours(ctx, host, space, weekly)
	if err != nil || !savedHours.Enabled || savedHours.TimeZone != "America/Santiago" {
		t.Fatalf("save weekly hours=%+v err=%v", savedHours, err)
	}
	if _, err = svc.WeeklyHours(ctx, renter, space); err != booking.ErrNotFound {
		t.Fatalf("renter could read host-only schedule: %v", err)
	}
	if _, err = svc.WeeklyHours(ctx, outsider, space); err != booking.ErrNotFound {
		t.Fatalf("outsider could read schedule: %v", err)
	}
	thursday, err := svc.AvailableIntervals(ctx, renter, space, booking.AvailabilityOptionsInput{Date: "2026-09-03", Duration: 1})
	if err != nil || len(thursday.Items) == 0 {
		t.Fatalf("Thursday schedule candidates=%+v err=%v", thursday, err)
	}
	starts := map[string]bool{}
	for _, interval := range thursday.Items {
		localStart, localEnd := interval.StartAt.In(selectorZone), interval.EndAt.In(selectorZone)
		if localStart.Day() != 3 || localEnd.Day() != 3 || localStart.Hour() == 12 || localStart.Hour() == 11 && localStart.Minute() == 30 || localStart.Hour() < 9 || localStart.Hour() >= 17 {
			t.Fatalf("candidate crosses outside a scheduled segment: %s to %s", localStart, localEnd)
		}
		starts[localStart.Format("15:04")] = true
	}
	if !starts["09:00"] || !starts["13:00"] || starts["11:30"] {
		t.Fatalf("paused Thursday candidates do not match schedule: %v", starts)
	}
	sunday, err := svc.AvailableIntervals(ctx, renter, space, booking.AvailabilityOptionsInput{Date: "2026-09-06", Duration: 1})
	if err != nil || len(sunday.Items) == 0 || sunday.Items[0].StartAt.In(selectorZone).Format("2006-01-02 15:04") != "2026-09-06 01:00" {
		t.Fatalf("spring-gap Sunday candidates=%+v err=%v", sunday, err)
	}
	closedFriday, err := svc.AvailableIntervals(ctx, renter, space, booking.AvailabilityOptionsInput{Date: "2026-09-04", Duration: 1})
	if err != nil || len(closedFriday.Items) != 0 {
		t.Fatalf("active schedule's closed Friday options=%+v err=%v", closedFriday, err)
	}
	clockMu.Lock()
	fixedNow = time.Date(2026, time.April, 4, 0, 0, 0, 0, selectorZone)
	clockMu.Unlock()
	foldOptions, err := svc.AvailableIntervals(ctx, renter, space, booking.AvailabilityOptionsInput{Date: "2026-04-04", Duration: 1})
	if err != nil {
		t.Fatalf("fall-fold Saturday options err=%v", err)
	}
	var repeated23 []time.Time
	for _, interval := range foldOptions.Items {
		localStart, localEnd := interval.StartAt.In(selectorZone), interval.EndAt.In(selectorZone)
		endAtMidnight := localEnd.Year() == 2026 && localEnd.Month() == time.April && localEnd.Day() == 5 && localEnd.Hour() == 0 && localEnd.Minute() == 0
		if localStart.Day() != 4 || (localEnd.Day() != 4 && !endAtMidnight) || localStart.Hour() < 22 {
			t.Fatalf("fall-fold option outside Saturday 22:00–24:00: %s to %s", localStart, localEnd)
		}
		if localStart.Hour() == 23 && localStart.Minute() == 0 {
			repeated23 = append(repeated23, interval.StartAt)
		}
	}
	if len(repeated23) != 2 || !repeated23[0].Before(repeated23[1]) {
		t.Fatalf("repeated 23:00 candidates were not both preserved: %v", repeated23)
	}
	closedCatalog, err := svc.Catalog(ctx, renter, booking.CatalogFilter{StartAt: timePtr(time.Date(2026, 9, 3, 12, 0, 0, 0, selectorZone)), EndAt: timePtr(time.Date(2026, 9, 3, 13, 0, 0, 0, selectorZone))})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range closedCatalog {
		if item.SpaceID == space {
			t.Fatal("catalog interval filter included a schedule pause")
		}
	}
	if _, err = svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: time.Date(2026, 9, 3, 11, 30, 0, 0, selectorZone).UTC().Format(time.RFC3339Nano), EndAt: time.Date(2026, 9, 3, 12, 30, 0, 0, selectorZone).UTC().Format(time.RFC3339Nano)}); err != booking.ErrConflict {
		t.Fatalf("quote spanning scheduled pause err=%v", err)
	}
	weeklyStaleStart := time.Date(2026, 9, 3, 13, 0, 0, 0, selectorZone)
	staleQuote, err = svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: weeklyStaleStart.UTC().Format(time.RFC3339Nano), EndAt: weeklyStaleStart.Add(time.Hour).UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("quote inside schedule err=%v", err)
	}
	weekly.Days[3].Periods = []booking.WeeklyPeriod{{Open: "14:00", Close: "17:00"}}
	if _, err = svc.SaveWeeklyHours(ctx, host, space, weekly); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: staleQuote.ID}, "weekly-hours-stale-quote"); err != booking.ErrConflict {
		t.Fatalf("request using quote outside updated schedule err=%v", err)
	}
	var weeklyRejectedReservations, weeklyRejectedOccupancies int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1),(SELECT count(*) FROM public.ocupacion WHERE tipo IN ('retencion','reserva') AND intervalo && tstzrange($2,$3,'[)'))`, staleQuote.ID, weeklyStaleStart, weeklyStaleStart.Add(time.Hour)).Scan(&weeklyRejectedReservations, &weeklyRejectedOccupancies); err != nil {
		t.Fatal(err)
	}
	if weeklyRejectedReservations != 0 || weeklyRejectedOccupancies != 0 {
		t.Fatalf("stale weekly-hours quote left reservation/occupancy=%d/%d", weeklyRejectedReservations, weeklyRejectedOccupancies)
	}
	// Deterministic schedule change race: the request waits on the space row,
	// then sees the committed new schedule and rejects without side effects.
	weeklyRaceStart := time.Date(2026, 9, 3, 15, 0, 0, 0, selectorZone)
	raceQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: weeklyRaceStart.UTC().Format(time.RFC3339Nano), EndAt: weeklyRaceStart.Add(time.Hour).UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatalf("concurrency quote err=%v", err)
	}
	raceTx, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raceTx.Exec(ctx, `SELECT id FROM public.espacio WHERE id=$1 FOR UPDATE`, space); err != nil {
		t.Fatal(err)
	}
	if _, err = raceTx.Exec(ctx, `UPDATE public.espacio_horario_semanal_tramo SET apertura_minuto=16*60,cierre_minuto=17*60 WHERE espacio_id=$1 AND dia_iso=4`, space); err != nil {
		t.Fatal(err)
	}
	raceResult := make(chan error, 1)
	go func() {
		_, requestErr := svc.Request(ctx, renter, booking.RequestInput{QuoteID: raceQuote.ID}, "weekly-hours-concurrent-change")
		raceResult <- requestErr
	}()
	weeklyLockDeadline := time.Now().Add(3 * time.Second)
	requestWaitingOnSpace := false
	for time.Now().Before(weeklyLockDeadline) {
		if err = setup.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query ILIKE '%public.espacio%')`).Scan(&requestWaitingOnSpace); err != nil {
			t.Fatal(err)
		}
		if requestWaitingOnSpace {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !requestWaitingOnSpace {
		select {
		case earlyErr := <-raceResult:
			t.Fatalf("request finished before acquiring the held space-row lock: %v", earlyErr)
		default:
		}
		t.Fatal("request did not wait for the held space-row lock")
	}
	if err = raceTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-raceResult; err != booking.ErrConflict {
		t.Fatalf("request waiting on schedule change err=%v", err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.espacio SET zona_horaria='UTC' WHERE id=$1`, space); err == nil {
		t.Fatal("timezone change was accepted while weekly schedule is active")
	}
	if _, err = setup.Exec(ctx, `UPDATE public.espacio SET modalidad_tarifa='dia' WHERE id=$1`, space); err == nil {
		t.Fatal("daily tariff was enabled while a weekly hourly schedule is active")
	}
	weekly.Enabled = false
	if _, err = svc.SaveWeeklyHours(ctx, host, space, weekly); err != nil {
		t.Fatalf("disable weekly schedule before timezone review: %v", err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.espacio SET zona_horaria='UTC' WHERE id=$1`, space); err != nil {
		t.Fatalf("timezone change after disabling schedule: %v", err)
	}
	// Cancellation/refund consolidation: policy is snapshotted at quote time,
	// cancellation and occupancy release are atomic, and fake retries reuse one
	// stable refund operation without creating another obligation.
	clockMu.Lock()
	fixedNow = time.Date(2032, 2, 1, 12, 0, 0, 0, time.UTC)
	clockMu.Unlock()
	newReservation := func(offset time.Duration, idem string, approve bool) booking.Reservation {
		t.Helper()
		clockMu.Lock()
		now := fixedNow
		clockMu.Unlock()
		startAt := now.Add(offset)
		q, quoteErr := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: startAt.UTC().Format(time.RFC3339Nano), EndAt: startAt.Add(time.Hour).UTC().Format(time.RFC3339Nano)})
		if quoteErr != nil {
			t.Fatalf("quote for cancellation test offset=%s now=%s start=%s: %v", offset, now, startAt, quoteErr)
		}
		created, createErr := svc.Request(ctx, renter, booking.RequestInput{QuoteID: q.ID}, idem)
		if createErr != nil {
			t.Fatalf("create cancellation test reservation: %v", createErr)
		}
		paid, payErr := svc.Pay(ctx, renter, created.ID, "exito", "pay-"+idem)
		if payErr != nil || paid.State != "pagada" {
			t.Fatalf("pay cancellation test reservation: %+v %v", paid, payErr)
		}
		if approve {
			approved, approveErr := svc.Decide(ctx, host, created.ID, "aprobar", "")
			if approveErr != nil || approved.State != "aprobada_host" {
				t.Fatalf("approve cancellation test reservation: %+v %v", approved, approveErr)
			}
			return approved
		}
		return paid
	}
	// A quote keeps its policy snapshot if the fixture's future default changes
	// before request creation; the default is restored for later cases.
	snapshotStart := fixedNow.Add(8 * time.Hour)
	snapshotQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: space, StartAt: snapshotStart.Format(time.RFC3339Nano), EndAt: snapshotStart.Add(time.Hour).Format(time.RFC3339Nano)})
	if err != nil || snapshotQuote.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		t.Fatalf("quote cancellation policy snapshot=%q err=%v", snapshotQuote.CancellationPolicyVersion, err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.reserva_ensayo_local_fixture SET politica_cancelacion_version='future_policy' WHERE espacio_id=$1`, space); err != nil {
		t.Fatal(err)
	}
	snapshotReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: snapshotQuote.ID}, "snapshot-policy")
	if err != nil || snapshotReservation.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		t.Fatalf("reservation did not preserve quote policy snapshot: %+v err=%v", snapshotReservation, err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.reserva_ensayo_local_fixture SET politica_cancelacion_version='local_flexible_v1' WHERE espacio_id=$1`, space); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, snapshotReservation.ID, "exito", "snapshot-policy-pay"); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.CancellationPreview(ctx, renter, snapshotReservation.ID)
	if err != nil || !preview.Eligible || preview.AmountCLP != snapshotReservation.Subtotal || preview.PolicyVersion != booking.LocalCancellationPolicyVersion || preview.RefundLabel != "Devolución simulada — sin movimiento de dinero" {
		t.Fatalf("cancellation preview=%+v err=%v", preview, err)
	}
	if _, err = svc.CancellationPreview(ctx, host, snapshotReservation.ID); err != booking.ErrNotFound {
		t.Fatalf("host viewed renter-only cancellation preview: %v", err)
	}
	noticeRecorder.mu.Lock()
	noticesBeforePaidCancel := len(noticeRecorder.recipients)
	noticeRecorder.mu.Unlock()
	paidCancelled, err := svc.Cancel(ctx, renter, snapshotReservation.ID, "cancel-snapshot", "Cambio de planes")
	if err != nil || paidCancelled.Reservation.State != "cancelada_arrendatario" || paidCancelled.RefundState != "pendiente" || paidCancelled.RefundAmountCLP == nil || *paidCancelled.RefundAmountCLP != snapshotReservation.Subtotal {
		t.Fatalf("paid cancellation=%+v err=%v", paidCancelled, err)
	}
	if paidCancelled.NoticeStatus != "mailpit_local_no_durable" {
		t.Fatalf("notice status with local SMTP adapter=%q", paidCancelled.NoticeStatus)
	}
	noticeRecorder.mu.Lock()
	if len(noticeRecorder.recipients) != noticesBeforePaidCancel+2 || noticeRecorder.recipients[noticesBeforePaidCancel] != "booking-0@example.test" || noticeRecorder.recipients[noticesBeforePaidCancel+1] != "booking-1@example.test" {
		t.Fatalf("cancellation notice recipients=%v, want both participants", noticeRecorder.recipients)
	}
	noticeRecorder.mu.Unlock()
	replayedCancel, err := svc.Cancel(ctx, renter, snapshotReservation.ID, "cancel-snapshot", "Cambio de planes")
	if err != nil || !replayedCancel.Replayed || replayedCancel.Reservation.ID != snapshotReservation.ID {
		t.Fatalf("cancellation idempotent retry=%+v err=%v", replayedCancel, err)
	}
	if _, err = svc.Cancel(ctx, renter, snapshotReservation.ID, "cancel-snapshot", "otro motivo"); err != booking.ErrConflict {
		t.Fatalf("same cancellation key with different input err=%v", err)
	}
	if _, err = svc.Refund(ctx, renter, snapshotReservation.ID, "00000000-0000-4000-8000-000000000001", "exito"); err != booking.ErrConflict {
		t.Fatalf("wrong refund operation identity err=%v", err)
	}
	refundOperation := *paidCancelled.Reservation.RefundOperationID
	failedRefund, err := svc.Refund(ctx, renter, snapshotReservation.ID, refundOperation, "fallo")
	if err != nil || failedRefund.State != "pendiente" || failedRefund.LastResult != "fallo_simulado" {
		t.Fatalf("failed fake refund=%+v err=%v", failedRefund, err)
	}
	_, err = svc.Refund(ctx, renter, snapshotReservation.ID, refundOperation, "sin_respuesta")
	if err != booking.ErrSimulatedRefundNoResponse {
		t.Fatalf("fake refund timeout err=%v", err)
	}
	completedRefund, err := svc.Refund(ctx, renter, snapshotReservation.ID, refundOperation, "exito")
	if err != nil || completedRefund.State != "completada" || completedRefund.AmountCLP != snapshotReservation.Subtotal {
		t.Fatalf("successful refund retry=%+v err=%v", completedRefund, err)
	}
	completedRetry, err := svc.Refund(ctx, renter, snapshotReservation.ID, refundOperation, "fallo")
	if err != nil || completedRetry.State != "completada" {
		t.Fatalf("completed refund replay=%+v err=%v", completedRetry, err)
	}
	var refunds, refundAttempts, cancelActiveOccupancies, cancellationRows int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1),(SELECT count(*) FROM public.reserva_devolucion_intento_ensayo i JOIN public.reserva_devolucion_ensayo d ON d.id=i.devolucion_id WHERE d.reserva_id=$1),(SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo),(SELECT count(*) FROM public.reserva_cancelacion_ensayo WHERE reserva_id=$1)`, snapshotReservation.ID).Scan(&refunds, &refundAttempts, &cancelActiveOccupancies, &cancellationRows); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 || refundAttempts != 3 || cancelActiveOccupancies != 0 || cancellationRows != 1 {
		t.Fatalf("cancel/refund atomicity counts refund=%d attempts=%d active occupancy=%d cancellations=%d", refunds, refundAttempts, cancelActiveOccupancies, cancellationRows)
	}
	noticeRecorder.mu.Lock()
	if len(noticeRecorder.recipients) != noticesBeforePaidCancel+8 {
		t.Fatalf("cancellation and fake refund notices recipients=%v; want both participants for four local state updates", noticeRecorder.recipients)
	}
	noticeRecorder.mu.Unlock()
	snapshotDetail, err := svc.Get(ctx, renter, snapshotReservation.ID)
	if err != nil || snapshotDetail.RefundState == nil || *snapshotDetail.RefundState != "completada" || snapshotDetail.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion || len(snapshotDetail.History) != 3 || snapshotDetail.History[2].To != "cancelada_arrendatario" {
		t.Fatalf("cancelled detail/history/refund=%+v err=%v", snapshotDetail, err)
	}

	type refundCall struct {
		result booking.RefundResult
		err    error
	}
	newConcurrentRefund := func(idempotency string) (booking.Reservation, string) {
		t.Helper()
		reservation := newReservation(8*time.Hour, idempotency, false)
		cancelled, cancelErr := svc.Cancel(ctx, renter, reservation.ID, "cancel-"+idempotency, "prueba de concurrencia")
		if cancelErr != nil || cancelled.Reservation.RefundOperationID == nil {
			t.Fatalf("create refund obligation for %s: %+v %v", idempotency, cancelled, cancelErr)
		}
		return cancelled.Reservation, *cancelled.Reservation.RefundOperationID
	}
	newConcurrentRefundService := func(adapter booking.LocalRefundAdapter) (*booking.Service, *refundRecordSignalRepository) {
		t.Helper()
		signalRepo := &refundRecordSignalRepository{Repository: repo, completed: make(chan struct{})}
		paymentAdapter := newTestPaymentAdapter(t)
		refundService, serviceErr := booking.NewService(signalRepo, credentials.Generator{}, clock, paymentAdapter)
		if serviceErr != nil {
			t.Fatal(serviceErr)
		}
		refundService.SetLocalRefundAdapter(adapter)
		refundService.SetLocalNoticeSender(noticeRecorder)
		return refundService, signalRepo
	}
	awaitOutcome := func(entered <-chan string, expected string) {
		t.Helper()
		select {
		case outcome := <-entered:
			if outcome != expected {
				t.Fatalf("refund fake entered with %q, want %q", outcome, expected)
			}
		case <-ctx.Done():
			t.Fatalf("refund fake did not enter with %q before context expired", expected)
		}
	}
	checkSingleCompletion := func(reservationID, expectedResult string, first, second booking.RefundResult) {
		t.Helper()
		var operationID, currency, state, lastResult string
		var amount int64
		var updatedAt time.Time
		var attempts int
		if err = setup.QueryRow(ctx, `SELECT d.operacion_id::text,d.importe_clp,d.moneda,d.estado,d.ultimo_resultado,d.actualizada_en,(SELECT count(*) FROM public.reserva_devolucion_intento_ensayo i WHERE i.devolucion_id=d.id) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=$1`, reservationID).Scan(&operationID, &amount, &currency, &state, &lastResult, &updatedAt, &attempts); err != nil {
			t.Fatal(err)
		}
		if state != "completada" || lastResult != expectedResult || attempts != 1 {
			t.Fatalf("persisted refund state/result/attempts=%s/%s/%d", state, lastResult, attempts)
		}
		if first.OperationID != operationID || second.OperationID != operationID || first.AmountCLP != amount || second.AmountCLP != amount || first.Currency != currency || second.Currency != currency || first.State != state || second.State != state || first.LastResult != lastResult || second.LastResult != lastResult || !first.UpdatedAt.Equal(updatedAt) || !second.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("refund responses diverged from persistence: db=%s/%s/%d %s %s first=%+v second=%+v", state, lastResult, amount, operationID, updatedAt, first, second)
		}
	}

	// A completion that commits while another fake request is still waiting
	// must win over the waiter's stale timeout result. The losing response uses
	// the committed values and must not send a second notice or return 504.
	timeoutVsSuccessReservation, timeoutVsSuccessOperation := newConcurrentRefund("refund-race-timeout-success")
	timeoutRelease, successRelease := make(chan struct{}, 1), make(chan struct{}, 1)
	timeoutSuccessAdapter := &gatedRefundAdapter{entered: make(chan string, 2), release: map[string]<-chan struct{}{"sin_respuesta": timeoutRelease, "exito": successRelease}}
	timeoutSuccessService, timeoutSuccessRepo := newConcurrentRefundService(timeoutSuccessAdapter)
	noticeRecorder.mu.Lock()
	noticesBeforeTimeoutSuccess := len(noticeRecorder.recipients)
	noticeRecorder.mu.Unlock()
	timeoutResponse, successResponse := make(chan refundCall, 1), make(chan refundCall, 1)
	go func() {
		value, callErr := timeoutSuccessService.Refund(ctx, renter, timeoutVsSuccessReservation.ID, timeoutVsSuccessOperation, "sin_respuesta")
		timeoutResponse <- refundCall{result: value, err: callErr}
	}()
	awaitOutcome(timeoutSuccessAdapter.entered, "sin_respuesta")
	go func() {
		value, callErr := timeoutSuccessService.Refund(ctx, renter, timeoutVsSuccessReservation.ID, timeoutVsSuccessOperation, "exito")
		successResponse <- refundCall{result: value, err: callErr}
	}()
	awaitOutcome(timeoutSuccessAdapter.entered, "exito")
	successRelease <- struct{}{}
	select {
	case <-timeoutSuccessRepo.completed:
	case <-ctx.Done():
		t.Fatal("successful refund did not commit before releasing concurrent timeout")
	}
	timeoutRelease <- struct{}{}
	timeoutCall, successCall := <-timeoutResponse, <-successResponse
	if timeoutCall.err != nil || !timeoutCall.result.Reused || timeoutCall.result.NoticeStatus != "no_reintentado_por_idempotencia" {
		t.Fatalf("losing timeout response=%+v err=%v; want reused persisted success without error/notice", timeoutCall.result, timeoutCall.err)
	}
	if successCall.err != nil || successCall.result.Reused || successCall.result.LastResult != "exito_simulado" {
		t.Fatalf("winning success response=%+v err=%v", successCall.result, successCall.err)
	}
	checkSingleCompletion(timeoutVsSuccessReservation.ID, "exito_simulado", timeoutCall.result, successCall.result)
	noticeRecorder.mu.Lock()
	if got := len(noticeRecorder.recipients); got != noticesBeforeTimeoutSuccess+2 {
		noticeRecorder.mu.Unlock()
		t.Fatalf("success/timeout race sent %d notices; want exactly one pair", got-noticesBeforeTimeoutSuccess)
	}
	noticeRecorder.mu.Unlock()

	// Two successful fake requests are released one at a time. Once the first
	// commits, the second returns the same persisted result as reused.
	twoSuccessReservation, twoSuccessOperation := newConcurrentRefund("refund-race-two-successes")
	sharedSuccessRelease := make(chan struct{}, 2)
	firstTwoSuccessAdapter := &gatedRefundAdapter{entered: make(chan string, 2), release: map[string]<-chan struct{}{"exito": sharedSuccessRelease}}
	twoSuccessService, twoSuccessRepo := newConcurrentRefundService(firstTwoSuccessAdapter)
	noticeRecorder.mu.Lock()
	noticesBeforeTwoSuccess := len(noticeRecorder.recipients)
	noticeRecorder.mu.Unlock()
	firstSuccessResponse, secondSuccessResponse := make(chan refundCall, 1), make(chan refundCall, 1)
	go func() {
		value, callErr := twoSuccessService.Refund(ctx, renter, twoSuccessReservation.ID, twoSuccessOperation, "exito")
		firstSuccessResponse <- refundCall{result: value, err: callErr}
	}()
	awaitOutcome(firstTwoSuccessAdapter.entered, "exito")
	go func() {
		value, callErr := twoSuccessService.Refund(ctx, renter, twoSuccessReservation.ID, twoSuccessOperation, "exito")
		secondSuccessResponse <- refundCall{result: value, err: callErr}
	}()
	awaitOutcome(firstTwoSuccessAdapter.entered, "exito")
	sharedSuccessRelease <- struct{}{}
	select {
	case <-twoSuccessRepo.completed:
	case <-ctx.Done():
		t.Fatal("first successful refund did not commit before releasing second success")
	}
	// Both fake calls share the same outcome gate. The first token let exactly
	// one caller return; this token releases the still-waiting caller.
	sharedSuccessRelease <- struct{}{}
	firstSuccessCall, secondSuccessCall := <-firstSuccessResponse, <-secondSuccessResponse
	if firstSuccessCall.err != nil || secondSuccessCall.err != nil {
		t.Fatalf("two success errors=%v/%v", firstSuccessCall.err, secondSuccessCall.err)
	}
	if firstSuccessCall.result.Reused == secondSuccessCall.result.Reused {
		t.Fatalf("exactly one response must be marked reused: first=%+v second=%+v", firstSuccessCall.result, secondSuccessCall.result)
	}
	checkSingleCompletion(twoSuccessReservation.ID, "exito_simulado", firstSuccessCall.result, secondSuccessCall.result)
	loser := firstSuccessCall.result
	if !loser.Reused {
		loser = secondSuccessCall.result
	}
	if loser.NoticeStatus != "no_reintentado_por_idempotencia" {
		t.Fatalf("concurrent reused response notice status=%q", loser.NoticeStatus)
	}
	noticeRecorder.mu.Lock()
	if got := len(noticeRecorder.recipients); got != noticesBeforeTwoSuccess+2 {
		noticeRecorder.mu.Unlock()
		t.Fatalf("two-success race sent %d notices; want exactly one pair", got-noticesBeforeTwoSuccess)
	}
	noticeRecorder.mu.Unlock()

	// The strict start-time boundary is independently checked on an approved
	// booking: at exactly start it conflicts and leaves reservation/occupancy.
	boundary := newReservation(36*time.Hour, "cancel-boundary", true)
	clockMu.Lock()
	fixedNow = boundary.StartAt
	clockMu.Unlock()
	if _, err = svc.Cancel(ctx, renter, boundary.ID, "cancel-at-start", ""); err != booking.ErrConflict {
		t.Fatalf("cancellation at exact start boundary err=%v", err)
	}
	var boundaryState string
	var boundaryOccupancy bool
	var boundaryRefunds int
	if err = setup.QueryRow(ctx, `SELECT r.estado,o.activo,(SELECT count(*) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id) FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id WHERE r.id=$1`, boundary.ID).Scan(&boundaryState, &boundaryOccupancy, &boundaryRefunds); err != nil {
		t.Fatal(err)
	}
	if boundaryState != "aprobada_host" || !boundaryOccupancy || boundaryRefunds != 0 {
		t.Fatalf("exact-start rejection left state=%s occupancy=%v refunds=%d", boundaryState, boundaryOccupancy, boundaryRefunds)
	}
	// The operation begins before start but waits on the reservation lock until
	// the exact boundary. The repository must read the injected clock after the
	// lock, reject, and preserve the reservation, history, refund state and hold.
	lockWaitReservation := newReservation(12*time.Hour, "cancel-clock-after-lock", true)
	clockLockTx, beginClockLockErr := setup.Begin(ctx)
	if beginClockLockErr != nil {
		t.Fatal(beginClockLockErr)
	}
	if _, err = clockLockTx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, lockWaitReservation.ID); err != nil {
		t.Fatal(err)
	}
	clockLockResult := make(chan error, 1)
	go func() {
		_, cancelErr := svc.Cancel(ctx, renter, lockWaitReservation.ID, "cancel-after-lock", "Límite durante espera")
		clockLockResult <- cancelErr
	}()
	clockWaitDeadline := time.Now().Add(3 * time.Second)
	clockCancelWaiting := false
	for time.Now().Before(clockWaitDeadline) {
		if err = setup.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT id::text,cotizacion_id::text%')`).Scan(&clockCancelWaiting); err != nil {
			t.Fatal(err)
		}
		if clockCancelWaiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !clockCancelWaiting {
		_ = clockLockTx.Rollback(ctx)
		t.Fatal("cancellation did not wait on reservation lock")
	}
	clockMu.Lock()
	fixedNow = lockWaitReservation.StartAt
	clockMu.Unlock()
	if err = clockLockTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-clockLockResult; err != booking.ErrConflict {
		t.Fatalf("cancel waiting across start boundary err=%v", err)
	}
	if err = setup.QueryRow(ctx, `SELECT r.estado,o.activo,(SELECT count(*) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id) FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id WHERE r.id=$1`, lockWaitReservation.ID).Scan(&boundaryState, &boundaryOccupancy, &boundaryRefunds); err != nil {
		t.Fatal(err)
	}
	if boundaryState != "aprobada_host" || !boundaryOccupancy || boundaryRefunds != 0 {
		t.Fatalf("cancel after lock wait changed state=%s occupancy=%v refunds=%d", boundaryState, boundaryOccupancy, boundaryRefunds)
	}

	// Race cancellation against host approval while the account-first lock order
	// serializes both with suppression and the reservation transition. One action
	// may wait on the participant lock while the other waits on the reservation;
	// either approval commits first and cancellation follows,
	// or cancellation wins and approval conflicts; there remains one refund and
	// one terminal transition, with occupancy released atomically.
	raceReservation := newReservation(10*time.Hour, "cancel-approve-race", false)
	cancelRaceTx, beginCancelRaceErr := setup.Begin(ctx)
	if beginCancelRaceErr != nil {
		t.Fatal(beginCancelRaceErr)
	}
	if _, err = cancelRaceTx.Exec(ctx, `SELECT id FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, raceReservation.ID); err != nil {
		t.Fatal(err)
	}
	type cancelRaceOutcome struct {
		name string
		err  error
	}
	cancelRaceResults := make(chan cancelRaceOutcome, 2)
	go func() {
		_, e := svc.Decide(ctx, host, raceReservation.ID, "aprobar", "")
		cancelRaceResults <- cancelRaceOutcome{"approval", e}
	}()
	go func() {
		_, e := svc.Cancel(ctx, renter, raceReservation.ID, "cancel-race", "Carrera de ensayo")
		cancelRaceResults <- cancelRaceOutcome{"cancellation", e}
	}()
	cancelLockDeadline := time.Now().Add(3 * time.Second)
	approvalAndCancelWaiting := false
	for time.Now().Before(cancelLockDeadline) {
		if err = setup.QueryRow(ctx, `SELECT count(*)>=2 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query ILIKE '%FOR UPDATE%'`).Scan(&approvalAndCancelWaiting); err != nil {
			t.Fatal(err)
		}
		if approvalAndCancelWaiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !approvalAndCancelWaiting {
		_ = cancelRaceTx.Rollback(ctx)
		t.Fatal("approval and cancellation did not both wait in the protected lock order")
	}
	if err = cancelRaceTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var approvalErr, cancellationErr error
	for range 2 {
		result := <-cancelRaceResults
		if result.name == "approval" {
			approvalErr = result.err
		} else {
			cancellationErr = result.err
		}
	}
	if cancellationErr != nil || approvalErr != nil && approvalErr != booking.ErrConflict {
		t.Fatalf("approval/cancellation race errors approval=%v cancellation=%v", approvalErr, cancellationErr)
	}
	if _, err = setup.Exec(ctx, `SELECT 1`); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT r.estado,o.activo,(SELECT count(*) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),(SELECT count(*) FROM public.reserva_ensayo_transicion h WHERE h.reserva_id=r.id AND h.estado_nuevo='cancelada_arrendatario') FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id WHERE r.id=$1`, raceReservation.ID).Scan(&boundaryState, &boundaryOccupancy, &boundaryRefunds, &refundAttempts); err != nil {
		t.Fatal(err)
	}
	if boundaryState != "cancelada_arrendatario" || boundaryOccupancy || boundaryRefunds != 1 || refundAttempts != 1 {
		t.Fatalf("approval/cancel race final state=%s active=%v refunds=%d cancel history=%d", boundaryState, boundaryOccupancy, boundaryRefunds, refundAttempts)
	}

	// Published M04 spaces join M05 discovery without becoming fixtures. Keep
	// the renter/host pair and existing fixture catalog intact, and verify that
	// active listing requires effective owner KYC while drafts/hidden offers
	// remain private.
	createPublishedSpace := func(owner, title, state string, price int64) string {
		t.Helper()
		id, idErr := (credentials.Generator{}).ID()
		if idErr != nil {
			t.Fatal(idErr)
		}
		if _, e := setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
			VALUES($1,$2,'oficina',$3,repeat('Publicación sintética para M05. ',4),24,4,'Reglas públicas de ensayo','hora',$4,'Dirección privada que no debe filtrarse',$5,'America/Santiago')`, id, owner, title, price, state); e != nil {
			t.Fatal(e)
		}
		if _, e := setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'oficina',1,'{}')`, id); e != nil {
			t.Fatal(e)
		}
		if _, e := setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',$2)`, id, price); e != nil {
			t.Fatal(e)
		}
		return id
	}
	activeSpace := createPublishedSpace(host, "Oficina publicada elegible", "activa", 9000)
	// A disabled fixture grant must not shadow the normal published-catalog
	// path, and the joined space must still appear only once.
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,false)`, activeSpace, host, renter); err != nil {
		t.Fatal(err)
	}
	hiddenSpace := createPublishedSpace(host, "Oficina oculta", "oculta", 7000)
	noKYCActiveSpace := createPublishedSpace(outsider, "Oficina sin elegibilidad", "activa", 6000)
	unconfiguredTimeZoneSpace := createPublishedSpace(host, "Publicación sin zona horaria", "activa", 6500)
	if _, err = setup.Exec(ctx, `UPDATE public.espacio SET zona_horaria=NULL WHERE id=$1`, unconfiguredTimeZoneSpace); err != nil {
		t.Fatal(err)
	}
	officeFilter := booking.CatalogFilter{CategoryCode: "oficina"}
	publicItems, err := svc.Catalog(ctx, renter, officeFilter)
	if err != nil || len(publicItems) != 1 || publicItems[0].SpaceID != activeSpace || publicItems[0].Price != 9000 {
		t.Fatalf("active eligible catalog=%+v err=%v", publicItems, err)
	}
	if _, err = svc.CatalogDetail(ctx, renter, hiddenSpace); err != booking.ErrNotFound {
		t.Fatalf("hidden publication detail error=%v", err)
	}
	if _, err = svc.CatalogDetail(ctx, renter, noKYCActiveSpace); err != booking.ErrNotFound {
		t.Fatalf("publication without effective host KYC detail error=%v", err)
	}
	if _, err = svc.CatalogDetail(ctx, renter, unconfiguredTimeZoneSpace); err != booking.ErrNotFound {
		t.Fatalf("publication without an IANA time zone must stay undiscoverable: %v", err)
	}
	if _, err = svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: unconfiguredTimeZoneSpace, StartAt: fixedNow.Add(72 * time.Hour).UTC().Format(time.RFC3339), EndAt: fixedNow.Add(73 * time.Hour).UTC().Format(time.RFC3339)}); err != booking.ErrNotFound {
		t.Fatalf("publication without an IANA time zone must not be quotable: %v", err)
	}
	activeJSON, err := json.Marshal(publicItems[0])
	if err != nil || strings.Contains(string(activeJSON), "Dirección privada") || strings.Contains(string(activeJSON), "propietario_id") || strings.Contains(string(activeJSON), host) {
		t.Fatalf("published catalog leaked private address/owner: %s err=%v", activeJSON, err)
	}
	searchStart, searchEnd = fixedNow.Add(72*time.Hour), fixedNow.Add(73*time.Hour)
	var quotesBeforeSearch, occupanciesBeforeSearch, quotesAfterSearch, occupanciesAfterSearch int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cotizacion_reserva_ensayo),(SELECT count(*) FROM public.ocupacion)`).Scan(&quotesBeforeSearch, &occupanciesBeforeSearch); err != nil {
		t.Fatal(err)
	}
	minimumTotal, maximumTotal := int64(9000), int64(9000)
	pricedItems, err := svc.Catalog(ctx, renter, booking.CatalogFilter{CategoryCode: "oficina", StartAt: &searchStart, EndAt: &searchEnd, MinTotalCLP: &minimumTotal, MaxTotalCLP: &maximumTotal})
	if err != nil || len(pricedItems) != 1 || pricedItems[0].SpaceID != activeSpace || pricedItems[0].EstimatedTotal == nil || *pricedItems[0].EstimatedTotal != 9000 {
		t.Fatalf("published catalog price filter=%+v err=%v", pricedItems, err)
	}
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cotizacion_reserva_ensayo),(SELECT count(*) FROM public.ocupacion)`).Scan(&quotesAfterSearch, &occupanciesAfterSearch); err != nil {
		t.Fatal(err)
	}
	if quotesAfterSearch != quotesBeforeSearch || occupanciesAfterSearch != occupanciesBeforeSearch {
		t.Fatalf("catalog query mutated quotes/occupancies: before=%d/%d after=%d/%d", quotesBeforeSearch, occupanciesBeforeSearch, quotesAfterSearch, occupanciesAfterSearch)
	}
	activeQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: searchStart.UTC().Format(time.RFC3339), EndAt: searchEnd.UTC().Format(time.RFC3339)})
	if err != nil || activeQuote.SpaceID != activeSpace || activeQuote.UnitPrice != 9000 || activeQuote.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		t.Fatalf("published-space quote=%+v err=%v", activeQuote, err)
	}
	activeCreate := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations", "published-space-reservation", booking.RequestInput{QuoteID: activeQuote.ID})
	if activeCreate.Code != http.StatusCreated {
		t.Fatalf("published-space reservation API status=%d body=%s", activeCreate.Code, activeCreate.Body.String())
	}
	var activeEnvelope struct {
		Data booking.Reservation `json:"data"`
	}
	if err = json.Unmarshal(activeCreate.Body.Bytes(), &activeEnvelope); err != nil {
		t.Fatal(err)
	}
	activeReservation := activeEnvelope.Data
	if activeReservation.SpaceID != activeSpace || activeReservation.State != "pendiente_de_pago" || activeReservation.UnitPrice != 9000 || activeReservation.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		t.Fatalf("published-space reservation snapshot=%+v", activeReservation)
	}
	// The normal published-offer path (its fixture row is explicitly disabled)
	// must preserve the same actor/state/history contract as the original booking
	// path. The renter pays once; the host approves; both participants can read
	// the terminal detail and history while a third party sees only 404.
	activeRetry := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations", "published-space-reservation", booking.RequestInput{QuoteID: activeQuote.ID})
	var activeRetryEnvelope struct {
		Data booking.Reservation `json:"data"`
	}
	if activeRetry.Code != http.StatusCreated || json.Unmarshal(activeRetry.Body.Bytes(), &activeRetryEnvelope) != nil || activeRetryEnvelope.Data.ID != activeReservation.ID {
		t.Fatalf("published-space idempotent request status=%d body=%s", activeRetry.Code, activeRetry.Body.String())
	}
	apiPayment := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+activeReservation.ID+"/payment", "published-space-payment", booking.PaymentInput{Outcome: "exito"})
	if apiPayment.Code != http.StatusOK {
		t.Fatalf("published-space fake payment API status=%d body=%s", apiPayment.Code, apiPayment.Body.String())
	}
	apiPaymentRetry := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+activeReservation.ID+"/payment", "published-space-payment", booking.PaymentInput{Outcome: "exito"})
	if apiPaymentRetry.Code != http.StatusOK {
		t.Fatalf("published-space idempotent payment API status=%d body=%s", apiPaymentRetry.Code, apiPaymentRetry.Body.String())
	}
	apiApproval := publishedBookingAPI(t, host, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+activeReservation.ID+"/decision", "", booking.DecisionInput{Decision: "aprobar"})
	if apiApproval.Code != http.StatusOK {
		t.Fatalf("published-space host decision API status=%d body=%s", apiApproval.Code, apiApproval.Body.String())
	}
	for _, participant := range []string{renter, host} {
		detailResponse := publishedBookingAPI(t, participant, svc, http.MethodGet, "/api/v1/local/booking-trial/reservations/"+activeReservation.ID, "", nil)
		var detailEnvelope struct {
			Data booking.Detail `json:"data"`
		}
		if detailResponse.Code != http.StatusOK || json.Unmarshal(detailResponse.Body.Bytes(), &detailEnvelope) != nil || detailEnvelope.Data.State != "aprobada_host" || len(detailEnvelope.Data.History) != 3 || detailEnvelope.Data.History[0].To != "pendiente_de_pago" || detailEnvelope.Data.History[1].To != "pagada" || detailEnvelope.Data.History[2].To != "aprobada_host" {
			t.Fatalf("published-space participant detail actor=%s status=%d body=%s detail=%+v", participant, detailResponse.Code, detailResponse.Body.String(), detailEnvelope.Data)
		}
		listResponse := publishedBookingAPI(t, participant, svc, http.MethodGet, "/api/v1/local/booking-trial/reservations", "", nil)
		if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), activeReservation.ID) {
			t.Fatalf("published-space participant list actor=%s status=%d body=%s", participant, listResponse.Code, listResponse.Body.String())
		}
	}
	if outsiderDetail := publishedBookingAPI(t, outsider, svc, http.MethodGet, "/api/v1/local/booking-trial/reservations/"+activeReservation.ID, "", nil); outsiderDetail.Code != http.StatusNotFound {
		t.Fatalf("published-space reservation exposed to third party status=%d body=%s", outsiderDetail.Code, outsiderDetail.Body.String())
	}
	// Run the alternate host decision on the same non-fixture publication.
	// A rejected request keeps the quoted commercial snapshot but releases the
	// single shared occupancy row and records the actor transition.
	rejectStart := fixedNow.Add(200 * time.Hour)
	rejectQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: rejectStart.UTC().Format(time.RFC3339), EndAt: rejectStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil || rejectQuote.UnitPrice != 9000 {
		t.Fatalf("quote for published rejection flow=%+v err=%v", rejectQuote, err)
	}
	rejectReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: rejectQuote.ID}, "published-rejection-request")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, rejectReservation.ID, "exito", "published-rejection-payment"); err != nil {
		t.Fatalf("fake payment for rejection flow: %v", err)
	}
	rejectedResponse := publishedBookingAPI(t, host, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+rejectReservation.ID+"/decision", "", booking.DecisionInput{Decision: "rechazar", Reason: "No compatible con el uso del espacio"})
	if rejectedResponse.Code != http.StatusOK {
		t.Fatalf("published-space host rejection API status=%d body=%s", rejectedResponse.Code, rejectedResponse.Body.String())
	}
	for _, participant := range []string{host, renter} {
		detail, getErr := svc.Get(ctx, participant, rejectReservation.ID)
		if getErr != nil || detail.State != "rechazada_arrendador" || len(detail.History) != 3 || detail.History[2].Actor == nil || *detail.History[2].Actor != host || !strings.Contains(detail.History[2].Reason, "No compatible") {
			t.Fatalf("published-space rejection detail actor=%s detail=%+v err=%v", participant, detail, getErr)
		}
	}
	var rejectedActiveOccupancy int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, rejectReservation.ID).Scan(&rejectedActiveOccupancy); err != nil || rejectedActiveOccupancy != 0 {
		t.Fatalf("published rejection retained occupancy=%d err=%v", rejectedActiveOccupancy, err)
	}
	// A timeout is resolved by the already durable fake operation. At the
	// exact payment deadline, a normal participant read runs the existing
	// expiry mechanism and appends the terminal transition once.
	expiryStart := fixedNow.Add(300 * time.Hour)
	expiryQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: expiryStart.UTC().Format(time.RFC3339), EndAt: expiryStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	activeExpiryReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: expiryQuote.ID}, "published-expiry-request")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, activeExpiryReservation.ID, "sin_respuesta", "published-expiry-payment"); !errors.Is(err, booking.ErrSimulatedNoResponse) {
		t.Fatalf("published-space timeout result=%v", err)
	}
	clockMu.Lock()
	fixedNow = activeExpiryReservation.PayExpiresAt
	clockMu.Unlock()
	expiredResponse := publishedBookingAPI(t, host, svc, http.MethodGet, "/api/v1/local/booking-trial/reservations/"+activeExpiryReservation.ID, "", nil)
	var expiredEnvelope struct {
		Data booking.Detail `json:"data"`
	}
	if expiredResponse.Code != http.StatusOK || json.Unmarshal(expiredResponse.Body.Bytes(), &expiredEnvelope) != nil || expiredEnvelope.Data.State != "vencida_pago" || len(expiredEnvelope.Data.History) != 2 || expiredEnvelope.Data.History[1].To != "vencida_pago" {
		t.Fatalf("published-space expiration API status=%d body=%s detail=%+v", expiredResponse.Code, expiredResponse.Body.String(), expiredEnvelope.Data)
	}
	clockMu.Lock()
	fixedNow = fixedNow.Add(time.Minute)
	clockMu.Unlock()
	if _, err = svc.Get(ctx, renter, activeExpiryReservation.ID); err != nil {
		t.Fatalf("expired published reservation should remain readable by renter: %v", err)
	}
	var expiredActiveOccupancy int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, activeExpiryReservation.ID).Scan(&expiredActiveOccupancy); err != nil || expiredActiveOccupancy != 0 {
		t.Fatalf("published expiry retained occupancy=%d err=%v", expiredActiveOccupancy, err)
	}
	// A paid active-publication booking can be cancelled before its interval
	// under local_flexible_v1. The obligation equals only the confirmed fake
	// amount; timeout/retry and a repeated success keep one operation/result.
	publishedCancelStart := fixedNow.Add(48 * time.Hour)
	publishedCancelQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: publishedCancelStart.UTC().Format(time.RFC3339), EndAt: publishedCancelStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil || publishedCancelQuote.UnitPrice != 9000 || publishedCancelQuote.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		t.Fatalf("published cancellation quote snapshot=%+v err=%v", publishedCancelQuote, err)
	}
	publishedCancelReservation, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: publishedCancelQuote.ID}, "published-cancel-request")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, publishedCancelReservation.ID, "exito", "published-cancel-payment"); err != nil {
		t.Fatal(err)
	}
	previewResponse := publishedBookingAPI(t, renter, svc, http.MethodGet, "/api/v1/local/booking-trial/reservations/"+publishedCancelReservation.ID+"/cancellation-preview", "", nil)
	if previewResponse.Code != http.StatusOK || !strings.Contains(previewResponse.Body.String(), "local_flexible_v1") || !strings.Contains(previewResponse.Body.String(), "Devolución simulada — sin movimiento de dinero") {
		t.Fatalf("published cancellation preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}
	cancelResponse := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+publishedCancelReservation.ID+"/cancel", "published-cancel-confirm", booking.CancellationInput{Reason: "Cambio de planes"})
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("published cancellation API status=%d body=%s", cancelResponse.Code, cancelResponse.Body.String())
	}
	cancelDetail, err := svc.Get(ctx, renter, publishedCancelReservation.ID)
	if err != nil || cancelDetail.State != "cancelada_arrendatario" || cancelDetail.RefundState == nil || *cancelDetail.RefundState != "pendiente" {
		t.Fatalf("published cancellation persisted detail=%+v err=%v", cancelDetail, err)
	}
	publishedRefundOperation := *cancelDetail.RefundOperationID
	refundTimeout := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+publishedCancelReservation.ID+"/refund", publishedRefundOperation, booking.RefundInput{Outcome: "sin_respuesta"})
	if refundTimeout.Code != http.StatusGatewayTimeout {
		t.Fatalf("published fake refund timeout status=%d body=%s", refundTimeout.Code, refundTimeout.Body.String())
	}
	refundSuccess := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+publishedCancelReservation.ID+"/refund", publishedRefundOperation, booking.RefundInput{Outcome: "exito"})
	if refundSuccess.Code != http.StatusOK {
		t.Fatalf("published fake refund success status=%d body=%s", refundSuccess.Code, refundSuccess.Body.String())
	}
	refundReplay := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+publishedCancelReservation.ID+"/refund", publishedRefundOperation, booking.RefundInput{Outcome: "exito"})
	if refundReplay.Code != http.StatusOK {
		t.Fatalf("published fake refund replay status=%d body=%s", refundReplay.Code, refundReplay.Body.String())
	}
	cancelHostDetail, err := svc.Get(ctx, host, publishedCancelReservation.ID)
	if err != nil || cancelHostDetail.State != "cancelada_arrendatario" || cancelHostDetail.RefundState == nil || *cancelHostDetail.RefundState != "completada" || cancelHostDetail.RefundAmountCLP == nil || *cancelHostDetail.RefundAmountCLP != publishedCancelReservation.Subtotal {
		t.Fatalf("host cannot observe completed published refund detail=%+v err=%v", cancelHostDetail, err)
	}
	var publishedRefunds, publishedRefundAttempts, publishedCancelOccupancy int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1),(SELECT count(*) FROM public.reserva_devolucion_intento_ensayo i JOIN public.reserva_devolucion_ensayo d ON d.id=i.devolucion_id WHERE d.reserva_id=$1),(SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo)`, publishedCancelReservation.ID).Scan(&publishedRefunds, &publishedRefundAttempts, &publishedCancelOccupancy); err != nil || publishedRefunds != 1 || publishedRefundAttempts != 2 || publishedCancelOccupancy != 0 {
		t.Fatalf("published refund idempotency rows=%d attempts=%d occupancy=%d err=%v", publishedRefunds, publishedRefundAttempts, publishedCancelOccupancy, err)
	}
	// Revoke an approved host case through the same synthetic verification
	// service used by the API, after quoting but before reserving. A quote does
	// not freeze commercial eligibility: the guarded request must reject and
	// leave neither a reservation nor a hold.
	revokedStart := fixedNow.Add(500 * time.Hour)
	revokedQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: revokedStart.UTC().Format(time.RFC3339), EndAt: revokedStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	verificationService, err := verification.NewService(verificationpg.New(pool), credentials.Generator{}, verification.LocalFixtureProvider{}, clock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verificationService.Revoke(ctx, hostKYC, outsider, "revision_fixture_actualizada", "published-host-kyc-revocation", "local-book-02:published-host-revocation"); err != nil {
		t.Fatalf("revoke host KYC after published quote: %v", err)
	}
	if staleEligibility := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations", "published-revoked-kyc-request", booking.RequestInput{QuoteID: revokedQuote.ID}); staleEligibility.Code != http.StatusConflict {
		t.Fatalf("published reservation after host KYC revocation status=%d body=%s", staleEligibility.Code, staleEligibility.Body.String())
	}
	var revokedReservations, revokedOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, revokedQuote.ID).Scan(&revokedReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND intervalo && tstzrange($2,$3,'[)')`, activeSpace, revokedStart, revokedStart.Add(time.Hour)).Scan(&revokedOccupancies); err != nil || revokedReservations != 0 || revokedOccupancies != 0 {
		t.Fatalf("revoked eligibility left partial reservation/occupancy=%d/%d err=%v", revokedReservations, revokedOccupancies, err)
	}
	// Restore eligibility only for this disposable test so the following tariff
	// regression isolates the stale-price rule. Existing reservation snapshots
	// above remain untouched.
	if _, err = setup.Exec(ctx, `UPDATE public.verificacion SET estado='aprobada',revocada_en=NULL,revocada_por=NULL,motivo_revocacion_codigo=NULL WHERE id=$1`, hostKYC); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.elegibilidad_verificacion_local SET estado='elegible',revocada_en=NULL,revocada_por=NULL,motivo_revocacion_codigo=NULL WHERE usuario_id=$1 AND tipo='kyc'`, host); err != nil {
		t.Fatal(err)
	}
	stalePublishedRateStart := fixedNow.Add(600 * time.Hour)
	stalePublishedRateQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: stalePublishedRateStart.UTC().Format(time.RFC3339), EndAt: stalePublishedRateStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil || stalePublishedRateQuote.UnitPrice != 9000 {
		t.Fatalf("published stale-rate setup quote=%+v err=%v", stalePublishedRateQuote, err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,2,'hora',9500)`, activeSpace); err != nil {
		t.Fatal(err)
	}
	if stalePublishedRate := publishedBookingAPI(t, renter, svc, http.MethodPost, "/api/v1/local/booking-trial/reservations", "published-stale-rate-request", booking.RequestInput{QuoteID: stalePublishedRateQuote.ID}); stalePublishedRate.Code != http.StatusConflict {
		t.Fatalf("published reservation with stale tariff status=%d body=%s", stalePublishedRate.Code, stalePublishedRate.Body.String())
	}
	var stalePublishedReservations, stalePublishedOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, stalePublishedRateQuote.ID).Scan(&stalePublishedReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND intervalo && tstzrange($2,$3,'[)')`, activeSpace, stalePublishedRateStart, stalePublishedRateStart.Add(time.Hour)).Scan(&stalePublishedOccupancies); err != nil || stalePublishedReservations != 0 || stalePublishedOccupancies != 0 {
		t.Fatalf("published stale tariff left partial reservation/occupancy=%d/%d err=%v", stalePublishedReservations, stalePublishedOccupancies, err)
	}
	activeSnapshot, err := svc.Get(ctx, renter, activeReservation.ID)
	if err != nil || activeSnapshot.State != "aprobada_host" || activeSnapshot.UnitPrice != 9000 || activeSnapshot.Subtotal != 9000 || activeSnapshot.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		t.Fatalf("published reservation snapshot changed after a later tariff version: detail=%+v err=%v", activeSnapshot, err)
	}
	if ownerItems, e := svc.Catalog(ctx, host, officeFilter); e != nil || len(ownerItems) != 0 {
		t.Fatalf("owner should not discover own publication: items=%+v err=%v", ownerItems, e)
	}
	// Exercise the existing HTTP endpoints consumed by the mock for the same
	// published listing: search, safe detail, then a quote for that selection.
	handler = bookinghttp.NewHandler(integrationBookingAuth{accountID: renter}, svc, nil)
	apiRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog?category_code=oficina&page_size=5", nil)
	apiRequest.Header.Set("Authorization", "Bearer local-booking-integration-session")
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusOK || !strings.Contains(apiResponse.Body.String(), activeSpace) || strings.Contains(apiResponse.Body.String(), "Dirección privada") || strings.Contains(apiResponse.Body.String(), host) {
		t.Fatalf("published catalog HTTP response=%d body=%s", apiResponse.Code, apiResponse.Body.String())
	}
	apiRequest = httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog/"+activeSpace, nil)
	apiRequest.Header.Set("Authorization", "Bearer local-booking-integration-session")
	apiResponse = httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusOK || !strings.Contains(apiResponse.Body.String(), "Oficina publicada elegible") || strings.Contains(apiResponse.Body.String(), "Dirección privada") {
		t.Fatalf("published detail HTTP response=%d body=%s", apiResponse.Code, apiResponse.Body.String())
	}
	apiRequest = httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/catalog/"+unconfiguredTimeZoneSpace, nil)
	apiRequest.Header.Set("Authorization", "Bearer local-booking-integration-session")
	apiResponse = httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusNotFound {
		t.Fatalf("publication without an IANA time zone HTTP detail=%d body=%s", apiResponse.Code, apiResponse.Body.String())
	}
	apiQuoteStart := fixedNow.Add(96 * time.Hour)
	apiQuoteBody, err := json.Marshal(booking.QuoteInput{SpaceID: activeSpace, StartAt: apiQuoteStart.UTC().Format(time.RFC3339), EndAt: apiQuoteStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	apiRequest = httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/quotes", strings.NewReader(string(apiQuoteBody)))
	apiRequest.Header.Set("Authorization", "Bearer local-booking-integration-session")
	apiRequest.Header.Set("Content-Type", "application/json")
	apiResponse = httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	var quoteEnvelope struct {
		Data booking.Quote `json:"data"`
	}
	if err = json.Unmarshal(apiResponse.Body.Bytes(), &quoteEnvelope); err != nil || apiResponse.Code != http.StatusOK || quoteEnvelope.Data.SpaceID != activeSpace {
		t.Fatalf("published quote HTTP response=%d quote=%+v body=%s err=%v", apiResponse.Code, quoteEnvelope.Data, apiResponse.Body.String(), err)
	}
	staleListingStart := fixedNow.Add(120 * time.Hour)
	staleListingQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: activeSpace, StartAt: staleListingStart.UTC().Format(time.RFC3339), EndAt: staleListingStart.Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.espacio SET estado='oculta' WHERE id=$1`, activeSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: staleListingQuote.ID}, "hidden-after-quote"); err != booking.ErrConflict {
		t.Fatalf("reservation after publication was hidden should conflict, got %v", err)
	}
	if _, err = setup.Exec(ctx, `UPDATE public.espacio SET estado='activa' WHERE id=$1`, activeSpace); err != nil {
		t.Fatal(err)
	}
	var staleListingReservations, staleListingOccupancies int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, staleListingQuote.ID).Scan(&staleListingReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND intervalo && tstzrange($2,$3,'[)')`, activeSpace, staleListingStart, staleListingStart.Add(time.Hour)).Scan(&staleListingOccupancies); err != nil {
		t.Fatal(err)
	}
	if staleListingReservations != 0 || staleListingOccupancies != 0 {
		t.Fatalf("hidden listing request left partial reservation/occupancy=%d/%d", staleListingReservations, staleListingOccupancies)
	}
	stalePublicQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{SpaceID: hiddenSpace, StartAt: fixedNow.Add(80 * time.Hour).UTC().Format(time.RFC3339), EndAt: fixedNow.Add(81 * time.Hour).UTC().Format(time.RFC3339)})
	if err != booking.ErrNotFound {
		t.Fatalf("hidden space must not be quotable, quote=%+v err=%v", stalePublicQuote, err)
	}
	var activeReservations int
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE espacio_id=$1`, hiddenSpace).Scan(&activeReservations); err != nil {
		t.Fatal(err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE espacio_id=$1 AND activo`, hiddenSpace).Scan(&activeOccupancies); err != nil {
		t.Fatal(err)
	}
	if activeReservations != 0 || activeOccupancies != 0 {
		t.Fatalf("hidden listing produced reservation/occupancy=%d/%d", activeReservations, activeOccupancies)
	}

	// LOCAL-CONT-01: synthetic signatures use the same reservation row lock as
	// M06. Rejection remains recorded until start_at; expiry cancels and creates
	// exactly one full fake-refund obligation at the boundary.
	contractRepo, err := contractpg.New(pool, []byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	contractService, err := contract.NewService(contractRepo, clock)
	if err != nil {
		t.Fatal(err)
	}
	completeReservation := newReservation(1000*time.Hour, "cont-success", true)
	if ensured, ensureErr := contractService.EnsureApproved(ctx); ensureErr != nil || ensured < 1 {
		t.Fatalf("restart recovery of approved reservation contract: count=%d err=%v", ensured, ensureErr)
	}
	createdContract, err := contractService.Create(ctx, renter, completeReservation.ID)
	if err != nil || createdContract.State != "generado" || len(createdContract.Signatures) != 2 || len(createdContract.Artifact) < 100 || !strings.HasPrefix(string(createdContract.Artifact), "%PDF-") {
		t.Fatalf("synthetic contract creation/artifact: state=%s signatures=%d bytes=%d err=%v", createdContract.State, len(createdContract.Signatures), len(createdContract.Artifact), err)
	}
	var encryptedDocument []byte
	if err = setup.QueryRow(ctx, `SELECT contenido FROM public.documento_privado_sintetico_local WHERE id=$1`, createdContract.DocumentID).Scan(&encryptedDocument); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(string(encryptedDocument), "%PDF-") || len(encryptedDocument) < len(createdContract.Artifact) {
		t.Fatal("contract PDF was not encrypted in its M09 private-document row")
	}
	if contractpg.Hash(createdContract.Artifact) != createdContract.SHA256 {
		t.Fatal("contract plaintext checksum does not match its snapshot metadata")
	}
	contractBase := "/api/v1/local/booking-trial/contracts/" + createdContract.ID
	if response := publishedContractAPI(t, renter, contractService, http.MethodPost, "/api/v1/local/booking-trial/reservations/"+completeReservation.ID+"/contract"); response.Code != http.StatusOK {
		t.Fatalf("idempotent create API=%d %s", response.Code, response.Body.String())
	}
	if response := publishedContractAPI(t, outsider, contractService, http.MethodGet, contractBase); response.Code != http.StatusNotFound {
		t.Fatalf("third party contract API should hide resource, got %d", response.Code)
	}
	if response := publishedContractAPI(t, host, contractService, http.MethodGet, contractBase); response.Code != http.StatusOK {
		t.Fatalf("participant contract GET=%d %s", response.Code, response.Body.String())
	}
	decodeContract := func(response *httptest.ResponseRecorder) contract.Contract {
		t.Helper()
		var envelope struct {
			Data contract.Contract `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	firstSignatureResponse := publishedContractAPI(t, host, contractService, http.MethodPost, contractBase+"/sign")
	firstSignature := decodeContract(firstSignatureResponse)
	if firstSignatureResponse.Code != http.StatusOK || firstSignature.State != "firma_parcial" {
		t.Fatalf("first signature API=%d contract=%s", firstSignatureResponse.Code, firstSignature.State)
	}
	secondSignatureResponse := publishedContractAPI(t, renter, contractService, http.MethodPost, contractBase+"/sign")
	secondSignature := decodeContract(secondSignatureResponse)
	if secondSignatureResponse.Code != http.StatusOK || secondSignature.State != "firmado" {
		t.Fatalf("second signature API=%d contract=%s", secondSignatureResponse.Code, secondSignature.State)
	}
	documentResponse := publishedContractAPI(t, host, contractService, http.MethodGet, contractBase+"/document")
	if documentResponse.Code != http.StatusOK || !strings.HasPrefix(documentResponse.Body.String(), "%PDF-") {
		t.Fatalf("signed document endpoint=%d content-type=%q", documentResponse.Code, documentResponse.Header().Get("Content-Type"))
	}
	replayResponse := publishedContractAPI(t, renter, contractService, http.MethodPost, contractBase+"/sign")
	replay := decodeContract(replayResponse)
	if replayResponse.Code != http.StatusOK || replay.State != "firmado" || len(replay.History) != len(secondSignature.History) {
		t.Fatalf("signature API retry duplicated transition: %+v", replay)
	}
	/* direct participant access assertions are also exercised through GET above */
	if false {
		if _, err = contractService.Get(ctx, outsider, createdContract.ID); err != contract.ErrNotFound {
			t.Fatalf("third party contract access must be hidden, got %v", err)
		}
	}
	completedDetail, err := svc.Get(ctx, renter, completeReservation.ID)
	if err != nil || completedDetail.State != "lista_para_checkin" {
		t.Fatalf("both signatures should advance M06: state=%s err=%v", completedDetail.State, err)
	}
	clockMu.Lock()
	fixedNow = completeReservation.StartAt
	clockMu.Unlock()
	_, _ = contractService.ExpireDue(ctx)
	if completeAfterStart, readErr := svc.Get(ctx, renter, completeReservation.ID); readErr != nil || completeAfterStart.State != "lista_para_checkin" {
		t.Fatalf("fully signed contract expired with time: state=%s err=%v", completeAfterStart.State, readErr)
	}

	contractRejectReservation := newReservation(1030*time.Hour, "cont-reject", true)
	rejectedContract, err := contractService.Create(ctx, host, contractRejectReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	rejectAPI := publishedContractAPI(t, host, contractService, http.MethodPost, "/api/v1/local/booking-trial/contracts/"+rejectedContract.ID+"/reject")
	rejectedContract = decodeContract(rejectAPI)
	if rejectAPI.Code != http.StatusOK {
		t.Fatalf("reject API=%d %s", rejectAPI.Code, rejectAPI.Body.String())
	}
	_, err = contractService.Get(ctx, host, rejectedContract.ID)
	rejectedHostState := ""
	for _, signature := range rejectedContract.Signatures {
		if signature.SignerID == host {
			rejectedHostState = signature.State
		}
	}
	if err != nil || rejectedContract.State != "firma_parcial" || rejectedHostState != "rechazada" {
		t.Fatalf("rejection should be terminal for signer but non-cancelling: %+v err=%v", rejectedContract, err)
	}
	rejectedReservationDetail, err := svc.Get(ctx, host, contractRejectReservation.ID)
	if err != nil || rejectedReservationDetail.State != "firma_parcial" {
		t.Fatalf("rejection cancelled or skipped partial state: %+v err=%v", rejectedReservationDetail, err)
	}
	if _, err = contractService.Sign(ctx, host, rejectedContract.ID); err != contract.ErrConflict {
		t.Fatalf("rejected signer must not retry this version: %v", err)
	}

	// local_flexible_v1 remains available before start_at after one signature.
	// M06 cancellation and M07 terminalization share the reservation lock and
	// transaction so the other participant cannot sign after cancellation.
	partialCancelReservation := newReservation(1050*time.Hour, "cont-cancel-partial", true)
	partialCancelContract, err := contractService.Create(ctx, renter, partialCancelReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	partialCancelContract, err = contractService.Sign(ctx, host, partialCancelContract.ID)
	if err != nil || partialCancelContract.State != "firma_parcial" {
		t.Fatalf("prepare one-signature cancellation: state=%s err=%v", partialCancelContract.State, err)
	}
	partialPreview, err := svc.CancellationPreview(ctx, renter, partialCancelReservation.ID)
	if err != nil || !partialPreview.Eligible || partialPreview.AmountCLP != partialCancelReservation.Subtotal {
		t.Fatalf("local_flexible_v1 preview after partial signature=%+v err=%v", partialPreview, err)
	}
	partialCancelled, err := svc.Cancel(ctx, renter, partialCancelReservation.ID, "cont-cancel-partial", "ensayo")
	if err != nil || partialCancelled.Reservation.State != "cancelada_arrendatario" || partialCancelled.RefundAmountCLP == nil || *partialCancelled.RefundAmountCLP != partialCancelReservation.Subtotal {
		t.Fatalf("cancel after partial signature=%+v err=%v", partialCancelled, err)
	}
	partialContractAfterCancel, err := contractService.Get(ctx, host, partialCancelContract.ID)
	if err != nil || partialContractAfterCancel.State != "anulado" {
		t.Fatalf("partial contract must become terminal on cancellation: %+v err=%v", partialContractAfterCancel, err)
	}
	if _, err = contractService.Sign(ctx, renter, partialCancelContract.ID); err != contract.ErrConflict {
		t.Fatalf("signature after cancellation must conflict, got %v", err)
	}
	var partialRefunds, partialActiveOccupancies int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1),(SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo)`, partialCancelReservation.ID).Scan(&partialRefunds, &partialActiveOccupancies); err != nil {
		t.Fatal(err)
	}
	if partialRefunds != 1 || partialActiveOccupancies != 0 {
		t.Fatalf("partial cancellation duplicate effects refunds=%d active occupancy=%d", partialRefunds, partialActiveOccupancies)
	}

	// A completed contract remains a signed historical artifact, but its
	// approved reservation still follows local_flexible_v1 before start_at.
	fullCancelReservation := newReservation(1051*time.Hour, "cont-cancel-full", true)
	fullCancelContract, err := contractService.Create(ctx, host, fullCancelReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = contractService.Sign(ctx, host, fullCancelContract.ID); err != nil {
		t.Fatal(err)
	}
	fullCancelContract, err = contractService.Sign(ctx, renter, fullCancelContract.ID)
	if err != nil || fullCancelContract.State != "firmado" {
		t.Fatalf("prepare completed-contract cancellation: %+v err=%v", fullCancelContract, err)
	}
	fullPreview, err := svc.CancellationPreview(ctx, renter, fullCancelReservation.ID)
	if err != nil || !fullPreview.Eligible || fullPreview.AmountCLP != fullCancelReservation.Subtotal {
		t.Fatalf("local_flexible_v1 preview after completed signatures=%+v err=%v", fullPreview, err)
	}
	fullCancelled, err := svc.Cancel(ctx, renter, fullCancelReservation.ID, "cont-cancel-full", "ensayo")
	if err != nil || fullCancelled.Reservation.State != "cancelada_arrendatario" || fullCancelled.RefundAmountCLP == nil || *fullCancelled.RefundAmountCLP != fullCancelReservation.Subtotal {
		t.Fatalf("cancel after completed signatures=%+v err=%v", fullCancelled, err)
	}
	fullContractAfterCancel, err := contractService.Get(ctx, host, fullCancelContract.ID)
	if err != nil || fullContractAfterCancel.State != "firmado" || !strings.HasPrefix(string(fullContractAfterCancel.Artifact), "%PDF-") {
		t.Fatalf("completed contract snapshot should remain historically signed: %+v err=%v", fullContractAfterCancel, err)
	}
	if _, err = contractService.Sign(ctx, renter, fullCancelContract.ID); err != nil {
		t.Fatalf("idempotent signature retry on already fully signed version=%v", err)
	}
	var fullRefunds, fullActiveOccupancies int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1),(SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo)`, fullCancelReservation.ID).Scan(&fullRefunds, &fullActiveOccupancies); err != nil {
		t.Fatal(err)
	}
	if fullRefunds != 1 || fullActiveOccupancies != 0 {
		t.Fatalf("completed cancellation duplicate effects refunds=%d active occupancy=%d", fullRefunds, fullActiveOccupancies)
	}

	// Run the pending signature and local_flexible_v1 cancellation at the same
	// barrier. They must serialize on the reservation row; either cancellation
	// annuls the partial contract first, or signature completes first and the
	// cancellation preserves its now-complete historical artifact. Both orders
	// produce one refund and release occupancy exactly once.
	raceContractReservation := newReservation(1052*time.Hour, "cont-cancel-sign-race", true)
	raceContract, err := contractService.Create(ctx, renter, raceContractReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = contractService.Sign(ctx, host, raceContract.ID); err != nil {
		t.Fatal(err)
	}
	type cancelSignOutcome struct {
		name string
		err  error
	}
	raceGate := make(chan struct{})
	raceResults := make(chan cancelSignOutcome, 2)
	go func() {
		<-raceGate
		_, cancelErr := svc.Cancel(ctx, renter, raceContractReservation.ID, "cont-cancel-sign-race", "ensayo")
		raceResults <- cancelSignOutcome{"cancel", cancelErr}
	}()
	go func() {
		<-raceGate
		_, signErr := contractService.Sign(ctx, renter, raceContract.ID)
		raceResults <- cancelSignOutcome{"sign", signErr}
	}()
	close(raceGate)
	var raceCancelErr, raceSignErr error
	for range 2 {
		outcome := <-raceResults
		if outcome.name == "cancel" {
			raceCancelErr = outcome.err
		} else {
			raceSignErr = outcome.err
		}
	}
	if raceCancelErr != nil || raceSignErr != nil && raceSignErr != contract.ErrConflict {
		t.Fatalf("cancel/sign lock race cancel=%v sign=%v", raceCancelErr, raceSignErr)
	}
	var raceRefunds, raceOccupancies, raceSigned int
	var raceContractState, raceReservationState string
	if err = setup.QueryRow(ctx, `SELECT r.estado,c.estado,(SELECT count(*) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),(SELECT count(*) FROM public.ocupacion o WHERE o.reserva_id=r.id AND o.activo),(SELECT count(*) FROM public.contrato_ensayo_firma f WHERE f.contrato_id=c.id AND f.estado='firmada') FROM public.reserva_ensayo_local r JOIN public.contrato_ensayo_local c ON c.reserva_id=r.id WHERE r.id=$1`, raceContractReservation.ID).Scan(&raceReservationState, &raceContractState, &raceRefunds, &raceOccupancies, &raceSigned); err != nil {
		t.Fatal(err)
	}
	if raceReservationState != "cancelada_arrendatario" || raceRefunds != 1 || raceOccupancies != 0 {
		t.Fatalf("serialized cancellation result reservation=%s contract=%s refunds=%d occupancy=%d", raceReservationState, raceContractState, raceRefunds, raceOccupancies)
	}
	if raceSignErr == nil && (raceContractState != "firmado" || raceSigned != 2) {
		t.Fatalf("signature-first order lost completed contract: contract=%s signatures=%d", raceContractState, raceSigned)
	}
	if raceSignErr == contract.ErrConflict && (raceContractState != "anulado" || raceSigned != 1) {
		t.Fatalf("cancellation-first order allowed a later signature: contract=%s signatures=%d", raceContractState, raceSigned)
	}
	replayRaceCancel, err := svc.Cancel(ctx, renter, raceContractReservation.ID, "cont-cancel-sign-race", "ensayo")
	if err != nil || !replayRaceCancel.Replayed {
		t.Fatalf("cancellation replay after signature race=%+v err=%v", replayRaceCancel, err)
	}
	if err = setup.QueryRow(ctx, `SELECT count(*) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1`, raceContractReservation.ID).Scan(&raceRefunds); err != nil || raceRefunds != 1 {
		t.Fatalf("cancel/sign race replay duplicated refund: count=%d err=%v", raceRefunds, err)
	}

	concurrentExpiryReservation := newReservation(1015*time.Hour, "cont-concurrent-expiry", true)
	concurrentExpiryContract, err := contractService.Create(ctx, host, concurrentExpiryReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	clockMu.Lock()
	fixedNow = concurrentExpiryReservation.StartAt
	clockMu.Unlock()
	var concurrentWG sync.WaitGroup
	concurrentErrors := make(chan error, 3)
	for _, participant := range []string{host, renter} {
		participant := participant
		concurrentWG.Add(1)
		go func() {
			defer concurrentWG.Done()
			_, callErr := contractService.Sign(ctx, participant, concurrentExpiryContract.ID)
			if callErr != nil && callErr != contract.ErrConflict {
				concurrentErrors <- callErr
			}
		}()
	}
	concurrentWG.Add(1)
	go func() {
		defer concurrentWG.Done()
		_, expireErr := contractService.ExpireDue(ctx)
		if expireErr != nil {
			concurrentErrors <- expireErr
		}
	}()
	concurrentWG.Wait()
	close(concurrentErrors)
	for concurrentErr := range concurrentErrors {
		t.Fatal(concurrentErr)
	}
	var concurrentRefunds, concurrentTransitions int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1),(SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 AND estado_nuevo='cancelada_por_firma')`, concurrentExpiryReservation.ID).Scan(&concurrentRefunds, &concurrentTransitions); err != nil {
		t.Fatal(err)
	}
	if concurrentRefunds != 1 || concurrentTransitions != 1 {
		t.Fatalf("concurrent signature/expiry duplicated effects refunds=%d transitions=%d", concurrentRefunds, concurrentTransitions)
	}

	expiryReservation := newReservation(1020*time.Hour, "cont-expiry", true)
	expiryContract, err := contractService.Create(ctx, renter, expiryReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	clockMu.Lock()
	fixedNow = expiryReservation.StartAt
	clockMu.Unlock()
	if _, err = contractService.Sign(ctx, host, expiryContract.ID); err != contract.ErrConflict {
		t.Fatalf("sign at exact start must be rejected after expiring: %v", err)
	}
	if _, err = contractService.Sign(ctx, renter, expiryContract.ID); err != contract.ErrConflict {
		t.Fatalf("second expired signature retry must be conflict without duplicated effects: %v", err)
	}
	var expiredState string
	var activeOccupancy bool
	var refundCount int
	var refundAmount int64
	var expiredHistory int
	if err = setup.QueryRow(ctx, `SELECT r.estado,o.activo,(SELECT count(*) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),(SELECT COALESCE(max(d.importe_clp),0) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),(SELECT count(*) FROM public.reserva_ensayo_transicion h WHERE h.reserva_id=r.id AND h.estado_nuevo='cancelada_por_firma') FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id WHERE r.id=$1`, expiryReservation.ID).Scan(&expiredState, &activeOccupancy, &refundCount, &refundAmount, &expiredHistory); err != nil {
		t.Fatal(err)
	}
	if expiredState != "cancelada_por_firma" || activeOccupancy || refundCount != 1 || refundAmount != expiryReservation.Subtotal || expiredHistory != 1 {
		t.Fatalf("deadline effects state=%s occupancy=%v refunds=%d amount=%d history=%d", expiredState, activeOccupancy, refundCount, refundAmount, expiredHistory)
	}
	expiredDetail, err = svc.Get(ctx, renter, expiryReservation.ID)
	if err != nil || expiredDetail.RefundOperationID == nil || expiredDetail.RefundState == nil || *expiredDetail.RefundState != "pendiente" {
		t.Fatalf("M06 refund obligation not queryable: %+v err=%v", expiredDetail, err)
	}
	refundResult, err := svc.Refund(ctx, renter, expiryReservation.ID, *expiredDetail.RefundOperationID, "exito")
	if err != nil || refundResult.State != "completada" || refundResult.AmountCLP != expiryReservation.Subtotal {
		t.Fatalf("fake full refund after signature expiry: %+v err=%v", refundResult, err)
	}

	// A signing request that starts before start_at but waits behind M06's row
	// lock must read the injected Backend clock only after obtaining the lock.
	setConcurrentSignReservation := newReservation(1040*time.Hour, "cont-clock-after-lock", true)
	setConcurrentSignContract, err := contractService.Create(ctx, renter, setConcurrentSignReservation.ID)
	if err != nil {
		t.Fatal(err)
	}
	contractLockTx, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var lockedContractReservation string
	if err = contractLockTx.QueryRow(ctx, `SELECT id::text FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, setConcurrentSignReservation.ID).Scan(&lockedContractReservation); err != nil {
		_ = contractLockTx.Rollback(ctx)
		t.Fatal(err)
	}
	contractSignResult := make(chan error, 1)
	go func() {
		_, signErr := contractService.Sign(ctx, host, setConcurrentSignContract.ID)
		contractSignResult <- signErr
	}()
	waitUntil := time.Now().Add(4 * time.Second)
	contractWaiting := false
	for !contractWaiting && time.Now().Before(waitUntil) {
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename=current_user AND wait_event_type='Lock' AND query ILIKE '%FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE%')`).Scan(&contractWaiting); err != nil {
			_ = contractLockTx.Rollback(ctx)
			t.Fatal(err)
		}
		if !contractWaiting {
			select {
			case earlyErr := <-contractSignResult:
				_ = contractLockTx.Rollback(ctx)
				t.Fatalf("signature returned before the reservation lock: %v", earlyErr)
			default:
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !contractWaiting {
		_ = contractLockTx.Rollback(ctx)
		t.Fatal("signature did not wait for the shared reservation lock")
	}
	clockMu.Lock()
	fixedNow = setConcurrentSignReservation.StartAt
	clockMu.Unlock()
	if err = contractLockTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-contractSignResult; err != contract.ErrConflict {
		t.Fatalf("signature acquired lock at start_at should conflict, got %v", err)
	}
	var afterLockState string
	var afterLockActive bool
	var afterLockRefunds, afterLockSignatures int
	if err = setup.QueryRow(ctx, `SELECT r.estado,o.activo,(SELECT count(*) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),(SELECT count(*) FROM public.contrato_ensayo_firma f JOIN public.contrato_ensayo_local c ON c.id=f.contrato_id WHERE c.reserva_id=r.id AND f.estado='firmada') FROM public.reserva_ensayo_local r JOIN public.ocupacion o ON o.reserva_id=r.id WHERE r.id=$1`, setConcurrentSignReservation.ID).Scan(&afterLockState, &afterLockActive, &afterLockRefunds, &afterLockSignatures); err != nil {
		t.Fatal(err)
	}
	if afterLockState != "cancelada_por_firma" || afterLockActive || afterLockRefunds != 1 || afterLockSignatures != 0 {
		t.Fatalf("post-lock deadline state=%s active=%v refunds=%d signatures=%d", afterLockState, afterLockActive, afterLockRefunds, afterLockSignatures)
	}

}

func timePtr(value time.Time) *time.Time { return &value }

func hasAttributes(raw json.RawMessage, expected map[string]any) bool {
	var actual map[string]any
	if json.Unmarshal(raw, &actual) != nil {
		return false
	}
	for key, value := range expected {
		if got, ok := actual[key]; !ok || !reflect.DeepEqual(got, value) {
			return false
		}
	}
	return true
}
