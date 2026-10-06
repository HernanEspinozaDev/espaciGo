package booking

import (
	"sort"
	"time"
	_ "time/tzdata"
)

const availabilityHorizonDays = 90

// availabilityCandidates builds one date's candidates in the fixture's IANA
// zone. Hourly intervals are elapsed durations; daily/monthly intervals use
// local calendar boundaries. Repeated local times yield both UTC instants.
func availabilityCandidates(unit, dateText string, duration int, zone string, now time.Time) ([]AvailableInterval, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, ErrInvalid
	}
	date, err := time.ParseInLocation("2006-01-02", dateText, loc)
	if err != nil || date.Format("2006-01-02") != dateText {
		return nil, ErrInvalid
	}
	today := now.In(loc)
	startToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	startSelected := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	if startSelected.Before(startToday) || date.After(startToday.AddDate(0, 0, availabilityHorizonDays)) {
		return nil, ErrInvalid
	}
	now = now.UTC()
	items := make([]AvailableInterval, 0)
	switch unit {
	case "hora":
		if duration != 1 && duration != 2 && duration != 4 {
			return nil, ErrInvalid
		}
		for minute := 0; minute < 24*60; minute += 30 {
			for _, start := range localWallInstants(date, minute/60, minute%60, loc) {
				end := start.Add(time.Duration(duration) * time.Hour)
				if start.After(now) {
					items = append(items, AvailableInterval{StartAt: start.UTC(), EndAt: end.UTC()})
				}
			}
		}
	case "dia":
		if duration < 1 || duration > 3 {
			return nil, ErrInvalid
		}
		starts := localWallInstants(date, 0, 0, loc)
		endDate := date.AddDate(0, 0, duration)
		ends := localWallInstants(endDate, 0, 0, loc)
		if len(starts) > 0 && len(ends) > 0 && starts[0].After(now) {
			items = append(items, AvailableInterval{StartAt: starts[0].UTC(), EndAt: ends[0].UTC()})
		}
	case "mes":
		if duration != 1 {
			return nil, ErrInvalid
		}
		starts := localWallInstants(date, 0, 0, loc)
		endDate := addCalendarMonthsClamped(date, 1)
		ends := localWallInstants(endDate, 0, 0, loc)
		if len(starts) > 0 && len(ends) > 0 && starts[0].After(now) {
			items = append(items, AvailableInterval{StartAt: starts[0].UTC(), EndAt: ends[0].UTC()})
		}
	default:
		return nil, ErrInvalid
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartAt.Before(items[j].StartAt) })
	return items, nil
}

// localWallInstants resolves a wall-clock time without relying on time.Date's
// implicit choice during a DST fold. It returns zero values for a DST gap and
// both UTC instants for a repeated wall time.
func localWallInstants(date time.Time, hour, minute int, loc *time.Location) []time.Time {
	year, month, day := date.Date()
	wallUTC := time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
	offsets := make(map[int]struct{})
	for probe := wallUTC.Add(-36 * time.Hour); !probe.After(wallUTC.Add(36 * time.Hour)); probe = probe.Add(15 * time.Minute) {
		_, offset := probe.In(loc).Zone()
		offsets[offset] = struct{}{}
	}
	instants := make([]time.Time, 0, 2)
	for offset := range offsets {
		candidate := wallUTC.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(loc)
		ly, lm, ld := local.Date()
		lh, lmin, sec := local.Clock()
		if ly == year && lm == month && ld == day && lh == hour && lmin == minute && sec == 0 {
			instants = append(instants, candidate)
		}
	}
	sort.Slice(instants, func(i, j int) bool { return instants[i].Before(instants[j]) })
	return instants
}

func addCalendarMonthsClamped(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	monthIndex := int(year)*12 + int(month-1) + months
	targetYear, targetMonth := monthIndex/12, time.Month(monthIndex%12+1)
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetYear, targetMonth, day, 0, 0, 0, 0, time.UTC)
}
