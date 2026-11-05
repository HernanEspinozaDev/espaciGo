package documentverification

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"
)

type reviewerAccessRepo struct {
	x Snapshot
	a Authorization
}

func (r *reviewerAccessRepo) ListReviewerCases(context.Context, string) ([]Snapshot, error) {
	return []Snapshot{r.x}, nil
}
func (r *reviewerAccessRepo) LoadReviewerCase(context.Context, string, string) (Snapshot, error) {
	return r.x, nil
}
func (r *reviewerAccessRepo) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, Snapshot, Authorization) error) error {
	return read(ctx, r.x, r.a)
}

type reviewerAccessStore struct {
	content                      []byte
	owner, space, caseID, fileID string
}

func (s *reviewerAccessStore) ReadIdentityDocument(_ context.Context, o, c, f string) ([]byte, error) {
	s.owner, s.caseID, s.fileID = o, c, f
	return s.content, nil
}
func (s *reviewerAccessStore) ReadSpaceDocument(_ context.Context, o, sp, c, f string) ([]byte, error) {
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
}

type testRepository struct{}

func (*testRepository) CreateCase(context.Context, string, Purpose, string, func(Authorization) (Snapshot, error)) (Snapshot, error) {
	return Snapshot{}, ErrInvalid
}
func (*testRepository) Transition(context.Context, string, Purpose, string, uint64, TransitionAudit, func(*Snapshot, Authorization) error) error {
	return ErrInvalid
}
