package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
	"time"
)

type reviewerAccessRepo struct {
	x                Snapshot
	a                Authorization
	list             []Snapshot
	readCalls        int
	revokeAfterFirst bool
}

func (r *reviewerAccessRepo) ListReviewerCases(context.Context, string) ([]Snapshot, error) {
	if r.list != nil {
		return r.list, nil
	}
	return []Snapshot{r.x}, nil
}
func (r *reviewerAccessRepo) LoadReviewerCase(context.Context, string, string) (Snapshot, error) {
	return r.x, nil
}
func (r *reviewerAccessRepo) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, Snapshot, Authorization) error) error {
	r.readCalls++
	authorization := r.a
	if r.revokeAfterFirst && r.readCalls > 1 {
		authorization.ReviewerGrants = nil
	}
	return read(ctx, r.x, authorization)
}

type reviewerAccessStore struct {
	content                      []byte
	owner, space, caseID, fileID string
	identityReads, spaceReads    int
}

func (s *reviewerAccessStore) ReadIdentityDocument(_ context.Context, o, c, f string) ([]byte, error) {
	s.identityReads++
	s.owner, s.caseID, s.fileID = o, c, f
	return s.content, nil
}
func (s *reviewerAccessStore) ReadSpaceDocument(_ context.Context, o, sp, c, f string) ([]byte, error) {
	s.spaceReads++
	s.owner, s.space, s.caseID, s.fileID = o, sp, c, f
	return s.content, nil
}
func TestReviewerAccessDownloadRequiresExactCurrentGrantAndIntegrity(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	cid := "99999999-9999-4999-8999-999999999999"
	fid := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	body := []byte("clean synthetic PDF")
	x := Snapshot{ID: cid, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview, Files: []File{{ID: fid, State: FileClean, Size: int64(len(body)), MIME: "application/pdf", ContentSHA256: sha256.Sum256(body), RetentionAt: time.Now().Add(time.Hour)}}}
	repo := &reviewerAccessRepo{x: x, a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}}}
	store := &reviewerAccessStore{content: body}
	cases, _ := NewService(&testRepository{})
	svc, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Download(context.Background(), actor, cid, fid)
	if err != nil || string(got.Content) != string(body) || store.owner != subject {
		t.Fatalf("download=%q store=%+v err=%v", got.Content, store, err)
	}
	repo.a.ReviewerGrants[0].ScopeID = "88888888-8888-4888-8888-888888888888"
	if _, err = svc.Download(context.Background(), actor, cid, fid); err == nil {
		t.Fatal("wrong exact scope allowed")
	}
	repo.a.ReviewerGrants[0].ScopeID = subject
	repo.a.ReviewerGrants = append(repo.a.ReviewerGrants, ReviewerGrant{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: cid})
	if _, err = svc.Download(context.Background(), actor, cid, fid); err == nil {
		t.Fatal("dual capability allowed")
	}
	repo.x.SubjectID = actor
	repo.a.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: actor}}
	store.owner = ""
	if _, err = svc.Download(context.Background(), actor, cid, fid); err == nil || store.owner != "" {
		t.Fatalf("self-review downloaded evidence: owner=%q err=%v", store.owner, err)
	}
}

func TestReviewerAccessDownloadRejectsNonCleanEvidenceBeforeStoreRead(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	caseID := "99999999-9999-4999-8999-999999999999"
	fileID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	body := []byte("synthetic private identity evidence")
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
			Files: []File{{
				ID: fileID, State: FilePending, Size: int64(len(body)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(body), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
			}},
		},
		a: Authorization{
			ActorID: actor,
			ReviewerGrants: []ReviewerGrant{{
				ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject,
			}},
		},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{content: body}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	for _, fileState := range []FileState{FilePending, FileBlocked, FileError} {
		t.Run(string(fileState), func(t *testing.T) {
			repo.x.Files[0].State = fileState
			got, err := access.Download(context.Background(), actor, caseID, fileID)
			if !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || got.MIME != "" || store.owner != "" || store.identityReads != 0 || store.spaceReads != 0 {
				t.Fatalf("non-clean evidence was exposed: content=%q mime=%q store=%+v err=%v", got.Content, got.MIME, store, err)
			}
		})
	}
}

