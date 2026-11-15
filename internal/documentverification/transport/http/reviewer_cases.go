package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/HernanEspinozaDev/espaciGo/internal/calendar"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const reviewerCasesRoute = "/api/v1/document-verification/reviewer/cases"

type ReviewerCasesService interface {
	List(context.Context, string) ([]dv.Snapshot, error)
	Load(context.Context, string, string) (dv.Snapshot, error)
	LoadDetail(context.Context, string, string) (dv.ReviewerCaseDetail, error)
	Review(context.Context, string, string, dv.Capability, dv.Decision, dv.ReasonCode, time.Time, uint64) error
	ReviewIdentity(context.Context, string, string, dv.Decision, dv.ReasonCode, dv.DocumentFormat, time.Time, *dv.IdentityReviewChecklist, time.Time, uint64) error
	Revoke(context.Context, string, string, dv.Purpose, dv.ReasonCode, uint64) error
	Download(context.Context, string, string, string) (dv.DownloadedDocument, error)
}
type reviewerCaseDTO struct {
	ownerCaseDTO
	DocumentFormatObserved    dv.DocumentFormat               `json:"document_format_observed,omitempty"`
	DocumentExpiresAtObserved *string                         `json:"document_expires_at_observed,omitempty"`
	IdentityDeclaration       *reviewerIdentityDeclarationDTO `json:"identity_declaration,omitempty"`
}

type reviewerIdentityDeclarationDTO struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	RUT         string `json:"rut"`
	DateOfBirth string `json:"date_of_birth"`
}

func reviewerDTO(x dv.Snapshot) reviewerCaseDTO {
	out := reviewerCaseDTO{ownerCaseDTO: ownerDTO(x), DocumentFormatObserved: x.DocumentFormatObserved}
	if x.Purpose == dv.PurposeIdentity && x.DocumentFormatDeclared != "" {
		out.ownerCaseDTO.DocumentExpiresAt = nil
		if !x.DocumentExpiresAt.IsZero() {
			value := observedExpiryDate(x.DocumentExpiresAt)
			out.DocumentExpiresAtObserved = &value
		}
	}
	return out
}

func reviewerSnapshotMatchesRequest(snapshot dv.Snapshot, id string) bool {
	return snapshot.ID == id && (snapshot.Purpose == dv.PurposeIdentity || snapshot.Purpose == dv.PurposeSpace)
}

const observedExpiryLocation = "America/Santiago"

func observedExpiryBoundary(value string) (time.Time, error) {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil || date.Format(time.DateOnly) != value {
		return time.Time{}, dv.ErrInvalid
	}
	location, err := time.LoadLocation(observedExpiryLocation)
	if err != nil {
		return time.Time{}, err
	}
	year, month, day := date.Date()
	if _, ok := calendar.FirstInstantOfDate(year, month, day, location); !ok {
		return time.Time{}, dv.ErrInvalid
	}
	nextDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	year, month, day = nextDate.Date()
	boundary, ok := calendar.FirstInstantOfDate(year, month, day, location)
	if !ok {
		return time.Time{}, dv.ErrInvalid
	}
	return boundary.UTC(), nil
}

