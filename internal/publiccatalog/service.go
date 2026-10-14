package publiccatalog

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"golang.org/x/text/unicode/norm"
)

const cursorVersion = 3

var areaPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,6})(?:\.[0-9]{1,2})?$`)
var attributeDecimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

type Service struct {
	repo       Repository
	key        []byte
	imageStore PublicImageStore
}

// New derives a purpose-specific cursor key from a stable local application
// master key. The same master key must be supplied to every replica/restart.
func New(repo Repository, masterKey []byte) (*Service, error) {
	if repo == nil || len(masterKey) < 32 {
		return nil, ErrInvalid
	}
	salt := []byte("EspaciGo public catalog cursor v1")
	extract := hmac.New(sha256.New, salt)
	_, _ = extract.Write(masterKey)
	prk := extract.Sum(nil)
	expand := hmac.New(sha256.New, prk)
	_, _ = expand.Write([]byte("public-catalog/cursor-signing\x01"))
	return &Service{repo: repo, key: expand.Sum(nil)}, nil
}

func NewWithImageStore(repo Repository, masterKey []byte, files PublicImageStore) (*Service, error) {
	s, err := New(repo, masterKey)
	if err != nil {
		return nil, err
	}
	if files == nil {
		return nil, ErrInvalid
	}
	s.imageStore = files
	return s, nil
}

func (s *Service) ImageContent(ctx context.Context, spaceID, imageID string) (PublicImage, []byte, error) {
	if !validUUID(spaceID) || !validUUID(imageID) || s.imageStore == nil {
		return PublicImage{}, nil, ErrNotFound
	}
	repo, ok := s.repo.(PublicImageRepository)
	if !ok {
		return PublicImage{}, nil, ErrNotFound
	}
	item, err := repo.PublicImage(ctx, strings.ToLower(spaceID), strings.ToLower(imageID))
	if err != nil {
		return PublicImage{}, nil, err
	}
	if item.MIME != "image/png" || item.Size < 1 || item.Size > 1<<20 || item.FileID == "" {
		return PublicImage{}, nil, ErrNotFound
	}
	blob, err := s.imageStore.Get(ctx, item.FileID)
	if err != nil || int64(len(blob)) != item.Size {
		return PublicImage{}, nil, ErrNotFound
	}
	digest := sha256.Sum256(blob)
	if hex.EncodeToString(digest[:]) != item.SHA256 || len(blob) < 8 || string(blob[:8]) != "\x89PNG\r\n\x1a\n" {
		return PublicImage{}, nil, ErrNotFound
	}
	return item, blob, nil
}

func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	return s.repo.Categories(ctx)
}

func (s *Service) AttributeFilters(ctx context.Context, category string) ([]AttributeFilter, error) {
	active, err := s.repo.HasActiveCategory(ctx, category)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrNotFound
	}
	return s.repo.AttributeFilters(ctx, category)
}

func (s *Service) Locations(ctx context.Context, query, region string, limit int) ([]Location, error) {
	query = normalizeQuery(query)
	if utf8.RuneCountInString(query) > 80 || (region != "" && !validRegionCode(region)) || limit < 1 || limit > 400 {
		return nil, ErrInvalid
	}
	if region != "" {
		active, err := s.repo.HasRegion(ctx, region)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, ErrInvalid
		}
	}
	return s.repo.Locations(ctx, query, region, limit)
}

func (s *Service) List(ctx context.Context, filters Filters) (Page, error) {
	filters.Query = normalizeQuery(filters.Query)
	if err := validate(&filters); err != nil {
		return Page{}, err
	}
	if filters.CategoryCode != "" {
		active, err := s.repo.HasActiveCategory(ctx, filters.CategoryCode)
		if err != nil {
			return Page{}, err
		}
		if !active {
			return Page{}, ErrInvalid
		}
	}
	if filters.RegionCode != "" {
		active, err := s.repo.HasRegion(ctx, filters.RegionCode)
		if err != nil {
			return Page{}, err
		}
		if !active {
			return Page{}, ErrInvalid
		}
	}
	if filters.CommuneCode != "" {
		active, err := s.repo.HasCommune(ctx, filters.RegionCode, filters.CommuneCode)
		if err != nil {
			return Page{}, err
		}
		if !active {
			return Page{}, ErrInvalid
		}
	}
	if len(filters.Attributes)+len(filters.TypedAttributes) > 0 {
		if filters.CategoryCode == "" {
			return Page{}, ErrInvalid
		}
		allowed, err := s.repo.AttributeFilters(ctx, filters.CategoryCode)
		if err != nil {
			return Page{}, err
		}
		valid := make(map[string]AttributeFilter, len(allowed))
		for _, filter := range allowed {
			if _, duplicate := valid[filter.Code]; duplicate {
				return Page{}, ErrInvalid
			}
			valid[filter.Code] = filter
		}
		for code := range filters.Attributes {
			definition, ok := valid[code]
			if !ok || definition.Type != "boolean" || (filters.AttributeVersion != 0 && filters.AttributeVersion != definition.ProfileVersion) {
				return Page{}, ErrInvalid
			}
			filters.AttributeVersion = definition.ProfileVersion
		}
		for code, predicate := range filters.TypedAttributes {
			definition, ok := valid[code]
			_, legacyPresent := filters.Attributes[code]
			if !ok || definition.Type == "boolean" || legacyPresent ||
				(filters.AttributeVersion != 0 && filters.AttributeVersion != definition.ProfileVersion) ||
				!validateAttributePredicate(definition, predicate) {
				return Page{}, ErrInvalid
			}
			predicate.Type = definition.Type
			sort.Strings(predicate.Values)
			filters.TypedAttributes[code] = predicate
			filters.AttributeVersion = definition.ProfileVersion
		}
		if len(filters.Attributes)+len(filters.TypedAttributes) > 16 {
			return Page{}, ErrInvalid
		}
	}
	fingerprint := filtersFingerprint(filters)
	var after *Cursor
	if filters.Cursor != "" {
		cursor, err := s.decodeCursor(filters.Cursor)
		if err != nil || cursor.Version != cursorVersion || cursor.Fingerprint != fingerprint || cursor.Sort != filters.Sort || cursor.PageSize != filters.PageSize || !validUUID(cursor.LastSpaceID) {
			return Page{}, ErrInvalid
		}
		after = &cursor
	}
	items, err := s.repo.List(ctx, filters, after, filters.PageSize+1)
	if err != nil {
		return Page{}, err
	}
	if filters.StartAt != nil && filters.EndAt != nil {
		for i := range items {
			units, unitErr := booking.PriceUnits(items[i].RateUnit, *filters.StartAt, *filters.EndAt, items[i].TimeZone)
			if unitErr != nil || units < 1 || items[i].BasePriceCLP > math.MaxInt64/units {
				items[i].EstimatedTotalCLP = nil
				continue
			}
			total := items[i].BasePriceCLP * units
			items[i].EstimatedTotalCLP = &total
		}
	}
	page := Page{Items: items}
	if len(page.Items) > filters.PageSize {
		page.Items = page.Items[:filters.PageSize]
		last := page.Items[len(page.Items)-1]
		cursor := Cursor{
			Version: cursorVersion, Fingerprint: fingerprint, Sort: filters.Sort, PageSize: filters.PageSize,
			LastCategoryOrder: last.CategoryOrder, LastTitleKey: last.TitleKey,
			LastUnitPriceCLP: last.BasePriceCLP, LastEstimatedTotalCLP: last.EstimatedTotalCLP,
			LastRelevanceRank: last.RelevanceRank, LastSpaceID: last.SpaceID,
		}
		page.NextCursor, err = s.encodeCursor(cursor)
		if err != nil {
			return Page{}, err
		}
	}
	return page, nil
}

func validateAttributePredicate(definition AttributeFilter, predicate AttributePredicate) bool {
	if predicate.Type != "" && predicate.Type != definition.Type {
		return false
	}
	if predicate.Value != nil {
		return definition.Type == "enum" && predicate.Minimum == nil && predicate.Maximum == nil && len(predicate.Values) == 0 && contains(definition.Options, *predicate.Value)
	}
	if len(predicate.Values) > 0 {
		if definition.Type != "enum_list" || predicate.Minimum != nil || predicate.Maximum != nil || len(predicate.Values) > len(definition.Options) {
			return false
		}
		seen := make(map[string]bool, len(predicate.Values))
		for _, value := range predicate.Values {
			if seen[value] || !contains(definition.Options, value) {
				return false
			}
			seen[value] = true
		}
		return true
	}
	if predicate.Minimum == nil && predicate.Maximum == nil || (definition.Type != "integer" && definition.Type != "number") {
		return false
	}
	var minimum, maximum *big.Rat
	var ok bool
	if predicate.Minimum != nil {
		minimum, ok = validateAttributeBound(definition, *predicate.Minimum)
		if !ok {
			return false
		}
	}
	if predicate.Maximum != nil {
		maximum, ok = validateAttributeBound(definition, *predicate.Maximum)
		if !ok {
			return false
		}
	}
	return minimum == nil || maximum == nil || minimum.Cmp(maximum) <= 0
}

func validateAttributeBound(definition AttributeFilter, value string) (*big.Rat, bool) {
	if len(value) > 64 || !attributeDecimalPattern.MatchString(value) {
		return nil, false
	}
	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, false
	}
	if definition.Type == "integer" && !parsed.IsInt() {
		return nil, false
	}
	if definition.Minimum != nil {
		limit, valid := profileNumber(*definition.Minimum)
		if !valid || parsed.Cmp(limit) < 0 {
			return nil, false
		}
	}
	if definition.Maximum != nil {
		limit, valid := profileNumber(*definition.Maximum)
		if !valid || parsed.Cmp(limit) > 0 {
			return nil, false
		}
	}
	if definition.Step != nil {
		step, valid := profileNumber(*definition.Step)
		if !valid || step.Sign() <= 0 {
			return nil, false
		}
		origin := new(big.Rat)
		if definition.Minimum != nil {
			origin, valid = profileNumber(*definition.Minimum)
			if !valid {
				return nil, false
			}
		}
		delta := new(big.Rat).Sub(parsed, origin)
		quotient := new(big.Rat).Quo(delta, step)
		if !quotient.IsInt() {
			return nil, false
		}
	}
	return parsed, true
}

func profileNumber(value json.Number) (*big.Rat, bool) {
	return new(big.Rat).SetString(value.String())
}

func contains(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}

func (s *Service) Detail(ctx context.Context, spaceID string) (Space, error) {
	if !validUUID(spaceID) {
		return Space{}, ErrNotFound
	}
	return s.repo.Detail(ctx, strings.ToLower(spaceID))
}

func validate(filters *Filters) error {
	if filters.PageSize == 0 && !filters.PageSizeSet {
		filters.PageSize = DefaultPageSize
	}
	if filters.PageSize < 1 || filters.PageSize > MaxPageSize {
		return ErrInvalid
	}
	if filters.Sort == "" {
		filters.Sort = "default"
	}
	if filters.Sort != "default" && filters.Sort != "title_asc" && filters.Sort != "title_desc" && filters.Sort != "unit_price_asc" && filters.Sort != "unit_price_desc" && filters.Sort != "relevance" && filters.Sort != "estimated_total_asc" && filters.Sort != "estimated_total_desc" {
		return ErrInvalid
	}
	if utf8.RuneCountInString(filters.Query) > 120 {
		return ErrInvalid
	}
	for _, r := range filters.Query {
		if unicode.IsControl(r) {
			return ErrInvalid
		}
	}
	if filters.MinAreaM2 != "" && (!areaPattern.MatchString(filters.MinAreaM2) || strings.TrimLeft(strings.ReplaceAll(filters.MinAreaM2, ".", ""), "0") == "") {
		return ErrInvalid
	}
	if filters.RegionCode != "" && !validRegionCode(filters.RegionCode) {
		return ErrInvalid
	}
	if filters.CommuneCode != "" && (!validCommuneCode(filters.CommuneCode) || filters.RegionCode == "") {
		return ErrInvalid
	}
	if len(filters.Attributes) > 16 || len(filters.TypedAttributes) > 16 || len(filters.Attributes)+len(filters.TypedAttributes) > 16 {
		return ErrInvalid
	}
	for key := range filters.Attributes {
		if !validAttributeCode(key) {
			return ErrInvalid
		}
	}
	if (filters.StartAt == nil) != (filters.EndAt == nil) {
		return ErrInvalid
	}
	if filters.StartAt != nil {
		if !filters.EndAt.After(*filters.StartAt) || filters.EndAt.Sub(*filters.StartAt) > 366*24*time.Hour {
			return ErrInvalid
		}
	}
	usesTotal := filters.MinTotalCLP != nil || filters.MaxTotalCLP != nil || filters.Sort == "estimated_total_asc" || filters.Sort == "estimated_total_desc"
	if usesTotal && filters.StartAt == nil {
		return ErrInvalid
	}
	for _, amount := range []*int64{filters.MinTotalCLP, filters.MaxTotalCLP} {
		if amount != nil && *amount < 0 {
			return ErrInvalid
		}
	}
	if filters.MinTotalCLP != nil && filters.MaxTotalCLP != nil && *filters.MinTotalCLP > *filters.MaxTotalCLP {
		return ErrInvalid
	}
	if filters.CategoryCode != "" && (len(filters.CategoryCode) > 80 || strings.TrimSpace(filters.CategoryCode) != filters.CategoryCode) {
		return ErrInvalid
	}
	if filters.RateUnit != "" && filters.RateUnit != "hora" && filters.RateUnit != "dia" && filters.RateUnit != "mes" {
		return ErrInvalid
	}
	if filters.Sort == "unit_price_asc" || filters.Sort == "unit_price_desc" || filters.MinUnitPriceCLP != nil || filters.MaxUnitPriceCLP != nil {
		if filters.RateUnit == "" {
			return ErrInvalid
		}
	}
	for _, price := range []*int64{filters.MinUnitPriceCLP, filters.MaxUnitPriceCLP} {
		if price != nil && *price < 0 {
			return ErrInvalid
		}
	}
	if filters.MinUnitPriceCLP != nil && filters.MaxUnitPriceCLP != nil && *filters.MinUnitPriceCLP > *filters.MaxUnitPriceCLP {
		return ErrInvalid
	}
	if filters.Cursor != "" && len(filters.Cursor) > 4096 {
		return ErrInvalid
	}
	return nil
}

func normalizeQuery(query string) string {
	return norm.NFC.String(strings.Join(strings.Fields(query), " "))
}

func validRegionCode(code string) bool  { return len(code) == 2 && digitsOnly(code) }
func validCommuneCode(code string) bool { return len(code) == 5 && digitsOnly(code) }
func validAttributeCode(code string) bool {
	if len(code) < 1 || len(code) > 60 || code[0] < 'a' || code[0] > 'z' {
		return false
	}
	for _, r := range code[1:] {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}
func digitsOnly(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func filtersFingerprint(filters Filters) string {
	canonical := struct {
		Category         string                        `json:"category"`
		Unit             string                        `json:"unit"`
		Min              *int64                        `json:"min,omitempty"`
		Max              *int64                        `json:"max,omitempty"`
		Sort             string                        `json:"sort"`
		Size             int                           `json:"size"`
		Query            string                        `json:"query"`
		Region           string                        `json:"region"`
		Commune          string                        `json:"commune"`
		MinArea          string                        `json:"min_area"`
		Attributes       map[string]bool               `json:"attributes"`
		TypedAttributes  map[string]AttributePredicate `json:"typed_attributes"`
		AttributeVersion int                           `json:"attribute_version"`
		StartAt          *time.Time                    `json:"start_at"`
		EndAt            *time.Time                    `json:"end_at"`
		MinTotal         *int64                        `json:"min_total"`
		MaxTotal         *int64                        `json:"max_total"`
	}{filters.CategoryCode, filters.RateUnit, filters.MinUnitPriceCLP, filters.MaxUnitPriceCLP, filters.Sort, filters.PageSize,
		filters.Query, filters.RegionCode, filters.CommuneCode, filters.MinAreaM2, filters.Attributes, filters.TypedAttributes, filters.AttributeVersion,
		filters.StartAt, filters.EndAt, filters.MinTotalCLP, filters.MaxTotalCLP}
	raw, _ := json.Marshal(canonical)
	digest := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (s *Service) encodeCursor(cursor Cursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(raw)
	return "v3." + base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) decodeCursor(value string) (Cursor, error) {
	var cursor Cursor
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != "v3" || len(value) > 4096 {
		return cursor, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return cursor, ErrInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return cursor, ErrInvalid
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(raw, &cursor) != nil {
		return Cursor{}, ErrInvalid
	}
	return cursor, nil
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
