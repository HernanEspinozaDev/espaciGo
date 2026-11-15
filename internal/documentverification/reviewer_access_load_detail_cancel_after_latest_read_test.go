package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type reviewerAccessCancelAfterLatestReadRepository struct {
	*reviewerAccessRepo
	cancel context.CancelFunc
}

func (r *reviewerAccessCancelAfterLatestReadRepository) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, Snapshot, Authorization) error) error {
	err := r.reviewerAccessRepo.WithReviewerCaseForRead(ctx, actor, id, read)
	if err == nil && r.reviewerAccessRepo.readCalls == 3 {
		r.cancel()
	}
	return err
}

func TestReviewerIdentityDetailFailsClosedWhenContextCancelsAfterLatestCaseRead(t *testing.T) {
	const (
		actorID    = "66666666-6666-4666-8666-666666666666"
		subjectID  = "77777777-7777-4777-8777-777777777777"
		caseID     = "99999999-9999-4999-8999-999999999999"
		revisionID = "88888888-8888-4888-8888-888888888888"
	)
	declaration := ReviewerIdentityDeclaration{
		FirstName:   "Synthetic",
		LastName:    "Identity",
		RUT:         "synthetic-rut",
		DateOfBirth: "1990-01-02",
	}
	newRepository := func() *reviewerAccessRepo {
		return &reviewerAccessRepo{
			x: Snapshot{
				ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity,
				State: CaseInReview, Revision: 7,
				IdentityDeclarationRevisionID: revisionID,
				DocumentFormatDeclared:        DocumentFormatNew,
			},
			a: Authorization{
				ActorID: actorID,
				ReviewerGrants: []ReviewerGrant{{
					ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID,
				}},
			},
		}
	}
	newService := func(t *testing.T, repository ReviewerCaseReader, declarationReads *int) *ReviewerAccessService {
		t.Helper()
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		reader := ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, gotRevisionID string) (ReviewerIdentityDeclaration, error) {
			*declarationReads++
			if accountID != subjectID || gotRevisionID != revisionID {
				t.Fatalf("declaration reader scope=(%q,%q)", accountID, gotRevisionID)
			}
			return declaration, nil
		})
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repository, cases, &reviewerAccessStore{}, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}

	controlRepository := newRepository()
	controlReads := 0
	control := newService(t, controlRepository, &controlReads)
	got, err := control.LoadDetail(context.Background(), actorID, caseID)
	if err != nil || got.IdentityDeclaration == nil || *got.IdentityDeclaration != declaration || controlRepository.readCalls != 3 || controlReads != 1 {
		t.Fatalf("valid control detail=%+v reads=%d/%d err=%v", got, controlRepository.readCalls, controlReads, err)
	}

	baseRepository := newRepository()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repository := &reviewerAccessCancelAfterLatestReadRepository{reviewerAccessRepo: baseRepository, cancel: cancel}
	declarationReads := 0
	service := newService(t, repository, &declarationReads)
	detail, err := service.LoadDetail(ctx, actorID, caseID)
	if !errors.Is(ctx.Err(), context.Canceled) || !errors.Is(err, ErrDocumentStorage) ||
		!reflect.DeepEqual(detail, ReviewerCaseDetail{}) || baseRepository.readCalls != 3 || declarationReads != 1 {
		t.Fatalf("canceled detail=%+v err=%v ctx err=%v repository reads=%d declaration reads=%d", detail, err, ctx.Err(), baseRepository.readCalls, declarationReads)
	}
}
