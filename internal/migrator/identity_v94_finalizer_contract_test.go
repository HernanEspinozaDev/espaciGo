package migrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityV94FinalizerPinsMigrationName(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "db", "repair", "finalize_native_kyc_v94.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"(SELECT name FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'V000094__identity_delivered_session_outbox_retirement.sql'",
		"(SELECT checksum FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9'",
	} {
		if !strings.Contains(string(b), required) {
			t.Errorf("finalizer must pin exact ledger identity: missing %s", required)
		}
	}
}
