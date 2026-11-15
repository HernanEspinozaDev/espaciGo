package migrator

import (
	"context"
	"errors"
	"net/url"
	"testing"

	documentpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/documentverification"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	catalogpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/publiccatalog"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Called only within the isolated, empty-account full-schema harness. Real
// repositories exercise old and new SQL projections under the actual runtime
// role; this is not an HTTP E2E or a claim about every previous API image.
func assertRuntimeAPISchemaContract(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
	t.Helper()
	if err := dbbootstrap.GrantRuntimePermissions(ctx, admin); err != nil {
		t.Fatal(err)
	}
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
}
