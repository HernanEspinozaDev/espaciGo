// Package gallery exposes the local synthetic gallery of an owner's space.
// Image bytes are created by the backend; client supplied files are not accepted.
package gallery

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const MaxPhotosPerSpace = 10
const SyntheticFixture = "synthetic-png-v1"

var (
	ErrInvalid              = errors.New("gallery: invalid request")
	ErrNotFound             = errors.New("gallery: not found")
	ErrLimit                = errors.New("gallery: photo limit reached")
	ErrFileStore            = errors.New("gallery: private file unavailable")
	ErrCandidateUnavailable = errors.New("gallery: file candidate is being cleaned")
)

type Photo struct {
	ID        string    `json:"id"`
	Fixture   string    `json:"fixture_code"`
	MIME      string    `json:"mime_type"`
	SHA256    string    `json:"sha256"`
	Size      int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type Removal struct {
	PhotoID string `json:"photo_id"`
	Removed bool   `json:"removed"`
	Reused  bool   `json:"reused"`
}

type Repository interface {
	ReserveCandidate(context.Context, string, string, string, time.Time) error
	BeginCandidate(context.Context, string, string, string) (CandidateWriter, error)
	QueueCandidateCleanup(context.Context, string, time.Time) error
	ClaimCandidateCleanup(context.Context, int) ([]string, error)
	CompleteCandidateCleanup(context.Context, string) error
	FailCandidateCleanup(context.Context, string, time.Time) error
	List(context.Context, string, string) ([]Photo, error)
	Get(context.Context, string, string, string) (Photo, error)
	Remove(context.Context, string, string, string) (Removal, error)
	PendingCleanup(context.Context, int) ([]string, error)
	CompleteCleanup(context.Context, string, time.Time) error
	FailCleanup(context.Context, string, time.Time) error
}

// CandidateWriter keeps the durable candidate row locked while bytes are
// written and either attached to a photo or queued for cleanup.
type CandidateWriter interface {
	Add(context.Context, Photo, string, time.Time) (Photo, bool, error)
	QueueCleanup(context.Context, time.Time) error
	Close() error
}

type FileStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type IDGenerator interface{ ID() (string, error) }

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func validUUID(v string) bool { return uuidPattern.MatchString(strings.TrimSpace(v)) }