func TestReviewerAccessCannotDownloadReplacedIdentityFile(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	caseID := "99999999-9999-4999-8999-999999999999"
	oldFileID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	currentFileID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	body := []byte("synthetic replacement identity evidence")
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview, Revision: 3,
			IdentityDeclarationRevisionID: "88888888-8888-4888-8888-888888888888",
			DocumentFormatDeclared:        DocumentFormatNew,
			CaptureConsentVersion:         IdentityCaptureConsentVersion,
			CaptureConsentedAt:            time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
			Files: []File{
				{
					ID: currentFileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean,
					Size: int64(len(body)), MIME: "image/png", ContentSHA256: sha256.Sum256(body),
					RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean,
					Size: int64(len("synthetic identity back")), MIME: "image/png", ContentSHA256: sha256.Sum256([]byte("synthetic identity back")),
					RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean,
					Size: int64(len("synthetic identity selfie")), MIME: "image/jpeg", ContentSHA256: sha256.Sum256([]byte("synthetic identity selfie")),
					RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		a: Authorization{
			ActorID: actor,
			ReviewerGrants: []ReviewerGrant{{
				ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject,
			}},
		},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{content: body}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	got, err := access.Download(context.Background(), actor, caseID, oldFileID)
	if !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || got.MIME != "" || store.owner != "" || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("replaced evidence remained available: content=%q mime=%q store=%+v err=%v", got.Content, got.MIME, store, err)
	}
	got, err = access.Download(context.Background(), actor, caseID, currentFileID)
	if err != nil || string(got.Content) != string(body) || got.MIME != "image/png" || store.owner != subject || store.identityReads != 1 {
		t.Fatalf("current clean evidence unavailable: content=%q mime=%q store=%+v err=%v", got.Content, got.MIME, store, err)
	}
}

func TestReviewerAccessRejectsRevokedIdentityCaseBeforeStoreRead(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	caseID := "99999999-9999-4999-8999-999999999999"
	fileID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	body := []byte("synthetic revoked identity evidence")
	revokedAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseRevoked,
			Revision: 5, RevokedAt: &revokedAt,
			DocumentFormatDeclared: DocumentFormatNew,
			DocumentFormatObserved: DocumentFormatNew,
			CaptureConsentVersion:  IdentityCaptureConsentVersion,
			CaptureConsentedAt:     time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			DocumentExpiresAt:      time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
			VerifiedAt:             time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			VerifiedUntil:          time.Date(2027, 10, 1, 12, 0, 0, 0, time.UTC),
			Files: []File{
				{
					ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean,
					Size: int64(len(body)), MIME: "image/png", ContentSHA256: sha256.Sum256(body),
					RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean,
					Size: int64(len("synthetic revoked identity back")), MIME: "image/png", ContentSHA256: sha256.Sum256([]byte("synthetic revoked identity back")),
					RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean,
					Size: int64(len("synthetic revoked identity selfie")), MIME: "image/jpeg", ContentSHA256: sha256.Sum256([]byte("synthetic revoked identity selfie")),
					RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		a: Authorization{
			ActorID: actor,
			ReviewerGrants: []ReviewerGrant{{
				ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject,
			}},
		},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{content: body}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	got, err := access.Download(context.Background(), actor, caseID, fileID)
	if !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || got.MIME != "" ||
		store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("revoked identity evidence remained available: content=%q mime=%q store=%+v err=%v", got.Content, got.MIME, store, err)
	}
}

func TestReviewerAccessSpaceDownloadUsesAssignedSpaceScopeAndStore(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	spaceID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	caseID := "99999999-9999-4999-8999-999999999999"
	fileID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	body := []byte("synthetic space evidence")
	store := &reviewerAccessStore{content: body}
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, SpaceID: spaceID, Purpose: PurposeSpace, State: CaseInReview,
			Files: []File{{ID: fileID, State: FileClean, Size: int64(len(body)), MIME: "application/pdf", ContentSHA256: sha256.Sum256(body), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)}},
		},
		a: Authorization{
			ActorID: actor, SpaceOwnerID: subject,
			ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: spaceID}},
		},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	got, err := access.Download(context.Background(), actor, caseID, fileID)
	if err != nil || string(got.Content) != string(body) || got.MIME != "application/pdf" {
		t.Fatalf("space download=%+v err=%v", got, err)
	}
	if store.owner != subject || store.space != spaceID || store.caseID != caseID || store.fileID != fileID {
		t.Fatalf("space store scope=(%q,%q,%q,%q)", store.owner, store.space, store.caseID, store.fileID)
	}
}

