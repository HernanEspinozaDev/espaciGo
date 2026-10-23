package spaces

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid             = errors.New("spaces: invalid draft")
	ErrNotFound            = errors.New("spaces: draft not found")
	ErrEligibilityRequired = errors.New("spaces: synthetic KYC eligibility required")
	ErrPublicationConflict = errors.New("spaces: publication state conflict")
	ErrEnabledFixture      = errors.New("spaces: enabled synthetic fixture prevents publication transition")
	ErrTimeZoneRequired    = errors.New("spaces: valid IANA time zone required before publication")
	ErrDeleteBlocked       = errors.New("spaces: publication has open obligations")
)

// FieldError exposes only a stable field path and local validation code. It
// never contains submitted values or validator internals.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// ValidationError preserves errors.Is(err, ErrInvalid) while carrying safe,
// allowlisted field details for clients that can present them.
type ValidationError struct {
	Details []FieldError
}

func (e *ValidationError) Error() string { return ErrInvalid.Error() }
func (e *ValidationError) Unwrap() error { return ErrInvalid }

func NewValidationError(details []FieldError) error {
	if len(details) == 0 {
		return ErrInvalid
	}
	return &ValidationError{Details: append([]FieldError(nil), details...)}
}

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
	TimeZone               *string        `json:"time_zone,omitempty"`
	RegionCode             *string        `json:"region_code,omitempty"`
	CommuneCode            *string        `json:"comuna_code,omitempty"`
	CoverPhotoID           *string        `json:"cover_photo_id,omitempty"`
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
	RegionCode             string         `json:"region_code,omitempty"`
	CommuneCode            string         `json:"comuna_code,omitempty"`
	AttributeSchemaVersion int            `json:"attribute_schema_version,omitempty"`
	Attributes             map[string]any `json:"attributes,omitempty"`
}

// PublishedContentInput limits edits of an existing local publication to
// supported listing details. Nil means that field is unchanged.
type PublishedContentInput struct {
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	Capacity     *int32  `json:"capacity,omitempty"`
	UsageRules   *string `json:"usage_rules,omitempty"`
	BasePriceCLP *int64  `json:"base_price_clp,omitempty"`
	RegionCode   *string `json:"region_code,omitempty"`
	CommuneCode  *string `json:"comuna_code,omitempty"`
}

const PublicLocationCatalogVersion = "SUBDERE_DPA_2018"

// Deletion is a durable receipt for retiring one synthetic/local publication.
// Preserved contains only names of retained historical record groups.
type Deletion struct {
	SpaceID      string    `json:"space_id"`
	State        string    `json:"state"`
	OccurredAt   time.Time `json:"occurred_at"`
	Removed      []string  `json:"removed"`
	Preserved    []string  `json:"preserved"`
	FilesPending int       `json:"files_pending_cleanup"`
	Reused       bool      `json:"reused"`
}

func (i PublishedContentInput) Validate() error {
	if len(i.FieldErrors()) > 0 {
		return ErrInvalid
	}
	return nil
}

func (i PublishedContentInput) FieldErrors() []FieldError {
	var fields []FieldError
	if i.Title == nil && i.Description == nil && i.Capacity == nil && i.UsageRules == nil && i.BasePriceCLP == nil && i.RegionCode == nil && i.CommuneCode == nil {
		return []FieldError{{Field: "title", Code: "required"}}
	}
	if i.Title != nil {
		if strings.TrimSpace(*i.Title) == "" {
			fields = append(fields, FieldError{Field: "title", Code: "required"})
		} else if len([]rune(*i.Title)) > 70 {
			fields = append(fields, FieldError{Field: "title", Code: "max_length"})
		}
	}
	if i.Description != nil && len([]rune(strings.TrimSpace(*i.Description))) < 100 {
		fields = append(fields, FieldError{Field: "description", Code: "min_length"})
	}
	if i.Capacity != nil && *i.Capacity <= 0 {
		fields = append(fields, FieldError{Field: "capacity", Code: "minimum"})
	}
	if i.UsageRules != nil {
		if strings.TrimSpace(*i.UsageRules) == "" {
			fields = append(fields, FieldError{Field: "usage_rules", Code: "required"})
		} else if len([]rune(*i.UsageRules)) > 250 {
			fields = append(fields, FieldError{Field: "usage_rules", Code: "max_length"})
		}
	}
	if i.BasePriceCLP != nil && *i.BasePriceCLP <= 5000 {
		fields = append(fields, FieldError{Field: "base_price_clp", Code: "minimum"})
	}
	if (i.RegionCode == nil) != (i.CommuneCode == nil) {
		fields = append(fields, FieldError{Field: "region_code", Code: "location_pair"}, FieldError{Field: "comuna_code", Code: "location_pair"})
	}
	if i.RegionCode != nil && i.CommuneCode != nil {
		if (*i.RegionCode == "") != (*i.CommuneCode == "") {
			fields = append(fields, FieldError{Field: "region_code", Code: "location_pair"}, FieldError{Field: "comuna_code", Code: "location_pair"})
		}
		if *i.RegionCode != "" && !validPublicLocationCodes(*i.RegionCode, *i.CommuneCode) {
			fields = append(fields, FieldError{Field: "region_code", Code: "invalid_value"}, FieldError{Field: "comuna_code", Code: "invalid_value"})
		}
	}
	return fields
}

