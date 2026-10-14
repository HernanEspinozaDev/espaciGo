package publiccatalog

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("public catalog: invalid request")
	ErrNotFound = errors.New("public catalog: resource not found")
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 48
)

type Category struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type Location struct {
	RegionCode  string `json:"region_code"`
	RegionName  string `json:"region_name"`
	CommuneCode string `json:"comuna_code"`
	CommuneName string `json:"comuna_name"`
}

type AttributeFilter struct {
	Code           string       `json:"code"`
	Label          string       `json:"label"`
	Type           string       `json:"type"`
	Order          int          `json:"order"`
	Unit           string       `json:"unit,omitempty"`
	Options        []string     `json:"options,omitempty"`
	Minimum        *json.Number `json:"minimum,omitempty"`
	Maximum        *json.Number `json:"maximum,omitempty"`
	Step           *json.Number `json:"step,omitempty"`
	ProfileVersion int          `json:"profile_version"`
}

// AttributePredicate is a validated public filter expression. Boolean filters
// retain the legacy Attributes map; other profile types use one equality/list
// value or an inclusive decimal range. Type is filled from the server profile.
type AttributePredicate struct {
	Type    string   `json:"type,omitempty"`
	Minimum *string  `json:"minimum,omitempty"`
	Maximum *string  `json:"maximum,omitempty"`
	Value   *string  `json:"value,omitempty"`
	Values  []string `json:"values,omitempty"`
}

// Space is an intentional public projection. It contains no owner, address,
// profile JSON, fixture state, availability, or file/blob references.
type Space struct {
	SpaceID           string           `json:"space_id"`
	CategoryCode      string           `json:"category_code"`
	CategoryName      string           `json:"category_name"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	AreaM2            float64          `json:"area_m2"`
	RateUnit          string           `json:"rate_unit"`
	BasePriceCLP      int64            `json:"base_price_clp"`
	Currency          string           `json:"currency"`
	RegionCode        *string          `json:"region_code,omitempty"`
	RegionName        *string          `json:"region_name,omitempty"`
	CommuneCode       *string          `json:"comuna_code,omitempty"`
	CommuneName       *string          `json:"comuna_name,omitempty"`
	EstimatedTotalCLP *int64           `json:"estimated_total_clp"`
	UsageRules        string           `json:"usage_rules,omitempty"`
	Features          []PublicFeature  `json:"features"`
	Images            []PublicImageRef `json:"images"`
	CoverImageID      *string          `json:"cover_image_id,omitempty"`
	TimeZone          string           `json:"-"`

	CategoryOrder int     `json:"-"`
	TitleKey      string  `json:"-"`
	RelevanceRank float64 `json:"-"`
}

type PublicFeature struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Value bool   `json:"value"`
}
type PublicImageRef struct {
	ImageID string `json:"image_id"`
	IsCover bool   `json:"is_cover"`
}

type Filters struct {
	CategoryCode     string
	RateUnit         string
	MinUnitPriceCLP  *int64
	MaxUnitPriceCLP  *int64
	Sort             string
	PageSize         int
	PageSizeSet      bool
	Cursor           string
	Query            string
	RegionCode       string
	CommuneCode      string
	MinAreaM2        string
	Attributes       map[string]bool
	TypedAttributes  map[string]AttributePredicate
	AttributeVersion int
	StartAt          *time.Time
	EndAt            *time.Time
	MinTotalCLP      *int64
	MaxTotalCLP      *int64
}

type Cursor struct {
	Version               int     `json:"v"`
	Fingerprint           string  `json:"f"`
	Sort                  string  `json:"s"`
	PageSize              int     `json:"n"`
	LastCategoryOrder     int     `json:"c,omitempty"`
	LastTitleKey          string  `json:"t,omitempty"`
	LastUnitPriceCLP      int64   `json:"p,omitempty"`
	LastEstimatedTotalCLP *int64  `json:"e,omitempty"`
	LastRelevanceRank     float64 `json:"r,omitempty"`
	LastSpaceID           string  `json:"i"`
}

type Page struct {
	Items      []Space `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Repository interface {
	Categories(context.Context) ([]Category, error)
	HasActiveCategory(context.Context, string) (bool, error)
	AttributeFilters(context.Context, string) ([]AttributeFilter, error)
	HasRegion(context.Context, string) (bool, error)
	HasCommune(context.Context, string, string) (bool, error)
	Locations(context.Context, string, string, int) ([]Location, error)
	List(context.Context, Filters, *Cursor, int) ([]Space, error)
	Detail(context.Context, string) (Space, error)
}

type PublicImageRepository interface {
	PublicImage(context.Context, string, string) (PublicImage, error)
}
type PublicImageStore interface {
	Get(context.Context, string) ([]byte, error)
}
type PublicImage struct {
	MIME, SHA256, FileID string
	Size                 int64
}
