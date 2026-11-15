package migrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	identitydeclarationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixedDeclarationID string

func (id fixedDeclarationID) ID() (string, error) { return string(id), nil }

type v85DeclarationAuthenticator struct{ accountID string }

func (a v85DeclarationAuthenticator) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: a.accountID}, nil
}

type v85SyntheticDiditClient struct {
	sessionID string
	createN   int
	deleteN   int
}

func (c *v85SyntheticDiditClient) CreateSession(_ context.Context, req verification.CreateSessionRequest) (verification.CreateSessionResponse, error) {
	c.createN++
	return verification.CreateSessionResponse{
		SessionID:  c.sessionID,
		SessionURL: "https://verify.didit.me/session/" + req.VendorData,
	}, nil
}

func (c *v85SyntheticDiditClient) GetSessionDecision(_ context.Context, sessionID string) (verification.DiditDecisionResponse, error) {
	return verification.DiditDecisionResponse{
		SessionID:  sessionID,
		Status:     "Approved",
		WorkflowID: "synthetic-workflow",
		IDVerifications: []verification.DiditIDVerificationNode{
			{
				Status:         "Approved",
				IssuingState:   "CHL",
				DocumentType:   "Identity Card",
				PersonalNumber: "11.222.333-9",
				FirstName:      "Synthetic",
				LastName:       "HTTP Person",
				DateOfBirth:    "1990-01-02",
				ExpirationDate: "2032-12-31",
			},
		},
		LivenessChecks: []verification.DiditLivenessNode{
			{Status: "Approved", Score: 95.0},
		},
		FaceMatches: []verification.DiditFaceMatchNode{
			{Status: "Approved", Score: 93.5},
		},
	}, nil
}

func (c *v85SyntheticDiditClient) VerifyWebhookSignature(_ string, _ []byte) bool {
	return true
}

func (c *v85SyntheticDiditClient) DeleteSession(_ context.Context, _ string) error {
	c.deleteN++
	return nil
}

type v85SeqIDGen struct {
	ids []string
	idx int
}

func (g *v85SeqIDGen) ID() (string, error) {
	id := g.ids[g.idx%len(g.ids)]
	g.idx++
	return id, nil
}

