package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	pricingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/pricing"
	rentalflowpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/rentalflow"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminlocal"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	rentalflowhttp "github.com/HernanEspinozaDev/espaciGo/internal/rentalflow/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type commercialQuoteIntegrationIdentityGate struct{}

func (commercialQuoteIntegrationIdentityGate) RequireForBooking(context.Context, string, string, time.Time) error {
	return nil
}

type commercialQuoteIntegrationIdentityReader struct{}

func (commercialQuoteIntegrationIdentityReader) CurrentForRentalContract(_ context.Context, _ string, _ string, subjectID string, _ time.Time) (rentalflow.ContractIdentity, error) {
	return rentalflow.ContractIdentity{SubjectID: subjectID, LegalName: "Parte de ensayo", RUT: "11.111.111-1"}, nil
}

type commercialQuoteIntegrationAuthenticator struct{ accountID string }

func (a commercialQuoteIntegrationAuthenticator) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: a.accountID}, nil
}

// assertCommercialQuotePersistence exercises QuoteService -> PostgreSQL write/read
// -> ContractPreviewService on the disposable full-schema target. Identity is
// deliberately synthetic so this does not claim identity-provider integration.
func assertCommercialQuotePersistence(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
	t.Helper()
	if err := dbbootstrap.GrantRuntimePermissions(ctx, admin); err != nil {
		t.Fatal(err)
	}
	assertCommercialQuotePersistenceWithExistingRuntimeACL(t, ctx, adminURL, admin)
}

