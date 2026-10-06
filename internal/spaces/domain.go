package spaces

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
)

var (
	ErrInvalid  = errors.New("spaces: invalid draft")
	ErrNotFound = errors.New("spaces: draft not found")
)

type Draft struct {
	ID                     string         `json:"id"`
	CategoryCode           string         `json:"category_code"`
	CategoryName           string         `json:"category_name"`
	Title                  string         `json:"title"`
	Description            string         `json:"description"`
	AreaM2                 float64        `json:"area_m2"`
	Capacity               int32          `json:"capacity"`
	UsageRules             string         `json:"usage_rules"`
	RateUnit               string         `json:"rate_unit"`
	BasePriceCLP           int64          `json:"base_price_clp"`
	Address                string         `json:"address"`
	State                  string         `json:"state"`
	AttributeSchemaVersion int            `json:"attribute_schema_version"`
	Attributes             map[string]any `json:"attributes"`
}

type Input struct {
	CategoryCode           string         `json:"category_code"`
	Title                  string         `json:"title"`
	Description            string         `json:"description"`
	AreaM2                 float64        `json:"area_m2"`
	Capacity               int32          `json:"capacity"`
	UsageRules             string         `json:"usage_rules"`
	RateUnit               string         `json:"rate_unit"`
	BasePriceCLP           int64          `json:"base_price_clp"`
	Address                string         `json:"address"`
	AttributeSchemaVersion int            `json:"attribute_schema_version,omitempty"`
	Attributes             map[string]any `json:"attributes,omitempty"`
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
	// version 0 selects the latest profile; a positive version selects that
	// immutable profile version for existing drafts.
	Profile(ctx context.Context, category string, version int) (Profile, error)
	Create(ctx context.Context, owner string, input Input) (Draft, error)
	ListOwn(ctx context.Context, owner string) ([]Draft, error)
	GetOwn(ctx context.Context, owner, id string) (Draft, error)
	UpdateOwn(ctx context.Context, owner, id string, input Input) (Draft, error)
}

type Category struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Profile definitions are persisted as versioned catalog data. They drive API
// validation and mock form generation from a single source.
type Profile struct {
	CategoryCode  string                `json:"category_code"`
	SchemaVersion int                   `json:"schema_version"`
	Attributes    []AttributeDefinition `json:"attributes"`
}
type AttributeDefinition struct {
	Code            string   `json:"code"`
	Order           int      `json:"order"`
	Label           string   `json:"label"`
	Description     string   `json:"description,omitempty"`
	Type            string   `json:"type"`
	Unit            string   `json:"unit,omitempty"`
	Options         []string `json:"options,omitempty"`
	Minimum         *float64 `json:"minimum,omitempty"`
	Maximum         *float64 `json:"maximum,omitempty"`
	Step            *float64 `json:"step,omitempty"`
	FilterCandidate bool     `json:"filter_candidate"`
}

func (p Profile) ValidateAttributes(values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	data, err := json.Marshal(values)
	if err != nil || len(data) > 16*1024 {
		return ErrInvalid
	}
	defs := make(map[string]AttributeDefinition, len(p.Attributes))
	for _, d := range p.Attributes {
		defs[d.Code] = d
	}
	for code, value := range values {
		d, ok := defs[code]
		if !ok || value == nil {
			return ErrInvalid
		}
		if err := d.validate(value); err != nil {
			return err
		}
	}
	if p.CategoryCode == "quincho" {
		typ, hasType := values["tipo_parrilla"].(string)
		count, hasCount := numeric(values["parrillas_disponibles"])
		if hasType && typ == "sin_parrilla" && hasCount && count != 0 {
			return ErrInvalid
		}
		if hasCount && count > 0 && (!hasType || typ == "sin_parrilla") {
			return ErrInvalid
		}
	}
	return nil
}

// ValidateAttributeFilters validates a partial set of search predicates using
// the selected immutable profile version. Unlike a draft, a filter is a set of
// independent predicates and must not trigger cross-attribute draft rules.
func (p Profile) ValidateAttributeFilters(values map[string]any) error {
	data, err := json.Marshal(values)
	if err != nil || len(data) > 16*1024 {
		return ErrInvalid
	}
	defs := make(map[string]AttributeDefinition, len(p.Attributes))
	for _, d := range p.Attributes {
		defs[d.Code] = d
	}
	for code, value := range values {
		definition, ok := defs[code]
		if !ok || value == nil || definition.validate(value) != nil {
			return ErrInvalid
		}
	}
	return nil
}

func (d AttributeDefinition) validate(value any) error {
	switch d.Type {
	case "boolean":
		if _, ok := value.(bool); !ok {
			return ErrInvalid
		}
	case "integer":
		n, ok := numeric(value)
		if !ok || n != math.Trunc(n) {
			return ErrInvalid
		}
		if d.Minimum != nil && n < *d.Minimum || d.Maximum != nil && n > *d.Maximum {
			return ErrInvalid
		}
	case "number":
		n, ok := numeric(value)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return ErrInvalid
		}
		if d.Minimum != nil && n < *d.Minimum || d.Maximum != nil && n > *d.Maximum {
			return ErrInvalid
		}
		if d.Step != nil && math.Abs(n / *d.Step - math.Round(n / *d.Step)) > 1e-7 {
			return ErrInvalid
		}
	case "enum":
		v, ok := value.(string)
		if !ok || !contains(d.Options, v) {
			return ErrInvalid
		}
	case "enum_list":
		values, ok := value.([]any)
		if !ok || len(values) == 0 || len(values) > len(d.Options) {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for _, item := range values {
			v, ok := item.(string)
			if !ok || !contains(d.Options, v) || seen[v] {
				return ErrInvalid
			}
			seen[v] = true
		}
	default:
		return ErrInvalid
	}
	return nil
}
func numeric(v any) (float64, bool) { n, ok := v.(float64); return n, ok }
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
