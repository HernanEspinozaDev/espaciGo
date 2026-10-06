package conversation

import (
	"context"
	"errors"
	"time"
)

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
