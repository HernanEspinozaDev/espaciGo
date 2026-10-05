package spaces

import (
	"context"
	"errors"
	"math"
	"strings"
)

var (
	ErrInvalid  = errors.New("spaces: invalid draft")
	ErrNotFound = errors.New("spaces: draft not found")
)

type Draft struct {
	ID           string  `json:"id"`
	CategoryCode string  `json:"category_code"`
	CategoryName string  `json:"category_name"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	AreaM2       float64 `json:"area_m2"`
	Capacity     int32   `json:"capacity"`
	UsageRules   string  `json:"usage_rules"`
	RateUnit     string  `json:"rate_unit"`
	BasePriceCLP int64   `json:"base_price_clp"`
	Address      string  `json:"address"`
	State        string  `json:"state"`
}

type Input struct {
	CategoryCode string  `json:"category_code"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	AreaM2       float64 `json:"area_m2"`
	Capacity     int32   `json:"capacity"`
	UsageRules   string  `json:"usage_rules"`
	RateUnit     string  `json:"rate_unit"`
	BasePriceCLP int64   `json:"base_price_clp"`
	Address      string  `json:"address"`
}

func (i Input) Validate() error {
	areaCents := i.AreaM2 * 100
	if strings.TrimSpace(i.Title) == "" || len([]rune(i.Title)) > 70 || len([]rune(strings.TrimSpace(i.Description))) < 100 || math.IsNaN(i.AreaM2) || math.IsInf(i.AreaM2, 0) || i.AreaM2 < 0.01 || i.AreaM2 > 99999999.99 || math.Abs(areaCents-math.Round(areaCents)) > 1e-7 || i.CategoryCode == "" || i.Capacity <= 0 || strings.TrimSpace(i.UsageRules) == "" || len([]rune(i.UsageRules)) > 250 || (i.RateUnit != "hora" && i.RateUnit != "dia" && i.RateUnit != "mes") || i.BasePriceCLP <= 5000 || strings.TrimSpace(i.Address) == "" || len([]rune(i.Address)) > 500 {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	Categories(ctx context.Context) ([]Category, error)
	Create(ctx context.Context, owner string, input Input) (Draft, error)
	ListOwn(ctx context.Context, owner string) ([]Draft, error)
	GetOwn(ctx context.Context, owner, id string) (Draft, error)
	UpdateOwn(ctx context.Context, owner, id string, input Input) (Draft, error)
}

type Category struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
