package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const ownerCasesRoute = "/api/v1/document-verification/cases"

type OwnerCasesService interface {
	Start(context.Context, string, documentverification.CreateInput) (documentverification.Snapshot, error)
	ListOwnCases(context.Context, string) ([]documentverification.Snapshot, error)
	LoadOwnCase(context.Context, string, string) (documentverification.Snapshot, error)
	Submit(context.Context, string, string, documentverification.Purpose, uint64) error
}
type OwnerUploadService interface {
	Upload(context.Context, string, string, uint64, []byte) (documentverification.UploadReceipt, error)
}
type OwnerCasesHandler struct {
	auth    Authenticator
	service OwnerCasesService
	upload  OwnerUploadService
}

func NewOwnerCasesHandler(auth Authenticator, service OwnerCasesService) http.Handler {
	return &OwnerCasesHandler{auth: auth, service: service}
}
func NewOwnerCasesHandlerWithUpload(auth Authenticator, service OwnerCasesService, upload OwnerUploadService) http.Handler {
	return &OwnerCasesHandler{auth: auth, service: service, upload: upload}
}
func RegisterOwnerCasesRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle(ownerCasesRoute, handler)
	mux.Handle(ownerCasesRoute+"/", handler)
}

type ownerFileDTO struct {
	ID    string                         `json:"id"`
	State documentverification.FileState `json:"state"`
	Size  int64                          `json:"size"`
	MIME  string                         `json:"mime"`
	Pages int                            `json:"pages"`
}
type ownerCaseDTO struct {
	ID                string                         `json:"id"`
	Purpose           documentverification.Purpose   `json:"purpose"`
	SpaceID           string                         `json:"space_id,omitempty"`
	State             documentverification.CaseState `json:"state"`
	Revision          uint64                         `json:"revision"`
	DocumentExpiresAt *time.Time                     `json:"document_expires_at,omitempty"`
	VerifiedAt        *time.Time                     `json:"verified_at,omitempty"`
	VerifiedUntil     *time.Time                     `json:"verified_until,omitempty"`
	Files             []ownerFileDTO                 `json:"files"`
}

func ownerDTO(x documentverification.Snapshot) ownerCaseDTO {
	d := ownerCaseDTO{ID: x.ID, Purpose: x.Purpose, SpaceID: x.SpaceID, State: x.State, Revision: x.Revision, Files: []ownerFileDTO{}}
	if !x.DocumentExpiresAt.IsZero() {
		v := x.DocumentExpiresAt
		d.DocumentExpiresAt = &v
	}
	if !x.VerifiedAt.IsZero() {
		v := x.VerifiedAt
		d.VerifiedAt = &v
	}
	if !x.VerifiedUntil.IsZero() {
		v := x.VerifiedUntil
		d.VerifiedUntil = &v
	}
	for _, f := range x.Files {
		d.Files = append(d.Files, ownerFileDTO{f.ID, f.State, f.Size, f.MIME, f.Pages})
	}
	return d
}
func (h *OwnerCasesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Vary", "Authorization")
	if r.URL.RawQuery != "" {
		http.Error(w, "not found", 404)
		return
	}
	if h == nil || h.auth == nil || h.service == nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthenticated", 401)
		return
	}
	p, err := h.auth.Authorize(r.Context(), identity.Secret(parts[1]), "", identity.UserOperation)
	if err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			http.Error(w, "not found", 404)
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthenticated", 401)
		}
		return
	}
	actor := p.AccountID
	route, id, ok := parseOwnerCasesRoute(r.URL.Path)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch route {
	case "collection":
		switch r.Method {
		case http.MethodPost:
			h.create(w, r, actor)
		case http.MethodGet:
			h.list(w, r, actor)
		default:
			ownerMethodNotAllowed(w, "GET, POST")
		}
	case "case":
		if r.Method != http.MethodGet {
			ownerMethodNotAllowed(w, http.MethodGet)
			return
		}
		h.load(w, r, actor, id)
	case "submit":
		if r.Method != http.MethodPost {
			ownerMethodNotAllowed(w, http.MethodPost)
			return
		}
		h.submit(w, r, actor, id)
	case "files":
		if r.Method != http.MethodPost {
			ownerMethodNotAllowed(w, http.MethodPost)
			return
		}
		h.uploadFile(w, r, actor, id)
	}
}

func parseOwnerCasesRoute(path string) (route, id string, ok bool) {
	if path == ownerCasesRoute || path == ownerCasesRoute+"/" {
		return "collection", "", true
	}
	if !strings.HasPrefix(path, ownerCasesRoute+"/") {
		return "", "", false
	}
	segments := strings.Split(strings.TrimPrefix(path, ownerCasesRoute+"/"), "/")
	if len(segments) == 1 && documentverification.ValidateID(segments[0]) == nil {
		return "case", segments[0], true
	}
	if len(segments) == 2 && documentverification.ValidateID(segments[0]) == nil {
		switch segments[1] {
		case "submit":
			return "submit", segments[0], true
		case "files":
			return "files", segments[0], true
		}
	}
	return "", "", false
}

func ownerMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
func (h *OwnerCasesHandler) create(w http.ResponseWriter, r *http.Request, actor string) {
	var in struct {
		Purpose documentverification.Purpose `json:"purpose"`
		SpaceID string                       `json:"space_id"`
		Expires *time.Time                   `json:"document_expires_at"`
	}
	if decodeOwnerJSON(w, r, &in) != nil {
		return
	}
	ci := documentverification.CreateInput{Purpose: in.Purpose, SpaceID: in.SpaceID}
	if in.Expires != nil {
		ci.DocumentExpiresAt = *in.Expires
	}
	x, err := h.service.Start(r.Context(), actor, ci)
	if err != nil {
		ownerError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(x.Revision, 10)+`"`)
	writeOwnerJSON(w, 201, ownerDTO(x))
}
func (h *OwnerCasesHandler) list(w http.ResponseWriter, r *http.Request, actor string) {
	xs, err := h.service.ListOwnCases(r.Context(), actor)
	if err != nil {
		ownerError(w, err)
		return
	}
	out := make([]ownerCaseDTO, 0, len(xs))
	for _, x := range xs {
		out = append(out, ownerDTO(x))
	}
	writeOwnerJSON(w, 200, out)
}
func (h *OwnerCasesHandler) load(w http.ResponseWriter, r *http.Request, actor, id string) {
	x, err := h.service.LoadOwnCase(r.Context(), actor, id)
	if err != nil {
		ownerError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(x.Revision, 10)+`"`)
	writeOwnerJSON(w, 200, ownerDTO(x))
}
func (h *OwnerCasesHandler) submit(w http.ResponseWriter, r *http.Request, actor, id string) {
	rev, ok := ownerIfMatch(w, r)
	if !ok {
		return
	}
	var err error
	x, err := h.service.LoadOwnCase(r.Context(), actor, id)
	if err != nil {
		ownerError(w, err)
		return
	}
	if x.Revision != rev {
		http.Error(w, "case revision conflict", 409)
		return
	}
	if err = h.service.Submit(r.Context(), actor, id, x.Purpose, rev); err != nil {
		ownerError(w, err)
		return
	}
	x, err = h.service.LoadOwnCase(r.Context(), actor, id)
	if err != nil {
		ownerError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(x.Revision, 10)+`"`)
	writeOwnerJSON(w, 200, ownerDTO(x))
}
func ownerIfMatch(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	tags := r.Header.Values("If-Match")
	if len(tags) == 0 || len(tags) == 1 && tags[0] == "" {
		http.Error(w, "precondition required", 428)
		return 0, false
	}
	if len(tags) != 1 || len(tags[0]) < 3 || tags[0][0] != '"' || tags[0][len(tags[0])-1] != '"' {
		http.Error(w, "invalid precondition", 400)
		return 0, false
	}
	raw := tags[0][1 : len(tags[0])-1]
	revision, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || strconv.FormatUint(revision, 10) != raw {
		http.Error(w, "invalid precondition", 400)
		return 0, false
	}
	return revision, true
}
func (h *OwnerCasesHandler) uploadFile(w http.ResponseWriter, r *http.Request, actor, id string) {
	if h.upload == nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	if documentverification.ValidateID(id) != nil {
		http.Error(w, "not found", 404)
		return
	}
	revision, ok := ownerIfMatch(w, r)
	if !ok {
		return
	}
	var err error
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "multipart/form-data" {
		http.Error(w, "multipart form required", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, documentverification.MaxUploadBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "invalid multipart request", 400)
		return
	}
	part, err := reader.NextPart()
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request too large", 413)
		} else {
			http.Error(w, "invalid multipart request", 400)
		}
		return
	}
	if part.FormName() != "file" || part.FileName() == "" {
		_ = part.Close()
		http.Error(w, "exactly one file is required", 400)
		return
	}
	content, readErr := io.ReadAll(io.LimitReader(part, documentverification.MaxUploadBytes+1))
	closeErr := part.Close()
	if readErr != nil || closeErr != nil {
		http.Error(w, "invalid file", 400)
		return
	}
	if len(content) == 0 {
		http.Error(w, "empty file", 400)
		return
	}
	if int64(len(content)) > documentverification.MaxUploadBytes {
		http.Error(w, "file too large", 413)
		return
	}
	extraPart, extraErr := reader.NextPart()
	if extraErr != io.EOF {
		if extraPart != nil {
			_ = extraPart.Close()
		}
		var maxErr *http.MaxBytesError
		if errors.As(extraErr, &maxErr) {
			http.Error(w, "request too large", 413)
		} else {
			http.Error(w, "exactly one file is required", 400)
		}
		return
	}
	receipt, err := h.upload.Upload(r.Context(), actor, id, revision, content)
	if err != nil {
		if errors.Is(err, documentverification.ErrQuota) {
			http.Error(w, "document limit reached", 409)
		} else {
			ownerError(w, err)
		}
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(receipt.Revision, 10)+`"`)
	writeOwnerJSON(w, http.StatusCreated, struct {
		File     ownerFileDTO `json:"file"`
		Revision uint64       `json:"revision"`
	}{ownerFileDTO{receipt.File.ID, receipt.File.State, receipt.File.Size, receipt.File.MIME, receipt.File.Pages}, receipt.Revision})
}
func decodeOwnerJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		ownerJSONDecodeError(w, err)
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("invalid trailing JSON")
		}
		ownerJSONDecodeError(w, err)
		return err
	}
	return nil
}
func ownerJSONDecodeError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid request", http.StatusBadRequest)
}
func ownerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, documentverification.ErrNotFound), errors.Is(err, documentverification.ErrCapability), errors.Is(err, documentverification.ErrTransition):
		http.Error(w, "not found", 404)
	case errors.Is(err, documentverification.ErrConflict):
		http.Error(w, "case revision conflict", 409)
	case errors.Is(err, documentverification.ErrInvalid):
		http.Error(w, "invalid request", 400)
	default:
		http.Error(w, "temporarily unavailable", 503)
	}
}
func writeOwnerJSON(w http.ResponseWriter, status int, v any) {
	var b bytes.Buffer
	if json.NewEncoder(&b).Encode(v) != nil {
		http.Error(w, "temporarily unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