// The combined identity schema has explicit revocations that a legacy runtime
// bootstrap must not regrant. Exercise the same behavior using existing ACLs.
func assertCommercialQuotePersistenceWithExistingRuntimeACL(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
	t.Helper()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.User("espacigo_api_login")
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 2
	cfg.ConnConfig.RuntimeParams["application_name"] = "commercial-quote-integration"
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	ownerID, renterID, spaceID, adminID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, account := range []struct{ id, email string }{
		{ownerID, ownerID + "@commercial-quote-persist.invalid"},
		{renterID, renterID + "@commercial-quote-persist.invalid"},
		{adminID, adminID + "@commercial-quote-persist.invalid"},
	} {
		if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-only','activo')`, account.id, account.email); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
		VALUES($1,$2,'oficina','Commercial quote persistence fixture',repeat('Descripción sintética para la cotización. ',4),10,1,'Solo ensayo','hora',10000,'Dirección sintética','activa','UTC')`, spaceID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',10000)`, spaceID); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	nowFn := func() time.Time { return now }
	if _, err := admin.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,concedido_por,motivo_codigo) VALUES($1,$2,$1,'bootstrap_propietario')`, adminID, now); err != nil {
		t.Fatal(err)
	}
	repo, err := rentalflowpg.NewRepository(runtime, runtime, nowFn)
	if err != nil {
		t.Fatal(err)
	}
	staleListing, err := repo.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,2,'hora',15000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	availabilityDate := now.Add(24 * time.Hour).Format("2006-01-02")
	date, err := time.Parse("2006-01-02", availabilityDate)
	if err != nil {
		t.Fatal(err)
	}
	weekday := int16((int(date.Weekday())+6)%7 + 1)
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo) VALUES($1,true)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,$2,1,0,1440)`, spaceID, weekday); err != nil {
		t.Fatal(err)
	}
	quotes, err := rentalflow.NewQuoteService(repo, repo, repo, nowFn)
	if err != nil {
		t.Fatal(err)
	}
	availabilityService, err := rentalflow.NewAvailabilityService(repo, repo, nowFn)
	if err != nil {
		t.Fatal(err)
	}
	previewService, err := rentalflow.NewContractPreviewService(repo, repo, repo, commercialQuoteIntegrationIdentityGate{}, commercialQuoteIntegrationIdentityReader{}, nowFn)
	if err != nil {
		t.Fatal(err)
	}
	handler := rentalflowhttp.New(commercialQuoteIntegrationAuthenticator{accountID: renterID}, availabilityService, quotes, previewService, nil)

	availabilityRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/rental-availability?"+url.Values{"date": {availabilityDate}, "duration": {"1"}}.Encode(), nil)
	availabilityResponse := httptest.NewRecorder()
	handler.ServeHTTP(availabilityResponse, availabilityRequest)
	if availabilityResponse.Code != http.StatusOK {
		t.Fatalf("public availability returned %d: %s", availabilityResponse.Code, availabilityResponse.Body.String())
	}
	var availabilityEnvelope struct {
		Data rentalflow.Availability `json:"data"`
	}
	if err := json.NewDecoder(availabilityResponse.Body).Decode(&availabilityEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(availabilityEnvelope.Data.Items) == 0 {
		t.Fatalf("HTTP availability returned no eligible interval: %+v", availabilityEnvelope.Data.Items)
	}
	start := availabilityEnvelope.Data.Items[0].StartAt
	if _, err := repo.CreateValidatedQuote(ctx, renterID, staleListing, rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, 1, 10000, now); !errors.Is(err, rentalflow.ErrConflict) {
		t.Fatalf("quote write must reject a stale price snapshot: %v", err)
	}

	quoteRequestBody, err := json.Marshal(rentalflow.QuoteRequest{SpaceID: spaceID, StartAt: start.Format(time.RFC3339Nano), EndAt: start.Add(time.Hour).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(string(quoteRequestBody)))
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-integration-token")
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusCreated {
		t.Fatalf("quote HTTP create returned %d: %s", quoteResponse.Code, quoteResponse.Body.String())
	}
	var quoteEnvelope struct {
		Data rentalflow.Quote `json:"data"`
	}
	if err := json.NewDecoder(quoteResponse.Body).Decode(&quoteEnvelope); err != nil {
		t.Fatal(err)
	}
	quote := quoteEnvelope.Data
	if quote.ID == "" || quote.SpaceTitle != "Commercial quote persistence fixture" || quote.UnitPriceCLP != 15000 || quote.ExpiresAt.Sub(quote.CreatedAt) != rentalflow.QuoteLifetime {
		t.Fatalf("quote snapshot/lifetime mismatch: %+v", quote)
	}
	type quoteCheckFixture struct {
		ownerID, renterID, title, modality, currency, timeZone string
		rateVersion                                            int32
		unitPrice, units, subtotal                             int64
		startAt, endAt, expiresAt                              time.Time
	}
	validCheckFixture := quoteCheckFixture{
		ownerID: ownerID, renterID: renterID, title: "Constraint fixture", rateVersion: 1,
		modality: "hora", unitPrice: 8000, currency: "CLP", units: 1, subtotal: 8000,
		startAt: start, endAt: start.Add(time.Hour), timeZone: "UTC", expiresAt: now.Add(rentalflow.QuoteLifetime),
	}
	invalidQuoteCases := []struct {
		name           string
		constraintName string
		mutate         func(*quoteCheckFixture)
	}{
		{name: "same participant", constraintName: "cotizacion_arriendo_comercial_participantes_ck", mutate: func(f *quoteCheckFixture) { f.renterID = f.ownerID }},
		{name: "blank title", constraintName: "cotizacion_arriendo_comercial_titulo_ck", mutate: func(f *quoteCheckFixture) { f.title = " " }},
		{name: "invalid rate version", constraintName: "cotizacion_arriendo_comercial_tarifa_ck", mutate: func(f *quoteCheckFixture) { f.rateVersion = 0 }},
		{name: "invalid rate modality", constraintName: "cotizacion_arriendo_comercial_tarifa_ck", mutate: func(f *quoteCheckFixture) { f.modality = "año" }},
		{name: "nonpositive unit price", constraintName: "cotizacion_arriendo_comercial_tarifa_ck", mutate: func(f *quoteCheckFixture) { f.unitPrice, f.subtotal = 0, 0 }},
		{name: "unsupported currency", constraintName: "cotizacion_arriendo_comercial_tarifa_ck", mutate: func(f *quoteCheckFixture) { f.currency = "USD" }},
		{name: "unsupported units", constraintName: "cotizacion_arriendo_comercial_unidades_ck", mutate: func(f *quoteCheckFixture) { f.units, f.subtotal = 3, 24000 }},
		{name: "subtotal mismatch", constraintName: "cotizacion_arriendo_comercial_subtotal_ck", mutate: func(f *quoteCheckFixture) { f.subtotal = 7999 }},
		{name: "empty time zone", constraintName: "cotizacion_arriendo_comercial_intervalo_ck", mutate: func(f *quoteCheckFixture) { f.timeZone = "" }},
		{name: "nonpositive interval", constraintName: "cotizacion_arriendo_comercial_intervalo_ck", mutate: func(f *quoteCheckFixture) { f.endAt = f.startAt }},
		{name: "noncanonical expiry", constraintName: "cotizacion_arriendo_comercial_vigencia_ck", mutate: func(f *quoteCheckFixture) { f.expiresAt = now.Add(rentalflow.QuoteLifetime + time.Minute) }},
	}
	for _, tc := range invalidQuoteCases {
		t.Run("database check "+tc.name, func(t *testing.T) {
			fixture := validCheckFixture
			tc.mutate(&fixture)
			_, insertErr := admin.Exec(ctx, `INSERT INTO public.cotizacion_arriendo_comercial(
				id,espacio_id,propietario_id,arrendatario_id,titulo_espacio,tarifa_version,modalidad,precio_unitario_clp,moneda,
				unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
				uuid.NewString(), spaceID, fixture.ownerID, fixture.renterID, fixture.title, fixture.rateVersion,
				fixture.modality, fixture.unitPrice, fixture.currency, fixture.units, fixture.subtotal,
				fixture.startAt, fixture.endAt, fixture.timeZone, now, fixture.expiresAt)
			var pgErr *pgconn.PgError
			if !errors.As(insertErr, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != tc.constraintName {
				t.Fatalf("invalid quote must fail its %s check with SQLSTATE 23514, got %v", tc.constraintName, insertErr)
			}
		})
	}
	available, err := repo.IntervalAvailable(ctx, rentalflow.Listing{ID: spaceID}, rentalflow.Interval{StartAt: quote.StartAt, EndAt: quote.EndAt})
	if err != nil || !available {
		t.Fatalf("creating a quote must not create an occupancy hold: available=%v err=%v", available, err)
	}
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+quote.ID+"/contract-preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-integration-token")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("contract preview HTTP returned %d: %s", previewResponse.Code, previewResponse.Body.String())
	}
	var previewEnvelope struct {
		Data rentalflow.ContractPreview `json:"data"`
	}
	if err := json.NewDecoder(previewResponse.Body).Decode(&previewEnvelope); err != nil {
		t.Fatal(err)
	}
	preview := previewEnvelope.Data
	if preview.State != "preview" || preview.QuoteID != quote.ID || preview.SpaceTitle != quote.SpaceTitle || preview.SubtotalCLP != quote.SubtotalCLP || len(preview.Parties) != 2 {
		t.Fatalf("persisted quote contract preview mismatch: preview=%+v", preview)
	}
	if _, err := repo.GetForParticipant(ctx, uuid.NewString(), quote.ID); !errors.Is(err, rentalflow.ErrNotFound) {
		t.Fatalf("nonparticipant read must be hidden: %v", err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.espacio SET titulo='Título vivo posterior' WHERE id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	readback, err := repo.GetForParticipant(ctx, renterID, quote.ID)
	if err != nil || readback.SpaceTitle != "Commercial quote persistence fixture" {
		t.Fatalf("readback must use stored title snapshot: title=%q err=%v", readback.SpaceTitle, err)
	}
	if purged, err := repo.PurgeExpiredCommercialQuotes(ctx, quote.ExpiresAt.Add(-time.Nanosecond), 10); err != nil || purged != 0 {
		t.Fatalf("quote must survive immediately before expiry: purged=%d err=%v", purged, err)
	}
	if purged, err := repo.PurgeExpiredCommercialQuotes(ctx, quote.ExpiresAt, 10); err != nil || purged != 1 {
		t.Fatalf("quote must be purged at expiry: purged=%d err=%v", purged, err)
	}
	if _, err := repo.GetForParticipant(ctx, renterID, quote.ID); !errors.Is(err, rentalflow.ErrNotFound) {
		t.Fatalf("purged quote readback must be not found: %v", err)
	}
	expiredPreviewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+quote.ID+"/contract-preview", nil)
	expiredPreviewRequest.Header.Set("Authorization", "Bearer synthetic-integration-token")
	expiredPreviewResponse := httptest.NewRecorder()
	handler.ServeHTTP(expiredPreviewResponse, expiredPreviewRequest)
	if expiredPreviewResponse.Code != http.StatusNotFound {
		t.Fatalf("preview of purged quote returned %d, want 404: %s", expiredPreviewResponse.Code, expiredPreviewResponse.Body.String())
	}
	if _, err := admin.Exec(ctx, `UPDATE public.espacio_horario_semanal SET activo=false WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.espacio SET modalidad_tarifa='dia',precio_base_clp=30000 WHERE id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,3,'dia',30000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	dailyListing, err := repo.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	dailyStart := time.Date(2030, 4, 1, 0, 0, 0, 0, time.UTC)
	dailyInterval := rentalflow.Interval{StartAt: dailyStart, EndAt: dailyStart.AddDate(0, 0, 2)}
	if _, err := repo.CreateValidatedQuote(ctx, renterID, dailyListing, dailyInterval, 2, 60000, now); err != nil {
		t.Fatalf("clear two-day interval should accept a quote: %v", err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,intervalo,tipo,motivo) VALUES($1,$2,tstzrange($3,$4,'[)'),'bloqueo_manual','Bloqueo sintético del segundo día')`, uuid.NewString(), spaceID, dailyStart.Add(36*time.Hour), dailyStart.Add(37*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateValidatedQuote(ctx, renterID, dailyListing, dailyInterval, 2, 60000, now); !errors.Is(err, rentalflow.ErrConflict) {
		t.Fatalf("two-day quote must reject occupancy on its second day: %v", err)
	}
	var quoteCount int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE espacio_id=$1`, spaceID).Scan(&quoteCount); err != nil {
		t.Fatal(err)
	}
	if quoteCount != 1 {
		t.Fatalf("rejected two-day quote changed persisted quote count to %d, want 1", quoteCount)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM public.ocupacion WHERE espacio_id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.espacio SET modalidad_tarifa='mes',precio_base_clp=200000 WHERE id=$1`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,4,'mes',200000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	monthlyListing, err := repo.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	monthlyStart := dailyStart
	monthlyInterval := rentalflow.Interval{StartAt: monthlyStart, EndAt: monthlyStart.AddDate(0, 1, 0)}
	if _, err := repo.CreateValidatedQuote(ctx, renterID, monthlyListing, monthlyInterval, 1, 200000, now); err != nil {
		t.Fatalf("clear monthly interval should accept a quote: %v", err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,intervalo,tipo,motivo) VALUES($1,$2,tstzrange($3,$4,'[)'),'bloqueo_manual','Bloqueo sintético al final del mes')`, uuid.NewString(), spaceID, monthlyStart.Add(29*24*time.Hour+12*time.Hour), monthlyStart.Add(29*24*time.Hour+13*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateValidatedQuote(ctx, renterID, monthlyListing, monthlyInterval, 1, 200000, now); !errors.Is(err, rentalflow.ErrConflict) {
		t.Fatalf("monthly quote must reject occupancy near its end: %v", err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE espacio_id=$1`, spaceID).Scan(&quoteCount); err != nil {
		t.Fatal(err)
	}
	if quoteCount != 2 {
		t.Fatalf("rejected monthly quote changed persisted quote count to %d, want 2", quoteCount)
	}
	assertCommercialQuoteWriterWaitsForSharedLock(t, ctx, adminURL, runtime, repo, ownerID, renterID, spaceID, monthlyListing, monthlyInterval, now,
		"pricing.Repository.UpdateRate", `SELECT modalidad_tarifa,precio_base_clp,actualizado_en FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado IN (`, 210000,
		func(ctx context.Context, writerPool *pgxpool.Pool) error {
			_, err := pricingpg.New(writerPool).UpdateRate(ctx, ownerID, spaceID, pricing.RateInput{Unit: "mes", Amount: 210000})
			return err
		})
	var publishedRevision time.Time
	if err := runtime.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, spaceID).Scan(&publishedRevision); err != nil {
		t.Fatal(err)
	}
	publishedPrice := int64(220000)
	assertCommercialQuoteWriterWaitsForSharedLock(t, ctx, adminURL, runtime, repo, ownerID, renterID, spaceID, monthlyListing, monthlyInterval, now,
		"spaces.Repository.UpdatePublishedOwnIfRevision", `SELECT actualizado_en FROM public.espacio WHERE propietario_id=$1 AND id=$2 AND estado IN (`, publishedPrice,
		func(ctx context.Context, writerPool *pgxpool.Pool) error {
			_, err := spacespg.New(writerPool, credentials.Generator{}).UpdatePublishedOwnIfRevision(ctx, ownerID, spaceID, publishedRevision, spaces.PublishedContentInput{BasePriceCLP: &publishedPrice})
			return err
		})
	var globalAdminRevision time.Time
	if err := runtime.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, spaceID).Scan(&globalAdminRevision); err != nil {
		t.Fatal(err)
	}
	globalAdminPrice := "230000"
	globalAdminCorrelationID := uuid.NewString()
	assertCommercialQuoteWriterWaitsForSharedLock(t, ctx, adminURL, runtime, repo, ownerID, renterID, spaceID, monthlyListing, monthlyInterval, now,
		"adminglobal.Service.UpdatePublication", `SELECT estado,actualizado_en,modalidad_tarifa,precio_base_clp,superficie_m2::float8 FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, 230000,
		func(ctx context.Context, writerPool *pgxpool.Pool) error {
			service, err := adminglobal.New(writerPool, func() time.Time { return now })
			if err != nil {
				return err
			}
			_, err = service.UpdatePublication(ctx, adminID, spaceID, globalAdminCorrelationID, adminglobal.PublicationContentUpdate{
				BasePriceCLP:      &globalAdminPrice,
				ExpectedUpdatedAt: globalAdminRevision,
				Reason:            "revision_administrativa",
			})
			return err
		})
	assertCreateValidatedQuoteSerializesWithGlobalPublicationUpdate(t, ctx, adminURL, admin, runtime, repo, ownerID, renterID, adminID, spaceID, monthlyInterval, now)
	t.Run("admin-local-account-block", func(t *testing.T) {
		assertAdminLocalBlockSerializesWithQuoteLock(t, ctx, adminURL, admin, runtime, repo, adminID, renterID, monthlyListing, monthlyInterval, now, nowFn)
	})
	t.Run("authentication-account-lock", func(t *testing.T) {
		actionTokenID := uuid.NewString()
		actionTokenHash := strings.ReplaceAll(uuid.NewString(), "-", "") + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := admin.Exec(ctx, `INSERT INTO public.token_accion (id,usuario_id,proposito,token_hash,creado_en,expira_en) VALUES ($1,$2,'verificar_correo',$3,$4,$5)`, actionTokenID, renterID, actionTokenHash, now, now.Add(time.Hour)); err != nil {
			t.Fatalf("insert synthetic verification token: %v", err)
		}
		sessionHash := strings.ReplaceAll(uuid.NewString(), "-", "") + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := admin.Exec(ctx, `INSERT INTO public.sesion (id,usuario_id,token_hash,creada_en,ultima_actividad_en,expira_en) VALUES ($1,$2,$3,$4,$4,$5)`, uuid.NewString(), renterID, sessionHash, now, now.Add(time.Hour)); err != nil {
			t.Fatalf("insert synthetic session: %v", err)
		}
		lookups := []struct {
			name       string
			lookup     identity.AccountLookup
			blockState bool
		}{
			{name: "email", lookup: identity.AccountLookup{Email: renterID + "@commercial-quote-persist.invalid"}, blockState: true},
			{name: "verification-id", lookup: identity.AccountLookup{VerificationID: actionTokenID}},
			{name: "action-token-id", lookup: identity.AccountLookup{ActionTokenID: actionTokenID}},
			{name: "session-hash", lookup: identity.AccountLookup{SessionHash: sessionHash}},
		}
		for _, tc := range lookups {
			t.Run(tc.name, func(t *testing.T) {
				assertAuthenticationLookupSerializesWithQuoteLock(t, ctx, adminURL, admin, repo, renterID, tc.lookup, tc.blockState, monthlyListing, monthlyInterval, now)
			})
		}
	})
	t.Run("retention-purge-publication-lock", func(t *testing.T) {
		assertRetentionPurgeSerializesWithPublicationActivation(t, ctx, adminURL, admin, runtime, ownerID, adminID, spaceID, now)
	})
}

func assertRetentionPurgeSerializesWithPublicationActivation(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn, runtime *pgxpool.Pool, ownerID, adminID, spaceID string, now time.Time) {
	t.Helper()
	updatedAt := now.Add(20 * time.Microsecond)
	if err := admin.QueryRow(ctx, `UPDATE public.espacio SET estado='borrador',actualizado_en=$2 WHERE id=$1 RETURNING actualizado_en`, spaceID, updatedAt).Scan(&updatedAt); err != nil {
		t.Fatalf("prepare draft publication fixture: %v", err)
	}
	verificationID := uuid.NewString()
	resolvedAt := now.AddDate(-3, 0, 0)
	if _, err := admin.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,creada_en,resuelta_en)
		VALUES($1,$2,'kyc','aprobada','fixture:'||$3,'retention-purge:'||$3,$4,$5,$5)`, verificationID, ownerID, verificationID, adminID, resolvedAt); err != nil {
		t.Fatalf("insert synthetic due verification: %v", err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en)
		VALUES($1,'kyc',$2,'elegible',$3)`, ownerID, verificationID, resolvedAt); err != nil {
		t.Fatalf("insert synthetic eligible projection: %v", err)
	}

	holder, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	holderCommitted := false
	defer func() {
		if !holderCommitted {
			_ = holder.Rollback(context.Background())
		}
	}()
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, ownerID); err != nil {
		t.Fatalf("hold owner account advisory lock: %v", err)
	}
	var holderPID int32
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	observer, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect separate retention lock observer: %v", err)
	}
	defer func() {
		if err := observer.Close(context.Background()); err != nil {
			t.Errorf("close retention lock observer: %v", err)
		}
	}()

	writerAppName := "retention-pub-" + uuid.NewString()
	writerConfig := runtime.Config().Copy()
	writerConfig.ConnConfig.RuntimeParams["application_name"] = writerAppName
	writerPool, err := pgxpool.NewWithConfig(ctx, writerConfig)
	if err != nil {
		t.Fatalf("create publication writer pool: %v", err)
	}
	defer writerPool.Close()
	publicationService, err := adminglobal.New(writerPool, func() time.Time { return now.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	writerCtx, cancelWriter := context.WithTimeout(ctx, 5*time.Second)
	writerDone := make(chan error, 1)
	go func() {
		_, err := publicationService.SetPublicationState(writerCtx, adminID, spaceID, uuid.NewString(), adminglobal.PublicationStateInput{
			State:             "activa",
			ExpectedUpdatedAt: updatedAt,
			Reason:            "revision_administrativa",
		})
		writerDone <- err
	}()
	writerReceived := false
	defer func() {
		cancelWriter()
		if !writerReceived {
			select {
			case <-writerDone:
			case <-time.After(5 * time.Second):
				t.Error("publication writer did not stop after cancellation")
			}
		}
	}()
	writerPID, err := waitForPostgresWriterBlockedByQuery(writerCtx, observer, holderPID, writerAppName, "SELECT pg_advisory_xact_lock(hashtextextended")
	if err != nil {
		select {
		case writerErr := <-writerDone:
			writerReceived = true
			t.Fatalf("publication activation did not wait on an account lock; writer returned: %v; observer: %v", writerErr, err)
		default:
		}
		diagnosticCtx, cancelDiagnostic := context.WithTimeout(context.Background(), time.Second)
		defer cancelDiagnostic()
		diagnosticConn, diagnosticConnectErr := pgx.Connect(diagnosticCtx, adminURL)
		if diagnosticConnectErr != nil {
			t.Fatalf("publication activation must wait on an account lock: %v; diagnostic connection: %v", err, diagnosticConnectErr)
		}
		defer diagnosticConn.Close(context.Background())
		rows, diagnosticErr := diagnosticConn.Query(diagnosticCtx, `SELECT pid,application_name,state,COALESCE(wait_event_type,''),COALESCE(wait_event,''),pg_blocking_pids(pid)::text,query FROM pg_catalog.pg_stat_activity WHERE datname=current_database() AND pid<>$1 ORDER BY pid`, holderPID)
		var activity strings.Builder
		if diagnosticErr == nil {
			defer rows.Close()
			for rows.Next() {
				var pid int32
				var appName, state, waitType, waitEvent, blockers, query string
				if scanErr := rows.Scan(&pid, &appName, &state, &waitType, &waitEvent, &blockers, &query); scanErr != nil {
					diagnosticErr = scanErr
					break
				}
				fmt.Fprintf(&activity, " pid=%d app=%q state=%q wait=%q/%q blockers=%s query=%q;", pid, appName, state, waitType, waitEvent, blockers, query)
			}
			if rows.Err() != nil {
				diagnosticErr = rows.Err()
			}
		}
		t.Fatalf("publication activation must wait on an account lock: %v; activity=%s diagnostic_error=%v", err, activity.String(), diagnosticErr)
	}
	var ownerLockWait bool
	if err := observer.QueryRow(writerCtx, `SELECT EXISTS (
		SELECT 1 FROM pg_catalog.pg_stat_activity a
		JOIN pg_catalog.pg_locks waiting ON waiting.pid=a.pid
		JOIN pg_catalog.pg_locks held ON held.locktype=waiting.locktype
			AND held.database IS NOT DISTINCT FROM waiting.database
			AND held.classid=waiting.classid AND held.objid=waiting.objid AND held.objsubid=waiting.objsubid
		WHERE a.pid=$1 AND a.query LIKE $4 || '%' AND NOT waiting.granted AND waiting.locktype='advisory' AND waiting.objsubid=1
			AND held.pid=$2 AND held.granted
			AND waiting.classid=((hashtextextended($3::text,0)>>32)&4294967295)::oid
			AND waiting.objid=(hashtextextended($3::text,0)&4294967295)::oid
	)`, writerPID, holderPID, ownerID, "SELECT pg_advisory_xact_lock(hashtextextended").Scan(&ownerLockWait); err != nil || !ownerLockWait {
		t.Fatalf("publication writer PID %d must wait on the owner's advisory lock: waiting=%t err=%v", writerPID, ownerLockWait, err)
	}
	if writerPID == holderPID {
		t.Fatalf("publication writer PID %d must differ from lock holder", writerPID)
	}
	select {
	case err := <-writerDone:
		writerReceived = true
		t.Fatalf("publication activation completed before retention purge: %v", err)
	default:
	}

	retentionRepo := identitypg.NewIdentityRepository(runtime)
	result, err := retentionRepo.PrepareExpiredLocalRetentions(ctx, now.Add(time.Hour), 20)
	if err != nil || result.VerificationsPurged != 1 {
		t.Fatalf("due KYC retention purge result=%+v err=%v", result, err)
	}
	var remainingCase, remainingEligibility int
	if err := admin.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM public.verificacion WHERE id=$1),
		(SELECT count(*) FROM public.elegibilidad_verificacion_local WHERE verificacion_id=$1)`, verificationID).Scan(&remainingCase, &remainingEligibility); err != nil || remainingCase != 0 || remainingEligibility != 0 {
		t.Fatalf("retention purge readback: case=%d eligibility=%d err=%v", remainingCase, remainingEligibility, err)
	}
	select {
	case err := <-writerDone:
		writerReceived = true
		t.Fatalf("publication activation must remain blocked until account lock release, got %v", err)
	default:
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatalf("release owner account advisory lock: %v", err)
	}
	holderCommitted = true
	select {
	case err := <-writerDone:
		writerReceived = true
		if !errors.Is(err, adminglobal.ErrConflict) {
			t.Fatalf("publication activation after expired KYC purge returned %v; want conflict", err)
		}
	case <-writerCtx.Done():
		t.Fatalf("publication activation did not finish after retention purge and lock release: %v", writerCtx.Err())
	}
	var state string
	if err := admin.QueryRow(ctx, `SELECT estado FROM public.espacio WHERE id=$1`, spaceID).Scan(&state); err != nil || state != "borrador" {
		t.Fatalf("rejected activation changed publication state=%q err=%v", state, err)
	}
}

func assertAdminLocalBlockSerializesWithQuoteLock(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn, runtime *pgxpool.Pool, quoteRepo *rentalflowpg.Repository, adminID, renterID string, listing rentalflow.Listing, interval rentalflow.Interval, now time.Time, nowFn func() time.Time) {
	t.Helper()
	appName := "commercial-admin-local-block-" + uuid.NewString()
	blockConfig := runtime.Config().Copy()
	blockConfig.MaxConns = 1
	blockConfig.ConnConfig.RuntimeParams["application_name"] = appName
	blockPool, err := pgxpool.NewWithConfig(ctx, blockConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer blockPool.Close()
	service, err := adminlocal.New(blockPool, nowFn)
	if err != nil {
		t.Fatal(err)
	}

	holder, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = holder.Rollback(context.Background())
		}
	}()
	var holderPID int32
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, renterID); err != nil {
		t.Fatal(err)
	}
	observer, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close(context.Background())

	finished := make(chan error, 1)
	go func() {
		_, err := service.BlockAccount(ctx, adminID, renterID, "revision_administrativa", uuid.NewString())
		finished <- err
	}()
	const waitingSQL = `SELECT EXISTS (
    SELECT 1 FROM pg_catalog.pg_stat_activity a
    WHERE a.wait_event_type='Lock'
      AND a.query LIKE '%pg_advisory_xact_lock(hashtextextended%'
      AND $1=ANY(pg_catalog.pg_blocking_pids(a.pid))
)`
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	waiting := false
	for !waiting {
		select {
		case err := <-finished:
			t.Fatalf("administrative account block completed before acquiring the quote account lock: %v", err)
		default:
		}
		if err := observer.QueryRow(ctx, waitingSQL, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("administrative account block completed before acquiring the quote account lock: %v", err)
		case <-ticker.C:
		case <-timer.C:
			var activity string
			_ = observer.QueryRow(ctx, `SELECT COALESCE(string_agg(format('pid=%s wait=%s query=%s blockers=%s',pid,wait_event_type,query,pg_blocking_pids(pid)), E'\n'), '<none>') FROM pg_catalog.pg_stat_activity WHERE application_name=$1`, appName).Scan(&activity)
			t.Fatalf("administrative account block did not wait on the exact renter advisory lock; activity: %s", activity)
		case <-ctx.Done():
			t.Fatalf("waiting for administrative account block: %v", ctx.Err())
		}
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	committed = true
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("administrative account block after lock release: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("administrative account block did not finish after lock release: %v", ctx.Err())
	}
	var blocked bool
	if err := observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, renterID).Scan(&blocked); err != nil || !blocked {
		t.Fatalf("administrative block readback=%v err=%v; want blocked", blocked, err)
	}
	if _, err := quoteRepo.CreateValidatedQuote(ctx, renterID, listing, interval, 1, listing.UnitPriceCLP, now); !errors.Is(err, rentalflow.ErrNotFound) {
		t.Fatalf("quote creation after administrative account block returned %v; want ErrNotFound", err)
	}
}

func assertAuthenticationLookupSerializesWithQuoteLock(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn, quoteRepo *rentalflowpg.Repository, renterID string, lookup identity.AccountLookup, blockState bool, listing rentalflow.Listing, interval rentalflow.Interval, now time.Time) {
	t.Helper()
	appName := "commercial-login-state-" + uuid.NewString()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.User("espacigo_api_login")
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	authRepo := identitypg.NewIdentityRepository(pool)

	holder, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	holderCommitted := false
	defer func() {
		if !holderCommitted {
			_ = holder.Rollback(context.Background())
		}
	}()
	var holderPID int32
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, renterID); err != nil {
		t.Fatal(err)
	}
	observer, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close(context.Background())

	blockedUntil := now.Add(30 * time.Minute)
	finished := make(chan error, 1)
	go func() {
		finished <- authRepo.WithLockedAccount(ctx, lookup, func(account identity.Account, tx identity.AuthenticationTransaction) error {
			if account.ID != renterID {
				return errors.New("authentication lookup resolved an unexpected account")
			}
			if blockState {
				return tx.SaveLoginState(ctx, account.ID, identity.AccountBlocked, 5, &blockedUntil)
			}
			return nil
		})
	}()

	const waitingSQL = `SELECT EXISTS (
    SELECT 1 FROM pg_catalog.pg_stat_activity a
    WHERE a.wait_event_type='Lock'
      AND a.query LIKE '%pg_advisory_xact_lock(hashtextextended%'
      AND $1=ANY(pg_catalog.pg_blocking_pids(a.pid))
)`
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	waiting := false
	for !waiting {
		select {
		case err := <-finished:
			t.Fatalf("authentication transaction completed before acquiring the quote account lock: %v", err)
		default:
		}
		if err := observer.QueryRow(ctx, waitingSQL, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("authentication transaction completed before acquiring the quote account lock: %v", err)
		case <-ticker.C:
		case <-timer.C:
			var activity string
			_ = observer.QueryRow(ctx, `SELECT COALESCE(string_agg(format('pid=%s wait=%s query=%s blockers=%s',pid,wait_event_type,query,pg_blocking_pids(pid)), E'\n'), '<none>') FROM pg_catalog.pg_stat_activity WHERE application_name=$1`, appName).Scan(&activity)
			t.Fatalf("authentication state update did not wait on the exact renter advisory lock; activity: %s", activity)
		case <-ctx.Done():
			t.Fatalf("waiting for authentication state lock: %v", ctx.Err())
		}
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	holderCommitted = true
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("save blocked login state: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("authentication state update did not finish after releasing lock: %v", ctx.Err())
	}

	if blockState {
		var state string
		if err := observer.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1`, renterID).Scan(&state); err != nil || state != string(identity.AccountBlocked) {
			t.Fatalf("login state readback=%q err=%v; want blocked", state, err)
		}
		if _, err := quoteRepo.CreateValidatedQuote(ctx, renterID, listing, interval, 1, listing.UnitPriceCLP, now); !errors.Is(err, rentalflow.ErrNotFound) {
			t.Fatalf("quote creation after serialized account block returned %v; want ErrNotFound", err)
		}
	}
}

func assertCommercialQuoteWriterWaitsForSharedLock(t *testing.T, ctx context.Context, adminURL string, runtime *pgxpool.Pool, quoteRepo *rentalflowpg.Repository, ownerID, renterID, spaceID string, listing rentalflow.Listing, interval rentalflow.Interval, now time.Time, writerName, writerQueryPrefix string, expectedPrice int64, writeRate func(context.Context, *pgxpool.Pool) error) {
	t.Helper()
	lockTx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lockTx.Rollback(context.Background()) }()
	var lockedSpaceID string
	if err := lockTx.QueryRow(ctx, `SELECT id::text FROM public.espacio WHERE id=$1 AND estado='activa' FOR SHARE`, spaceID).Scan(&lockedSpaceID); err != nil {
		t.Fatalf("acquire quote-style shared space lock: %v", err)
	}
	var blockerPID int32
	if err := lockTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}
	observer, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect isolated lock observer: %v", err)
	}
	defer func() {
		if err := observer.Close(context.Background()); err != nil {
			t.Errorf("close isolated lock observer: %v", err)
		}
	}()
	writerAppName := "commercial-quote-writer-" + uuid.NewString()
	writerConfig := runtime.Config().Copy()
	writerConfig.ConnConfig.RuntimeParams["application_name"] = writerAppName
	writerPool, err := pgxpool.NewWithConfig(ctx, writerConfig)
	if err != nil {
		t.Fatalf("create isolated %s pool: %v", writerName, err)
	}
	defer writerPool.Close()

	concurrentCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- writeRate(concurrentCtx, writerPool)
	}()
	writerReceived := false
	defer func() {
		cancel()
		if !writerReceived {
			select {
			case <-writerDone:
			case <-time.After(5 * time.Second):
				t.Error("writer goroutine did not stop after cancellation")
			}
		}
	}()

	writerPID, err := waitForPostgresWriterBlockedByQuery(concurrentCtx, observer, blockerPID, writerAppName, writerQueryPrefix)
	if err != nil {
		t.Fatalf("%s must wait on the quote-style shared space lock: %v", writerName, err)
	}
	if writerPID == blockerPID {
		t.Fatalf("writer backend PID %d must differ from the shared-lock holder", writerPID)
	}
	select {
	case err := <-writerDone:
		writerReceived = true
		t.Fatalf("%s completed before the quote released its shared space lock: %v", writerName, err)
	default:
	}

	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("release quote-style shared space lock: %v", err)
	}
	select {
	case err := <-writerDone:
		writerReceived = true
		if err != nil {
			t.Fatalf("%s after shared space lock release: %v", writerName, err)
		}
	case <-concurrentCtx.Done():
		t.Fatalf("%s did not finish after shared space lock release: %v", writerName, concurrentCtx.Err())
	}
	currentListing, err := quoteRepo.PublicListing(ctx, spaceID)
	if err != nil || currentListing.UnitPriceCLP != expectedPrice {
		t.Fatalf("%s readback: price=%d want=%d err=%v", writerName, currentListing.UnitPriceCLP, expectedPrice, err)
	}
	if _, err := quoteRepo.CreateValidatedQuote(ctx, renterID, listing, interval, 1, listing.UnitPriceCLP, now); !errors.Is(err, rentalflow.ErrConflict) {
		t.Fatalf("stale quote snapshot must reject the newly written tariff: %v", err)
	}
}

