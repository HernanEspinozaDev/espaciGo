package dbbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCombinedHistoryACLParsingRegressions(t *testing.T) {
	t.Run("V85 unconditional revoke and conditional grant", func(t *testing.T) {
		source, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "V000085__identity_encrypted_declarations_and_minimal_events.sql"))
		if err != nil {
			t.Fatal(err)
		}
		effects, err := ExtractMigrationRuntimeACL(85, source)
		if err != nil {
			t.Fatal(err)
		}
		var unconditionalRevoke, conditionalGrant bool
		for _, effect := range effects {
			if effect.Action == "REVOKE" && effect.Condition == "" {
				unconditionalRevoke = true
			}
			if effect.Action == "GRANT" && effect.Condition == "role-exists:espacigo_runtime" {
				conditionalGrant = true
			}
		}
		if !unconditionalRevoke || !conditionalGrant {
			t.Fatalf("V85 lost action-specific condition: revoke=%v grant=%v effects=%+v", unconditionalRevoke, conditionalGrant, effects)
		}
	})
	t.Run("V95 procedural role mention without ACL", func(t *testing.T) {
		source, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "V000095__identity_document_case_declaration_binding.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ExtractMigrationRuntimeACL(95, source); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("supported text function signatures", func(t *testing.T) {
		for _, source := range []string{
			"GRANT EXECUTE ON FUNCTION public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz) TO espacigo_runtime;",
			"REVOKE ALL ON FUNCTION public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz) FROM PUBLIC;",
		} {
			grantee := "espacigo_runtime"
			if strings.Contains(source, " FROM PUBLIC") {
				grantee = "PUBLIC"
			}
			effects, err := ExtractMigrationACLForGrantee(90, []byte(source), grantee)
			if err != nil {
				t.Fatal(err)
			}
			if len(effects) != 1 || effects[0].Object != "public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz)" {
				t.Fatalf("unexpected parsed function ACL: %+v", effects)
			}
		}
	})
}
