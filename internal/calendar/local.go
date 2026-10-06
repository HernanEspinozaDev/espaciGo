// Package calendar resolves local calendar labels to instants without relying
// on time.Date's unspecified normalization for daylight-saving gaps.
package calendar

import (
	"sort"
	"time"
)

// FirstInstantOfDate returns the earliest instant whose local date is y-m-d.
// If midnight is skipped, it returns the transition instant that begins the
// date. A date skipped entirely by a timezone change has no such instant.
func FirstInstantOfDate(year int, month time.Month, day int, loc *time.Location) (time.Time, bool) {
	return ResolveWallTime(year, month, day, 0, 0, 0, 0, loc)
}

// ResolveWallTime selects the earliest instant matching the requested local
// wall time. Repeated wall times use their first occurrence; a nonexistent
// wall time within a forward clock jump resolves to the jump's first instant.
func ResolveWallTime(year int, month time.Month, day, hour, minute, second, nanosecond int, loc *time.Location) (time.Time, bool) {
	wall := time.Date(year, month, day, hour, minute, second, nanosecond, time.UTC)
	candidates := make([]time.Time, 0, 2)
	for offset := range offsetsNear(wall, loc) {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(loc)
		ly, lm, ld := local.Date()
		lh, lmin, ls := local.Clock()
		if ly == year && lm == month && ld == day && lh == hour && lmin == minute && ls == second && local.Nanosecond() == nanosecond {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
		return candidates[0], true
	}
	return firstGapInstant(wall, year, month, day, loc)
}

// AddMonthsClamped applies calendar-month billing from v's local date and wall
// time, clamps to the target month's last date, then resolves DST gaps/folds.
func AddMonthsClamped(v time.Time, months int64) (time.Time, bool) {
	year, month, day := v.Date()
	monthIndex := int64(year)*12 + int64(month-1) + months
	targetYear := int(monthIndex / 12)
	targetMonth := time.Month(monthIndex%12) + 1
	if targetMonth < time.January {
		targetYear--
		targetMonth = time.December
	}
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	hour, minute, second := v.Clock()
	return ResolveWallTime(targetYear, targetMonth, day, hour, minute, second, v.Nanosecond(), v.Location())
}

// DateOrdinal maps a calendar date to an integer day independent of its zone.
func DateOrdinal(year int, month time.Month, day int) int64 {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func firstGapInstant(wall time.Time, year int, month time.Month, day int, loc *time.Location) (time.Time, bool) {
	from, to := wall.Add(-72*time.Hour), wall.Add(72*time.Hour)
	for probe := from; probe.Before(to); {
		_, oldOffset := probe.In(loc).Zone()
		_, zoneEnd := probe.In(loc).ZoneBounds()
		if zoneEnd.IsZero() || !zoneEnd.Before(to) {
			return time.Time{}, false
		}
		_, newOffset := zoneEnd.In(loc).Zone()
		if newOffset > oldOffset {
			oldWall := zoneEnd.Add(time.Duration(oldOffset) * time.Second)
			newWall := zoneEnd.Add(time.Duration(newOffset) * time.Second)
			if !wall.Before(oldWall) && wall.Before(newWall) {
				local := zoneEnd.In(loc)
				y, m, d := local.Date()
				if y == year && m == month && d == day {
					return zoneEnd, true
				}
			}
		}
		probe = zoneEnd
	}
	return time.Time{}, false
}

func offsetsNear(wall time.Time, loc *time.Location) map[int]struct{} {
	offsets := make(map[int]struct{})
	for probe := wall.Add(-72 * time.Hour); !probe.After(wall.Add(72 * time.Hour)); probe = probe.Add(15 * time.Minute) {
		_, offset := probe.In(loc).Zone()
		offsets[offset] = struct{}{}
	}
	return offsets
}
