package calendar

import (
	"testing"
	"time"
)

func FuzzAddMonthsClampedPreservesClampedCivilDate(f *testing.F) {
	f.Add(uint8(30), uint8(0), uint8(30), uint8(9), uint8(0), uint8(0))
	f.Add(uint8(28), uint8(0), uint8(30), uint8(9), uint8(0), uint8(1))
	f.Add(uint8(11), uint8(10), uint8(29), uint8(9), uint8(0), uint8(2))
	f.Add(uint8(26), uint8(2), uint8(7), uint8(2), uint8(0), uint8(1))

	zones := [...]string{"America/Santiago", "America/New_York", "Pacific/Apia", "UTC"}
	f.Fuzz(func(t *testing.T, yearOffset, monthOffset, dayOffset, hourOffset, monthDelta, zoneIndex uint8) {
		year := 2000 + int(yearOffset%41)
		month := time.Month(1 + monthOffset%12)
		lastStartDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
		day := 1 + int(dayOffset)%lastStartDay
		hour := int(hourOffset % 24)
		months := int64(1 + monthDelta%24)
		loc, err := time.LoadLocation(zones[int(zoneIndex)%len(zones)])
		if err != nil {
			t.Fatal(err)
		}
		start := time.Date(year, month, day, hour, 0, 0, 0, loc)
		startYear, startMonth, startDay := start.In(loc).Date()

		monthIndex := int64(startYear)*12 + int64(startMonth-1) + months
		wantYear := int(monthIndex / 12)
		wantMonth := time.Month(monthIndex%12) + 1
		wantDay := startDay
		lastDay := time.Date(wantYear, wantMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if wantDay > lastDay {
			wantDay = lastDay
		}
		_, dateExists := FirstInstantOfDate(wantYear, wantMonth, wantDay, loc)

		got, ok := AddMonthsClamped(start, months)
		if !dateExists {
			if ok || !got.IsZero() {
				t.Fatalf("anniversary on skipped date resolved to %s, %v; want zero and false", got.In(loc), ok)
			}
			return
		}
		if !ok {
			t.Fatalf("valid clamped anniversary %04d-%02d-%02d returned false", wantYear, wantMonth, wantDay)
		}
		gotYear, gotMonth, gotDay := got.In(loc).Date()
		if gotYear != wantYear || gotMonth != wantMonth || gotDay != wantDay {
			t.Fatalf("anniversary resolved to %04d-%02d-%02d; want clamped local date %04d-%02d-%02d", gotYear, gotMonth, gotDay, wantYear, wantMonth, wantDay)
		}
	})
}