func (i Input) Validate() error {
	if len(i.FieldErrors()) > 0 {
		return ErrInvalid
	}
	return nil
}

func (i Input) FieldErrors() []FieldError {
	var fields []FieldError
	if strings.TrimSpace(i.CategoryCode) == "" {
		fields = append(fields, FieldError{Field: "category_code", Code: "required"})
	}
	if strings.TrimSpace(i.Title) == "" {
		fields = append(fields, FieldError{Field: "title", Code: "required"})
	} else if len([]rune(i.Title)) > 70 {
		fields = append(fields, FieldError{Field: "title", Code: "max_length"})
	}
	if len([]rune(strings.TrimSpace(i.Description))) < 100 {
		fields = append(fields, FieldError{Field: "description", Code: "min_length"})
	}
	areaCents := i.AreaM2 * 100
	if math.IsNaN(i.AreaM2) || math.IsInf(i.AreaM2, 0) {
		fields = append(fields, FieldError{Field: "area_m2", Code: "invalid_value"})
	} else if i.AreaM2 < 0.01 {
		fields = append(fields, FieldError{Field: "area_m2", Code: "minimum"})
	} else if i.AreaM2 > 99999999.99 {
		fields = append(fields, FieldError{Field: "area_m2", Code: "maximum"})
	} else if math.Abs(areaCents-math.Round(areaCents)) > 1e-7 {
		fields = append(fields, FieldError{Field: "area_m2", Code: "step"})
	}
	if i.Capacity <= 0 {
		fields = append(fields, FieldError{Field: "capacity", Code: "minimum"})
	}
	if strings.TrimSpace(i.UsageRules) == "" {
		fields = append(fields, FieldError{Field: "usage_rules", Code: "required"})
	} else if len([]rune(i.UsageRules)) > 250 {
		fields = append(fields, FieldError{Field: "usage_rules", Code: "max_length"})
	}
	if i.RateUnit != "hora" && i.RateUnit != "dia" && i.RateUnit != "mes" {
		fields = append(fields, FieldError{Field: "rate_unit", Code: "invalid_value"})
	}
	if i.BasePriceCLP <= 5000 {
		fields = append(fields, FieldError{Field: "base_price_clp", Code: "minimum"})
	}
	if strings.TrimSpace(i.Address) == "" {
		fields = append(fields, FieldError{Field: "address", Code: "required"})
	} else if len([]rune(i.Address)) > 500 {
		fields = append(fields, FieldError{Field: "address", Code: "max_length"})
	}
	if (i.RegionCode == "") != (i.CommuneCode == "") {
		fields = append(fields, FieldError{Field: "region_code", Code: "location_pair"}, FieldError{Field: "comuna_code", Code: "location_pair"})
	} else if i.RegionCode != "" && !validPublicLocationCodes(i.RegionCode, i.CommuneCode) {
		fields = append(fields, FieldError{Field: "region_code", Code: "invalid_value"}, FieldError{Field: "comuna_code", Code: "invalid_value"})
	}
	return fields
}

func validPublicLocationCodes(regionCode, communeCode string) bool {
	if len(regionCode) != 2 || len(communeCode) != 5 {
		return false
	}
	for _, value := range regionCode + communeCode {
		if value < '0' || value > '9' {
			return false
		}
	}
	return strings.HasPrefix(communeCode, regionCode)
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
	SetPublicationState(ctx context.Context, owner, id, state, correlationID string) (Draft, error)
	UpdatePublishedOwn(ctx context.Context, owner, id string, input PublishedContentInput) (Draft, error)
	DeleteOwn(ctx context.Context, owner, id, correlationID string) (Deletion, error)
}

