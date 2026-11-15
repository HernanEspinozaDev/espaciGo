package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type cancelAfterOwnerUploadFake struct {
	*ownerUploadFake
	cancel context.CancelFunc
}

func (f *cancelAfterOwnerUploadFake) Upload(ctx context.Context, actor, caseID string, revision uint64, role documentverification.EvidenceRole, content []byte) (documentverification.UploadReceipt, error) {
	receipt, err := f.ownerUploadFake.Upload(ctx, actor, caseID, revision, role, content)
	f.cancel()
	return receipt, err
}

func TestOwnerUploadDoesNotStartOrReturnAfterCancellation(t *testing.T) {
	const uploadPath = "/api/v1/document-verification/cases/" + ownerUploadCase + "/files"
	for _, tc := range []struct {
		name               string
		cancelBeforeUpload bool
		wantUploadCalls    int
	}{
		{name: "already canceled before service call", cancelBeforeUpload: true},
		{name: "canceled while service finishes", wantUploadCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			receipt := documentverification.UploadReceipt{
				File:     documentverification.File{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", State: documentverification.FilePending, Size: 12, MIME: "image/png", Pages: 1},
				Revision: 4,
			}
			base := &ownerUploadFake{receipt: receipt}
			var upload OwnerUploadService = base
			if tc.cancelBeforeUpload {
				cancel()
			} else {
				upload = &cancelAfterOwnerUploadFake{ownerUploadFake: base, cancel: cancel}
			}
			handler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, upload)
			request := ownerUploadRequest(t, uploadPath, `"3"`, []byte("synthetic image bytes")).WithContext(ctx)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" || response.Header().Get("ETag") != "" || strings.Contains(response.Body.String(), receipt.File.ID) {
				t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
			}
			if base.calls != tc.wantUploadCalls || ctx.Err() != context.Canceled {
				t.Fatalf("upload calls=%d want=%d canceled=%v", base.calls, tc.wantUploadCalls, ctx.Err() != nil)
			}
		})
	}
}
