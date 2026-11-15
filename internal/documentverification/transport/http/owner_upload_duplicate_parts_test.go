package http

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestOwnerCaseUploadRejectsDuplicateFileAndRoleParts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parts []string
	}{
		{name: "duplicate file", parts: []string{"file", "file"}},
		{name: "duplicate role", parts: []string{"evidence_role", "evidence_role"}},
		{name: "duplicate role after otherwise valid upload", parts: []string{"file", "evidence_role", "evidence_role"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			for i, field := range tc.parts {
				if field == "file" {
					part, err := mw.CreateFormFile(field, "synthetic-"+strconv.Itoa(i)+".png")
					if err != nil {
						t.Fatal(err)
					}
					if _, err := part.Write([]byte("synthetic evidence")); err != nil {
						t.Fatal(err)
					}
					continue
				}
				part, err := mw.CreateFormField(field)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := part.Write([]byte(documentverification.EvidenceRoleIdentityFront)); err != nil {
					t.Fatal(err)
				}
			}
			if err := mw.Close(); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", &body)
			request.Header.Set("Authorization", "Bearer synthetic-session")
			request.Header.Set("Content-Type", mw.FormDataContentType())
			request.Header.Set("If-Match", `"3"`)

			fake := &ownerUploadFake{}
			response := httptest.NewRecorder()
			NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || fake.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, fake.calls, response.Body.String())
			}
		})
	}
}
