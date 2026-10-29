package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const routeActor = "66666666-6666-4666-8666-666666666666"

type routeCaseRepository struct {
	snapshot documentverification.Snapshot
	auth     documentverification.Authorization
}

func (r *routeCaseRepository) LoadSpaceCaseForDownload(_ context.Context, actor, caseID string) (documentverification.Snapshot, documentverification.Authorization, error) {
	return r.snapshot, r.auth, nil
}

type routePrivateStore struct {
	content []byte
	calls   int
}

func (s *routePrivateStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.calls++
	return append([]byte(nil), s.content...), nil
}

func TestOwnerDownloadHTTPRouteComposesPolicyAndPrivateStore(t *testing.T) {
	const owner = "66666666-6666-4666-8666-666666666666"
	const caseID = "99999999-9999-4999-8999-999999999999"
	const fileID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const spaceID = "88888888-8888-4888-8888-888888888888"
	const supportID = "77777777-7777-4777-8777-777777777777"
	content := []byte("synthetic space document")
	repo := &routeCaseRepository{
		snapshot: documentverification.Snapshot{
			Purpose: documentverification.PurposeSpace, ID: caseID, SubjectID: owner, SpaceID: spaceID,
			State: documentverification.CaseInReview,
			Files: []documentverification.File{{ID: fileID, State: documentverification.FileClean, Size: int64(len(content)), MIME: "application/pdf", Pages: 1}},
		},
		auth: documentverification.Authorization{ActorID: owner, SpaceOwnerID: owner},
	}
	store := &routePrivateStore{content: content}
	downloadService, err := documentverification.NewSpaceDownloadService(repo, store)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterOwnerDownloadRoute(mux, NewOwnerDownloadHandler(routeAuthenticator{principal: identity.Principal{AccountID: owner}}, downloadService))
	url := "/api/v1/document-verification/space-cases/" + caseID + "/files/" + fileID + "/content"
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer synthetic-session")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	res := request()
	if res.Code != http.StatusOK || res.Body.String() != string(content) || store.calls != 1 {
		t.Fatalf("open clean case response=%d body=%q store_calls=%d", res.Code, res.Body.String(), store.calls)
	}

	// Closed cases and support actors must be denied before object access.
	repo.snapshot.State = documentverification.CaseApproved
	res = request()
	if res.Code != http.StatusNotFound || store.calls != 1 {
		t.Fatalf("closed case response=%d store_calls=%d", res.Code, store.calls)
	}
	repo.snapshot.State = documentverification.CaseInReview
	repo.auth.ActorID = supportID
	auth := routeAuthenticator{principal: identity.Principal{AccountID: supportID}}
	mux = http.NewServeMux()
	RegisterOwnerDownloadRoute(mux, NewOwnerDownloadHandler(auth, downloadService))
	res = request()
	if res.Code != http.StatusNotFound || store.calls != 1 {
		t.Fatalf("non-owner/support response=%d store_calls=%d", res.Code, store.calls)
	}
}

type routeAuthenticator struct {
	principal identity.Principal
	err       error
}

func (a routeAuthenticator) Authorize(_ context.Context, secret identity.Secret, role identity.Role, activity identity.ActivityKind) (identity.Principal, error) {
	if secret != identity.Secret("synthetic-session") || role != "" || activity != identity.UserOperation {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	return a.principal, a.err
}

type routeDownloader struct {
	result documentverification.DownloadedDocument
	err    error
	actor  string
	caseID string
	fileID string
}

func (d *routeDownloader) Download(_ context.Context, actor, caseID, fileID string) (documentverification.DownloadedDocument, error) {
	d.actor, d.caseID, d.fileID = actor, caseID, fileID
	return d.result, d.err
}

func TestOwnerDownloadRouteReturnsPrivateAttachmentForAuthorizedCleanFile(t *testing.T) {
	content := []byte("synthetic-document")
	downloader := &routeDownloader{result: documentverification.DownloadedDocument{Content: content, MIME: "application/pdf"}}
	mux := http.NewServeMux()
	RegisterOwnerDownloadRoute(mux, NewOwnerDownloadHandler(routeAuthenticator{principal: identity.Principal{AccountID: routeActor}}, downloader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content", nil)
	req.Header.Set("Authorization", "Bearer synthetic-session")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != string(content) {
		t.Fatalf("response=%d %q", res.Code, res.Body.String())
	}
	if res.Header().Get("Cache-Control") != "private, no-store" || res.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(res.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("unsafe download headers: %v", res.Header())
	}
	if res.Header().Get("Content-Type") != "application/pdf" || downloader.actor != routeActor || downloader.caseID == "" || downloader.fileID == "" {
		t.Fatalf("download route scope/type incorrect: headers=%v downloader=%+v", res.Header(), downloader)
	}
}

func TestOwnerDownloadRouteFailsClosedForDeniedOrUnavailableDocument(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "closed or unclean", err: documentverification.ErrFilesNotClean, want: http.StatusNotFound},
		{name: "storage failure", err: documentverification.ErrDocumentStorage, want: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downloader := &routeDownloader{err: tc.err}
			mux := http.NewServeMux()
			RegisterOwnerDownloadRoute(mux, NewOwnerDownloadHandler(routeAuthenticator{principal: identity.Principal{AccountID: routeActor}}, downloader))
			req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content", nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != tc.want || strings.Contains(res.Body.String(), "synthetic-document") {
				t.Fatalf("response=%d %q; want %d without bytes", res.Code, res.Body.String(), tc.want)
			}
		})
	}
}

func TestOwnerDownloadRouteRequiresAuthenticationAndRejectsQuery(t *testing.T) {
	downloader := &routeDownloader{result: documentverification.DownloadedDocument{Content: []byte("synthetic"), MIME: "image/png"}}
	mux := http.NewServeMux()
	RegisterOwnerDownloadRoute(mux, NewOwnerDownloadHandler(routeAuthenticator{principal: identity.Principal{AccountID: routeActor}}, downloader))
	for _, tc := range []struct {
		name string
		url  string
		auth string
		want int
	}{
		{name: "missing session", url: "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content", want: http.StatusUnauthorized},
		{name: "query denied", url: "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content?key=private", auth: "Bearer synthetic-session", want: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("response=%d want=%d body=%q", res.Code, tc.want, res.Body.String())
			}
		})
	}
	if downloader.actor != "" {
		t.Fatal("denied requests reached the download service")
	}
}

func TestOwnerDownloadRouteUnavailableHandlerFailsClosed(t *testing.T) {
	mux := http.NewServeMux()
	RegisterOwnerDownloadRoute(mux, NewOwnerDownloadHandler(routeAuthenticator{principal: identity.Principal{AccountID: routeActor}}, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content", nil)
	req.Header.Set("Authorization", "Bearer synthetic-session")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("response=%d want=%d", res.Code, http.StatusServiceUnavailable)
	}
}
