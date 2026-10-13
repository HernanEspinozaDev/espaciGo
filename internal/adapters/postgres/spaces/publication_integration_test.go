package spacespg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	gallerypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/gallery"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	pricingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/pricing"
	publiccatalogpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/publiccatalog"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	bookinghttp "github.com/HernanEspinozaDev/espaciGo/internal/booking/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type publicationDeleteFiles struct {
	*evidencefs.Store
	failNext bool
}

func (f *publicationDeleteFiles) Delete(ctx context.Context, id string) error {
	if f.failNext {
		f.failNext = false
		return fmt.Errorf("temporary synthetic file deletion failure")
	}
	return f.Store.Delete(ctx, id)
}

type publicationFlowAuth struct{ ownerID, renterID string }

func (a publicationFlowAuth) Authorize(_ context.Context, token identity.Secret, required identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	var principal identity.Principal
	switch token {
	case "owner-token":
		principal = identity.Principal{AccountID: a.ownerID, Roles: []identity.Role{identity.RoleLandlord, identity.RoleTenant}}
	case "renter-token":
		principal = identity.Principal{AccountID: a.renterID, Roles: []identity.Role{identity.RoleTenant}}
	default:
		return identity.Principal{}, identity.ErrUnauthorized
	}
	if required == identity.RoleLandlord && !containsIdentityRole(principal.Roles, identity.RoleLandlord) {
		return identity.Principal{}, identity.ErrForbidden
	}
	return principal, nil
}

