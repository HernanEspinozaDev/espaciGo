package verification

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestEvaluateDecisionFailClosed(t *testing.T) {
	attempt := &IdentityVerificationAttempt{
		ID:                    "att-123",
		IDCountry:             "CHL",
		DocumentType:          "ID",
		DeclaredFirstName:     "Juan Carlos",
		DeclaredLastName:      "Perez Gonzalez",
		DeclaredDateOfBirth:   "1990-05-15",
		DeclaredRUTNormalized: "12345678-5",
		DiditSessionID:        "sess-abc",
	}

	// 1. Success matching scenario
	validDecision := &DiditDecisionResponse{
		SessionID:            "sess-abc",
		Status:               "Approved",
		Warnings:             nil,
		FullName:             "Juan Carlos Perez Gonzalez",
		DateOfBirth:          "1990-05-15",
		IdentificationNumber: "12.345.678-5",
		IDCountry:            "CHL",
		DocumentType:         "ID",
		Liveness:             true,
		FaceMatch:            true,
	}
	status, flags, warnings := EvaluateDecision(attempt, validDecision)
	if status != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", status)
	}
	if len(flags) != 0 || len(warnings) != 0 {
		t.Errorf("expected zero flags and warnings, got %v, %v", flags, warnings)
	}

	// 2. Mismatch warning: fail closed to IN_REVIEW
	mismatchWarningDecision := *validDecision
	mismatchWarningDecision.Warnings = []string{"FULL_NAME_MISMATCH_WITH_PROVIDED"}
	status, flags, warnings = EvaluateDecision(attempt, &mismatchWarningDecision)
	if status != "IN_REVIEW" {
		t.Errorf("expected IN_REVIEW for warning mismatch, got %s", status)
	}
	if !flags["FULL_NAME_MISMATCH_WITH_PROVIDED"].(bool) {
		t.Errorf("expected flag for full name mismatch warning")
	}

	// 3. Different RUT: fail closed to IN_REVIEW
	rutMismatchDecision := *validDecision
	rutMismatchDecision.IdentificationNumber = "11111111-1"
	status, flags, warnings = EvaluateDecision(attempt, &rutMismatchDecision)
	if status != "IN_REVIEW" {
		t.Errorf("expected IN_REVIEW for RUT mismatch, got %s", status)
	}
	if !flags["rut"].(bool) {
		t.Errorf("expected flag for rut mismatch")
	}

	// 4. Different DOB: fail closed to IN_REVIEW
	dobMismatchDecision := *validDecision
	dobMismatchDecision.DateOfBirth = "1985-01-01"
	status, flags, warnings = EvaluateDecision(attempt, &dobMismatchDecision)
	if status != "IN_REVIEW" {
		t.Errorf("expected IN_REVIEW for DOB mismatch, got %s", status)
	}
	if !flags["date_of_birth"].(bool) {
		t.Errorf("expected flag for date_of_birth mismatch")
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "test-secret-key-12345"
	client := NewHTTPDiditClient("key", secret, true, 0)
	payload := []byte(`{"event":"session.completed","session_id":"sess-999"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := hex.EncodeToString(mac.Sum(nil))

	if !client.VerifyWebhookSignature(validSig, payload) {
		t.Errorf("expected signature verification to succeed")
	}

	if client.VerifyWebhookSignature("invalid-sig", payload) {
		t.Errorf("expected invalid signature to fail")
	}

	if client.VerifyWebhookSignature(validSig, []byte(`altered-payload`)) {
		t.Errorf("expected altered payload to fail")
	}
}
