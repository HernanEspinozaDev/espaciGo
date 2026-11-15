package verification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	DiditLiveURL    = "https://verification.didit.me"
	DiditSandboxURL = "https://verification-sandbox.didit.me"
)

var (
	ErrDiditRateLimited = errors.New("didit rate limited")
	ErrDiditUnavailable = errors.New("didit service unavailable")
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
	FirstName             string   `json:"first_name"`
	LastName              string   `json:"last_name"`
	DateOfBirth           string   `json:"date_of_birth"`
	IDCountry             string   `json:"id_country"`
	DocumentType          string   `json:"document_type"`
	IdentificationNumber  string   `json:"identification_number"`
	ExpectedDocumentTypes []string `json:"expected_document_types,omitempty"`
}
type CreateSessionRequest struct {
	WorkflowID      string          `json:"workflow_id"`
	VendorData      string          `json:"vendor_data"`
	Callback        string          `json:"callback,omitempty"`
	Language        string          `json:"language,omitempty"`
	ExpectedDetails ExpectedDetails `json:"expected_details"`
}
type CreateSessionResponse struct {
	SessionID  string `json:"session_id"`
	SessionURL string `json:"url"`
}

// DiditWarningEntry represents a warning entry inside a Didit V3 decision node.
type DiditWarningEntry struct {
	Risk             string `json:"risk"`
	ShortDescription string `json:"short_description"`
}

// DiditIDVerificationNode represents an ID verification node in Didit V3 decision responses.
// Media URLs (portraits, document images, videos) are intentionally omitted so they are discarded on unmarshal.
type DiditIDVerificationNode struct {
	Status               string              `json:"status"`
	FirstName            string              `json:"first_name"`
	LastName             string              `json:"last_name"`
	FullName             string              `json:"full_name"`
	DateOfBirth          string              `json:"date_of_birth"`
	IdentificationNumber string              `json:"identification_number"`
	PersonalNumber       string              `json:"personal_number"`
	IssuingState         string              `json:"issuing_state"`
	DocumentType         string              `json:"document_type"`
	ExpirationDate       string              `json:"expiration_date"`
	Warnings             []DiditWarningEntry `json:"warnings"`
}

// DiditLivenessNode represents a liveness check node in Didit V3 decision responses.
type DiditLivenessNode struct {
	Status   string              `json:"status"`
	Score    float64             `json:"score,omitempty"`
	Warnings []DiditWarningEntry `json:"warnings"`
}

// DiditFaceMatchNode represents a 1:1 face match node in Didit V3 decision responses.
type DiditFaceMatchNode struct {
	Status   string              `json:"status"`
	Score    float64             `json:"score,omitempty"`
	Warnings []DiditWarningEntry `json:"warnings"`
}

type DiditDecisionResponse struct {
	SessionID            string                    `json:"session_id"`
	WorkflowID           string                    `json:"workflow_id,omitempty"`
	Status               string                    `json:"status"`
	VendorData           string                    `json:"vendor_data,omitempty"`
	IDCountry            string                    `json:"id_country"`
	DocumentType         string                    `json:"document_type"`
	FullName             string                    `json:"full_name"`
	DateOfBirth          string                    `json:"date_of_birth"`
	IdentificationNumber string                    `json:"identification_number"`
	Liveness             bool                      `json:"liveness_passed"`
	FaceMatch            bool                      `json:"face_match_passed"`
	Warnings             []string                  `json:"warnings"`
	IDVerifications      []DiditIDVerificationNode `json:"id_verifications,omitempty"`
	LivenessChecks       []DiditLivenessNode       `json:"liveness_checks,omitempty"`
	FaceMatches          []DiditFaceMatchNode      `json:"face_matches,omitempty"`
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
	return &HTTPDiditClient{BaseURL: base, APIKey: apiKey, WebhookSecret: secret, HTTP: &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}
func (c *HTTPDiditClient) request(ctx context.Context, method, path string, in, out any) error {
	var bodyReader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bodyReader)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("x-api-key", c.APIKey)
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := h.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDiditUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrDiditRateLimited
	}
	if resp.StatusCode >= 500 {
		return ErrDiditUnavailable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("didit request failed")
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	return dec.Decode(out)
}
func (c *HTTPDiditClient) CreateSession(ctx context.Context, r CreateSessionRequest) (CreateSessionResponse, error) {
	var out CreateSessionResponse
	err := c.request(ctx, http.MethodPost, "/v3/session/", r, &out)
	return out, err
}
func (c *HTTPDiditClient) GetSessionDecision(ctx context.Context, id string) (DiditDecisionResponse, error) {
	var out DiditDecisionResponse
	err := c.request(ctx, http.MethodGet, "/v3/session/"+id+"/decision/", nil, &out)
	return out, err
}
func (c *HTTPDiditClient) DeleteSession(ctx context.Context, id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ErrInvalid
	}
	return c.request(ctx, http.MethodDelete, "/v3/session/"+trimmed+"/delete/", nil, nil)
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

