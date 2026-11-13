package occupancy

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestParseIntervalRequiresFiniteRFC3339IncreasingBounds(t *testing.T) {
	start, end, err := parseInterval("2030-01-01T13:00:00Z", "2030-01-01T14:00:00Z")
	if err != nil || !start.Equal(time.Date(2030, 1, 1, 13, 0, 0, 0, time.UTC)) || !end.After(start) {
		t.Fatalf("parsed interval = %s .. %s, err=%v", start, end, err)
	}
	for _, pair := range [][2]string{
		{"2030-01-01T10:00:00Z", "2030-01-01T10:00:00Z"},
		{"2030-01-01T11:00:00Z", "2030-01-01T10:00:00Z"},
		{"2030-01-01", "2030-01-01T10:00:00Z"},
		{"2030-01-01T10:00:00-03:00", "2030-01-01T11:00:00-03:00"},
		{"2030-02-30T10:00:00Z", "2030-03-01T10:00:00Z"},
		{"2030-01-01T24:00:00Z", "2030-01-02T00:00:00Z"},
		{"2030-01-01T10:00:60Z", "2030-01-01T11:00:00Z"},
		{"2030-01-01T10:00:00z", "2030-01-01T11:00:00Z"},
	} {
		if _, _, err := parseInterval(pair[0], pair[1]); err == nil {
			t.Errorf("accepted invalid interval %q .. %q", pair[0], pair[1])
		}
	}
}

func TestParseIntervalPreservesFractionalUTCPrecision(t *testing.T) {
	start, end, err := parseInterval(
		"2030-01-01T13:00:00.123456789Z",
		"2030-01-01T14:00:00.987654321Z",
	)
	if err != nil {
		t.Fatalf("parseInterval() error = %v", err)
	}
	if start.Nanosecond() != 123456789 || end.Nanosecond() != 987654321 {
		t.Fatalf("fractional precision = %d / %d", start.Nanosecond(), end.Nanosecond())
	}
	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatalf("locations = %v / %v, want UTC", start.Location(), end.Location())
	}
}

func TestValidTimeZoneUsesIANAZoneDatabase(t *testing.T) {
	for _, zone := range []string{"America/Santiago", "Pacific/Auckland", "Pacific/Easter", "UTC", "EST"} {
		if !validTimeZone(zone) {
			t.Errorf("rejected IANA zone %q", zone)
		}
	}
	for _, zone := range []string{"", " ../etc/passwd", "America/../UTC", "not-a-zone", "Local"} {
		if validTimeZone(zone) {
			t.Errorf("accepted invalid zone %q", zone)
		}
	}
	if validTimeZone("Factory") {
		t.Error("accepted noncommittal Factory zone as a space locality")
	}
}

type timeZoneCASRepositoryStub struct {
	Repository
	calls     int
	owner     string
	spaceID   string
	zone      string
	expected  time.Time
	updatedAt time.Time
}

func (r *timeZoneCASRepositoryStub) SetTimeZoneCAS(_ context.Context, owner, spaceID, zone string, expected time.Time) (time.Time, error) {
	r.calls++
	r.owner, r.spaceID, r.zone, r.expected = owner, spaceID, zone, expected
	return r.updatedAt, nil
}

