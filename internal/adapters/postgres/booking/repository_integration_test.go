package bookingpg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	conversationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/conversation"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
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
	var messagesRead, messagesInsert, messagesUpdate, messagesDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.mensaje_reserva_ensayo','SELECT'),has_table_privilege(current_user,'public.mensaje_reserva_ensayo','INSERT'),has_table_privilege(current_user,'public.mensaje_reserva_ensayo','UPDATE'),has_table_privilege(current_user,'public.mensaje_reserva_ensayo','DELETE')`).Scan(&messagesRead, &messagesInsert, &messagesUpdate, &messagesDelete); err != nil {
		t.Fatal(err)
	}
	if !messagesRead || !messagesInsert || messagesUpdate || messagesDelete {
		t.Fatalf("message grants SELECT/INSERT/UPDATE/DELETE=%v/%v/%v/%v", messagesRead, messagesInsert, messagesUpdate, messagesDelete)
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
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'bodega',1,'{"altura_util_m":3.2}'::jsonb)`, secondSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',12000)`, secondSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id) VALUES($1,$2,$3)`, secondSpace, host, renter); err != nil {
		t.Fatal(err)
	}
	privateSpace := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,'oficina','Draft no catalogado',repeat('Draft no visible en resultados. ',4),10,1,'Privado','hora',8000,'Privada','America/Santiago')`, privateSpace, host); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'oficina',1,'{}')`, privateSpace); err != nil {
		t.Fatal(err)
	}
	if _, err = setup.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, privateSpace); err != nil {
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
			foundWarehouse = item.CategoryCode == "bodega" && item.Price == 12000 && item.ProfileVersion == 1 && string(item.Attributes) == `{"altura_util_m": 3.2}`
		}
	}
	if !foundWarehouse {
		t.Fatalf("second fixture lacks its own category/profile/tariff: %+v", items)
	}
	detailItem, err := svc.CatalogDetail(ctx, renter, secondSpace)
	if err != nil || detailItem.CategoryCode != "bodega" || detailItem.CategoryName != "Bodega" || detailItem.ProfileVersion != 1 || string(detailItem.Attributes) != `{"altura_util_m": 3.2}` {
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
	if err != nil || selectedQuote.SpaceID != secondSpace || selectedQuote.UnitPrice != 12000 || selectedQuote.CategoryCode != "bodega" || selectedQuote.ProfileVersion != 1 || string(selectedQuote.ProfileValues) != `{"altura_util_m": 3.2}` {
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
	if err != nil || quoteForAnotherSpace.UnitPrice != 12000 {
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
	if _, err = svc.Cancel(ctx, host, results[0].ID); err != booking.ErrNotFound {
		t.Fatalf("host cancelled as renter: %v", err)
	}
	if _, err = svc.Decide(ctx, host, results[0].ID, "aprobar"); err != booking.ErrConflict {
		t.Fatalf("host decided before payment: %v", err)
	}
	if _, err = svc.Decide(ctx, renter, results[0].ID, "aprobar"); err != booking.ErrNotFound {
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
	if _, err = svc.Cancel(ctx, renter, results[0].ID); err != booking.ErrConflict {
		t.Fatalf("renter cancelled after payment: %v", err)
	}
	if _, err = svc.Decide(ctx, renter, results[0].ID, "aprobar"); err != booking.ErrNotFound {
		t.Fatalf("renter decided on own paid reservation: %v", err)
	}
	approved, err := svc.Decide(ctx, host, results[0].ID, "aprobar")
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
	var messageCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1`, results[0].ID).Scan(&messageCount); err != nil || messageCount != 5 {
		t.Fatalf("idempotent retry left message count=%d err=%v", messageCount, err)
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
	if _, err = svc.Decide(ctx, host, results[0].ID, "aprobar"); err != booking.ErrConflict {
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
	if _, err = svc.Pay(ctx, renter, retry.ID, "exito", "payment-two"); err != nil {
		t.Fatal(err)
	}
	rejected, err := svc.Decide(ctx, host, retry.ID, "rechazar")
	if err != nil || rejected.State != "rechazada_arrendador" {
		t.Fatalf("rejection: %+v %v", rejected, err)
	}
	if _, err = conversationService.Send(ctx, renter, retry.ID, "rejected-write", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("rejected reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, host, retry.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 0 {
		t.Fatalf("rejected thread not available read-only: %+v err=%v", terminalRead, readErr)
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
	cancelled, err := svc.Cancel(ctx, renter, cancelPending.ID)
	if err != nil || cancelled.State != "cancelada_arrendatario" {
		t.Fatalf("renter cancellation while pending: %+v err=%v", cancelled, err)
	}
	if _, err = conversationService.Send(ctx, renter, cancelPending.ID, "after-cancel", "No debe enviarse"); err != conversation.ErrConflict {
		t.Fatalf("cancelled reservation allowed message write: %v", err)
	}
	if terminalRead, readErr := conversationService.List(ctx, host, cancelPending.ID, nil, 10); readErr != nil || len(terminalRead.Items) != 1 || terminalRead.Items[0].Body != "Mensaje antes de cancelar" {
		t.Fatalf("cancelled thread was not preserved read-only: %+v err=%v", terminalRead, readErr)
	}
	if _, err = svc.Cancel(ctx, renter, cancelPending.ID); err != booking.ErrConflict {
		t.Fatalf("renter repeated cancellation: %v", err)
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
}
