package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	identitydeclarationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/zohoauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/platform/health"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	nativeKYCHTTPHarnessEnv      = "NATIVE_KYC_HTTP_HARNESS"
	nativeKYCHTTPReadyFileEnv    = "NATIVE_KYC_HTTP_READY_FILE"
	nativeKYCHTTPStopFileEnv     = "NATIVE_KYC_HTTP_STOP_FILE"
	nativeKYCHTTPHarnessDeadline = 15 * time.Minute
)

// TestNativeKYCAPIWithBFF exposes the real local KYC mux to the isolated Next
// BFF while journaling the first declaration write before PostgreSQL mutation.
// It is opt-in and exits only after the private stop marker is created.
func TestNativeKYCAPIWithBFF(t *testing.T) {
	if os.Getenv(nativeKYCHTTPHarnessEnv) != "1" {
		t.Skip("explicitly opt in to the journaled native KYC HTTP harness")
	}
	readyPath, stopPath := os.Getenv(nativeKYCHTTPReadyFileEnv), os.Getenv(nativeKYCHTTPStopFileEnv)
	if !filepath.IsAbs(readyPath) || !filepath.IsAbs(stopPath) || readyPath == stopPath || filepath.Dir(readyPath) != filepath.Dir(stopPath) {
		t.Fatal("native KYC HTTP harness control paths are invalid")
	}
	controlDir, err := os.Lstat(filepath.Dir(readyPath))
	if err != nil {
		t.Fatal("native KYC HTTP harness control directory is unavailable")
	}
	controlOwner, ownerOK := controlDir.Sys().(*syscall.Stat_t)
	if !controlDir.IsDir() || controlDir.Mode()&os.ModeSymlink != 0 || controlDir.Mode().Perm() != 0700 || !ownerOK || controlOwner.Uid != uint32(os.Getuid()) {
		t.Fatal("native KYC HTTP harness control directory must be operator-owned mode 0700")
	}
	for _, path := range []string{readyPath, stopPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("native KYC HTTP harness control path already exists; preserve it")
		}
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("native KYC repository root could not be resolved")
	}
	privateDir := filepath.Join(root, ".local", "testing", "kyc-native")
	policyPath := filepath.Join(root, ".local", "testing", "test-account-policy.json")
	fixturePaths := []string{filepath.Join(privateDir, "fixture.json"), filepath.Join(privateDir, "fixture-2.json")}
	if err := validateNativeIdentityPrivateFile(policyPath); err != nil {
		t.Fatal("native KYC private policy metadata is invalid")
	}
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal("native KYC private policy could not be read")
	}
	var fixtures [2]nativeFixtureReceipt
	for i, path := range fixturePaths {
		if err := validateNativeIdentityPrivateFile(path); err != nil {
			t.Fatal("authorized native KYC fixture receipt metadata is invalid")
		}
		data, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(data, &fixtures[i]) != nil || !fixtures[i].Verified || !canonicalNativeUUID(fixtures[i].AccountID) {
			t.Fatal("authorized native KYC fixture receipt is invalid")
		}
	}
	accountID, otherAccountID := fixtures[0].AccountID, fixtures[1].AccountID
	if err := validateNativeIdentityAccountPair(policyBytes, accountID, otherAccountID); err != nil {
		t.Fatal("native KYC fixture pair is not explicitly authorized")
	}

	profile, err := runtimeProfile(os.Getenv)
	if err != nil || profile != runtimeProfileKYCWorkerFree {
		t.Fatal("native KYC HTTP harness requires the dedicated worker-free profile")
	}
	cfg, err := loadRuntimeConfig(os.Getenv, profile)
	if err != nil {
		t.Fatal("native KYC HTTP harness configuration is invalid")
	}
	t.Cleanup(cfg.close)
	keyring, err := loadIdentityDeclarationKeyring(os.Getenv)
	if err != nil || keyring == nil {
		t.Fatal("native KYC encryption keyring is unavailable")
	}
	limit, err := strconv.Atoi(os.Getenv("LOCAL_VERIFICATION_IP_LIMIT"))
	if err != nil || limit < 1 || limit > 1000 {
		t.Fatal("native KYC local authentication quota is invalid")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	source, err := migrator.DiscoverMigrations(filepath.Join(root, "db", "migrations"))
	if err != nil || len(source) < identityDeclarationNativeLedgerVersion {
		t.Fatal("native KYC migration source inventory is incomplete")
	}
	if err := readNativeIdentityPreflight(ctx, source, identityDeclarationNativeLedgerVersion); err != nil {
		t.Fatal("native KYC destination or ledger preflight failed")
	}
	if source[92-1].Version != 92 || source[92-1].Checksum != identityDeclarationNativeV92Checksum {
		t.Fatal("native KYC source V92 checksum differs from the reviewed value")
	}

	cleanupDSN := os.Getenv(identityDeclarationNativeCleanupDSNEnv)
	if err := validateIdentityDeclarationCleanupDSN(cleanupDSN); err != nil {
		t.Fatal("native KYC cleanup target is invalid")
	}
	cleanupConfig, err := parseNativeIdentityConfig(cleanupDSN, "/home/nandev/.local/share/espacigo-native-secrets/pgpass")
	if err != nil {
		t.Fatal("native KYC cleanup connection configuration is invalid")
	}
	cleanupConfig.MaxConns = 1
	cleanupConfig.AfterConnect = func(connectCtx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(connectCtx, `SET ROLE espacigo_schema_owner`); err != nil {
			return err
		}
		var session, current string
		if err := conn.QueryRow(connectCtx, `SELECT session_user,current_user`).Scan(&session, &current); err != nil {
			return err
		}
		if session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
			return errors.New("native KYC cleanup identity mismatch")
		}
		return nil
	}
	cleanupPool, err := pgxpool.NewWithConfig(ctx, cleanupConfig)
	if err != nil {
		t.Fatal("native KYC cleanup pool could not be initialized")
	}
	t.Cleanup(cleanupPool.Close)
	if err := cleanupPool.QueryRow(ctx, `SELECT 1`).Scan(new(int)); err != nil {
		t.Fatal("native KYC cleanup principal is unavailable")
	}
	var cleanupAuthorized bool
	if err := cleanupPool.QueryRow(ctx, `SELECT
 has_table_privilege(current_user,'public.evento_auditoria_local','DELETE')
 AND NOT has_schema_privilege(current_user,'public','CREATE')
 AND (SELECT bool_and(has_column_privilege(current_user,'public.evento_auditoria_local',c,'SELECT')) FROM unnest(ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id']) AS audit_cols(c))
 AND (SELECT bool_and(has_column_privilege(current_user,'public.outbox_evento_local',c,'SELECT')) FROM unnest(ARRAY['agregado_tipo','agregado_id','tipo','clave_deduplicacion']) AS outbox_cols(c))
 AND has_table_privilege(current_user,'public.outbox_evento_local','DELETE')`).Scan(&cleanupAuthorized); err != nil || !cleanupAuthorized {
		t.Fatal("native KYC cleanup authority is not prepared; no write was started")
	}

	pool, closeConnector, err := newDatabasePool(ctx, cfg)
	if err != nil {
		t.Fatal("native KYC runtime database pool could not be initialized")
	}
	t.Cleanup(closeConnector)
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("native KYC runtime database is unavailable")
	}
	var databaseName, sessionUser, currentUser, serverAddress string
	var serverPort int
	if err := pool.QueryRow(ctx, `SELECT current_database(),session_user,current_user,inet_server_addr()::text,inet_server_port()`).Scan(&databaseName, &sessionUser, &currentUser, &serverAddress, &serverPort); err != nil {
		t.Fatal("native KYC runtime target readback failed")
	}
	serverIP := nativeKYCRuntimeServerIP(serverAddress)
	if databaseName != identityDeclarationNativeTarget || sessionUser != "espacigo_api_login" || currentUser != "espacigo_runtime" || serverIP == nil || !serverIP.IsLoopback() || serverPort != 5432 {
		t.Fatal("native KYC runtime connected to an unexpected database or role")
	}
	preflightTx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("native KYC account preflight could not start")
	}
	for _, fixtureID := range []string{accountID, otherAccountID} {
		var active bool
		if err := preflightTx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.usuario WHERE id=$1::uuid AND estado='activo')`, fixtureID).Scan(&active); err != nil || !active {
			_ = preflightTx.Rollback(ctx)
			t.Fatal("authorized native KYC account is inactive or unreadable")
		}
	}
	for _, fixtureID := range []string{accountID, otherAccountID} {
		var existingRows int64
		if err := preflightTx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration')+
 (SELECT count(*) FROM public.outbox_evento_local o WHERE o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion' AND EXISTS (SELECT 1 FROM public.identity_provider_attempt a WHERE a.account_id=$1::uuid AND (o.clave_deduplicacion='didit_session_deletion:' || a.provider_session_id)))`, fixtureID).Scan(&existingRows); err != nil || existingRows != 0 {
			_ = preflightTx.Rollback(ctx)
			t.Fatal("authorized native KYC fixture already has declaration artifacts; preserve existing data")
		}
	}
	var baselineDiditOutboxRows int64
	if err := preflightTx.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND tipo='didit.session_remote_deletion'`).Scan(&baselineDiditOutboxRows); err != nil {
		_ = preflightTx.Rollback(ctx)
		t.Fatal("native KYC Didit outbox baseline could not be read")
	}
	if err := preflightTx.Commit(ctx); err != nil {
		t.Fatal("native KYC account preflight could not close")
	}

	pairMode := os.Getenv("NATIVE_KYC_HTTP_PAIR_JOURNALS") == "1"
	returnFailureMode := os.Getenv("NATIVE_KYC_HTTP_RETURN_FAILURES") == "1"
	expiryMode := os.Getenv("NATIVE_KYC_HTTP_EXPIRY") == "1"
	expiryRetryMode := os.Getenv("NATIVE_KYC_HTTP_EXPIRY_RETRY") == "1"
	if expiryRetryMode && !expiryMode {
		t.Fatal("native expiry retry requires the bounded expiry mode")
	}
	revocationMode := os.Getenv("NATIVE_KYC_HTTP_REVOCATION") == "1"
	approvalMode := os.Getenv("NATIVE_KYC_HTTP_APPROVAL")
	if approvalMode != "" && approvalMode != "supersession" && approvalMode != "withdrawal" {
		t.Fatal("native approval mode is invalid")
	}
	if approvalMode != "" && (pairMode || returnFailureMode || expiryMode || revocationMode || os.Getenv("NATIVE_KYC_HTTP_EXPECT_WEBHOOK_EVENT") != "") {
		t.Fatal("native approval mode cannot combine with another transition mode")
	}
	attemptJournalPath := filepath.Join(privateDir, "attempt-test-intent.json")
	if _, err := os.Lstat(attemptJournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native attempt journal already exists or cannot be checked; preserve it")
	}
	var approvalRepo *nativeAttemptJournalRepository
	if revocationMode && (expiryMode || returnFailureMode || pairMode || os.Getenv("NATIVE_KYC_HTTP_EXPECT_WEBHOOK_EVENT") == "1") {
		t.Fatal("native revocation mode cannot be combined with other transition modes")
	}
	if expiryMode && (returnFailureMode || pairMode || os.Getenv("NATIVE_KYC_HTTP_EXPECT_WEBHOOK_EVENT") == "1") {
		t.Fatal("native expiry mode cannot be combined with other transition modes")
	}
	if returnFailureMode && (pairMode || os.Getenv("NATIVE_KYC_HTTP_EXPECT_WEBHOOK_EVENT") == "1") {
		t.Fatal("native return failure mode cannot be combined with paired or webhook expectations")
	}
	journalPaths := []string{filepath.Join(privateDir, "declaration-test-intent.json"), filepath.Join(privateDir, "declaration-test-intent-2.json")}
	for _, path := range journalPaths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("native KYC declaration journal already exists or cannot be checked; preserve it")
		}
	}
	var trackedRepo *nativeDeclarationIntentRepository
	var trackedRepos []*nativeDeclarationIntentRepository
	var declarationRepo verification.DeclarationRevisionRepository
	baseRepo := identitydeclarationpg.New(pool)
	if pairMode {
		pair, err := newNativeDeclarationIntentPairRepository(baseRepo, privateDir, accountID, otherAccountID)
		if err != nil {
			t.Fatal("native KYC paired journal repository could not be initialized")
		}
		trackedRepos = pair.trackedRepositories()
		trackedRepo = trackedRepos[0]
		declarationRepo = pair
	} else if approvalMode == "supersession" {
		supersession, err := newNativeSupersessionRepository(baseRepo, privateDir, accountID)
		if err != nil {
			t.Fatal("native supersession journal configuration is invalid")
		}
		trackedRepos = supersession.trackedRepositories()
		trackedRepo = trackedRepos[0]
		declarationRepo = supersession
	} else {
		trackedRepo = &nativeDeclarationIntentRepository{delegate: baseRepo, journalPath: journalPaths[0]}
		trackedRepos = []*nativeDeclarationIntentRepository{trackedRepo}
		declarationRepo = trackedRepo
	}
	t.Cleanup(func() {
		if approvalMode != "" {
			cleanupNativeApprovalHarness(t, cleanupPool, trackedRepos, approvalRepo, attemptJournalPath, privateDir, baselineDiditOutboxRows)
			return
		}
		for i := len(trackedRepos) - 1; i >= 0; i-- {
			repo := trackedRepos[i]
			if !repo.intentWritten {
				continue
			}
			if err := validateNativeIdentityPrivateFile(repo.journalPath); err != nil {
				t.Errorf("native KYC cleanup journal is missing or invalid; preserve database artifacts")
				return
			}
			data, err := os.ReadFile(repo.journalPath)
			if err != nil {
				t.Errorf("native KYC cleanup journal could not be read")
				return
			}
			var intent nativeDeclarationIntent
			expectedAccount := accountID
			if pairMode && i == 1 {
				expectedAccount = otherAccountID
			}
			if json.Unmarshal(data, &intent) != nil || intent.AccountID != expectedAccount || intent != repo.trustedIntent {
				t.Errorf("native KYC cleanup journal differs from trusted repository input")
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			err = cleanupNativeIdentityDeclarationFixture(cleanupCtx, cleanupPool, intent.AccountID, intent.CorrelationID, intent.DeclarationID)
			cleanupCancel()
			if err != nil {
				t.Errorf("native KYC journal-scoped cleanup failed; preserve journal and artifacts")
				return
			}
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		var currentDiditOutboxRows int64
		if err := cleanupPool.QueryRow(cleanupCtx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND tipo='didit.session_remote_deletion'`).Scan(&currentDiditOutboxRows); err != nil || currentDiditOutboxRows != baselineDiditOutboxRows {
			t.Errorf("native KYC Didit outbox readback differs from pre-test baseline; preserve journals")
			return
		}
		for _, repo := range trackedRepos {
			if repo.intentWritten {
				if err := os.Remove(repo.journalPath); err != nil {
					t.Errorf("native KYC journal could not be retired after verified cleanup")
					return
				}
			}
		}
		dir, err := os.Open(privateDir)
		if err == nil {
			err = dir.Sync()
			_ = dir.Close()
		}
		if err != nil {
			t.Errorf("native KYC journal retirement could not be synced")
		}
	})
	if pairMode {
		t.Cleanup(func() {
			if !trackedRepos[0].intentWritten || !trackedRepos[1].intentWritten {
				t.Errorf("paired journal expectation missing an intent")
				return
			}
			verifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			verifyTx, err := pool.BeginTx(verifyCtx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
			if err != nil {
				t.Errorf("paired conflict read-only verification could not start")
				return
			}
			defer verifyTx.Rollback(verifyCtx)
			var artifacts int64
			err = verifyTx.QueryRow(verifyCtx, `SELECT
 (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration' )`, otherAccountID).Scan(&artifacts)
			if err != nil || artifacts != 0 {
				t.Errorf("paired conflict account has unexpected declaration artifacts")
				return
			}
			trustedA := trackedRepos[0].trustedIntent
			if !canonicalNativeUUID(trustedA.AccountID) || trustedA.AccountID != accountID || !canonicalNativeUUID(trustedA.DeclarationID) || trustedA.CorrelationID == "" {
				t.Errorf("paired trusted declaration intent is invalid")
				return
			}
			var revisionCount int
			var revisionOwner, revisionID string
			err = verifyTx.QueryRow(verifyCtx, `SELECT count(*), COALESCE(min(account_id::text),''), COALESCE(min(id::text),'') FROM public.identity_declaration_revision WHERE id=$1::uuid`, trustedA.DeclarationID).Scan(&revisionCount, &revisionOwner, &revisionID)
			if err != nil || revisionCount != 1 || revisionOwner != trustedA.AccountID || revisionID != trustedA.DeclarationID {
				t.Errorf("paired trusted declaration was not persisted exactly once for its owner")
				return
			}
			// Correlation is persisted in the declaration audit row; require exactly one
			// matching actor/resource/audit correlation without reading private payload.
			var auditCount int
			err = verifyTx.QueryRow(verifyCtx, `SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration' AND recurso_id=$2::uuid AND correlacion_id=$3`, trustedA.AccountID, trustedA.DeclarationID, trustedA.CorrelationID).Scan(&auditCount)
			if err != nil || auditCount != 1 {
				t.Errorf("paired trusted declaration audit correlation did not match exactly once")
				return
			}
			var claimCount int
			var claimOwner, claimDeclaration string
			err = verifyTx.QueryRow(verifyCtx, `SELECT count(*), COALESCE(min(account_id::text),''), COALESCE(min(declaration_id::text),'') FROM public.identity_rut_claim WHERE account_id=$1::uuid AND declaration_id=$2::uuid`, trustedA.AccountID, trustedA.DeclarationID).Scan(&claimCount, &claimOwner, &claimDeclaration)
			if err != nil || claimCount != 1 || claimOwner != trustedA.AccountID || claimDeclaration != trustedA.DeclarationID {
				t.Errorf("paired trusted declaration RUT claim was not bound exactly once")
				return
			}
			if err := verifyTx.Commit(verifyCtx); err != nil {
				t.Errorf("paired conflict read-only verification could not close")
			}
		})
	}

	if os.Getenv("NATIVE_KYC_HTTP_EXPECT_WEBHOOK_EVENT") == "1" {
		t.Cleanup(func() {
			if !trackedRepo.intentWritten {
				t.Errorf("native KYC webhook expectation has no trusted declaration intent")
				return
			}
			verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer verifyCancel()
			verifyTx, err := pool.BeginTx(verifyCtx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
			if err != nil {
				t.Errorf("native KYC webhook read-only verification could not start")
				return
			}
			defer verifyTx.Rollback(verifyCtx)
			intent := trackedRepo.trustedIntent
			var eventCount int
			var processingStatus string
			var bodyHashLength int
			err = verifyTx.QueryRow(verifyCtx, `SELECT count(*), COALESCE(min(e.processing_status),''), COALESCE(min(octet_length(e.body_sha256)),0)
FROM public.identity_provider_event e
JOIN public.identity_provider_attempt a ON a.id=e.attempt_id
WHERE a.account_id=$1::uuid AND a.declaration_id=$2::uuid`, intent.AccountID, intent.DeclarationID).Scan(&eventCount, &processingStatus, &bodyHashLength)
			if err != nil || eventCount != 1 || processingStatus != "PROCESSED" || bodyHashLength != 32 {
				t.Errorf("native KYC webhook event readback did not match the expected processed event")
				return
			}
			var attemptCount int
			var attemptStatus string
			err = verifyTx.QueryRow(verifyCtx, `SELECT count(*), COALESCE(min(status),'') FROM public.identity_provider_attempt WHERE account_id=$1::uuid AND declaration_id=$2::uuid`, intent.AccountID, intent.DeclarationID).Scan(&attemptCount, &attemptStatus)
			if err != nil || attemptCount != 1 || attemptStatus != "IN_REVIEW" {
				t.Errorf("native KYC authoritative attempt readback did not match InReview")
			}
		})
	}

	identityRepository := identitypg.NewIdentityRepository(pool)
	authService, err := identity.NewAuthenticationService(identityRepository, password.Hasher{}, zohoauth.UnavailableMailer{}, devauth.NewIPLimiter(limit), credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal("native KYC authentication service could not be initialized")
	}
	declarationService, err := newRuntimeIdentityDeclarationService(os.Getenv, declarationRepo, time.Now)
	if err != nil || declarationService == nil {
		t.Fatal("native KYC declaration service could not be initialized")
	}
	var providerRepo verification.ProviderAttemptRepository = identitydeclarationpg.New(pool)
	if approvalMode != "" {
		trusted := func() nativeDeclarationIntent {
			var intent nativeDeclarationIntent
			if validateNativeIdentityPrivateFile(journalPaths[0]) != nil {
				return intent
			}
			data, err := os.ReadFile(journalPaths[0])
			if err != nil || json.Unmarshal(data, &intent) != nil || intent.AccountID != accountID {
				return nativeDeclarationIntent{}
			}
			return intent
		}
		approvalRepo, err = newNativeAttemptJournalRepository(providerRepo, attemptJournalPath, accountID, trusted)
		if err != nil {
			t.Fatal("native approval attempt journal configuration is invalid")
		}
		approvalRepo.beforeCommit = func(ctx context.Context, in verification.CommitWebhookDecisionInput, intent nativeAttemptIntent) error {
			return readNativeApprovalRetention(ctx, pool, in, intent)
		}
		approvalRepo.commitError = func(err error) {
			var state interface{ SQLState() string }
			if errors.As(err, &state) {
				t.Logf("native approval commit SQLSTATE=%s", state.SQLState())
			} else {
				t.Log("native approval commit returned a non-SQL error")
			}
		}
		providerRepo = approvalRepo
		t.Cleanup(func() {
			verifyNativeApprovalHarness(t, pool, trackedRepos, approvalRepo, approvalMode, baselineDiditOutboxRows)
		})
	}
	if returnFailureMode || expiryMode || revocationMode {
		// The journal was absent at startup and is exclusively created by this run's
		// wrapper before persistence. Read that immutable receipt without racing on
		// the wrapper's in-memory write fields; cleanup still compares both copies.
		trusted := func() nativeDeclarationIntent {
			var intent nativeDeclarationIntent
			if validateNativeIdentityPrivateFile(journalPaths[0]) != nil {
				return intent
			}
			data, err := os.ReadFile(journalPaths[0])
			if err != nil || json.Unmarshal(data, &intent) != nil || intent.AccountID != accountID {
				return nativeDeclarationIntent{}
			}
			return intent
		}
		var transitionComplete func() bool
		expectedStatus := "FAILED"
		expectedEvents := 0
		if revocationMode {
			// No fault or clock injection: the signed HTTP webhook performs the
			// real pending-attempt revocation; SQL below verifies its durable result.
			expectedStatus = "REJECTED"
			expectedEvents = 2
			transitionComplete = func() bool { return true }
		} else if expiryMode {
			expiryRepo, err := newNativeExpiryRepository(providerRepo, accountID, trusted)
			if err != nil {
				t.Fatal("native expiry repository configuration is invalid")
			}
			providerRepo = expiryRepo
			expectedStatus = "EXPIRED"
			transitionComplete = func() bool { return expiryRepo.Snapshot() == (nativeExpirySnapshot{ClockOverrides: 1, Expired: true}) }
		} else {
			failureRepo, err := newNativeReturnFailureRepository(providerRepo, accountID, trusted)
			if err != nil {
				t.Fatal("native return failure repository configuration is invalid")
			}
			providerRepo = failureRepo
			transitionComplete = func() bool {
				return failureRepo.Snapshot() == (nativeReturnFailureSnapshot{Faults: 1, Transitions: 1, Recovered: true})
			}
		}
		t.Cleanup(func() {
			if !transitionComplete() {
				t.Errorf("native controlled transition did not complete exactly once")
				return
			}
			if !trackedRepo.intentWritten {
				t.Errorf("native return failure verification has no trusted declaration")
				return
			}
			verifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			tx, err := pool.BeginTx(verifyCtx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
			if err != nil {
				t.Errorf("native failed attempt readback could not start")
				return
			}
			defer tx.Rollback(verifyCtx)
			intent := trackedRepo.trustedIntent
			var attempts, failed, revisions, claims, events, outbox int
			err = tx.QueryRow(verifyCtx, `SELECT
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid AND declaration_id=$2::uuid),
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid AND declaration_id=$2::uuid AND status=$3 AND (status<>'EXPIRED' OR updated_at=created_at+interval '30 minutes')),
 (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid AND id=$2::uuid),
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid AND declaration_id=$2::uuid),
 (SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid AND a.declaration_id=$2::uuid),
 (SELECT count(*) FROM public.outbox_evento_local o WHERE EXISTS (SELECT 1 FROM public.identity_provider_attempt a WHERE a.account_id=$1::uuid AND a.declaration_id=$2::uuid AND (o.agregado_id=a.id OR o.clave_deduplicacion='didit_session_deletion:' || a.provider_session_id)))`, intent.AccountID, intent.DeclarationID, expectedStatus).Scan(&attempts, &failed, &revisions, &claims, &events, &outbox)
			expectedAttempts := 1
			if expiryRetryMode {
				expectedAttempts = 2
			}
			if err != nil || attempts != expectedAttempts || failed != 1 || revisions != 1 || claims != 1 || events != expectedEvents || outbox != 0 {
				t.Errorf("native failed attempt artifacts differ from the scoped expectation")
				return
			}
			if expiryRetryMode {
				var newPending int
				if err := tx.QueryRow(verifyCtx, `SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid AND declaration_id=$2::uuid AND status='PENDING_USER_INPUT' AND provider_session_id IS NOT NULL`, intent.AccountID, intent.DeclarationID).Scan(&newPending); err != nil || newPending != 1 {
					t.Error("post-expiry new attempt was not persisted exactly once")
					return
				}
			}
			if revocationMode {
				var processed, ignored, validHashes int
				err = tx.QueryRow(verifyCtx, `SELECT
 count(*) FILTER (WHERE e.processing_status='PROCESSED' AND e.event_type='user.status.updated'),
 count(*) FILTER (WHERE e.processing_status='IGNORED' AND e.event_type='status.updated'),
 count(*) FILTER (WHERE octet_length(e.body_sha256)=32 AND e.processed_at IS NOT NULL)
 FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id
 WHERE a.account_id=$1::uuid AND a.declaration_id=$2::uuid`, intent.AccountID, intent.DeclarationID).Scan(&processed, &ignored, &validHashes)
				if err != nil || processed != 1 || ignored != 1 || validHashes != 2 {
					t.Errorf("native revoked attempt events differ from scoped replay expectations")
					return
				}
			}
			if err := tx.Commit(verifyCtx); err != nil {
				t.Errorf("native failed attempt readback could not close")
			}
		})
	}
	diditService, err := newTestDiditService(os.Getenv, providerRepo, declarationService, time.Now, identitydeclarationpg.NewRutClaimStore(pool), identitydeclarationpg.NewPostgresSessionDeletionQueue(pool))
	if err != nil || diditService == nil {
		t.Fatal("native KYC fake-provider service could not be initialized")
	}
	verificationHandler := verificationhttp.NewHandlerWithDeclaration(authService, nil, cfg.allowedOrigins, nil, declarationService, diditService)
	apiHandler := newLocalKYCMux(
		identityhttp.NewHandler(authService, identityRepository, cfg.allowedOrigins),
		verificationHandler,
		health.NewHandler(pool, cfg.allowedOrigins),
	)
	server := newHTTPServer(cfg.httpAddr, identityhttp.RestrictedAccountMiddleware(authService, apiHandler, cfg.allowedOrigins...))
	listener, err := net.Listen("tcp", cfg.httpAddr)
	if err != nil {
		t.Fatal("native KYC dedicated API listener is unavailable")
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	serverStarted := true
	t.Cleanup(func() {
		if !serverStarted {
			return
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	})
	healthClient := &http.Client{Timeout: time.Second}
	for _, path := range []string{"/health/live", "/health/ready"} {
		healthDeadline := time.Now().Add(5 * time.Second)
		for {
			healthCtx, healthCancel := context.WithTimeout(context.Background(), time.Second)
			healthRequest, requestErr := http.NewRequestWithContext(healthCtx, http.MethodGet, "http://"+cfg.httpAddr+path, nil)
			var healthResponse *http.Response
			if requestErr == nil {
				healthResponse, requestErr = healthClient.Do(healthRequest)
			}
			ready := requestErr == nil && healthResponse != nil && healthResponse.StatusCode == http.StatusOK
			if healthResponse != nil {
				_ = healthResponse.Body.Close()
			}
			healthCancel()
			if ready {
				break
			}
			select {
			case serveErr := <-serveErrors:
				if serveErr != nil {
					t.Fatal("native KYC API server stopped before readiness")
				}
				t.Fatal("native KYC API server exited before readiness")
			default:
			}
			if !time.Now().Before(healthDeadline) {
				t.Fatal("native KYC API health/readiness check did not succeed")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	readyFile, err := os.OpenFile(readyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal("native KYC HTTP readiness marker could not be created")
	}
	if err := readyFile.Sync(); err != nil {
		_ = readyFile.Close()
		t.Fatal("native KYC HTTP readiness marker could not be synced")
	}
	if err := readyFile.Close(); err != nil {
		t.Fatal("native KYC HTTP readiness marker could not be closed")
	}
	if err := validateNativeIdentityPrivateFile(readyPath); err != nil {
		t.Fatal("native KYC HTTP readiness marker metadata is invalid")
	}
	t.Cleanup(func() { _ = os.Remove(readyPath) })

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(nativeKYCHTTPHarnessDeadline)
	defer deadline.Stop()
	for {
		select {
		case serveErr := <-serveErrors:
			if !errors.Is(serveErr, http.ErrServerClosed) {
				t.Fatal("native KYC API server stopped unexpectedly")
			}
			if _, err := os.Lstat(stopPath); err != nil {
				t.Fatal("native KYC API server stopped before the authorized stop marker")
			}
			serverStarted = false
			return
		case <-ticker.C:
			if _, err := os.Lstat(stopPath); err == nil {
				if err := validateNativeIdentityPrivateFile(stopPath); err != nil {
					t.Fatal("native KYC HTTP stop marker metadata is invalid")
				}
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				shutdownErr := server.Shutdown(shutdownCtx)
				shutdownCancel()
				if shutdownErr != nil {
					t.Fatal("native KYC API server could not shut down cleanly")
				}
				serveErr := <-serveErrors
				if !errors.Is(serveErr, http.ErrServerClosed) {
					t.Fatal("native KYC API server shutdown was not confirmed")
				}
				serverStarted = false
				return
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal("native KYC HTTP stop marker could not be checked")
			}
		case <-deadline.C:
			t.Fatal("native KYC HTTP harness exceeded its bounded runtime")
		}
	}
}

func nativeKYCRuntimeServerIP(address string) net.IP {
	if ip, _, err := net.ParseCIDR(address); err == nil {
		return ip
	}
	return net.ParseIP(address)
}

func TestNativeKYCRuntimeServerIP(t *testing.T) {
	for _, address := range []string{"127.0.0.1/32", "::1/128", "127.0.0.1", "::1"} {
		if ip := nativeKYCRuntimeServerIP(address); ip == nil || !ip.IsLoopback() {
			t.Errorf("loopback server address %q rejected", address)
		}
	}
	for _, address := range []string{"", "invalid", "192.0.2.1/32", "2001:db8::1/128"} {
		if ip := nativeKYCRuntimeServerIP(address); ip != nil && ip.IsLoopback() {
			t.Errorf("non-loopback server address %q accepted", address)
		}
	}
}
