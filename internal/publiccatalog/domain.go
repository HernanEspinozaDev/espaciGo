package publiccatalog

import (
	"context"
	"errors"
)

var (
	ErrInvalid  = errors.New("public catalog: invalid request")
	ErrNotFound = errors.New("public catalog: resource not found")
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 25
)

type Category struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

// Space is an intentional public projection. It contains no owner, address,
// profile JSON, fixture state, availability, or file/blob references.
type Space struct {
	SpaceID      string  `json:"space_id"`
	CategoryCode string  `json:"category_code"`
	CategoryName string  `json:"category_name"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	AreaM2       float64 `json:"area_m2"`
	RateUnit     string  `json:"rate_unit"`
	BasePriceCLP int64   `json:"base_price_clp"`
	Currency     string  `json:"currency"`

	CategoryOrder int    `json:"-"`
	TitleKey      string `json:"-"`
}

type Filters struct {
	CategoryCode    string
	RateUnit        string
	MinUnitPriceCLP *int64
	MaxUnitPriceCLP *int64
	Sort            string
	PageSize        int
	Cursor          string
}

type Cursor struct {
	Version           int    `json:"v"`
	Fingerprint       string `json:"f"`
	Sort              string `json:"s"`
	PageSize          int    `json:"n"`
	LastCategoryOrder int    `json:"c,omitempty"`
	LastTitleKey      string `json:"t,omitempty"`
	LastUnitPriceCLP  int64  `json:"p,omitempty"`
	LastSpaceID       string `json:"i"`
}

type Page struct {
	Items      []Space `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Repository interface {
	Categories(context.Context) ([]Category, error)
	HasActiveCategory(context.Context, string) (bool, error)
	List(context.Context, Filters, *Cursor, int) ([]Space, error)
	Detail(context.Context, string) (Space, error)
}
