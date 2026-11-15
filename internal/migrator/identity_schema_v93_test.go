package migrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityV93VerifiedClaimReverificationContract(t *testing.T) {
	path := filepath.Join("..", "..", "db", "migrations", "V000093__identity_verified_claim_reverification.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, required := range []string{
		"CREATE OR REPLACE FUNCTION public.commit_verified_identity_decision(",
		"session_user <> 'espacigo_migrator_login'", "current_user <> 'espacigo_schema_owner'",
		"current_schema() <> 'public'", "SECURITY DEFINER", "SET search_path = pg_catalog",
		"1d89b78eef8fbea571e8d8c1b914c52394d24a4b8b0a653c2b748bc41c27d646",
		"'CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW'",
		"'RECEIVED', 'FAILED'", "verified_at IS NOT NULL", "expires_at IS NULL",
		"expires_at > v_processed_at", "verified_at = COALESCE(v_claim.verified_at, v_processed_at)",
		"WHERE fingerprint = p_rut_fingerprint", "AND account_id = v_attempt.account_id",
		"AND declaration_id = v_attempt.declaration_id", "UPDATE public.identity_provider_attempt",
		"UPDATE public.identity_provider_event", "DELETE FROM public.identity_declaration_revision",
		"INSERT INTO public.outbox_evento_local", "GET DIAGNOSTICS v_rows = ROW_COUNT",
		"REVOKE ALL ON FUNCTION public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz) FROM PUBLIC",
		"GRANT EXECUTE ON FUNCTION public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz) TO espacigo_runtime",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("V93 missing verified-claim re-verification contract %q", required)
		}
	}
	if strings.Contains(sql, "GRANT ALL") || strings.Contains(sql, "ALTER ROLE") {
		t.Fatal("V93 must not grant or elevate privileges")
	}
	if !strings.Contains(sql, "v_search_path text[];") || strings.Contains(sql, "has_function_privilege('PUBLIC'") {
		t.Fatal("V93 guard must use catalog text[] type and PUBLIC ACL grantee, not a role named PUBLIC")
	}
	if !strings.Contains(sql, "ORDER BY x::text") {
		t.Fatal("V93 ACL comparison must order text values, not aclitem values")
	}
	clock := strings.Index(sql, "v_processed_at := GREATEST(")
	eligibility := strings.Index(sql, "v_claim.expires_at > v_processed_at")
	if clock < 0 || eligibility < clock || strings.Contains(sql, "v_claim.expires_at > p_processed_at") {
		t.Fatal("V93 eligibility and claim update must use the same effective clock before any outbox write")
	}
}
