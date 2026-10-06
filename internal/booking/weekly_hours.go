package booking

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// WeeklyHours contains the explicitly configured local schedule for one
// synthetic hourly fixture. ISO weekdays use Monday=1 through Sunday=7.
type WeeklyHours struct {
	SpaceID  string      `json:"space_id"`
	Enabled  bool        `json:"enabled"`
	TimeZone string      `json:"time_zone"`
	Days     []WeeklyDay `json:"days"`
}

type WeeklyDay struct {
	Weekday int            `json:"weekday"`
	Periods []WeeklyPeriod `json:"periods"`
}

type WeeklyPeriod struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

type weeklyMinutePeriod struct{ open, close int }

// ValidateWeeklyHours requires a canonical seven-day shape. Omitted/empty
// periods mean closed once the schedule is enabled.
func ValidateWeeklyHours(value WeeklyHours) error {
	if len(value.Days) != 7 {
		return ErrInvalid
	}
	seen := [8]bool{}
	for _, day := range value.Days {
		if day.Weekday < 1 || day.Weekday > 7 || seen[day.Weekday] || len(day.Periods) > 2 {
			return ErrInvalid
		}
		seen[day.Weekday] = true
		periods := make([]weeklyMinutePeriod, 0, len(day.Periods))
		for _, period := range day.Periods {
			open, okOpen := parseWeeklyClock(period.Open, false)
			close, okClose := parseWeeklyClock(period.Close, true)
			if !okOpen || !okClose || close <= open {
				return ErrInvalid
			}
			periods = append(periods, weeklyMinutePeriod{open: open, close: close})
		}
		sort.Slice(periods, func(i, j int) bool { return periods[i].open < periods[j].open })
		for i := 1; i < len(periods); i++ {
			if periods[i].open < periods[i-1].close {
				return ErrInvalid
			}
		}
	}
	for day := 1; day <= 7; day++ {
		if !seen[day] {
			return ErrInvalid
		}
	}
	return nil
}

func parseWeeklyClock(value string, closing bool) (int, bool) {
	if closing && value == "24:00" {
		return 24 * 60, true
	}
	if len(value) != 5 || value[2] != ':' {
		return 0, false
	}
	hour, e1 := strconv.Atoi(value[:2])
	minute, e2 := strconv.Atoi(value[3:])
	if e1 != nil || e2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 || fmt.Sprintf("%02d:%02d", hour, minute) != value {
		return 0, false
	}
	return hour*60 + minute, true
}

// intervalFitsWeeklyHours checks every constant-offset UTC segment in the
// requested interval. This handles DST gaps/folds without treating elapsed
// hours as wall-clock hours. Boundaries are half-open; 24:00 is the exclusive
// end of its local weekday.
func IntervalFitsWeeklyHours(start, end time.Time, zone string, schedule WeeklyHours) bool {
	if !end.After(start) {
		return false
	}
	if !schedule.Enabled {
		return true
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false
	}
	startLocal := start.In(loc)
	startYear, startMonth, startDay := startLocal.Date()
	periods := make(map[int][]weeklyMinutePeriod, 7)
	for _, day := range schedule.Days {
		for _, period := range day.Periods {
			open, openOK := parseWeeklyClock(period.Open, false)
			close, closeOK := parseWeeklyClock(period.Close, true)
			if !openOK || !closeOK || close <= open {
				return false
			}
			periods[day.Weekday] = append(periods[day.Weekday], weeklyMinutePeriod{open: open, close: close})
		}
	}
	for cursor := start.UTC(); cursor.Before(end.UTC()); {
		local := cursor.In(loc)
		_, zoneEnd := local.ZoneBounds()
		segmentEnd := end.UTC()
		if !zoneEnd.IsZero() && zoneEnd.Before(segmentEnd) {
			segmentEnd = zoneEnd.UTC()
		}
		if !segmentEnd.After(cursor) {
			return false
		}
		last := segmentEnd.Add(-time.Nanosecond).In(loc)
		ly, lm, ld := local.Date()
		y2, m2, d2 := last.Date()
		if ly != y2 || lm != m2 || ld != d2 || ly != startYear || lm != startMonth || ld != startDay {
			return false
		}
		weekday := int(local.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startSecond := local.Hour()*3600 + local.Minute()*60 + local.Second()
		lastSecond := last.Hour()*3600 + last.Minute()*60 + last.Second()
		endSecond := lastSecond + 1
		fits := false
		for _, period := range periods[weekday] {
			if startSecond >= period.open*60 && endSecond <= period.close*60 {
				fits = true
				break
			}
		}
		if !fits {
			return false
		}
		cursor = segmentEnd
	}
	return true
}

func normalizeWeeklyDays(days []WeeklyDay) []WeeklyDay {
	out := make([]WeeklyDay, 0, len(days))
	for _, day := range days {
		periods := append([]WeeklyPeriod(nil), day.Periods...)
		sort.Slice(periods, func(i, j int) bool { return strings.Compare(periods[i].Open, periods[j].Open) < 0 })
		out = append(out, WeeklyDay{Weekday: day.Weekday, Periods: periods})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Weekday < out[j].Weekday })
	return out
}
