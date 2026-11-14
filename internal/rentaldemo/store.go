package rentaldemo

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/calendar"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
)

var (
	ErrInvalid   = errors.New("rentaldemo: invalid")
	ErrNotFound  = errors.New("rentaldemo: not found")
	ErrForbidden = errors.New("rentaldemo: forbidden")
	ErrConflict  = errors.New("rentaldemo: conflict")
)

const (
	ActorHost    = "host"
	ActorRenter  = "renter"
	ActorRenter2 = "renter2"
)
const (
	SpaceIDHour  = "a1000000-0000-4000-8000-000000000001"
	SpaceIDDay   = "a1000000-0000-4000-8000-000000000002"
	SpaceIDMonth = "a1000000-0000-4000-8000-000000000003"
)
const (
	HostID    = "a2000000-0000-4000-8000-000000000001"
	RenterID  = "a2000000-0000-4000-8000-000000000002"
	Renter2ID = "a2000000-0000-4000-8000-000000000003"
)

type Interval struct {
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}
type Space struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	State        string `json:"state"`
	TimeZone     string `json:"time_zone"`
	RateUnit     string `json:"rate_unit"`
	UnitPriceCLP string `json:"unit_price_clp"`
	Revision     string `json:"revision"`
	OpenHour     int    `json:"open_hour"`
	CloseHour    int    `json:"close_hour"`
}
type Day struct {
	Date      string     `json:"date"`
	Status    string     `json:"status"`
	Intervals []Interval `json:"intervals"`
}
type Month struct {
	SpaceID      string  `json:"space_id"`
	Title        string  `json:"title"`
	TimeZone     string  `json:"time_zone"`
	RateUnit     string  `json:"rate_unit"`
	UnitPriceCLP string  `json:"unit_price_clp"`
	Revision     string  `json:"revision,omitempty"`
	Month        string  `json:"month"`
	Today        string  `json:"today"`
	Days         []Day   `json:"days"`
	Blocks       []Block `json:"blocks,omitempty"`
}
type Block struct {
	ID      string `json:"id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}
type QuoteRequest struct {
	SpaceID string `json:"space_id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}
type Quote struct {
	ID           string `json:"id"`
	SpaceID      string `json:"space_id"`
	SpaceTitle   string `json:"space_title"`
	StartAt      string `json:"start_at"`
	EndAt        string `json:"end_at"`
	TimeZone     string `json:"time_zone"`
	RateUnit     string `json:"rate_unit"`
	UnitPriceCLP string `json:"unit_price_clp"`
	Units        int    `json:"units"`
	SubtotalCLP  string `json:"subtotal_clp"`
	Currency     string `json:"currency"`
	ExpiresAt    string `json:"expires_at"`
	revision     int64  `json:"-"`
	owner, actor string
}
type Confirmation struct {
	Quote
	QuoteID       string `json:"quote_id"`
	IntentID      string `json:"intent_id"`
	Status        string `json:"status"`
	PaymentStatus string `json:"payment_status"`
}
type listing struct {
	Space
	owner     string
	amount    int64
	recurring bool
}
type occupancy struct {
	space      string
	start, end time.Time
}
type Store struct {
	mu       sync.RWMutex
	now      func() time.Time
	ids      interface{ ID() (string, error) }
	spaces   map[string]*listing
	blocks   map[string][]Block
	quotes   map[string]Quote
	occupied []occupancy
	attempts map[string]Confirmation
	intents  map[string]string
}

func NewStore(clock func() time.Time) (*Store, error) {
	if clock == nil {
		return nil, ErrInvalid
	}
	s := &Store{now: clock, ids: credentials.Generator{}, spaces: map[string]*listing{}, blocks: map[string][]Block{}, quotes: map[string]Quote{}, attempts: map[string]Confirmation{}, intents: map[string]string{}}
	for _, v := range []struct {
		id, title, unit string
		price           int64
	}{{SpaceIDHour, "Sala horaria", "hora", 12000}, {SpaceIDDay, "Espacio diario", "dia", 50000}, {SpaceIDMonth, "Espacio mensual", "mes", 700000}} {
		s.spaces[v.id] = &listing{Space: Space{ID: v.id, Title: v.title, State: "activa", TimeZone: "America/Santiago", RateUnit: v.unit, UnitPriceCLP: strconv.FormatInt(v.price, 10), Revision: "1", OpenHour: 8, CloseHour: 20}, owner: HostID, amount: v.price, recurring: true}
	}
	return s, nil
}
func (s *Store) check(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrInvalid
	}
	return nil
}
func actorID(a string) (string, bool) {
	switch a {
	case ActorHost:
		return HostID, true
	case ActorRenter:
		return RenterID, true
	case ActorRenter2:
		return Renter2ID, true
	}
	return "", false
}
func (s *Store) Space(ctx context.Context, actor, id string, owner bool) (Space, error) {
	if e := s.check(ctx); e != nil {
		return Space{}, e
	}
	if _, ok := actorID(actor); !ok {
		return Space{}, ErrForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	x := s.spaces[id]
	if x == nil || (!owner && x.State != "activa") {
		return Space{}, ErrNotFound
	}
	if owner && actor != ActorHost {
		return Space{}, ErrForbidden
	}
	return x.Space, nil
}
func (s *Store) UpdateSpace(ctx context.Context, actor, id, expected, state, price string, open, close int) (Space, error) {
	if e := s.check(ctx); e != nil {
		return Space{}, e
	}
	if actor != ActorHost {
		return Space{}, ErrForbidden
	}
	p, e := strconv.ParseInt(price, 10, 64)
	if e != nil || p <= 0 || open < 0 || close > 24 || open >= close || (state != "activa" && state != "borrador" && state != "oculta") {
		return Space{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.spaces[id]
	if x == nil {
		return Space{}, ErrNotFound
	}
	if expected != x.Revision {
		return Space{}, ErrConflict
	}
	x.State = state
	x.UnitPriceCLP = price
	x.amount = p
	x.OpenHour = open
	x.CloseHour = close
	x.Revision = next(x.Revision)
	return x.Space, nil
}
func next(v string) string { n, _ := strconv.ParseInt(v, 10, 64); return strconv.FormatInt(n+1, 10) }
func (s *Store) Block(ctx context.Context, actor, id, expected, startText, endText string) (Block, error) {
	if e := s.check(ctx); e != nil {
		return Block{}, e
	}
	if actor != ActorHost {
		return Block{}, ErrForbidden
	}
	a, e := time.Parse(time.RFC3339Nano, startText)
	if e != nil {
		return Block{}, ErrInvalid
	}
	b, e := time.Parse(time.RFC3339Nano, endText)
	if e != nil || !b.After(a) {
		return Block{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.spaces[id]
	if x == nil {
		return Block{}, ErrNotFound
	}
	if expected != x.Revision {
		return Block{}, ErrConflict
	}
	v, e := s.ids.ID()
	if e != nil {
		return Block{}, ErrInvalid
	}
	z := Block{v, a.UTC().Format(time.RFC3339Nano), b.UTC().Format(time.RFC3339Nano)}
	s.blocks[id] = append(s.blocks[id], z)
	x.Revision = next(x.Revision)
	return z, nil
}
func (s *Store) Unblock(ctx context.Context, actor, id, blockID, expected string) error {
	if e := s.check(ctx); e != nil {
		return e
	}
	if actor != ActorHost {
		return ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.spaces[id]
	if x == nil {
		return ErrNotFound
	}
	if expected != x.Revision {
		return ErrConflict
	}
	bs := s.blocks[id]
	for i, b := range bs {
		if b.ID == blockID {
			s.blocks[id] = append(bs[:i], bs[i+1:]...)
			x.Revision = next(x.Revision)
			return nil
		}
	}
	return ErrNotFound
}
func (s *Store) CreateQuote(ctx context.Context, actor string, in QuoteRequest) (Quote, error) {
	if e := s.check(ctx); e != nil {
		return Quote{}, e
	}
	aid, ok := actorID(actor)
	if !ok || actor == ActorHost {
		return Quote{}, ErrForbidden
	}
	a, e := time.Parse(time.RFC3339Nano, in.StartAt)
	if e != nil {
		return Quote{}, ErrInvalid
	}
	b, e := time.Parse(time.RFC3339Nano, in.EndAt)
	if e != nil || !b.After(a) {
		return Quote{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.spaces[in.SpaceID]
	if x == nil || x.State != "activa" {
		return Quote{}, ErrNotFound
	}
	q, e := s.quoteLocked(x, aid, a.UTC(), b.UTC())
	if e == nil {
		s.quotes[q.ID] = q
	}
	return q, e
}
func (s *Store) quoteLocked(x *listing, actor string, a, b time.Time) (Quote, error) {
	now := s.now()
	return s.quoteAtLocked(x, actor, a, b, now)
}

func (s *Store) quoteAtLocked(x *listing, actor string, a, b, now time.Time) (Quote, error) {
	if !a.After(now) {
		return Quote{}, ErrInvalid
	}
	loc, e := time.LoadLocation(x.TimeZone)
	if e != nil {
		return Quote{}, ErrInvalid
	}
	local := a.In(loc)
	today := now.In(loc)
	delta := calendar.DateOrdinal(local.Year(), local.Month(), local.Day()) - calendar.DateOrdinal(today.Year(), today.Month(), today.Day())
	if delta < 0 || delta > 90 {
		return Quote{}, ErrInvalid
	}
	u, e := pricing.BilledUnits(x.RateUnit, a, b, x.TimeZone)
	if e != nil {
		return Quote{}, ErrInvalid
	}
	valid := x.RateUnit == "hora" && (u == 1 || u == 2 || u == 4) || x.RateUnit == "dia" && u >= 1 && u <= 3 || x.RateUnit == "mes" && u == 1
	if !valid || x.amount > math.MaxInt64/u {
		return Quote{}, ErrInvalid
	}
	day := local.Day()
	found := false
	for _, candidate := range candidates(x, local.Year(), local.Month(), day, int(u), loc) {
		if parse(candidate.StartAt).Equal(a) && parse(candidate.EndAt).Equal(b) {
			found = true
			break
		}
	}
	if !found {
		return Quote{}, ErrInvalid
	}
	if !s.available(x.Space.ID, a, b) {
		return Quote{}, ErrConflict
	}
	id, e := s.ids.ID()
	if e != nil {
		return Quote{}, ErrInvalid
	}
	return Quote{ID: id, SpaceID: x.ID, SpaceTitle: x.Title, StartAt: a.Format(time.RFC3339Nano), EndAt: b.Format(time.RFC3339Nano), TimeZone: x.TimeZone, RateUnit: x.RateUnit, UnitPriceCLP: x.UnitPriceCLP, Units: int(u), SubtotalCLP: strconv.FormatInt(int64(u)*x.amount, 10), Currency: "CLP", ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano), revision: mustInt(x.Revision), owner: x.owner, actor: actor}, nil
}
func mustInt(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }
func (s *Store) available(id string, a, b time.Time) bool {
	for _, z := range s.blocks[id] {
		x, _ := time.Parse(time.RFC3339Nano, z.StartAt)
		y, _ := time.Parse(time.RFC3339Nano, z.EndAt)
		if a.Before(y) && x.Before(b) {
			return false
		}
	}
	for _, o := range s.occupied {
		if o.space == id && a.Before(o.end) && o.start.Before(b) {
			return false
		}
	}
	return true
}
func (s *Store) Confirm(ctx context.Context, actor, quoteID string, approved bool, intent string) (Confirmation, error) {
	if e := s.check(ctx); e != nil {
		return Confirmation{}, e
	}
	aid, ok := actorID(actor)
	if !ok || actor == ActorHost {
		return Confirmation{}, ErrForbidden
	}
	if !approved || !validUUID(intent) {
		return Confirmation{}, ErrInvalid
	}
	intent = strings.ToLower(intent)
	s.mu.Lock()
	defer s.mu.Unlock()
	key := aid + "/" + intent
	if old, ok := s.intents[key]; ok {
		if old != quoteID {
			return Confirmation{}, ErrConflict
		}
		return s.attempts[key], nil
	}
	q, ok := s.quotes[quoteID]
	if !ok {
		qv, found := s.quoteByID(quoteID)
		if !found {
			return Confirmation{}, ErrNotFound
		}
		q = qv
	}
	if q.actor != aid {
		return Confirmation{}, ErrForbidden
	}
	now := s.now()
	if !now.Before(parse(q.ExpiresAt)) || !parse(q.StartAt).After(now) {
		return Confirmation{}, ErrConflict
	}
	x := s.spaces[q.SpaceID]
	if x == nil || x.State != "activa" || x.Revision != strconv.FormatInt(q.revision, 10) || x.amount != mustInt(q.UnitPriceCLP) || !s.available(q.SpaceID, parse(q.StartAt), parse(q.EndAt)) {
		return Confirmation{}, ErrConflict
	}
	c := Confirmation{Quote: q, QuoteID: q.ID, IntentID: intent, Status: "confirmed_demo", PaymentStatus: "paid_simulated"}
	s.occupied = append(s.occupied, occupancy{q.SpaceID, parse(q.StartAt), parse(q.EndAt)})
	x.Revision = next(x.Revision)
	s.intents[key] = quoteID
	s.attempts[key] = c
	return c, nil
}
func (s *Store) quoteByID(id string) (Quote, bool) { q, ok := s.quotes[id]; return q, ok }
func validUUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
func (s *Store) Attempt(ctx context.Context, actor, intent string) (Confirmation, error) {
	if e := s.check(ctx); e != nil {
		return Confirmation{}, e
	}
	aid, ok := actorID(actor)
	if !ok {
		return Confirmation{}, ErrForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.attempts[aid+"/"+strings.ToLower(intent)]
	if !ok {
		return Confirmation{}, ErrNotFound
	}
	return c, nil
}
func parse(v string) time.Time { x, _ := time.Parse(time.RFC3339Nano, v); return x }
func localDayBounds(y int, m time.Month, day int, loc *time.Location) (time.Time, time.Time, bool) {
	start, ok := calendar.FirstInstantOfDate(y, m, day, loc)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	ny, nm, nd := y, m, day+1
	if nd > time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		nd = 1
		nm++
		if nm > 12 {
			nm = 1
			ny++
		}
	}
	end, ok := calendar.FirstInstantOfDate(ny, nm, nd, loc)
	return start, end, ok
}
func candidates(x *listing, y int, m time.Month, day, duration int, loc *time.Location) []Interval {
	start, end, ok := localDayBounds(y, m, day, loc)
	if !ok {
		return []Interval{}
	}
	out := []Interval{}
	if x.RateUnit == "hora" {
		for a := start; a.Before(end); a = a.Add(time.Minute) {
			h, min, _ := a.In(loc).Clock()
			if min%30 != 0 || h < x.OpenHour || h >= x.CloseHour {
				continue
			}
			b := a.Add(time.Duration(duration) * time.Hour)
			if b.After(end) {
				continue
			}
			valid := true
			for p := a; p.Before(b); p = p.Add(30 * time.Minute) {
				ph, pm, _ := p.In(loc).Clock()
				if pm%30 != 0 || ph < x.OpenHour || ph >= x.CloseHour {
					valid = false
					break
				}
			}
			if valid {
				out = append(out, Interval{a.UTC().Format(time.RFC3339Nano), b.UTC().Format(time.RFC3339Nano)})
			}
		}
	} else {
		a := start
		var b time.Time
		if x.RateUnit == "dia" {
			target := time.Date(y, m, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, duration)
			b, ok = calendar.FirstInstantOfDate(target.Year(), target.Month(), target.Day(), loc)
		} else {
			target, valid := calendar.AddMonthsClamped(time.Date(y, m, day, 0, 0, 0, 0, time.UTC), int64(duration))
			if !valid {
				return out
			}
			b, ok = calendar.FirstInstantOfDate(target.Year(), target.Month(), target.Day(), loc)
			if !ok {
				return out
			}
		}
		if ok && b.After(a) {
			out = append(out, Interval{a.UTC().Format(time.RFC3339Nano), b.UTC().Format(time.RFC3339Nano)})
		}
	}
	return out
}
func (s *Store) Month(ctx context.Context, actor, id, month string, duration int, owner bool) (Month, error) {
	if e := s.check(ctx); e != nil {
		return Month{}, e
	}
	if owner && actor != ActorHost {
		return Month{}, ErrForbidden
	}
	d, e := time.Parse("2006-01", month)
	if e != nil || d.Format("2006-01") != month {
		return Month{}, ErrInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	x := s.spaces[id]
	if x == nil || (!owner && x.State != "activa") {
		return Month{}, ErrNotFound
	}
	if !((x.RateUnit == "hora" && (duration == 1 || duration == 2 || duration == 4)) || (x.RateUnit == "dia" && duration >= 1 && duration <= 3) || (x.RateUnit == "mes" && duration == 1)) {
		return Month{}, ErrInvalid
	}
	loc, e := time.LoadLocation(x.TimeZone)
	if e != nil {
		return Month{}, ErrInvalid
	}
	now := s.now().In(loc)
	m := Month{SpaceID: id, Title: x.Title, TimeZone: x.TimeZone, RateUnit: x.RateUnit, UnitPriceCLP: x.UnitPriceCLP, Month: month, Today: now.Format("2006-01-02")}
	if owner {
		m.Revision = x.Revision
		m.Blocks = append([]Block{}, s.blocks[id]...)
	}
	for day := 1; day <= time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day(); day++ {
		date := time.Date(d.Year(), d.Month(), day, 0, 0, 0, 0, time.UTC)
		delta := calendar.DateOrdinal(date.Year(), date.Month(), date.Day()) - calendar.DateOrdinal(now.Year(), now.Month(), now.Day())
		status := "unavailable"
		intervals := []Interval{}
		if delta < 0 || delta > 90 {
			status = "outside_horizon"
		} else {
			all := candidates(x, d.Year(), d.Month(), day, duration, loc)
			for _, v := range all {
				a := parse(v.StartAt)
				b := parse(v.EndAt)
				if a.After(now) && s.available(id, a, b) {
					intervals = append(intervals, v)
				}
			}
			if len(intervals) > 0 {
				status = "available"
				if len(intervals) < len(all) || (x.RateUnit == "hora" && (x.OpenHour != 0 || x.CloseHour != 24)) {
					status = "partial"
				}
			}
		}
		m.Days = append(m.Days, Day{Date: date.Format("2006-01-02"), Status: status, Intervals: intervals})
	}
	return m, nil
}
