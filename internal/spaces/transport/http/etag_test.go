package spaceshttp

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

func TestDraftETagRoundTrip(t *testing.T) {
	updated := time.Date(2026, 10, 27, 12, 13, 14, 123456789, time.FixedZone("offset", -3*60*60))
	draft := spaces.Draft{ID: "space-123", UpdatedAt: updated}
	got := draftETag(draft)
	want := `"` + base64.RawURLEncoding.EncodeToString([]byte("space-123\x00"+updated.UTC().Format(time.RFC3339Nano))) + `"`
	if got != want {
		t.Fatalf("draftETag() = %q, want %q", got, want)
	}
	id, err := parseIfMatch([]string{got}, draft.ID)
	if err != nil {
		t.Fatalf("parseIfMatch() error = %v", err)
	}
	if id != updated.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("parsed timestamp = %q", id)
	}
}

func TestExportedParseIfMatchPreconditionErrors(t *testing.T) {
	cases := []struct {
		name     string
		values   []string
		expected string
		want     error
	}{
		{"missing", nil, "space-123", ErrMissingPrecondition},
		{"invalid", []string{"*"}, "space-123", ErrInvalidPrecondition},
		{"stale", []string{draftETag(spaces.Draft{ID: "other", UpdatedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})}, "space-123", ErrStalePrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseIfMatch(tc.values, tc.expected)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseIfMatchPreconditionErrors(t *testing.T) {
	valid := draftETag(spaces.Draft{ID: "space-123", UpdatedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})
	cases := []struct {
		name     string
		values   []string
		expected string
		want     error
	}{
		{"missing", nil, "space-123", errMissingPrecondition},
		{"multiple", []string{valid, valid}, "space-123", errInvalidPrecondition},
		{"comma list", []string{valid + ", " + valid}, "space-123", errInvalidPrecondition},
		{"weak", []string{"W/" + valid}, "space-123", errInvalidPrecondition},
		{"wildcard", []string{"*"}, "space-123", errInvalidPrecondition},
		{"unquoted", []string{strings.Trim(valid, `"`)}, "space-123", errInvalidPrecondition},
		{"bad base64", []string{`"%%%"`}, "space-123", errInvalidPrecondition},
		{"mismatch", []string{draftETag(spaces.Draft{ID: "other", UpdatedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})}, "space-123", errStalePrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseIfMatch(tc.values, tc.expected)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
		})
	}
}
