package documentverificationpg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	dvhttp "github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type committedGapAuthenticator struct{ actor string }

func (a committedGapAuthenticator) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: a.actor}, nil
}

// Only the scheduling boundary is controlled: the real service commits first,
// an independent audited administrative withdrawal commits next, and the real
// HTTP handler then invokes the real authorization-backed metadata reader.
type committedGapService struct {
	*dv.ReviewerAccessService
	withdraw func(context.Context) error
	reviews  int
}

func (s *committedGapService) Review(ctx context.Context, actor, id string, capability dv.Capability, decision dv.Decision, reason dv.ReasonCode, until time.Time, revision uint64) error {
	s.reviews++
	if err := s.ReviewerAccessService.Review(ctx, actor, id, capability, decision, reason, until, revision); err != nil {
		return err
	}
	return s.withdraw(ctx)
}

// Explicitly persistent synthetic experiment: consumes one submitted identity
// case and its grant. It never resets state, creates accounts, or modifies ACLs.
func TestLocalReviewerCommittedApprovalThenWithdrawalHidesCase(t *testing.T) {
	if os.Getenv("ESPACIGO_LOCAL_REVIEWER_COMMITTED_GAP_TEST") != "1" {
		t.Skip("requires explicit committed-gap opt-in and prepared native fixture")
	}
	actor, caseID := os.Getenv("ESPACIGO_REVIEWER_TEST_ACTOR"), os.Getenv("ESPACIGO_REVIEWER_TEST_CASE")
	if dv.ValidateID(actor) != nil || dv.ValidateID(caseID) != nil {
		t.Fatal("explicit existing synthetic fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	connect := func() *pgx.Conn {
		c, err := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=espacigo_f5_gate_20261101 sslmode=disable")
		if err != nil {
			t.Fatal("selected local connection unavailable")
		}
		t.Cleanup(func() { c.Close(context.Background()) })
		return c
	}
	admin, runtime, observer := connect(), connect(), connect()
	var db, user string
	var version, accounts, ledger int
	if err := admin.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int,(SELECT count(*) FROM public.usuario),(SELECT count(*) FROM public.schema_migrations)`).Scan(&db, &user, &version, &accounts, &ledger); err != nil || db != "espacigo_f5_gate_20261101" || user != "postgres" || version != 180006 || accounts != 2 || ledger != 83 {
		t.Fatal("synthetic target preflight mismatch")
	}
	var subject, grantID string
	var revision uint64
	if err := admin.QueryRow(ctx, `SELECT c.subject_id::text,c.revision,g.id::text FROM public.document_verification_case c JOIN public.document_verification_reviewer_grant g ON g.reviewer_id=$1 AND g.capability='review_identity' AND g.scope_kind='identity_subject' AND g.scope_id=c.subject_id WHERE c.id=$2 AND c.purpose='identity' AND c.state='enviada' AND c.superseded_at IS NULL AND c.subject_id<>$1 AND c.document_expires_at>now() AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>now()) AND EXISTS(SELECT 1 FROM public.document_verification_file f WHERE f.case_id=c.id AND f.state='limpio')`, actor, caseID).Scan(&subject, &revision, &grantID); err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok {
			t.Fatalf("prepared fixture query SQLSTATE=%s message=%s", pgerr.Code, pgerr.Message)
		}
		t.Fatalf("prepared submitted fixture and exact active grant required: %T", err)
	}
	if _, err := runtime.Exec(ctx, `SET SESSION AUTHORIZATION espacigo_api_login; SET ROLE espacigo_runtime`); err != nil {
		t.Fatal("limited runtime unavailable")
	}
	var login, role string
	var elevated bool
	if err := runtime.QueryRow(ctx, `SELECT session_user,current_user,(SELECT rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls FROM pg_roles WHERE rolname=current_user)`).Scan(&login, &role, &elevated); err != nil || login != "espacigo_api_login" || role != "espacigo_runtime" || elevated {
		t.Fatal("limited runtime identity mismatch")
	}
	repo := newWithTransactionBeginner(runtime)
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, service, unusedReviewerDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}
	wrapped := &committedGapService{ReviewerAccessService: access, withdraw: func(ctx context.Context) error {
		tx, err := admin.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(context.Background())
		if err = lockActor(ctx, tx, actor); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `UPDATE public.document_verification_reviewer_grant SET revoked_at=clock_timestamp(),revoked_by=$2,revocation_reason_code='temporary_coverage_ended' WHERE id=$1 AND revoked_at IS NULL`, grantID, subject)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			t.Fatal("exact grant withdrawal not applied")
		}
		return tx.Commit(ctx)
	}}
	body, _ := json.Marshal(map[string]any{"capability": "review_identity", "decision": "approve", "expected_revision": revision})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/reviewer/cases/"+caseID+"/review", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer synthetic-local-actor")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	dvhttp.NewReviewerCasesHandler(committedGapAuthenticator{actor}, wrapped).ServeHTTP(response, request.WithContext(ctx))
	if response.Code != 404 || wrapped.reviews != 1 || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("postcommit hidden readback status=%d reviews=%d", response.Code, wrapped.reviews)
	}
	read, err := observer.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback(context.Background())
	var approved, revoked, audited, reviewed bool
	var actualRevision uint64
	err = read.QueryRow(ctx, `SELECT c.state='aprobada' AND c.superseded_at IS NULL,c.revision,g.revoked_at IS NOT NULL,EXISTS(SELECT 1 FROM public.document_verification_reviewer_grant_event e WHERE e.grant_id=g.id AND e.action='revoked'),EXISTS(SELECT 1 FROM public.document_verification_event e WHERE e.case_id=c.id AND e.action='review_decision' AND e.actor_id=$2 AND e.to_state='aprobada' AND e.revision=$4) FROM public.document_verification_case c JOIN public.document_verification_reviewer_grant g ON g.id=$3 WHERE c.id=$1`, caseID, actor, grantID, revision+1).Scan(&approved, &actualRevision, &revoked, &audited, &reviewed)
	if err != nil || !approved || actualRevision != revision+1 || !revoked || !audited || !reviewed {
		t.Fatal("independent committed state and audit readback mismatch")
	}
	if err = read.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Log("REAL_HTTP_404_AFTER_COMMITTED_APPROVAL_AND_COMMITTED_ASSIGNMENT_WITHDRAWAL; INDEPENDENT_READ_ONLY_COMMIT_AUDIT_PASS; SELF_CASE_NOT_SUPERSEDED; NO_REPLAY")
}
