package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerCancelAfterDownloadService struct {
	ReviewerCasesService
	cancel   context.CancelFunc
	document dv.DownloadedDocument
}

func (s reviewerCancelAfterDownloadService) Download(context.Context, string, string, string) (dv.DownloadedDocument, error) {
	s.cancel()
	return s.document, nil
}

func TestReviewerDownloadHandlerDoesNotWriteDocumentWhenCanceledAfterServiceReturns(t *testing.T) {
	privateBytes := []byte("synthetic private identity evidence")
	ctx, cancel := context.WithCancel(context.Background())
	service := reviewerCancelAfterDownloadService{
		cancel:   cancel,
		document: dv.DownloadedDocument{MIME: "image/png", Content: privateBytes},
	}
	path := reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content"
	response := httptest.NewRecorder()
	request := reviewerReq(http.MethodGet, path, "").WithContext(ctx)

	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.Bytes())
	}
	if bytes.Contains(response.Body.Bytes(), privateBytes) || response.Header().Get("Content-Disposition") != "" ||
		response.Header().Get("Content-Length") != "" || response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("canceled response leaked document: headers=%v body=%q", response.Header(), response.Body.Bytes())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("canceled response security headers=%v", response.Header())
	}
}
