package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"image"
	"image/png"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRetentionAuditReadbackRequiresOnlyExpectedRows(t *testing.T) {
	tests := []struct {
		name            string
		total, valid    int
		setupAttempted  bool
		outcomeVerified bool
		want            bool
	}{
		{name: "no setup and no audit", total: 0, valid: 0, want: true},
		{name: "no setup but unexpected audit", total: 1, valid: 1},
		{name: "ambiguous setup without outcome", total: 0, valid: 0, setupAttempted: true, want: true},
		{name: "ambiguous commit exact event", total: 1, valid: 1, setupAttempted: true, want: true},
		{name: "verified outcome requires event", total: 0, valid: 0, setupAttempted: true, outcomeVerified: true},
		{name: "verified exact outcome", total: 1, valid: 1, setupAttempted: true, outcomeVerified: true, want: true},
		{name: "unexpected action", total: 1, valid: 0},
		{name: "duplicate expected events", total: 2, valid: 2},
		{name: "invalid counts", total: -1, valid: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retentionAuditReadbackValid(tt.total, tt.valid, tt.setupAttempted, tt.outcomeVerified); got != tt.want {
				t.Fatalf("retentionAuditReadbackValid(%d, %d, %t, %t) = %t, want %t", tt.total, tt.valid, tt.setupAttempted, tt.outcomeVerified, got, tt.want)
			}
		})
	}
}

func TestRetentionCaseSnapshotRequiresSeededDraftRevision(t *testing.T) {
	if !retentionCaseSnapshotValid("borrador", 1) {
		t.Fatal("seeded draft case snapshot was rejected")
	}
	for _, tc := range []struct {
		state    string
		revision int64
	}{{"enviada", 1}, {"borrador", 2}, {"", 1}} {
		if retentionCaseSnapshotValid(tc.state, tc.revision) {
			t.Fatalf("unexpected case snapshot accepted: state=%q revision=%d", tc.state, tc.revision)
		}
	}
}

func TestRetentionCleanupRequiresEachMutableFixtureRowAbsent(t *testing.T) {
	if !retentionMutableRowsAbsent(0, 0, 0, 0) {
		t.Fatal("empty fixture residue was rejected")
	}
	for _, tc := range []struct {
		name   string
		actors int
		cases  int
		files  int
		jobs   int
	}{
		{name: "actor", actors: 1},
		{name: "case", cases: 1},
		{name: "file", files: 1},
		{name: "job", jobs: 1},
		{name: "negative count", actors: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if retentionMutableRowsAbsent(tc.actors, tc.cases, tc.files, tc.jobs) {
				t.Fatalf("fixture residue was accepted: actor/case/file/job=%d/%d/%d/%d", tc.actors, tc.cases, tc.files, tc.jobs)
			}
		})
	}
}

func retentionAuditReadbackValid(total, valid int, setupAttempted, outcomeVerified bool) bool {
	return total >= 0 && total <= 1 && valid == total && (setupAttempted || total == 0) && (!outcomeVerified || total == 1)
}

func retentionMutableRowsAbsent(actors, cases, files, jobs int) bool {
	return actors == 0 && cases == 0 && files == 0 && jobs == 0
}

func retentionCaseSnapshotValid(state string, revision int64) bool {
	return state == "borrador" && revision == 1
}

func TestRetentionFixtureCleanupRequiresAcknowledgedSetupCommit(t *testing.T) {
	tests := []struct {
		name               string
		preflighted        bool
		commitAcknowledged bool
		want               bool
	}{
		{name: "no preflight"},
		{name: "preflight alone is not ownership", preflighted: true},
		{name: "acknowledgment without preflight", commitAcknowledged: true},
		{name: "acknowledged owned fixture", preflighted: true, commitAcknowledged: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retentionFixtureCleanupAllowed(tt.preflighted, tt.commitAcknowledged); got != tt.want {
				t.Fatalf("retentionFixtureCleanupAllowed(%t, %t) = %t, want %t", tt.preflighted, tt.commitAcknowledged, got, tt.want)
			}
		})
	}
}

