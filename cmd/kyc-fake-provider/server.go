package main

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
)

const (
	fakeAPIKey     = "local-kyc-test-api-key"
	fakeWorkflowID = "local-kyc-test-workflow"
	fakeSessionURL = "https://verify.didit.me/session/"
)

type fakeSession struct {
	workflowID string
	vendorData string
}

type fakeProvider struct {
	mu                sync.Mutex
	sessions          map[string]fakeSession
	creationRequests  int
	syntheticApproval bool
}

func NewHandler() http.Handler {
	provider := &fakeProvider{sessions: make(map[string]fakeSession)}
	return http.HandlerFunc(provider.serveHTTP)
}

func NewSyntheticApprovalHandler() http.Handler {
	provider := &fakeProvider{sessions: make(map[string]fakeSession), syntheticApproval: true}
	return http.HandlerFunc(provider.serveHTTP)
}

func (p *fakeProvider) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Header.Get("x-api-key") != fakeAPIKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.URL.RawQuery != "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v3/session/" {
		p.mu.Lock()
		p.creationRequests++
		p.mu.Unlock()
		p.createSession(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/__local_test/stats" {
		p.writeStats(w)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "v3" && parts[1] == "session" {
		sessionID := parts[2]
		if _, err := uuid.Parse(sessionID); err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodGet && parts[3] == "decision":
			p.getDecision(w, r, sessionID)
			return
		case r.Method == http.MethodDelete && parts[3] == "delete":
			p.deleteSession(w, r, sessionID)
			return
		}
	}
	http.NotFound(w, r)
}

func (p *fakeProvider) writeStats(w http.ResponseWriter) {
	p.mu.Lock()
	creationRequests, activeSessions := p.creationRequests, len(p.sessions)
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, struct {
		CreationRequests int `json:"creation_requests"`
		ActiveSessions   int `json:"active_sessions"`
	}{CreationRequests: creationRequests, ActiveSessions: activeSessions})
}

func (p *fakeProvider) createSession(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var request verification.CreateSessionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if request.WorkflowID != fakeWorkflowID || strings.TrimSpace(request.VendorData) == "" ||
		strings.TrimSpace(request.ExpectedDetails.FirstName) == "" || strings.TrimSpace(request.ExpectedDetails.LastName) == "" ||
		strings.TrimSpace(request.ExpectedDetails.DateOfBirth) == "" || request.ExpectedDetails.IDCountry != "CHL" ||
		request.ExpectedDetails.DocumentType != "ID" || strings.TrimSpace(request.ExpectedDetails.IdentificationNumber) == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if p.syntheticApproval {
		e := request.ExpectedDetails
		rut := strings.NewReplacer(".", "", "-", "").Replace(strings.TrimSpace(e.IdentificationNumber))
		if e.FirstName != "Synthetic" || e.LastName != "Person" || e.DateOfBirth != "1990-01-02" || e.IDCountry != "CHL" || e.DocumentType != "ID" || strings.ToUpper(rut) != "112223339" || (e.ExpectedDocumentTypes != nil && (len(e.ExpectedDocumentTypes) != 1 || e.ExpectedDocumentTypes[0] != "ID")) {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
	}

	sessionID := uuid.NewString()
	p.mu.Lock()
	p.sessions[sessionID] = fakeSession{workflowID: request.WorkflowID, vendorData: request.VendorData}
	p.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id": sessionID,
		"url":        fakeSessionURL + sessionID,
	})
}

func (p *fakeProvider) getDecision(w http.ResponseWriter, r *http.Request, sessionID string) {
	p.mu.Lock()
	session, ok := p.sessions[sessionID]
	p.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	decision := verification.DiditDecisionResponse{SessionID: sessionID, WorkflowID: session.workflowID, Status: "In Review", VendorData: session.vendorData}
	if p.syntheticApproval {
		decision.Status = "Approved"
		decision.IDVerifications = []verification.DiditIDVerificationNode{{Status: "Approved", FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", IdentificationNumber: "11.222.333-9", IssuingState: "CHL", DocumentType: "Identity Card", ExpirationDate: "2030-01-01"}}
		decision.LivenessChecks = []verification.DiditLivenessNode{{Status: "Approved", Score: 98.2}}
		decision.FaceMatches = []verification.DiditFaceMatchNode{{Status: "Approved", Score: 95.4}}
	}
	writeJSON(w, http.StatusOK, decision)
}

func (p *fakeProvider) deleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	p.mu.Lock()
	_, ok := p.sessions[sessionID]
	if ok {
		delete(p.sessions, sessionID)
	}
	p.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