func TestReviewerAccessDownloadRejectsUnknownPurposeBeforeReadingStore(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	caseID := "99999999-9999-4999-8999-999999999999"
	fileID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	body := []byte("synthetic evidence")
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: Purpose("unsupported"), State: CaseInReview,
			Files: []File{{ID: fileID, State: FileClean, Size: int64(len(body)), MIME: "image/png", ContentSHA256: sha256.Sum256(body), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)}},
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{content: body}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	if got, err := access.Download(context.Background(), actor, caseID, fileID); !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || store.owner != "" {
		t.Fatalf("unsupported-purpose evidence was exposed: content=%q store=%+v err=%v", got.Content, store, err)
	}
}

func TestReviewerQueueContainsOnlyReviewableAssignedCases(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	repo := &reviewerAccessRepo{list: []Snapshot{
		{ID: "11111111-1111-4111-8111-111111111111", SubjectID: subject, Purpose: PurposeIdentity, State: CaseSubmitted},
		{ID: "22222222-2222-4222-8222-222222222222", SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview},
		{ID: "33333333-3333-4333-8333-333333333333", SubjectID: subject, Purpose: PurposeIdentity, State: CaseNeedsEvidence},
		{ID: "44444444-4444-4444-8444-444444444444", SubjectID: subject, Purpose: PurposeIdentity, State: CaseDraft},
		{ID: "55555555-5555-4555-8555-555555555555", SubjectID: subject, Purpose: PurposeIdentity, State: CaseApproved},
	}}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessService(repo, cases, &reviewerAccessStore{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.List(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].State != CaseSubmitted || got[1].State != CaseInReview || got[2].State != CaseNeedsEvidence {
		t.Fatalf("reviewer queue states=%v, want only submitted/in-review/needs-evidence", got)
	}
}

func TestReviewerSpaceDetailDoesNotReadIdentityDeclaration(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	spaceID := "88888888-8888-4888-8888-888888888888"
	caseID := "99999999-9999-4999-8999-999999999999"
	x := Snapshot{ID: caseID, SubjectID: subject, SpaceID: spaceID, Purpose: PurposeSpace, State: CaseInReview, Revision: 3}
	repo := &reviewerAccessRepo{
		x: x,
		a: Authorization{ActorID: actor, SpaceOwnerID: subject, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: spaceID}}},
	}
	readerCalls := 0
	reader := ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		readerCalls++
		return ReviewerIdentityDeclaration{FirstName: "Synthetic Private", LastName: "Must Stay Hidden", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}, nil
	})
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, &reviewerAccessStore{}, reader)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.LoadDetail(context.Background(), actor, caseID)
	if err != nil || !reflect.DeepEqual(detail, ReviewerCaseDetail{Snapshot: x}) || readerCalls != 0 || repo.readCalls != 1 {
		t.Fatalf("space detail=%+v reader calls=%d repository reads=%d err=%v", detail, readerCalls, repo.readCalls, err)
	}
}

func TestReviewerDetailRejectsUnsupportedPurposeBeforeDeclarationRead(t *testing.T) {
	const (
		actor      = "66666666-6666-4666-8666-666666666666"
		subject    = "77777777-7777-4777-8777-777777777777"
		caseID     = "99999999-9999-4999-8999-999999999999"
		revisionID = "88888888-8888-4888-8888-888888888888"
	)
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: Purpose("unsupported"),
			State: CaseInReview, Revision: 7,
			IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{
			ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject,
		}}},
	}
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

	detail, err := service.LoadDetail(context.Background(), actor, caseID)
	if !errors.Is(err, ErrNotFound) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || declarationReads != 0 || repo.readCalls != 1 {
		t.Fatalf("unsupported-purpose detail=%+v err=%v declaration reads=%d repository reads=%d", detail, err, declarationReads, repo.readCalls)
	}
}

