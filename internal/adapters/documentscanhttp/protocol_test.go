package documentscanhttp

import (
	"encoding/json"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestScanResponseUsesDeclaredWireNames(t *testing.T) {
	b, err := json.Marshal(ScanResponse{State: dv.FileClean, ScannerVersion: "synthetic", SignaturesAt: time.Unix(1, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"state", "scanner_version", "signatures_at"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("missing declared wire field %s", name)
		}
	}
	if len(fields) != 3 {
		t.Errorf("unexpected wire fields: %d", len(fields))
	}
}
