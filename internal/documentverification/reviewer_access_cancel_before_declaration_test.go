package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
)

type reviewerCancelAfterFirstReadRepository struct {
	*reviewerAccessRepo
	cancel context.CancelFunc
}

func (r *reviewerCancelAfterFirstReadRepository) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, Snapshot, Authorization) error) error {
	err := r.reviewerAccessRepo.WithReviewerCaseForRead(ctx, actor, id, read)
	if r.readCalls == 1 {
		r.cancel()
	}
	return err
}

func TestReviewerIdentityDetailSkipsDeclarationReadWhenContextCancelsAfterCaseRead(t *testing.T) {
	const (
		actor      = "66666666-6666-4666-8666-666666666666"
		subject    = "77777777-7777-4777-8777-777777777777"
		caseID     = "99999999-9999-4999-8999-999999999999"
		revisionID = "88888888-8888-4888-8888-888888888888"
	)
	base := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity,
			State: CaseInReview, Revision: 7,
			IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{
			ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject,
		}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &reviewerCancelAfterFirstReadRepository{reviewerAccessRepo: base, cancel: cancel}
	declarationReads := 0
	reader := ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		declarationReads++
		return ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Private", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}, nil
	})
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, &reviewerAccessStore{}, reader)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.LoadDetail(ctx, actor, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || ctx.Err() != context.Canceled || declarationReads != 0 || base.readCalls != 1 {
		t.Fatalf("detail=%+v err=%v ctx err=%v declaration reads=%d repository reads=%d", detail, err, ctx.Err(), declarationReads, base.readCalls)
	}
}

func TestReviewerIdentityDetailSkipsRepositoryReadWhenContextAlreadyCanceled(t *testing.T) {
	const (
		actor      = "66666666-6666-4666-8666-666666666666"
		subject    = "77777777-7777-4777-8777-777777777777"
		caseID     = "99999999-9999-4999-8999-999999999999"
		revisionID = "88888888-8888-4888-8888-888888888888"
	)
	base := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity,
			State: CaseInReview, Revision: 7,
			IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{
			ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject,
		}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &reviewerCancelAfterFirstReadRepository{reviewerAccessRepo: base, cancel: cancel}
	declarationReads := 0
	reader := ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		declarationReads++
		return ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Private", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}, nil
	})
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, &reviewerAccessStore{}, reader)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.LoadDetail(ctx, actor, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || declarationReads != 0 || base.readCalls != 0 {
		t.Fatalf("detail=%+v err=%v declaration reads=%d repository reads=%d", detail, err, declarationReads, base.readCalls)
	}
}

const (
	reviewerCancelDownloadActor   = "66666666-6666-4666-8666-666666666666"
	reviewerCancelDownloadSubject = "77777777-7777-4777-8777-777777777777"
	reviewerCancelDownloadCase    = "99999999-9999-4999-8999-999999999999"
	reviewerCancelDownloadFile    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type reviewerCancelBeforeCallbackRepository struct {
	*reviewerAccessRepo
	cancel context.CancelFunc
}

func (r *reviewerCancelBeforeCallbackRepository) WithReviewerCaseForRead(ctx context.Context, _, _ string, read func(context.Context, Snapshot, Authorization) error) error {
	r.readCalls++
	r.cancel()
	return read(ctx, r.x, r.a)
}

func reviewerCancelDownloadRepo() *reviewerAccessRepo {
	content := []byte("synthetic private identity evidence")
	return &reviewerAccessRepo{
		x: Snapshot{
			ID: reviewerCancelDownloadCase, SubjectID: reviewerCancelDownloadSubject,
			Purpose: PurposeIdentity, State: CaseInReview,
			Files: []File{{
				ID: reviewerCancelDownloadFile, State: FileClean,
				Size: int64(len(content)), MIME: "image/jpeg", ContentSHA256: sha256.Sum256(content),
			}},
		},
		a: Authorization{ActorID: reviewerCancelDownloadActor, ReviewerGrants: []ReviewerGrant{{
			ReviewerID: reviewerCancelDownloadActor, Capability: CapabilityIdentity, ScopeID: reviewerCancelDownloadSubject,
		}}},
	}
}

func reviewerCancelDownloadService(t *testing.T, repo ReviewerCaseReader) (*ReviewerAccessService, *reviewerAccessStore) {
	t.Helper()
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("synthetic private identity evidence")
	store := &reviewerAccessStore{content: content}
	service, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func TestReviewerDownloadSkipsRepositoryReadWhenContextAlreadyCanceled(t *testing.T) {
	base := reviewerCancelDownloadRepo()
	service, store := reviewerCancelDownloadService(t, base)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	download, err := service.Download(ctx, reviewerCancelDownloadActor, reviewerCancelDownloadCase, reviewerCancelDownloadFile)
	if !errors.Is(err, ErrDocumentStorage) || len(download.Content) != 0 || base.readCalls != 0 || store.identityReads != 0 {
		t.Fatalf("download=%+v err=%v repository reads=%d storage reads=%d", download, err, base.readCalls, store.identityReads)
	}
}

func TestReviewerDownloadSkipsStoreReadWhenContextCancelsBeforeReadCallback(t *testing.T) {
	base := reviewerCancelDownloadRepo()
	ctx, cancel := context.WithCancel(context.Background())
	repo := &reviewerCancelBeforeCallbackRepository{reviewerAccessRepo: base, cancel: cancel}
	service, store := reviewerCancelDownloadService(t, repo)

	download, err := service.Download(ctx, reviewerCancelDownloadActor, reviewerCancelDownloadCase, reviewerCancelDownloadFile)
	if !errors.Is(err, ErrDocumentStorage) || len(download.Content) != 0 || ctx.Err() != context.Canceled || base.readCalls != 1 || store.identityReads != 0 {
		t.Fatalf("download=%+v err=%v ctx err=%v repository reads=%d storage reads=%d", download, err, ctx.Err(), base.readCalls, store.identityReads)
	}
}
