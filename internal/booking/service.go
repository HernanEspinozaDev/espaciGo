package booking

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
)

type Service struct {
	repo                      Repository
	ids                       identity.CredentialGenerator
	payment                   LocalPaymentAdapter
	now                       func() time.Time
	quoteTTL, payTTL, hostTTL time.Duration
	catalogCursorKey          [32]byte
}

func NewService(repo Repository, ids identity.CredentialGenerator, now func() time.Time, payment LocalPaymentAdapter) (*Service, error) {
	return NewServiceWithTTLs(repo, ids, now, payment, 15*time.Minute, 15*time.Minute, 24*time.Hour)
}
func NewServiceWithTTLs(repo Repository, ids identity.CredentialGenerator, now func() time.Time, payment LocalPaymentAdapter, quoteTTL, payTTL, hostTTL time.Duration) (*Service, error) {
	if repo == nil || ids == nil || now == nil || payment == nil || quoteTTL < time.Minute || quoteTTL > time.Hour || payTTL < time.Minute || payTTL > time.Hour || hostTTL < time.Hour || hostTTL > 72*time.Hour {
		return nil, ErrInvalid
	}
	s := &Service{repo: repo, ids: ids, payment: payment, now: now, quoteTTL: quoteTTL, payTTL: payTTL, hostTTL: hostTTL}
	if _, err := rand.Read(s.catalogCursorKey[:]); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) Fixture(ctx context.Context, actor string) (Fixture, error) {
	if !uuid.MatchString(actor) {
		return Fixture{}, ErrNotFound
	}
	return s.repo.Fixture(ctx, actor)
}
func (s *Service) Catalog(ctx context.Context, actor string, filter CatalogFilter) ([]CatalogItem, error) {
	if !uuid.MatchString(actor) || (filter.CategoryCode != "" && !validCategory(filter.CategoryCode)) {
		return nil, ErrInvalid
	}
	if (filter.StartAt == nil) != (filter.EndAt == nil) {
		return nil, ErrInvalid
	}
	if (filter.MinTotalCLP != nil || filter.MaxTotalCLP != nil) && filter.StartAt == nil {
		return nil, ErrInvalid
	}
	if filter.MinTotalCLP != nil && *filter.MinTotalCLP < 0 || filter.MaxTotalCLP != nil && *filter.MaxTotalCLP < 0 ||
		filter.MinTotalCLP != nil && filter.MaxTotalCLP != nil && *filter.MinTotalCLP > *filter.MaxTotalCLP {
		return nil, ErrInvalid
	}
	nearby := filter.Latitude != nil || filter.Longitude != nil || filter.RadiusKM != nil
	if nearby {
		if filter.Latitude == nil || filter.Longitude == nil || filter.RadiusKM == nil ||
			math.IsNaN(*filter.Latitude) || math.IsInf(*filter.Latitude, 0) || *filter.Latitude < -90 || *filter.Latitude > 90 ||
			math.IsNaN(*filter.Longitude) || math.IsInf(*filter.Longitude, 0) || *filter.Longitude < -180 || *filter.Longitude > 180 {
			return nil, ErrInvalid
		}
		switch *filter.RadiusKM {
		case 1, 3, 5, 10, 25:
		default:
			return nil, ErrInvalid
		}
	}
	if len(filter.Attributes) > 0 {
		if filter.CategoryCode == "" || filter.ProfileVersion < 1 {
			return nil, ErrInvalid
		}
		profile, err := s.repo.CatalogProfile(ctx, filter.CategoryCode, filter.ProfileVersion)
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalid
		}
		if err != nil {
			return nil, err
		}
		if profile.CategoryCode != filter.CategoryCode || profile.SchemaVersion != filter.ProfileVersion || profile.ValidateAttributeFilters(filter.Attributes) != nil {
			return nil, ErrInvalid
		}
	} else if filter.ProfileVersion != 0 {
		return nil, ErrInvalid
	}
	if filter.StartAt != nil {
		now := s.now().UTC()
		if !filter.EndAt.After(*filter.StartAt) || !filter.StartAt.After(now) {
			return nil, ErrInvalid
		}
	}
	// Resolve expired payment/host holds before computing availability. Catalog
	// reads must not leave an expired reservation blocking a space until some
	// unrelated reservation endpoint happens to trigger the expiry mechanism.
	if err := s.repo.Expire(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	items, err := s.repo.Catalog(ctx, actor, filter)
	if err != nil {
		return nil, err
	}
	filtered := items
	if filter.StartAt != nil {
		filtered = make([]CatalogItem, 0, len(items))
		for _, item := range items {
			units, e := PriceUnits(item.RateUnit, *filter.StartAt, *filter.EndAt, item.TimeZone)
			if e != nil || units < 1 || item.Price > math.MaxInt64/units {
				return nil, ErrInvalid
			}
			total := item.Price * units
			item.EstimatedTotal = &total
			if filter.MinTotalCLP != nil && total < *filter.MinTotalCLP || filter.MaxTotalCLP != nil && total > *filter.MaxTotalCLP {
				continue
			}
			filtered = append(filtered, item)
		}
	}
	if nearby {
		for i := range filtered {
			distance := math.Round(filtered[i].DistanceMeters/100) / 10
			filtered[i].DistanceKM = &distance
			filtered[i].DistanceKind = "direct"
		}
		sort.Slice(filtered, func(i, j int) bool { return compareCatalog(filtered[i], filtered[j], "distance") < 0 })
		return filtered, nil
	}
	if filter.StartAt != nil {
		sort.Slice(filtered, func(i, j int) bool { return compareCatalog(filtered[i], filtered[j], "price") < 0 })
	}
	return filtered, nil
}

