package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/calendar"
)

// ArchiveSectionRepository exports owner-scoped rate revisions and private
// simulation snapshots without consulting or changing availability.
type ArchiveSectionRepository interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
}

var (
	ErrInvalid  = errors.New("pricing: invalid request")
	ErrNotFound = errors.New("pricing: private resource not found")
	ErrConflict = errors.New("pricing: interval unavailable")
)

type Rate struct {
	SpaceID   string    `json:"space_id"`
	Version   int64     `json:"version"`
	Unit      string    `json:"rate_unit"`
	Amount    int64     `json:"base_price"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

type Simulation struct {
	ID          string    `json:"id"`
	SpaceID     string    `json:"space_id"`
	RateVersion int64     `json:"rate_version"`
	RateUnit    string    `json:"rate_unit"`
	BasePrice   int64     `json:"base_price"`
	BilledUnits int64     `json:"billed_units"`
	Currency    string    `json:"currency"`
	Subtotal    int64     `json:"base_subtotal"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	TimeZone    string    `json:"time_zone"`
	Private     bool      `json:"private"`
	CreatedAt   time.Time `json:"created_at"`
}

type RateInput struct {
	Unit   string `json:"rate_unit"`
	Amount int64  `json:"base_price"`
}
type SimulationInput struct {
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}

type Repository interface {
	CurrentRate(context.Context, string, string) (Rate, error)
	RateHistory(context.Context, string, string) ([]Rate, error)
	UpdateRate(context.Context, string, string, RateInput) (Rate, error)
	CreateSimulation(context.Context, string, string, string, string, Rate, time.Time, time.Time, int64, int64) (Simulation, error)
	GetSimulation(context.Context, string, string, string) (Simulation, error)
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func validateRate(in RateInput) error {
	if (in.Unit != "hora" && in.Unit != "dia" && in.Unit != "mes") || in.Amount <= 5000 {
		return ErrInvalid
	}
	return nil
}

func parseWindow(in SimulationInput) (time.Time, time.Time, error) {
	if !strings.HasSuffix(in.StartAt, "Z") || !strings.HasSuffix(in.EndAt, "Z") {
		return time.Time{}, time.Time{}, ErrInvalid
	}
	start, err := time.Parse(time.RFC3339Nano, in.StartAt)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalid
	}
	end, err := time.Parse(time.RFC3339Nano, in.EndAt)
	if err != nil || !end.After(start) {
		return time.Time{}, time.Time{}, ErrInvalid
	}
	return start.UTC(), end.UTC(), nil
}

func units(unit string, startUTC, endUTC time.Time, zone string) (int64, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return 0, ErrInvalid
	}
	switch unit {
	case "hora":
		seconds := endUTC.Sub(startUTC).Seconds()
		n := int64(math.Ceil(seconds / 3600))
		if n < 1 {
			n = 1
		}
		return n, nil
	case "dia":
		s := startUTC.In(loc)
		e := endUTC.In(loc)
		endBoundary, exists := calendar.FirstInstantOfDate(e.Year(), e.Month(), e.Day(), loc)
		endYear, endMonth, endDay := e.Date()
		if exists && e.Equal(endBoundary) {
			e = e.Add(-time.Nanosecond)
			endYear, endMonth, endDay = e.In(loc).Date()
		}
		startYear, startMonth, startDay := s.Date()
		startOrdinal := calendar.DateOrdinal(startYear, startMonth, startDay)
		endOrdinal := calendar.DateOrdinal(endYear, endMonth, endDay)
		return endOrdinal - startOrdinal + 1, nil
	case "mes":
		s := startUTC.In(loc)
		e := endUTC.In(loc)
		months := int64((e.Year()-s.Year())*12 + int(e.Month()-s.Month()))
		if months < 1 {
			return 1, nil
		}
		anniversary, exists := addMonthsClamped(s, months)
		if !exists {
			return 0, ErrInvalid
		}
		if e.After(anniversary) {
			months++
		}
		return months, nil
	default:
		return 0, ErrInvalid
	}
}

func addMonthsClamped(v time.Time, months int64) (time.Time, bool) {
	return calendar.AddMonthsClamped(v, months)
}
