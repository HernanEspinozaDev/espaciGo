package media

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	MaxInputBytes = 10 << 20
	MaxPixels     = int64(25_000_000)
	MaxSide       = 12_000
	MaxSpaceMedia = 10
	MaxAltText    = 250
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89a-fA-F][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func validUUID(v string) bool { return uuidRE.MatchString(strings.TrimSpace(v)) }

var (
	ErrInvalid      = errors.New("media: invalid image or request")
	ErrNotFound     = errors.New("media: not found")
	ErrLimit        = errors.New("media: gallery limit reached")
	ErrConflict     = errors.New("media: idempotency or state conflict")
	ErrUnavailable  = errors.New("media: private object store unavailable")
	ErrNotSupported = errors.New("media: image format not supported")
)

type Purpose string

const (
	PurposeSpace   Purpose = "espacio"
	PurposeProfile Purpose = "perfil"
)

type VariantName string

const (
	VariantDisplay   VariantName = "display"
	VariantThumbnail VariantName = "thumbnail"
	VariantAvatar    VariantName = "avatar"
)

type Variant struct {
	Name       VariantName `json:"name"`
	URL        string      `json:"url"`
	MIME       string      `json:"mime_type"`
	Size       int64       `json:"size_bytes"`
	SHA256     string      `json:"sha256"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	ObjectKey  string      `json:"-"`
	Provider   string      `json:"-"`
	Bucket     string      `json:"-"`
	Generation int64       `json:"-"`
	Bytes      []byte      `json:"-"`
}

type Photo struct {
	ID         string    `json:"id"`
	SpaceID    string    `json:"space_id"`
	OwnerID    string    `json:"-"`
	AltText    string    `json:"alt_text"`
	Visible    bool      `json:"visible"`
	Cover      bool      `json:"cover"`
	Position   int       `json:"position"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	Variants   []Variant `json:"variants"`
	Idempotent bool      `json:"reused,omitempty"`
}

type Reservation struct {
	ID        string
	OwnerID   string
	SpaceID   string
	Purpose   Purpose
	Token     int64
	LeaseTill time.Time
	State     string
	InputHash string
}

type StoredObject struct {
	ObjectKey  string
	Provider   string
	Bucket     string
	Generation int64
	Size       int64
	SHA256     string
}

type ObjectStore interface {
	Put(context.Context, string, []byte, string) (StoredObject, error)
	Get(context.Context, string, int64) ([]byte, error)
	Generation(context.Context, string) (int64, error)
	Delete(context.Context, string, int64) error
}

type Repository interface {
	Reserve(context.Context, string, string, string, Purpose, string, string, time.Time, time.Time) (Reservation, bool, error)
	PlanVariant(context.Context, string, int64, VariantName, string) error
	FinishVariantWrite(context.Context, string, VariantName, string) error
	RecordVariant(context.Context, string, int64, Variant) error
	FinalizeSpace(context.Context, string, int64, string, string, time.Time) (Photo, error)
	FailReservation(context.Context, string, int64, time.Time) error
	ListSpace(context.Context, string, string) ([]Photo, error)
	GetSpace(context.Context, string, string, string) (Photo, error)
	UpdateSpacePhoto(context.Context, string, string, string, *string, *bool, *int, time.Time) (Photo, error)
	SelectSpaceCover(context.Context, string, string, string, time.Time) error
	RemoveSpacePhoto(context.Context, string, string, string, time.Time) error
	GetVariant(context.Context, string, string, string) (Variant, error)
	FinalizeProfile(context.Context, string, string, int64, time.Time) error
	GetProfilePhoto(context.Context, string) (Photo, error)
	DeleteProfilePhoto(context.Context, string, time.Time) error
	ClaimCleanup(context.Context, int, time.Time) ([]CleanupItem, error)
	CompleteCleanup(context.Context, string, int64, time.Time) error
	FailCleanup(context.Context, string, int64, time.Time) error
}

type CleanupItem struct {
	AssetID  string
	Token    int64
	Variants []Variant
	Intents  []ObjectIntent
}

type ObjectIntent struct {
	Name          VariantName
	ObjectKey     string
	Generation    int64
	WriteComplete bool
}
