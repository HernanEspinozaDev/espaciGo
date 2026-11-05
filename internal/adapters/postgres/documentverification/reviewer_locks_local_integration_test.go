package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

// Runs only inside the explicitly selected synthetic target. Account and grant
// IDs come from its private fixture. Reads use a login-equivalent limited SQL
// session; administrative assignment withdrawal is rolled back in full.
func TestLocalReviewerReadSerializesAssignmentWithdrawal(t *testing.T) {
	if os.Getenv("ESPACIGO_LOCAL_REVIEWER_LOCK_TEST") != "1" {
		t.Skip("requires the exact authorized synthetic PostgreSQL target")
	}
	actor, caseID := os.Getenv("ESPACIGO_REVIEWER_TEST_ACTOR"), os.Getenv("ESPACIGO_REVIEWER_TEST_CASE")
	if !validUUID(actor) || !validUUID(caseID) {
		t.Fatal("explicit private fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connect := func() *pgx.Conn {
		c, e := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=espacigo_f5_gate_20261101 sslmode=disable")
		if e != nil {
			t.Fatal("selected local SQL connection unavailable")
		}
		t.Cleanup(func() { c.Close(context.Background()) })
		return c
	}
	admin, observer, runtime := connect(), connect(), connect()
	var database, user string
	var version, accounts, ledger int
	if e := admin.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int,(SELECT count(*) FROM public.usuario),(SELECT count(*) FROM public.schema_migrations)`).Scan(&database, &user, &version, &accounts, &ledger); e != nil || database != "espacigo_f5_gate_20261101" || user != "postgres" || version != 180006 || accounts != 2 || ledger != 83 {
		t.Fatal("local synthetic identity/ledger preflight mismatch")
	}
	var grant, owner string
	var auditBefore int
	if e := admin.QueryRow(ctx, `SELECT g.id::text,c.subject_id::text,(SELECT count(*) FROM public.document_verification_reviewer_grant_event e WHERE e.grant_id=g.id)
 FROM public.document_verification_case c JOIN public.document_verification_reviewer_grant g
 ON g.reviewer_id=$1 AND g.capability='review_identity' AND g.scope_kind='identity_subject' AND g.scope_id=c.subject_id
 WHERE c.id=$2 AND c.purpose='identity' AND c.state='enviada' AND c.superseded_at IS NULL
 AND c.subject_id<>$1 AND g.revoked_at IS NULL AND g.expires_at>now()`, actor, caseID).Scan(&grant, &owner, &auditBefore); e != nil {
		t.Fatal("exact owned active identity review fixture required")
	}
	if _, e := runtime.Exec(ctx, `SET SESSION AUTHORIZATION espacigo_api_login; SET ROLE espacigo_runtime`); e != nil {
		t.Fatal("limited runtime test session unavailable")
	}
	var login, role string
	var elevated bool
	if e := runtime.QueryRow(ctx, `SELECT session_user,current_user,(SELECT rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls FROM pg_roles WHERE rolname=current_user)`).Scan(&login, &role, &elevated); e != nil || login != "espacigo_api_login" || role != "espacigo_runtime" || elevated {
		t.Fatal("runtime test identity is not limited")
	}
	repo := newWithTransactionBeginner(runtime)
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	readDone := make(chan error, 1)
	go func() {
		readDone <- repo.WithReviewerCaseForRead(ctx, actor, caseID, func(_ context.Context, x dv.Snapshot, a dv.Authorization) error {
			if x.ID != caseID || a.ActorID != actor || a.SubjectID != owner || !reviewerGrantMatches(a.ReviewerGrants, actor, x) {
				return errors.New("callback scope mismatch")
			}
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-entered:
	case e := <-readDone:
		t.Fatal("real limited reviewer read did not reach protected callback", e)
	case <-ctx.Done():
		t.Fatal("callback timeout")
	}
	withdraw, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal("withdrawal transaction unavailable")
	}
	defer withdraw.Rollback(context.Background())
	lockDone := make(chan error, 1)
	go func() { lockDone <- lockActor(ctx, withdraw, actor) }()
	blocked := false
	for i := 0; i < 100; i++ {
		if e := observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock' AND $2=ANY(pg_blocking_pids(pid)))`, int32(admin.PgConn().PID()), int32(runtime.PgConn().PID())).Scan(&blocked); e != nil {
			t.Fatal("lock observation unavailable")
		}
		if blocked {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("withdrawal must wait while authorized reader holds the callback")
	}
	close(release)
	if e := <-readDone; e != nil {
		t.Fatal("reader did not finish before withdrawal", e)
	}
	if e := <-lockDone; e != nil {
		t.Fatal("withdrawal could not acquire reviewer lock after reader")
	}
	result, e := withdraw.Exec(ctx, `UPDATE public.document_verification_reviewer_grant SET revoked_at=clock_timestamp(),revoked_by=$2,revocation_reason_code='temporary_coverage_ended' WHERE id=$1 AND reviewer_id=$3 AND revoked_at IS NULL`, grant, owner, actor)
	if e != nil || result.RowsAffected() != 1 {
		t.Fatal("exact synthetic grant withdrawal failed")
	}
	var during bool
	if e = withdraw.QueryRow(ctx, `SELECT revoked_at IS NOT NULL AND EXISTS(SELECT 1 FROM public.document_verification_reviewer_grant_event e WHERE e.grant_id=g.id AND e.action='revoked') FROM public.document_verification_reviewer_grant g WHERE id=$1`, grant).Scan(&during); e != nil || !during {
		t.Fatal("transactional withdrawal/audit not observed")
	}
	if e = withdraw.Rollback(ctx); e != nil {
		t.Fatal("withdrawal rollback failed")
	}
	var preserved bool
	var auditAfter int
	if e = observer.QueryRow(ctx, `SELECT revoked_at IS NULL,(SELECT count(*) FROM public.document_verification_reviewer_grant_event e WHERE e.grant_id=g.id) FROM public.document_verification_reviewer_grant g WHERE id=$1`, grant).Scan(&preserved, &auditAfter); e != nil || !preserved || auditAfter != auditBefore {
		t.Fatal("original grant/audit not preserved after rollback")
	}
	if _, e = repo.LoadReviewerCase(ctx, actor, caseID); e != nil {
		t.Fatal("retained assignment readback failed")
	}
	t.Log("REAL_LIMITED_REVIEWER_CALLBACK_BLOCKS_WITHDRAWAL_THEN_RELEASES_AND_ROLLBACK_PRESERVES_ASSIGNMENT_AUDIT_PASS")
}
