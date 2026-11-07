package verificationhttp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type mockAuth struct{}

func (mockAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{
		AccountID: "acc-test-12345678-1234-1234-1234-123456789012",
		Roles:     []identity.Role{"usuario"},
	}, nil
}

func computeHMAC(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestDiditIntentHTTPHandlers(t *testing.T) {
	handler := NewHandler(mockAuth{}, nil, []string{"https://example.com"})

	// 1. Create intent - Valid request
	validBody := map[string]interface{}{
		"first_name":      "Esteban",
		"last_name":       "Silva",
		"date_of_birth":   "1992-08-20",
		"rut":             "12.345.678-5",
		"consent_given":   true,
		"consent_version": "v1.0-20261107",
	}
	raw, _ := json.Marshal(validBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/intent", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d, body: %s", w.Code, w.Body.String())
	}

	var createResp CreateDiditIntentResponse
	if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if createResp.ID == "" || createResp.RedirectURL == "" {
		t.Errorf("expected id and redirect_url, got %v", createResp)
	}

	// 2. Query status of created intent
	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/didit/intent/"+createResp.ID, nil)
	statusReq.Header.Set("Authorization", "Bearer valid-token")
	wStatus := httptest.NewRecorder()
	handler.ServeHTTP(wStatus, statusReq)

	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body: %s", wStatus.Code, wStatus.Body.String())
	}
	var statusResp GetDiditIntentResponse
	if err := json.NewDecoder(wStatus.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode status: %v", err)
	}
	if statusResp.ID != createResp.ID || statusResp.Status != "PENDING_USER_INPUT" || statusResp.BadgeResult != "NOT_VERIFIED" {
		t.Errorf("unexpected status response: %+v", statusResp)
	}

	// 3. Invalid RUT error
	invalidRutBody := map[string]interface{}{
		"first_name":      "Esteban",
		"last_name":       "Silva",
		"date_of_birth":   "1992-08-20",
		"rut":             "12345678-9",
		"consent_given":   true,
		"consent_version": "v1.0-20261107",
	}
	rawBad, _ := json.Marshal(invalidRutBody)
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/intent", bytes.NewReader(rawBad))
	reqBad.Header.Set("Authorization", "Bearer valid-token")
	wBad := httptest.NewRecorder()
	handler.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad rut, got %d", wBad.Code)
	}

	// 4. Missing consent error
	noConsentBody := map[string]interface{}{
		"first_name":      "Esteban",
		"last_name":       "Silva",
		"date_of_birth":   "1992-08-20",
		"rut":             "12.345.678-5",
		"consent_given":   false,
		"consent_version": "v1.0-20261107",
	}
	rawNoConsent, _ := json.Marshal(noConsentBody)
	reqNoConsent := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/intent", bytes.NewReader(rawNoConsent))
	reqNoConsent.Header.Set("Authorization", "Bearer valid-token")
	wNoConsent := httptest.NewRecorder()
	handler.ServeHTTP(wNoConsent, reqNoConsent)
	if wNoConsent.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing consent, got %d", wNoConsent.Code)
	}
}