func TestReviewerIdentityDetailRequiresExactAssignmentAndRevalidatesIt(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	caseID := "99999999-9999-4999-8999-999999999999"
	revisionID := "88888888-8888-4888-8888-888888888888"
	want := ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Review Fixture", RUT: "11.222.333-9", DateOfBirth: "1990-02-03"}
	readerCalls := 0
	reader := ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
		readerCalls++
		if accountID != subject || id != revisionID {
			t.Fatalf("declaration read scope=(%q,%q)", accountID, id)
		}
		return want, nil
	})
	newService := func(repo *reviewerAccessRepo) *ReviewerAccessService {
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, &reviewerAccessStore{}, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}
	x := Snapshot{ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview, Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew}
	a := Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}}

	t.Run("assigned reviewer receives allowlisted declaration", func(t *testing.T) {
		readerCalls = 0
		repo := &reviewerAccessRepo{x: x, a: a}
		detail, err := newService(repo).LoadDetail(context.Background(), actor, caseID)
		if err != nil || detail.IdentityDeclaration == nil || *detail.IdentityDeclaration != want || readerCalls != 1 || repo.readCalls != 3 {
			t.Fatalf("detail=%+v reader=%d checks=%d err=%v", detail, readerCalls, repo.readCalls, err)
		}
	})
	t.Run("wrong scope does not read declaration", func(t *testing.T) {
		readerCalls = 0
		wrong := a
		wrong.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: caseID}}
		repo := &reviewerAccessRepo{x: x, a: wrong}
		if detail, err := newService(repo).LoadDetail(context.Background(), actor, caseID); err == nil || detail.IdentityDeclaration != nil || readerCalls != 0 {
			t.Fatalf("wrong-scope grant returned private detail: detail=%+v reads=%d err=%v", detail, readerCalls, err)
		}
	})
	t.Run("self review does not read declaration", func(t *testing.T) {
		readerCalls = 0
		self := x
		self.SubjectID = actor
		self.IdentityDeclarationRevisionID = revisionID
		selfGrant := Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: actor}}}
		repo := &reviewerAccessRepo{x: self, a: selfGrant}
		if detail, err := newService(repo).LoadDetail(context.Background(), actor, caseID); err == nil || detail.IdentityDeclaration != nil || readerCalls != 0 || repo.readCalls != 1 {
			t.Fatalf("self-review returned private declaration: detail=%+v reads=%d checks=%d err=%v", detail, readerCalls, repo.readCalls, err)
		}
	})
	t.Run("foreign or mixed grants do not read declaration", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			grants []ReviewerGrant
		}{
			{name: "no active grant", grants: nil},
			{name: "foreign reviewer", grants: []ReviewerGrant{{ReviewerID: subject, Capability: CapabilityIdentity, ScopeID: subject}}},
			{name: "wrong capability", grants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: subject}}},
			{name: "mixed capabilities", grants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}, {ReviewerID: actor, Capability: CapabilitySpace, ScopeID: caseID}}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				readerCalls = 0
				authorization := Authorization{ActorID: actor, ReviewerGrants: tc.grants}
				repo := &reviewerAccessRepo{x: x, a: authorization}
				if detail, err := newService(repo).LoadDetail(context.Background(), actor, caseID); err == nil || detail.IdentityDeclaration != nil || readerCalls != 0 || repo.readCalls != 1 {
					t.Fatalf("invalid reviewer grant returned private declaration: detail=%+v reads=%d checks=%d err=%v", detail, readerCalls, repo.readCalls, err)
				}
			})
		}
	})
	t.Run("revocation between authorization checks prevents declaration read", func(t *testing.T) {
		readerCalls = 0
		repo := &reviewerAccessRepo{x: x, a: a, revokeAfterFirst: true}
		if detail, err := newService(repo).LoadDetail(context.Background(), actor, caseID); !errors.Is(err, ErrNotFound) || detail.IdentityDeclaration != nil || readerCalls != 0 || repo.readCalls != 2 {
			t.Fatalf("revoked assignment returned private detail: detail=%+v reads=%d checks=%d err=%v", detail, readerCalls, repo.readCalls, err)
		}
	})
}

func TestReviewerIdentityDetailSuppressesDeclarationWhenLinkedRevisionChangesDuringRead(t *testing.T) {
	actor := "11111111-1111-4111-8111-111111111111"
	subject := "22222222-2222-4222-8222-222222222222"
	caseID := "33333333-3333-4333-8333-333333333333"
	revisionID := "44444444-4444-4444-8444-444444444444"
	replacementRevisionID := "55555555-5555-4555-8555-555555555555"
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
			Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}},
	}
	readerCalls := 0
	reader := ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
		readerCalls++
		if accountID != subject || id != revisionID {
			t.Fatalf("declaration read scope=(%q,%q)", accountID, id)
		}
		repo.x.IdentityDeclarationRevisionID = replacementRevisionID
		return ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Fixture", RUT: "11.222.333-9", DateOfBirth: "1990-02-03"}, nil
	})
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, &reviewerAccessStore{}, reader)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.LoadDetail(context.Background(), actor, caseID)
	if !errors.Is(err, ErrNotFound) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || readerCalls != 1 || repo.readCalls != 3 || repo.x.IdentityDeclarationRevisionID != replacementRevisionID {
		t.Fatalf("changed declaration linkage returned detail=%+v reader=%d checks=%d linkedRevision=%q err=%v", detail, readerCalls, repo.readCalls, repo.x.IdentityDeclarationRevisionID, err)
	}
}