func containsIdentityRole(roles []identity.Role, target identity.Role) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func invokePublicationAPI(handler http.Handler, method, path, token string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func waitForAccountLockWaiters(t *testing.T, ctx context.Context, pool *pgxpool.Pool, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname=current_database() AND wait_event_type='Lock'
			AND query LIKE '%SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE%'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d account-lock waiter(s)", count)
}

func TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions(t *testing.T) {
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
	db := fmt.Sprintf("space_pub_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{db}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{db}.Sanitize()+" WITH (FORCE)")
		_, _ = admin.Exec(context.Background(), `DROP ROLE IF EXISTS espacigo_runtime`)
	})
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + db
	if _, err = migrator.Run(ctx, u.String(), "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	adminPool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	if _, err = adminPool.Exec(ctx, `DO $$ BEGIN CREATE ROLE espacigo_runtime LOGIN PASSWORD 'runtime-test-only'; EXCEPTION WHEN duplicate_object THEN ALTER ROLE espacigo_runtime LOGIN PASSWORD 'runtime-test-only'; END $$;`); err != nil {
		t.Fatal(err)
	}
	bootstrapConn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	if err = dbbootstrap.GrantRuntimePermissions(ctx, bootstrapConn); err != nil {
		t.Fatal(err)
	}
	_ = bootstrapConn.Close(ctx)
	runtimeURL := *u
	runtimeURL.User = url.UserPassword("espacigo_runtime", "runtime-test-only")
	pool, err := pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	owner, other, adminID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	for id, n := range map[string]int{owner: 1, other: 2, adminID: 3} {
		email := fmt.Sprintf("publish-%d@example.test", n)
		if _, err = adminPool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'x','activo')`, id, email); err != nil {
			t.Fatal(err)
		}
	}
	repo := New(pool, credentials.Generator{})
	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	in := spaces.Input{CategoryCode: "oficina", Title: "Oficina local", Description: strings.Repeat("Espacio sintético para comprobar publicación local sin exposición pública. ", 2), AreaM2: 20, Capacity: 4, UsageRules: "Sin fumar", RateUnit: "hora", BasePriceCLP: 8000, Address: "Dirección sintética"}
	draft, err := svc.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, draft.ID, "activa", "publish-missing-time-zone"); err != spaces.ErrTimeZoneRequired {
		t.Fatalf("publication without IANA time zone error=%v", err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio SET zona_horaria='UTC' WHERE id=$1`, draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, draft.ID, "activa", "publish-no-kyc"); err != spaces.ErrEligibilityRequired {
		t.Fatalf("publication without KYC error=%v", err)
	}
	if _, err = repo.SetPublicationState(ctx, other, draft.ID, "activa", "foreign-owner"); err != spaces.ErrNotFound {
		t.Fatalf("foreign owner error=%v", err)
	}
	// A pending/rejected case and KYB are not a substitute for an effective personal KYC approval.
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia) VALUES
	('10000000-0000-4000-8000-000000000001',$1,'kyb','en_revision','fixture:10000000-0000-4000-8000-000000000001','pending-kyb')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, draft.ID, "activa", "still-no-kyc"); err != spaces.ErrEligibilityRequired {
		t.Fatalf("KYB substituted for KYC: %v", err)
	}
	verificationID := "10000000-0000-4000-8000-000000000002"
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,resuelta_en)
	VALUES($1,$2,'kyc','aprobada',$3,'approved-kyc',$4,now())`, verificationID, owner, "fixture:"+verificationID, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en) VALUES($1,'kyc',$2,'elegible',now())`, owner, verificationID); err != nil {
		t.Fatal(err)
	}
	// An enabled synthetic fixture intentionally remains in the existing
	// draft-only catalog/reservation flow. Publication must conflict atomically.
	renterVerification := "10000000-0000-4000-8000-000000000003"
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,resuelta_en)
	VALUES($1,$2,'kyc','aprobada',$3,'approved-renter-kyc',$4,now())`, renterVerification, other, "fixture:"+renterVerification, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en) VALUES($1,'kyc',$2,'elegible',now())`, other, renterVerification); err != nil {
		t.Fatal(err)
	}
	// Exercise the HTTP APIs for create-without-zone -> clear publication
	// rejection -> explicit configuration -> publication -> renter search,
	// detail, and quote. This listing has no fixture allowlist entry.
	apiAuth := publicationFlowAuth{ownerID: owner, renterID: other}
	calendarService, err := occupancy.NewService(occupancypg.New(pool), credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	pricingService, err := pricing.NewService(pricingpg.New(pool), calendarService, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesAPI := spaceshttp.NewHandlerWithPricing(apiAuth, svc, nil, calendarService, pricingService)
	bookingPayment, err := fakebooking.New([]byte("space-zone-publication-local-payment-key-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	bookingService, err := booking.NewService(bookingpg.New(pool), credentials.Generator{}, time.Now, bookingPayment)
	if err != nil {
		t.Fatal(err)
	}
	bookingAPI := bookinghttp.NewHandler(apiAuth, bookingService, nil)
	apiDraftInput := spaces.Input{CategoryCode: "oficina", Title: "Oficina API con zona explícita", Description: strings.Repeat("Espacio sintético publicado tras configurar zona IANA. ", 2), AreaM2: 28, Capacity: 5, UsageRules: "Sin fumar", RateUnit: "hora", BasePriceCLP: 11000, Address: "Dirección sintética privada", AttributeSchemaVersion: 1, Attributes: map[string]any{}}
	apiDraftBody, err := json.Marshal(apiDraftInput)
	if err != nil {
		t.Fatal(err)
	}
	createResponse := invokePublicationAPI(spacesAPI, http.MethodPost, "/api/v1/spaces", "owner-token", apiDraftBody)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create draft without zone response=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("created draft response=%s err=%v", createResponse.Body.String(), err)
	}
	publicationBody := []byte(`{"state":"activa"}`)
	missingZoneResponse := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+created.ID+"/publication", "owner-token", publicationBody)
	var timeZoneAPIError struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if missingZoneResponse.Code != http.StatusConflict || json.Unmarshal(missingZoneResponse.Body.Bytes(), &timeZoneAPIError) != nil || timeZoneAPIError.Error.Code != "time_zone_required" || !strings.Contains(timeZoneAPIError.Error.Message, "IANA") {
		t.Fatalf("publish without zone response=%d body=%s", missingZoneResponse.Code, missingZoneResponse.Body.String())
	}
	stillDraft, err := svc.GetOwn(ctx, owner, created.ID)
	if err != nil || stillDraft.State != "borrador" || stillDraft.TimeZone != nil {
		t.Fatalf("rejected publication changed draft: %+v err=%v", stillDraft, err)
	}
	zoneBody := []byte(`{"time_zone":"America/Santiago"}`)
	configuredResponse := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+created.ID+"/availability", "owner-token", zoneBody)
	if configuredResponse.Code != http.StatusOK || !strings.Contains(configuredResponse.Body.String(), `"time_zone":"America/Santiago"`) {
		t.Fatalf("configure zone response=%d body=%s", configuredResponse.Code, configuredResponse.Body.String())
	}
	publishResponse := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+created.ID+"/publication", "owner-token", publicationBody)
	if publishResponse.Code != http.StatusOK || !strings.Contains(publishResponse.Body.String(), `"state":"activa"`) {
		t.Fatalf("publish configured draft response=%d body=%s", publishResponse.Code, publishResponse.Body.String())
	}
	searchResponse := invokePublicationAPI(bookingAPI, http.MethodGet, "/api/v1/local/booking-trial/catalog?category_code=oficina&page_size=25", "renter-token", nil)
	if searchResponse.Code != http.StatusOK || !strings.Contains(searchResponse.Body.String(), created.ID) || strings.Contains(searchResponse.Body.String(), "Dirección sintética privada") {
		t.Fatalf("renter search response=%d body=%s", searchResponse.Code, searchResponse.Body.String())
	}
	detailResponse := invokePublicationAPI(bookingAPI, http.MethodGet, "/api/v1/local/booking-trial/catalog/"+created.ID, "renter-token", nil)
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), "Oficina API con zona explícita") || !strings.Contains(detailResponse.Body.String(), "America/Santiago") {
		t.Fatalf("renter detail response=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	quoteStart := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	quoteBody, err := json.Marshal(booking.QuoteInput{SpaceID: created.ID, StartAt: quoteStart.Format(time.RFC3339), EndAt: quoteStart.Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	quoteResponse := invokePublicationAPI(bookingAPI, http.MethodPost, "/api/v1/local/booking-trial/quotes", "renter-token", quoteBody)
	if quoteResponse.Code != http.StatusOK || !strings.Contains(quoteResponse.Body.String(), created.ID) || !strings.Contains(quoteResponse.Body.String(), "America/Santiago") {
		t.Fatalf("renter quote response=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
	}
	// A deletion that wins the shared account-lock queue must prevent a
	// reservation from being created from an earlier quote. Both operations
	// serialize on the landlord account and the space row; the losing booking
	// request must revalidate publication state after the deletion commits.
	var raceQuoteEnvelope struct {
		Data booking.Quote `json:"data"`
	}
	if err = json.Unmarshal(quoteResponse.Body.Bytes(), &raceQuoteEnvelope); err != nil || raceQuoteEnvelope.Data.ID == "" {
		t.Fatalf("decode quote for deletion race: quote=%+v err=%v", raceQuoteEnvelope.Data, err)
	}
	raceQuote := raceQuoteEnvelope.Data
	accountBlocker, err := adminPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = accountBlocker.Exec(ctx, `SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE`, owner); err != nil {
		_ = accountBlocker.Rollback(ctx)
		t.Fatal(err)
	}
	deleteRace := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		deleteRace <- invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+created.ID, "owner-token", nil)
	}()
	waitForAccountLockWaiters(t, ctx, adminPool, 1)
	requestRace := make(chan error, 1)
	go func() {
		_, requestErr := bookingService.Request(ctx, other, booking.RequestInput{QuoteID: raceQuote.ID}, "retirement-booking-race")
		requestRace <- requestErr
	}()
	waitForAccountLockWaiters(t, ctx, adminPool, 2)
	if err = accountBlocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	deleteRaceResponse := <-deleteRace
	if deleteRaceResponse.Code != http.StatusOK || !strings.Contains(deleteRaceResponse.Body.String(), `"state":"eliminada"`) {
		t.Fatalf("delete won concurrent reservation race status=%d body=%s", deleteRaceResponse.Code, deleteRaceResponse.Body.String())
	}
	if requestErr := <-requestRace; !errors.Is(requestErr, booking.ErrConflict) {
		t.Fatalf("reservation from quote after retirement error=%v, want conflict", requestErr)
	}
	var racedReservations int
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local WHERE cotizacion_id=$1`, raceQuote.ID).Scan(&racedReservations); err != nil || racedReservations != 0 {
		t.Fatalf("retirement race created a reservation: count=%d err=%v", racedReservations, err)
	}
	// A legacy hidden publication can be repaired by its owner and then
	// activated; no default zone is inferred by the server.
	legacyDraft, err := svc.Create(ctx, owner, apiDraftInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio SET estado='oculta' WHERE id=$1`, legacyDraft.ID); err != nil {
		t.Fatal(err)
	}
	legacyZoneResponse := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+legacyDraft.ID+"/availability", "owner-token", zoneBody)
	if legacyZoneResponse.Code != http.StatusOK {
		t.Fatalf("repair legacy hidden publication response=%d body=%s", legacyZoneResponse.Code, legacyZoneResponse.Body.String())
	}
	legacyPublishResponse := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+legacyDraft.ID+"/publication", "owner-token", publicationBody)
	if legacyPublishResponse.Code != http.StatusOK {
		t.Fatalf("publish repaired legacy listing response=%d body=%s", legacyPublishResponse.Code, legacyPublishResponse.Body.String())
	}
	fixtureDraft, err := svc.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio SET zona_horaria='UTC' WHERE id=$1`, fixtureDraft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, fixtureDraft.ID, owner, other); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, fixtureDraft.ID, "activa", "fixture-publication-conflict"); err != spaces.ErrEnabledFixture {
		t.Fatalf("enabled fixture publication error=%v", err)
	}
	unchanged, err := repo.GetOwn(ctx, owner, fixtureDraft.ID)
	if err != nil || unchanged.State != "borrador" {
		t.Fatalf("fixture state changed after rejected transition: state=%s err=%v", unchanged.State, err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio SET estado='activa' WHERE id=$1`, fixtureDraft.ID); err != nil {
		t.Fatal(err)
	}
	enabledFixtureDelete := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "owner-token", nil)
	if enabledFixtureDelete.Code != http.StatusConflict || !strings.Contains(enabledFixtureDelete.Body.String(), "publication_delete_blocked") {
		t.Fatalf("delete enabled fixture status=%d body=%s", enabledFixtureDelete.Code, enabledFixtureDelete.Body.String())
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio SET estado='borrador' WHERE id=$1`, fixtureDraft.ID); err != nil {
		t.Fatal(err)
	}
	bookingRepo := bookingpg.New(pool)
	catalog, err := bookingRepo.Catalog(ctx, other, booking.CatalogFilter{})
	foundFixture := false
	for _, item := range catalog {
		if item.SpaceID == fixtureDraft.ID {
			foundFixture = true
		}
	}
	if err != nil || !foundFixture {
		t.Fatalf("fixture catalog after conflict: items=%+v err=%v", catalog, err)
	}
	quoteID, _ := (credentials.Generator{}).ID()
	reservationID, _ := (credentials.Generator{}).ID()
	occupancyID, _ := (credentials.Generator{}).ID()
	now := time.Now().UTC()
	startAt := now.Add(48 * time.Hour).Truncate(time.Minute)
	quote, err := bookingRepo.Quote(ctx, other, fixtureDraft.ID, quoteID, startAt, startAt.Add(time.Hour), func() time.Time { return now }, 15*time.Minute)
	if err != nil {
		t.Fatalf("quote after rejected publication: %v", err)
	}
	fingerprint := sha256.Sum256([]byte("fixture reservation"))
	reservation, err := bookingRepo.Create(ctx, other, quote.ID, "fixture-reservation-key", fingerprint[:], reservationID, occupancyID, 15*time.Minute, func() time.Time { return now })
	if err != nil || reservation.State != "pendiente_de_pago" {
		t.Fatalf("reservation after rejected publication: %+v err=%v", reservation, err)
	}
	weekly := booking.WeeklyHours{SpaceID: fixtureDraft.ID, Enabled: true, Days: make([]booking.WeeklyDay, 7)}
	for i := range weekly.Days {
		weekly.Days[i] = booking.WeeklyDay{Weekday: i + 1, Periods: []booking.WeeklyPeriod{}}
	}
	weekly.Days[2].Periods = []booking.WeeklyPeriod{{Open: "09:00", Close: "17:00"}}
	if _, err = bookingService.SaveWeeklyHours(ctx, owner, fixtureDraft.ID, weekly); err != nil {
		t.Fatalf("enable weekly schedule for tariff guard: %v", err)
	}
	modeChange := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/tariff", "owner-token", []byte(`{"rate_unit":"dia","base_price":8000}`))
	if modeChange.Code != http.StatusConflict || !strings.Contains(modeChange.Body.String(), `"code":"tariff_calendar_conflict"`) {
		t.Fatalf("mode change with active weekly schedule status=%d body=%s", modeChange.Code, modeChange.Body.String())
	}
	var versionsBeforeConflict int
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$1`, fixtureDraft.ID).Scan(&versionsBeforeConflict); err != nil || versionsBeforeConflict != 1 {
		t.Fatalf("incompatible mode change created a tariff version: count=%d err=%v", versionsBeforeConflict, err)
	}
	weekly.Enabled = false
	if _, err = bookingService.SaveWeeklyHours(ctx, owner, fixtureDraft.ID, weekly); err != nil {
		t.Fatalf("disable weekly schedule before mode change: %v", err)
	}
	// Once the allowlist entry is disabled, a local publication can be edited.
	// Existing quote and reservation snapshots must remain fixed.
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_ensayo_local_fixture SET habilitada=false WHERE espacio_id=$1`, fixtureDraft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, fixtureDraft.ID, "activa", "activate-for-edit-test"); err != nil {
		t.Fatalf("activate non-fixture publication for edit test: %v", err)
	}
	modeChange = invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/tariff", "owner-token", []byte(`{"rate_unit":"dia","base_price":8000}`))
	if modeChange.Code != http.StatusOK || !strings.Contains(modeChange.Body.String(), `"rate_unit":"dia"`) || !strings.Contains(modeChange.Body.String(), `"version":2`) {
		t.Fatalf("active listing rate-mode change status=%d body=%s", modeChange.Code, modeChange.Body.String())
	}
	if foreignMode := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/tariff", "renter-token", []byte(`{"rate_unit":"mes","base_price":9000}`)); foreignMode.Code != http.StatusForbidden {
		t.Fatalf("non-owner rate-mode update status=%d body=%s", foreignMode.Code, foreignMode.Body.String())
	}
	// A legacy active listing with a reservation can be repaired only when its
	// persisted zone is absent; the historical reservation snapshot is never
	// rewritten by this endpoint.
	zoneReplacement := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/availability", "owner-token", zoneBody)
	if zoneReplacement.Code != http.StatusConflict || !strings.Contains(zoneReplacement.Body.String(), `"code":"time_zone_locked"`) {
		t.Fatalf("configured publication zone replacement response=%d body=%s", zoneReplacement.Code, zoneReplacement.Body.String())
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio SET zona_horaria=NULL WHERE id=$1`, fixtureDraft.ID); err != nil {
		t.Fatal(err)
	}
	foreignRepair := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/availability", "renter-token", zoneBody)
	if foreignRepair.Code != http.StatusNotFound {
		t.Fatalf("non-owner legacy repair response=%d body=%s", foreignRepair.Code, foreignRepair.Body.String())
	}
	protectedRepair := invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/availability", "owner-token", zoneBody)
	if protectedRepair.Code != http.StatusConflict || !strings.Contains(protectedRepair.Body.String(), `"code":"time_zone_in_use"`) {
		t.Fatalf("legacy listing with reservation repair response=%d body=%s", protectedRepair.Code, protectedRepair.Body.String())
	}
	var reservationZone string
	if err = adminPool.QueryRow(ctx, `SELECT zona_horaria FROM public.reserva_ensayo_local WHERE espacio_id=$1`, fixtureDraft.ID).Scan(&reservationZone); err != nil || reservationZone != "UTC" {
		t.Fatalf("legacy repair changed reservation snapshot zone=%q err=%v", reservationZone, err)
	}
	newTitle, newPrice := "Título publicado editado", int64(12000)
	// Another operation changes the rate after the mock loaded the original
	// 8,000 CLP. A stale form submits only its changed title, never the old rate.
	updated, err := svc.UpdatePublishedOwn(ctx, owner, fixtureDraft.ID, spaces.PublishedContentInput{BasePriceCLP: &newPrice})
	if err != nil || updated.State != "activa" || updated.Title != fixtureDraft.Title || updated.BasePriceCLP != newPrice {
		t.Fatalf("concurrent rate update=%+v err=%v", updated, err)
	}
	updated, err = svc.UpdatePublishedOwn(ctx, owner, fixtureDraft.ID, spaces.PublishedContentInput{Title: &newTitle})
	if err != nil || updated.State != "activa" || updated.Title != newTitle || updated.BasePriceCLP != newPrice {
		t.Fatalf("stale title-only listing edit=%+v err=%v", updated, err)
	}
	regionCode, communeCode := "13", "13123"
	updated, err = svc.UpdatePublishedOwn(ctx, owner, fixtureDraft.ID, spaces.PublishedContentInput{RegionCode: &regionCode, CommuneCode: &communeCode})
	if err != nil || updated.State != "activa" || updated.RegionCode == nil || *updated.RegionCode != regionCode || updated.CommuneCode == nil || *updated.CommuneCode != communeCode {
		t.Fatalf("active public location edit=%+v err=%v", updated, err)
	}
	publicService, err := publiccatalog.New(publiccatalogpg.New(pool), []byte("space-public-search-test-master-key-32"))
	if err != nil {
		t.Fatal(err)
	}
	publicHandler := publiccataloghttp.New(publicService)
	publicQuery := "/api/v1/public/spaces?q=" + url.QueryEscape("Título publicado editado") + "&region_code=13&comuna_code=13123"
	publicResponse := httptest.NewRecorder()
	publicHandler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet, publicQuery, nil))
	if publicResponse.Code != http.StatusOK || !strings.Contains(publicResponse.Body.String(), fixtureDraft.ID) || !strings.Contains(publicResponse.Body.String(), `"region_code":"13"`) || strings.Contains(publicResponse.Body.String(), "Dirección privada") {
		t.Fatalf("public search after owner content/location edit status=%d body=%s", publicResponse.Code, publicResponse.Body.String())
	}
	newDescription := "Descripción sintética nueva para validar la edición de los datos propios que se muestran en la publicación local."
	newCapacity := int32(6)
	newRules := "Respetar los horarios y dejar el espacio limpio"
	updated, err = svc.UpdatePublishedOwn(ctx, owner, fixtureDraft.ID, spaces.PublishedContentInput{Description: &newDescription, Capacity: &newCapacity, UsageRules: &newRules})
	if err != nil || updated.State != "activa" || updated.Description != newDescription || updated.Capacity != newCapacity || updated.UsageRules != newRules || updated.BasePriceCLP != newPrice {
		t.Fatalf("active details edit=%+v err=%v", updated, err)
	}
	detail, err := bookingRepo.Get(ctx, other, reservation.ID)
	if err != nil || detail.UnitPrice != 8000 || detail.Subtotal != 8000 {
		t.Fatalf("existing reservation snapshot changed: %+v err=%v", detail.Reservation, err)
	}
	var quotePrice, reservationPrice int64
	var quoteUnit, reservationUnit string
	var rateVersions int
	if err = adminPool.QueryRow(ctx, `SELECT q.precio_unitario_clp,r.precio_unitario_clp,q.modalidad,r.modalidad,(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$1) FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local r ON r.cotizacion_id=q.id WHERE q.id=$2`, fixtureDraft.ID, quote.ID).Scan(&quotePrice, &reservationPrice, &quoteUnit, &reservationUnit, &rateVersions); err != nil {
		t.Fatal(err)
	}
	if quotePrice != 8000 || reservationPrice != 8000 || quoteUnit != "hora" || reservationUnit != "hora" || rateVersions != 3 {
		t.Fatalf("price version/snapshots quote=%d/%s reservation=%d/%s versions=%d", quotePrice, quoteUnit, reservationPrice, reservationUnit, rateVersions)
	}
	if _, err = svc.SetPublicationState(ctx, owner, fixtureDraft.ID, "oculta", "hide-after-edit"); err != nil {
		t.Fatal(err)
	}
	publicResponse = httptest.NewRecorder()
	publicHandler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet, publicQuery, nil))
	if publicResponse.Code != http.StatusOK || strings.Contains(publicResponse.Body.String(), fixtureDraft.ID) {
		t.Fatalf("hidden listing remained in public search status=%d body=%s", publicResponse.Code, publicResponse.Body.String())
	}
	newHiddenTitle := "Título oculto editado"
	updated, err = svc.UpdatePublishedOwn(ctx, owner, fixtureDraft.ID, spaces.PublishedContentInput{Title: &newHiddenTitle})
	if err != nil || updated.State != "oculta" || updated.Title != newHiddenTitle || updated.BasePriceCLP != newPrice {
		t.Fatalf("hidden listing title edit=%+v err=%v", updated, err)
	}
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$1`, fixtureDraft.ID).Scan(&rateVersions); err != nil {
		t.Fatal(err)
	}
	if rateVersions != 3 {
		t.Fatalf("title-only edit created a tariff version: count=%d", rateVersions)
	}
	modeChange = invokePublicationAPI(spacesAPI, http.MethodPut, "/api/v1/spaces/"+fixtureDraft.ID+"/tariff", "owner-token", []byte(`{"rate_unit":"mes","base_price":12000}`))
	if modeChange.Code != http.StatusOK || !strings.Contains(modeChange.Body.String(), `"rate_unit":"mes"`) || !strings.Contains(modeChange.Body.String(), `"version":4`) {
		t.Fatalf("hidden listing rate-mode change status=%d body=%s", modeChange.Code, modeChange.Body.String())
	}
	if err = adminPool.QueryRow(ctx, `SELECT q.precio_unitario_clp,r.precio_unitario_clp,q.modalidad,r.modalidad FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local r ON r.cotizacion_id=q.id WHERE q.id=$1`, quote.ID).Scan(&quotePrice, &reservationPrice, &quoteUnit, &reservationUnit); err != nil {
		t.Fatal(err)
	}
	if quotePrice != 8000 || reservationPrice != 8000 || quoteUnit != "hora" || reservationUnit != "hora" {
		t.Fatalf("hidden mode change altered historical snapshots: quote=%d/%s reservation=%d/%s", quotePrice, quoteUnit, reservationPrice, reservationUnit)
	}
	if _, err = svc.UpdatePublishedOwn(ctx, other, fixtureDraft.ID, spaces.PublishedContentInput{Title: &newTitle}); err != spaces.ErrNotFound {
		t.Fatalf("foreign published-content edit error=%v", err)
	}
	start := make(chan struct{})
	type publishResult struct {
		draft spaces.Draft
		err   error
	}
	results := make(chan publishResult, 2)
	for _, correlation := range []string{"publish-1", "publish-repeat"} {
		go func(correlation string) {
			<-start
			draft, err := svc.SetPublicationState(ctx, owner, draft.ID, "activa", correlation)
			results <- publishResult{draft, err}
		}(correlation)
	}
	close(start)
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil || got.draft.State != "activa" {
			t.Fatalf("concurrent publish=%+v err=%v", got.draft, got.err)
		}
	}
	var firstCount int
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.espacio_publicacion_historial_local WHERE espacio_id=$1`, draft.ID).Scan(&firstCount); err != nil || firstCount != 1 {
		t.Fatalf("concurrent transition count=%d err=%v", firstCount, err)
	}
	hidden, err := svc.SetPublicationState(ctx, owner, draft.ID, "oculta", "hide-1")
	if err != nil || hidden.State != "oculta" {
		t.Fatalf("hide=%+v err=%v", hidden, err)
	}
	active, err := svc.SetPublicationState(ctx, owner, draft.ID, "activa", "publish-2")
	if err != nil || active.State != "activa" {
		t.Fatalf("republish=%+v err=%v", active, err)
	}
	var count int
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.espacio_publicacion_historial_local WHERE espacio_id=$1`, draft.ID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("publication history count=%d err=%v", count, err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.elegibilidad_verificacion_local SET estado='revocada',revocada_en=now(),revocada_por=$2,motivo_revocacion_codigo='aprobacion_fixture_incorrecta' WHERE usuario_id=$1 AND tipo='kyc'`, owner, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, draft.ID, "oculta", "hide-2"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, draft.ID, "activa", "publish-after-revoke"); err != spaces.ErrEligibilityRequired {
		t.Fatalf("revoked KYC publication error=%v", err)
	}
	// A pending reservation is an atomic blocker: the failed request must keep
	// the offer, rate snapshots, and booking history available.
	unauthorizedDelete := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "renter-token", nil)
	if unauthorizedDelete.Code != http.StatusForbidden {
		t.Fatalf("non-owner publication delete status=%d body=%s", unauthorizedDelete.Code, unauthorizedDelete.Body.String())
	}
	blockedDelete := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "owner-token", nil)
	if blockedDelete.Code != http.StatusConflict || !strings.Contains(blockedDelete.Body.String(), "publication_delete_blocked") {
		t.Fatalf("delete with pending booking status=%d body=%s", blockedDelete.Code, blockedDelete.Body.String())
	}
	stillPublished, err := repo.GetOwn(ctx, owner, fixtureDraft.ID)
	if err != nil || stillPublished.State != "oculta" {
		t.Fatalf("blocked delete modified publication: %+v err=%v", stillPublished, err)
	}
	if _, err = bookingRepo.Get(ctx, other, reservation.ID); err != nil {
		t.Fatalf("blocked delete removed reservation snapshot: %v", err)
	}
	// Even after the booking becomes terminal, an authorized guarantee that has
	// not yet been released remains an independent financial blocker.
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_por_pago' WHERE id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=clock_timestamp() WHERE id=(SELECT ocupacion_id FROM public.reserva_ensayo_local WHERE id=$1)`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	guaranteeUpdate, guaranteeErr := adminPool.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET autorizado_clp=50000,estado='liberacion_pendiente',actualizada_en=clock_timestamp() WHERE reserva_id=$1`, reservation.ID)
	if guaranteeErr != nil || guaranteeUpdate.RowsAffected() != 1 {
		t.Fatalf("prepare pending guarantee release rows=%v err=%v", guaranteeUpdate, guaranteeErr)
	}
	financialBlock := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "owner-token", nil)
	if financialBlock.Code != http.StatusConflict || !strings.Contains(financialBlock.Body.String(), "publication_delete_blocked") {
		t.Fatalf("delete with pending guarantee release status=%d body=%s", financialBlock.Code, financialBlock.Body.String())
	}
	if _, err = bookingRepo.Get(ctx, other, reservation.ID); err != nil {
		t.Fatalf("blocked deletion removed terminal historical reservation: %v", err)
	}
	// Resolve the financial blocker and make this reservation terminal, with a
	// persisted check-out older than the 24-hour claim window and a confirmed
	// fake settlement. The successful deletion must preserve these snapshots.
	if _, err = adminPool.Exec(ctx, `UPDATE public.cotizacion_reserva_ensayo SET inicio=inicio-interval '4 days',termino=termino-interval '4 days',creada_en=creada_en-interval '4 days',vence_en=vence_en-interval '4 days' WHERE id=(SELECT cotizacion_id FROM public.reserva_ensayo_local WHERE id=$1)`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='finalizada',inicio=inicio-interval '4 days',termino=termino-interval '4 days',creada_en=creada_en-interval '4 days',actualizada_en=clock_timestamp()-interval '3 days' WHERE id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado='parcialmente_capturada',autorizado_clp=50000,capturado_clp=12000,liberado_clp=38000,actualizada_en=clock_timestamp() WHERE reserva_id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en) VALUES('90000000-0000-4000-8000-000000000003',$1,'exito_simulado','delete-test-payment',clock_timestamp())`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en) VALUES('90000000-0000-4000-8000-000000000005',$1,(SELECT COALESCE(max(secuencia),0)+1 FROM public.reserva_ensayo_transicion WHERE reserva_id=$1),'cancelada_por_pago','finalizada',$2,'ensayo_terminal_para_prueba_de_eliminacion',clock_timestamp())`, reservation.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en)
		VALUES('90000000-0000-4000-8000-000000000002',$1,'checkout',$2,clock_timestamp()-interval '48 hours','UTC','{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}'::jsonb,'registrada','delete-test-checkout',decode(repeat('ab',32),'hex'),clock_timestamp()-interval '48 hours')`, reservation.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.operacion_arriendo_historial_ensayo_local(reserva_id,operacion_id,accion,actor_id,ocurrida_en) VALUES($1,'90000000-0000-4000-8000-000000000002','checkout_registrado',$2,clock_timestamp()-interval '48 hours')`, reservation.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_liquidacion_ensayo_local(id,reserva_id,anfitrion_id,politica_version,moneda,arriendo_confirmado_clp,devoluciones_confirmadas_clp,comision_bps,comision_clp,transferencia_clp,estado,clave_idempotencia,huella_solicitud,creada_en,actualizada_en,completada_en)
		VALUES('90000000-0000-4000-8000-000000000004',$1,$2,'liquidacion_local_0pct_v1','CLP',8000,0,0,0,8000,'liquidada','delete-test-settlement',decode(repeat('cd',32),'hex'),clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours')`, reservation.ID, owner); err != nil {
		t.Fatal(err)
	}
	// A confirmed deduction and confirmed release of the remainder leave no
	// guarantee obligation even though the summary state remains partial.
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en)
		VALUES('90000000-0000-4000-8000-000000000006','90000000-0000-4000-8000-000000000002','synthetic-png-v1','image/png',repeat('a',64),128,clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reclamo_dano_ensayo_local(id,reserva_id,anfitrion_id,arrendatario_id,checkout_operacion_id,checkout_evidencia_id,descripcion,estado,clave_idempotencia,huella_solicitud,abierto_en,plazo_reclamo_hasta)
		VALUES('90000000-0000-4000-8000-000000000007',$1,$2,$3,'90000000-0000-4000-8000-000000000002','90000000-0000-4000-8000-000000000006','Daño sintético','resuelta','delete-claim-key',decode(repeat('ef',32),'hex'),clock_timestamp()-interval '48 hours',clock_timestamp()-interval '24 hours')`, reservation.ID, owner, other); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
		SELECT '90000000-0000-4000-8000-000000000008',id,'autorizacion','delete-auth',decode(repeat('a1',32),'hex'),50000,'exito','confirmada',clock_timestamp()-interval '48 hours','exito_simulado',clock_timestamp()-interval '48 hours',clock_timestamp()-interval '48 hours',clock_timestamp()-interval '48 hours' FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
		SELECT '90000000-0000-4000-8000-000000000009',id,'captura','delete-capture',decode(repeat('a2',32),'hex'),12000,'exito','confirmada',clock_timestamp()-interval '24 hours','exito_simulado',clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours' FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
		SELECT '90000000-0000-4000-8000-000000000010',id,'liberacion','delete-release',decode(repeat('a3',32),'hex'),38000,'exito','confirmada',clock_timestamp()-interval '24 hours','exito_simulado',clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours' FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reclamo_dano_resolucion_ensayo_local(reclamo_id,resultado,motivo_codigo,administrador_id,resuelta_en,clave_idempotencia,huella_solicitud)
		VALUES('90000000-0000-4000-8000-000000000007','acogido','evidencia_suficiente',$1,clock_timestamp()-interval '24 hours','delete-claim-resolve',decode(repeat('ef',32),'hex'))`, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO public.reserva_decision_financiera_ensayo_local(garantia_id,reserva_id,reclamo_id,resultado_reclamo,deduccion_clp,motivo_codigo,evidencia_id,administrador_id,clave_idempotencia,huella_solicitud,estado,creada_en,actualizada_en)
		SELECT g.id,$1,'90000000-0000-4000-8000-000000000007','acogido',12000,'dano_acreditado','90000000-0000-4000-8000-000000000006',$2,'delete-finance-decision',decode(repeat('ee',32),'hex'),'aplicada',clock_timestamp()-interval '24 hours',clock_timestamp()-interval '24 hours' FROM public.reserva_garantia_ensayo_local g WHERE g.reserva_id=$1`, reservation.ID, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_garantia_operacion_ensayo_local SET estado='por_conciliar' WHERE id='90000000-0000-4000-8000-000000000010'`); err != nil {
		t.Fatal(err)
	}
	pendingGuaranteeReconciliation := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "owner-token", nil)
	if pendingGuaranteeReconciliation.Code != http.StatusConflict || !strings.Contains(pendingGuaranteeReconciliation.Body.String(), "publication_delete_blocked") {
		t.Fatalf("delete with fully allocated but unreconciled guarantee operation status=%d body=%s", pendingGuaranteeReconciliation.Code, pendingGuaranteeReconciliation.Body.String())
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_garantia_operacion_ensayo_local SET estado='confirmada' WHERE id='90000000-0000-4000-8000-000000000010'`); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET liberado_clp=0,actualizada_en=clock_timestamp() WHERE reserva_id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	remainingGuaranteeBlock := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "owner-token", nil)
	if remainingGuaranteeBlock.Code != http.StatusConflict || !strings.Contains(remainingGuaranteeBlock.Body.String(), "publication_delete_blocked") {
		t.Fatalf("delete with unallocated guarantee balance status=%d body=%s", remainingGuaranteeBlock.Code, remainingGuaranteeBlock.Body.String())
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET liberado_clp=38000,actualizada_en=clock_timestamp() WHERE reserva_id=$1`, reservation.ID); err != nil {
		t.Fatal(err)
	}
	// Eligible retirement queues private bytes durably. A transient unlink
	// failure is recorded and the existing worker completes it on retry.
	root, err := os.MkdirTemp("", "espacigo-publication-delete-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if err = os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	baseFiles, err := evidencefs.New(filepath.Clean(root))
	if err != nil {
		t.Fatal(err)
	}
	files := &publicationDeleteFiles{Store: baseFiles, failNext: true}
	galleryService, err := gallery.NewService(gallerypg.New(pool), files, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	photo, _, err := galleryService.AddSynthetic(ctx, owner, fixtureDraft.ID, "delete-gallery-photo-01")
	if err != nil {
		t.Fatalf("add delete fixture photo: %v", err)
	}
	if _, err = galleryService.SelectCover(ctx, owner, fixtureDraft.ID, photo.ID); err != nil {
		t.Fatal(err)
	}
	deleted := invokePublicationAPI(spacesAPI, http.MethodDelete, "/api/v1/spaces/"+fixtureDraft.ID, "owner-token", nil)
	if deleted.Code != http.StatusOK || !strings.Contains(deleted.Body.String(), `"state":"eliminada"`) || !strings.Contains(deleted.Body.String(), `"files_pending_cleanup":1`) {
		t.Fatalf("delete eligible publication status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	if _, err = repo.GetOwn(ctx, owner, fixtureDraft.ID); err != spaces.ErrNotFound {
		t.Fatalf("deleted offer remains owner-visible: %v", err)
	}
	deletedCatalog := invokePublicationAPI(bookingAPI, http.MethodGet, "/api/v1/local/booking-trial/catalog?category_code=oficina&page_size=25", "renter-token", nil)
	if deletedCatalog.Code != http.StatusOK || strings.Contains(deletedCatalog.Body.String(), fixtureDraft.ID) {
		t.Fatalf("deleted offer remains in catalog status=%d body=%s", deletedCatalog.Code, deletedCatalog.Body.String())
	}
	deletedDetail := invokePublicationAPI(bookingAPI, http.MethodGet, "/api/v1/local/booking-trial/catalog/"+fixtureDraft.ID, "renter-token", nil)
	if deletedDetail.Code != http.StatusNotFound {
		t.Fatalf("deleted offer detail status=%d body=%s", deletedDetail.Code, deletedDetail.Body.String())
	}
	items, err := repo.ListOwn(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == fixtureDraft.ID {
			t.Fatal("deleted offer appears in owner list")
		}
	}
	var storedState, photoState string
	var storedCover *string
	var fileID *string
	var summary []byte
	var histories int
	if err = adminPool.QueryRow(ctx, `SELECT s.estado,s.portada_foto_id::text,g.estado,g.archivo_id::text,h.resumen,(SELECT count(*) FROM public.espacio_eliminacion_historial_local WHERE espacio_id=s.id) FROM public.espacio s JOIN public.espacio_galeria_sintetica_local g ON g.espacio_id=s.id JOIN public.espacio_eliminacion_historial_local h ON h.espacio_id=s.id WHERE s.id=$1`, fixtureDraft.ID).Scan(&storedState, &storedCover, &photoState, &fileID, &summary, &histories); err != nil {
		t.Fatal(err)
	}
	if storedState != "eliminada" || storedCover != nil || photoState != "retirada" || fileID == nil || histories != 1 {
		t.Fatalf("retirement persistence state=%s cover=%v photo=%s file=%v history=%d", storedState, storedCover, photoState, fileID, histories)
	}
	photoPath := filepath.Join(root, photo.ID+".png")
	if _, err = os.Stat(photoPath); err != nil {
		t.Fatalf("private file should remain until worker: %v", err)
	}
	clock := time.Now().Add(-time.Hour)
	retryGallery, err := gallery.NewService(gallerypg.New(pool), files, credentials.Generator{}, func() time.Time { return clock })
	if err != nil {
		t.Fatal(err)
	}
	if err = retryGallery.CleanRetiredOnce(ctx, 20); err != nil {
		t.Fatal(err)
	}
	if err = adminPool.QueryRow(ctx, `SELECT ultimo_codigo_error FROM public.espacio_galeria_sintetica_local WHERE id=$1`, photo.ID).Scan(new(string)); err != nil {
		t.Fatalf("failed cleanup should remain durable: %v", err)
	}
	if _, err = os.Stat(photoPath); err != nil {
		t.Fatalf("failed cleanup removed file unexpectedly: %v", err)
	}
	if err = retryGallery.CleanRetiredOnce(ctx, 20); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(photoPath); !os.IsNotExist(err) {
		t.Fatalf("retry did not remove retired file: %v", err)
	}
	if err = adminPool.QueryRow(ctx, `SELECT archivo_id FROM public.espacio_galeria_sintetica_local WHERE id=$1`, photo.ID).Scan(&fileID); err != nil || fileID != nil {
		t.Fatalf("cleanup did not clear file reference: file=%v err=%v", fileID, err)
	}
	firstReceipt, err := svc.DeleteOwn(ctx, owner, fixtureDraft.ID, "retry-delete-first")
	if err != nil {
		t.Fatal(err)
	}
	secondReceipt, err := svc.DeleteOwn(ctx, owner, fixtureDraft.ID, "retry-delete-second")
	if err != nil || !secondReceipt.Reused || secondReceipt.SpaceID != firstReceipt.SpaceID || len(secondReceipt.Preserved) != len(firstReceipt.Preserved) {
		t.Fatalf("delete replay=%+v first=%+v err=%v", secondReceipt, firstReceipt, err)
	}
	if _, err = svc.DeleteOwn(ctx, other, fixtureDraft.ID, "foreign-delete"); err != spaces.ErrNotFound {
		t.Fatalf("foreign delete error=%v", err)
	}
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.espacio_publicacion_historial_local WHERE espacio_id=$1`, fixtureDraft.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("publication history not preserved count=%d err=%v", count, err)
	}
	archiveSections, err := repo.ExportOwnArchiveSections(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	var deletionEvents []map[string]any
	if err = json.Unmarshal(archiveSections["space_deletion_history"], &deletionEvents); err != nil {
		t.Fatalf("owner ZIP deletion history=%s err=%v", archiveSections["space_deletion_history"], err)
	}
	foundFixtureDeletion := false
	for _, event := range deletionEvents {
		if event["space_id"] == fixtureDraft.ID && event["actor"] == "self" {
			foundFixtureDeletion = true
		}
	}
	if !foundFixtureDeletion {
		t.Fatalf("owner ZIP omitted deletion for retired fixture: %s", archiveSections["space_deletion_history"])
	}
	retained, err := bookingRepo.Get(ctx, other, reservation.ID)
	if err != nil || retained.UnitPrice != 8000 || retained.Subtotal != 8000 || retained.State != "finalizada" || len(retained.History) < 2 || retained.History[len(retained.History)-1].To != "finalizada" {
		t.Fatalf("deleted publication history/snapshot unavailable: detail=%+v err=%v", retained, err)
	}
}
