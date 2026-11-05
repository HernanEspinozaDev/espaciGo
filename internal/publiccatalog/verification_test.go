package publiccatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSpaceVerificationJSONContainsOnlyPublicBooleans(t *testing.T) {
	space := Space{Verification: VerificationSummary{IdentityVerified: true, SpaceVerified: false}}
	encoded, err := json.Marshal(space)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	verification, ok := got["verification"].(map[string]any)
	if !ok {
		t.Fatalf("verification missing or wrong type: %s", encoded)
	}
	if verification["identity_verified"] != true || verification["space_verified"] != false {
		t.Fatalf("unexpected verification payload: %v", verification)
	}
	if len(verification) != 2 {
		t.Fatalf("verification must expose only two booleans: %v", verification)
	}
	for _, forbidden := range []string{"owner_id", "case_id", "file_id", "reviewer", "verified_at", "expires_at", "path"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("private field %q leaked: %s", forbidden, encoded)
		}
	}
}