func TestSetTimeZoneCASValidatesAndForwardsIANARevision(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	expected := time.Date(2030, 1, 2, 3, 4, 5, 600, time.UTC)

	for _, tc := range []struct {
		name     string
		owner    string
		spaceID  string
		zone     string
		expected time.Time
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID, zone: "America/Santiago", expected: expected},
		{name: "invalid owner hex", owner: "00000000-0000-4000-8000-00000000000g", spaceID: spaceID, zone: "America/Santiago", expected: expected},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid", zone: "America/Santiago", expected: expected},
		{name: "empty timezone", owner: owner, spaceID: spaceID, zone: "", expected: expected},
		{name: "local timezone", owner: owner, spaceID: spaceID, zone: "Local", expected: expected},
		{name: "noncommittal factory timezone", owner: owner, spaceID: spaceID, zone: "Factory", expected: expected},
		{name: "path-like timezone", owner: owner, spaceID: spaceID, zone: "../UTC", expected: expected},
		{name: "unknown timezone", owner: owner, spaceID: spaceID, zone: "Not/A_Real_Zone", expected: expected},
		{name: "zero expected revision", owner: owner, spaceID: spaceID, zone: "America/Santiago"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &timeZoneCASRepositoryStub{}
			service, err := NewService(repo, credentials.Generator{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.SetTimeZoneCAS(context.Background(), tc.owner, tc.spaceID, tc.zone, tc.expected); err != ErrInvalid {
				t.Fatalf("SetTimeZoneCAS() error = %v, want %v", err, ErrInvalid)
			}
			if repo.calls != 0 {
				t.Fatalf("invalid request reached repository %d times", repo.calls)
			}
		})
	}

	repo := &timeZoneCASRepositoryStub{updatedAt: expected.Add(time.Second)}
	service, err := NewService(repo, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.SetTimeZoneCAS(context.Background(), owner, spaceID, "Pacific/Easter", expected)
	if err != nil || !got.Equal(repo.updatedAt) {
		t.Fatalf("SetTimeZoneCAS() = %s, %v; want %s, nil", got, err, repo.updatedAt)
	}
	if repo.calls != 1 || repo.owner != owner || repo.spaceID != spaceID || repo.zone != "Pacific/Easter" || !repo.expected.Equal(expected) {
		t.Fatalf("repository call = %+v; want one call with exact owner, space, zone, and revision", repo)
	}
}

type timeZoneAccessRepositoryStub struct {
	Repository
	setCalls       int
	getCalls       int
	setOwner       string
	setSpaceID     string
	setZone        string
	getOwner       string
	getSpaceID     string
	storedTimeZone string
}

func (r *timeZoneAccessRepositoryStub) SetTimeZone(_ context.Context, owner, spaceID, zone string) error {
	r.setCalls++
	r.setOwner, r.setSpaceID, r.setZone = owner, spaceID, zone
	return nil
}

func (r *timeZoneAccessRepositoryStub) TimeZone(_ context.Context, owner, spaceID string) (string, error) {
	r.getCalls++
	r.getOwner, r.getSpaceID = owner, spaceID
	return r.storedTimeZone, nil
}

func TestSetTimeZoneValidatesAndForwardsIANAZone(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	for _, tc := range []struct {
		name    string
		owner   string
		spaceID string
		zone    string
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID, zone: "Pacific/Easter"},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid", zone: "Pacific/Easter"},
		{name: "empty timezone", owner: owner, spaceID: spaceID, zone: ""},
		{name: "local timezone", owner: owner, spaceID: spaceID, zone: "Local"},
		{name: "noncommittal factory timezone", owner: owner, spaceID: spaceID, zone: "Factory"},
		{name: "path-like timezone", owner: owner, spaceID: spaceID, zone: "../UTC"},
		{name: "unknown timezone", owner: owner, spaceID: spaceID, zone: "Not/A_Real_Zone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &timeZoneAccessRepositoryStub{}
			service, err := NewService(repo, credentials.Generator{})
			if err != nil {
				t.Fatal(err)
			}
			if err := service.SetTimeZone(context.Background(), tc.owner, tc.spaceID, tc.zone); err != ErrInvalid {
				t.Fatalf("SetTimeZone() error = %v, want %v", err, ErrInvalid)
			}
			if repo.setCalls != 0 {
				t.Fatalf("invalid request reached repository %d times", repo.setCalls)
			}
		})
	}

	repo := &timeZoneAccessRepositoryStub{}
	service, err := NewService(repo, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetTimeZone(context.Background(), owner, spaceID, "Pacific/Easter"); err != nil {
		t.Fatalf("SetTimeZone() error = %v", err)
	}
	if repo.setCalls != 1 || repo.setOwner != owner || repo.setSpaceID != spaceID || repo.setZone != "Pacific/Easter" {
		t.Fatalf("SetTimeZone repository call = %+v", repo)
	}
}

