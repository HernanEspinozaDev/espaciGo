package booking

import (
	"context"
	"testing"
	"time"
)

const ownerWeeklyOwner = "11111111-1111-4111-8111-111111111111"
const ownerWeeklySpace = "22222222-2222-4222-8222-222222222222"

func TestOwnerWeeklyHoursUsesOwnerScopedCASRepository(t *testing.T) {
	repo := &ownerWeeklyHoursRepoFake{}
	svc := &Service{repo: repo}
	got, err := svc.OwnerWeeklyHours(context.Background(), ownerWeeklyOwner, ownerWeeklySpace)
	if err != nil || got.SpaceID != ownerWeeklySpace {
		t.Fatalf("read: got=%+v err=%v", got, err)
	}
	wantRevision := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, err = svc.SaveOwnerWeeklyHoursCAS(context.Background(), ownerWeeklyOwner, ownerWeeklySpace, ownerWeeklyValue(), wantRevision)
	if err != nil || got.SpaceID != ownerWeeklySpace || !repo.expected.Equal(wantRevision) {
		t.Fatalf("save: got=%+v expected=%s err=%v", got, repo.expected, err)
	}
	if _, err = svc.OwnerWeeklyHours(context.Background(), "33333333-3333-4333-8333-333333333333", ownerWeeklySpace); err != ErrNotFound {
		t.Fatalf("foreign owner: %v", err)
	}
}

func ownerWeeklyValue() WeeklyHours {
	days := make([]WeeklyDay, 7)
	for i := range days {
		days[i] = WeeklyDay{Weekday: i + 1}
	}
	return WeeklyHours{Enabled: true, TimeZone: "America/Santiago", Days: days}
}

type ownerWeeklyHoursRepoFake struct {
	Repository
	expected time.Time
}

func (r *ownerWeeklyHoursRepoFake) WeeklyHoursForOwner(_ context.Context, owner, space string) (WeeklyHours, error) {
	if owner != ownerWeeklyOwner || space != ownerWeeklySpace {
		return WeeklyHours{}, ErrNotFound
	}
	v := ownerWeeklyValue()
	v.SpaceID = space
	return v, nil
}
func (r *ownerWeeklyHoursRepoFake) SaveWeeklyHoursForOwnerCAS(_ context.Context, owner, space string, v WeeklyHours, expected time.Time) (WeeklyHours, error) {
	if owner != ownerWeeklyOwner || space != ownerWeeklySpace {
		return WeeklyHours{}, ErrNotFound
	}
	r.expected = expected
	v.SpaceID = space
	return v, nil
}
