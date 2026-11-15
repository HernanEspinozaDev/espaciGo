package migrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIdentityV95CaseDeclarationBindingContract(t *testing.T) {
	path := filepath.Join("..", "..", "db", "migrations", "V000095__identity_document_case_declaration_binding.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, required := range []string{
		"identity_declaration_revision_id uuid",
		"document_format_declared text",
		"capture_consent_version text",
		"capture_consented_at timestamptz",
		"FOREIGN KEY (identity_declaration_revision_id, subject_id)",
		"REFERENCES public.identity_declaration_revision(id, account_id)",
		"ON DELETE RESTRICT",
		"identity-capture-local-v1",
		"document_format_declared IS NOT NULL",
		"capture_consent_version IS NOT NULL",
		"capture_consented_at IS NOT NULL",
		"document_verification_case_identity_declaration_idx",
		"CREATE FUNCTION public.document_verification_identity_case_insert_guard()",
		"CREATE TRIGGER document_verification_identity_case_insert_guard",
		"CREATE FUNCTION public.document_verification_identity_case_link_guard()",
		"CREATE TRIGGER document_verification_identity_case_link_guard",
		"NEW.identity_declaration_revision_id IS DISTINCT FROM OLD.identity_declaration_revision_id",
		"NEW.state IS NULL",
		"NEW.state NOT IN ('rechazada','vencida','revocada')",
		"CREATE FUNCTION public.identity_declaration_case_retention_guard()",
		"RAISE lock_not_available",
		"CREATE TRIGGER identity_declaration_case_retention_guard",
		"GRANT INSERT (identity_declaration_revision_id,document_format_declared,capture_consent_version,capture_consented_at)",
		"GRANT UPDATE (identity_declaration_revision_id)",
		"ON public.document_verification_case TO espacigo_runtime",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("V95 missing identity-case persistence contract %q", required)
		}
	}
	if strings.Contains(sql, "DROP ") || strings.Contains(sql, "DELETE FROM public.document_verification_case") {
		t.Fatal("V95 must preserve V1-V94 history and existing identity cases")
	}
}

func assertV95IdentityCaseBindingContract(t *testing.T, ctx context.Context, db *pgx.Conn) {
	t.Helper()
	var linkedFK bool
	if err := db.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(confdeltype='r')
FROM pg_catalog.pg_constraint
WHERE conname='document_verification_case_identity_declaration_fk'
  AND conrelid='public.document_verification_case'::regclass
  AND confrelid='public.identity_declaration_revision'::regclass AND contype='f'`).Scan(&linkedFK); err != nil || !linkedFK {
		t.Fatalf("V95 composite owner/declaration FK is missing or not RESTRICT: %v", err)
	}
	var insertLink, updateLink, updateConsent, insertFormat, insertConsentVersion, insertConsentTime bool
	if err := db.QueryRow(ctx, `SELECT
    pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','identity_declaration_revision_id','INSERT'),
    pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','identity_declaration_revision_id','UPDATE'),
    pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','capture_consent_version','UPDATE'),
    pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','document_format_declared','INSERT'),
    pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','capture_consent_version','INSERT'),
    pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','capture_consented_at','INSERT')`).Scan(
		&insertLink, &updateLink, &updateConsent, &insertFormat, &insertConsentVersion, &insertConsentTime); err != nil ||
		!insertLink || !updateLink || updateConsent || !insertFormat || !insertConsentVersion || !insertConsentTime {
		t.Fatalf("V95 identity-case column grants are wrong: err=%v insertLink=%v updateLink=%v updateConsent=%v format=%v consentVersion=%v consentTime=%v",
			err, insertLink, updateLink, updateConsent, insertFormat, insertConsentVersion, insertConsentTime)
	}
	var triggers int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_trigger
WHERE tgrelid IN ('public.document_verification_case'::regclass,'public.identity_declaration_revision'::regclass)
  AND tgname IN ('document_verification_identity_case_insert_guard','document_verification_identity_case_link_guard','identity_declaration_case_retention_guard')
  AND tgenabled='O' AND NOT tgisinternal`).Scan(&triggers); err != nil || triggers != 3 {
		t.Fatalf("V95 insert/link/retention guards are not enabled: count=%d err=%v", triggers, err)
	}
}
