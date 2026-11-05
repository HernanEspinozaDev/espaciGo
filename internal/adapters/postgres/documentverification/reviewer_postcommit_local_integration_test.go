package documentverificationpg

import (
	"context"
	"os"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

// A nested pgx transaction is a savepoint. Repository commits therefore stay
// inside the outer transaction and are rolled back when the test finishes.
type reviewerSavepointBeginner struct{ tx pgx.Tx }

func (b reviewerSavepointBeginner) BeginTx(ctx context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return b.tx.Begin(ctx)
}

type unusedReviewerDocumentStore struct{}

func (unusedReviewerDocumentStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}
func (unusedReviewerDocumentStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

// Exercises the real repository/service authorization and transition against
// the explicitly selected two-account synthetic target. The fixture case and
// its grant must already exist; no fixture is created or persisted by this test.
func TestLocalReviewerApprovalRemainsReadableAfterPostcommitLoad(t *testing.T) {
	if os.Getenv("ESPACIGO_LOCAL_REVIEWER_POSTCOMMIT_TEST") != "1" {
		t.Skip("requires the exact authorized synthetic PostgreSQL target")
	}
	actor, caseID := os.Getenv("ESPACIGO_REVIEWER_TEST_ACTOR"), os.Getenv("ESPACIGO_REVIEWER_TEST_CASE")
	if dv.ValidateID(actor) != nil || dv.ValidateID(caseID) != nil {
		t.Fatal("explicit private fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=espacigo_f5_gate_20261101 sslmode=disable")
	if err != nil {
		t.Fatal("selected local SQL connection unavailable")
	}
	defer conn.Close(context.Background())
	var database, user string
	var version, accounts, ledger int
	if err = conn.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int,(SELECT count(*) FROM public.usuario),(SELECT count(*) FROM public.schema_migrations)`).Scan(&database, &user, &version, &accounts, &ledger); err != nil || database != "espacigo_f5_gate_20261101" || user != "postgres" || version != 180006 || accounts != 2 || ledger != 83 {
		t.Fatal("local synthetic identity/ledger preflight mismatch")
	}
	var subject, state string
	var revision uint64
	var expires time.Time
	var fileCount, cleanCount int
	if err = conn.QueryRow(ctx, `SELECT c.subject_id::text,c.state,c.revision,c.document_expires_at,
 (SELECT count(*) FROM public.document_verification_file f WHERE f.case_id=c.id),
 (SELECT count(*) FROM public.document_verification_file f WHERE f.case_id=c.id AND f.state='limpio')
 FROM public.document_verification_case c JOIN public.document_verification_reviewer_grant g
 ON g.reviewer_id=$1 AND g.capability='review_identity' AND g.scope_kind='identity_subject' AND g.scope_id=c.subject_id
 WHERE c.id=$2 AND c.purpose='identity' AND c.state='enviada' AND c.superseded_at IS NULL
 AND c.subject_id<>$1 AND c.document_expires_at>now() AND g.revoked_at IS NULL
 AND (g.expires_at IS NULL OR g.expires_at>now())`, actor, caseID).Scan(&subject, &state, &revision, &expires, &fileCount, &cleanCount); err != nil || fileCount == 0 || cleanCount != fileCount {
		t.Fatal("existing submitted identity case with exact active grant and clean files required")
	}
	if _, err = conn.Exec(ctx, `SET SESSION AUTHORIZATION espacigo_api_login`); err != nil {
		t.Fatal("limited runtime login unavailable")
	}
	if _, err = conn.Exec(ctx, `SET ROLE espacigo_runtime`); err != nil {
		t.Fatal("limited runtime role unavailable")
	}
	var login, role string
	var elevated bool
	if err = conn.QueryRow(ctx, `SELECT session_user,current_user,(SELECT rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls FROM pg_roles WHERE rolname=current_user)`).Scan(&login, &role, &elevated); err != nil || login != "espacigo_api_login" || role != "espacigo_runtime" || elevated {
		t.Fatal("runtime test identity is not limited")
	}
	outer, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal("rollback transaction unavailable")
	}
	defer outer.Rollback(context.Background())
	repo := newWithTransactionBeginner(reviewerSavepointBeginner{tx: outer})
	cases, err := dv.NewService(repo)
	if err != nil {
		t.Fatal("real document verification service initialization failed")
	}
	access, err := dv.NewReviewerAccessService(repo, cases, unusedReviewerDocumentStore{})
	if err != nil {
		t.Fatal("reviewer service initialization failed")
	}
	access.SetClock(func() time.Time { return time.Now().UTC() })
	if err = access.Review(ctx, actor, caseID, dv.CapabilityIdentity, dv.DecisionApprove, "", time.Time{}, revision); err != nil {
		t.Fatalf("real repository approval failed: %v", err)
	}
	loaded, err := access.Load(ctx, actor, caseID)
	if err != nil {
		t.Fatalf("real postcommit metadata load failed after approval: %v", err)
	}
	if loaded.ID != caseID || loaded.SubjectID != subject || loaded.State != dv.CaseApproved || loaded.Revision != revision+1 || loaded.ReviewerID != actor || loaded.SupersededAt != nil {
		t.Fatalf("unexpected post-review snapshot: id=%q subject=%q state=%q revision=%d reviewer=%q superseded=%v", loaded.ID, loaded.SubjectID, loaded.State, loaded.Revision, loaded.ReviewerID, loaded.SupersededAt)
	}
	if err = outer.Rollback(ctx); err != nil {
		t.Fatal("synthetic review rollback failed")
	}
	t.Log("REAL_REPOSITORY_REVIEW_THEN_LOAD_APPROVED_SELF_CASE_NOT_SUPERSEDED_PASS; OUTER_TRANSACTION_ROLLED_BACK")
}