func assertCreateValidatedQuoteSerializesWithGlobalPublicationUpdate(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn, runtime *pgxpool.Pool, quoteRepo *rentalflowpg.Repository, ownerID, renterID, adminID, spaceID string, interval rentalflow.Interval, now time.Time) {
	t.Helper()
	listing, err := quoteRepo.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	liveListing, err := quoteRepo.PublicListing(ctx, spaceID)
	if err != nil || liveListing != listing {
		t.Fatalf("quote fixture listing is not stable before concurrency: first=%+v second=%+v err=%v", listing, liveListing, err)
	}
	concurrencyInterval := rentalflow.Interval{StartAt: interval.EndAt, EndAt: interval.EndAt.AddDate(0, 1, 0)}
	dateText := concurrencyInterval.StartAt.In(time.UTC).Format("2006-01-02")
	available, err := quoteRepo.PublicIntervals(ctx, listing, dateText, 1)
	if err != nil {
		t.Fatalf("preflight quote interval availability: %v", err)
	}
	intervalAvailable := false
	for _, candidate := range available {
		if candidate.StartAt.Equal(concurrencyInterval.StartAt) && candidate.EndAt.Equal(concurrencyInterval.EndAt) {
			intervalAvailable = true
			break
		}
	}
	if !intervalAvailable {
		t.Fatalf("concurrency fixture interval [%s,%s) is not in available intervals: %+v", concurrencyInterval.StartAt, concurrencyInterval.EndAt, available)
	}
	var expectedUpdatedAt time.Time
	if err := runtime.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, spaceID).Scan(&expectedUpdatedAt); err != nil {
		t.Fatal(err)
	}

	const barrierKey1 int32 = 1936024434
	const barrierKey2 int32 = 1397056561
	const barrierFunction = `CREATE OR REPLACE FUNCTION public.test_commercial_quote_insert_barrier() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(1936024434, 1397056561);
    RETURN NEW;
END;
$$`
	const barrierTrigger = `CREATE TRIGGER test_commercial_quote_insert_barrier BEFORE INSERT ON public.cotizacion_arriendo_comercial FOR EACH ROW EXECUTE FUNCTION public.test_commercial_quote_insert_barrier()`
	defer func() {
		if _, err := admin.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_commercial_quote_insert_barrier ON public.cotizacion_arriendo_comercial`); err != nil {
			t.Errorf("drop disposable quote barrier trigger: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DROP FUNCTION IF EXISTS public.test_commercial_quote_insert_barrier()`); err != nil {
			t.Errorf("drop disposable quote barrier function: %v", err)
		}
	}()
	if _, err := admin.Exec(ctx, barrierFunction); err != nil {
		t.Fatalf("create disposable quote barrier function: %v", err)
	}
	if _, err := admin.Exec(ctx, barrierTrigger); err != nil {
		t.Fatalf("create disposable quote barrier trigger: %v", err)
	}

	barrierTx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = barrierTx.Rollback(context.Background()) }()
	if _, err := barrierTx.Exec(ctx, `SELECT pg_advisory_xact_lock($1,$2)`, barrierKey1, barrierKey2); err != nil {
		t.Fatalf("hold disposable quote insert barrier: %v", err)
	}
	var barrierPID int32
	if err := barrierTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&barrierPID); err != nil {
		t.Fatal(err)
	}
	observer, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect disposable quote observer: %v", err)
	}
	defer func() {
		if err := observer.Close(context.Background()); err != nil {
			t.Errorf("close disposable quote observer: %v", err)
		}
	}()

	quoteAppName := "commercial-quote-create-" + uuid.NewString()
	quoteConfig := runtime.Config().Copy()
	quoteConfig.ConnConfig.RuntimeParams["application_name"] = quoteAppName
	quotePool, err := pgxpool.NewWithConfig(ctx, quoteConfig)
	if err != nil {
		t.Fatalf("create dedicated quote pool: %v", err)
	}
	defer quotePool.Close()
	quoteWriter, err := rentalflowpg.NewRepository(quotePool, quotePool, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	type quoteOutcome struct {
		quote rentalflow.Quote
		err   error
	}
	quoteCtx, cancelQuote := context.WithTimeout(ctx, 5*time.Second)
	quoteDone := make(chan quoteOutcome, 1)
	go func() {
		quote, err := quoteWriter.CreateValidatedQuote(quoteCtx, renterID, listing, concurrencyInterval, 1, listing.UnitPriceCLP, now)
		quoteDone <- quoteOutcome{quote: quote, err: err}
	}()
	quoteReceived := false
	defer func() {
		cancelQuote()
		if !quoteReceived {
			select {
			case <-quoteDone:
			case <-time.After(5 * time.Second):
				t.Error("CreateValidatedQuote goroutine did not stop after cancellation")
			}
		}
	}()
	quotePID, err := waitForPostgresWriterBlockedByQuery(quoteCtx, observer, barrierPID, quoteAppName, `INSERT INTO public.cotizacion_arriendo_comercial (`)
	if err != nil {
		cancelQuote()
		select {
		case result := <-quoteDone:
			quoteReceived = true
			t.Fatalf("CreateValidatedQuote did not reach the insert barrier (%v); early result err=%v quote=%+v", err, result.err, result.quote)
		case <-time.After(5 * time.Second):
			t.Fatalf("CreateValidatedQuote did not reach the insert barrier (%v) and failed to stop after cancellation", err)
		}
	}

	writerAppName := "commercial-q-writer-" + uuid.NewString()
	writerConfig := runtime.Config().Copy()
	writerConfig.ConnConfig.RuntimeParams["application_name"] = writerAppName
	writerPool, err := pgxpool.NewWithConfig(ctx, writerConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer writerPool.Close()
	service, err := adminglobal.New(writerPool, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	globalPrice := "240000"
	correlationID := uuid.NewString()
	writerCtx, cancelWriter := context.WithTimeout(ctx, 5*time.Second)
	writerDone := make(chan error, 1)
	go func() {
		_, err := service.UpdatePublication(writerCtx, adminID, spaceID, correlationID, adminglobal.PublicationContentUpdate{
			BasePriceCLP:      &globalPrice,
			ExpectedUpdatedAt: expectedUpdatedAt,
			Reason:            "revision_administrativa",
		})
		writerDone <- err
	}()
	writerReceived := false
	defer func() {
		cancelWriter()
		if !writerReceived {
			select {
			case <-writerDone:
			case <-time.After(5 * time.Second):
				t.Error("global publication writer goroutine did not stop after cancellation")
			}
		}
	}()
	writerPID, err := waitForPostgresWriterBlockedByAccountLock(writerCtx, observer, quotePID, writerAppName, ownerID)
	if err != nil {
		t.Fatalf("global publication writer must wait on the quote's owner advisory lock: %v", err)
	}
	if writerPID == quotePID {
		t.Fatalf("writer backend PID %d must differ from the quote backend", writerPID)
	}
	select {
	case err := <-writerDone:
		writerReceived = true
		t.Fatalf("global publication writer completed before the quote transaction committed: %v", err)
	default:
	}
	if err := barrierTx.Commit(ctx); err != nil {
		t.Fatalf("release disposable quote insert barrier: %v", err)
	}

	var created rentalflow.Quote
	select {
	case result := <-quoteDone:
		quoteReceived = true
		if result.err != nil {
			t.Fatalf("CreateValidatedQuote concurrent with a rate update: %v", result.err)
		}
		created = result.quote
	case <-quoteCtx.Done():
		t.Fatalf("CreateValidatedQuote did not finish after barrier release: %v", quoteCtx.Err())
	}
	select {
	case err := <-writerDone:
		writerReceived = true
		if err != nil {
			t.Fatalf("global publication writer after quote commit: %v", err)
		}
	case <-writerCtx.Done():
		t.Fatalf("global publication writer did not finish after quote commit: %v", writerCtx.Err())
	}
	if created.UnitPriceCLP != listing.UnitPriceCLP || created.SubtotalCLP != listing.UnitPriceCLP {
		t.Fatalf("concurrent quote snapshot price=%d subtotal=%d, want original price %d", created.UnitPriceCLP, created.SubtotalCLP, listing.UnitPriceCLP)
	}
	readback, err := quoteRepo.GetForParticipant(ctx, renterID, created.ID)
	if err != nil || readback.UnitPriceCLP != listing.UnitPriceCLP || readback.SubtotalCLP != listing.UnitPriceCLP {
		t.Fatalf("persisted quote snapshot price=%d subtotal=%d, want %d; err=%v", readback.UnitPriceCLP, readback.SubtotalCLP, listing.UnitPriceCLP, err)
	}
	updatedListing, err := quoteRepo.PublicListing(ctx, spaceID)
	if err != nil || updatedListing.UnitPriceCLP != 240000 {
		t.Fatalf("global price readback=%d, want 240000; err=%v", updatedListing.UnitPriceCLP, err)
	}
}

func waitForPostgresWriterBlockedByAccountLock(ctx context.Context, observer *pgx.Conn, blockerPID int32, writerAppName, accountID string) (int32, error) {
	deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var writerPID int32
		err := observer.QueryRow(deadline, `SELECT a.pid
FROM pg_catalog.pg_stat_activity a
JOIN pg_catalog.pg_locks waiting ON waiting.pid=a.pid
JOIN pg_catalog.pg_locks held ON held.pid=$1
  AND held.locktype=waiting.locktype
  AND held.database IS NOT DISTINCT FROM waiting.database
  AND held.classid=waiting.classid AND held.objid=waiting.objid AND held.objsubid=waiting.objsubid
  AND held.granted
WHERE a.datname=current_database() AND a.application_name=$2 AND a.pid<>$1
  AND a.wait_event_type='Lock' AND $1=ANY(pg_catalog.pg_blocking_pids(a.pid))
  AND waiting.locktype='advisory' AND NOT waiting.granted AND waiting.objsubid=1
  AND waiting.classid=((hashtextextended($3::text,0)>>32)&4294967295)::oid
  AND waiting.objid=(hashtextextended($3::text,0)&4294967295)::oid
ORDER BY a.pid LIMIT 1`, blockerPID, writerAppName, accountID).Scan(&writerPID)
		if err == nil {
			return writerPID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
		select {
		case <-deadline.Done():
			var state, waitType, waitEvent, query, blockers string
			diagnosticCtx, cancelDiagnostic := context.WithTimeout(context.Background(), time.Second)
			defer cancelDiagnostic()
			diagnosticErr := observer.QueryRow(diagnosticCtx, `SELECT state,COALESCE(wait_event_type,''),COALESCE(wait_event,''),query,pg_blocking_pids(pid)::text
				FROM pg_catalog.pg_stat_activity WHERE application_name=$1 ORDER BY pid LIMIT 1`, writerAppName).Scan(&state, &waitType, &waitEvent, &query, &blockers)
			if diagnosticErr != nil {
				return 0, fmt.Errorf("timed out waiting for account advisory lock; writer diagnostic: %w", diagnosticErr)
			}
			return 0, fmt.Errorf("timed out waiting for account advisory lock; writer state=%q wait=%q/%q blockers=%s query=%q", state, waitType, waitEvent, blockers, query)
		case <-ticker.C:
		}
	}
}

func waitForPostgresWriterBlockedByQuery(ctx context.Context, observer *pgx.Conn, blockerPID int32, writerAppName, queryPrefix string) (int32, error) {
	deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var writerPID int32
		err := observer.QueryRow(deadline, `SELECT pid FROM pg_catalog.pg_stat_activity WHERE datname=current_database() AND application_name=$2 AND pid<>$1 AND wait_event_type='Lock' AND $1=ANY(pg_catalog.pg_blocking_pids(pid)) AND query LIKE $3 || '%' ORDER BY pid LIMIT 1`, blockerPID, writerAppName, queryPrefix).Scan(&writerPID)
		if err == nil {
			return writerPID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			var state, waitType, waitEvent, query, blockers string
			diagnosticCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			diagnosticErr := observer.QueryRow(diagnosticCtx, `SELECT state,COALESCE(wait_event_type,''),COALESCE(wait_event,''),query,pg_blocking_pids(pid)::text FROM pg_catalog.pg_stat_activity WHERE application_name=$1 ORDER BY pid LIMIT 1`, writerAppName).Scan(&state, &waitType, &waitEvent, &query, &blockers)
			cancel()
			if diagnosticErr != nil {
				return 0, fmt.Errorf("writer observation query failed: %w; follow-up diagnostic: %v", err, diagnosticErr)
			}
			return 0, fmt.Errorf("writer observation query failed: %w; writer state=%q wait=%q/%q blockers=%s query=%q", err, state, waitType, waitEvent, blockers, query)
		}
		select {
		case <-deadline.Done():
			return 0, errors.New("timed out waiting for the writer query to block on quote transaction")
		case <-ticker.C:
		}
	}
}
