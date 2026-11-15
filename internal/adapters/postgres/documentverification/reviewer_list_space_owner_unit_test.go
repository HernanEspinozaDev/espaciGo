package documentverificationpg

import (
	"context"
	"errors"
	"reflect"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func reviewerListAdapterSpaceSnapshotValues() []any {
	return []any{
		reviewerListAdapterCaseID, "space", reviewerListAdapterOwner, reviewerListAdapterSpaceID,
		"en_revision", int64(7), "", nil, nil, nil, nil, nil,
		"", "", "", nil, "",
	}
}

func TestListReviewerCasesRevalidatesSpaceOwnerBeforeReturningCandidate(t *testing.T) {
	lockErr := errors.New("synthetic space-owner lock failure")
	tests := []struct {
		name          string
		ownerMatches  bool
		lockErr       error
		wantCases     int
		wantErr       error
		wantCommitted bool
		wantRollback  int
	}{
		{
			name:          "owned space is returned after lock",
			ownerMatches:  true,
			wantCases:     1,
			wantCommitted: true,
			wantRollback:  1,
		},
		{
			name:          "owner mismatch omits candidate",
			wantCases:     0,
			wantCommitted: true,
			wantRollback:  1,
		},
		{
			name:         "lock error returns no candidate",
			lockErr:      lockErr,
			wantErr:      lockErr,
			wantRollback: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			grant := dv.ReviewerGrant{
				ReviewerID: reviewerListAdapterActor,
				Capability: dv.CapabilitySpace,
				ScopeID:    reviewerListAdapterSpaceID,
			}
			repository, tx := reviewerListAdapterService(t, []dv.ReviewerGrant{grant})
			tx.candidateSnapshot = reviewerListAdapterSpaceSnapshotValues()
			tx.spaceOwner = tc.ownerMatches
			tx.spaceOwnerErr = tc.lockErr

			cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || cases != nil {
					t.Fatalf("cases=%v err=%v, want nil cases and lock error %v", cases, err, tc.wantErr)
				}
			} else if err != nil || cases == nil || len(cases) != tc.wantCases {
				t.Fatalf("cases=%v err=%v, want %d cases and no error", cases, err, tc.wantCases)
			}

			if tx.spaceLockCalls != 1 || !reflect.DeepEqual(tx.spaceLockArgs, []any{reviewerListAdapterSpaceID, reviewerListAdapterOwner}) {
				t.Fatalf("space lock calls=%d args=%v, want one exact space/owner lock", tx.spaceLockCalls, tx.spaceLockArgs)
			}
			if tx.candidateListCalls != 1 || tx.candidateRows == nil || tx.candidateRows.closeCalls != 1 {
				t.Fatalf("candidate list calls=%d rows=%v, want one query and closed rows", tx.candidateListCalls, tx.candidateRows)
			}

			wantFurtherReads := 0
			if tc.ownerMatches && tc.lockErr == nil {
				wantFurtherReads = 1
			}
			if tx.candidateReadCalls != wantFurtherReads || tx.grantQueryCalls != wantFurtherReads || tx.fileQueryCalls != wantFurtherReads || tx.currentDeclarationCalls != 0 {
				t.Fatalf("candidate/grant/file/declaration reads=%d/%d/%d/%d, want %d/%d/%d/0",
					tx.candidateReadCalls, tx.grantQueryCalls, tx.fileQueryCalls, tx.currentDeclarationCalls,
					wantFurtherReads, wantFurtherReads, wantFurtherReads)
			}
			if tx.committed != tc.wantCommitted || tx.rollbackCalls != tc.wantRollback {
				t.Fatalf("committed=%v rollback calls=%d, want %v/%d", tx.committed, tx.rollbackCalls, tc.wantCommitted, tc.wantRollback)
			}
			if tc.wantCases == 1 && (cases[0].Purpose != dv.PurposeSpace || cases[0].SubjectID != reviewerListAdapterOwner || cases[0].SpaceID != reviewerListAdapterSpaceID) {
				t.Fatalf("returned case=%+v, want exact synthetic space and owner", cases[0])
			}
		})
	}
}
