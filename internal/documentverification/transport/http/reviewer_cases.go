package http

import (
	"context"
	"errors"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const reviewerCasesRoute = "/api/v1/document-verification/reviewer/cases"

type ReviewerCasesService interface {
	List(context.Context, string) ([]dv.Snapshot, error)
	Load(context.Context, string, string) (dv.Snapshot, error)
	Review(context.Context, string, string, dv.Capability, dv.Decision, dv.ReasonCode, time.Time, uint64) error
	Revoke(context.Context, string, string, dv.Purpose, dv.ReasonCode, uint64) error
	Download(context.Context, string, string, string) (dv.DownloadedDocument, error)
}
type ReviewerCasesHandler struct {
	auth    Authenticator
	service ReviewerCasesService
}

func NewReviewerCasesHandler(auth Authenticator, service ReviewerCasesService) http.Handler {
	return &ReviewerCasesHandler{auth: auth, service: service}
}
func RegisterReviewerCasesRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle(reviewerCasesRoute, handler)
	mux.Handle(reviewerCasesRoute+"/", handler)
}
func (h *ReviewerCasesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Vary", "Authorization")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.RawQuery != "" {
		http.Error(w, "not found", 404)
		return
	}
	if h == nil || h.auth == nil || h.service == nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	p := strings.Fields(r.Header.Get("Authorization"))
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthenticated", 401)
		return
	}
	principal, err := h.auth.Authorize(r.Context(), identity.Secret(p[1]), "", identity.UserOperation)
	if err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			http.Error(w, "not found", 404)
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthenticated", 401)
		}
		return
	}
	path := strings.TrimPrefix(r.URL.Path, reviewerCasesRoute)
	if path != "" && !strings.HasPrefix(path, "/") {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if path == "" || path == "/" {
		if r.Method != http.MethodGet {
			ownerMethodNotAllowed(w, "GET")
			return
		}
		xs, e := h.service.List(r.Context(), principal.AccountID)
		if e != nil {
			ownerError(w, e)
			return
		}
		items := make([]ownerCaseDTO, 0, len(xs))
		for _, x := range xs {
			items = append(items, ownerDTO(x))
		}
		writeOwnerJSON(w, 200, struct {
			Items []ownerCaseDTO `json:"items"`
		}{items})
		return
	}
	if len(parts) == 1 && dv.ValidateID(parts[0]) == nil && r.Method == http.MethodGet {
		h.load(w, r, principal.AccountID, parts[0])
		return
	}
	if len(parts) == 4 && dv.ValidateID(parts[0]) == nil && parts[1] == "files" && dv.ValidateID(parts[2]) == nil && parts[3] == "content" && r.Method == http.MethodGet {
		d, e := h.service.Download(r.Context(), principal.AccountID, parts[0], parts[2])
		if e != nil {
			ownerError(w, e)
			return
		}
		typ, name, ok := fixedDownloadType(d.MIME)
		if !ok {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", typ)
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(d.Content)))
		w.WriteHeader(200)
		_, _ = w.Write(d.Content)
		return
	}
	if len(parts) == 2 && dv.ValidateID(parts[0]) == nil && r.Method == http.MethodPost {
		switch parts[1] {
		case "review":
			h.review(w, r, principal.AccountID, parts[0])
			return
		case "revoke":
			h.revoke(w, r, principal.AccountID, parts[0])
			return
		}
	}
	if (len(parts) == 1 && r.Method != http.MethodGet) || (len(parts) == 2 && r.Method != http.MethodPost) || (len(parts) == 4 && r.Method != http.MethodGet) {
		ownerMethodNotAllowed(w, "GET, POST")
		return
	}
	http.NotFound(w, r)
}
func (h *ReviewerCasesHandler) load(w http.ResponseWriter, r *http.Request, actor, id string) {
	x, e := h.service.Load(r.Context(), actor, id)
	if e != nil {
		ownerError(w, e)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(x.Revision, 10)+`"`)
	writeOwnerJSON(w, 200, ownerDTO(x))
}
func (h *ReviewerCasesHandler) review(w http.ResponseWriter, r *http.Request, actor, id string) {
	var in struct {
		Capability dv.Capability `json:"capability"`
		Decision   dv.Decision   `json:"decision"`
		Reason     dv.ReasonCode `json:"reason_code"`
		Revision   uint64        `json:"expected_revision"`
		Until      *time.Time    `json:"verified_until"`
	}
	if decodeOwnerJSON(w, r, &in) != nil {
		return
	}
	until := time.Time{}
	if in.Until != nil {
		until = *in.Until
	}
	if e := h.service.Review(r.Context(), actor, id, in.Capability, in.Decision, in.Reason, until, in.Revision); e != nil {
		ownerError(w, e)
		return
	}
	h.load(w, r, actor, id)
}
func (h *ReviewerCasesHandler) revoke(w http.ResponseWriter, r *http.Request, actor, id string) {
	var in struct {
		Purpose  dv.Purpose    `json:"purpose"`
		Reason   dv.ReasonCode `json:"reason_code"`
		Revision uint64        `json:"expected_revision"`
	}
	if decodeOwnerJSON(w, r, &in) != nil {
		return
	}
	if e := h.service.Revoke(r.Context(), actor, id, in.Purpose, in.Reason, in.Revision); e != nil {
		ownerError(w, e)
		return
	}
	h.load(w, r, actor, id)
}