func TestReviewerIdentityDetailFailsClosedWhenDeclarationReaderErrors(t *testing.T) {
	actor := "11111111-1111-4111-8111-111111111111"
	subject := "22222222-2222-4222-8222-222222222222"
	caseID := "33333333-3333-4333-8333-333333333333"
	revisionID := "44444444-4444-4444-8444-444444444444"
	wantDeclaration := ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Review Fixture", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
			Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}},
	}
	store := &reviewerAccessStore{}
	newService := func(reader ReviewerIdentityDeclarationReader) *ReviewerAccessService {
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}

	control, err := newService(ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
		if accountID != subject || id != revisionID {
			t.Fatalf("control declaration read scope=(%q,%q)", accountID, id)
		}
		return wantDeclaration, nil
	})).LoadDetail(context.Background(), actor, caseID)
	if err != nil || control.IdentityDeclaration == nil || *control.IdentityDeclaration != wantDeclaration || repo.readCalls != 3 {
		t.Fatalf("valid control fixture detail=%+v checks=%d err=%v", control, repo.readCalls, err)
	}

	repo.readCalls = 0
	readerCalls := 0
	detail, err := newService(ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
		readerCalls++
		if accountID != subject || id != revisionID {
			t.Fatalf("failing declaration read scope=(%q,%q)", accountID, id)
		}
		return wantDeclaration, errors.New("synthetic declaration reader failure")
	})).LoadDetail(context.Background(), actor, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || readerCalls != 1 || repo.readCalls != 2 || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("reader failure exposed detail=%+v reader=%d checks=%d store=%+v err=%v", detail, readerCalls, repo.readCalls, store, err)
	}
}

func TestReviewerIdentityDetailFailsClosedWhenDeclarationReaderCancelsAfterReturningData(t *testing.T) {
	const (
		actorID    = "11111111-1111-4111-8111-111111111111"
		subjectID  = "22222222-2222-4222-8222-222222222222"
		caseID     = "33333333-3333-4333-8333-333333333333"
		revisionID = "44444444-4444-4444-8444-444444444444"
	)
	declaration := ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Review Fixture", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview,
			Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actorID, ReviewerGrants: []ReviewerGrant{{ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID}}},
	}
	store := &reviewerAccessStore{}
	newService := func(reader ReviewerIdentityDeclarationReader) *ReviewerAccessService {
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}

	control, err := newService(ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		return declaration, nil
	})).LoadDetail(context.Background(), actorID, caseID)
	if err != nil || control.IdentityDeclaration == nil || *control.IdentityDeclaration != declaration || repo.readCalls != 3 {
		t.Fatalf("valid control fixture detail=%+v checks=%d err=%v", control, repo.readCalls, err)
	}

	repo.readCalls = 0
	ctx, cancel := context.WithCancel(context.Background())
	readerCalls := 0
	detail, err := newService(ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
		readerCalls++
		if accountID != subjectID || id != revisionID {
			t.Fatalf("declaration read scope=(%q,%q)", accountID, id)
		}
		cancel()
		return declaration, nil
	})).LoadDetail(ctx, actorID, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || readerCalls != 1 || repo.readCalls != 2 || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("canceled declaration read exposed detail=%+v reader=%d checks=%d store=%+v err=%v", detail, readerCalls, repo.readCalls, store, err)
	}
}

