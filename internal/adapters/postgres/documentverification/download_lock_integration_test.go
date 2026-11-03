package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func downloadFixtureCleanupAllowed(preflighted, setupCommitAttempted, setupCommitAcknowledged bool) bool {
	return preflighted && setupCommitAttempted && setupCommitAcknowledged
}

func downloadTransferAuditReadbackValid(total, valid int, transferCommitAttempted, transferCommitAcknowledged bool) bool {
	if total < 0 || total > 1 || valid != total || transferCommitAcknowledged && !transferCommitAttempted {
		return false
	}
	if transferCommitAcknowledged {
		return total == 1
	}
	return transferCommitAttempted || total == 0
}

func TestDownloadFixtureCleanupRequiresAcknowledgedSetupCommit(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		preflighted             bool
		setupCommitAttempted    bool
		setupCommitAcknowledged bool
		want                    bool
	}{
		{name: "no preflight"},
		{name: "preflight only", preflighted: true},
		{name: "attempt without acknowledgment", preflighted: true, setupCommitAttempted: true},
		{name: "acknowledgment without attempt", preflighted: true, setupCommitAcknowledged: true},
		{name: "owned committed fixture", preflighted: true, setupCommitAttempted: true, setupCommitAcknowledged: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := downloadFixtureCleanupAllowed(tc.preflighted, tc.setupCommitAttempted, tc.setupCommitAcknowledged); got != tc.want {
				t.Fatalf("downloadFixtureCleanupAllowed(%t, %t, %t) = %t, want %t", tc.preflighted, tc.setupCommitAttempted, tc.setupCommitAcknowledged, got, tc.want)
			}
		})
	}
}

func TestDownloadTransferAuditReadbackDistinguishesCommitOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		total, valid                    int
		attempted, acknowledged, wantOK bool
	}{
		{name: "no attempt no audit", wantOK: true},
		{name: "no attempt unexpected audit", total: 1, valid: 1},
		{name: "ambiguous commit rolled back", attempted: true, wantOK: true},
		{name: "ambiguous commit persisted exact event", total: 1, valid: 1, attempted: true, wantOK: true},
		{name: "acknowledged commit exact event", total: 1, valid: 1, attempted: true, acknowledged: true, wantOK: true},
		{name: "acknowledged commit missing event", attempted: true, acknowledged: true},
		{name: "unexpected event", total: 1, valid: 0, attempted: true},
		{name: "extra events", total: 2, valid: 2, attempted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := downloadTransferAuditReadbackValid(tc.total, tc.valid, tc.attempted, tc.acknowledged); got != tc.wantOK {
				t.Fatalf("downloadTransferAuditReadbackValid(%d, %d, %t, %t) = %t, want %t", tc.total, tc.valid, tc.attempted, tc.acknowledged, got, tc.wantOK)
			}
		})
	}
}