func (c *HTTPDiditClient) VerifyWebhookSignatureV2(signature string, payload []byte) bool {
	canonical, err := CanonicalDiditWebhookPayload(payload)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(c.WebhookSecret))
	_, _ = mac.Write(canonical)
	signature = strings.TrimPrefix(signature, "sha256=")
	provided, err := hex.DecodeString(signature)
	return err == nil && hmac.Equal(mac.Sum(nil), provided)
}

// CanonicalDiditWebhookPayload sorts object keys recursively and normalizes
// numeric values for Didit's V2 webhook signature.
func CanonicalDiditWebhookPayload(payload []byte) ([]byte, error) {
	if !utf8.Valid(payload) {
		return nil, errors.New("invalid webhook JSON encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("trailing webhook JSON")
	}
	var output bytes.Buffer
	if err := appendCanonicalJSON(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendCanonicalJSON(output *bytes.Buffer, value any) error {
	switch item := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		output.WriteString(strconv.FormatBool(item))
	case string:
		appendCanonicalString(output, item)
	case json.Number:
		number, err := canonicalNumber(item.String())
		if err != nil {
			return err
		}
		output.WriteString(number)
	case []any:
		output.WriteByte('[')
		for i, element := range item {
			if i != 0 {
				output.WriteByte(',')
			}
			if err := appendCanonicalJSON(output, element); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for i, key := range keys {
			if i != 0 {
				output.WriteByte(',')
			}
			appendCanonicalString(output, key)
			output.WriteByte(':')
			if err := appendCanonicalJSON(output, item[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return errors.New("unsupported webhook JSON value")
	}
	return nil
}

func appendCanonicalString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, char := range value {
		switch char {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(char)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if char < 0x20 {
				fmt.Fprintf(output, `\u%04x`, char)
			} else {
				output.WriteRune(char)
			}
		}
	}
	output.WriteByte('"')
}

func canonicalNumber(text string) (string, error) {
	if !strings.ContainsAny(text, ".eE") {
		integer, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return "", errors.New("invalid webhook number")
		}
		return integer.String(), nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return "", errors.New("invalid webhook number")
	}
	if math.Trunc(value) == value {
		integer, _ := new(big.Float).SetFloat64(value).Int(nil)
		return integer.String(), nil
	}
	return pythonFloatRepresentation(value), nil
}

func pythonFloatRepresentation(value float64) string {
	text := strconv.FormatFloat(value, 'g', -1, 64)
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign = "-"
		text = text[1:]
	}
	parts := strings.SplitN(text, "e", 2)
	mantissa := parts[0]
	explicitExponent := 0
	if len(parts) == 2 {
		explicitExponent, _ = strconv.Atoi(parts[1])
	}
	decimalPosition := strings.IndexByte(mantissa, '.')
	if decimalPosition < 0 {
		decimalPosition = len(mantissa)
	}
	digits := strings.ReplaceAll(mantissa, ".", "")
	firstNonZero := strings.IndexAny(digits, "123456789")
	if firstNonZero < 0 {
		return "0"
	}
	exponent := decimalPosition - firstNonZero - 1 + explicitExponent
	digits = strings.TrimLeft(digits, "0")
	if exponent >= -4 && exponent < 16 {
		point := exponent + 1
		switch {
		case point <= 0:
			return sign + "0." + strings.Repeat("0", -point) + digits
		case point >= len(digits):
			return sign + digits + strings.Repeat("0", point-len(digits))
		default:
			return sign + digits[:point] + "." + digits[point:]
		}
	}
	exponentText := fmt.Sprintf("%+03d", exponent)
	return sign + digits[:1] + scientificFraction(digits[1:]) + "e" + exponentText
}

func scientificFraction(digits string) string {
	if digits == "" {
		return ""
	}
	return "." + digits
}

func isDiditApprovedStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "APPROVED":
		return true
	default:
		return false
	}
}

func normalizeDiditDocumentType(docType string) string {
	switch strings.ToUpper(strings.TrimSpace(docType)) {
	case "ID", "IDENTITY CARD", "IDENTITY_CARD":
		return "ID"
	default:
		return strings.TrimSpace(docType)
	}
}

func EvaluateDecision(a *IdentityVerificationAttempt, d *DiditDecisionResponse) (string, map[string]interface{}, []string) {
	flags := map[string]interface{}{}
	if a == nil || d == nil {
		return "IN_REVIEW", flags, []string{"MISSING_DECISION"}
	}

	eff := *d
	warnings := append([]string(nil), d.Warnings...)
	hasV3Nodes := len(d.IDVerifications) > 0 || len(d.LivenessChecks) > 0 || len(d.FaceMatches) > 0
	if hasV3Nodes {
		if len(d.IDVerifications) == 0 || len(d.LivenessChecks) == 0 || len(d.FaceMatches) == 0 {
			flags["missing_node"] = true
		}
		if len(d.IDVerifications) > 0 {
			idNode := d.IDVerifications[0]
			if !isDiditApprovedStatus(idNode.Status) {
				flags["id_verification_unapproved"] = true
			}
			if eff.FullName == "" {
				if strings.TrimSpace(idNode.FullName) != "" {
					eff.FullName = strings.TrimSpace(idNode.FullName)
				} else {
					eff.FullName = strings.TrimSpace(idNode.FirstName + " " + idNode.LastName)
				}
			}
			if eff.DateOfBirth == "" {
				eff.DateOfBirth = strings.TrimSpace(idNode.DateOfBirth)
			}
			if eff.IdentificationNumber == "" {
				if strings.TrimSpace(idNode.IdentificationNumber) != "" {
					eff.IdentificationNumber = strings.TrimSpace(idNode.IdentificationNumber)
				} else {
					eff.IdentificationNumber = strings.TrimSpace(idNode.PersonalNumber)
				}
			}
			if eff.IDCountry == "" {
				eff.IDCountry = strings.ToUpper(strings.TrimSpace(idNode.IssuingState))
			}
			if eff.DocumentType == "" {
				eff.DocumentType = normalizeDiditDocumentType(idNode.DocumentType)
			}
			if strings.TrimSpace(idNode.ExpirationDate) != "" {
				if exp, err := time.Parse("2006-01-02", strings.TrimSpace(idNode.ExpirationDate)); err != nil || exp.Before(time.Now().UTC().Truncate(24*time.Hour)) {
					flags["document_expired"] = true
				}
			}
			for _, w := range idNode.Warnings {
				if strings.TrimSpace(w.Risk) != "" {
					warnings = append(warnings, strings.TrimSpace(w.Risk))
				}
			}
		}
		if len(d.LivenessChecks) > 0 {
			liveNode := d.LivenessChecks[0]
			eff.Liveness = isDiditApprovedStatus(liveNode.Status)
			if !eff.Liveness {
				flags["liveness_unapproved"] = true
			}
			for _, w := range liveNode.Warnings {
				if strings.TrimSpace(w.Risk) != "" {
					warnings = append(warnings, strings.TrimSpace(w.Risk))
				}
			}
		}
		if len(d.FaceMatches) > 0 {
			faceNode := d.FaceMatches[0]
			eff.FaceMatch = isDiditApprovedStatus(faceNode.Status)
			if !eff.FaceMatch {
				flags["face_match_unapproved"] = true
			}
			for _, w := range faceNode.Warnings {
				if strings.TrimSpace(w.Risk) != "" {
					warnings = append(warnings, strings.TrimSpace(w.Risk))
				}
			}
		}
	}

	critical := map[string]bool{
		"FULL_NAME_MISMATCH_WITH_PROVIDED":             true,
		"NAME_DOES_NOT_MATCH_PROVIDED_DATA":            true,
		"DOB_MISMATCH_WITH_PROVIDED":                   true,
		"DATE_OF_BIRTH_DOES_NOT_MATCH_PROVIDED_DATA":   true,
		"COUNTRY_MISMATCH_WITH_PROVIDED":               true,
		"IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED": true,
		"ID_NUMBER_DOES_NOT_MATCH_PROVIDED_DATA":       true,
		"DOCUMENT_EXPIRED":                             true,
	}
	for _, w := range warnings {
		if critical[w] {
			flags[w] = true
		}
	}
	if eff.SessionID != "" && a.DiditSessionID != "" && eff.SessionID != a.DiditSessionID {
		flags["session_id"] = true
	}
	if eff.WorkflowID != "" && a.WorkflowID != "" && eff.WorkflowID != a.WorkflowID {
		flags["workflow_id"] = true
	}
	name := strings.ToLower(strings.Join(strings.Fields(eff.FullName), " "))
	expected := strings.ToLower(strings.Join(strings.Fields(a.DeclaredFirstName+" "+a.DeclaredLastName), " "))
	if name != expected {
		flags["full_name"] = true
	}
	gotDOB := eff.DateOfBirth
	if t, e := time.Parse("2006-01-02", gotDOB); e == nil {
		gotDOB = t.Format("2006-01-02")
	}
	if gotDOB != a.DeclaredDateOfBirth {
		flags["date_of_birth"] = true
	}
	rut, e := NormalizeRut(eff.IdentificationNumber)
	expectedRut, ee := NormalizeRut(a.DeclaredRUTNormalized)
	if e != nil || ee != nil || rut != expectedRut {
		flags["rut"] = true
	}
	if eff.IDCountry != a.IDCountry {
		flags["country"] = true
	}
	if normalizeDiditDocumentType(eff.DocumentType) != a.DocumentType {
		flags["document_type"] = true
	}
	if !isDiditApprovedStatus(eff.Status) {
		return "IN_REVIEW", flags, warnings
	}
	if len(flags) > 0 || !eff.Liveness || !eff.FaceMatch || a.DiditSessionID == "" {
		return "IN_REVIEW", flags, warnings
	}
	return "APPROVED", flags, warnings
}
