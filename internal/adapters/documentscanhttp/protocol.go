package documentscanhttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

// ScanResponse is the bounded HTTP wire contract, not an authorization decision.
// The client and processor independently validate state, metadata and freshness.
const maxSignatureAge = 48 * time.Hour

type ScanResponse struct {
	State          dv.FileState `json:"state"`
	ScannerVersion string       `json:"scanner_version"`
	SignaturesAt   time.Time    `json:"signatures_at"`
}

func decodeScanResponse(data []byte) (ScanResponse, error) {
	bad := errors.New("documentscanhttp: invalid response")
	var result ScanResponse
	dec := json.NewDecoder(bytes.NewReader(data))
	first, err := dec.Token()
	if err != nil || first != json.Delim('{') {
		return ScanResponse{}, bad
	}
	seen := make(map[string]bool, 3)
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return ScanResponse{}, bad
		}
		seen[name] = true
		switch name {
		case "state":
			err = dec.Decode(&result.State)
		case "scanner_version":
			err = dec.Decode(&result.ScannerVersion)
		case "signatures_at":
			err = dec.Decode(&result.SignaturesAt)
		default:
			return ScanResponse{}, bad
		}
		if err != nil {
			return ScanResponse{}, bad
		}
	}
	last, err := dec.Token()
	if err != nil || last != json.Delim('}') || len(seen) != 3 {
		return ScanResponse{}, bad
	}
	if _, err := dec.Token(); err != io.EOF {
		return ScanResponse{}, bad
	}
	return result, nil
}
