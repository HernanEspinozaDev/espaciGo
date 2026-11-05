package documentverificationpg

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	clamavadapter "github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	committedScanDatabase = "espacigo_f5_gate_20261101"
	committedScanServer   = "172.28.0.2"
	committedScanClamAV   = "172.25.0.2:3310"
	committedScanEICAR    = `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`
)

func TestCommittedScanEICARHasCanonicalBytes(t *testing.T) {
	const canonical = "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"
	if committedScanEICAR != canonical || len(committedScanEICAR) != 68 {
		t.Fatalf("committed EICAR fixture is not the canonical 68-byte sample: length=%d", len(committedScanEICAR))
	}
}

// TestPostgresCommittedRealScannerOutcomesAreVisibleAfterCommit requires the
// explicitly authorized F5 PostgreSQL and ClamAV endpoints. It commits only
// uniquely identified synthetic fixtures and removes their mutable rows after
// verifying the two immutable scan-result audit events.
func TestPostgresCommittedRealScannerOutcomesAreVisibleAfterCommit(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	address := os.Getenv("ESPACIGO_CLAMAV_TEST_ADDRESS")
	if dsn == "" || address == "" {
		t.Skip("F5 database and local ClamAV opt-ins are required")
	}
	if address != committedScanClamAV {
		t.Fatal("refusing scanner outside the exact authorized F5 endpoint")
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
		t.Fatal("unable to verify administrative passfile identity")
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
		t.Fatal("refusing a non-canonical administrative connection URL")
	}
	if connectionURL.User != nil {
		if _, hasPassword := connectionURL.User.Password(); hasPassword {
			t.Fatal("refusing a password embedded in the integration DSN")
		}
	}
	for name, values := range connectionURL.Query() {
		if name != "sslmode" || len(values) != 1 || values[0] != "disable" {
			t.Fatal("refusing unapproved DSN connection parameters")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adminURL := *connectionURL
	adminQuery := adminURL.Query()
	adminQuery.Set("passfile", adminPassfile)
	adminURL.RawQuery = adminQuery.Encode()
	adminConfig, err := pgxpool.ParseConfig(adminURL.String())
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if adminConfig.ConnConfig.Host != "127.0.0.1" || adminConfig.ConnConfig.Port != 15434 || adminConfig.ConnConfig.Database != committedScanDatabase {
		t.Fatal("refusing database target outside the exact authorized F5 endpoint")
	}
	if adminConfig.ConnConfig.User != "postgres" || len(adminConfig.ConnConfig.Fallbacks) != 0 || adminConfig.ConnConfig.Config.TLSConfig != nil {
		t.Fatal("refusing unexpected fixture setup identity")
	}
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal("unable to connect to the authorized F5 target")
	}
	t.Cleanup(admin.Close)
	var database, sessionUser, setupRole, server string
	if err := admin.QueryRow(ctx, `SELECT current_database(),session_user,current_user,host(inet_server_addr())`).Scan(&database, &sessionUser, &setupRole, &server); err != nil {
		t.Fatal("unable to verify F5 database identity")
	}
	if database != committedScanDatabase || sessionUser != "postgres" || setupRole != "postgres" || server != committedScanServer {
		t.Fatal("refusing unexpected server, database, or fixture setup role")
	}

	runtimeURL := *connectionURL
	runtimeURL.User = url.User("espacigo_api_login")
	runtimeQuery := runtimeURL.Query()
	runtimeQuery.Set("passfile", runtimePassfile)
	runtimeURL.RawQuery = runtimeQuery.Encode()
	workerConfig, err := pgxpool.ParseConfig(runtimeURL.String())
	if err != nil {
		t.Fatal("unable to parse restricted API-runtime connection configuration")
	}
	if workerConfig.ConnConfig.Host != "127.0.0.1" || workerConfig.ConnConfig.Port != 15434 || workerConfig.ConnConfig.Database != committedScanDatabase || workerConfig.ConnConfig.User != "espacigo_api_login" || len(workerConfig.ConnConfig.Fallbacks) != 0 || workerConfig.ConnConfig.Config.TLSConfig != nil {
		t.Fatal("refusing unexpected restricted API-runtime target or identity")
	}
	workerConfig.MaxConns = 1
	if workerConfig.ConnConfig.RuntimeParams == nil {
		workerConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	workerConfig.ConnConfig.RuntimeParams["application_name"] = "f5-committed-scan-worker"
	workerConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`); err != nil {
			return err
		}
		var loginRole, role, applicationName string
		var loginCanLogin, loginSuperuser, loginCreateRole, loginCreateDB, loginReplication, loginBypassRLS, loginInherit bool
		var runtimeCanLogin, runtimeSuperuser, runtimeCreateRole, runtimeCreateDB, runtimeReplication, runtimeBypassRLS, runtimeInherit bool
		if err := conn.QueryRow(ctx, `SELECT session_user,current_user,
			login.rolcanlogin,login.rolsuper,login.rolcreaterole,login.rolcreatedb,login.rolreplication,login.rolbypassrls,login.rolinherit,
			active.rolcanlogin,active.rolsuper,active.rolcreaterole,active.rolcreatedb,active.rolreplication,active.rolbypassrls,active.rolinherit,
			current_setting('application_name')
			FROM pg_catalog.pg_roles login CROSS JOIN pg_catalog.pg_roles active
			WHERE login.rolname=session_user AND active.rolname=current_user`).Scan(&loginRole, &role,
			&loginCanLogin, &loginSuperuser, &loginCreateRole, &loginCreateDB, &loginReplication, &loginBypassRLS, &loginInherit,
			&runtimeCanLogin, &runtimeSuperuser, &runtimeCreateRole, &runtimeCreateDB, &runtimeReplication, &runtimeBypassRLS, &runtimeInherit,
			&applicationName); err != nil {
			return err
		}
		if loginRole != "espacigo_api_login" || role != "espacigo_runtime" ||
			!loginCanLogin || loginSuperuser || loginCreateRole || loginCreateDB || loginReplication || loginBypassRLS || loginInherit ||
			runtimeCanLogin || runtimeSuperuser || runtimeCreateRole || runtimeCreateDB || runtimeReplication || runtimeBypassRLS || runtimeInherit ||
			applicationName != "f5-committed-scan-worker" {
			return fmt.Errorf("runtime connection is not restricted")
		}
		return nil
	}
	worker, err := pgxpool.NewWithConfig(ctx, workerConfig)
	if err != nil {
		t.Fatal("unable to create restricted runtime worker pool")
	}
	t.Cleanup(worker.Close)

	// Reject pre-existing work and any connected runtime-capable session before
	// committing fixtures; do not claim or modify another consumer's job.
	var active, runtimeSessions int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_job WHERE kind='scan' AND state IN ('pendiente','reintentar','procesando')`).Scan(&active); err != nil || active != 0 {
		t.Fatalf("refusing non-idle shared scan queue: active=%d err=%v", active, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_stat_activity a JOIN pg_catalog.pg_roles r ON r.rolname=a.usename WHERE a.datname=current_database() AND a.pid<>pg_backend_pid() AND a.usename IS NOT NULL AND NOT r.rolsuper AND pg_catalog.pg_has_role(r.oid,'espacigo_runtime','MEMBER')`).Scan(&runtimeSessions); err != nil || runtimeSessions != 0 {
		t.Fatalf("refusing shared scan queue with runtime-capable sessions: sessions=%d err=%v", runtimeSessions, err)
	}

	cleanActor, infectedActor := uuid.NewString(), uuid.NewString()
	cleanCase, infectedCase := uuid.NewString(), uuid.NewString()
	cleanFile, infectedFile := uuid.NewString(), uuid.NewString()
	var fixtureSetupCommitAttempted, fixtureSetupCommitAcknowledged, outcomesVerified bool
	cleanContent := []byte("F5 committed scan fixture: clean synthetic bytes")
	infectedContent := []byte(committedScanEICAR)
	cleanHash, infectedHash := sha256.Sum256(cleanContent), sha256.Sum256(infectedContent)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("unable to create synthetic storage key")
	}
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), key)
	if err != nil {
		t.Fatal("unable to create encrypted synthetic document store")
	}
	if err := store.PutIdentityDocument(ctx, cleanActor, cleanCase, cleanFile, cleanContent); err != nil {
		t.Fatal("unable to store encrypted clean fixture")
	}
	if err := store.PutIdentityDocument(ctx, infectedActor, infectedCase, infectedFile, infectedContent); err != nil {
		t.Fatal("unable to store encrypted EICAR fixture")
	}

	setup, err := admin.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin synthetic scan fixture setup")
	}
	if _, err := setup.Exec(ctx, `LOCK TABLE public.document_verification_job IN ACCESS EXCLUSIVE MODE NOWAIT`); err != nil {
		_ = setup.Rollback(context.Background())
		t.Fatalf("refusing concurrent scan queue consumer: %v", err)
	}
	// Exact-ID absence check is read-only. Cleanup is registered only after this
	// succeeds, and before any fixture INSERT can write these identifiers.
	var collisions int
	if err := setup.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM public.usuario WHERE id=ANY($1::uuid[]))+
		(SELECT count(*) FROM public.document_verification_case WHERE id=ANY($2::uuid[]))+
		(SELECT count(*) FROM public.document_verification_file WHERE id=ANY($3::uuid[]))+
		(SELECT count(*) FROM public.document_verification_job WHERE file_id=ANY($3::uuid[]))+
		(SELECT count(*) FROM public.document_verification_event WHERE case_id=ANY($2::uuid[]) OR file_id=ANY($3::uuid[]) OR actor_id=ANY($1::uuid[]) OR subject_id=ANY($1::uuid[]))`,
		[]string{cleanActor, infectedActor}, []string{cleanCase, infectedCase}, []string{cleanFile, infectedFile}).Scan(&collisions); err != nil || collisions != 0 {
		_ = setup.Rollback(context.Background())
		t.Fatalf("refusing colliding committed scan fixture IDs: rows=%d err=%v", collisions, err)
	}
	registerCleanup := func() {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cleanupCancel()
			if fixtureSetupCommitAttempted && fixtureSetupCommitAcknowledged {
				tx, err := admin.Begin(cleanupCtx)
				if err != nil {
					t.Errorf("begin committed scan fixture cleanup: %v", err)
				} else {
					cleanupFailed := false
					for _, item := range []struct {
						label, sql string
						args       []any
					}{
						{"jobs", `DELETE FROM public.document_verification_job WHERE file_id=ANY($1::uuid[]) AND kind IN ('scan','retention_delete')`, []any{[]string{cleanFile, infectedFile}}},
						{"files", `DELETE FROM public.document_verification_file WHERE (id=$1 AND case_id=$3) OR (id=$2 AND case_id=$4)`, []any{cleanFile, infectedFile, cleanCase, infectedCase}},
						{"cases", `DELETE FROM public.document_verification_case WHERE (id=$1 AND purpose='identity' AND subject_id=$3) OR (id=$2 AND purpose='identity' AND subject_id=$4)`, []any{cleanCase, infectedCase, cleanActor, infectedActor}},
						{"actors", `DELETE FROM public.usuario WHERE (id=$1 AND correo_original=$3 AND correo_normalizado=$3) OR (id=$2 AND correo_original=$4 AND correo_normalizado=$4)`, []any{cleanActor, infectedActor, cleanActor + "@example.invalid", infectedActor + "@example.invalid"}},
					} {
						if _, err := tx.Exec(cleanupCtx, item.sql, item.args...); err != nil {
							t.Errorf("delete committed scan fixture %s: %v", item.label, err)
							if rollbackErr := tx.Rollback(cleanupCtx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
								t.Errorf("rollback committed scan fixture cleanup: %v", rollbackErr)
							}
							cleanupFailed = true
							break
						}
					}
					if !cleanupFailed {
						if err := tx.Commit(cleanupCtx); err != nil {
							t.Errorf("commit committed scan fixture cleanup: %v", err)
						}
					}
				}
			}
			var remaining int
			if err := admin.QueryRow(cleanupCtx, `SELECT
			(SELECT count(*) FROM public.usuario WHERE id=ANY($1::uuid[]))+
			(SELECT count(*) FROM public.document_verification_case WHERE id=ANY($2::uuid[]))+
			(SELECT count(*) FROM public.document_verification_file WHERE id=ANY($3::uuid[]))+
			(SELECT count(*) FROM public.document_verification_job WHERE file_id=ANY($3::uuid[]))`, []string{cleanActor, infectedActor}, []string{cleanCase, infectedCase}, []string{cleanFile, infectedFile}).Scan(&remaining); err != nil {
				t.Errorf("verify committed scan fixture cleanup/audit: %v", err)
			} else if remaining != 0 {
				t.Errorf("committed scan cleanup left %d mutable fixture rows", remaining)
			}
			var totalAudits, validAudits, distinctCases, distinctFiles int
			if err := admin.QueryRow(cleanupCtx, `SELECT count(*),
			count(*) FILTER(WHERE action='scan_result' AND actor_kind='scanner' AND actor_id IS NULL AND purpose='identity' AND
				((subject_id=$1 AND case_id=$3 AND file_id=$5 AND result_code='limpio' AND reason_code IS NULL) OR
				 (subject_id=$2 AND case_id=$4 AND file_id=$6 AND result_code='bloqueado' AND reason_code='malware_detectado'))),
			count(DISTINCT case_id) FILTER(WHERE action='scan_result'),
			count(DISTINCT file_id) FILTER(WHERE action='scan_result')
			FROM public.document_verification_event WHERE case_id=ANY($7::uuid[]) OR file_id=ANY($8::uuid[]) OR actor_id IN($1,$2) OR subject_id IN($1,$2)`, cleanActor, infectedActor, cleanCase, infectedCase, cleanFile, infectedFile, []string{cleanCase, infectedCase}, []string{cleanFile, infectedFile}).Scan(&totalAudits, &validAudits, &distinctCases, &distinctFiles); err != nil {
				t.Errorf("verify retained append-only scan audit rows: %v", err)
			} else if totalAudits > 2 || totalAudits != validAudits || distinctCases != totalAudits || distinctFiles != totalAudits || outcomesVerified && totalAudits != 2 {
				t.Errorf("retained scan audit mismatch: total=%d valid=%d distinct_cases=%d distinct_files=%d fully_verified=%t", totalAudits, validAudits, distinctCases, distinctFiles, outcomesVerified)
			}
		})
	}
	registerCleanup()
	var otherJobs int
	if err := setup.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_job WHERE kind='scan' AND state IN ('pendiente','reintentar','procesando')`).Scan(&otherJobs); err != nil || otherJobs != 0 {
		_ = setup.Rollback(context.Background())
		t.Fatalf("refusing scan queue with preexisting eligible work: jobs=%d err=%v", otherJobs, err)
	}
	for _, actor := range []string{cleanActor, infectedActor} {
		if _, err := setup.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
			_ = setup.Rollback(context.Background())
			t.Fatal("seed committed scan fixture actor failed")
		}
	}
	for _, fixture := range []struct {
		actor, caseID, fileID string
		content               []byte
		hash                  [32]byte
	}{{cleanActor, cleanCase, cleanFile, cleanContent, cleanHash}, {infectedActor, infectedCase, infectedFile, infectedContent, infectedHash}} {
		if _, err := setup.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, fixture.caseID, fixture.actor); err != nil {
			_ = setup.Rollback(context.Background())
			t.Fatal("seed committed scan fixture case failed")
		}
		if _, err := setup.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,$3,'image/png',0,$4)`, fixture.fileID, fixture.caseID, len(fixture.content), fixture.hash[:]); err != nil {
			_ = setup.Rollback(context.Background())
			t.Fatal("seed committed scan fixture file failed")
		}
		if _, err := setup.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at) VALUES($1,'scan','pendiente',0,clock_timestamp())`, fixture.fileID); err != nil {
			_ = setup.Rollback(context.Background())
			t.Fatal("seed committed scan fixture job failed")
		}
	}
	claims := make([]ScanClaim, 0, 2)
	for _, fixture := range []struct{ fileID string }{{cleanFile}, {infectedFile}} {
		var claim ScanClaim
		var hash []byte
		err := setup.QueryRow(ctx, `UPDATE public.document_verification_job j
			SET state='procesando',attempts=attempts+1,lease_until=clock_timestamp()+interval '10 minutes',updated_at=clock_timestamp()
			FROM public.document_verification_file f JOIN public.document_verification_case c ON c.id=f.case_id
			WHERE j.file_id=f.id AND j.kind='scan' AND j.state='pendiente' AND f.id=$1
			RETURNING j.id::text,f.id::text,c.id::text,c.subject_id::text,c.purpose,COALESCE(c.space_id::text,''),f.size_bytes,f.mime_type,f.pages,f.content_sha256,j.attempts,c.revision,j.lease_until,f.retention_at`, fixture.fileID).
			Scan(&claim.JobID, &claim.FileID, &claim.CaseID, &claim.SubjectID, &claim.Purpose, &claim.SpaceID, &claim.Size, &claim.MIME, &claim.Pages, &hash, &claim.Attempt, &claim.CaseRevision, &claim.LeaseUntil, &nullablePT{v: &claim.PriorRetentionAt})
		if err != nil {
			_ = setup.Rollback(context.Background())
			t.Fatalf("preclaim fixture scan job: %v", err)
		}
		if len(hash) != 32 {
			_ = setup.Rollback(context.Background())
			t.Fatal("invalid fixture content hash")
		}
		copy(claim.ContentSHA256[:], hash)
		claims = append(claims, claim)
	}
	fixtureSetupCommitAttempted = true
	if err := setup.Commit(ctx); err != nil {
		t.Fatal("commit synthetic scan fixtures failed")
	}
	fixtureSetupCommitAcknowledged = true

	// The worker uses its own runtime pool and repository transactions; outcome
	// commits are therefore independently observable from the admin pool.
	scanner, err := clamavadapter.New(clamavadapter.Config{Address: address, Timeout: 5 * time.Second, MaxBytes: dv.MaxUploadBytes, MaxSignatureAge: 48 * time.Hour})
	if err != nil {
		t.Fatal("configure authorized ClamAV adapter failed")
	}
	jobs := &scanWorkerClaimRecorder{scanProcessorJobs: New(worker)}
	processor := NewScanProcessor(jobs, store, scanner, 2*time.Minute, 30*time.Second, 48*time.Hour)
	if processor == nil {
		t.Fatal("unable to create real scan processor")
	}
	processor.claimPollInterval = 10 * time.Second
	for i, claim := range claims {
		if err := processor.processClaim(ctx, claim); err != nil {
			t.Fatalf("process fixture scan result %d: %v", i+1, err)
		}
	}
	if len(jobs.claims) != 0 {
		t.Fatalf("test processor unexpectedly claimed queue work: %d", len(jobs.claims))
	}

	// Queries use the separate administrative pool/connection after ProcessOne
	// returned, which happens only after each repository transaction commits.
	for _, want := range []struct {
		fileID, caseID, state, code, retentionReason, auditReason string
		retentionRequired                                         bool
	}{
		{cleanFile, cleanCase, string(dv.FileClean), "", "", "", false},
		{infectedFile, infectedCase, string(dv.FileBlocked), "malware_detected", "malicious_file", string(dv.ReasonMalwareDetected), true},
	} {
		var state, code, reason, scannerVersion, jobState string
		var inspected, leaseCleared, signaturesFresh, retentionWithinPolicy bool
		var retentionJobs, auditCount int
		err := admin.QueryRow(ctx, `SELECT f.state,COALESCE(f.last_error_code,''),COALESCE(f.retention_reason_code,''),COALESCE(f.scanner_version,''),f.inspected_at IS NOT NULL,j.state,j.lease_until IS NULL,
			f.signatures_at>=clock_timestamp()-interval '48 hours' AND f.signatures_at<=clock_timestamp()+interval '5 minutes',
			COALESCE(f.retention_at>=clock_timestamp()+interval '23 hours' AND f.retention_at<=clock_timestamp()+interval '25 hours',false),
			(SELECT count(*) FROM public.document_verification_job r WHERE r.file_id=f.id AND r.kind='retention_delete' AND r.state='pendiente' AND r.lease_until IS NULL AND r.available_at=f.retention_at),
			(SELECT count(*) FROM public.document_verification_event e WHERE e.case_id=$2 AND e.file_id=$1 AND e.action='scan_result' AND e.actor_kind='scanner' AND e.actor_id IS NULL AND e.result_code=f.state AND COALESCE(e.reason_code,'')=$3)
			FROM public.document_verification_file f JOIN public.document_verification_job j ON j.file_id=f.id AND j.kind='scan' WHERE f.id=$1`, want.fileID, want.caseID, want.auditReason).Scan(&state, &code, &reason, &scannerVersion, &inspected, &jobState, &leaseCleared, &signaturesFresh, &retentionWithinPolicy, &retentionJobs, &auditCount)
		if err != nil {
			t.Fatalf("independent post-commit readback for fixture: %v", err)
		}
		wantRetentionJobCount := 0
		if want.retentionRequired {
			wantRetentionJobCount = 1
		}
		if state != want.state || code != want.code || reason != want.retentionReason || scannerVersion == "" || !inspected || jobState != "completado" || !leaseCleared || !signaturesFresh || retentionWithinPolicy != want.retentionRequired || retentionJobs != wantRetentionJobCount || auditCount != 1 {
			t.Fatalf("post-commit scan result mismatch: state=%q code=%q reason=%q scanner_present=%t inspected=%t job=%q lease_cleared=%t signatures_fresh=%t retention_ok=%t retention_jobs=%d audits=%d", state, code, reason, scannerVersion != "", inspected, jobState, leaseCleared, signaturesFresh, retentionWithinPolicy, retentionJobs, auditCount)
		}
	}
	outcomesVerified = true
}
