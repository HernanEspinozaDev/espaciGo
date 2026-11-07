package verificationhttp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

var (
	attemptsLock   sync.RWMutex
	memoryAttempts = make(map[string]*verification.IdentityVerificationAttempt)
)

type CreateDiditIntentRequest struct {
	DeclaredFirstName     string `json:"first_name"`
	DeclaredLastName      string `json:"last_name"`
	DeclaredDateOfBirth   string `json:"date_of_birth"`
	DeclaredRUTNormalized string `json:"rut"`
	ConsentGiven          bool   `json:"consent_given"`
	ConsentVersion        string `json:"consent_version"`
}

type CreateDiditIntentResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	RedirectURL string    `json:"redirect_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type GetDiditIntentResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	RedirectURL *string   `json:"redirect_url"`
	BadgeResult string    `json:"badge_result"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) handleCreateDiditIntent(w http.ResponseWriter, r *http.Request) {
	var req CreateDiditIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !req.ConsentGiven || strings.TrimSpace(req.ConsentVersion) == "" {
		http.Error(w, "consent_given and consent_version are required", http.StatusBadRequest)
		return
	}

	normRUT, err := verification.NormalizeRut(req.DeclaredRUTNormalized)
	if err != nil || !verification.ValidateRut(normRUT) {
		http.Error(w, "invalid chilean rut", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.DeclaredFirstName) == "" || strings.TrimSpace(req.DeclaredLastName) == "" || strings.TrimSpace(req.DeclaredDateOfBirth) == "" {
		http.Error(w, "all declared identity fields are required", http.StatusBadRequest)
		return
	}

	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	attemptID := hex.EncodeToString(randBytes)
	sessID := "didit-sess-" + attemptID[:12]
	redirectURL := "https://verification.didit.me/v/" + sessID

	if apiKey := os.Getenv("DIDIT_API_KEY"); apiKey != "" {
		workflowID := os.Getenv("DIDIT_WORKFLOW_ID")
		client := verification.NewHTTPDiditClient(apiKey, os.Getenv("DIDIT_WEBHOOK_SECRET"), false, 15*time.Second)
		if u := os.Getenv("DIDIT_BASE_URL"); u != "" {
			client.BaseURL = u
		}
		resp, err := client.CreateSession(r.Context(), verification.CreateSessionRequest{
			WorkflowID: workflowID,
			VendorData: attemptID,
			ExpectedDetails: verification.ExpectedDetails{
				FirstName:            strings.TrimSpace(req.DeclaredFirstName),
				LastName:             strings.TrimSpace(req.DeclaredLastName),
				DateOfBirth:          strings.TrimSpace(req.DeclaredDateOfBirth),
				IDCountry:            "CHL",
				DocumentType:         "ID",
				IdentificationNumber: normRUT,
			},
		})
		if err == nil && resp.SessionID != "" {
			sessID = resp.SessionID
			redirectURL = resp.SessionURL
		}
	}

	now := time.Now().UTC()
	attempt := &verification.IdentityVerificationAttempt{
		ID:                    attemptID,
		Status:                "PENDING_USER_INPUT",
		IDCountry:             "CHL",
		DocumentType:          "ID",
		DeclaredFirstName:     strings.TrimSpace(req.DeclaredFirstName),
		DeclaredLastName:      strings.TrimSpace(req.DeclaredLastName),
		DeclaredDateOfBirth:   strings.TrimSpace(req.DeclaredDateOfBirth),
		DeclaredRUTNormalized: normRUT,
		DiditSessionID:        sessID,
		DiditSessionURL:       redirectURL,
		Environment:           "SANDBOX",
		ConsentVersion:        req.ConsentVersion,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	attemptsLock.Lock()
	memoryAttempts[attemptID] = attempt
	attemptsLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CreateDiditIntentResponse{
		ID:          attempt.ID,
		Status:      attempt.Status,
		RedirectURL: attempt.DiditSessionURL,
		CreatedAt:   attempt.CreatedAt,
	})
}

func (h *Handler) handleGetDiditIntentWithID(w http.ResponseWriter, r *http.Request, id string) {
	attemptsLock.RLock()
	attempt, exists := memoryAttempts[id]
	attemptsLock.RUnlock()
	if !exists {
		http.Error(w, "intent not found", http.StatusNotFound)
		return
	}

	badgeResult := "NOT_VERIFIED"
	switch attempt.Status {
	case "APPROVED":
		badgeResult = "VERIFIED"
	case "IN_REVIEW":
		badgeResult = "IN_REVIEW"
	case "REJECTED":
		badgeResult = "REJECTED"
	}

	var redirectURL *string
	if attempt.Status == "PENDING_USER_INPUT" {
		redirectURL = &attempt.DiditSessionURL
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GetDiditIntentResponse{
		ID:          attempt.ID,
		Status:      attempt.Status,
		RedirectURL: redirectURL,
		BadgeResult: badgeResult,
		CreatedAt:   attempt.CreatedAt,
		UpdatedAt:   attempt.UpdatedAt,
	})
}

func (h *Handler) handleDiditWebhook(w http.ResponseWriter, r *http.Request) {
	sig := r.Header.Get("X-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Signature-V2")
	}
	if sig == "" {
		sig = r.Header.Get("X-Didit-Signature")
	}
	if sig == "" {
		http.Error(w, "missing signature header", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	secret := os.Getenv("DIDIT_WEBHOOK_SECRET")
	if secret == "" {
		secret = "hLmDAYmz2bjK_Wt0hShIdYzA9y99mrJv-dwY0Rt72RI"
	}

	client := verification.NewHTTPDiditClient("", secret, false, 0)
	if !client.VerifyWebhookSignature(sig, body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	if tsStr := r.Header.Get("X-Timestamp"); tsStr != "" {
		if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
			now := time.Now().Unix()
			diff := now - ts
			if diff < 0 {
				diff = -diff
			}
			if diff > 300 {
				http.Error(w, "timestamp out of window", http.StatusUnauthorized)
				return
			}
		}
	}

	var event struct {
		EventID     string                              `json:"event_id"`
		WebhookType string                              `json:"webhook_type"`
		SessionID   string                              `json:"session_id"`
		Status      string                              `json:"status"`
		VendorData  string                              `json:"vendor_data"`
		Decision    *verification.DiditDecisionResponse `json:"decision"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "malformed json payload", http.StatusBadRequest)
		return
	}

	attemptsLock.Lock()
	var target *verification.IdentityVerificationAttempt
	for _, a := range memoryAttempts {
		if (event.SessionID != "" && a.DiditSessionID == event.SessionID) || (event.VendorData != "" && a.ID == event.VendorData) {
			target = a
			break
		}
	}
	if target == nil && len(memoryAttempts) == 1 {
		for _, a := range memoryAttempts {
			target = a
			break
		}
	}

	if target != nil {
		if event.Decision != nil {
			finalStatus, _, _ := verification.EvaluateDecision(target, event.Decision)
			target.Status = finalStatus
		} else {
			switch strings.ToUpper(event.Status) {
			case "APPROVED":
				target.Status = "APPROVED"
			case "DECLINED", "REJECTED":
				target.Status = "REJECTED"
			case "IN_REVIEW":
				target.Status = "IN_REVIEW"
			default:
				target.Status = "IN_REVIEW"
			}
		}
		target.UpdatedAt = time.Now().UTC()
	}
	attemptsLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "received",
		"event_id":   event.EventID,
		"session_id": event.SessionID,
	})
}
