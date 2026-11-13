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
	expected := time.Date(2026, 1, 2, 0, 4, 5, 0, time.FixedZone("UTC-03", -3*60*60))
	got, err = svc.SaveOwnerWeeklyHoursCAS(context.Background(), ownerWeeklyOwner, ownerWeeklySpace, ownerWeeklyValue(), expected)
	if err != nil || got.SpaceID != ownerWeeklySpace || !repo.expected.Equal(expected) || repo.expected.Location() != time.UTC {
		t.Fatalf("owner schedule CAS: got=%+v expected=%s err=%v", got, repo.expected, err)
	}
}

type ownerWeeklyHoursCountingRepo struct {
	ownerWeeklyHoursRepoFake
	readCalls int
	saveCalls int
}

func (r *ownerWeeklyHoursCountingRepo) WeeklyHoursForOwner(ctx context.Context, owner, space string) (WeeklyHours, error) {
	r.readCalls++
	return r.ownerWeeklyHoursRepoFake.WeeklyHoursForOwner(ctx, owner, space)
}

func (r *ownerWeeklyHoursCountingRepo) SaveWeeklyHoursForOwnerCAS(ctx context.Context, owner, space string, value WeeklyHours, expected time.Time) (WeeklyHours, error) {
	r.saveCalls++
	return r.ownerWeeklyHoursRepoFake.SaveWeeklyHoursForOwnerCAS(ctx, owner, space, value, expected)
}

func TestOwnerWeeklyHoursServiceRejectsInvalidCASBeforeRepositoryWrite(t *testing.T) {
	duplicateWeekday := ownerWeeklyValue()
	duplicateWeekday.Days[0].Weekday = duplicateWeekday.Days[1].Weekday
	weekdayOutsideISOWeek := ownerWeeklyValue()
	weekdayOutsideISOWeek.Days[6].Weekday = 8
	validRevision := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	cases := []struct {
		name, owner, space string
		value              WeeklyHours
		expected           time.Time
	}{
		{name: "invalid owner", owner: "not-a-uuid", space: ownerWeeklySpace, value: ownerWeeklyValue(), expected: validRevision},
		{name: "invalid space", owner: ownerWeeklyOwner, space: "not-a-uuid", value: ownerWeeklyValue(), expected: validRevision},
		{name: "missing revision", owner: ownerWeeklyOwner, space: ownerWeeklySpace, value: ownerWeeklyValue()},
		{name: "duplicate weekday", owner: ownerWeeklyOwner, space: ownerWeeklySpace, value: duplicateWeekday, expected: validRevision},
		{name: "weekday outside ISO week", owner: ownerWeeklyOwner, space: ownerWeeklySpace, value: weekdayOutsideISOWeek, expected: validRevision},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &ownerWeeklyHoursCountingRepo{}
			svc := NewOwnerWeeklyHoursService(repo)
			if _, err := svc.SaveOwnerWeeklyHoursCAS(context.Background(), tc.owner, tc.space, tc.value, tc.expected); err != ErrInvalid {
				t.Fatalf("SaveOwnerWeeklyHoursCAS() error = %v, want %v", err, ErrInvalid)
			}
			if repo.saveCalls != 0 {
				t.Fatalf("repository save calls = %d, want 0", repo.saveCalls)
			}
		})
	}
}

func TestOwnerWeeklyHoursServiceRejectsInvalidReadScopeBeforeRepositoryCall(t *testing.T) {
	cases := []struct {
		name       string
		owner      string
		space      string
		nilService bool
		nilRepo    bool
	}{
		{name: "invalid owner", owner: "not-a-uuid", space: ownerWeeklySpace},
		{name: "invalid space", owner: ownerWeeklyOwner, space: "not-a-uuid"},
		{name: "nil service", owner: ownerWeeklyOwner, space: ownerWeeklySpace, nilService: true},
		{name: "nil repository", owner: ownerWeeklyOwner, space: ownerWeeklySpace, nilRepo: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &ownerWeeklyHoursCountingRepo{}
			var svc *OwnerWeeklyHoursService
			if !tc.nilService {
				if tc.nilRepo {
					svc = NewOwnerWeeklyHoursService(nil)
				} else {
					svc = NewOwnerWeeklyHoursService(repo)
				}
			}
			if _, err := svc.OwnerWeeklyHours(context.Background(), tc.owner, tc.space); err != ErrNotFound {
				t.Fatalf("OwnerWeeklyHours() error = %v, want %v", err, ErrNotFound)
			}
			if repo.readCalls != 0 {
				t.Fatalf("repository read calls = %d, want 0", repo.readCalls)
			}
		})
	}
}

func TestOwnerWeeklyHoursServiceRequiresRepositoryForValidCAS(t *testing.T) {
	expected := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var nilService *OwnerWeeklyHoursService
	if _, err := nilService.SaveOwnerWeeklyHoursCAS(context.Background(), ownerWeeklyOwner, ownerWeeklySpace, ownerWeeklyValue(), expected); err != ErrNotFound {
		t.Fatalf("nil service CAS error = %v, want %v", err, ErrNotFound)
	}
	if _, err := NewOwnerWeeklyHoursService(nil).SaveOwnerWeeklyHoursCAS(context.Background(), ownerWeeklyOwner, ownerWeeklySpace, ownerWeeklyValue(), expected); err != ErrNotFound {
		t.Fatalf("nil repository CAS error = %v, want %v", err, ErrNotFound)
	}
}
