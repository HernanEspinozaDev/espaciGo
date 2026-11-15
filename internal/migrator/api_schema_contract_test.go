package migrator

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	documentpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/documentverification"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	catalogpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/publiccatalog"
	rentalflowpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/rentalflow"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Called only within the isolated, empty-account full-schema harness. Real
// repositories exercise old and new SQL projections under the actual runtime
// role; this is not an HTTP E2E or a claim about every previous API image.
func assertRuntimeAPISchemaContract(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
	t.Helper()
	// The shared fixture installs baseline ACL before identity tightening.
	// Replaying the old bootstrap here would conceal revocation regressions.
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
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const absent = "10000000-0000-4000-8000-000000000001"
	if _, err := identitypg.NewIdentityRepository(pool).AccountByID(ctx, absent); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("legacy identity projection is incompatible with current schema/runtime: %v", err)
	}
	categories, err := catalogpg.New(pool).Categories(ctx)
	if err != nil || len(categories) < 8 {
		t.Fatalf("public catalogue projection is incompatible with current schema/runtime: %v", err)
	}
	if _, err := documentpg.New(pool).CurrentForAccount(ctx, absent); !errors.Is(err, identityverification.ErrNotFound) {
		t.Fatalf("current document projection is incompatible with schema/runtime: %v", err)
	}
	const lockQuery = `SELECT reviewer_id::text,capability,scope_id::text FROM public.document_verification_reviewer_grant WHERE reviewer_id=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>clock_timestamp()) ORDER BY capability,scope_id FOR SHARE`
	if _, err := pool.Exec(ctx, lockQuery, absent); err != nil {
		t.Fatalf("V83 runtime row-lock contract is missing: %v", err)
	}
	if _, err := admin.Exec(ctx, "REVOKE UPDATE(id) ON public.document_verification_reviewer_grant FROM espacigo_runtime"); err != nil {
		t.Fatal(err)
	}
	_, rejected := pool.Exec(ctx, lockQuery, absent)
	// Restore only the fixture privilege even when the negative assertion fails.
	if _, err := admin.Exec(ctx, "GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_runtime"); err != nil {
		t.Fatal(err)
	}
	var permissionError *pgconn.PgError
	if !errors.As(rejected, &permissionError) || permissionError.Code != "42501" {
		t.Fatalf("schema/API contract did not catch the missing column-level lock privilege: %v", rejected)
	}
	var tableUpdate, ledgerSelect, createSchema bool
	if err := pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.document_verification_reviewer_grant','UPDATE'),has_table_privilege(current_user,'public.schema_migrations','SELECT'),has_schema_privilege(current_user,'public','CREATE')`).Scan(&tableUpdate, &ledgerSelect, &createSchema); err != nil || tableUpdate || ledgerSelect || createSchema {
		t.Fatalf("runtime gained migration or generic reviewer privileges: %v", err)
	}
	const ownerID = "10000000-0000-4000-8000-000000000095"
	const otherOwnerID = "10000000-0000-4000-8000-000000000096"
	const declarationID = "20000000-0000-4000-8000-000000000095"
	const priorDeclarationID = "20000000-0000-4000-8000-000000000094"
	const otherDeclarationID = "20000000-0000-4000-8000-000000000096"
	t.Cleanup(func() {
		for _, query := range []struct {
			name string
			sql  string
			args []any
		}{
			{"cases", `DELETE FROM public.document_verification_case WHERE subject_id IN ($1,$2)`, []any{ownerID, otherOwnerID}},
			{"declarations", `DELETE FROM public.identity_declaration_revision WHERE account_id IN ($1,$2)`, []any{ownerID, otherOwnerID}},
			{"owners", `DELETE FROM public.usuario WHERE id IN ($1,$2)`, []any{ownerID, otherOwnerID}},
		} {
			if _, err := admin.Exec(context.Background(), query.sql, query.args...); err != nil {
				t.Errorf("cleanup synthetic VI-01 %s: %v", query.name, err)
			}
		}
		var remaining int
		if err := admin.QueryRow(context.Background(), `SELECT
			(SELECT count(*) FROM public.document_verification_case WHERE subject_id IN ($1,$2))
		  + (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id IN ($1,$2))
		  + (SELECT count(*) FROM public.usuario WHERE id IN ($1,$2))`, ownerID, otherOwnerID).Scan(&remaining); err != nil || remaining != 0 {
			t.Errorf("synthetic VI-01 cleanup left %d rows: %v", remaining, err)
		}
	})
	for _, owner := range []struct {
		id    string
		email string
	}{{ownerID, "vi95-owner@example.test"}, {otherOwnerID, "vi95-other-owner@example.test"}} {
		if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
VALUES($1,$2,$2,'synthetic-test-hash','activo')`, owner.id, owner.email); err != nil {
			t.Fatalf("insert synthetic VI-01 owner: %v", err)
		}
	}
	for _, declaration := range []struct {
		revision int64
		id       string
	}{{1, priorDeclarationID}, {2, declarationID}} {
		if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision
(id,account_id,revision,key_version,ciphertext,schema_version,encryption_algorithm,consent_version,consent_given_at,retention_at)
VALUES($1,$2,$3,'vi95-test-key',decode(repeat('01',29),'hex'),'1','AES-256-GCM/v1','vi95-declaration-v1',clock_timestamp(),clock_timestamp()+interval '1 day')`, declaration.id, ownerID, declaration.revision); err != nil {
			t.Fatalf("insert synthetic saved declaration revision %d: %v", declaration.revision, err)
		}
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision
(id,account_id,revision,key_version,ciphertext,schema_version,encryption_algorithm,consent_version,consent_given_at,retention_at)
VALUES($1,$2,1,'vi95-test-key',decode(repeat('01',29),'hex'),'1','AES-256-GCM/v1','vi95-declaration-v1',clock_timestamp(),clock_timestamp()+interval '1 day')`, otherDeclarationID, otherOwnerID); err != nil {
		t.Fatalf("insert synthetic other-owner declaration: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id)
VALUES('30000000-0000-4000-8000-000000000095','identity',$1)`, ownerID)
	var rejectedCaseInsert *pgconn.PgError
	if !errors.As(err, &rejectedCaseInsert) || rejectedCaseInsert.Code != "23514" {
		t.Fatalf("runtime identity insert without separate consent/declaration must fail, got %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.document_verification_case
(id,purpose,subject_id,identity_declaration_revision_id,document_format_declared,capture_consent_version,capture_consented_at)
VALUES('30000000-0000-4000-8000-000000000096','identity',$1,$2,'unsure','identity-capture-local-v1',clock_timestamp())`, ownerID, otherDeclarationID)
	var crossOwnerFKError *pgconn.PgError
	if !errors.As(err, &crossOwnerFKError) || crossOwnerFKError.Code != "23503" {
		t.Fatalf("runtime identity insert bound to another owner's declaration must fail FK, got %v", err)
	}
	service, err := dv.NewService(documentpg.New(pool))
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Start(ctx, ownerID, dv.CreateInput{
		Purpose: dv.PurposeIdentity, DocumentFormatDeclared: dv.DocumentFormatUnsure,
		CaptureConsentGiven: true, CaptureConsentVersion: dv.IdentityCaptureConsentVersion,
	})
	if err != nil {
		t.Fatalf("create identity case from authenticated owner declaration: %v", err)
	}
	loaded, err := service.LoadOwnCase(ctx, ownerID, created.ID)
	if err != nil {
		t.Fatalf("read back identity case: %v", err)
	}
	if loaded.SubjectID != ownerID || loaded.IdentityDeclarationRevisionID != declarationID ||
		loaded.DocumentFormatDeclared != dv.DocumentFormatUnsure ||
		loaded.CaptureConsentVersion != dv.IdentityCaptureConsentVersion || loaded.CaptureConsentedAt.IsZero() {
		t.Fatalf("identity case readback lost owner/declaration/consent binding: %+v", loaded)
	}
	_, err = pool.Exec(ctx, `UPDATE public.document_verification_case SET identity_declaration_revision_id=NULL,state=NULL WHERE id=$1`, created.ID)
	var nullStateLinkError *pgconn.PgError
	if !errors.As(err, &nullStateLinkError) || nullStateLinkError.Code != "23514" {
		t.Fatalf("link guard must fail closed for a NULL terminal-state value, got %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE public.document_verification_case SET identity_declaration_revision_id=$1 WHERE id=$2`, priorDeclarationID, created.ID)
	var linkGuardError *pgconn.PgError
	if !errors.As(err, &linkGuardError) || linkGuardError.Code != "23514" {
		t.Fatalf("runtime active-case declaration rebind must fail with check violation, got %v", err)
	}
	loaded, err = service.LoadOwnCase(ctx, ownerID, created.ID)
	if err != nil || loaded.IdentityDeclarationRevisionID != declarationID {
		t.Fatalf("rejected active-case rebind changed declaration link: %+v err=%v", loaded, err)
	}
	_, err = admin.Exec(ctx, `DELETE FROM public.identity_declaration_revision WHERE id=$1 AND account_id=$2`, declarationID, ownerID)
	var linkedRetentionError *pgconn.PgError
	if !errors.As(err, &linkedRetentionError) || linkedRetentionError.Code != "55P03" {
		t.Fatalf("deleting a declaration linked to an active case must be deferred: %v", err)
	}
	repository := documentpg.New(pool)
	err = repository.Transition(ctx, ownerID, dv.PurposeIdentity, created.ID, loaded.Revision,
		dv.TransitionAudit{Action: "case_transitioned", ResultCode: string(dv.CaseRejected)},
		func(snapshot *dv.Snapshot, _ dv.Authorization) error {
			snapshot.State = dv.CaseRejected
			// This adapter-level contract exercises terminal-link release, not
			// reviewer authorization. The legacy case CHECK requires a reviewer ID
			// for every state beyond draft/submitted.
			snapshot.ReviewerID = ownerID
			return nil
		})
	if err != nil {
		t.Fatalf("terminal identity transition should release the saved declaration link: %v", err)
	}
	loaded, err = service.LoadOwnCase(ctx, ownerID, created.ID)
	if err != nil || loaded.State != dv.CaseRejected || loaded.IdentityDeclarationRevisionID != "" {
		t.Fatalf("terminal case did not release its declaration link: %+v err=%v", loaded, err)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM public.identity_declaration_revision WHERE id=$1 AND account_id=$2`, declarationID, ownerID); err != nil {
		t.Fatalf("terminal case should no longer block declaration retention: %v", err)
	}
	t.Run("public-gallery-sync", func(t *testing.T) {
		assertRuntimePublicGallerySync(t, ctx, admin, pool)
	})
	var hasV97 bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.schema_migrations WHERE version=97)`).Scan(&hasV97); err != nil {
		t.Fatal(err)
	}
	if hasV97 {
		t.Run("commercial-quotes", func(t *testing.T) {
			assertCommercialQuoteSchemaContract(t, ctx, admin)
		})
	}
}

