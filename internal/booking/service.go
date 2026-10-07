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
	refund                    LocalRefundAdapter
	notices                   LocalNoticeSender
	now                       func() time.Time
	quoteTTL, payTTL, hostTTL time.Duration
	catalogCursorKey          [32]byte
}

// SetLocalRefundAdapter and SetLocalNoticeSender are used only by the local
// prototype wiring. No production payment or durable-notification adapter is
// provided by this service.
func (s *Service) SetLocalRefundAdapter(adapter LocalRefundAdapter) { s.refund = adapter }
func (s *Service) SetLocalNoticeSender(sender LocalNoticeSender)    { s.notices = sender }

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
			if item.RateUnit == "hora" {
				if hoursRepo, ok := s.repo.(WeeklyHoursRepository); ok {
					hours, scheduleErr := hoursRepo.WeeklyHoursForSpace(ctx, item.SpaceID)
					if scheduleErr != nil {
						return nil, scheduleErr
					}
					if hours.Enabled && !IntervalFitsWeeklyHours(*filter.StartAt, *filter.EndAt, item.TimeZone, hours) {
						continue
					}
				}
			}
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
	if item.RateUnit == "hora" {
		if hoursRepo, ok := s.repo.(WeeklyHoursRepository); ok {
			hours, scheduleErr := hoursRepo.WeeklyHoursForSpace(ctx, spaceID)
			if scheduleErr != nil {
				return AvailabilityOptions{}, scheduleErr
			}
			if hours.Enabled {
				keptCandidates := make([]AvailableInterval, 0, len(candidates))
				keptAvailable := make([]bool, 0, len(candidates))
				for i, candidate := range candidates {
					if IntervalFitsWeeklyHours(candidate.StartAt, candidate.EndAt, item.TimeZone, hours) {
						keptCandidates = append(keptCandidates, candidate)
						keptAvailable = append(keptAvailable, available[i])
					}
				}
				candidates, available = keptCandidates, keptAvailable
			}
		}
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

func (s *Service) WeeklyHours(ctx context.Context, host, spaceID string) (WeeklyHours, error) {
	if !uuid.MatchString(host) || !uuid.MatchString(spaceID) {
		return WeeklyHours{}, ErrNotFound
	}
	repo, ok := s.repo.(WeeklyHoursRepository)
	if !ok {
		return WeeklyHours{}, ErrNotFound
	}
	return repo.WeeklyHoursForHost(ctx, host, spaceID)
}

func (s *Service) SaveWeeklyHours(ctx context.Context, host, spaceID string, value WeeklyHours) (WeeklyHours, error) {
	if !uuid.MatchString(host) || !uuid.MatchString(spaceID) {
		return WeeklyHours{}, ErrNotFound
	}
	if ValidateWeeklyHours(value) != nil {
		return WeeklyHours{}, ErrInvalid
	}
	repo, ok := s.repo.(WeeklyHoursRepository)
	if !ok {
		return WeeklyHours{}, ErrNotFound
	}
	value.Days = normalizeWeeklyDays(value.Days)
	return repo.SaveWeeklyHours(ctx, host, spaceID, value)
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
	if !uuid.MatchString(renter) || !uuid.MatchString(id) || strings.TrimSpace(key) == "" || len(key) > 200 || (outcome != "exito" && outcome != "rechazo" && outcome != "sin_respuesta") {
		return Reservation{}, ErrInvalid
	}
	repo, ok := s.repo.(PaymentLifecycleRepository)
	if !ok {
		return Reservation{}, ErrInvalid
	}
	// Process authenticated callbacks before an expiry sweep. A callback
	// received before the deadline stays timely even if reconciliation runs
	// after that deadline.
	if err := s.ReconcilePendingPayments(ctx); err != nil {
		return Reservation{}, err
	}
	if err := s.repo.Expire(ctx, s.now().UTC()); err != nil {
		return Reservation{}, err
	}
	operationID, err := s.ids.ID()
	if err != nil {
		return Reservation{}, err
	}
	fingerprint := sha256.Sum256([]byte("local-payment-v1\n" + outcome))
	operation, created, err := repo.BeginPayment(ctx, renter, id, outcome, key, fingerprint[:], operationID, s.now)
	if err != nil {
		return Reservation{}, err
	}
	if operation.State == "vencida" {
		return Reservation{}, ErrConflict
	}
	if operation.State == "aplicada" {
		current, readErr := s.Get(ctx, renter, id)
		if readErr != nil {
			return Reservation{}, readErr
		}
		return current.Reservation, nil
	}
	var event *PaymentEvent
	if operation.State == "pendiente" {
		if created {
			event, err = s.payment.StartPayment(ctx, operation.ID, operation.Requested)
			if errors.Is(err, ErrSimulatedNoResponse) {
				current, readErr := s.Get(ctx, renter, id)
				if readErr != nil {
					return Reservation{}, readErr
				}
				return current.Reservation, ErrSimulatedNoResponse
			}
			if err != nil {
				// The durable intent may already have reached the adapter. Treat any
				// ambiguous adapter failure as unresolved; never call Start again.
				current, readErr := s.Get(ctx, renter, id)
				if readErr != nil {
					return Reservation{}, readErr
				}
				return current.Reservation, ErrSimulatedNoResponse
			}
		} else {
			// First ask whether the idempotent fake recorded a result. If the
			// process died before starting it, StartPayment is safe because the
			// fake itself persists one result per operation ID.
			event, err = s.payment.LookupPayment(ctx, operation)
			if err != nil {
				return Reservation{}, err
			}
			if event == nil && operation.State == "pendiente" && operation.ReservationState == "pendiente_de_pago" && operation.PayExpiresAt.After(s.now().UTC()) {
				event, err = s.payment.StartPayment(ctx, operation.ID, operation.Requested)
				if errors.Is(err, ErrSimulatedNoResponse) {
					current, readErr := s.Get(ctx, renter, id)
					if readErr != nil {
						return Reservation{}, readErr
					}
					return current.Reservation, ErrSimulatedNoResponse
				}
				if err != nil {
					return Reservation{}, err
				}
			}
		}
	}
	if event != nil {
		if _, err = s.IngestPaymentEvent(ctx, *event); err != nil {
			return Reservation{}, err
		}
	}
	if err = s.ReconcilePendingPayments(ctx); err != nil {
		return Reservation{}, err
	}
	current, err := s.Get(ctx, renter, id)
	if err != nil {
		return Reservation{}, err
	}
	if current.State == "pendiente_de_pago" {
		return current.Reservation, ErrSimulatedNoResponse
	}
	if current.State != "pagada" && current.State != "cancelada_por_pago" {
		return current.Reservation, ErrConflict
	}
	return current.Reservation, nil
}

// IngestPaymentEvent authenticates before persisting. The inbox row is
// immutable and survives a process restart; the reconciler applies it later.
func (s *Service) IngestPaymentEvent(ctx context.Context, event PaymentEvent) (PaymentEventReceipt, error) {
	if !uuid.MatchString(event.OperationID) || strings.TrimSpace(event.EventID) == "" || len(event.EventID) > 200 ||
		(event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return PaymentEventReceipt{}, ErrInvalid
	}
	if !s.payment.VerifyPaymentEvent(event) {
		return PaymentEventReceipt{}, ErrUnauthenticatedPaymentEvent
	}
	repo, ok := s.repo.(PaymentLifecycleRepository)
	if !ok {
		return PaymentEventReceipt{}, ErrInvalid
	}
	fingerprint := sha256.Sum256([]byte(event.EventID + "\n" + event.OperationID + "\n" + event.Outcome))
	reused, err := repo.RecordPaymentEvent(ctx, event, fingerprint[:], s.now().UTC())
	if err != nil {
		return PaymentEventReceipt{}, err
	}
	return PaymentEventReceipt{Accepted: true, Reused: reused}, nil
}

// ReconcilePendingPayments first applies authenticated inbox records, then
// queries the durable fake. If an intent was persisted before a crash but the
// fake was never started, it starts it using the fake's operation-scoped
// idempotency key. Finally it expires remaining deadlines.
func (s *Service) ReconcilePendingPayments(ctx context.Context) error {
	repo, ok := s.repo.(PaymentLifecycleRepository)
	if !ok {
		return ErrInvalid
	}
	for pass := 0; pass < 2; pass++ {
		eventIDs, err := repo.PendingPaymentEventIDs(ctx, 100)
		if err != nil {
			return err
		}
		for _, eventID := range eventIDs {
			_, applyErr := repo.ApplyPaymentEvent(ctx, eventID, s.now, s.hostTTL)
			if applyErr != nil && !errors.Is(applyErr, ErrConflict) {
				return applyErr
			}
		}
		if pass == 0 {
			operations, err := repo.PendingPayments(ctx, 100)
			if err != nil {
				return err
			}
			for _, operation := range operations {
				if operation.ReservationState != "pendiente_de_pago" || !operation.PayExpiresAt.After(s.now().UTC()) {
					continue
				}
				event, lookupErr := s.payment.LookupPayment(ctx, operation)
				if lookupErr != nil {
					return lookupErr
				}
				if event == nil {
					event, lookupErr = s.payment.StartPayment(ctx, operation.ID, operation.Requested)
					if errors.Is(lookupErr, ErrSimulatedNoResponse) {
						// It may have recorded a result while losing its response. A
						// status query recovers it without starting another charge.
						event, lookupErr = s.payment.LookupPayment(ctx, operation)
					}
					if lookupErr != nil {
						return lookupErr
					}
					if event == nil {
						continue
					}
				}
				if _, ingestErr := s.IngestPaymentEvent(ctx, *event); ingestErr != nil {
					return ingestErr
				}
			}
		}
	}
	return s.repo.Expire(ctx, s.now().UTC())
}

// RunPaymentReconciler retries durable work at startup and periodically. A
// failure leaves records pending for the next pass; it does not log payloads.
func (s *Service) RunPaymentReconciler(ctx context.Context, interval time.Duration) {
	if interval < time.Second {
		interval = 5 * time.Second
	}
	_ = s.ReconcilePendingPayments(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.ReconcilePendingPayments(ctx)
		}
	}
}
func (s *Service) Decide(ctx context.Context, host, id, decision, reason string) (Reservation, error) {
	reason = strings.TrimSpace(reason)
	if !uuid.MatchString(host) || !uuid.MatchString(id) || (decision != "aprobar" && decision != "rechazar") || decision == "rechazar" && reason == "" {
		return Reservation{}, ErrInvalid
	}
	if err := s.repo.Expire(ctx, s.now().UTC()); err != nil {
		return Reservation{}, err
	}
	return s.repo.Decide(ctx, host, id, decision, reason, s.now)
}
func (s *Service) CancellationPreview(ctx context.Context, renter, id string) (CancellationPreview, error) {
	if !uuid.MatchString(renter) || !uuid.MatchString(id) {
		return CancellationPreview{}, ErrNotFound
	}
	now := s.now().UTC()
	if err := s.repo.Expire(ctx, now); err != nil {
		return CancellationPreview{}, err
	}
	return s.repo.CancellationPreview(ctx, renter, id, now)
}