func TestReviewerIdentityDetailFailsClosedWhenContextCancelsAfterRevalidation(t *testing.T) {
	const (
		actorID    = "11111111-1111-4111-8111-111111111111"
		subjectID  = "22222222-2222-4222-8222-222222222222"
		caseID     = "33333333-3333-4333-8333-333333333333"
		revisionID = "44444444-4444-4444-8444-444444444444"
	)
	declaration := ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Review Fixture", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview,
			Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actorID, ReviewerGrants: []ReviewerGrant{{ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID}}},
	}
	store := &reviewerAccessStore{}
	reader := ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		return declaration, nil
	})
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	control, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
	if err != nil {
		t.Fatal(err)
	}
	if detail, err := control.LoadDetail(context.Background(), actorID, caseID); err != nil || detail.IdentityDeclaration == nil || *detail.IdentityDeclaration != declaration || repo.readCalls != 3 {
		t.Fatalf("valid control fixture detail=%+v checks=%d err=%v", detail, repo.readCalls, err)
	}

	repo.readCalls = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancelRepo := &reviewerAccessCancelAfterCallbackRepo{reviewerAccessRepo: repo, cancel: cancel, cancelAfterCall: 2}
	service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(cancelRepo, cases, store, reader)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.LoadDetail(ctx, actorID, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || repo.readCalls != 2 || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("canceled revalidation exposed detail=%+v checks=%d store=%+v err=%v", detail, repo.readCalls, store, err)
	}
}

func TestReviewerIdentityDetailRejectsIncompleteDeclarationFields(t *testing.T) {
	actor := "11111111-1111-4111-8111-111111111111"
	subject := "22222222-2222-4222-8222-222222222222"
	caseID := "33333333-3333-4333-8333-333333333333"
	revisionID := "44444444-4444-4444-8444-444444444444"
	validDeclaration := ReviewerIdentityDeclaration{FirstName: "Synthetic", LastName: "Review Fixture", RUT: "synthetic-rut", DateOfBirth: "synthetic-date"}
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
			Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}},
	}
	store := &reviewerAccessStore{}
	newService := func(reader ReviewerIdentityDeclarationReader) *ReviewerAccessService {
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}

	control, err := newService(ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		return validDeclaration, nil
	})).LoadDetail(context.Background(), actor, caseID)
	if err != nil || control.IdentityDeclaration == nil || *control.IdentityDeclaration != validDeclaration || repo.readCalls != 3 {
		t.Fatalf("valid control fixture detail=%+v checks=%d err=%v", control, repo.readCalls, err)
	}

	cases := []struct {
		name string
		omit func(*ReviewerIdentityDeclaration)
	}{
		{name: "first name", omit: func(d *ReviewerIdentityDeclaration) { d.FirstName = "" }},
		{name: "last name", omit: func(d *ReviewerIdentityDeclaration) { d.LastName = "" }},
		{name: "RUT", omit: func(d *ReviewerIdentityDeclaration) { d.RUT = "" }},
		{name: "date of birth", omit: func(d *ReviewerIdentityDeclaration) { d.DateOfBirth = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo.readCalls = 0
			store.identityReads, store.spaceReads = 0, 0
			readerCalls := 0
			incomplete := validDeclaration
			tc.omit(&incomplete)
			service := newService(ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
				readerCalls++
				if accountID != subject || id != revisionID {
					t.Fatalf("declaration read scope=(%q,%q)", accountID, id)
				}
				return incomplete, nil
			}))
			detail, err := service.LoadDetail(context.Background(), actor, caseID)
			if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || readerCalls != 1 || repo.readCalls != 2 || store.identityReads != 0 || store.spaceReads != 0 {
				t.Fatalf("incomplete declaration exposed detail=%+v reader=%d checks=%d store=%+v err=%v", detail, readerCalls, repo.readCalls, store, err)
			}
		})
	}
}

type reviewerIdentityDecisionRepo struct {
	x     Snapshot
	a     Authorization
	audit TransitionAudit
}

func (r *reviewerIdentityDecisionRepo) CreateCase(context.Context, string, Purpose, string, func(Authorization) (Snapshot, error)) (Snapshot, error) {
	return Snapshot{}, ErrInvalid
}
func (r *reviewerIdentityDecisionRepo) Transition(_ context.Context, actor string, purpose Purpose, id string, rev uint64, audit TransitionAudit, change func(*Snapshot, Authorization) error) error {
	r.audit = audit
	if actor != r.a.ActorID || purpose != r.x.Purpose || id != r.x.ID || rev != r.x.Revision {
		return ErrConflict
	}
	return change(&r.x, r.a)
}
func (r *reviewerIdentityDecisionRepo) ListReviewerCases(context.Context, string) ([]Snapshot, error) {
	return []Snapshot{r.x}, nil
}
func (r *reviewerIdentityDecisionRepo) LoadReviewerCase(context.Context, string, string) (Snapshot, error) {
	return r.x, nil
}
func (r *reviewerIdentityDecisionRepo) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, Snapshot, Authorization) error) error {
	return read(ctx, r.x, r.a)
}

