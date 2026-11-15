package migrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIdentityV96ReviewerObservationAuditContract(t *testing.T) {
	path := filepath.Join("..", "..", "db", "migrations", "V000096__identity_reviewer_observation_audit.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, required := range []string{
		"ADD COLUMN document_format_observed text",
		"ADD COLUMN identity_review_observations jsonb",
		"document_verification_case_observed_format_ck",
		"document_verification_event_identity_observations_ck",
		"identity_review_observations = pg_catalog.jsonb_build_object(",
		"identity_review_observations->'identity_review_checklist' = pg_catalog.jsonb_build_object(",
		"document_format_observed",
		"document_expires_at_observed",
		"identity_review_checklist",
		"CREATE FUNCTION public.document_verification_identity_review_audit_guard()",
		"CREATE TRIGGER document_verification_identity_review_audit_guard",
		"v_revision IS DISTINCT FROM NEW.revision",
		"v_reviewer IS DISTINCT FROM NEW.actor_id",
		"GRANT UPDATE (document_format_observed)",
		"GRANT INSERT (identity_review_observations)",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("V96 missing identity-review persistence contract %q", required)
		}
	}
	if strings.Contains(sql, "DROP ") || strings.Contains(sql, "DELETE FROM public.document_verification_") {
		t.Fatal("V96 must be append-only and preserve historical cases/events")
	}
}

func assertV96IdentityReviewAuditContract(t *testing.T, ctx context.Context, db *pgx.Conn) {
	t.Helper()
	var caseColumn, eventColumn, caseUpdate, eventInsert, checkConstraint, trigger bool
	if err := db.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid='public.document_verification_case'::regclass AND attname='document_format_observed' AND attnum>0 AND NOT attisdropped),
		EXISTS (SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid='public.document_verification_event'::regclass AND attname='identity_review_observations' AND attnum>0 AND NOT attisdropped),
		pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','document_format_observed','UPDATE'),
		pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_event','identity_review_observations','INSERT'),
		EXISTS (SELECT 1 FROM pg_catalog.pg_constraint WHERE conrelid='public.document_verification_event'::regclass AND conname='document_verification_event_identity_observations_ck' AND contype='c'),
		EXISTS (SELECT 1 FROM pg_catalog.pg_trigger WHERE tgrelid='public.document_verification_event'::regclass AND tgname='document_verification_identity_review_audit_guard' AND tgenabled='O' AND NOT tgisinternal)`).Scan(
		&caseColumn, &eventColumn, &caseUpdate, &eventInsert, &checkConstraint, &trigger); err != nil ||
		!caseColumn || !eventColumn || !caseUpdate || !eventInsert || !checkConstraint || !trigger {
		t.Fatalf("V96 identity-review schema/ACL/trigger contract failed: err=%v case=%v event=%v update=%v insert=%v check=%v trigger=%v",
			err, caseColumn, eventColumn, caseUpdate, eventInsert, checkConstraint, trigger)
	}
}
