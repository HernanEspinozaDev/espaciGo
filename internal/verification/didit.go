package verification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DiditLiveURL    = "https://verification.didit.me"
	DiditSandboxURL = "https://verification-sandbox.didit.me"
)

type IdentityVerificationAttempt struct {
	ID                    string    `json:"id"`
	AccountID             string    `json:"-"`
	Status                string    `json:"status"`
	IDCountry             string    `json:"id_country"`
	DocumentType          string    `json:"document_type"`
	DeclaredFirstName     string    `json:"-"`
	DeclaredLastName      string    `json:"-"`
	DeclaredDateOfBirth   string    `json:"-"`
	DeclaredRUTNormalized string    `json:"-"`
	DiditSessionID        string    `json:"-"`
	DiditSessionURL       string    `json:"redirect_url,omitempty"`
	WorkflowID            string    `json:"-"`
	Environment           string    `json:"-"`
	ConsentVersion        string    `json:"-"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
type DiditWebhookInboxEvent struct {
	EventID   string          `json:"event_id"`
	SessionID string          `json:"session_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"-"`
	Signature string          `json:"-"`
}
type ExpectedDetails struct {
	FirstName            string `json:"first_name"`
	LastName             string `json:"last_name"`
	DateOfBirth          string `json:"date_of_birth"`
	IDCountry            string `json:"id_country"`
	DocumentType         string `json:"document_type"`
	IdentificationNumber string `json:"identification_number"`
}
type CreateSessionRequest struct {
	WorkflowID      string          `json:"workflow_id"`
	VendorData      string          `json:"vendor_data"`
	ExpectedDetails ExpectedDetails `json:"expected_details"`
}
type CreateSessionResponse struct {
	SessionID  string `json:"session_id"`
	SessionURL string `json:"url"`
}
type DiditDecisionResponse struct {
	SessionID            string   `json:"session_id"`
	Status               string   `json:"status"`
	IDCountry            string   `json:"id_country"`
	DocumentType         string   `json:"document_type"`
	FullName             string   `json:"full_name"`
	DateOfBirth          string   `json:"date_of_birth"`
	IdentificationNumber string   `json:"identification_number"`
	Liveness             bool     `json:"liveness_passed"`
	FaceMatch            bool     `json:"face_match_passed"`
	Warnings             []string `json:"warnings"`
}
type DiditClient interface {
	CreateSession(context.Context, CreateSessionRequest) (CreateSessionResponse, error)
	GetSessionDecision(context.Context, string) (DiditDecisionResponse, error)
	VerifyWebhookSignature(signature string, payload []byte) bool
}
type HTTPDiditClient struct {
	BaseURL, APIKey, WebhookSecret string
	HTTP                           *http.Client
}

func NewHTTPDiditClient(apiKey, secret string, sandbox bool, timeout time.Duration) *HTTPDiditClient {
	base := DiditLiveURL
	if sandbox {
		base = "https://verification-sandbox.didit.me"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPDiditClient{BaseURL: base, APIKey: apiKey, WebhookSecret: secret, HTTP: &http.Client{Timeout: timeout}}
}
func (c *HTTPDiditClient) request(ctx context.Context, method, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := h.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("didit request failed")
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	return dec.Decode(out)
}
func (c *HTTPDiditClient) CreateSession(ctx context.Context, r CreateSessionRequest) (CreateSessionResponse, error) {
	var out CreateSessionResponse
	err := c.request(ctx, http.MethodPost, "/v2/session/", r, &out)
	return out, err
}
func (c *HTTPDiditClient) GetSessionDecision(ctx context.Context, id string) (DiditDecisionResponse, error) {
	var out DiditDecisionResponse
	err := c.request(ctx, http.MethodGet, "/v2/session/"+id+"/decision/", nil, &out)
	return out, err
}
func (c *HTTPDiditClient) VerifyWebhookSignature(signature string, payload []byte) bool {
	mac := hmac.New(sha256.New, []byte(c.WebhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	signature = strings.TrimPrefix(signature, "sha256=")
	a, e1 := hex.DecodeString(expected)
	b, e2 := hex.DecodeString(signature)
	return e1 == nil && e2 == nil && hmac.Equal(a, b)
}

func EvaluateDecision(a *IdentityVerificationAttempt, d *DiditDecisionResponse) (string, map[string]interface{}, []string) {
	flags := map[string]interface{}{}
	if a == nil || d == nil {
		return "IN_REVIEW", flags, []string{"MISSING_DECISION"}
	}
	warnings := append([]string(nil), d.Warnings...)
	critical := map[string]bool{"FULL_NAME_MISMATCH_WITH_PROVIDED": true, "DOB_MISMATCH_WITH_PROVIDED": true, "COUNTRY_MISMATCH_WITH_PROVIDED": true, "IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED": true}
	for _, w := range warnings {
		if critical[w] {
			flags[w] = true
		}
	}
	name := strings.ToLower(strings.Join(strings.Fields(d.FullName), " "))
	expected := strings.ToLower(strings.Join(strings.Fields(a.DeclaredFirstName+" "+a.DeclaredLastName), " "))
	if name != expected {
		flags["full_name"] = true
	}
	gotDOB := d.DateOfBirth
	if t, e := time.Parse("2006-01-02", gotDOB); e == nil {
		gotDOB = t.Format("2006-01-02")
	}
	if gotDOB != a.DeclaredDateOfBirth {
		flags["date_of_birth"] = true
	}
	rut, e := NormalizeRut(d.IdentificationNumber)
	expectedRut, ee := NormalizeRut(a.DeclaredRUTNormalized)
	if e != nil || ee != nil || rut != expectedRut {
		flags["rut"] = true
	}
	if d.IDCountry != a.IDCountry {
		flags["country"] = true
	}
	if d.DocumentType != a.DocumentType {
		flags["document_type"] = true
	}
	if d.Status != "Approved" && d.Status != "APPROVED" {
		return "IN_REVIEW", flags, warnings
	}
	if len(flags) > 0 || !d.Liveness || !d.FaceMatch || a.DiditSessionID == "" {
		return "IN_REVIEW", flags, warnings
	}
	return "APPROVED", flags, warnings
}
