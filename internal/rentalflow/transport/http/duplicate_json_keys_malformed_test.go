package rentalflowhttp

import "testing"

func TestRejectDuplicateJSONKeysRejectsMalformedObjects(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{name: "truncated immediately after object opener", body: []byte(`{`)},
		{name: "missing object delimiter", body: []byte(`{"key":1`)},
		{name: "missing value", body: []byte(`{"key":}`)},
		{name: "truncated value", body: []byte(`{"key":`)},
		{name: "mismatched object closer", body: []byte{'{', '"', 'k', 'e', 'y', '"', ':', '1', ']'}},
		{name: "non-ASCII object key", body: []byte(`{"espacio-é":1}`)},
		{name: "multiple JSON values", body: []byte(`{"key":1} {}`)},
		{name: "malformed trailing bytes", body: []byte{'{', '"', 'k', '"', ':', '1', '}', ' ', 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := rejectDuplicateJSONKeys(tc.body); err == nil {
				t.Fatalf("rejectDuplicateJSONKeys(%q) returned nil", tc.body)
			}
		})
	}
}
