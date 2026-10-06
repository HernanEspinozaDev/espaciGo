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
	// Treat dateText as a calendar label. Parsing it in the fixture zone can
	// normalize midnight into the previous date when that midnight is skipped.
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil || date.Format("2006-01-02") != dateText {
		return nil, ErrInvalid
	}
	today := now.In(loc)
	todayLabel := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(todayLabel) || date.After(todayLabel.AddDate(0, 0, availabilityHorizonDays)) {
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
				if start.After(now) && end.After(start) {
					items = append(items, AvailableInterval{StartAt: start.UTC(), EndAt: end.UTC()})
				}
			}
		}
	case "dia":
		if duration < 1 || duration > 3 {
			return nil, ErrInvalid
		}
		start, startOK := firstInstantOfLocalDate(date, loc)
		endDate := date.AddDate(0, 0, duration)
		end, endOK := firstInstantOfLocalDate(endDate, loc)
		if startOK && endOK && end.After(start) && start.After(now) {
			items = append(items, AvailableInterval{StartAt: start.UTC(), EndAt: end.UTC()})
		}
	case "mes":
		if duration != 1 {
			return nil, ErrInvalid
		}
		start, startOK := firstInstantOfLocalDate(date, loc)
		endDate := addCalendarMonthsClamped(date, 1)
		end, endOK := firstInstantOfLocalDate(endDate, loc)
		if startOK && endOK && end.After(start) && start.After(now) {
			items = append(items, AvailableInterval{StartAt: start.UTC(), EndAt: end.UTC()})
		}
	default:
		return nil, ErrInvalid
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartAt.Before(items[j].StartAt) })
	return items, nil
}

// firstInstantOfLocalDate resolves the first instant belonging to the given
// calendar label. If local midnight is skipped, the post-transition offset
// maps it to the first valid time of that date.
func firstInstantOfLocalDate(date time.Time, loc *time.Location) (time.Time, bool) {
	year, month, day := date.Date()
	wallUTC := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	first := time.Time{}
	for offset := range offsetsNear(wallUTC, loc) {
		candidate := wallUTC.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(loc)
		ly, lm, ld := local.Date()
		if ly == year && lm == month && ld == day && (first.IsZero() || candidate.Before(first)) {
			first = candidate
		}
	}
	return first, !first.IsZero()
}

// localWallInstants resolves a wall-clock time without relying on time.Date's
// implicit choice during a DST fold. It returns zero values for a DST gap and
// both UTC instants for a repeated wall time.
func localWallInstants(date time.Time, hour, minute int, loc *time.Location) []time.Time {
	year, month, day := date.Date()
	wallUTC := time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
	offsets := offsetsNear(wallUTC, loc)
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

func offsetsNear(wallUTC time.Time, loc *time.Location) map[int]struct{} {
	offsets := make(map[int]struct{})
	for probe := wallUTC.Add(-36 * time.Hour); !probe.After(wallUTC.Add(36 * time.Hour)); probe = probe.Add(15 * time.Minute) {
		_, offset := probe.In(loc).Zone()
		offsets[offset] = struct{}{}
	}
	return offsets
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
