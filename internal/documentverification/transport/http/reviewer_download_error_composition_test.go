package http

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerDownloadPartialResponseStore struct {
	content       []byte
	err           error
	identityReads int
	spaceReads    int
	ownerID       string
	caseID        string
	fileID        string
}

func (s *reviewerDownloadPartialResponseStore) ReadIdentityDocument(_ context.Context, ownerID, caseID, fileID string) ([]byte, error) {
	s.identityReads++
	s.ownerID, s.caseID, s.fileID = ownerID, caseID, fileID
	return append([]byte(nil), s.content...), s.err
}

func (s *reviewerDownloadPartialResponseStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.spaceReads++
	return nil, dv.ErrNotFound
}

func TestReviewerIdentityDownloadPartialStorageErrorStaysNeutralAtHTTPBoundary(t *testing.T) {
	const privateBytes = "synthetic partial private reviewer file"
	const privateErrText = "synthetic backend diagnostic must not escape"
	const subjectID = "77777777-7777-4777-8777-777777777777"
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	store := &reviewerDownloadPartialResponseStore{
		content: []byte(privateBytes), err: errors.New(privateErrText),
	}
	repository := &reviewerDetailRouteRepository{
		snapshot: dv.Snapshot{
			ID: reviewerID, SubjectID: subjectID,
			Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 4,
			Files: []dv.File{{
				ID: reviewerFile, EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean,
				Size: int64(len(privateBytes)), MIME: "image/png", ContentSHA256: sha256.Sum256([]byte(privateBytes)),
				RetentionAt: now.Add(time.Hour),
			}},
		},
		authorization: dv.Authorization{
			ActorID: reviewerActor,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: subjectID,
			}},
		},
	}
	cases, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repository, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return now })
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))
	path := reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, reviewerReq(http.MethodGet, path, ""))

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d body=%q, want neutral 503", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), privateBytes) || strings.Contains(response.Body.String(), privateErrText) ||
		response.Header().Get("Content-Disposition") != "" || response.Header().Get("Content-Length") != "" ||
		response.Header().Get("ETag") != "" {
		t.Fatalf("partial download or diagnostic escaped: headers=%v body=%q", response.Header(), response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("download error security headers=%v", response.Header())
	}
	if store.identityReads != 1 || store.spaceReads != 0 || store.ownerID != subjectID || store.caseID != reviewerID ||
		store.fileID != reviewerFile || len(repository.actors) != 1 ||
		repository.actors[0] != reviewerActor || len(repository.caseIDs) != 1 || repository.caseIDs[0] != reviewerID {
		t.Fatalf("repository scope=%v/%v store scope=%q/%q/%q reads=%d/%d, want one reviewer-scoped identity read", repository.actors, repository.caseIDs, store.ownerID, store.caseID, store.fileID, store.identityReads, store.spaceReads)
	}
}
