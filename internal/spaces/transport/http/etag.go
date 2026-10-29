package spaceshttp

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

var (
	ErrMissingPrecondition = errors.New("missing If-Match precondition")
	ErrInvalidPrecondition = errors.New("invalid If-Match precondition")
	ErrStalePrecondition   = errors.New("stale If-Match precondition")

	errMissingPrecondition = ErrMissingPrecondition
	errInvalidPrecondition = ErrInvalidPrecondition
	errStalePrecondition   = ErrStalePrecondition
)

func DraftETag(draft spaces.Draft) string {
	payload := draft.ID + "\x00" + draft.UpdatedAt.UTC().Format(time.RFC3339Nano)
	return `"` + base64.RawURLEncoding.EncodeToString([]byte(payload)) + `"`
}

func draftETag(draft spaces.Draft) string { return DraftETag(draft) }

// ParseIfMatch validates exactly one strong entity tag and returns its timestamp.
func ParseIfMatch(values []string, expectedID string) (string, error) {
	return parseIfMatch(values, expectedID)
}

// parseIfMatch validates exactly one strong entity tag and returns its timestamp.
func parseIfMatch(values []string, expectedID string) (string, error) {
	if len(values) == 0 {
		return "", errMissingPrecondition
	}
	if len(values) != 1 {
		return "", errInvalidPrecondition
	}
	tag := values[0]
	if len(tag) < 3 || tag[0] != '"' || tag[len(tag)-1] != '"' {
		return "", errInvalidPrecondition
	}
	encoded := tag[1 : len(tag)-1]
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return "", errInvalidPrecondition
	}
	id, timestamp, ok := strings.Cut(string(decoded), "\x00")
	if !ok || id == "" || strings.Contains(timestamp, "\x00") || timestamp == "" {
		return "", errInvalidPrecondition
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || parsed.Location() != time.UTC || parsed.Format(time.RFC3339Nano) != timestamp {
		return "", errInvalidPrecondition
	}
	if id != expectedID {
		return "", errStalePrecondition
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}
