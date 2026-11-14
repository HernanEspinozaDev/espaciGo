package rentalflowhttp

import (
	"encoding/json"
	"strings"
	"testing"
)

// errorDisclosureBody excludes the random correlation ID from privacy assertions
// while retaining every other response field for leak detection.
func errorDisclosureBody(t *testing.T, body []byte) string {
	t.Helper()

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response envelope for disclosure assertion: %v", err)
	}
	rawError, ok := envelope["error"]
	if !ok {
		t.Fatalf("response has no error envelope: %s", body)
	}
	var apiError map[string]json.RawMessage
	if err := json.Unmarshal(rawError, &apiError); err != nil || len(apiError) == 0 {
		t.Fatalf("decode error object for disclosure assertion: err=%v body=%s", err, rawError)
	}
	delete(apiError, "request_id")
	filteredError, err := json.Marshal(apiError)
	if err != nil {
		t.Fatalf("marshal filtered error object: %v", err)
	}
	envelope["error"] = filteredError
	filteredEnvelope, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal filtered response envelope: %v", err)
	}
	return string(filteredEnvelope)
}

func TestErrorDisclosureBodyIgnoresRequestIDButRetainsLeakableFields(t *testing.T) {
	body := []byte(`{"error":{"code":"conflict","message":"No fue posible completar la solicitud.","request_id":"d12000c0-e08d-4867-ab62-7ec64c9a3f54"}}`)
	filtered := errorDisclosureBody(t, body)
	if strings.Contains(filtered, "12000") || !strings.Contains(filtered, "conflict") {
		t.Fatalf("disclosure surface=%s; should ignore request_id and retain error code", filtered)
	}

	body = []byte(`{"error":{"code":"conflict","message":"private subtotal 12000","request_id":"safe-id"}}`)
	filtered = errorDisclosureBody(t, body)
	if !strings.Contains(filtered, "12000") {
		t.Fatalf("disclosure surface=%s; should retain sensitive error message", filtered)
	}
}
