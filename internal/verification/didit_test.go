package verification

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestCanonicalDiditWebhookPayload(t *testing.T) {
	input := []byte(`{"2":"dos","10":"diez","nombre":"José","float":27.0,"tiny":0.00001,"nested":{"b":true,"a":null}}`)
	got, err := CanonicalDiditWebhookPayload(input)
	if err != nil {
		t.Fatalf("canonicalize payload: %v", err)
	}
	want := `{"10":"diez","2":"dos","float":27,"nested":{"a":null,"b":true},"nombre":"José","tiny":1e-05}`
	if string(got) != want {
		t.Fatalf("canonical payload = %s, want %s", got, want)
	}
	got, err = CanonicalDiditWebhookPayload([]byte(`{"number":1000000000000000.125}`))
	if err != nil {
		t.Fatalf("canonicalize large fractional number: %v", err)
	}
	if string(got) != `{"number":1000000000000000.1}` {
		t.Fatalf("canonical large fractional number = %s", got)
	}
	got, err = CanonicalDiditWebhookPayload([]byte(`{"number":1000000000000000129}`))
	if err != nil || string(got) != `{"number":1000000000000000129}` {
		t.Fatalf("canonical exact integer = %s, err=%v", got, err)
	}
	got, err = CanonicalDiditWebhookPayload([]byte(`{"🙂":"emoji","Ｆ":"fullwidth"}`))
	if err != nil || string(got) != `{"Ｆ":"fullwidth","🙂":"emoji"}` {
		t.Fatalf("canonical Unicode code-point key order = %s, err=%v", got, err)
	}
}

func TestVerifyWebhookSignatureV2(t *testing.T) {
	const secret = "test-secret-kyc08-v2"
	payload := []byte(`{ "tiny": 0.0000100, "float": 27.00, "nombre":"Jos\u00e9", "nested":{"b":true,"a":null}, "2":"dos", "10":"diez" }`)
	client := NewHTTPDiditClient("", secret, false, 0)
	const signature = "3ca10a3c0daa684080f5b8f17c0f700cc246bca31bb0f9f6d79f17c873ced4a7"
	if !client.VerifyWebhookSignatureV2(signature, payload) {
		t.Fatal("expected the canonical V2 signature to be accepted")
	}
	if client.VerifyWebhookSignatureV2(signature, []byte(strings.Replace(string(payload), "dos", "tres", 1))) {
		t.Fatal("expected a changed signed payload to be rejected")
	}
}

func TestEvaluateDecisionMissingDecisionFailsClosed(t *testing.T) {
	status, flags, warnings := EvaluateDecision(nil, nil)
	if status != "IN_REVIEW" || len(flags) != 0 || len(warnings) != 1 || warnings[0] != "MISSING_DECISION" {
		t.Fatalf("expected missing decision to fail closed, got status=%q flags=%v warnings=%v", status, flags, warnings)
	}
}

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

func TestEvaluateDecisionMissingInputsFailClosed(t *testing.T) {
	t.Run("missing attempt", func(t *testing.T) {
		status, flags, warnings := EvaluateDecision(nil, &DiditDecisionResponse{})
		if status != "IN_REVIEW" || len(flags) != 0 || len(warnings) != 1 || warnings[0] != "MISSING_DECISION" {
			t.Fatalf("got status=%q flags=%v warnings=%v", status, flags, warnings)
		}
	})

	t.Run("missing decision", func(t *testing.T) {
		status, flags, warnings := EvaluateDecision(&IdentityVerificationAttempt{}, nil)
		if status != "IN_REVIEW" || len(flags) != 0 || len(warnings) != 1 || warnings[0] != "MISSING_DECISION" {
			t.Fatalf("got status=%q flags=%v warnings=%v", status, flags, warnings)
		}
	})
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
