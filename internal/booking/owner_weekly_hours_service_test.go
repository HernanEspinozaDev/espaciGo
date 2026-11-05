package booking

import (
	"context"
	"testing"
	"time"
)

func TestOwnerWeeklyHoursServiceIsolatesScheduleFromBookingDependencies(t *testing.T) {
	repo := &ownerWeeklyHoursRepoFake{}
	svc := NewOwnerWeeklyHoursService(repo)
	got, err := svc.OwnerWeeklyHours(context.Background(), ownerWeeklyOwner, ownerWeeklySpace)
	if err != nil || got.SpaceID != ownerWeeklySpace {
		t.Fatalf("owner schedule read: got=%+v err=%v", got, err)
	}
	expected := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, err = svc.SaveOwnerWeeklyHoursCAS(context.Background(), ownerWeeklyOwner, ownerWeeklySpace, ownerWeeklyValue(), expected)
	if err != nil || got.SpaceID != ownerWeeklySpace || !repo.expected.Equal(expected) {
		t.Fatalf("owner schedule CAS: got=%+v expected=%s err=%v", got, repo.expected, err)
	}
}
