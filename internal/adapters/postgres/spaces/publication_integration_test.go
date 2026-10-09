package spacespg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	bookinghttp "github.com/HernanEspinozaDev/espaciGo/internal/booking/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
	spacesAPI := spaceshttp.NewHandler(apiAuth, svc, nil, calendarService)
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
	// Once the allowlist entry is disabled, a local publication can be edited.
	// Existing quote and reservation snapshots must remain fixed.
	if _, err = adminPool.Exec(ctx, `UPDATE public.reserva_ensayo_local_fixture SET habilitada=false WHERE espacio_id=$1`, fixtureDraft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SetPublicationState(ctx, owner, fixtureDraft.ID, "activa", "activate-for-edit-test"); err != nil {
		t.Fatalf("activate non-fixture publication for edit test: %v", err)
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
	var rateVersions int
	if err = adminPool.QueryRow(ctx, `SELECT q.precio_unitario_clp,r.precio_unitario_clp,(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$1) FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local r ON r.cotizacion_id=q.id WHERE q.id=$2`, fixtureDraft.ID, quote.ID).Scan(&quotePrice, &reservationPrice, &rateVersions); err != nil {
		t.Fatal(err)
	}
	if quotePrice != 8000 || reservationPrice != 8000 || rateVersions != 2 {
		t.Fatalf("price version/snapshots quote=%d reservation=%d versions=%d", quotePrice, reservationPrice, rateVersions)
	}
	if _, err = svc.SetPublicationState(ctx, owner, fixtureDraft.ID, "oculta", "hide-after-edit"); err != nil {
		t.Fatal(err)
	}
	newHiddenTitle := "Título oculto editado"
	updated, err = svc.UpdatePublishedOwn(ctx, owner, fixtureDraft.ID, spaces.PublishedContentInput{Title: &newHiddenTitle})
	if err != nil || updated.State != "oculta" || updated.Title != newHiddenTitle || updated.BasePriceCLP != newPrice {
		t.Fatalf("hidden listing title edit=%+v err=%v", updated, err)
	}
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$1`, fixtureDraft.ID).Scan(&rateVersions); err != nil {
		t.Fatal(err)
	}
	if rateVersions != 2 {
		t.Fatalf("title-only edit created a tariff version: count=%d", rateVersions)
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
}