type catalogCursor struct {
	Version  int        `json:"v"`
	Actor    string     `json:"a"`
	Filter   string     `json:"f"`
	Mode     string     `json:"m"`
	PageSize int        `json:"n"`
	Last     catalogKey `json:"l"`
}

type catalogKey struct {
	SpaceID        string  `json:"i"`
	CategoryOrder  int     `json:"c,omitempty"`
	Title          string  `json:"t,omitempty"`
	DistanceMeters float64 `json:"d,omitempty"`
	EstimatedTotal *int64  `json:"p,omitempty"`
}

func (s *Service) CatalogPage(ctx context.Context, actor string, filter CatalogFilter, pageSize int, cursor string) (CatalogPage, error) {
	if pageSize == 0 {
		pageSize = 5
	}
	if pageSize < 1 || pageSize > 25 {
		return CatalogPage{}, ErrInvalid
	}
	mode := catalogOrderMode(filter)
	fingerprint, err := catalogFilterFingerprint(filter, pageSize, mode)
	if err != nil {
		return CatalogPage{}, ErrInvalid
	}
	var decoded *catalogCursor
	if cursor != "" {
		c, decodeErr := s.decodeCatalogCursor(cursor)
		if decodeErr != nil || c.Version != 1 || c.Actor != actor || c.Filter != fingerprint || c.Mode != mode || c.PageSize != pageSize {
			return CatalogPage{}, ErrInvalid
		}
		decoded = &c
	}
	items, err := s.Catalog(ctx, actor, filter)
	if err != nil {
		return CatalogPage{}, err
	}
	start := 0
	if decoded != nil {
		anchorFound := false
		for i := range items {
			if compareCatalogKey(items[i], decoded.Last, mode) == 0 {
				start, anchorFound = i+1, true
				break
			}
		}
		if !anchorFound {
			start = sort.Search(len(items), func(i int) bool { return compareCatalogKey(items[i], decoded.Last, mode) > 0 })
		}
	}
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	page := CatalogPage{Items: append(make([]CatalogItem, 0, end-start), items[start:end]...)}
	if end < len(items) {
		page.NextCursor, err = s.encodeCatalogCursor(catalogCursor{Version: 1, Actor: actor, Filter: fingerprint, Mode: mode, PageSize: pageSize, Last: catalogKeyFor(items[end-1])})
		if err != nil {
			return CatalogPage{}, err
		}
	}
	return page, nil
}