func assertCommercialQuoteSchemaContract(t *testing.T, ctx context.Context, admin *pgx.Conn) {
	t.Helper()
	var tableExists bool
	if err := admin.QueryRow(ctx, `SELECT to_regclass('public.cotizacion_arriendo_comercial') IS NOT NULL`).Scan(&tableExists); err != nil || !tableExists {
		t.Fatalf("commercial quote table missing: %v", err)
	}
	var owner, hasSelect, hasInsert, hasDelete, hasUpdate bool
	if err := admin.QueryRow(ctx, `
SELECT pg_catalog.pg_get_userbyid(c.relowner)='espacigo_schema_owner',
       has_table_privilege('espacigo_runtime',c.oid,'SELECT'),
       has_table_privilege('espacigo_runtime',c.oid,'INSERT'),
       has_table_privilege('espacigo_runtime',c.oid,'DELETE'),
       has_table_privilege('espacigo_runtime',c.oid,'UPDATE')
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='public' AND c.relname='cotizacion_arriendo_comercial'`).Scan(&owner, &hasSelect, &hasInsert, &hasDelete, &hasUpdate); err != nil {
		t.Fatal(err)
	}
	if !owner || !hasSelect || !hasInsert || !hasDelete || hasUpdate {
		t.Fatalf("commercial quote owner/runtime grants differ: owner=%v select=%v insert=%v delete=%v update=%v", owner, hasSelect, hasInsert, hasDelete, hasUpdate)
	}
	var titleSnapshot, expiryColumn bool
	if err := admin.QueryRow(ctx, `SELECT
       EXISTS(SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid='public.cotizacion_arriendo_comercial'::regclass AND attname='titulo_espacio' AND attnum>0 AND NOT attisdropped),
       EXISTS(SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid='public.cotizacion_arriendo_comercial'::regclass AND attname='vence_en' AND attnum>0 AND NOT attisdropped)`).Scan(&titleSnapshot, &expiryColumn); err != nil || !titleSnapshot || !expiryColumn {
		t.Fatalf("commercial quote snapshot/expiry columns missing: title=%v expiry=%v err=%v", titleSnapshot, expiryColumn, err)
	}
	var quoteColumns string
	if err := admin.QueryRow(ctx, `SELECT string_agg(attname, ',' ORDER BY attnum)
FROM pg_catalog.pg_attribute
WHERE attrelid='public.cotizacion_arriendo_comercial'::regclass AND attnum>0 AND NOT attisdropped`).Scan(&quoteColumns); err != nil {
		t.Fatal(err)
	}
	const expectedQuoteColumns = "id,espacio_id,propietario_id,arrendatario_id,titulo_espacio,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en"
	if quoteColumns != expectedQuoteColumns {
		t.Fatalf("commercial quote columns are not the minimized allowlist: got=%q want=%q", quoteColumns, expectedQuoteColumns)
	}
	var accountCascadeCount, spaceCascadeCount int
	if err := admin.QueryRow(ctx, `SELECT
       count(*) FILTER (WHERE confrelid='public.usuario'::regclass),
       count(*) FILTER (WHERE confrelid='public.espacio'::regclass)
FROM pg_catalog.pg_constraint
WHERE conrelid='public.cotizacion_arriendo_comercial'::regclass
  AND contype='f' AND confdeltype='c'`).Scan(&accountCascadeCount, &spaceCascadeCount); err != nil {
		t.Fatal(err)
	}
	if accountCascadeCount != 2 || spaceCascadeCount != 1 {
		t.Fatalf("commercial quote cascade FKs differ: account=%d space=%d", accountCascadeCount, spaceCascadeCount)
	}
}

