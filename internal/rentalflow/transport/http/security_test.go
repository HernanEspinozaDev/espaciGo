package rentalflowhttp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func TestQuoteRejectsQueryParameters(t *testing.T) {
	quote := &quoteStub{}
	h := newTestHandler(&authStub{principal: identity.Principal{AccountID: testActorID}}, &availabilityStub{}, quote, &previewStub{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes?return_url=https://evil.invalid", strings.NewReader(`{"space_id":"`+testSpaceID+`","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || quote.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, quote.calls, w.Body.String())
	}
}

func TestQuoteRejectsDuplicateAuthorizationHeaders(t *testing.T) {
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	quote := &quoteStub{}
	h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(`{"space_id":"`+testSpaceID+`","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Add("Authorization", "Bearer first-token")
	r.Header.Add("Authorization", "Bearer second-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || w.Header().Get("WWW-Authenticate") != "Bearer" || auth.calls != 0 || quote.calls != 0 {
		t.Fatalf("status=%d challenge=%q auth_calls=%d quote_calls=%d body=%s", w.Code, w.Header().Get("WWW-Authenticate"), auth.calls, quote.calls, w.Body.String())
	}
}

func TestQuoteRejectsDuplicateContentTypeHeaders(t *testing.T) {
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	quote := &quoteStub{}
	h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(`{"space_id":"`+testSpaceID+`","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`))
	r.Header.Set("Authorization", "Bearer token")
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType || quote.calls != 0 {
		t.Fatalf("status=%d quote_calls=%d body=%s", w.Code, quote.calls, w.Body.String())
	}
}

func TestPreviewRejectsBodyAndQueryParameters(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		body string
	}{
		{name: "body", url: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview", body: `{"rut":"12345678-5"}`},
		{name: "query", url: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview?identity=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preview := &previewStub{}
			h := newTestHandler(&authStub{principal: identity.Principal{AccountID: testActorID}}, &availabilityStub{}, &quoteStub{}, preview)
			r := httptest.NewRequest(http.MethodPost, tc.url, strings.NewReader(tc.body))
			if tc.name == "body" {
				r.ContentLength = -1 // exercise chunked/unknown-length request behavior
			}
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || preview.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, preview.calls, w.Body.String())
			}
		})
	}
}