func TestReviewerAccessReviewIdentityPersistsObservedDecision(t *testing.T) {
	for _, declaredFormat := range []DocumentFormat{DocumentFormatNew, DocumentFormatUnsure} {
		t.Run(string(declaredFormat), func(t *testing.T) {
			actor := "66666666-6666-4666-8666-666666666666"
			subject := "77777777-7777-4777-8777-777777777777"
			cid := "99999999-9999-4999-8999-999999999999"
			files := []File{
				{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 4, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			}
			repo := &reviewerIdentityDecisionRepo{x: Snapshot{ID: cid, SubjectID: subject, Purpose: PurposeIdentity, State: CaseSubmitted, Revision: 4, IdentityDeclarationRevisionID: "88888888-8888-4888-8888-888888888888", DocumentFormatDeclared: declaredFormat, CaptureConsentVersion: IdentityCaptureConsentVersion, CaptureConsentedAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), Files: files}, a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}}}
			cases, err := NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			cases.SetClock(func() time.Time { return time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC) })
			store := &reviewerAccessStore{}
			access, err := NewReviewerAccessService(repo, cases, store)
			if err != nil {
				t.Fatal(err)
			}
			observedExpiry := time.Date(2030, 5, 1, 0, 0, 0, 0, time.UTC)
			checklist := passedIdentityReviewChecklist()
			if err := access.ReviewIdentity(context.Background(), actor, cid, DecisionApprove, "", DocumentFormatLegacy, observedExpiry, checklist, time.Time{}, 4); err != nil {
				t.Fatal(err)
			}
			if repo.x.DocumentFormatDeclared != declaredFormat || repo.x.DocumentFormatObserved != DocumentFormatLegacy ||
				!repo.x.DocumentExpiresAt.Equal(observedExpiry) || !repo.x.VerifiedUntil.Equal(observedExpiry) || repo.x.State != CaseApproved ||
				repo.audit.Action != "review_decision" || repo.audit.ResultCode != string(CaseApproved) || repo.audit.ReasonCode != "" ||
				repo.audit.DocumentFormatObserved != DocumentFormatLegacy || !repo.audit.DocumentExpiresAtObserved.Equal(observedExpiry) ||
				!reflect.DeepEqual(repo.audit.IdentityReviewChecklist, checklist) {
				t.Fatalf("observed decision not persisted: case=%+v audit=%+v", repo.x, repo.audit)
			}
		})
	}
}

type testRepository struct{}

func (*testRepository) CreateCase(context.Context, string, Purpose, string, func(Authorization) (Snapshot, error)) (Snapshot, error) {
	return Snapshot{}, ErrInvalid
}
func (*testRepository) Transition(context.Context, string, Purpose, string, uint64, TransitionAudit, func(*Snapshot, Authorization) error) error {
	return ErrInvalid
}

func TestReviewerAccessServiceConstructorAndClockBoundaries(t *testing.T) {
	repo := &reviewerAccessRepo{}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{}

	// 1. NewReviewerAccessService nil checks
	if _, err := NewReviewerAccessService(nil, cases, store); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil repository, got %v", err)
	}
	if _, err := NewReviewerAccessService(repo, nil, store); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil cases, got %v", err)
	}
	if _, err := NewReviewerAccessService(repo, cases, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil store, got %v", err)
	}

	svc, err := NewReviewerAccessService(repo, cases, store)
	if err != nil || svc == nil {
		t.Fatalf("unexpected error creating ReviewerAccessService: svc=%v, err=%v", svc, err)
	}
	if svc.clock == nil || svc.clock().Location() != time.UTC {
		t.Fatalf("expected default initialized clock in UTC, got %v", svc.clock())
	}

	// 2. NewReviewerAccessServiceWithIdentityDeclarationReader boundaries
	reader := ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, _, _ string) (ReviewerIdentityDeclaration, error) {
		return ReviewerIdentityDeclaration{}, nil
	})
	if _, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for nil reader, got %v", err)
	}
	if _, err := NewReviewerAccessServiceWithIdentityDeclarationReader(nil, cases, store, reader); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid when base constructor fails, got %v", err)
	}

	svcWithReader, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
	if err != nil || svcWithReader == nil || svcWithReader.declarations == nil {
		t.Fatalf("unexpected error creating svcWithReader: svc=%v, err=%v", svcWithReader, err)
	}

	// 3. SetClock boundaries
	var nilSvc *ReviewerAccessService
	nilSvc.SetClock(func() time.Time { return time.Time{} }) // safe no-op

	svc.SetClock(nil) // safe no-op
	fixedTime := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return fixedTime })
	if got := svc.clock(); !got.Equal(fixedTime) {
		t.Fatalf("SetClock did not update clock, got %v, want %v", got, fixedTime)
	}
}

