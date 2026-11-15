package verificationhttp

import (
	"net/http"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type SaveIdentityDeclarationRequest struct {
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	DateOfBirth    string `json:"date_of_birth"`
	RUT            string `json:"rut"`
	IDCountry      string `json:"id_country"`
	DocumentType   string `json:"document_type"`
	ConsentGiven   bool   `json:"consent_given"`
	ConsentVersion string `json:"consent_version"`
}

type IdentityDeclarationResponse struct {
	ID             string    `json:"id"`
	Revision       int64     `json:"revision"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	DateOfBirth    string    `json:"date_of_birth"`
	RUT            string    `json:"rut"`
	IDCountry      string    `json:"id_country"`
	DocumentType   string    `json:"document_type"`
	ConsentVersion string    `json:"consent_version"`
	ConsentGivenAt time.Time `json:"consent_given_at"`
}

func (h *Handler) handleSaveIdentityDeclaration(w http.ResponseWriter, r *http.Request, principal identity.Principal) {
	var request SaveIdentityDeclarationRequest
	if !decode(w, r, &request) {
		return
	}
	declaration, err := h.declaration.Save(r.Context(), principal.AccountID, verification.DeclarationPayload{
		FirstName:    request.FirstName,
		LastName:     request.LastName,
		DateOfBirth:  request.DateOfBirth,
		RUT:          request.RUT,
		IDCountry:    request.IDCountry,
		DocumentType: request.DocumentType,
	}, request.ConsentGiven, request.ConsentVersion, w.Header().Get("X-Request-ID"))
	if err != nil {
		serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, identityDeclarationResponse(declaration))
}

func (h *Handler) handleGetIdentityDeclaration(w http.ResponseWriter, r *http.Request, principal identity.Principal) {
	declaration, err := h.declaration.GetCurrent(r.Context(), principal.AccountID)
	if err != nil {
		serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, identityDeclarationResponse(declaration))
}

func identityDeclarationResponse(declaration verification.SavedDeclaration) IdentityDeclarationResponse {
	return IdentityDeclarationResponse{
		ID:             declaration.ID,
		Revision:       declaration.Revision,
		FirstName:      declaration.Payload.FirstName,
		LastName:       declaration.Payload.LastName,
		DateOfBirth:    declaration.Payload.DateOfBirth,
		RUT:            declaration.Payload.RUT,
		IDCountry:      declaration.Payload.IDCountry,
		DocumentType:   declaration.Payload.DocumentType,
		ConsentVersion: declaration.ConsentVersion,
		ConsentGivenAt: declaration.ConsentGivenAt,
	}
}
