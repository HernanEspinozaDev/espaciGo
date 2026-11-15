package documentverificationpg

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type ownerDecisionReasonScanRow dv.ReasonCode

func (row ownerDecisionReasonScanRow) Scan(dest ...any) error {
	if len(dest) != 18 {
		return fmt.Errorf("owner snapshot scan destinations=%d, want 18", len(dest))
	}
	reason, ok := dest[17].(*dv.ReasonCode)
	if !ok {
		return fmt.Errorf("owner decision reason destination has type %T", dest[17])
	}
	*reason = dv.ReasonCode(row)
	return nil
}

func TestScanOwnerSnapshotKeepsDecisionReasonInPrivateProjection(t *testing.T) {
	const reason = dv.ReasonEvidenceIncomplete
	snapshot, err := scanOwnerSnapshot(ownerDecisionReasonScanRow(reason))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.OwnerDecisionReason != reason {
		t.Fatalf("OwnerDecisionReason=%q, want %q", snapshot.OwnerDecisionReason, reason)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), string(reason)) || strings.Contains(string(encoded), "OwnerDecisionReason") {
		t.Fatalf("private decision reason appeared in Snapshot JSON: %s", encoded)
	}
}
