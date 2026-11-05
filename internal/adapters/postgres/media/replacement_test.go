package mediapg

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestReplacementTargetMustMatchForReuse(t *testing.T) {
	for _, tc := range []struct {
		prior, requested string
		want             bool
	}{
		{"", "", true}, {"photo-a", "photo-a", true}, {"photo-a", "photo-b", false}, {"", "photo-a", false}, {"photo-a", "", false},
	} {
		if got := replacementTargetMatches(tc.prior, tc.requested); got != tc.want {
			t.Errorf("replacementTargetMatches(%q,%q)=%v want %v", tc.prior, tc.requested, got, tc.want)
		}
	}
}

func TestReplacementReplayRequiresConsistentTargetRevisionPair(t *testing.T) {
	stored := time.Date(2026, 10, 27, 22, 18, 16, 0, time.UTC)
	valid := pgtype.Timestamptz{Time: stored, Valid: true}
	missing := pgtype.Timestamptz{}
	for _, tc := range []struct {
		name              string
		priorTarget       string
		priorRevision     pgtype.Timestamptz
		requestedTarget   string
		requestedRevision string
		want              bool
	}{
		{name: "ordinary upload without replacement metadata", priorRevision: missing, want: true},
		{name: "replacement request cannot replay ordinary upload", requestedTarget: "target-a", requestedRevision: stored.Format(time.RFC3339Nano)},
		{name: "ordinary request cannot replay replacement upload", priorTarget: "target-a", priorRevision: valid, requestedRevision: stored.Format(time.RFC3339Nano)},
		{name: "replacement with matching target and revision", priorTarget: "target-a", priorRevision: valid, requestedTarget: "target-a", requestedRevision: stored.Format(time.RFC3339Nano), want: true},
		{name: "legacy target without stored revision", priorTarget: "target-a", priorRevision: missing, requestedTarget: "target-a", requestedRevision: stored.Format(time.RFC3339Nano)},
		{name: "stored revision without target", priorRevision: valid, requestedRevision: stored.Format(time.RFC3339Nano)},
		{name: "different target", priorTarget: "target-a", priorRevision: valid, requestedTarget: "target-b", requestedRevision: stored.Format(time.RFC3339Nano)},
		{name: "different revision", priorTarget: "target-a", priorRevision: valid, requestedTarget: "target-a", requestedRevision: stored.Add(time.Second).Format(time.RFC3339Nano)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := replacementReplayMatches(tc.priorTarget, tc.priorRevision, tc.requestedTarget, tc.requestedRevision); got != tc.want {
				t.Fatalf("replacementReplayMatches(target=%q, revision_valid=%t, requested_target=%q, requested_revision=%q)=%t want %t", tc.priorTarget, tc.priorRevision.Valid, tc.requestedTarget, tc.requestedRevision, got, tc.want)
			}
		})
	}
}