func TestDiditWebhookScenarios(t *testing.T) {
	secret := "test-secret-kyc05-sandbox"
	t.Setenv("DIDIT_WEBHOOK_SECRET", secret)
	handler := NewHandler(mockAuth{}, nil, []string{"https://example.com"})

	// Helper to create an intent first
	createIntent := func(fn, ln, dob, rut string) string {
		body := map[string]interface{}{
			"first_name":      fn,
			"last_name":       ln,
			"date_of_birth":   dob,
			"rut":             rut,
			"consent_given":   true,
			"consent_version": "v1.0-20261107",
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/intent", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create intent: %d body: %s", w.Code, w.Body.String())
		}
		var res CreateDiditIntentResponse
		_ = json.NewDecoder(w.Body).Decode(&res)
		return res.ID
	}

	// =========================================================================
	// ESCENARIO A: Flujo Aprobado (Approved) con firma válida HMAC
	// =========================================================================
	t.Run("Escenario A: Webhook Aprobado con firma valida", func(t *testing.T) {
		intentID := createIntent("Carlos", "Perez", "1985-04-12", "11.222.333-9")

		payload := map[string]interface{}{
			"event_id":     "evt-scenario-a-approved",
			"webhook_type": "status.updated",
			"vendor_data":  intentID,
			"status":       "Approved",
			"decision": map[string]interface{}{
				"status":                "Approved",
				"full_name":             "Carlos Perez",
				"date_of_birth":         "1985-04-12",
				"id_country":            "CHL",
				"document_type":         "ID",
				"identification_number": "11222333-9",
				"liveness_passed":       true,
				"face_match_passed":     true,
				"warnings":              []string{},
			},
		}
		raw, _ := json.Marshal(payload)
		sig := computeHMAC(secret, raw)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/webhook", bytes.NewReader(raw))
		req.Header.Set("X-Signature", sig)
		req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d, body: %s", w.Code, w.Body.String())
		}

		// Check that intent state in BFF is now APPROVED / VERIFIED
		statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/didit/intent/"+intentID, nil)
		statusReq.Header.Set("Authorization", "Bearer valid-token")
		wStatus := httptest.NewRecorder()
		handler.ServeHTTP(wStatus, statusReq)
		var statusResp GetDiditIntentResponse
		_ = json.NewDecoder(wStatus.Body).Decode(&statusResp)
		if statusResp.Status != "APPROVED" || statusResp.BadgeResult != "VERIFIED" {
			t.Errorf("expected APPROVED/VERIFIED, got status=%s badge=%s", statusResp.Status, statusResp.BadgeResult)
		}
	})

	// =========================================================================
	// ESCENARIO B: Rechazo o Discrepancia (Declined / Mismatch Fail-Closed)
	// =========================================================================
	t.Run("Escenario B: Webhook con discrepancia en RUT o warnings fail-closed", func(t *testing.T) {
		intentID := createIntent("Maria", "Gonzalez", "1990-01-15", "15.678.901-1")

		payload := map[string]interface{}{
			"event_id":     "evt-scenario-b-declined",
			"webhook_type": "status.updated",
			"vendor_data":  intentID,
			"status":       "Declined",
			"decision": map[string]interface{}	{
				"status":                "Declined",
				"full_name":             "Maria Gonzalez",
				"date_of_birth":         "1990-01-15",
				"id_country":            "CHL",
				"identification_number": "99999999-9", // RUT no coincide!
				"warnings":              []string{"IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED"},
			},
		}
		raw, _ := json.Marshal(payload)
		sig := computeHMAC(secret, raw)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/webhook", bytes.NewReader(raw))
		req.Header.Set("X-Signature-V2", sig)
		req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for valid webhook delivery, got %d", w.Code)
		}

		// Check that intent state in BFF is IN_REVIEW (fail-closed, never verified)
		statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/didit/intent/"+intentID, nil)
		statusReq.Header.Set("Authorization", "Bearer valid-token")
		wStatus := httptest.NewRecorder()
		handler.ServeHTTP(wStatus, statusReq)
		var statusResp GetDiditIntentResponse
		_ = json.NewDecoder(wStatus.Body).Decode(&statusResp)
		if statusResp.Status != "IN_REVIEW" || statusResp.BadgeResult != "IN_REVIEW" {
			t.Errorf("expected IN_REVIEW/IN_REVIEW on discrepancy, got status=%s badge=%s", statusResp.Status, statusResp.BadgeResult)
		}
	})

	// =========================================================================
	// ESCENARIO C: Firma Invalida / Spoofing / Tampering / Replay
	// =========================================================================
	t.Run("Escenario C: Firma invalida rechazada 401 Unauthorized", func(t *testing.T) {
		payload := []byte(`{"event_id":"evt-spoofed","status":"Approved"}`)

		// C.1 Firma corrupta
		reqBadSig := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/webhook", bytes.NewReader(payload))
		reqBadSig.Header.Set("X-Signature", "bad-signature-hex-1234567890abcdef")
		wBadSig := httptest.NewRecorder()
		handler.ServeHTTP(wBadSig, reqBadSig)
		if wBadSig.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for corrupted signature, got %d", wBadSig.Code)
		}

		// C.2 Sin cabecera de firma
		reqNoSig := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/webhook", bytes.NewReader(payload))
		wNoSig := httptest.NewRecorder()
		handler.ServeHTTP(wNoSig, reqNoSig)
		if wNoSig.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing signature header, got %d", wNoSig.Code)
		}

		// C.3 Replay attack (timestamp expirado > 300s)
		sigValid := computeHMAC(secret, payload)
		reqReplay := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/didit/webhook", bytes.NewReader(payload))
		reqReplay.Header.Set("X-Signature", sigValid)
		reqReplay.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Unix()-600)) // 10 min atras
		wReplay := httptest.NewRecorder()
		handler.ServeHTTP(wReplay, reqReplay)
		if wReplay.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for expired timestamp replay, got %d", wReplay.Code)
		}
	})
}
