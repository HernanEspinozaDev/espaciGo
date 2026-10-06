package booking

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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
}

func NewService(repo Repository, ids identity.CredentialGenerator, now func() time.Time, payment LocalPaymentAdapter) (*Service, error) {
	return NewServiceWithTTLs(repo, ids, now, payment, 15*time.Minute, 15*time.Minute, 24*time.Hour)
}
func NewServiceWithTTLs(repo Repository, ids identity.CredentialGenerator, now func() time.Time, payment LocalPaymentAdapter, quoteTTL, payTTL, hostTTL time.Duration) (*Service, error) {
	if repo == nil || ids == nil || now == nil || payment == nil || quoteTTL < time.Minute || quoteTTL > time.Hour || payTTL < time.Minute || payTTL > time.Hour || hostTTL < time.Hour || hostTTL > 72*time.Hour {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, ids: ids, payment: payment, now: now, quoteTTL: quoteTTL, payTTL: payTTL, hostTTL: hostTTL}, nil
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
	if filter.StartAt != nil {
		now := s.now().UTC()
		if !filter.EndAt.After(*filter.StartAt) || !filter.StartAt.After(now) {
			return nil, ErrInvalid
		}
	}
	return s.repo.Catalog(ctx, actor, filter)
}
func (s *Service) CatalogDetail(ctx context.Context, actor, spaceID string) (CatalogItem, error) {
	if !uuid.MatchString(actor) || !uuid.MatchString(spaceID) {
		return CatalogItem{}, ErrNotFound
	}
	return s.repo.CatalogDetail(ctx, actor, spaceID)
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
