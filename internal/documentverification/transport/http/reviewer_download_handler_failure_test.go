package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerDownloadFailureDoesNotReturnDocumentBytes(t *testing.T) {
	const privateBytes = "synthetic private reviewer document"
	privateErr := errors.New("synthetic private backend detail")
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "hidden missing", err: dv.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "storage failure", err: dv.ErrDocumentStorage, wantStatus: http.StatusServiceUnavailable},
		{name: "internal failure", err: privateErr, wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &reviewerFake{
				d:   dv.DownloadedDocument{MIME: "application/pdf", Content: []byte(privateBytes)},
				err: tc.err,
			}
			path := reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content"
			w := httptest.NewRecorder()
			reviewerHandler(f, nil).ServeHTTP(w, reviewerReq(http.MethodGet, path, ""))

			if w.Code != tc.wantStatus || len(f.calls) != 1 || f.calls[0] != "download" || f.actor != reviewerActor {
				t.Fatalf("status=%d actor=%q calls=%v body=%q", w.Code, f.actor, f.calls, w.Body.String())
			}
			if strings.Contains(w.Body.String(), privateBytes) || strings.Contains(w.Body.String(), privateErr.Error()) || w.Header().Get("Content-Disposition") != "" {
				t.Fatalf("failed download leaked content/diagnostic: headers=%v body=%q", w.Header(), w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Vary") != "Authorization" {
				t.Fatalf("failed download security headers=%v", w.Header())
			}
		})
	}
}
