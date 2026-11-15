package documentverificationpg

import (
	"context"
	"reflect"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerListAccessStore struct {
	reads int
}

func (s *reviewerListAccessStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.reads++
	return nil, dv.ErrNotFound
}

func (s *reviewerListAccessStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.reads++
	return nil, dv.ErrNotFound
}

func TestReviewerAccessListUsesRepositoryExactGrantResult(t *testing.T) {
	tests := []struct {
		name      string
		grant     dv.ReviewerGrant
		wantCases int
		wantFiles int
	}{
		{
			name: "exact identity grant reaches reviewer queue",
			grant: dv.ReviewerGrant{
				ReviewerID: reviewerListAdapterActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerListAdapterOwner,
			},
			wantCases: 1,
			wantFiles: 1,
		},
		{
			name: "different subject scope is filtered by adapter",
			grant: dv.ReviewerGrant{
				ReviewerID: reviewerListAdapterActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    "55555555-5555-4555-8555-555555555555",
			},
		},
		{
			name: "different reviewer is filtered by adapter",
			grant: dv.ReviewerGrant{
				ReviewerID: "66666666-6666-4666-8666-666666666667",
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerListAdapterOwner,
			},
		},
		{
			name: "different capability is filtered by adapter",
			grant: dv.ReviewerGrant{
				ReviewerID: reviewerListAdapterActor,
				Capability: dv.CapabilitySpace,
				ScopeID:    reviewerListAdapterOwner,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repository, tx := reviewerListAdapterService(t, []dv.ReviewerGrant{tc.grant})
			caseService, err := dv.NewService(repository)
			if err != nil {
				t.Fatal(err)
			}
			store := &reviewerListAccessStore{}
			access, err := dv.NewReviewerAccessService(repository, caseService, store)
			if err != nil {
				t.Fatal(err)
			}

			cases, err := access.List(context.Background(), reviewerListAdapterActor)
			if err != nil {
				t.Fatal(err)
			}
			if len(cases) != tc.wantCases || tx.fileQueryCalls != tc.wantFiles {
				t.Fatalf("cases=%d file reads=%d, want %d/%d", len(cases), tx.fileQueryCalls, tc.wantCases, tc.wantFiles)
			}
			if tx.candidateListCalls != 1 || tx.candidateReadCalls != 1 || tx.grantQueryCalls != 1 || !tx.committed {
				t.Fatalf("candidate-list/read/grant calls=%d/%d/%d committed=%v", tx.candidateListCalls, tx.candidateReadCalls, tx.grantQueryCalls, tx.committed)
			}
			wantCurrentDeclarationCalls := 0
			if tc.wantCases == 1 {
				wantCurrentDeclarationCalls = 1
			}
			if tx.currentDeclarationCalls != wantCurrentDeclarationCalls {
				t.Fatalf("current declaration checks=%d, want %d", tx.currentDeclarationCalls, wantCurrentDeclarationCalls)
			}
			if !reflect.DeepEqual(tx.candidateListArgs, []any{reviewerListAdapterActor}) ||
				!reflect.DeepEqual(tx.candidateReadArgs, []any{reviewerListAdapterCaseID, reviewerListAdapterActor}) ||
				!reflect.DeepEqual(tx.grantQueryArgs, []any{reviewerListAdapterActor}) {
				t.Fatalf("candidate args=%v reread args=%v grant args=%v", tx.candidateListArgs, tx.candidateReadArgs, tx.grantQueryArgs)
			}
			if store.reads != 0 {
				t.Fatalf("document store reads=%d, want none for queue listing", store.reads)
			}
			if tc.wantCases == 1 && (cases[0].ID != reviewerListAdapterCaseID || cases[0].SubjectID != reviewerListAdapterOwner) {
				t.Fatalf("returned reviewer case=%+v", cases[0])
			}
		})
	}
}
