package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerDownloadIntegrityRepository struct {
	snapshot dv.Snapshot
	auth     dv.Authorization
}

func (r *reviewerDownloadIntegrityRepository) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, nil
}

func (r *reviewerDownloadIntegrityRepository) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return r.snapshot, nil
}

func (r *reviewerDownloadIntegrityRepository) WithReviewerCaseForRead(ctx context.Context, _, _ string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	return read(ctx, r.snapshot, r.auth)
}

type reviewerDownloadIntegrityStore struct {
	content []byte
	reads   int
}

func (s *reviewerDownloadIntegrityStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.reads++
	return append([]byte(nil), s.content...), nil
}

func (s *reviewerDownloadIntegrityStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, nil
}

type reviewerDownloadIntegrityService struct {
	ReviewerCasesService
	access *dv.ReviewerAccessService
}

func (s reviewerDownloadIntegrityService) Download(ctx context.Context, actor, caseID, fileID string) (dv.DownloadedDocument, error) {
	return s.access.Download(ctx, actor, caseID, fileID)
}

type reviewerDownloadUnusedRepository struct{ dv.Repository }

func TestReviewerDownloadHandlerDoesNotReturnIntegrityMismatch(t *testing.T) {
	expected := []byte("synthetic expected identity evidence")
	stored := append([]byte(nil), expected...)
	stored[len(stored)-1] ^= 1
	subject := "77777777-7777-4777-8777-777777777777"
	repo := &reviewerDownloadIntegrityRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: subject, Purpose: dv.PurposeIdentity, State: dv.CaseInReview,
			Files: []dv.File{{
				ID: reviewerFile, State: dv.FileClean, Size: int64(len(expected)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(expected), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
			}},
		},
		auth: dv.Authorization{
			ActorID:        reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: subject}},
		},
	}
	store := &reviewerDownloadIntegrityStore{content: stored}
	caseService, err := dv.NewService(reviewerDownloadUnusedRepository{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, caseService, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	path := reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content"
	request := reviewerReq(http.MethodGet, path, "")
	response := httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, reviewerDownloadIntegrityService{access: access}).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || store.reads != 1 || response.Body.Len() == 0 {
		t.Fatalf("status=%d reads=%d body=%q", response.Code, store.reads, response.Body.String())
	}
	for _, leaked := range []string{string(stored), "image/png", "attachment"} {
		if bytes.Contains(response.Body.Bytes(), []byte(leaked)) {
			t.Fatalf("response leaked %q: %q", leaked, response.Body.String())
		}
	}
	contentType := strings.ToLower(response.Header().Get("Content-Type"))
	isBinaryDocumentType := strings.HasPrefix(contentType, "image/") || contentType == "application/pdf" || contentType == "application/octet-stream"
	if response.Header().Get("Content-Disposition") != "" || response.Header().Get("Content-Length") != "" || isBinaryDocumentType {
		t.Fatalf("download headers present on integrity failure: %v", response.Header())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("security headers=%v", response.Header())
	}
}