func TestQuoteValidatesMalformedTrailingAndBodySizeBoundaries(t *testing.T) {
	validBody := `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`
	cases := []struct {
		name           string
		contentType    string
		body           string
		unknownLength  bool
		wantStatus     int
		wantQuoteCalls int
	}{
		{name: "empty body", contentType: "application/json", body: "", wantStatus: http.StatusBadRequest},
		{name: "malformed JSON", contentType: "application/json", body: `{"space_id":`, wantStatus: http.StatusBadRequest},
		{name: "truncated object after field", contentType: "application/json", body: `{"space_id":"` + testSpaceID + `"`, wantStatus: http.StatusBadRequest},
		{name: "truncated object after comma", contentType: "application/json", body: `{"space_id":"` + testSpaceID + `",`, wantStatus: http.StatusBadRequest},
		{name: "mismatched object closing delimiter", contentType: "application/json", body: `{"space_id":"` + testSpaceID + `"]`, wantStatus: http.StatusBadRequest},
		{name: "top-level array", contentType: "application/json", body: `[]`, wantStatus: http.StatusBadRequest},
		{name: "top-level null", contentType: "application/json", body: `null`, wantStatus: http.StatusBadRequest},
		{name: "multiple JSON values", contentType: "application/json", body: validBody + ` {}`, wantStatus: http.StatusBadRequest},
		{name: "trailing non-JSON data", contentType: "application/json", body: validBody + ` trailing`, wantStatus: http.StatusBadRequest},
		{name: "multiple values after null", contentType: "application/json", body: `null {}`, wantStatus: http.StatusBadRequest},
		{name: "exactly 16 KiB", contentType: "application/json", body: validBody + strings.Repeat(" ", 16*1024-len(validBody)), wantStatus: http.StatusCreated, wantQuoteCalls: 1},
		{name: "oversized chunked JSON", contentType: "application/json", body: strings.Repeat(" ", 16*1024+1), unknownLength: true, wantStatus: http.StatusBadRequest},
		{name: "wrong media type", contentType: "text/plain", body: validBody, wantStatus: http.StatusUnsupportedMediaType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := &quoteStub{}
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(tc.body))
			if tc.unknownLength {
				r.ContentLength = -1
			}
			r.Header.Set("Content-Type", tc.contentType)
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.wantStatus || quote.calls != tc.wantQuoteCalls {
				t.Fatalf("status=%d want=%d quote_calls=%d want=%d body=%s", w.Code, tc.wantStatus, quote.calls, tc.wantQuoteCalls, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control=%q", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestRentalFlowCORSAllowsOnlyConfiguredOriginWithoutAuthenticatingPreflight(t *testing.T) {
	const allowedOrigin = "https://rental.example.test"
	for _, tc := range []struct {
		name            string
		origin          string
		wantStatus      int
		wantAllowOrigin string
	}{
		{name: "configured origin", origin: allowedOrigin, wantStatus: http.StatusNoContent, wantAllowOrigin: allowedOrigin},
		{name: "unconfigured origin", origin: "https://attacker.example.test", wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			h := New(auth, &availabilityStub{}, &quoteStub{}, &previewStub{}, []string{allowedOrigin})
			r := httptest.NewRequest(http.MethodOptions, "/api/v1/rental-quotes", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Access-Control-Request-Method", "POST")
			r.Header.Set("Access-Control-Request-Headers", "content-type,authorization")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.wantStatus || w.Header().Get("Access-Control-Allow-Origin") != tc.wantAllowOrigin {
				t.Fatalf("status=%d allow_origin=%q headers=%v", w.Code, w.Header().Get("Access-Control-Allow-Origin"), w.Header())
			}
			if auth.calls != 0 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Vary") != "Origin" {
				t.Fatalf("auth_calls=%d cache=%q vary=%q", auth.calls, w.Header().Get("Cache-Control"), w.Header().Get("Vary"))
			}
		})
	}
}

func TestRentalFlowRejectsDuplicateOriginHeadersBeforeDispatch(t *testing.T) {
	const allowedOrigin = "https://rental.example.test"
	for _, tc := range []struct {
		name      string
		method    string
		path      string
		preflight bool
	}{
		{name: "public availability", method: http.MethodGet, path: "/api/v1/spaces/" + testSpaceID + "/rental-availability?date=2030-01-02&duration=1"},
		{name: "private preview", method: http.MethodPost, path: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview"},
		{name: "preflight", method: http.MethodOptions, path: "/api/v1/spaces/" + testSpaceID + "/rental-availability", preflight: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			availability := &availabilityStub{}
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			preview := &previewStub{result: rentalflow.ContractPreview{QuoteID: testQuoteID, State: "preview"}}
			h := New(auth, availability, &quoteStub{}, preview, []string{allowedOrigin})
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Add("Origin", allowedOrigin)
			r.Header.Add("Origin", "https://attacker.example.test")
			if tc.preflight {
				r.Header.Set("Access-Control-Request-Method", http.MethodGet)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" ||
				availability.calls != 0 || auth.calls != 0 || preview.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d allow_origin=%q availability=%d auth=%d preview=%d cache=%q body=%s", w.Code, w.Header().Get("Access-Control-Allow-Origin"), availability.calls, auth.calls, preview.calls, w.Header().Get("Cache-Control"), w.Body.String())
			}
		})
	}
}

func TestRentalFlowKnownRoutesReturnJSONMethodNotAllowed(t *testing.T) {
	for _, tc := range []struct {
		name           string
		method         string
		path           string
		allowedMethods string
	}{
		{name: "availability", method: http.MethodPost, path: "/api/v1/spaces/" + testSpaceID + "/rental-availability", allowedMethods: "GET, OPTIONS"},
		{name: "availability HEAD", method: http.MethodHead, path: "/api/v1/spaces/" + testSpaceID + "/rental-availability", allowedMethods: "GET, OPTIONS"},
		{name: "quote", method: http.MethodGet, path: "/api/v1/rental-quotes", allowedMethods: "POST, OPTIONS"},
		{name: "contract preview", method: http.MethodGet, path: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview", allowedMethods: "POST, OPTIONS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			availability := &availabilityStub{}
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			quote := &quoteStub{}
			preview := &previewStub{}
			h := New(auth, availability, quote, preview, nil)
			r := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var envelope struct {
				Error struct {
					Code      string `json:"code"`
					RequestID string `json:"request_id"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid method error envelope: %v body=%q", err, w.Body.String())
			}
			if w.Code != http.StatusMethodNotAllowed || envelope.Error.Code != "method_not_allowed" || envelope.Error.RequestID == "" ||
				w.Header().Get("Allow") != tc.allowedMethods || w.Header().Get("Cache-Control") != "no-store" ||
				availability.calls != 0 || auth.calls != 0 || quote.calls != 0 || preview.calls != 0 {
				t.Fatalf("status=%d code=%q request_id=%q allow=%q cache=%q calls=(availability:%d auth:%d quote:%d preview:%d) body=%s", w.Code, envelope.Error.Code, envelope.Error.RequestID, w.Header().Get("Allow"), w.Header().Get("Cache-Control"), availability.calls, auth.calls, quote.calls, preview.calls, w.Body.String())
			}
		})
	}
}

func TestRentalFlowReplacesCallerRequestIDAndCorrelatesErrorEnvelope(t *testing.T) {
	const suppliedID = "caller-controlled-request-id"
	availability := &availabilityStub{err: rentalflow.ErrUnavailable}
	h := New(&authStub{}, availability, &quoteStub{}, &previewStub{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	r.Header.Add("X-Request-ID", suppliedID)
	r.Header.Add("X-Request-ID", suppliedID+"-duplicate")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var envelope struct {
		Error struct {
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid error envelope: %v body=%q", err, w.Body.String())
	}
	requestIDs := w.Header().Values("X-Request-ID")
	if w.Code != http.StatusServiceUnavailable || availability.calls != 1 || len(requestIDs) != 1 ||
		requestIDs[0] == suppliedID || requestIDs[0] == suppliedID+"-duplicate" || !rentalflow.ValidUUID(requestIDs[0]) ||
		envelope.Error.RequestID != requestIDs[0] || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d calls=%d header request IDs=%q envelope request ID=%q cache=%q body=%s", w.Code, availability.calls, requestIDs, envelope.Error.RequestID, w.Header().Get("Cache-Control"), w.Body.String())
	}
}

func TestRentalFlowPassesCanceledRequestContextToDependencies(t *testing.T) {
	quoteBody, err := json.Marshal(map[string]string{
		"space_id": testSpaceID,
		"start_at": "2030-01-02T13:00:00Z",
		"end_at":   "2030-01-02T14:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                    string
		method                  string
		path                    string
		body                    string
		wantAuthCalls           int
		wantAvailabilityCalls   int
		wantQuoteCalls          int
		wantPreviewCalls        int
		wantAuthContext         bool
		wantAvailabilityContext bool
		wantQuoteContext        bool
		wantPreviewContext      bool
	}{
		{
			name: "public availability", method: http.MethodGet,
			path:                  "/api/v1/spaces/" + testSpaceID + "/rental-availability?date=2030-01-02&duration=1",
			wantAvailabilityCalls: 1, wantAvailabilityContext: true,
		},
		{
			name: "authenticated quote", method: http.MethodPost, path: "/api/v1/rental-quotes", body: string(quoteBody),
			wantAuthCalls: 1, wantQuoteCalls: 1, wantAuthContext: true, wantQuoteContext: true,
		},
		{
			name: "authenticated preview", method: http.MethodPost,
			path:          "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview",
			wantAuthCalls: 1, wantPreviewCalls: 1, wantAuthContext: true, wantPreviewContext: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			availability := &availabilityStub{err: rentalflow.ErrUnavailable}
			quote := &quoteStub{err: rentalflow.ErrUnavailable}
			preview := &previewStub{err: rentalflow.ErrUnavailable}
			h := newTestHandler(auth, availability, quote, preview)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)).WithContext(ctx)
			if tc.method == http.MethodPost {
				r.Header.Set("Authorization", "Bearer token")
				if tc.body != "" {
					r.Header.Set("Content-Type", "application/json")
				}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v body=%q", err, w.Body.String())
			}
			if w.Code != http.StatusServiceUnavailable || envelope.Error.Code != "service_unavailable" ||
				auth.calls != tc.wantAuthCalls || availability.calls != tc.wantAvailabilityCalls ||
				quote.calls != tc.wantQuoteCalls || preview.calls != tc.wantPreviewCalls ||
				(tc.wantAuthContext && !errors.Is(auth.ctxErr, context.Canceled)) ||
				(tc.wantAvailabilityContext && !errors.Is(availability.ctxErr, context.Canceled)) ||
				(tc.wantQuoteContext && !errors.Is(quote.ctxErr, context.Canceled)) ||
				(tc.wantPreviewContext && !errors.Is(preview.ctxErr, context.Canceled)) ||
				w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d code=%q calls=(auth:%d availability:%d quote:%d preview:%d) contexts=(%v,%v,%v,%v) cache=%q body=%s", w.Code, envelope.Error.Code, auth.calls, availability.calls, quote.calls, preview.calls, auth.ctxErr, availability.ctxErr, quote.ctxErr, preview.ctxErr, w.Header().Get("Cache-Control"), w.Body.String())
			}
		})
	}
}

func TestRentalFlowRejectsUnsupportedPreflightMethodAndHeaders(t *testing.T) {
	const origin = "https://rental.example.test"
	for _, tc := range []struct {
		name            string
		method          string
		headers         string
		duplicateMethod bool
	}{
		{name: "unsupported method", method: http.MethodDelete, headers: "content-type"},
		{name: "unsupported header", method: http.MethodPost, headers: "content-type,x-custom"},
		{name: "missing requested method", headers: "content-type"},
		{name: "duplicate requested method", method: http.MethodPost, headers: "content-type", duplicateMethod: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			h := New(auth, &availabilityStub{}, &quoteStub{}, &previewStub{}, []string{origin})
			r := httptest.NewRequest(http.MethodOptions, "/api/v1/rental-quotes", nil)
			r.Header.Set("Origin", origin)
			if tc.method != "" {
				r.Header.Set("Access-Control-Request-Method", tc.method)
				if tc.duplicateMethod {
					r.Header.Add("Access-Control-Request-Method", tc.method)
				}
			}
			r.Header.Set("Access-Control-Request-Headers", tc.headers)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden || auth.calls != 0 ||
				w.Header().Get("Access-Control-Allow-Methods") != "" || w.Header().Get("Access-Control-Allow-Headers") != "" {
				t.Fatalf("status=%d auth_calls=%d allow_methods=%q allow_headers=%q headers=%v body=%s", w.Code, auth.calls, w.Header().Get("Access-Control-Allow-Methods"), w.Header().Get("Access-Control-Allow-Headers"), w.Header(), w.Body.String())
			}
		})
	}
}

func TestRentalFlowAllowsConfiguredOriginOnPublicAvailabilityRequest(t *testing.T) {
	const origin = "https://rental.example.test"
	auth := &authStub{}
	availability := &availabilityStub{}
	h := New(auth, availability, &quoteStub{}, &previewStub{}, []string{origin})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != origin ||
		w.Header().Get("Access-Control-Allow-Headers") != "" || w.Header().Get("Access-Control-Allow-Methods") != "" ||
		w.Header().Get("Vary") != "Origin" || w.Header().Get("Cache-Control") != "no-store" ||
		availability.calls != 1 || auth.calls != 0 {
		t.Fatalf("status=%d headers=%v availability_calls=%d auth_calls=%d body=%s", w.Code, w.Header(), availability.calls, auth.calls, w.Body.String())
	}
}

func TestRentalFlowPreflightIsMountedForEveryContractRoute(t *testing.T) {
	const origin = "https://rental.example.test"
	routes := []struct {
		path   string
		method string
	}{
		{path: "/api/v1/spaces/" + testSpaceID + "/rental-availability", method: http.MethodGet},
		{path: "/api/v1/rental-quotes", method: http.MethodPost},
		{path: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview", method: http.MethodPost},
	}
	auth := &authStub{}
	h := New(auth, &availabilityStub{}, &quoteStub{}, &previewStub{}, []string{origin})
	for _, route := range routes {
		t.Run(route.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodOptions, route.path, nil)
			r.Header.Set("Origin", origin)
			r.Header.Set("Access-Control-Request-Method", route.method)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != origin ||
				w.Header().Get("Access-Control-Allow-Methods") != route.method ||
				w.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" || auth.calls != 0 {
				t.Fatalf("route=%s status=%d allow_origin=%q allow_methods=%q allow_headers=%q auth_calls=%d body=%s", route.path, w.Code, w.Header().Get("Access-Control-Allow-Origin"), w.Header().Get("Access-Control-Allow-Methods"), w.Header().Get("Access-Control-Allow-Headers"), auth.calls, w.Body.String())
			}
		})
	}
}

func TestRentalFlowRejectsUnconfiguredOriginOnActualRouteBeforeService(t *testing.T) {
	const allowedOrigin = "https://rental.example.test"
	availability := &availabilityStub{}
	h := New(&authStub{}, availability, &quoteStub{}, &previewStub{}, []string{allowedOrigin})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	r.Header.Set("Origin", "https://attacker.example.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" || availability.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d allow_origin=%q service_calls=%d cache=%q body=%s", w.Code, w.Header().Get("Access-Control-Allow-Origin"), availability.calls, w.Header().Get("Cache-Control"), w.Body.String())
	}
}

func TestRentalFlowRejectsUnconfiguredOriginBeforePrivatePreviewAuthorization(t *testing.T) {
	const allowedOrigin = "https://rental.example.test"
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	preview := &previewStub{}
	h := New(auth, &availabilityStub{}, &quoteStub{}, preview, []string{allowedOrigin})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	r.Header.Set("Origin", "https://attacker.example.test")
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" || auth.calls != 0 || preview.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d allow_origin=%q auth_calls=%d preview_calls=%d cache=%q body=%s", w.Code, w.Header().Get("Access-Control-Allow-Origin"), auth.calls, preview.calls, w.Header().Get("Cache-Control"), w.Body.String())
	}
}

func TestRentalFlowAllowsConfiguredOriginOnPrivatePreview(t *testing.T) {
	const origin = "https://rental.example.test"
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	preview := &previewStub{result: rentalflow.ContractPreview{QuoteID: testQuoteID, State: "preview"}}
	h := New(auth, &availabilityStub{}, &quoteStub{}, preview, []string{origin})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != origin ||
		w.Header().Get("Access-Control-Allow-Headers") != "" || w.Header().Get("Access-Control-Allow-Methods") != "" ||
		w.Header().Get("Vary") != "Origin" || w.Header().Get("Cache-Control") != "no-store" ||
		auth.calls != 1 || preview.calls != 1 || preview.actorID != testActorID || preview.quoteID != testQuoteID {
		t.Fatalf("status=%d headers=%v auth_calls=%d preview=%+v body=%s", w.Code, w.Header(), auth.calls, preview, w.Body.String())
	}
}

func TestConfiguredOriginCanReadPrivatePreviewAuthenticationChallenge(t *testing.T) {
	const origin = "https://rental.example.test"
	auth := &authStub{err: identity.ErrUnauthorized}
	preview := &previewStub{}
	h := New(auth, &availabilityStub{}, &quoteStub{}, preview, []string{origin})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || w.Header().Get("Access-Control-Allow-Origin") != origin ||
		w.Header().Get("WWW-Authenticate") != "Bearer" || w.Header().Get("Cache-Control") != "no-store" ||
		auth.calls != 1 || preview.calls != 0 {
		t.Fatalf("status=%d headers=%v auth_calls=%d preview_calls=%d body=%s", w.Code, w.Header(), auth.calls, preview.calls, w.Body.String())
	}
}

type failingEntropyReader struct{}

func (failingEntropyReader) Read([]byte) (int, error) {
	return 0, errors.New("synthetic entropy failure")
}

// This test replaces the process-wide crypto/rand.Reader; keep it non-parallel.
func TestRequestIDGenerationFailureStopsBeforeRouteDispatch(t *testing.T) {
	availability := &availabilityStub{}
	h := New(&authStub{}, availability, &quoteStub{}, &previewStub{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	w := httptest.NewRecorder()
	previousReader := rand.Reader
	rand.Reader = failingEntropyReader{}
	t.Cleanup(func() { rand.Reader = previousReader })
	h.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError || w.Header().Get("X-Request-ID") != "" ||
		w.Header().Get("Cache-Control") != "no-store" || availability.calls != 0 || strings.Contains(w.Body.String(), "synthetic entropy failure") {
		t.Fatalf("status=%d request_id=%q cache=%q availability_calls=%d body=%s", w.Code, w.Header().Get("X-Request-ID"), w.Header().Get("Cache-Control"), availability.calls, w.Body.String())
	}
}

type failedRequestBody struct{}

func (failedRequestBody) Read([]byte) (int, error) {
	return 0, errors.New("synthetic body read failure")
}

func TestPreviewRejectsBodyProbeReadFailureBeforeAuthentication(t *testing.T) {
	auth := &authStub{}
	preview := &previewStub{}
	h := newTestHandler(auth, &availabilityStub{}, &quoteStub{}, preview)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	r.Body = io.NopCloser(failedRequestBody{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || auth.calls != 0 || preview.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d auth_calls=%d preview_calls=%d cache=%q body=%s", w.Code, auth.calls, preview.calls, w.Header().Get("Cache-Control"), w.Body.String())
	}
}