func (s *Service) Cancel(ctx context.Context, renter, id, key, reason string) (CancellationResult, error) {
	reason = strings.TrimSpace(reason)
	if !uuid.MatchString(renter) || !uuid.MatchString(id) || strings.TrimSpace(key) == "" || len(key) > 200 {
		return CancellationResult{}, ErrInvalid
	}
	fingerprint := sha256.Sum256(jsonBytes(struct {
		Reason string `json:"reason"`
	}{reason}))
	result, err := s.repo.Cancel(ctx, renter, id, key, reason, fingerprint[:], s.now)
	if err != nil {
		return CancellationResult{}, err
	}
	if result.Replayed {
		result.NoticeStatus = "no_reintentado_por_idempotencia"
		return result, nil
	}
	if result.RefundState == "pendiente" {
		result.NoticeStatus = s.sendNotice(ctx, renter, id, "Reserva cancelada · devolución simulada pendiente", "La reserva fue cancelada. La devolución simulada del subtotal está pendiente; no hay movimiento de dinero real.")
	} else {
		result.NoticeStatus = s.sendNotice(ctx, renter, id, "Reserva cancelada · sin devolución", "La reserva local fue cancelada antes del pago. No se generó devolución.")
	}
	return result, nil
}

func (s *Service) Refund(ctx context.Context, renter, id, operationID, outcome string) (RefundResult, error) {
	if !uuid.MatchString(renter) || !uuid.MatchString(id) || !uuid.MatchString(operationID) || (outcome != "exito" && outcome != "fallo" && outcome != "sin_respuesta") {
		return RefundResult{}, ErrInvalid
	}
	if s.refund == nil {
		return RefundResult{}, ErrConflict
	}
	operation, err := s.repo.RefundOperation(ctx, renter, id)
	if err != nil {
		return RefundResult{}, err
	}
	if operation.OperationID != operationID {
		return RefundResult{}, ErrConflict
	}
	if operation.State == "completada" {
		operation.Reused = true
		operation.NoticeStatus = "no_reintentado_por_idempotencia"
		return operation, nil
	}
	result, err := s.refund.ProcessRefund(ctx, operation.OperationID, operationID, outcome)
	if err != nil {
		return RefundResult{}, ErrInvalid
	}
	updated, err := s.repo.RecordRefund(ctx, renter, id, result, s.now().UTC())
	if err != nil {
		return RefundResult{}, err
	}
	if updated.Reused {
		updated.NoticeStatus = "no_reintentado_por_idempotencia"
		return updated, nil
	}
	if updated.State == "completada" {
		updated.NoticeStatus = s.sendNotice(ctx, renter, id, "Devolución simulada completada", "La devolución fake del 100 % del importe confirmado quedó completada. No hubo movimiento de dinero real.")
	} else {
		updated.NoticeStatus = s.sendNotice(ctx, renter, id, "Devolución simulada pendiente", "La reserva sigue cancelada y la devolución fake está pendiente. Reintenta la misma operación local; no hubo movimiento de dinero real.")
	}
	if result == "sin_respuesta_simulada" {
		return updated, ErrSimulatedRefundNoResponse
	}
	return updated, nil
}

func (s *Service) sendNotice(ctx context.Context, actor, reservationID, subject, body string) string {
	if s.notices == nil {
		return "no_disponible_no_durable"
	}
	recipients, err := s.repo.NoticeRecipients(ctx, actor, reservationID)
	if err != nil || len(recipients) != 2 {
		return "no_enviado_no_durable"
	}
	for _, recipient := range recipients {
		if err = s.notices.SendLocalBookingNotice(ctx, recipient, subject, SafetyBanner+"\r\n\r\n"+body); err != nil {
			return "no_enviado_no_durable"
		}
	}
	return "mailpit_local_no_durable"
}

func jsonBytes(value any) []byte { raw, _ := json.Marshal(value); return raw }
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