// TestPostgresSpaceDownloadHoldsOwnerLockThroughStorageRead verifies that the
// real repository keeps the current-owner row lock while encrypted bytes are
// read, and that an ownership transfer supersedes download access atomically.
func TestPostgresSpaceDownloadHoldsOwnerLockThroughStorageRead(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	for _, name := range []string{"PGPASSWORD", "PGSERVICE", "PGSERVICEFILE", "PGOPTIONS"} {
		if os.Getenv(name) != "" {
			t.Fatalf("refusing connection override from %s", name)
		}
	}
	adminPassfile := os.Getenv("PGPASSFILE")
	runtimePassfile := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_RUNTIME_PGPASSFILE")
	if err := validateF5TestPassfile(adminPassfile); err != nil {
		t.Fatal("refusing an unprotected administrative passfile")
	}
	if err := validateF5TestPassfile(runtimePassfile); err != nil {
		t.Fatal("refusing an unprotected API-runtime passfile")
	}
	adminPassfileInfo, err := os.Lstat(adminPassfile)
	if err != nil {
		t.Fatal("refusing an unreadable administrative passfile")
	}
	runtimePassfileInfo, err := os.Lstat(runtimePassfile)
	if err != nil || os.SameFile(adminPassfileInfo, runtimePassfileInfo) {
		t.Fatal("administrative and API-runtime passfiles must be distinct files")
	}
	connectionURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if (connectionURL.Scheme != "postgres" && connectionURL.Scheme != "postgresql") || connectionURL.User == nil || connectionURL.User.Username() != "postgres" || connectionURL.Fragment != "" {
		t.Fatal("refusing a noncanonical administrative integration DSN")
	}
	if connectionURL.User != nil {
		if _, hasPassword := connectionURL.User.Password(); hasPassword {
			t.Fatal("refusing a password embedded in the integration DSN")
		}
	}
	for key, values := range connectionURL.Query() {
		if key != "sslmode" || len(values) != 1 || (values[0] != "disable" && values[0] != "prefer") {
			t.Fatal("refusing unapproved integration DSN query parameters")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	expectedServerIP := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_EXPECTED_SERVER_IP")
	if expectedServerIP != "172.28.0.2" {
		t.Fatal("set the exact expected F5 PostgreSQL server address before running this integration test")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("unable to connect to the authorized F5 synthetic target")
	}
	t.Cleanup(pool.Close)
	var database, setupSessionUser, setupRole, serverIP string
	if err := pool.QueryRow(ctx, `SELECT current_database(),session_user,current_user,host(inet_server_addr())`).Scan(&database, &setupSessionUser, &setupRole, &serverIP); err != nil {
		t.Fatal("unable to verify the selected F5 target")
	}
	if database != "espacigo_f5_gate_20261101" || setupSessionUser != "postgres" || setupRole != "postgres" || serverIP != expectedServerIP {
		t.Fatal("refusing unexpected database, setup identity, or forwarded server address")
	}
	requireF5RetentionQueueIdle(t, ctx, pool)

	ownerID, newOwnerID, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	caseID, fileID := uuid.NewString(), uuid.NewString()
	var existingFixtureRows int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)+(SELECT count(*) FROM public.document_verification_case WHERE id=$4 OR subject_id IN($1,$2) OR space_id=$3)+(SELECT count(*) FROM public.document_verification_file WHERE id=$5 OR case_id=$4)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$5)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$4 OR file_id=$5 OR actor_id IN($1,$2) OR subject_id IN($1,$2))`, ownerID, newOwnerID, spaceID, caseID, fileID).Scan(&existingFixtureRows); err != nil {
		t.Fatal("unable to preflight exact synthetic space-download fixture IDs")
	}
	if existingFixtureRows != 0 {
		t.Fatalf("refusing colliding synthetic space-download fixture IDs: %d existing rows", existingFixtureRows)
	}
	var fixtureSetupCommitAttempted, fixtureSetupCommitAcknowledged bool
	var transferCommitAttempted, transferCommitAcknowledged bool
	fixtureIDsPreflighted := true
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if downloadFixtureCleanupAllowed(fixtureIDsPreflighted, fixtureSetupCommitAttempted, fixtureSetupCommitAcknowledged) {
			cleanupTx, err := pool.BeginTx(cleanupCtx, pgx.TxOptions{})
			if err != nil {
				t.Errorf("begin exact space-download fixture cleanup: %v", err)
			} else {
				cleanupStatements := []struct {
					sql  string
					args []any
				}{
					{`DELETE FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, []any{fileID}},
					{`DELETE FROM public.document_verification_file WHERE id=$1 AND case_id=$2`, []any{fileID, caseID}},
					{`DELETE FROM public.document_verification_case WHERE id=$1 AND purpose='space' AND subject_id=$2 AND space_id=$3`, []any{caseID, ownerID, spaceID}},
					{`DELETE FROM public.espacio_caracteristicas WHERE espacio_id=$1 AND categoria_codigo='sala_multiproposito'`, []any{spaceID}},
					{`DELETE FROM public.tarifa_espacio WHERE espacio_id=$1 AND version=1 AND modalidad='hora'`, []any{spaceID}},
					{`DELETE FROM public.espacio WHERE id=$1 AND propietario_id IN($2,$3)`, []any{spaceID, ownerID, newOwnerID}},
					{`DELETE FROM public.usuario WHERE (id=$1 AND correo_original=$3 AND correo_normalizado=$3) OR (id=$2 AND correo_original=$4 AND correo_normalizado=$4)`, []any{ownerID, newOwnerID, ownerID + "@example.invalid", newOwnerID + "@example.invalid"}},
				}
				cleanupFailed := false
				for _, statement := range cleanupStatements {
					if _, err := cleanupTx.Exec(cleanupCtx, statement.sql, statement.args...); err != nil {
						t.Errorf("remove exact synthetic space-download fixture: %v", err)
						if rollbackErr := cleanupTx.Rollback(cleanupCtx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
							t.Errorf("rollback exact space-download fixture cleanup: %v", rollbackErr)
						}
						cleanupFailed = true
						break
					}
				}
				if !cleanupFailed {
					if err := cleanupTx.Commit(cleanupCtx); err != nil {
						t.Errorf("commit exact space-download fixture cleanup: %v", err)
					}
				}
			}
		}
		var remaining int
		if err := pool.QueryRow(cleanupCtx, `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)+(SELECT count(*) FROM public.document_verification_case WHERE id=$4)+(SELECT count(*) FROM public.document_verification_file WHERE id=$5)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$5)`, ownerID, newOwnerID, spaceID, caseID, fileID).Scan(&remaining); err != nil {
			t.Errorf("verify exact space-download fixture cleanup: %v", err)
		} else if remaining != 0 {
			t.Errorf("space-download fixture cleanup left %d rows", remaining)
		}
		var auditEvents, validAuditEvents int
		if err := pool.QueryRow(cleanupCtx, `SELECT count(*),count(*) FILTER(WHERE case_id=$1 AND purpose='space' AND subject_id=$2 AND file_id IS NULL AND actor_kind='system' AND actor_id IS NULL AND action='case_superseded' AND from_state='borrador' AND to_state='borrador' AND result_code='borrador' AND reason_code='cambio_titularidad' AND revision=2) FROM public.document_verification_event WHERE case_id=$1 OR file_id=$3 OR actor_id IN($2,$4) OR subject_id IN($2,$4)`, caseID, ownerID, fileID, newOwnerID).Scan(&auditEvents, &validAuditEvents); err != nil {
			t.Errorf("verify retained ownership-transfer audit event: %v", err)
		} else if !downloadTransferAuditReadbackValid(auditEvents, validAuditEvents, transferCommitAttempted, transferCommitAcknowledged) {
			t.Errorf("retained ownership-transfer audit mismatch: total=%d matching=%d commit_attempted=%t commit_acknowledged=%t", auditEvents, validAuditEvents, transferCommitAttempted, transferCommitAcknowledged)
		}
	})

	var content bytes.Buffer
	if err := png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal("create synthetic PNG download fixture")
	}
	raw := content.Bytes()
	contentHash := sha256.Sum256(raw)
	setupTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("begin synthetic space-download fixture transaction")
	}
	defer setupTx.Rollback(context.Background())
	for _, actor := range []string{ownerID, newOwnerID} {
		if _, err := setupTx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
			t.Fatal("seed synthetic space-download owner")
		}
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','F5 download fixture',repeat('Synthetic space download description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, ownerID); err != nil {
		t.Fatal("seed synthetic download space")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal("seed synthetic download tariff")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal("seed synthetic download attributes")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseID, ownerID, spaceID); err != nil {
		t.Fatal("seed synthetic current-owner document case")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at) VALUES($1,$2,'limpio',$3,'image/png',0,$4,'synthetic-test',clock_timestamp(),clock_timestamp())`, fileID, caseID, len(raw), contentHash[:]); err != nil {
		t.Fatal("seed synthetic clean document metadata")
	}
	fixtureSetupCommitAttempted = true
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal("commit synthetic space-download fixture")
	}
	fixtureSetupCommitAcknowledged = true

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("create synthetic private-storage key")
	}
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), key)
	if err != nil {
		t.Fatal("create private synthetic document store")
	}
	if err := store.PutSpaceDocument(ctx, ownerID, spaceID, caseID, fileID, raw); err != nil {
		t.Fatal("write encrypted synthetic space document")
	}

	const applicationName = "f5-space-download-read"
	runtimeURL := *connectionURL
	runtimeURL.User = url.User("espacigo_api_login")
	runtimeQuery := runtimeURL.Query()
	runtimeQuery.Set("passfile", runtimePassfile)
	runtimeURL.RawQuery = runtimeQuery.Encode()
	runtimeConfig, err := pgxpool.ParseConfig(runtimeURL.String())
	if err != nil {
		t.Fatal("unable to parse restricted API-runtime connection configuration")
	}
	if runtimeConfig.ConnConfig.Host != "127.0.0.1" || runtimeConfig.ConnConfig.Port != 15434 || runtimeConfig.ConnConfig.Database != "espacigo_f5_gate_20261101" || runtimeConfig.ConnConfig.User != "espacigo_api_login" {
		t.Fatal("refusing an unexpected restricted API-runtime target or identity")
	}
	runtimeConfig.MaxConns = 1
	if runtimeConfig.ConnConfig.RuntimeParams == nil {
		runtimeConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	runtimeConfig.ConnConfig.RuntimeParams["application_name"] = applicationName
	runtimeConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	runtimePool, err := pgxpool.NewWithConfig(ctx, runtimeConfig)
	if err != nil {
		t.Fatal("create restricted runtime pool for private download")
	}
	t.Cleanup(runtimePool.Close)
	var sessionUser, currentUser, runtimeApplicationName string
	var loginCanLogin, loginSuperuser, loginCreateRole, loginCreateDB, loginReplication, loginBypassRLS, loginInherit bool
	var runtimeCanLogin, runtimeSuperuser, runtimeCreateRole, runtimeCreateDB, runtimeReplication, runtimeBypassRLS, runtimeInherit bool
	var downloadPID int32
	if err := runtimePool.QueryRow(ctx, `SELECT session_user,current_user,
		login.rolcanlogin,login.rolsuper,login.rolcreaterole,login.rolcreatedb,login.rolreplication,login.rolbypassrls,login.rolinherit,
		active.rolcanlogin,active.rolsuper,active.rolcreaterole,active.rolcreatedb,active.rolreplication,active.rolbypassrls,active.rolinherit,
		pg_backend_pid(),current_setting('application_name')
		FROM pg_catalog.pg_roles login CROSS JOIN pg_catalog.pg_roles active
		WHERE login.rolname=session_user AND active.rolname=current_user`).Scan(&sessionUser, &currentUser,
		&loginCanLogin, &loginSuperuser, &loginCreateRole, &loginCreateDB, &loginReplication, &loginBypassRLS, &loginInherit,
		&runtimeCanLogin, &runtimeSuperuser, &runtimeCreateRole, &runtimeCreateDB, &runtimeReplication, &runtimeBypassRLS, &runtimeInherit,
		&downloadPID, &runtimeApplicationName); err != nil || sessionUser != "espacigo_api_login" || currentUser != "espacigo_runtime" ||
		!loginCanLogin || loginSuperuser || loginCreateRole || loginCreateDB || loginReplication || loginBypassRLS || loginInherit ||
		runtimeCanLogin || runtimeSuperuser || runtimeCreateRole || runtimeCreateDB || runtimeReplication || runtimeBypassRLS || runtimeInherit ||
		runtimeApplicationName != applicationName {
		t.Fatal("private download is not running under the restricted API-login/runtime roles")
	}

	counter := &f5CountingSpaceStore{inner: store}
	blocker := &f5BlockingSpaceStore{inner: counter, readCompleted: make(chan struct{}), release: make(chan struct{})}
	service, err := dv.NewSpaceDownloadService(New(runtimePool), blocker)
	if err != nil {
		t.Fatal("create space download service")
	}
	type downloadResult struct {
		document dv.DownloadedDocument
		err      error
	}
	downloadDone := make(chan downloadResult, 1)
	var releaseOnce sync.Once
	releaseStorage := func() { releaseOnce.Do(func() { close(blocker.release) }) }
	var downloadFinished bool
	var transferTx pgx.Tx
	var transferDone chan error
	var transferFinished bool
	defer func() {
		releaseStorage()
		if !downloadFinished {
			select {
			case <-downloadDone:
				downloadFinished = true
			case <-time.After(5 * time.Second):
				cancel()
				t.Error("space download goroutine did not finish after releasing storage barrier")
			}
		}
		if transferDone != nil && !transferFinished {
			select {
			case <-transferDone:
				transferFinished = true
			case <-time.After(5 * time.Second):
				cancel()
				t.Error("ownership-transfer goroutine did not finish after releasing storage barrier")
			}
		}
		if transferTx != nil {
			if err := transferTx.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
				t.Errorf("rollback synthetic ownership transfer: %v", err)
			}
		}
	}()
	go func() {
		document, err := service.Download(ctx, ownerID, caseID, fileID)
		downloadDone <- downloadResult{document: document, err: err}
	}()
	select {
	case <-blocker.readCompleted:
	case result := <-downloadDone:
		downloadFinished = true
		t.Fatalf("download did not reach private storage: %v", result.err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for the private storage read")
	}

	transferTx, err = pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal("begin synthetic owner-transfer contender")
	}
	if _, err := transferTx.Exec(ctx, `SET LOCAL application_name='f5-space-owner-transfer'`); err != nil {
		t.Fatal("label synthetic owner-transfer transaction")
	}
	var transferPID int32
	if err := transferTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&transferPID); err != nil {
		t.Fatal("identify synthetic owner-transfer backend")
	}
	transferDone = make(chan error, 1)
	go func() {
		_, err := transferTx.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, newOwnerID)
		transferDone <- err
	}()
	waitForScanFenceBlocker(t, ctx, pool, "f5-space-owner-transfer", transferPID, downloadPID)

	releaseStorage()
	result := <-downloadDone
	downloadFinished = true
	if result.err != nil || !bytes.Equal(result.document.Content, raw) || result.document.MIME != "image/png" {
		t.Fatalf("authorized private download content/MIME mismatch: err=%v mime=%q", result.err, result.document.MIME)
	}
	if counter.reads.Load() != 1 {
		t.Fatalf("private storage read count=%d, want 1", counter.reads.Load())
	}
	if err := <-transferDone; err != nil {
		transferFinished = true
		t.Fatalf("owner transfer failed after download lock release: %v", err)
	}
	transferFinished = true
	transferCommitAttempted = true
	if err := transferTx.Commit(ctx); err != nil {
		t.Fatal("commit synthetic owner transfer")
	}
	transferCommitAcknowledged = true
	transferTx = nil
	var ownerAfterCommit, caseState, retentionReason string
	var superseded, fileClean, retentionDeadlineWithinPolicy bool
	var retentionJobs, retentionJobsAtDeadline, supersessionEvents int
	if err := pool.QueryRow(ctx, `SELECT s.propietario_id::text,c.state,c.superseded_at IS NOT NULL,f.state='limpio',COALESCE(f.retention_reason_code,''),f.retention_at>=clock_timestamp()+interval '29 days' AND f.retention_at<=clock_timestamp()+interval '31 days',(SELECT count(*) FROM public.document_verification_job WHERE file_id=f.id AND kind='retention_delete' AND state='pendiente'),(SELECT count(*) FROM public.document_verification_job WHERE file_id=f.id AND kind='retention_delete' AND state='pendiente' AND available_at=f.retention_at),(SELECT count(*) FROM public.document_verification_event WHERE case_id=c.id AND action='case_superseded' AND reason_code='cambio_titularidad') FROM public.espacio s JOIN public.document_verification_case c ON c.space_id=s.id JOIN public.document_verification_file f ON f.case_id=c.id WHERE s.id=$1 AND c.id=$2 AND f.id=$3`, spaceID, caseID, fileID).Scan(&ownerAfterCommit, &caseState, &superseded, &fileClean, &retentionReason, &retentionDeadlineWithinPolicy, &retentionJobs, &retentionJobsAtDeadline, &supersessionEvents); err != nil {
		t.Fatal("read back committed synthetic ownership transfer")
	}
	if ownerAfterCommit != newOwnerID || caseState != string(dv.CaseDraft) || !superseded || !fileClean || retentionReason != "rejected_or_replaced" || !retentionDeadlineWithinPolicy || retentionJobs != 1 || retentionJobsAtDeadline != 1 || supersessionEvents != 1 {
		t.Fatalf("committed transfer state mismatch: owner=%q case=%q superseded=%t file_clean=%t retention_reason=%q deadline_ok=%t retention_jobs=%d scheduled_at_deadline=%d audit_events=%d", ownerAfterCommit, caseState, superseded, fileClean, retentionReason, retentionDeadlineWithinPolicy, retentionJobs, retentionJobsAtDeadline, supersessionEvents)
	}
	transferredService, err := dv.NewSpaceDownloadService(New(runtimePool), counter)
	if err != nil {
		t.Fatal("create post-transfer space download service")
	}
	for _, actorID := range []string{ownerID, newOwnerID} {
		if _, err := transferredService.Download(ctx, actorID, caseID, fileID); !errors.Is(err, dv.ErrNotFound) {
			t.Fatalf("post-transfer download for actor %q error=%v, want not found", actorID, err)
		}
		if counter.reads.Load() != 1 {
			t.Fatalf("post-transfer request for actor %q reached private storage", actorID)
		}
	}
}

type f5CountingSpaceStore struct {
	inner dv.SpaceDocumentStore
	reads atomic.Int32
}

func (s *f5CountingSpaceStore) ReadSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) ([]byte, error) {
	s.reads.Add(1)
	return s.inner.ReadSpaceDocument(ctx, ownerID, spaceID, caseID, fileID)
}

type f5BlockingSpaceStore struct {
	inner         dv.SpaceDocumentStore
	readCompleted chan struct{}
	release       chan struct{}
	once          sync.Once
}

func (s *f5BlockingSpaceStore) ReadSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) ([]byte, error) {
	content, err := s.inner.ReadSpaceDocument(ctx, ownerID, spaceID, caseID, fileID)
	if err != nil {
		return nil, err
	}
	s.once.Do(func() { close(s.readCompleted) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return content, nil
}

func validateF5TestPassfile(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) == "/dev/null" {
		return errors.New("passfile path must be absolute and private")
	}
	cleanPath := filepath.Clean(path)
	volume := filepath.VolumeName(cleanPath)
	current := volume + string(filepath.Separator)
	relative := strings.TrimPrefix(cleanPath, current)
	components := strings.Split(relative, string(filepath.Separator))
	for i, component := range components {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("passfile path must not contain symlinked components")
		}
		if i < len(components)-1 && !info.IsDir() {
			return errors.New("passfile ancestors must be directories")
		}
		if i == len(components)-2 && info.Mode().Perm() != 0o700 {
			return errors.New("passfile directory must be private")
		}
	}
	info, err := os.Lstat(cleanPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.New("passfile must be a regular mode-0600 file")
	}
	return nil
}

func TestValidateF5TestPassfileRejectsSymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	privateDir := filepath.Join(root, "private")
	if err := os.Mkdir(privateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(privateDir, "pgpass")
	if err := os.WriteFile(passfile, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateF5TestPassfile(passfile); err != nil {
		t.Fatalf("valid private passfile rejected: %v", err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(privateDir, link); err != nil {
		t.Fatal(err)
	}
	if err := validateF5TestPassfile(filepath.Join(link, "pgpass")); err == nil {
		t.Fatal("passfile through symlinked ancestor was accepted")
	}
}