func TestTimeZoneValidatesAndForwardsSpaceLookup(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	for _, tc := range []struct {
		name    string
		owner   string
		spaceID string
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &timeZoneAccessRepositoryStub{}
			service, err := NewService(repo, credentials.Generator{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.TimeZone(context.Background(), tc.owner, tc.spaceID); err != ErrInvalid {
				t.Fatalf("TimeZone() error = %v, want %v", err, ErrInvalid)
			}
			if repo.getCalls != 0 {
				t.Fatalf("invalid lookup reached repository %d times", repo.getCalls)
			}
		})
	}

	repo := &timeZoneAccessRepositoryStub{storedTimeZone: "Pacific/Easter"}
	service, err := NewService(repo, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.TimeZone(context.Background(), owner, spaceID)
	if err != nil || got != repo.storedTimeZone {
		t.Fatalf("TimeZone() = %q, %v; want %q, nil", got, err, repo.storedTimeZone)
	}
	if repo.getCalls != 1 || repo.getOwner != owner || repo.getSpaceID != spaceID {
		t.Fatalf("TimeZone repository call = %+v", repo)
	}
}

type rangeRepositoryStub struct {
	Repository
	availabilityCalls int
	listBlocksCalls   int
	owner             string
	spaceID           string
	start             time.Time
	end               time.Time
	availability      Availability
	calendar          Calendar
}

func (r *rangeRepositoryStub) Availability(_ context.Context, owner, spaceID string, start, end time.Time) (Availability, error) {
	r.availabilityCalls++
	r.owner, r.spaceID, r.start, r.end = owner, spaceID, start, end
	return r.availability, nil
}

func (r *rangeRepositoryStub) ListBlocks(_ context.Context, owner, spaceID string, start, end time.Time) (Calendar, error) {
	r.listBlocksCalls++
	r.owner, r.spaceID, r.start, r.end = owner, spaceID, start, end
	return r.calendar, nil
}

func TestAvailabilityValidatesAndForwardsUTCWindow(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	for _, tc := range []struct {
		name    string
		owner   string
		spaceID string
		from    string
		to      string
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID, from: "2030-01-01T10:00:00Z", to: "2030-01-01T11:00:00Z"},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid", from: "2030-01-01T10:00:00Z", to: "2030-01-01T11:00:00Z"},
		{name: "empty start", owner: owner, spaceID: spaceID, to: "2030-01-01T11:00:00Z"},
		{name: "empty end", owner: owner, spaceID: spaceID, from: "2030-01-01T10:00:00Z"},
		{name: "non-UTC offset", owner: owner, spaceID: spaceID, from: "2030-01-01T10:00:00-03:00", to: "2030-01-01T11:00:00-03:00"},
		{name: "equal bounds", owner: owner, spaceID: spaceID, from: "2030-01-01T10:00:00Z", to: "2030-01-01T10:00:00Z"},
		{name: "inverted bounds", owner: owner, spaceID: spaceID, from: "2030-01-01T11:00:00Z", to: "2030-01-01T10:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &rangeRepositoryStub{}
			service, err := NewService(repo, credentials.Generator{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Availability(context.Background(), tc.owner, tc.spaceID, tc.from, tc.to); err != ErrInvalid {
				t.Fatalf("Availability() error = %v, want %v", err, ErrInvalid)
			}
			if repo.availabilityCalls != 0 {
				t.Fatalf("invalid request reached repository %d times", repo.availabilityCalls)
			}
		})
	}

	want := Availability{SpaceID: spaceID, TimeZone: "Pacific/Easter", Available: true}
	repo := &rangeRepositoryStub{availability: want}
	service, err := NewService(repo, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.Availability(context.Background(), owner, spaceID, "2030-01-01T13:00:00.123456789Z", "2030-01-01T14:00:00.987654321Z")
	if err != nil || got != want {
		t.Fatalf("Availability() = %+v, %v; want %+v, nil", got, err, want)
	}
	if repo.availabilityCalls != 1 || repo.owner != owner || repo.spaceID != spaceID || !repo.start.Equal(time.Date(2030, 1, 1, 13, 0, 0, 123456789, time.UTC)) || !repo.end.Equal(time.Date(2030, 1, 1, 14, 0, 0, 987654321, time.UTC)) {
		t.Fatalf("Availability repository call = %+v", repo)
	}
}

func TestListBlocksValidatesAndForwardsUTCWindow(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	for _, tc := range []struct {
		name    string
		owner   string
		spaceID string
		from    string
		to      string
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID, from: "2030-01-01T10:00:00Z", to: "2030-01-01T11:00:00Z"},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid", from: "2030-01-01T10:00:00Z", to: "2030-01-01T11:00:00Z"},
		{name: "empty start", owner: owner, spaceID: spaceID, to: "2030-01-01T11:00:00Z"},
		{name: "empty end", owner: owner, spaceID: spaceID, from: "2030-01-01T10:00:00Z"},
		{name: "equal bounds", owner: owner, spaceID: spaceID, from: "2030-01-01T10:00:00Z", to: "2030-01-01T10:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &rangeRepositoryStub{}
			service, err := NewService(repo, credentials.Generator{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.ListBlocks(context.Background(), tc.owner, tc.spaceID, tc.from, tc.to); err != ErrInvalid {
				t.Fatalf("ListBlocks() error = %v, want %v", err, ErrInvalid)
			}
			if repo.listBlocksCalls != 0 {
				t.Fatalf("invalid request reached repository %d times", repo.listBlocksCalls)
			}
		})
	}

	want := Calendar{SpaceID: spaceID, TimeZone: "Pacific/Easter", Items: []Block{}}
	repo := &rangeRepositoryStub{calendar: want}
	service, err := NewService(repo, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.ListBlocks(context.Background(), owner, spaceID, "2030-01-01T13:00:00.123456789Z", "2030-01-01T14:00:00.987654321Z")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ListBlocks() = %+v, %v; want %+v, nil", got, err, want)
	}
	if repo.listBlocksCalls != 1 || repo.owner != owner || repo.spaceID != spaceID || !repo.start.Equal(time.Date(2030, 1, 1, 13, 0, 0, 123456789, time.UTC)) || !repo.end.Equal(time.Date(2030, 1, 1, 14, 0, 0, 987654321, time.UTC)) {
		t.Fatalf("ListBlocks repository call = %+v", repo)
	}
}

type manualBlockIDsStub struct {
	id    string
	err   error
	calls int
}

func (s *manualBlockIDsStub) ID() (string, error) {
	s.calls++
	return s.id, s.err
}

func (*manualBlockIDsStub) Token() (identity.Secret, error) { return "", nil }

type manualBlockRepositoryStub struct {
	Repository
	createCalls int
	deleteCalls int
	owner       string
	spaceID     string
	blockID     string
	start       time.Time
	end         time.Time
	reason      string
	createErr   error
	deleteErr   error
	created     Block
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo Repository
		ids  identity.CredentialGenerator
	}{
		{name: "nil repository", ids: credentials.Generator{}},
		{name: "nil ID generator", repo: &manualBlockRepositoryStub{}},
		{name: "both dependencies nil"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewService(tc.repo, tc.ids)
			if service != nil || err != ErrInvalid {
				t.Fatalf("NewService() = %v, %v; want nil, %v", service, err, ErrInvalid)
			}
		})
	}
}