func assertCommercialQuoteSuppression(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
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
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["application_name"] = "commercial-quote-suppression-test"
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	ownerID, renterID, adminID, spaceID, quoteID, quoteFirstRenterID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, account := range []struct{ id, email string }{
		{ownerID, ownerID + "@commercial-quote.invalid"},
		{renterID, renterID + "@commercial-quote.invalid"},
		{adminID, adminID + "@commercial-quote.invalid"},
		{quoteFirstRenterID, quoteFirstRenterID + "@commercial-quote.invalid"},
	} {
		if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-only','activo')`, account.id, account.email); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
		VALUES($1,$2,'oficina','Commercial quote suppression fixture',repeat('Descripción sintética. ',5),10,1,'Solo ensayo','hora',8000,'Dirección sintética','activa','UTC')`, spaceID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	// Characterize suppression initiated by the commercial quote's owner with
	// an independent quote/account fixture; the renter-initiated case below
	// remains unchanged.
	ownerActorID, ownerActorRenterID, ownerActorSpaceID, ownerActorQuoteID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, account := range []struct{ id, email string }{
		{ownerActorID, ownerActorID + "@commercial-quote.invalid"},
		{ownerActorRenterID, ownerActorRenterID + "@commercial-quote.invalid"},
	} {
		if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-only','activo')`, account.id, account.email); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
		VALUES($1,$2,'oficina','Owner suppression fixture',repeat('Descripción sintética. ',5),10,1,'Solo ensayo','hora',8000,'Dirección sintética','borrador','UTC')`, ownerActorSpaceID, ownerActorID); err != nil {
		t.Fatal(err)
	}
	ownerActorCreatedAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	if _, err := admin.Exec(ctx, `INSERT INTO public.cotizacion_arriendo_comercial(
		id,espacio_id,propietario_id,arrendatario_id,titulo_espacio,tarifa_version,modalidad,precio_unitario_clp,moneda,
		unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en)
		VALUES($1,$2,$3,$4,'Owner suppression fixture',1,'hora',8000,'CLP',1,8000,$5,$6,'UTC',$7,$8)`,
		ownerActorQuoteID, ownerActorSpaceID, ownerActorID, ownerActorRenterID, ownerActorCreatedAt.Add(time.Hour), ownerActorCreatedAt.Add(2*time.Hour), ownerActorCreatedAt, ownerActorCreatedAt.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(identitypg.NewIdentityRepository(runtime))
	if err != nil {
		t.Fatal(err)
	}
	ownerRequest, err := service.RequestRight(ctx, ownerActorID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	ownerReviewAt := time.Now().UTC().Truncate(time.Microsecond)
	ownerReview, err := service.ReviewSuppression(ctx, adminID, ownerRequest.ID, "commercial-quote-owner-suppression-review", "commercial-quote-owner-suppression", func() time.Time { return ownerReviewAt })
	if err != nil || ownerReview.Outcome != "elegible" {
		t.Fatalf("review eligible quote owner suppression: outcome=%s err=%v", ownerReview.Outcome, err)
	}
	ownerExecuteAt := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	ownerResult, err := service.ExecuteSuppression(ctx, adminID, ownerRequest.ID, "commercial-quote-owner-suppression", "commercial-quote-owner-suppression", func() time.Time { return ownerExecuteAt }, nil)
	if err != nil || (ownerResult.Status != "completada" && ownerResult.Status != "limpieza_pendiente") {
		t.Fatalf("execute eligible quote owner suppression: status=%s err=%v", ownerResult.Status, err)
	}
	var ownerQuoteRemaining int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE id=$1`, ownerActorQuoteID).Scan(&ownerQuoteRemaining); err != nil || ownerQuoteRemaining != 0 {
		t.Fatalf("owner suppression retained commercial quote: remaining=%d err=%v", ownerQuoteRemaining, err)
	}

	createdAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	if _, err := admin.Exec(ctx, `INSERT INTO public.cotizacion_arriendo_comercial(
		id,espacio_id,propietario_id,arrendatario_id,titulo_espacio,tarifa_version,modalidad,precio_unitario_clp,moneda,
		unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en)
		VALUES($1,$2,$3,$4,'Commercial quote suppression fixture',1,'hora',8000,'CLP',1,8000,$5,$6,'UTC',$7,$8)`,
		quoteID, spaceID, ownerID, renterID, createdAt.Add(time.Hour), createdAt.Add(2*time.Hour), createdAt, createdAt.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	request, err := service.RequestRight(ctx, renterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	reviewAt := time.Now().UTC().Truncate(time.Microsecond)
	review, err := service.ReviewSuppression(ctx, adminID, request.ID, "commercial-quote-suppression-review", "commercial-quote-suppression", func() time.Time { return reviewAt })
	if err != nil || review.Outcome != "elegible" {
		t.Fatalf("review eligible account suppression: outcome=%s err=%v", review.Outcome, err)
	}
	executeAt := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	result, err := assertSuppressionSerializesCommercialQuote(t, ctx, adminURL, admin, service, adminID, request.ID, renterID, spaceID, executeAt)
	if err != nil || (result.Status != "completada" && result.Status != "limpieza_pendiente") {
		t.Fatalf("execute eligible account suppression: status=%s err=%v", result.Status, err)
	}
	var remaining int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE id=$1`, quoteID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("account suppression retained commercial quote: remaining=%d err=%v", remaining, err)
	}
	assertQuoteBeforeSuppressionDeletesCommercialQuote(t, ctx, adminURL, admin, service, adminID, quoteFirstRenterID, spaceID)

	// A request authenticated before suppression may reach quote persistence
	// afterward. The adapter must revalidate both participant account states
	// while holding the account advisory locks shared with suppression, rather
	// than allowing a new quote for a retired account.
	quoteRepository, err := rentalflowpg.NewRepository(runtime, runtime, func() time.Time { return executeAt })
	if err != nil {
		t.Fatal(err)
	}
	listing, err := quoteRepository.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(executeAt.Year(), executeAt.Month(), executeAt.Day()+1, 12, 0, 0, 0, time.UTC)
	if _, err := quoteRepository.CreateValidatedQuote(ctx, renterID, listing, rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, 1, 8000, executeAt); err == nil {
		t.Fatal("quote creation succeeded for an account after suppression")
	}
	var renterQuotes int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE arrendatario_id=$1`, renterID).Scan(&renterQuotes); err != nil || renterQuotes != 0 {
		t.Fatalf("quote persisted for suppressed renter: count=%d err=%v", renterQuotes, err)
	}
}