// AdministrativeDeletionRepository keeps the administrator actor separate
// from the publication owner while reusing the domain's terminal-retirement
// and retention rules.
type AdministrativeDeletionRepository interface {
	DeleteAsAdministrator(context.Context, string, string, string, string, time.Time, string) (Deletion, error)
}

// ArchiveSectionRepository exports only drafts owned by the authenticated
// account, including their saved category/profile versions.
type ArchiveSectionRepository interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
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
	Required        bool     `json:"required,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	Options         []string `json:"options,omitempty"`
	Minimum         *float64 `json:"minimum,omitempty"`
	Maximum         *float64 `json:"maximum,omitempty"`
	Step            *float64 `json:"step,omitempty"`
	FilterCandidate bool     `json:"filter_candidate"`
}

func (p Profile) ValidateAttributes(values map[string]any) error {
	data, err := json.Marshal(values)
	if err != nil || len(data) > 16*1024 {
		return ErrInvalid
	}
	defs := make(map[string]AttributeDefinition, len(p.Attributes))
	for _, d := range p.Attributes {
		defs[d.Code] = d
	}
	for _, definition := range p.Attributes {
		if definition.Required {
			if _, exists := values[definition.Code]; !exists {
				return ErrInvalid
			}
		}
	}
	if len(values) == 0 {
		return nil
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

// AttributeFieldErrorsForArea returns only profile-backed field paths and
// stable local codes. It is suitable for API responses; it never returns the
// invalid value or a validator error string.
func (p Profile) AttributeFieldErrorsForArea(values map[string]any, areaM2 float64) []FieldError {
	data, err := json.Marshal(values)
	if err != nil || len(data) > 16*1024 {
		return []FieldError{{Field: "attributes", Code: "maximum_size"}}
	}
	defs := make(map[string]AttributeDefinition, len(p.Attributes))
	for _, definition := range p.Attributes {
		defs[definition.Code] = definition
	}
	fields := make([]FieldError, 0)
	for _, definition := range p.Attributes {
		if definition.Required {
			if _, exists := values[definition.Code]; !exists {
				fields = append(fields, FieldError{Field: "attributes." + definition.Code, Code: "required"})
			}
		}
	}
	codes := make([]string, 0, len(values))
	for code := range values {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		value := values[code]
		definition, exists := defs[code]
		if !exists {
			fields = append(fields, FieldError{Field: "attributes", Code: "unknown_field"})
			continue
		}
		if value == nil || definition.validate(value) != nil {
			fields = append(fields, FieldError{Field: "attributes." + code, Code: "invalid_value"})
			continue
		}
		if p.CategoryCode == "parcela_eventos" && (code == "superficie_exterior_util_m2" || code == "superficie_cubierta_util_m2") {
			if n, ok := numeric(value); !ok || n > areaM2 {
				fields = append(fields, FieldError{Field: "attributes." + code, Code: "exceeds_area"})
			}
		}
	}
	if p.CategoryCode == "quincho" {
		typ, hasType := values["tipo_parrilla"].(string)
		count, hasCount := numeric(values["parrillas_disponibles"])
		if hasType && typ == "sin_parrilla" && hasCount && count != 0 {
			fields = append(fields, FieldError{Field: "attributes.parrillas_disponibles", Code: "invalid_combination"})
		}
		if hasCount && count > 0 && (!hasType || typ == "sin_parrilla") {
			fields = append(fields, FieldError{Field: "attributes.tipo_parrilla", Code: "invalid_combination"})
		}
	}
	return fields
}

// ValidateAttributesForArea applies the persisted category schema together
// with the cross-field area invariant shared by draft and administrative edits.
func (p Profile) ValidateAttributesForArea(values map[string]any, areaM2 float64) error {
	if err := p.ValidateAttributes(values); err != nil {
		return err
	}
	if p.CategoryCode != "parcela_eventos" {
		return nil
	}
	for _, code := range []string{"superficie_exterior_util_m2", "superficie_cubierta_util_m2"} {
		if value, exists := values[code]; exists {
			n, ok := numeric(value)
			if !ok || n > areaM2 {
				return ErrInvalid
			}
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