func catalogOrderMode(f CatalogFilter) string {
	if f.Latitude != nil {
		return "distance"
	}
	if f.StartAt != nil {
		return "price"
	}
	return "category"
}
func catalogFilterFingerprint(f CatalogFilter, n int, mode string) (string, error) {
	attributes := f.Attributes
	if attributes == nil {
		attributes = map[string]any{}
	}
	canonical := struct {
		Category   string         `json:"category"`
		Start      string         `json:"start,omitempty"`
		End        string         `json:"end,omitempty"`
		Min        *int64         `json:"min,omitempty"`
		Max        *int64         `json:"max,omitempty"`
		Profile    int            `json:"profile"`
		Attributes map[string]any `json:"attributes"`
		Lat        *float64       `json:"lat,omitempty"`
		Lon        *float64       `json:"lon,omitempty"`
		Radius     *int           `json:"radius,omitempty"`
		PageSize   int            `json:"page_size"`
		Mode       string         `json:"mode"`
	}{Category: f.CategoryCode, Profile: f.ProfileVersion, Attributes: attributes, Min: f.MinTotalCLP, Max: f.MaxTotalCLP, Lat: f.Latitude, Lon: f.Longitude, Radius: f.RadiusKM, PageSize: n, Mode: mode}
	if f.StartAt != nil {
		canonical.Start = f.StartAt.UTC().Format(time.RFC3339Nano)
		canonical.End = f.EndAt.UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
func (s *Service) encodeCatalogCursor(c catalogCursor) (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.catalogCursorKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, raw, []byte("catalog-cursor-v1"))
	return "v1." + base64.RawURLEncoding.EncodeToString(sealed), nil
}
func (s *Service) decodeCatalogCursor(value string) (catalogCursor, error) {
	var c catalogCursor
	if len(value) > 8192 || len(value) < 4 || value[:3] != "v1." {
		return c, fmt.Errorf("invalid cursor")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(value[3:])
	if err != nil {
		return c, err
	}
	block, err := aes.NewCipher(s.catalogCursorKey[:])
	if err != nil {
		return c, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(sealed) < gcm.NonceSize() {
		return c, fmt.Errorf("invalid cursor")
	}
	nonce := sealed[:gcm.NonceSize()]
	raw, err := gcm.Open(nil, nonce, sealed[gcm.NonceSize():], []byte("catalog-cursor-v1"))
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c)
	return c, err
}
func compareCatalog(a, b CatalogItem, mode string) int {
	cmp := func(x, y string) int {
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
		return 0
	}
	switch mode {
	case "distance":
		if a.DistanceMeters < b.DistanceMeters {
			return -1
		}
		if a.DistanceMeters > b.DistanceMeters {
			return 1
		}
		if a.EstimatedTotal != nil && b.EstimatedTotal != nil && *a.EstimatedTotal != *b.EstimatedTotal {
			if *a.EstimatedTotal < *b.EstimatedTotal {
				return -1
			}
			return 1
		}
		return cmp(a.SpaceID, b.SpaceID)
	case "price":
		if a.EstimatedTotal != nil && b.EstimatedTotal != nil && *a.EstimatedTotal != *b.EstimatedTotal {
			if *a.EstimatedTotal < *b.EstimatedTotal {
				return -1
			}
			return 1
		}
		return cmp(a.SpaceID, b.SpaceID)
	default:
		if a.CategoryOrder < b.CategoryOrder {
			return -1
		}
		if a.CategoryOrder > b.CategoryOrder {
			return 1
		}
		if v := cmp(a.Title, b.Title); v != 0 {
			return v
		}
		return cmp(a.SpaceID, b.SpaceID)
	}
}

func sortCatalogByGeo(items []CatalogItem) {
	sort.Slice(items, func(i, j int) bool { return compareCatalog(items[i], items[j], "distance") < 0 })
}

func catalogKeyFor(item CatalogItem) catalogKey {
	return catalogKey{SpaceID: item.SpaceID, CategoryOrder: item.CategoryOrder, Title: item.Title, DistanceMeters: item.DistanceMeters, EstimatedTotal: item.EstimatedTotal}
}
func compareCatalogKey(a CatalogItem, b catalogKey, mode string) int {
	return compareCatalog(a, CatalogItem{SpaceID: b.SpaceID, CategoryOrder: b.CategoryOrder, Title: b.Title, DistanceMeters: b.DistanceMeters, EstimatedTotal: b.EstimatedTotal}, mode)
}

func (s *Service) CatalogDetail(ctx context.Context, actor, spaceID string) (CatalogItem, error) {
	if !uuid.MatchString(actor) || !uuid.MatchString(spaceID) {
		return CatalogItem{}, ErrNotFound
	}
	return s.repo.CatalogDetail(ctx, actor, spaceID)
}

func (s *Service) AvailableIntervals(ctx context.Context, actor, spaceID string, in AvailabilityOptionsInput) (AvailabilityOptions, error) {
	if !uuid.MatchString(actor) || !uuid.MatchString(spaceID) {
		return AvailabilityOptions{}, ErrNotFound
	}
	item, err := s.repo.CatalogDetail(ctx, actor, spaceID)
	if err != nil {
		return AvailabilityOptions{}, err
	}
	now := s.now().UTC()
	candidates, err := availabilityCandidates(item.RateUnit, in.Date, in.Duration, item.TimeZone, now)
	if err != nil {
		return AvailabilityOptions{}, err
	}
	// Apply the shared expiry transition mechanism before treating active
	// occupancy rows as blocking. This is a read-only query; it creates no quote.
	if err = s.repo.Expire(ctx, now); err != nil {
		return AvailabilityOptions{}, err
	}
	available, err := s.repo.AvailableIntervals(ctx, actor, spaceID, candidates)
	if err != nil {
		return AvailabilityOptions{}, err
	}
	if len(available) != len(candidates) {
		return AvailabilityOptions{}, ErrNotFound
	}
	result := AvailabilityOptions{SpaceID: item.SpaceID, TimeZone: item.TimeZone, RateUnit: item.RateUnit, Items: make([]AvailableInterval, 0, len(candidates))}
	checkedAt := s.now().UTC()
	for i, candidate := range candidates {
		if available[i] && candidate.StartAt.After(checkedAt) {
			result.Items = append(result.Items, candidate)
		}
	}
	return result, nil
}

func (s *Service) Quote(ctx context.Context, renter string, in QuoteInput) (Quote, error) {
	if !uuid.MatchString(in.SpaceID) {
		return Quote{}, ErrInvalid
	}
	start, err := strictTime(in.StartAt)
	if err != nil {
		return Quote{}, ErrInvalid
	}
	end, err := strictTime(in.EndAt)
	if err != nil || !end.After(start) {
		return Quote{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Quote{}, err
	}
	now := s.now().UTC()
	if !start.After(now) {
		return Quote{}, ErrInvalid
	}
	return s.repo.Quote(ctx, renter, in.SpaceID, id, start, end, s.now, s.quoteTTL)
}

func validCategory(code string) bool {
	switch code {
	case "oficina", "sala_multiproposito", "bodega", "estacionamiento", "local_flexible", "stand", "quincho", "parcela_eventos":
		return true
	default:
		return false
	}
}
func (s *Service) Request(ctx context.Context, renter string, in RequestInput, key string) (Reservation, error) {
	if !uuid.MatchString(renter) || !uuid.MatchString(in.QuoteID) || strings.TrimSpace(key) == "" || len(key) > 200 {
		return Reservation{}, ErrInvalid
	}
	id, err := s.ids.ID()
	if err != nil {
		return Reservation{}, err
	}
	occupancyID, err := s.ids.ID()
	if err != nil {
		return Reservation{}, err
	}
	body, _ := json.Marshal(in)
	fingerprint := sha256.Sum256(body)
	return s.repo.Create(ctx, renter, in.QuoteID, key, fingerprint[:], id, occupancyID, s.payTTL, s.now)
}
func (s *Service) Get(ctx context.Context, actor, id string) (Detail, error) {
	if !uuid.MatchString(actor) || !uuid.MatchString(id) {
		return Detail{}, ErrNotFound
	}
	now := s.now().UTC()
	if err := s.repo.Expire(ctx, now); err != nil {
		return Detail{}, err
	}
	return s.repo.Get(ctx, actor, id)
}
func (s *Service) List(ctx context.Context, actor string) ([]Reservation, error) {
	if !uuid.MatchString(actor) {
		return nil, ErrNotFound
	}
	now := s.now().UTC()
	if err := s.repo.Expire(ctx, now); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, actor)
}
func (s *Service) Pay(ctx context.Context, renter, id, outcome, key string) (Reservation, error) {
	if !uuid.MatchString(renter) || !uuid.MatchString(id) || strings.TrimSpace(key) == "" || len(key) > 200 {
		return Reservation{}, ErrInvalid
	}
	resolved, err := s.payment.Process(ctx, outcome)
	if err != nil {
		return Reservation{}, ErrInvalid
	}
	now := s.now().UTC()
	if err := s.repo.Expire(ctx, now); err != nil {
		return Reservation{}, err
	}
	v, err := s.repo.Pay(ctx, renter, id, resolved, key, now, now.Add(s.hostTTL))
	if err != nil {
		return v, err
	}
	if resolved == "sin_respuesta" {
		return v, ErrSimulatedNoResponse
	}
	return v, nil
}
func (s *Service) Decide(ctx context.Context, host, id, decision string) (Reservation, error) {
	if !uuid.MatchString(host) || !uuid.MatchString(id) || (decision != "aprobar" && decision != "rechazar") {
		return Reservation{}, ErrInvalid
	}
	now := s.now().UTC()
	if err := s.repo.Expire(ctx, now); err != nil {
		return Reservation{}, err
	}
	return s.repo.Decide(ctx, host, id, decision, now)
}
func (s *Service) Cancel(ctx context.Context, renter, id string) (Reservation, error) {
	if !uuid.MatchString(renter) || !uuid.MatchString(id) {
		return Reservation{}, ErrNotFound
	}
	now := s.now().UTC()
	if err := s.repo.Expire(ctx, now); err != nil {
		return Reservation{}, err
	}
	return s.repo.Cancel(ctx, renter, id, now)
}
func strictTime(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, ErrInvalid
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || t.IsZero() {
		return time.Time{}, ErrInvalid
	}
	return t.UTC(), nil
}

// PriceUnits delegates to M05's shared timezone-aware tariff calculator.
func PriceUnits(unit string, start, end time.Time, zone string) (int64, error) {
	units, err := pricing.BilledUnits(unit, start, end, zone)
	if err != nil {
		return 0, ErrInvalid
	}
	return units, nil
}
