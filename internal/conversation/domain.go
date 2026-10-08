package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ArchiveSectionRepository exports only messages authored by the participant
// and that participant's own read cursor; it never returns counterpart text.
type ArchiveSectionRepository interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
}

var (
	ErrInvalid    = errors.New("conversation: invalid request")
	ErrNotFound   = errors.New("conversation: thread not found")
	ErrConflict   = errors.New("conversation: operation conflicts with thread state")
	ErrRepository = errors.New("conversation: repository unavailable")
)

const SafetyNotice = "ENSAYO LOCAL — MENSAJES SINTÉTICOS"

type Message struct {
	ID            string    `json:"id"`
	ReservationID string    `json:"reservation_id"`
	AuthorID      string    `json:"author_id"`
	Sequence      int64     `json:"sequence"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
}

type Page struct {
	Items       []Message `json:"items"`
	OlderCursor *int64    `json:"older_cursor"`
}

type Repository interface {
	List(context.Context, string, string, *int64, int) (Page, error)
	Send(context.Context, string, string, string, string, []byte, string, func() time.Time) (Message, error)
	MarkRead(context.Context, string, string, int64) (int64, error)
}
