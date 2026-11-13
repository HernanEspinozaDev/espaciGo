package migrator

import (
	"encoding/json"
	"testing"
)

func TestRecoveryInspectionReportJSONContract(t *testing.T) {
	r := RecoveryInspectionReport{Format: "espacigo.recovery-inspection.v1", ReadOnly: true, Applied: []HistoryEntry{}, Pending: []MigrationSummary{}, Authority: RecoveryAuthority{}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"format", "read_only", "database", "server_version", "session_user", "current_user", "applied", "pending", "authority"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing JSON field %q: %s", key, b)
		}
	}
	if got["format"] != "espacigo.recovery-inspection.v1" || got["read_only"] != true {
		t.Fatalf("unexpected contract: %s", b)
	}
}
