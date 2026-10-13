package publiccatalog

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

type Service struct {
	repo Repository
	key  []byte
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

func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	return s.repo.Categories(ctx)
}

func (s *Service) List(ctx context.Context, filters Filters) (Page, error) {
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
	fingerprint := filtersFingerprint(filters)
	var after *Cursor
	if filters.Cursor != "" {
		cursor, err := s.decodeCursor(filters.Cursor)
		if err != nil || cursor.Version != 1 || cursor.Fingerprint != fingerprint || cursor.Sort != filters.Sort || cursor.PageSize != filters.PageSize || !validUUID(cursor.LastSpaceID) {
			return Page{}, ErrInvalid
		}
		after = &cursor
	}
	items, err := s.repo.List(ctx, filters, after, filters.PageSize+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(page.Items) > filters.PageSize {
		page.Items = page.Items[:filters.PageSize]
		last := page.Items[len(page.Items)-1]
		cursor := Cursor{
			Version: 1, Fingerprint: fingerprint, Sort: filters.Sort, PageSize: filters.PageSize,
			LastCategoryOrder: last.CategoryOrder, LastTitleKey: last.TitleKey,
			LastUnitPriceCLP: last.BasePriceCLP, LastSpaceID: last.SpaceID,
		}
		page.NextCursor, err = s.encodeCursor(cursor)
		if err != nil {
			return Page{}, err
		}
	}
	return page, nil
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
	if filters.Sort != "default" && filters.Sort != "unit_price_asc" && filters.Sort != "unit_price_desc" {
		return ErrInvalid
	}
	if filters.CategoryCode != "" && (len(filters.CategoryCode) > 80 || strings.TrimSpace(filters.CategoryCode) != filters.CategoryCode) {
		return ErrInvalid
	}
	if filters.RateUnit != "" && filters.RateUnit != "hora" && filters.RateUnit != "dia" && filters.RateUnit != "mes" {
		return ErrInvalid
	}
	if filters.Sort != "default" || filters.MinUnitPriceCLP != nil || filters.MaxUnitPriceCLP != nil {
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

func filtersFingerprint(filters Filters) string {
	canonical := struct {
		Category string `json:"category"`
		Unit     string `json:"unit"`
		Min      *int64 `json:"min,omitempty"`
		Max      *int64 `json:"max,omitempty"`
		Sort     string `json:"sort"`
		Size     int    `json:"size"`
	}{filters.CategoryCode, filters.RateUnit, filters.MinUnitPriceCLP, filters.MaxUnitPriceCLP, filters.Sort, filters.PageSize}
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
	return "v1." + base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) decodeCursor(value string) (Cursor, error) {
	var cursor Cursor
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != "v1" || len(value) > 4096 {
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
