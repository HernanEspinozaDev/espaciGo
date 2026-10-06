package booking

import (
	"testing"
	"time"
)

func TestAvailabilityCandidatesHourlyUseElapsedDurationAndRejectPassedStarts(t *testing.T) {
	loc, _ := time.LoadLocation("America/Santiago")
	now := time.Date(2030, 1, 10, 10, 15, 0, 0, loc)
	items, err := availabilityCandidates("hora", "2030-01-10", 2, loc.String(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 27 || !items[0].StartAt.Equal(time.Date(2030, 1, 10, 10, 30, 0, 0, loc).UTC()) || items[0].EndAt.Sub(items[0].StartAt) != 2*time.Hour {
		t.Fatalf("hour candidates start/count/duration = %v/%d/%v", items[0].StartAt.In(loc), len(items), items[0].EndAt.Sub(items[0].StartAt))
	}
	if _, err = availabilityCandidates("hora", "2030-01-10", 3, loc.String(), now); err == nil {
		t.Fatal("unsupported hourly duration accepted")
	}
}

func TestAvailabilityCandidatesCalendarDaysRespectDSTAndHorizon(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, loc)
	items, err := availabilityCandidates("dia", "2026-03-08", 1, loc.String(), now)
	if err != nil || len(items) != 1 {
		t.Fatalf("DST day candidates=%+v err=%v", items, err)
	}
	if got := items[0].EndAt.Sub(items[0].StartAt); got != 23*time.Hour {
		t.Fatalf("local day spanning spring DST = %v, want 23h", got)
	}
	for _, date := range []string{"2026-03-07", "2026-06-05", "2026-06-06", "2026-06-07"} {
		_, err = availabilityCandidates("dia", date, 1, loc.String(), now)
		wantValid := date == "2026-03-07" || date == "2026-06-05"
		if wantValid != (err == nil) {
			t.Fatalf("date %s within 90-day inclusive horizon: err=%v", date, err)
		}
	}
}

func TestAvailabilityCandidatesOmitDSTGapAndDisambiguateRepeatedTime(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	gapDay, err := availabilityCandidates("hora", "2026-03-08", 1, loc.String(), time.Date(2026, 3, 7, 0, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range gapDay {
		if local := item.StartAt.In(loc); local.Hour() == 2 {
			t.Fatalf("nonexistent spring local time offered: %s", local)
		}
	}
	foldDay, err := availabilityCandidates("hora", "2026-11-01", 1, loc.String(), time.Date(2026, 10, 31, 0, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	var repeated []time.Time
	for _, item := range foldDay {
		local := item.StartAt.In(loc)
		if local.Hour() == 1 && local.Minute() == 30 {
			repeated = append(repeated, item.StartAt)
		}
	}
	if len(repeated) != 2 || repeated[1].Sub(repeated[0]) != time.Hour {
		t.Fatalf("repeated local 01:30 candidates = %v", repeated)
	}
}

func TestAvailabilityCandidatesCalendarMonthClampsAnniversary(t *testing.T) {
	items, err := availabilityCandidates("mes", "2030-01-31", 1, "America/Santiago", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(items) != 1 {
		t.Fatalf("month candidate=%+v err=%v", items, err)
	}
	loc, _ := time.LoadLocation("America/Santiago")
	end := items[0].EndAt.In(loc)
	if y, m, d := end.Date(); y != 2030 || m != time.February || d != 28 || end.Hour() != 0 {
		t.Fatalf("Jan 31 monthly anniversary was %s", end)
	}
	if _, err = availabilityCandidates("mes", "2030-01-31", 2, loc.String(), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("multiple-month option should not be offered")
	}
}