func retentionFixtureCleanupAllowed(preflighted, setupCommitAcknowledged bool) bool {
	return preflighted && setupCommitAcknowledged
}

// TestPostgresCommittedRetentionDeletionIsVisibleAfterCommit uses only unique
// synthetic fixture IDs and independently reads the committed runtime outcome.
func TestPostgresCommittedRetentionDeletionIsVisibleAfterCommit(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	runtimePassfile := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_RUNTIME_PGPASSFILE")
	if dsn == "" || runtimePassfile == "" || os.Getenv("PGPASSFILE") == "" {
		t.Skip("F5 database URL and separate setup/runtime passfile opt-ins are required")
	}
	for _, name := range []string{"PGPASSWORD", "PGSERVICE", "PGSERVICEFILE", "PGOPTIONS"} {
		if os.Getenv(name) != "" {
			t.Fatalf("refusing connection override from %s", name)
		}
	}
	setupPassfile := os.Getenv("PGPASSFILE")
	if err := validateF5TestPassfile(setupPassfile); err != nil {
		t.Fatal("refusing an unprotected setup passfile")
	}
	if err := validateF5TestPassfile(runtimePassfile); err != nil {
		t.Fatal("refusing an unprotected runtime passfile")
	}
	setupInfo, err := os.Lstat(setupPassfile)
	if err != nil {
		t.Fatal("unable to verify setup passfile")
	}
	runtimeInfo, err := os.Lstat(runtimePassfile)
	if err != nil || os.SameFile(setupInfo, runtimeInfo) {
		t.Fatal("setup and runtime passfiles must be distinct files")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.User.Username() != "postgres" || u.Fragment != "" {
		t.Fatal("refusing non-canonical F5 setup DSN")
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		t.Fatal("refusing password embedded in setup DSN")
	}
	for name, values := range u.Query() {
		if name != "sslmode" || len(values) != 1 || values[0] != "disable" {
			t.Fatal("refusing extra or non-canonical DSN parameter")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	setupURL := *u
	q := setupURL.Query()
	q.Set("passfile", setupPassfile)
	setupURL.RawQuery = q.Encode()
	setupConfig, err := pgxpool.ParseConfig(setupURL.String())
	if err != nil {
		t.Fatal("invalid setup DSN")
	}
	if cc := setupConfig.ConnConfig; cc.Host != "127.0.0.1" || cc.Port != 15434 || cc.Database != committedScanDatabase || cc.User != "postgres" || len(cc.Fallbacks) != 0 || cc.Config.TLSConfig != nil {
		t.Fatal("refusing setup target outside exact F5 endpoint")
	}
	setup, err := pgxpool.NewWithConfig(ctx, setupConfig)
	if err != nil {
		t.Fatal("unable to connect to authorized F5 setup target")
	}
	t.Cleanup(setup.Close)
	var database, sessionUser, currentUser string
	var server net.IP
	if err := setup.QueryRow(ctx, `SELECT current_database(),session_user,current_user,host(inet_server_addr())::inet`).Scan(&database, &sessionUser, &currentUser, &server); err != nil || database != committedScanDatabase || sessionUser != "postgres" || currentUser != "postgres" || server == nil || server.String() != committedScanServer {
		t.Fatal("refusing unexpected F5 server/database/setup identity")
	}

	runtimeURL := *u
	runtimeURL.User = url.User("espacigo_api_login")
	q = runtimeURL.Query()
	q.Set("passfile", runtimePassfile)
	runtimeURL.RawQuery = q.Encode()
	runtimeConfig, err := pgxpool.ParseConfig(runtimeURL.String())
	if err != nil {
		t.Fatal("invalid runtime connection configuration")
	}
	if cc := runtimeConfig.ConnConfig; cc.Host != "127.0.0.1" || cc.Port != 15434 || cc.Database != committedScanDatabase || cc.User != "espacigo_api_login" || len(cc.Fallbacks) != 0 || cc.Config.TLSConfig != nil {
		t.Fatal("refusing runtime target outside exact F5 endpoint")
	}
	runtimeConfig.MaxConns = 1
	runtimeConfig.AfterConnect = func(c context.Context, conn *pgx.Conn) error {
		var login, role string
		var loginCanLogin, loginSuper, loginCreateRole, loginCreateDB, loginReplication, loginBypass, loginInherit bool
		var roleCanLogin, roleSuper, roleCreateRole, roleCreateDB, roleReplication, roleBypass, roleInherit bool
		if err := conn.QueryRow(c, `SELECT session_user,current_user,
		(l.rolcanlogin),(l.rolsuper),(l.rolcreaterole),(l.rolcreatedb),(l.rolreplication),(l.rolbypassrls),(l.rolinherit),
		(r.rolcanlogin),(r.rolsuper),(r.rolcreaterole),(r.rolcreatedb),(r.rolreplication),(r.rolbypassrls),(r.rolinherit)
		FROM pg_catalog.pg_roles l CROSS JOIN pg_catalog.pg_roles r WHERE l.rolname=session_user AND r.rolname='espacigo_runtime'`).Scan(&login, &role, &loginCanLogin, &loginSuper, &loginCreateRole, &loginCreateDB, &loginReplication, &loginBypass, &loginInherit, &roleCanLogin, &roleSuper, &roleCreateRole, &roleCreateDB, &roleReplication, &roleBypass, &roleInherit); err != nil {
			return err
		}
		if login != "espacigo_api_login" || role != login || !loginCanLogin || loginSuper || loginCreateRole || loginCreateDB || loginReplication || loginBypass || loginInherit {
			return errors.New("runtime login flags/identity are unexpected before SET ROLE")
		}
		if _, err := conn.Exec(c, `SET ROLE espacigo_runtime`); err != nil {
			return err
		}
		if err := conn.QueryRow(c, `SELECT current_user`).Scan(&role); err != nil {
			return err
		}
		if role != "espacigo_runtime" || roleCanLogin || roleSuper || roleCreateRole || roleCreateDB || roleReplication || roleBypass || roleInherit {
			return errors.New("runtime role flags/identity are not restricted")
		}
		return nil
	}
	runtimePool, err := pgxpool.NewWithConfig(ctx, runtimeConfig)
	if err != nil {
		t.Fatal("unable to create restricted runtime pool")
	}
	t.Cleanup(runtimePool.Close)
	readPool, err := pgxpool.NewWithConfig(ctx, runtimeConfig.Copy())
	if err != nil {
		t.Fatal("unable to create independent runtime readback pool")
	}
	t.Cleanup(readPool.Close)

	actor, caseID, fileID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	var fixtureIDsPreflighted, fixtureSetupCommitAttempted, fixtureSetupCommitAcknowledged, outcomeVerified bool
	t.Cleanup(func() {
		if !fixtureIDsPreflighted {
			return
		}
		c, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if retentionFixtureCleanupAllowed(fixtureIDsPreflighted, fixtureSetupCommitAcknowledged) {
			for _, item := range []struct {
				name, sql string
				args      []any
			}{
				{"job", `DELETE FROM public.document_verification_job WHERE id=$1 AND file_id=$2 AND kind='retention_delete'`, []any{jobID, fileID}},
				{"file", `DELETE FROM public.document_verification_file WHERE id=$1 AND case_id=$2`, []any{fileID, caseID}},
				{"case", `DELETE FROM public.document_verification_case WHERE id=$1 AND purpose='identity' AND subject_id=$2`, []any{caseID, actor}},
				{"actor", `DELETE FROM public.usuario WHERE id=$1 AND correo_original=$2 AND correo_normalizado=$2 AND hash_clave='synthetic' AND estado='activo'`, []any{actor, actor + "@example.invalid"}},
			} {
				if _, err := setup.Exec(c, item.sql, item.args...); err != nil {
					t.Errorf("delete acknowledged exact retention %s fixture: %v", item.name, err)
				}
			}
		}
		var actors, cases, files, jobs int
		cleanupReadErr := setup.QueryRow(c, `SELECT
			(SELECT count(*) FROM public.usuario WHERE id=$1),
			(SELECT count(*) FROM public.document_verification_case WHERE id=$2),
			(SELECT count(*) FROM public.document_verification_file WHERE id=$3),
			(SELECT count(*) FROM public.document_verification_job WHERE id=$4)`, actor, caseID, fileID, jobID).Scan(&actors, &cases, &files, &jobs)
		if cleanupReadErr != nil || !retentionMutableRowsAbsent(actors, cases, files, jobs) {
			t.Errorf("mutable retention fixture rows actor/case/file/job=%d/%d/%d/%d err=%v", actors, cases, files, jobs, cleanupReadErr)
		}
		auditReadVerified := false
		var audits, valid int
		auditErr := readPool.QueryRow(c, `SELECT count(*),count(*) FILTER(WHERE case_id=$2 AND file_id=$3 AND purpose='identity' AND subject_id=$1 AND actor_kind='system' AND actor_id IS NULL AND action='retention_deleted' AND from_state='borrador' AND to_state='borrador' AND result_code='eliminado' AND reason_code='retencion_vencida' AND revision=1) FROM public.document_verification_event WHERE case_id=$2 OR file_id=$3`, actor, caseID, fileID).Scan(&audits, &valid)
		if auditErr == nil {
			auditReadVerified = retentionAuditReadbackValid(audits, valid, fixtureSetupCommitAttempted, outcomeVerified)
		}
		if !auditReadVerified {
			t.Errorf("retention audit cleanup readback: total=%d valid=%d outcome_verified=%t err=%v", audits, valid, outcomeVerified, auditErr)
		}
	})
	storageKey := make([]byte, 32)
	if _, err := rand.Read(storageKey); err != nil {
		t.Fatal("generate synthetic storage key")
	}
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), storageKey)
	if err != nil {
		t.Fatal("create encrypted temporary document store")
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal("encode synthetic PNG document")
	}
	content := raw.Bytes()
	if err := store.PutIdentityDocument(ctx, actor, caseID, fileID, content); err != nil {
		t.Fatal("store encrypted synthetic document")
	}
	stored, err := store.ReadIdentityDocument(ctx, actor, caseID, fileID)
	if err != nil || !bytes.Equal(stored, content) {
		t.Fatal("verify encrypted synthetic document before retention")
	}

	tx, err := setup.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("begin retention fixture setup")
	}
	var collisions int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id=$3)+(SELECT count(*) FROM public.document_verification_job WHERE id=$4 OR file_id=$3)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 OR file_id=$3)`, actor, caseID, fileID, jobID).Scan(&collisions); err != nil || collisions != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("refusing colliding retention fixture IDs: rows=%d err=%v", collisions, err)
	}
	fixtureIDsPreflighted = true
	if _, err := tx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal("seed synthetic actor")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,state,revision) VALUES($1,'identity',$2,'borrador',1)`, caseID, actor); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal("seed identity case")
	}
	hash := sha256.Sum256(content)
	if _, err := tx.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at,retention_at,retention_reason_code) VALUES($1,$2,'limpio',$3,'image/png',0,$4,'synthetic-test',clock_timestamp(),clock_timestamp(),clock_timestamp()-interval '1 second','abandoned_draft')`, fileID, caseID, len(content), hash[:]); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal("seed expired identity file")
	}
	var claimReadyBefore time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO public.document_verification_job(id,file_id,kind,state,attempts,available_at,lease_until) VALUES($1,$2,'retention_delete','pendiente',0,clock_timestamp()+interval '1 hour',NULL) RETURNING available_at`, jobID, fileID).Scan(&claimReadyBefore); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal("seed delayed exact retention job")
	}
	fixtureSetupCommitAttempted = true
	if err := tx.Commit(ctx); err != nil {
		t.Fatal("commit retention fixture")
	}
	fixtureSetupCommitAcknowledged = true

	repo := New(runtimePool)
	// Ordinary workers use clock_timestamp() and cannot claim this delayed row.
	// The test supplies its exact job ID and the stored cutoff to the same
	// production claim/lease path, without inspecting or consuming other jobs.
	claim, err := repo.claimReadyRetentionJob(ctx, 2*time.Minute, jobID, claimReadyBefore)
	if err != nil || claim == nil || claim.JobID != jobID || claim.FileID != fileID || claim.CaseID != caseID || claim.SubjectID != actor || claim.Purpose != "identity" || claim.Attempt != 1 {
		t.Fatalf("claim exact retention fixture: claim=%+v err=%v", claim, err)
	}
	if !claim.LeaseUntil.After(time.Now().Add(time.Minute)) || claim.LeaseUntil.After(time.Now().Add(3*time.Minute)) {
		t.Fatal("fixture lease must remain live beyond the bounded test context")
	}
	if err := repo.ProcessClaimedRetentionJob(ctx, *claim, func(c context.Context) error { return store.DeleteIdentityDocument(c, actor, caseID, fileID) }, 30*time.Second); err != nil {
		t.Fatalf("process claimed retention fixture: %v", err)
	}
	if _, err := store.ReadIdentityDocument(ctx, actor, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("encrypted document still exists after committed deletion: %v", err)
	}
	var state, caseState, casePurpose, fileState, retentionReason, action, purpose, subject, actorKind, fromState, toState, result, reason string
	var actorIDNull bool
	var caseRevision, revision int64
	var attempts int
	var leaseCleared, errorCleared, retentionExpired bool
	var events int
	if err := readPool.QueryRow(ctx, `SELECT j.state,j.attempts,j.lease_until IS NULL,j.last_error_code IS NULL,c.state,c.revision,c.purpose,f.state,f.retention_reason_code,COALESCE(f.retention_at<clock_timestamp(),false),e.action,e.purpose,e.subject_id::text,e.actor_kind,(e.actor_id IS NULL),e.from_state,e.to_state,e.result_code,e.reason_code,e.revision,(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 OR file_id=$3) FROM public.document_verification_job j JOIN public.document_verification_case c ON c.id=$2 AND c.subject_id=$4 JOIN public.document_verification_file f ON f.id=$3 AND f.case_id=c.id JOIN public.document_verification_event e ON e.case_id=$2 AND e.file_id=f.id AND e.action='retention_deleted' WHERE j.id=$1 AND j.file_id=f.id AND j.kind='retention_delete'`, jobID, caseID, fileID, actor).Scan(&state, &attempts, &leaseCleared, &errorCleared, &caseState, &caseRevision, &casePurpose, &fileState, &retentionReason, &retentionExpired, &action, &purpose, &subject, &actorKind, &actorIDNull, &fromState, &toState, &result, &reason, &revision, &events); err != nil {
		t.Fatal("independent post-commit runtime readback failed")
	}
	if state != "completado" || attempts != 1 || !leaseCleared || !errorCleared || !retentionCaseSnapshotValid(caseState, caseRevision) || casePurpose != "identity" || fileState != "limpio" || retentionReason != "abandoned_draft" || !retentionExpired || action != "retention_deleted" || purpose != "identity" || subject != actor || actorKind != "system" || !actorIDNull || fromState != "borrador" || toState != "borrador" || result != "eliminado" || reason != "retencion_vencida" || revision != 1 || events != 1 {
		t.Fatalf("post-commit retention result=%q/%d lease_cleared=%t error_cleared=%t case=%q/%d/%q file=%q/%q retention_expired=%t action=%q purpose=%q subject=%q actor=%q/%t transition=%q/%q result_code=%q reason=%q revision=%d events=%d", state, attempts, leaseCleared, errorCleared, caseState, caseRevision, casePurpose, fileState, retentionReason, retentionExpired, action, purpose, subject, actorKind, actorIDNull, fromState, toState, result, reason, revision, events)
	}
	outcomeVerified = true
}
