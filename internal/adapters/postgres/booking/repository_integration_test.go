package bookingpg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
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
	t.Cleanup(func() {
		cleanCtx, cc := context.WithTimeout(context.Background(), 20*time.Second)
		defer cc()
		_, _ = admin.Exec(cleanCtx, `DROP DATABASE `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
		_, _ = admin.Exec(cleanCtx, `DROP ROLE IF EXISTS espacigo_runtime`)
	})
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dbURL := u.String()
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
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(singleton,espacio_id,anfitrion_id,arrendatario_id) VALUES(true,$1,$2,$3)`, space, host, renter); err != nil {
		t.Fatal(err)
	}
	repo := New(pool)
	fixedNow := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return fixedNow }
	svc, err := booking.NewService(repo, credentials.Generator{}, clock, fakebooking.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, startAt := range []time.Time{fixedNow.Add(-time.Second), fixedNow} {
		_, err = svc.Quote(ctx, renter, booking.QuoteInput{StartAt: startAt.Format(time.RFC3339Nano), EndAt: startAt.Add(time.Hour).Format(time.RFC3339Nano)})
		if err != booking.ErrInvalid {
			t.Fatalf("quote start %s should be rejected at backend time %s: %v", startAt, fixedNow, err)
		}
	}
	if _, err = svc.Fixture(ctx, outsider); err != booking.ErrNotFound {
		t.Fatalf("unlisted fixture visible: %v", err)
	}
	staleStart := fixedNow.Add(5 * time.Minute)
	staleQuote, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: staleStart.Format(time.RFC3339Nano), EndAt: staleStart.Add(time.Hour).Format(time.RFC3339Nano)})
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
	quote, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: start.Format(time.RFC3339), EndAt: start.Add(90 * time.Minute).Format(time.RFC3339)})
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
	if _, err = svc.Get(ctx, outsider, results[0].ID); err != booking.ErrNotFound {
		t.Fatalf("outsider queried reservation: %v", err)
	}
	if _, err = svc.Pay(ctx, renter, results[0].ID, "exito", "payment-one"); err != nil {
		t.Fatal(err)
	}
	approved, err := svc.Decide(ctx, host, results[0].ID, "aprobar")
	if err != nil || approved.State != "aprobada_host" {
		t.Fatalf("approval: %+v %v", approved, err)
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
	quote2, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: start.Add(3 * time.Hour).Format(time.RFC3339), EndAt: start.Add(4 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := svc.Request(ctx, renter, booking.RequestInput{QuoteID: quote2.ID}, "other-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Pay(ctx, renter, retry.ID, "exito", "payment-two"); err != nil {
		t.Fatal(err)
	}
	rejected, err := svc.Decide(ctx, host, retry.ID, "rechazar")
	if err != nil || rejected.State != "rechazada_arrendador" {
		t.Fatalf("rejection: %+v %v", rejected, err)
	}
	quote3, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: start.Add(5 * time.Hour).Format(time.RFC3339), EndAt: start.Add(6 * time.Hour).Format(time.RFC3339)})
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
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE reserva_id=$1 AND activo`, pending.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired occupancy count=%d err=%v", count, err)
	}
	quote4, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: start.Add(7 * time.Hour).Format(time.RFC3339), EndAt: start.Add(8 * time.Hour).Format(time.RFC3339)})
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
	if _, err = svc.Request(ctx, renter, booking.RequestInput{QuoteID: quote2.ID}, "same-key"); err != booking.ErrConflict {
		t.Fatalf("idempotency payload conflict=%v", err)
	}
	// Two distinct keys and quotes for the same window race on the single
	// occupancy exclusion constraint; only one request can retain it.
	startRace := start.Add(24 * time.Hour)
	qa, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: startRace.Format(time.RFC3339), EndAt: startRace.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	qb, err := svc.Quote(ctx, renter, booking.QuoteInput{StartAt: startRace.Format(time.RFC3339), EndAt: startRace.Add(time.Hour).Format(time.RFC3339)})
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
}