func (r *manualBlockRepositoryStub) CreateBlock(_ context.Context, owner, spaceID, id string, start, end time.Time, reason string) (Block, error) {
	r.createCalls++
	r.owner, r.spaceID, r.blockID = owner, spaceID, id
	r.start, r.end, r.reason = start, end, reason
	return r.created, r.createErr
}

func (r *manualBlockRepositoryStub) DeleteBlock(_ context.Context, owner, spaceID, blockID string) error {
	r.deleteCalls++
	r.owner, r.spaceID, r.blockID = owner, spaceID, blockID
	return r.deleteErr
}

func TestCreateBlockValidatesAndForwardsManualBlock(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	blockID := "00000000-0000-4000-8000-000000000003"
	valid := BlockInput{
		StartAt: "2030-04-01T12:00:00.123456789Z",
		EndAt:   "2030-04-01T13:00:00.987654321Z",
		Reason:  strings.Repeat("😀", 500),
	}

	for _, tc := range []struct {
		name    string
		owner   string
		spaceID string
		input   BlockInput
	}{
		{name: "invalid start timestamp", owner: owner, spaceID: spaceID, input: BlockInput{StartAt: "2030-04-01", EndAt: valid.EndAt, Reason: "reason"}},
		{name: "invalid end timestamp", owner: owner, spaceID: spaceID, input: BlockInput{StartAt: valid.StartAt, EndAt: "2030-04-01", Reason: "reason"}},
		{name: "equal bounds", owner: owner, spaceID: spaceID, input: BlockInput{StartAt: valid.StartAt, EndAt: valid.StartAt, Reason: "reason"}},
		{name: "inverted bounds", owner: owner, spaceID: spaceID, input: BlockInput{StartAt: valid.EndAt, EndAt: valid.StartAt, Reason: "reason"}},
		{name: "empty trimmed reason", owner: owner, spaceID: spaceID, input: BlockInput{StartAt: valid.StartAt, EndAt: valid.EndAt, Reason: " 	 "}},
		{name: "501 Unicode codepoints", owner: owner, spaceID: spaceID, input: BlockInput{StartAt: valid.StartAt, EndAt: valid.EndAt, Reason: strings.Repeat("😀", 501)}},
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID, input: valid},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid", input: valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &manualBlockRepositoryStub{}
			ids := &manualBlockIDsStub{id: blockID}
			service, err := NewService(repo, ids)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.CreateBlock(context.Background(), tc.owner, tc.spaceID, tc.input); err != ErrInvalid {
				t.Fatalf("CreateBlock() error = %v, want %v", err, ErrInvalid)
			}
			if ids.calls != 0 || repo.createCalls != 0 {
				t.Fatalf("invalid request reached ID/repository: ID calls=%d repository calls=%d", ids.calls, repo.createCalls)
			}
		})
	}

	t.Run("ID generation error stops before repository", func(t *testing.T) {
		idErr := errors.New("synthetic ID failure")
		repo := &manualBlockRepositoryStub{}
		ids := &manualBlockIDsStub{id: blockID, err: idErr}
		service, err := NewService(repo, ids)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateBlock(context.Background(), owner, spaceID, valid); err != idErr {
			t.Fatalf("CreateBlock() error = %v, want ID error", err)
		}
		if ids.calls != 1 || repo.createCalls != 0 {
			t.Fatalf("ID/repository calls = %d/%d, want 1/0", ids.calls, repo.createCalls)
		}
	})

	t.Run("valid block normalizes UTC and accepts a 500-codepoint reason", func(t *testing.T) {
		want := Block{
			ID: blockID, SpaceID: spaceID,
			StartAt:  time.Date(2030, 4, 1, 12, 0, 0, 123456789, time.UTC),
			EndAt:    time.Date(2030, 4, 1, 13, 0, 0, 987654321, time.UTC),
			TimeZone: "Pacific/Easter", Reason: strings.Repeat("😀", 500),
		}
		repo := &manualBlockRepositoryStub{created: want}
		ids := &manualBlockIDsStub{id: blockID}
		service, err := NewService(repo, ids)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.CreateBlock(context.Background(), owner, spaceID, valid)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("CreateBlock() = %+v, %v; want %+v, nil", got, err, want)
		}
		if ids.calls != 1 || repo.createCalls != 1 || repo.owner != owner || repo.spaceID != spaceID || repo.blockID != blockID || !repo.start.Equal(want.StartAt) || !repo.end.Equal(want.EndAt) || repo.reason != want.Reason {
			t.Fatalf("ID/repository forwarding mismatch: IDs=%+v repository=%+v", ids, repo)
		}
	})

	t.Run("valid block trims reason before repository forwarding", func(t *testing.T) {
		trimmedInput := valid
		trimmedInput.Reason = "  mantención preventiva  "
		repo := &manualBlockRepositoryStub{}
		service, err := NewService(repo, &manualBlockIDsStub{id: blockID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateBlock(context.Background(), owner, spaceID, trimmedInput); err != nil {
			t.Fatalf("CreateBlock() error = %v", err)
		}
		if repo.createCalls != 1 || repo.reason != "mantención preventiva" {
			t.Fatalf("repository calls=%d reason=%q", repo.createCalls, repo.reason)
		}
	})

	t.Run("repository error propagates", func(t *testing.T) {
		repoErr := errors.New("synthetic repository failure")
		repo := &manualBlockRepositoryStub{createErr: repoErr}
		service, err := NewService(repo, &manualBlockIDsStub{id: blockID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateBlock(context.Background(), owner, spaceID, valid); err != repoErr {
			t.Fatalf("CreateBlock() error = %v, want repository error", err)
		}
		if repo.createCalls != 1 {
			t.Fatalf("repository calls=%d, want one", repo.createCalls)
		}
	})
}

func TestDeleteBlockValidatesAndForwardsOwnerScopedID(t *testing.T) {
	owner := "00000000-0000-4000-8000-000000000001"
	spaceID := "00000000-0000-4000-8000-000000000002"
	blockID := "00000000-0000-4000-8000-000000000003"
	for _, tc := range []struct {
		name    string
		owner   string
		spaceID string
		blockID string
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: spaceID, blockID: blockID},
		{name: "invalid space", owner: owner, spaceID: "not-a-uuid", blockID: blockID},
		{name: "invalid block ID", owner: owner, spaceID: spaceID, blockID: "not-a-uuid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &manualBlockRepositoryStub{}
			service, err := NewService(repo, credentials.Generator{})
			if err != nil {
				t.Fatal(err)
			}
			if err := service.DeleteBlock(context.Background(), tc.owner, tc.spaceID, tc.blockID); err != ErrNotFound {
				t.Fatalf("DeleteBlock() error = %v, want %v", err, ErrNotFound)
			}
			if repo.deleteCalls != 0 {
				t.Fatalf("invalid deletion reached repository %d times", repo.deleteCalls)
			}
		})
	}

	deleteErr := errors.New("synthetic delete failure")
	repo := &manualBlockRepositoryStub{deleteErr: deleteErr}
	service, err := NewService(repo, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteBlock(context.Background(), owner, spaceID, blockID); err != deleteErr {
		t.Fatalf("DeleteBlock() error = %v, want repository error", err)
	}
	if repo.deleteCalls != 1 || repo.owner != owner || repo.spaceID != spaceID || repo.blockID != blockID {
		t.Fatalf("DeleteBlock repository forwarding = %+v", repo)
	}
}