func TestReviewerAccessServiceReviewAndRevokeBoundaries(t *testing.T) {
	actor := "66666666-6666-4666-8666-666666666666"
	subject := "77777777-7777-4777-8777-777777777777"
	cid := "99999999-9999-4999-8999-999999999999"

	// 1. Nil receiver boundaries
	var nilSvc *ReviewerAccessService
	if err := nilSvc.Review(context.Background(), actor, cid, CapabilityIdentity, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil receiver Review expected ErrInvalid, got %v", err)
	}
	if err := nilSvc.ReviewIdentity(context.Background(), actor, cid, DecisionReject, ReasonDocumentInvalid, DocumentFormatLegacy, time.Time{}, nil, time.Time{}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil receiver ReviewIdentity expected ErrInvalid, got %v", err)
	}
	if err := nilSvc.Revoke(context.Background(), actor, cid, PurposeIdentity, ReasonDocumentInvalid, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil receiver Revoke expected ErrInvalid, got %v", err)
	}

	repo := &reviewerIdentityDecisionRepo{
		x: Snapshot{
			ID: cid, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview, Revision: 1,
			Files: []File{
				{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
			},
		},
		a: Authorization{
			ActorID:        actor,
			ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}},
		},
	}
	cases, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{}
	svc, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Invalid ID validation boundaries
	if err := svc.Review(context.Background(), actor, "invalid-uuid", CapabilityIdentity, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Review with invalid ID expected ErrInvalid, got %v", err)
	}
	if err := svc.ReviewIdentity(context.Background(), actor, "invalid-uuid", DecisionReject, ReasonDocumentInvalid, DocumentFormatLegacy, time.Time{}, nil, time.Time{}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReviewIdentity with invalid ID expected ErrInvalid, got %v", err)
	}
	if err := svc.Revoke(context.Background(), actor, "invalid-uuid", PurposeIdentity, ReasonDocumentInvalid, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Revoke with invalid ID expected ErrInvalid, got %v", err)
	}

	// 3. Delegation execution: Review
	if err := svc.Review(context.Background(), actor, cid, CapabilityIdentity, DecisionReject, ReasonDocumentInvalid, time.Time{}, 1); err != nil {
		t.Fatalf("expected Review to succeed and delegate to cases, got %v", err)
	}
	if repo.x.State != CaseRejected || repo.audit.Action != "review_decision" {
		t.Fatalf("expected case rejected by Review delegation, got %+v", repo.x)
	}

	// 3b. Delegation execution: ReviewIdentity
	repo.x.State = CaseInReview
	repo.x.Revision = 2
	if err := svc.ReviewIdentity(context.Background(), actor, cid, DecisionReject, ReasonDocumentInvalid, DocumentFormatLegacy, time.Time{}, nil, time.Time{}, 2); err != nil {
		t.Fatalf("expected ReviewIdentity to succeed and delegate to cases, got %v", err)
	}
	if repo.x.State != CaseRejected || repo.audit.Action != "review_decision" {
		t.Fatalf("expected case rejected by ReviewIdentity delegation, got %+v", repo.x)
	}

	// 4. Delegation execution: Revoke on approved case
	repo.x.State = CaseApproved
	repo.x.Revision = 3
	if err := svc.Revoke(context.Background(), actor, cid, PurposeIdentity, ReasonDocumentInvalid, 3); err != nil {
		t.Fatalf("expected Revoke to succeed and delegate to cases, got %v", err)
	}
	if repo.x.State != CaseRevoked || repo.audit.Action != "revoked" {
		t.Fatalf("expected case revoked by Revoke delegation, got %+v", repo.x)
	}
}