// assertV85IdentitySchema tests the new contract only in the disposable harness.
// It neither activates a KYC endpoint nor contacts a provider.
func assertV85IdentitySchema(t *testing.T, ctx context.Context, databaseURL string, admin *pgx.Conn) {
	t.Helper()
	var present bool
	if err := admin.QueryRow(ctx, `SELECT to_regclass('public.identity_declaration_revision') IS NOT NULL AND to_regclass('public.identity_provider_attempt') IS NOT NULL AND to_regclass('public.identity_provider_event') IS NOT NULL`).Scan(&present); err != nil || !present {
		t.Fatalf("V85 encrypted identity schema missing: %v", err)
	}
	var unsafe bool
	if err := admin.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.identity_verification_attempts','INSERT') OR has_table_privilege('espacigo_runtime','public.didit_webhook_inbox','INSERT') OR has_table_privilege('espacigo_runtime','public.identity_declaration_revision','UPDATE') OR has_table_privilege('espacigo_runtime','public.identity_provider_attempt','DELETE') OR has_table_privilege('espacigo_runtime','public.identity_provider_event','DELETE')`).Scan(&unsafe); err != nil || unsafe {
		t.Fatalf("V85 legacy/plaintext or mutable-snapshot authority: %v", err)
	}
	conn := connectTestDB(t, ctx, databaseURL)
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	must := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES ('85000000-0000-4000-8000-000000000001','v85-fixture@example.invalid','v85-fixture@example.invalid','not-a-real-password-hash','activo')`)
	must(`INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES ('85000000-0000-4000-8000-000000000002','v85-other@example.invalid','v85-other@example.invalid','not-a-real-password-hash','activo')`)
	must(`SET LOCAL ROLE espacigo_runtime`)
	ring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": bytes.Repeat([]byte{0x85}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte(`{"first_name":"Synthetic","rut":"11222333-9"}`)
	aad := []byte("identity-declaration:85000000-0000-4000-8000-000000000001:1")
	keyVersion, ciphertext, err := ring.Seal(aad, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	must(`INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,consent_version,consent_given_at,retention_at) VALUES ('85000000-0000-4000-8000-000000000003','85000000-0000-4000-8000-000000000001',1,$1,$2,'test-v1',now(),now()+interval '1 day')`, keyVersion, ciphertext)
	var stored []byte
	if err := tx.QueryRow(ctx, `SELECT ciphertext FROM public.identity_declaration_revision WHERE id='85000000-0000-4000-8000-000000000003'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	opened, err := ring.Open(keyVersion, aad, stored)
	if err != nil || !bytes.Equal(opened, plaintext) || bytes.Contains(stored, plaintext) {
		t.Fatal("encrypted declaration database roundtrip failed")
	}
	must(`INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,workflow_id,workflow_version,idempotency_key,retention_at) VALUES ('85000000-0000-4000-8000-000000000004','85000000-0000-4000-8000-000000000001','85000000-0000-4000-8000-000000000003','didit','SANDBOX','synthetic-app','synthetic-workflow','1','85000000-0000-4000-8000-000000000005',now()+interval '1 day')`)
	must(`INSERT INTO public.identity_provider_event(attempt_id,provider,environment,application_id,event_id,event_type,body_sha256,retention_at) VALUES ('85000000-0000-4000-8000-000000000004','didit','SANDBOX','synthetic-app','synthetic-event','status.updated',decode(repeat('cd',32),'hex'),now()+interval '1 day')`)
	must(`UPDATE public.identity_provider_attempt SET status='IN_REVIEW',updated_at=clock_timestamp(),provider_session_id='synthetic-session' WHERE id='85000000-0000-4000-8000-000000000004'`)
	must(`UPDATE public.identity_provider_event SET processing_status='PROCESSED',processed_at=clock_timestamp() WHERE event_id='synthetic-event'`)
	for _, bad := range []struct{ name, sql string }{
		{"short ciphertext", `INSERT INTO public.identity_declaration_revision(account_id,revision,key_version,ciphertext,consent_version,consent_given_at,retention_at) VALUES ('85000000-0000-4000-8000-000000000001',2,'synthetic-key',decode('ab','hex'),'test-v1',now(),now()+interval '1 day')`},
		{"cross-account declaration", `INSERT INTO public.identity_provider_attempt(account_id,declaration_id,provider,environment,application_id,workflow_id,workflow_version,idempotency_key,retention_at) VALUES ('85000000-0000-4000-8000-000000000002','85000000-0000-4000-8000-000000000003','didit','SANDBOX','synthetic-app','synthetic-workflow','1',gen_random_uuid(),now()+interval '1 day')`},
		{"duplicate active intention", `INSERT INTO public.identity_provider_attempt(account_id,declaration_id,provider,environment,application_id,workflow_id,workflow_version,idempotency_key,retention_at) VALUES ('85000000-0000-4000-8000-000000000001','85000000-0000-4000-8000-000000000003','didit','SANDBOX','synthetic-app','synthetic-workflow','1',gen_random_uuid(),now()+interval '1 day')`},
		{"duplicate event", `INSERT INTO public.identity_provider_event(attempt_id,provider,environment,application_id,event_id,event_type,body_sha256,retention_at) VALUES ('85000000-0000-4000-8000-000000000004','didit','SANDBOX','synthetic-app','synthetic-event','status.updated',decode(repeat('cd',32),'hex'),now()+interval '1 day')`},
		{"wrong event namespace", `INSERT INTO public.identity_provider_event(attempt_id,provider,environment,application_id,event_id,event_type,body_sha256,retention_at) VALUES ('85000000-0000-4000-8000-000000000004','didit','LIVE','synthetic-app','other-event','status.updated',decode(repeat('cd',32),'hex'),now()+interval '1 day')`},
		{"immutable declaration", `UPDATE public.identity_declaration_revision SET revision=2 WHERE id='85000000-0000-4000-8000-000000000003'`},
		{"immutable attempt owner", `UPDATE public.identity_provider_attempt SET account_id='85000000-0000-4000-8000-000000000002' WHERE id='85000000-0000-4000-8000-000000000004'`},
		{"partial encrypted decision", `UPDATE public.identity_provider_attempt SET encrypted_decision=decode(repeat('ab',29),'hex'),decision_key_version='synthetic-key',decision_algorithm=NULL WHERE id='85000000-0000-4000-8000-000000000004'`},
		{"missing processed time", `UPDATE public.identity_provider_event SET processed_at=NULL WHERE event_id='synthetic-event'`},
		{"invalid retention", `INSERT INTO public.identity_provider_event(attempt_id,provider,environment,application_id,event_id,event_type,body_sha256,retention_at) VALUES ('85000000-0000-4000-8000-000000000004','didit','SANDBOX','synthetic-app','old-event','status.updated',decode(repeat('cd',32),'hex'),now()-interval '1 day')`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if _, err := tx.Exec(ctx, `SAVEPOINT invalid_v85`); err != nil {
				t.Fatal(err)
			}
			_, rejected := tx.Exec(ctx, bad.sql)
			want := "23514"
			switch bad.name {
			case "cross-account declaration", "wrong event namespace":
				want = "23503"
			case "duplicate active intention", "duplicate event":
				want = "23505"
			case "immutable declaration", "immutable attempt owner":
				want = "42501"
			}
			var pgErr *pgconn.PgError
			if !errors.As(rejected, &pgErr) || pgErr.Code != want {
				t.Fatalf("unsafe mutation accepted or wrong failure: want %s, got %v", want, rejected)
			}
			if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT invalid_v85`); err != nil {
				t.Fatal(err)
			}
		})
	}
	assertV85DeclarationRepository(t, ctx, databaseURL, admin)
}

func assertV85DeclarationRepository(t *testing.T, ctx context.Context, databaseURL string, admin *pgx.Conn) {
	t.Helper()
	const accountID = "85000000-0000-4000-8000-000000000010"
	const declarationID = "85000000-0000-4000-8000-000000000011"
	const secondDeclarationID = "85000000-0000-4000-8000-000000000012"
	const concurrentDeclarationID1 = "85000000-0000-4000-8000-000000000013"
	const concurrentDeclarationID2 = "85000000-0000-4000-8000-000000000014"
	const serviceDeclarationID = "85000000-0000-4000-8000-000000000016"
	const serviceCorrelationID = "85000000-0000-4000-8000-000000000017"
	const failedAuditDeclarationID = "85000000-0000-4000-8000-000000000018"
	const failedAuditCorrelationID = "85000000-0000-4000-8000-000000000019"
	const httpDeclarationID = "85000000-0000-4000-8000-000000000020"
	defer func() {
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.evento_auditoria_local WHERE actor_id=$1 AND recurso_tipo='identity_declaration' AND accion='kyc.identity_declaration.save'`, accountID); err != nil {
			t.Errorf("remove synthetic KYC audit fixture: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_rut_claim WHERE account_id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic KYC RUT claim fixture: %v", err)
		}
		var remainingAudit int
		if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1 AND recurso_tipo='identity_declaration' AND accion='kyc.identity_declaration.save'`, accountID).Scan(&remainingAudit); err != nil || remainingAudit != 0 {
			t.Errorf("synthetic KYC audit cleanup left %d rows: %v", remainingAudit, err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_provider_event WHERE attempt_id IN (SELECT id FROM public.identity_provider_attempt WHERE account_id=$1)`, accountID); err != nil {
			t.Errorf("remove synthetic KYC provider event fixture: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_provider_attempt WHERE account_id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic KYC provider attempt fixture: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_declaration_revision WHERE account_id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic KYC declaration fixture: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.usuario WHERE id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic KYC account fixture: %v", err)
		}
		var remaining int
		if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1`, accountID).Scan(&remaining); err != nil || remaining != 0 {
			t.Errorf("synthetic KYC declaration cleanup left %d rows: %v", remaining, err)
		}
	}()
	if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES ($1,'repo-fixture@example.invalid','repo-fixture@example.invalid','synthetic-only','activo')`, accountID); err != nil {
		t.Fatal(err)
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MaxConns = 2
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := identitydeclarationpg.New(pool)
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": bytes.Repeat([]byte{0x85}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := verification.EncodeDeclarationPayload(verification.DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"})
	if err != nil {
		t.Fatal(err)
	}
	input := func(id string) identitydeclarationpg.RevisionInput {
		return identitydeclarationpg.RevisionInput{ID: id, AccountID: accountID, CorrelationID: "request:" + id, SchemaVersion: "1", ConsentVersion: "test-v1", ConsentGivenAt: time.Now().UTC(), RetentionAt: time.Now().UTC().Add(24 * time.Hour)}
	}
	seal := func(id string) func(int64) (string, []byte, error) {
		t.Helper()
		return func(revision int64) (string, []byte, error) {
			aad, err := verification.BuildDeclarationAAD(accountID, id, revision, "1")
			if err != nil {
				return "", nil, err
			}
			return keyring.Seal(aad, payload)
		}
	}
	first, err := repository.CreateRevision(ctx, input(declarationID), seal(declarationID))
	if err != nil || first.Revision != 1 {
		t.Fatalf("create first declaration revision: revision=%d err=%v", first.Revision, err)
	}
	readback, err := repository.GetCurrent(ctx, accountID)
	if err != nil || readback.ID != declarationID || readback.Revision != 1 || !bytes.Equal(readback.Ciphertext, first.Ciphertext) {
		t.Fatalf("independent declaration readback mismatch: revision=%#v err=%v", readback, err)
	}
	opened, err := keyring.Open(readback.KeyVersion, mustDeclarationAAD(t, accountID, declarationID, readback.Revision, readback.SchemaVersion), readback.Ciphertext)
	if err != nil || !bytes.Equal(opened, payload) {
		t.Fatal("persisted declaration did not decrypt with the bound synthetic AAD")
	}
	second, err := repository.CreateRevision(ctx, input(secondDeclarationID), seal(secondDeclarationID))
	if err != nil || second.Revision != 2 {
		t.Fatalf("create second declaration revision: revision=%d err=%v", second.Revision, err)
	}
	type createResult struct {
		revision int64
		err      error
	}
	start := make(chan struct{})
	results := make(chan createResult, 2)
	var writers sync.WaitGroup
	for _, id := range []string{concurrentDeclarationID1, concurrentDeclarationID2} {
		id := id
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			created, err := repository.CreateRevision(ctx, input(id), seal(id))
			results <- createResult{revision: created.Revision, err: err}
		}()
	}
	close(start)
	writers.Wait()
	close(results)
	seenRevisions := make(map[int64]bool, 2)
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent declaration save failed: %v", result.err)
		}
		seenRevisions[result.revision] = true
	}
	if !seenRevisions[3] || !seenRevisions[4] {
		t.Fatalf("concurrent saves did not receive unique sequential revisions: %#v", seenRevisions)
	}
	current, err := repository.GetCurrent(ctx, accountID)
	if err != nil || current.Revision != 4 {
		t.Fatalf("current declaration revision=%d err=%v; want revision 4", current.Revision, err)
	}
	declaration := verification.DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	service, err := verification.NewDeclarationService(repository, keyring, fixedDeclarationID(serviceDeclarationID), time.Now, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "test-v1")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.Save(ctx, accountID, declaration, true, "test-v1", serviceCorrelationID)
	if err != nil || saved.ID != serviceDeclarationID || saved.Revision != 5 || saved.Payload != declaration {
		t.Fatalf("declaration service save mismatch: snapshot=%#v err=%v", saved, err)
	}
	serviceReadback, err := service.GetCurrent(ctx, accountID)
	if err != nil || serviceReadback.Revision != 5 || serviceReadback.Payload != declaration {
		t.Fatalf("declaration service current readback mismatch: snapshot=%#v err=%v", serviceReadback, err)
	}
	var auditActor, auditResource, auditAction, auditResult, auditReason, auditCorrelation string
	var auditDetailSafe, auditRetentionValid bool
	if err := admin.QueryRow(ctx, `SELECT actor_id::text,recurso_tipo,accion,resultado,motivo_codigo,correlacion_id,(detalle_codigos='{"obligations_detected":[],"pending_checks":[]}'::jsonb),(retirar_en=ocurrido_en+interval '5 years') FROM public.evento_auditoria_local WHERE recurso_id=$1`, serviceDeclarationID).Scan(&auditActor, &auditResource, &auditAction, &auditResult, &auditReason, &auditCorrelation, &auditDetailSafe, &auditRetentionValid); err != nil {
		t.Fatal(err)
	}
	if auditActor != accountID || auditResource != "identity_declaration" || auditAction != "kyc.identity_declaration.save" || auditResult != "exito" || auditReason != "consent_accepted" || auditCorrelation != serviceCorrelationID || !auditDetailSafe || !auditRetentionValid {
		t.Fatalf("declaration audit mismatch: actor=%s resource=%s action=%s result=%s reason=%s correlation=%s detail_safe=%t retention=%t", auditActor, auditResource, auditAction, auditResult, auditReason, auditCorrelation, auditDetailSafe, auditRetentionValid)
	}
	restartPoolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	restartPoolConfig.MaxConns = 1
	restartPoolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	restartPool, err := pgxpool.NewWithConfig(ctx, restartPoolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer restartPool.Close()
	restartedService, err := verification.NewDeclarationService(identitydeclarationpg.New(restartPool), keyring, fixedDeclarationID(serviceDeclarationID), time.Now, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "test-v1")
	if err != nil {
		t.Fatal(err)
	}
	restartedReadback, err := restartedService.GetCurrent(ctx, accountID)
	if err != nil || restartedReadback.ID != serviceDeclarationID || restartedReadback.Revision != 5 || restartedReadback.Payload != declaration {
		t.Fatalf("new service/pool readback mismatch: snapshot=%#v err=%v", restartedReadback, err)
	}
	httpService, err := verification.NewDeclarationService(repository, keyring, fixedDeclarationID(httpDeclarationID), time.Now, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "test-v1")
	if err != nil {
		t.Fatal(err)
	}
	handler := verificationhttp.NewHandlerWithDeclaration(v85DeclarationAuthenticator{accountID: accountID}, nil, nil, nil, httpService)
	minorPost := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/identity-declaration", strings.NewReader(`{"first_name":"Minor","last_name":"HTTP Person","date_of_birth":"2010-01-02","rut":"11222333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"test-v1"}`))
	minorPost.Header.Set("Authorization", "Bearer synthetic-token")
	minorPost.Header.Set("Content-Type", "application/json")
	minorPostResponse := httptest.NewRecorder()
	handler.ServeHTTP(minorPostResponse, minorPost)
	if minorPostResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("HTTP minor declaration status=%d body=%s", minorPostResponse.Code, minorPostResponse.Body.String())
	}
	post := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/identity-declaration", strings.NewReader(`{"first_name":"Synthetic","last_name":"HTTP Person","date_of_birth":"1990-01-02","rut":"11222333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"test-v1"}`))
	post.Header.Set("Authorization", "Bearer synthetic-token")
	post.Header.Set("Content-Type", "application/json")
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusCreated || postResponse.Header().Get("X-Request-ID") == "" || postResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("HTTP declaration save status=%d request_id=%q cache=%q body=%s", postResponse.Code, postResponse.Header().Get("X-Request-ID"), postResponse.Header().Get("Cache-Control"), postResponse.Body.String())
	}
	var httpSaved verificationhttp.IdentityDeclarationResponse
	if err := json.NewDecoder(postResponse.Body).Decode(&httpSaved); err != nil || httpSaved.ID != httpDeclarationID || httpSaved.Revision != 6 {
		t.Fatalf("HTTP declaration save response=%#v err=%v", httpSaved, err)
	}
	get := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/identity-declaration", nil)
	get.Header.Set("Authorization", "Bearer synthetic-token")
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	var httpRead verificationhttp.IdentityDeclarationResponse
	if getResponse.Code != http.StatusOK || json.NewDecoder(getResponse.Body).Decode(&httpRead) != nil || httpRead.ID != httpDeclarationID || httpRead.Revision != 6 || httpRead.LastName != "HTTP Person" {
		t.Fatalf("HTTP declaration read status=%d response=%#v body=%s", getResponse.Code, httpRead, getResponse.Body.String())
	}
	var httpAuditCorrelation string
	if err := admin.QueryRow(ctx, `SELECT correlacion_id FROM public.evento_auditoria_local WHERE recurso_id=$1 AND accion='kyc.identity_declaration.save'`, httpDeclarationID).Scan(&httpAuditCorrelation); err != nil || httpAuditCorrelation != postResponse.Header().Get("X-Request-ID") {
		t.Fatalf("HTTP audit correlation=%q request_id=%q err=%v", httpAuditCorrelation, postResponse.Header().Get("X-Request-ID"), err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), `DROP TRIGGER IF EXISTS fail_synthetic_identity_declaration_audit ON public.evento_auditoria_local`); err != nil {
			t.Errorf("remove synthetic KYC audit failure trigger: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DROP FUNCTION IF EXISTS public.fail_synthetic_identity_declaration_audit()`); err != nil {
			t.Errorf("remove synthetic KYC audit failure function: %v", err)
		}
	}()
	if _, err := admin.Exec(ctx, `CREATE OR REPLACE FUNCTION public.fail_synthetic_identity_declaration_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.recurso_id='85000000-0000-4000-8000-000000000018'::uuid AND NEW.accion='kyc.identity_declaration.save' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE TRIGGER fail_synthetic_identity_declaration_audit BEFORE INSERT ON public.evento_auditoria_local FOR EACH ROW EXECUTE FUNCTION public.fail_synthetic_identity_declaration_audit()`); err != nil {
		t.Fatal(err)
	}
	failureService, err := verification.NewDeclarationService(repository, keyring, fixedDeclarationID(failedAuditDeclarationID), time.Now, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "test-v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failureService.Save(ctx, accountID, declaration, true, "test-v1", failedAuditCorrelationID); err == nil {
		t.Fatal("declaration save succeeded despite audit insert failure")
	}
	var failedDeclarationRows, failedAuditRows int
	if err := admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE id=$1),(SELECT count(*) FROM public.evento_auditoria_local WHERE recurso_id=$1 AND accion='kyc.identity_declaration.save')`, failedAuditDeclarationID).Scan(&failedDeclarationRows, &failedAuditRows); err != nil {
		t.Fatal(err)
	}
	if failedDeclarationRows != 0 || failedAuditRows != 0 {
		t.Fatalf("audit failure left partial state: declaration rows=%d audit rows=%d", failedDeclarationRows, failedAuditRows)
	}
	var serviceCiphertext []byte
	if err := admin.QueryRow(ctx, `SELECT ciphertext FROM public.identity_declaration_revision WHERE id=$1`, serviceDeclarationID).Scan(&serviceCiphertext); err != nil {
		t.Fatal(err)
	}
	for _, plaintext := range [][]byte{[]byte("Synthetic"), []byte("Person"), []byte("11222333-9")} {
		if bytes.Contains(serviceCiphertext, plaintext) {
			t.Fatal("service ciphertext unexpectedly contained plaintext")
		}
	}
	if _, err := repository.GetCurrent(ctx, "85000000-0000-4000-8000-000000000099"); !errors.Is(err, verification.ErrNotFound) {
		t.Fatalf("other account read error=%v; want not found", err)
	}
	sealErr := errors.New("synthetic sealer failure")
	_, err = repository.CreateRevision(ctx, input("85000000-0000-4000-8000-000000000015"), func(int64) (string, []byte, error) {
		return "", nil, sealErr
	})
	if !errors.Is(err, sealErr) {
		t.Fatalf("sealer error=%v; want propagated synthetic failure", err)
	}
	var stored []byte
	if err := admin.QueryRow(ctx, `SELECT ciphertext FROM public.identity_declaration_revision WHERE id=$1`, declarationID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, payload) {
		t.Fatal("ciphertext unexpectedly contained plaintext")
	}
	var rowCount int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1`, accountID).Scan(&rowCount); err != nil || rowCount != 6 {
		t.Fatalf("synthetic declaration row count=%d err=%v", rowCount, err)
	}

	syntheticProvider := &v85SyntheticDiditClient{sessionID: "85000000-0000-4000-8000-000000000090"}
	diditSvc, err := verification.NewDiditService(
		repository,
		httpService,
		syntheticProvider,
		keyring,
		&v85SeqIDGen{ids: []string{
			"85000000-0000-4000-8000-000000000091",
			"85000000-0000-4000-8000-000000000092",
			"85000000-0000-4000-8000-000000000093",
			"85000000-0000-4000-8000-000000000094",
		}},
		time.Now,
		verification.DiditServiceConfig{
			Environment:               "SANDBOX",
			ApplicationID:             "synthetic-app",
			WorkflowID:                "synthetic-workflow",
			WorkflowVersion:           "1",
			ConsentVersion:            "didit-consent-v1",
			DeclarationConsentVersion: "test-v1",
			AttemptTTL:                30 * time.Minute,
			Retention:                 func(at time.Time) time.Time { return at.Add(24 * time.Hour) },
			RutHMACSecret:             bytes.Repeat([]byte{0x42}, 32),
			RutHMACKeyVersion:         "v1",
		},
		identitydeclarationpg.NewRutClaimStore(pool),
	)
	if err != nil {
		t.Fatalf("NewDiditService failed: %v", err)
	}
	startReq := verification.StartDiditIntentInput{
		FirstName:      "Synthetic",
		LastName:       "HTTP Person",
		DateOfBirth:    "1990-01-02",
		RUT:            "11222333-9",
		IDCountry:      "CHL",
		DocumentType:   "ID",
		ConsentGiven:   true,
		ConsentVersion: "didit-consent-v1",
	}
	started1, err := diditSvc.StartOrResume(ctx, accountID, "idem-v85-1", startReq)
	if err != nil || started1.Status != "PENDING_USER_INPUT" || syntheticProvider.createN != 1 {
		t.Fatalf("StartOrResume 1 mismatch: started=%#v createN=%d err=%v", started1, syntheticProvider.createN, err)
	}
	started2, err := diditSvc.StartOrResume(ctx, accountID, "idem-v85-1", startReq)
	if err != nil || started2.ID != started1.ID || syntheticProvider.createN != 1 {
		t.Fatalf("StartOrResume idempotent replay mismatch: started2=%#v createN=%d err=%v", started2, syntheticProvider.createN, err)
	}
	var linkedDeclarationID string
	if err := admin.QueryRow(ctx, `SELECT declaration_id::text FROM public.identity_provider_attempt WHERE id=$1`, started1.ID).Scan(&linkedDeclarationID); err != nil {
		t.Fatalf("read synthetic attempt declaration binding before approval: %v", err)
	}
	bodyHash := bytes.Repeat([]byte{0xab}, 32)
	if _, err := diditSvc.ProcessWebhook(ctx, verification.DiditWebhookInput{
		EventID:           "evt-v85-1",
		EventType:         "status.updated",
		ProviderSessionID: syntheticProvider.sessionID,
		TopLevelStatus:    "Approved",
		BodySHA256:        bodyHash,
	}); err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}
	survivingDeclaration, err := repository.GetCurrent(ctx, accountID)
	if err != nil || survivingDeclaration.ID == linkedDeclarationID {
		t.Fatalf("approved attempt must not restore its purged declaration as current: current=%q linked=%q err=%v", survivingDeclaration.ID, linkedDeclarationID, err)
	}
	statusAfter, err := diditSvc.GetStatus(ctx, accountID, started1.ID)
	if err != nil || statusAfter.Status != "APPROVED" || statusAfter.BadgeResult != "NOT_VERIFIED" || syntheticProvider.deleteN != 1 {
		t.Fatalf("GetStatus after webhook mismatch: status=%#v deleteN=%d err=%v", statusAfter, syntheticProvider.deleteN, err)
	}
	var decisionPurged bool
	if err := admin.QueryRow(ctx, `SELECT encrypted_decision IS NULL FROM public.identity_provider_attempt WHERE id=$1`, started1.ID).Scan(&decisionPurged); err != nil || !decisionPurged {
		t.Fatalf("approved attempt must purge its encrypted decision payload: purged=%t err=%v", decisionPurged, err)
	}
}

func mustDeclarationAAD(t *testing.T, accountID, declarationID string, revision int64, schemaVersion string) []byte {
	t.Helper()
	aad, err := verification.BuildDeclarationAAD(accountID, declarationID, revision, schemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	return aad
}
