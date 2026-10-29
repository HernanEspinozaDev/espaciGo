package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const ownerSpaceDocumentRoute = "GET /api/v1/document-verification/space-cases/{caseID}/files/{fileID}/content"

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

type OwnerSpaceDocumentDownloader interface {
	Download(context.Context, string, string, string) (documentverification.DownloadedDocument, error)
}

type OwnerDownloadHandler struct {
	auth       Authenticator
	downloader OwnerSpaceDocumentDownloader
}

func NewOwnerDownloadHandler(auth Authenticator, downloader OwnerSpaceDocumentDownloader) http.Handler {
	return &OwnerDownloadHandler{auth: auth, downloader: downloader}
}

func RegisterOwnerDownloadRoute(mux *http.ServeMux, handler http.Handler) {
	mux.Handle(ownerSpaceDocumentRoute, handler)
}

func (h *OwnerDownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Vary", "Authorization")
	if r.URL.RawQuery != "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if h == nil || h.auth == nil || h.downloader == nil {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	principal, err := h.auth.Authorize(r.Context(), identity.Secret(parts[1]), "", identity.UserOperation)
	if err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			http.Error(w, "not found", http.StatusNotFound)
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
		}
		return
	}
	download, err := h.downloader.Download(r.Context(), principal.AccountID, r.PathValue("caseID"), r.PathValue("fileID"))
	if err != nil {
		switch {
		case errors.Is(err, documentverification.ErrNotFound),
			errors.Is(err, documentverification.ErrCapability),
			errors.Is(err, documentverification.ErrSelfReview),
			errors.Is(err, documentverification.ErrFilesNotClean),
			errors.Is(err, documentverification.ErrTransition):
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	contentType, filename, ok := fixedDownloadType(download.MIME)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(download.Content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(download.Content)
}

func fixedDownloadType(mime string) (contentType, filename string, ok bool) {
	switch mime {
	case "application/pdf":
		return "application/pdf", "space-document.pdf", true
	case "image/jpeg":
		return "image/jpeg", "space-document.jpg", true
	case "image/png":
		return "image/png", "space-document.png", true
	default:
		return "", "", false
	}
}
