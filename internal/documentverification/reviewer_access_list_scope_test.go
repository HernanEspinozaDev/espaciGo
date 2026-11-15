package documentverification

import (
	"context"
	"testing"
	"time"
)

func TestReviewerAccessListFailsClosedForUnreviewableSnapshots(t *testing.T) {
	const (
		actorID  = "66666666-6666-4666-8666-666666666666"
		subject  = "77777777-7777-4777-8777-777777777777"
		caseBase = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		spaceID  = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	supersededAt := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	revokedAt := time.Date(2026, 11, 12, 13, 0, 0, 0, time.UTC)
	repo := &reviewerAccessRepo{list: []Snapshot{
		{ID: caseBase, SubjectID: subject, Purpose: PurposeIdentity, State: CaseSubmitted},
		{ID: "44444444-5555-4444-8444-444444444444", SubjectID: subject, SpaceID: spaceID, Purpose: PurposeSpace, State: CaseSubmitted},
		{ID: "55555555-6666-4555-8555-555555555555", SubjectID: subject, SpaceID: spaceID, Purpose: PurposeSpace, State: CaseInReview},
		{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", SubjectID: subject, SpaceID: spaceID, Purpose: PurposeSpace, State: CaseNeedsEvidence},
		{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SubjectID: actorID, Purpose: PurposeIdentity, State: CaseInReview},
		{ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", SubjectID: subject, Purpose: Purpose("unsupported"), State: CaseInReview},
		{ID: "ffffffff-ffff-4fff-8fff-ffffffffffff", SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview, SupersededAt: &supersededAt},
		{ID: "11111111-2222-4111-8111-111111111111", SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview, RevokedAt: &revokedAt},
		{ID: "22222222-3333-4222-8222-222222222222", SubjectID: subject, Purpose: PurposeSpace, State: CaseInReview},
		{ID: "invalid-case-id", SubjectID: subject, Purpose: PurposeIdentity, State: CaseSubmitted},
		{ID: "33333333-4444-4333-8333-333333333333", SubjectID: "invalid-subject-id", Purpose: PurposeIdentity, State: CaseSubmitted},
		{ID: "11111111-1111-4111-8111-111111111111", SubjectID: subject, Purpose: PurposeIdentity, State: CaseApproved},
	}}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessService(repo, cases, &reviewerAccessStore{})
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.List(context.Background(), actorID)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{
		caseBase,
		"44444444-5555-4444-8444-444444444444",
		"55555555-6666-4555-8555-555555555555",
		"cccccccc-cccc-4ccc-8ccc-cccccccccccc",
	}
	if len(got) != len(wantIDs) {
		t.Fatalf("reviewer queue exposed unreviewable snapshots: got=%+v", got)
	}
	for i := range wantIDs {
		if got[i].ID != wantIDs[i] {
			t.Fatalf("reviewer queue[%d] id=%q, want %q", i, got[i].ID, wantIDs[i])
		}
	}
}
