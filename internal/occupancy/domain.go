package occupancy

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid          = errors.New("occupancy: invalid request")
	ErrNotFound         = errors.New("occupancy: resource not found")
	ErrConflict         = errors.New("occupancy: interval overlaps an active occupancy")
	ErrTimezoneRequired = errors.New("occupancy: space timezone is not configured")
)

type Block struct {
	ID        string    `json:"id"`
	SpaceID   string    `json:"space_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	TimeZone  string    `json:"time_zone"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type BlockInput struct {
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
	Reason  string `json:"reason"`
}

type Availability struct {
	SpaceID   string    `json:"space_id"`
	TimeZone  string    `json:"time_zone"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Available bool      `json:"available"`
}

type Calendar struct {
	SpaceID  string  `json:"space_id"`
	TimeZone string  `json:"time_zone"`
	Items    []Block `json:"items"`
}

type Repository interface {
	SetTimeZone(context.Context, string, string, string) error
	TimeZone(context.Context, string, string) (string, error)
	Availability(context.Context, string, string, time.Time, time.Time) (Availability, error)
	ListBlocks(context.Context, string, string, time.Time, time.Time) (Calendar, error)
	CreateBlock(context.Context, string, string, string, time.Time, time.Time, string) (Block, error)
	DeleteBlock(context.Context, string, string, string) error
}