func observedExpiryDate(boundary time.Time) string {
	location, err := time.LoadLocation(observedExpiryLocation)
	if err != nil {
		return ""
	}
	local := boundary.In(location)
	year, month, day := local.Date()
	dayStart, ok := calendar.FirstInstantOfDate(year, month, day, location)
	if ok && boundary.Equal(dayStart) {
		date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
		return date.Format(time.DateOnly)
	}
	return boundary.UTC().Format(time.DateOnly)
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
		if r.Context().Err() != nil {
			ownerError(w, dv.ErrDocumentStorage)
			return
		}
		items := make([]reviewerCaseDTO, 0, len(xs))
		for _, x := range xs {
			items = append(items, reviewerDTO(x))
		}
		writeOwnerJSON(w, 200, struct {
			Items []reviewerCaseDTO `json:"items"`
		}{items})
		return
	}
	if len(parts) == 1 && dv.ValidateID(parts[0]) == nil && r.Method == http.MethodGet {
		h.loadDetail(w, r, principal.AccountID, parts[0])
		return
	}
	if len(parts) == 4 && dv.ValidateID(parts[0]) == nil && parts[1] == "files" && dv.ValidateID(parts[2]) == nil && parts[3] == "content" && r.Method == http.MethodGet {
		d, e := h.service.Download(r.Context(), principal.AccountID, parts[0], parts[2])
		if e != nil {
			ownerError(w, e)
			return
		}
		if r.Context().Err() != nil {
			ownerError(w, dv.ErrDocumentStorage)
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
func (h *ReviewerCasesHandler) load(w http.ResponseWriter, r *http.Request, actor, id string, expectedPurpose dv.Purpose) {
	x, e := h.service.Load(r.Context(), actor, id)
	if e != nil {
		ownerError(w, e)
		return
	}
	if r.Context().Err() != nil {
		ownerError(w, dv.ErrDocumentStorage)
		return
	}
	if !reviewerSnapshotMatchesRequest(x, id) || x.Purpose != expectedPurpose {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(x.Revision, 10)+`"`)
	writeOwnerJSON(w, 200, reviewerDTO(x))
}

func (h *ReviewerCasesHandler) loadDetail(w http.ResponseWriter, r *http.Request, actor, id string) {
	detail, err := h.service.LoadDetail(r.Context(), actor, id)
	if err != nil {
		ownerError(w, err)
		return
	}
	if r.Context().Err() != nil {
		ownerError(w, dv.ErrDocumentStorage)
		return
	}
	if !reviewerSnapshotMatchesRequest(detail.Snapshot, id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if detail.Snapshot.Purpose == dv.PurposeIdentity && detail.IdentityDeclaration != nil && !reviewerIdentityDeclarationInScope(actor, detail) {
		ownerError(w, dv.ErrNotFound)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatUint(detail.Snapshot.Revision, 10)+`"`)
	dto := reviewerDTO(detail.Snapshot)
	if declaration := detail.IdentityDeclaration; detail.Snapshot.Purpose == dv.PurposeIdentity && declaration != nil {
		dto.IdentityDeclaration = &reviewerIdentityDeclarationDTO{
			FirstName: declaration.FirstName, LastName: declaration.LastName,
			RUT: declaration.RUT, DateOfBirth: declaration.DateOfBirth,
		}
	}
	writeOwnerJSON(w, 200, dto)
}

func reviewerIdentityDeclarationInScope(actor string, detail dv.ReviewerCaseDetail) bool {
	x, declaration := detail.Snapshot, detail.IdentityDeclaration
	if declaration == nil || x.Purpose != dv.PurposeIdentity || dv.ValidateID(x.SubjectID) != nil || x.SubjectID == actor ||
		dv.ValidateID(x.IdentityDeclarationRevisionID) != nil || !validReviewerDeclaredDocumentFormat(x.DocumentFormatDeclared) || x.RevokedAt != nil || x.SupersededAt != nil ||
		declaration.FirstName == "" || declaration.LastName == "" || declaration.RUT == "" || declaration.DateOfBirth == "" {
		return false
	}
	switch x.State {
	case dv.CaseSubmitted, dv.CaseInReview, dv.CaseNeedsEvidence:
		return true
	default:
		return false
	}
}

func validReviewerDeclaredDocumentFormat(format dv.DocumentFormat) bool {
	switch format {
	case dv.DocumentFormatLegacy, dv.DocumentFormatNew, dv.DocumentFormatUnsure:
		return true
	default:
		return false
	}
}

func (h *ReviewerCasesHandler) review(w http.ResponseWriter, r *http.Request, actor, id string) {
	var in struct {
		Capability     dv.Capability               `json:"capability"`
		Decision       dv.Decision                 `json:"decision"`
		Reason         dv.ReasonCode               `json:"reason_code"`
		Revision       uint64                      `json:"expected_revision"`
		Until          *time.Time                  `json:"verified_until"`
		ObservedFormat *dv.DocumentFormat          `json:"document_format_observed"`
		ObservedExpiry *string                     `json:"document_expires_at_observed"`
		Checklist      *dv.IdentityReviewChecklist `json:"identity_review_checklist"`
	}
	if decodeOwnerJSON(w, r, &in) != nil {
		return
	}
	until := time.Time{}
	if in.Until != nil {
		until = *in.Until
	}
	caseSnapshot, e := h.service.Load(r.Context(), actor, id)
	if e != nil {
		ownerError(w, e)
		return
	}
	if r.Context().Err() != nil {
		ownerError(w, dv.ErrDocumentStorage)
		return
	}
	if !reviewerSnapshotMatchesRequest(caseSnapshot, id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if caseSnapshot.Purpose == dv.PurposeIdentity && caseSnapshot.DocumentFormatDeclared != "" {
		if in.ObservedFormat == nil || !in.Checklist.Complete() {
			http.Error(w, "invalid request", 400)
			return
		}
		observedExpiry := time.Time{}
		if in.ObservedExpiry != nil {
			observedExpiry, e = observedExpiryBoundary(*in.ObservedExpiry)
			if e != nil {
				if errors.Is(e, dv.ErrInvalid) {
					http.Error(w, "invalid request", http.StatusBadRequest)
				} else {
					http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				}
				return
			}
		}
		if e = h.service.ReviewIdentity(r.Context(), actor, id, in.Decision, in.Reason, *in.ObservedFormat, observedExpiry, in.Checklist, until, in.Revision); e != nil {
			ownerError(w, e)
			return
		}
	} else if in.Checklist != nil {
		http.Error(w, "invalid request", 400)
		return
	} else if e = h.service.Review(r.Context(), actor, id, in.Capability, in.Decision, in.Reason, until, in.Revision); e != nil {
		ownerError(w, e)
		return
	}
	if r.Context().Err() != nil {
		ownerError(w, dv.ErrDocumentStorage)
		return
	}
	h.load(w, r, actor, id, caseSnapshot.Purpose)
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
	if r.Context().Err() != nil {
		ownerError(w, dv.ErrDocumentStorage)
		return
	}
	if e := h.service.Revoke(r.Context(), actor, id, in.Purpose, in.Reason, in.Revision); e != nil {
		ownerError(w, e)
		return
	}
	if r.Context().Err() != nil {
		ownerError(w, dv.ErrDocumentStorage)
		return
	}
	h.load(w, r, actor, id, in.Purpose)
}
