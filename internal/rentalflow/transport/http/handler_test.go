package rentalflowhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

const (
	testSpaceID = "11111111-1111-4111-8111-111111111111"
	testQuoteID = "22222222-2222-4222-8222-222222222222"
	testActorID = "33333333-3333-4333-8333-333333333333"
)

type authStub struct {
	principal identity.Principal
	err       error
	calls     int
	token     identity.Secret
	ctxErr    error
}

func (a *authStub) Authorize(ctx context.Context, token identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	a.calls++
	a.token = token
	if ctx != nil {
		a.ctxErr = ctx.Err()
	}
	return a.principal, a.err
}

type availabilityStub struct {
	result rentalflow.Availability
	err    error
	calls  int
	space  string
	date   string
	days   int
	ctxErr error
}

func (s *availabilityStub) ListPublic(ctx context.Context, space, date string, duration int) (rentalflow.Availability, error) {
	s.calls++
	s.space, s.date, s.days = space, date, duration
	if ctx != nil {
		s.ctxErr = ctx.Err()
	}
	return s.result, s.err
}

type quoteStub struct {
	result  rentalflow.Quote
	err     error
	calls   int
	actorID string
	request rentalflow.QuoteRequest
	ctxErr  error
}

func (s *quoteStub) Create(ctx context.Context, actor string, in rentalflow.QuoteRequest) (rentalflow.Quote, error) {
	s.calls++
	s.actorID, s.request = actor, in
	if ctx != nil {
		s.ctxErr = ctx.Err()
	}
	return s.result, s.err
}

type previewStub struct {
	result  rentalflow.ContractPreview
	err     error
	calls   int
	actorID string
	quoteID string
	ctxErr  error
}

func (s *previewStub) Preview(ctx context.Context, actor, quote string) (rentalflow.ContractPreview, error) {
	s.calls++
	s.actorID, s.quoteID = actor, quote
	if ctx != nil {
		s.ctxErr = ctx.Err()
	}
	return s.result, s.err
}

func newTestHandler(auth *authStub, availability *availabilityStub, quote *quoteStub, preview *previewStub) http.Handler {
	return New(auth, availability, quote, preview, nil)
}

func TestPublicAvailabilityIsUnauthenticatedMinimizedAllowlistedAndNoStore(t *testing.T) {
	availability := &availabilityStub{result: rentalflow.Availability{
		SpaceID: testSpaceID, TimeZone: "America/Santiago", RateUnit: "hora",
		CheckedAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		Items:     []rentalflow.Interval{{StartAt: time.Date(2030, 1, 2, 13, 0, 0, 0, time.UTC), EndAt: time.Date(2030, 1, 2, 14, 0, 0, 0, time.UTC)}},
	}}
	auth := &authStub{}
	h := newTestHandler(auth, availability, &quoteStub{}, &previewStub{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || auth.calls != 0 || availability.calls != 1 {
		t.Fatalf("status=%d authCalls=%d availabilityCalls=%d body=%s", w.Code, auth.calls, availability.calls, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q", w.Header().Get("Cache-Control"))
	}
	if availability.space != testSpaceID || availability.date != "2030-01-02" || availability.days != 1 {
		t.Fatalf("service args=%+v", availability)
	}
	for _, forbidden := range []string{"owner_id", "reservation_id", "occupant", "block_reason"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("public response contains private field %q: %s", forbidden, w.Body.String())
		}
	}
	assertKeys := func(fields map[string]json.RawMessage, expected ...string) {
		t.Helper()
		if len(fields) != len(expected) {
			t.Fatalf("JSON fields=%v, want exactly %v", fields, expected)
		}
		for _, key := range expected {
			if _, ok := fields[key]; !ok {
				t.Fatalf("JSON fields=%v, want key %q", fields, key)
			}
		}
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid public response JSON: %v", err)
	}
	assertKeys(envelope, "data")
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatalf("invalid availability data JSON: %v", err)
	}
	assertKeys(data, "space_id", "time_zone", "rate_unit", "checked_at", "items")
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(data["items"], &items); err != nil {
		t.Fatalf("invalid availability items JSON: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("availability items=%v, want one synthetic interval", items)
	}
	assertKeys(items[0], "start_at", "end_at")
}

func TestAvailabilityRejectsAmbiguousAndUnknownQueryBeforeService(t *testing.T) {
	for _, query := range []string{
		"duration=1",
		"date=2030-01-02",
		"date=2030-01-02&date=2030-01-03&duration=1",
		"date=2030-01-02&duration=1&duration=2",
		"date=2030-01-02&duration=1&owner_id=x",
		"date=2030-01-02&duration=1&owner_id=%ZZ",
		"date=2030-01-02&duration=1&broken=%",
		"date=2030-01-02&duration=0",
		"date=2030-01-02&duration=5",
		"date=2030-01-02&duration=one",
		"date=2030-01-02&duration=999999999999999999999999999999999999",
	} {
		t.Run(query, func(t *testing.T) {
			availability := &availabilityStub{}
			h := newTestHandler(&authStub{}, availability, &quoteStub{}, &previewStub{})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?"+query, nil))
			if w.Code != http.StatusBadRequest || availability.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, availability.calls, w.Body.String())
			}
		})
	}
}

func TestMalformedRentalIdentifiersAndDatesAreRejectedBeforeServiceDispatch(t *testing.T) {
	cases := []struct {
		name     string
		method   string
		path     string
		body     string
		auth     bool
		wantAuth int
	}{
		{name: "availability UUID", method: http.MethodGet, path: "/api/v1/spaces/not-a-uuid/rental-availability?date=2030-01-02&duration=1"},
		{name: "availability impossible date", method: http.MethodGet, path: "/api/v1/spaces/" + testSpaceID + "/rental-availability?date=2030-02-30&duration=1"},
		{name: "quote space UUID", method: http.MethodPost, path: "/api/v1/rental-quotes", body: `{"space_id":"not-a-uuid","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`, auth: true, wantAuth: 1},
		{name: "preview quote UUID", method: http.MethodPost, path: "/api/v1/rental-quotes/not-a-uuid/contract-preview", auth: true, wantAuth: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			availability := &availabilityStub{}
			quote := &quoteStub{}
			preview := &previewStub{}
			h := newTestHandler(auth, availability, quote, preview)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.auth {
				r.Header.Set("Authorization", "Bearer token")
			}
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || auth.calls != tc.wantAuth || availability.calls != 0 || quote.calls != 0 || preview.calls != 0 {
				t.Fatalf("status=%d auth_calls=%d availability=%d quote=%d preview=%d body=%s", w.Code, auth.calls, availability.calls, quote.calls, preview.calls, w.Body.String())
			}
		})
	}
}

func TestMalformedQuoteTimestampsAreRejectedBeforeServiceDispatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		startAt string
		endAt   string
	}{
		{name: "malformed start", startAt: "not-a-timestamp", endAt: "2030-01-02T14:00:00Z"},
		{name: "impossible end date", startAt: "2030-01-02T13:00:00Z", endAt: "2030-02-30T14:00:00Z"},
		{name: "zero start timestamp", startAt: "0001-01-01T00:00:00Z", endAt: "2030-01-02T14:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quote := &quoteStub{}
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
			body := `{"space_id":"` + testSpaceID + `","start_at":"` + tc.startAt + `","end_at":"` + tc.endAt + `"}`
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || auth.calls != 1 || quote.calls != 0 {
				t.Fatalf("status=%d auth_calls=%d quote_calls=%d body=%s", w.Code, auth.calls, quote.calls, w.Body.String())
			}
		})
	}
}

func TestQuoteRejectsDuplicateJSONFieldsBeforeServiceDispatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "exact duplicate", body: `{"space_id":"` + testSpaceID + `","space_id":"44444444-4444-4444-8444-444444444444","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`},
		{name: "case-insensitive duplicate", body: `{"space_id":"` + testSpaceID + `","SPACE_ID":"44444444-4444-4444-8444-444444444444","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`},
		{name: "escaped duplicate", body: `{"space_id":"` + testSpaceID + `","space\u005fid":"44444444-4444-4444-8444-444444444444","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`},
		{name: "Unicode case-fold duplicate", body: `{"space_id":"` + testSpaceID + `","\u017fpace_id":"44444444-4444-4444-8444-444444444444","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quote := &quoteStub{}
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || auth.calls != 1 || quote.calls != 0 {
				t.Fatalf("status=%d auth_calls=%d quote_calls=%d body=%s", w.Code, auth.calls, quote.calls, w.Body.String())
			}
		})
	}
}

func FuzzRejectDuplicateJSONKeys(f *testing.F) {
	for _, seed := range [][2]string{
		{"space_id", "space_id"},
		{"space_id", "SPACE_ID"},
		{"space_id", "ſpace_id"},
		{"space_id", "time_zone"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, first, second string) {
		if len(first) > 256 || len(second) > 256 {
			t.Skip()
		}
		firstKey, err := json.Marshal(first)
		if err != nil {
			t.Fatal(err)
		}
		secondKey, err := json.Marshal(second)
		if err != nil {
			t.Fatal(err)
		}
		body := []byte("{" + string(firstKey) + ":null," + string(secondKey) + ":null}")
		gotErr := rejectDuplicateJSONKeys(body) != nil
		wantErr := strings.EqualFold(first, second) || containsNonASCII(first) || containsNonASCII(second)
		if gotErr != wantErr {
			t.Fatalf("rejectDuplicateJSONKeys(%q, %q) error=%v, want error=%v", first, second, gotErr, wantErr)
		}
	})
}

func FuzzRejectDuplicateJSONKeysRawBodyDoesNotPanic(f *testing.F) {
	validObject, err := json.Marshal(map[string]string{"space_id": "value"})
	if err != nil {
		f.Fatal(err)
	}
	truncatedObject := append([]byte(nil), validObject[:len(validObject)-1]...)
	duplicateObject := []byte{'{', '"', 'x', '"', ':', '1', ',', '"', 'x', '"', ':', '2', '}'}
	nestedDuplicates := []byte{'{', '"', 'n', 'e', 's', 't', 'e', 'd', '"', ':', '{', '"', 'x', '"', ':', '1', ',', '"', 'x', '"', ':', '2', '}', '}'}
	trailingComma := append(append([]byte(nil), validObject[:len(validObject)-1]...), ',', '}')
	for _, seed := range [][]byte{
		{}, []byte("{}"), []byte("[]"), []byte("null"), []byte("{"),
		truncatedObject, trailingComma, duplicateObject, nestedDuplicates,
		[]byte{0xff, 0xfe, '{', '}'},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 16*1024 {
			t.Skip()
		}
		_ = rejectDuplicateJSONKeys(body)
	})
}

func FuzzRentalQuoteHTTPBodyPreservesResponseEnvelope(f *testing.F) {
	validBody, err := json.Marshal(map[string]string{
		"space_id": testSpaceID,
		"start_at": "2030-01-02T13:00:00Z",
		"end_at":   "2030-01-02T14:00:00Z",
	})
	if err != nil {
		f.Fatal(err)
	}
	valid := string(validBody)
	// Exact body-size boundaries are covered by TestQuoteValidatesMalformedTrailingAndBodySizeBoundaries.
	for _, seed := range [][]byte{
		{}, []byte("{"), []byte("null"), []byte(valid),
		[]byte{0xff, 0xfe, '{', '}'},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 16*1024 {
			t.Skip()
		}
		auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
		quote := &quoteStub{}
		h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
		r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(string(body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer fuzz-session")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if auth.calls != 1 || auth.token != "fuzz-session" {
			t.Fatalf("body fuzz did not reach authenticated handler: auth calls=%d token=%q status=%d", auth.calls, auth.token, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" || !rentalflow.ValidUUID(w.Header().Get("X-Request-ID")) || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("status=%d cache=%q requestID=%q body=%q", w.Code, w.Header().Get("Cache-Control"), w.Header().Get("X-Request-ID"), w.Body.String())
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode response envelope: %v body=%s", err, w.Body.String())
		}
		switch w.Code {
		case http.StatusCreated:
			if quote.calls != 1 || len(envelope) != 1 || envelope["data"] == nil {
				t.Fatalf("created response calls=%d envelope=%v body=%s", quote.calls, envelope, w.Body.String())
			}
		case http.StatusBadRequest:
			if quote.calls != 0 || len(envelope) != 1 || envelope["error"] == nil {
				t.Fatalf("invalid body reached quote service: calls=%d envelope=%v body=%s", quote.calls, envelope, w.Body.String())
			}
		default:
			t.Fatalf("unexpected quote body status=%d calls=%d body=%s", w.Code, quote.calls, w.Body.String())
		}
	})
}

func FuzzRentalQuoteDispatchesMappedValidTimestamps(f *testing.F) {
	for _, seed := range []struct {
		startOffsetSeconds uint32
		durationSeconds    uint32
		startNanos         uint32
		endNanos           uint32
	}{{0, 1, 0, 0}, {59 * 24 * 60 * 60, 3600, 123456789, 987654321}, {731 * 24 * 60 * 60, 86400, 999999999, 1}} {
		f.Add(seed.startOffsetSeconds, seed.durationSeconds, seed.startNanos, seed.endNanos)
	}

	f.Fuzz(func(t *testing.T, rawStartOffset, rawDuration, rawStartNanos, rawEndNanos uint32) {
		startOffsetSeconds := rawStartOffset % (800 * 24 * 60 * 60)
		durationSeconds := rawDuration%86400 + 1
		start := time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC).
			Add(time.Duration(startOffsetSeconds)*time.Second + time.Duration(rawStartNanos%1_000_000_000))
		end := start.Add(time.Duration(durationSeconds)*time.Second + time.Duration(rawEndNanos%1_000_000_000))
		startAt, endAt := start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)
		body, err := json.Marshal(struct {
			SpaceID string `json:"space_id"`
			StartAt string `json:"start_at"`
			EndAt   string `json:"end_at"`
		}{SpaceID: testSpaceID, StartAt: startAt, EndAt: endAt})
		if err != nil {
			t.Fatalf("marshal synthetic quote request: %v", err)
		}

		auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
		quote := &quoteStub{}
		handler := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer synthetic-quote-session")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusCreated || auth.calls != 1 || auth.token != "synthetic-quote-session" || quote.calls != 1 || quote.actorID != testActorID || quote.request.SpaceID != testSpaceID || quote.request.StartAt != startAt || quote.request.EndAt != endAt {
			t.Fatalf("valid mapped quote dispatch mismatch: status=%d authCalls=%d quoteCalls=%d actor=%q space=%q start=%q end=%q body=%s", response.Code, auth.calls, quote.calls, quote.actorID, quote.request.SpaceID, quote.request.StartAt, quote.request.EndAt, response.Body.String())
		}
		var envelope map[string]json.RawMessage
		if response.Header().Get("Cache-Control") != "no-store" || !rentalflow.ValidUUID(response.Header().Get("X-Request-ID")) || !json.Valid(response.Body.Bytes()) || json.Unmarshal(response.Body.Bytes(), &envelope) != nil || len(envelope) != 1 || envelope["data"] == nil {
			t.Fatalf("invalid quote response envelope: status=%d cache=%q requestID=%q body=%q", response.Code, response.Header().Get("Cache-Control"), response.Header().Get("X-Request-ID"), response.Body.String())
		}
	})
}

func containsNonASCII(value string) bool {
	for _, character := range value {
		if character > 0x7f {
			return true
		}
	}
	return false
}

func TestQuoteRequiresAuthenticationAndSerializesAllowlistedDataNoStore(t *testing.T) {
	quoteResult := rentalflow.Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: "44444444-4444-4444-8444-444444444444", RenterID: testActorID,
		SpaceTitle: "Sala", RateVersion: 9, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", Units: 1, SubtotalCLP: 12000,
		StartAt: time.Date(2030, 1, 2, 13, 0, 0, 0, time.UTC), EndAt: time.Date(2030, 1, 2, 14, 0, 0, 0, time.UTC), TimeZone: "America/Santiago",
		CreatedAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2030, 1, 1, 0, 15, 0, 0, time.UTC)}
	quote := &quoteStub{result: quoteResult}
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
	body := `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer session-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusCreated || auth.calls != 1 || auth.token != "session-token" || quote.calls != 1 || quote.actorID != testActorID {
		t.Fatalf("status=%d auth=%+v quote=%+v body=%s", w.Code, auth, quote, w.Body.String())
	}
	for _, forbidden := range []string{"owner_id", "renter_id", "rate_version"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("quote response exposes %q: %s", forbidden, w.Body.String())
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q", w.Header().Get("Cache-Control"))
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid quote response JSON: %v", err)
	}
	if len(envelope) != 1 || envelope["data"] == nil {
		t.Fatalf("quote response keys=%v, want only data", envelope)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatalf("invalid quote data JSON: %v", err)
	}
	wantFields := []string{"id", "space_id", "space_title", "rate_unit", "unit_price_clp", "currency", "units", "subtotal_clp", "start_at", "end_at", "time_zone", "created_at", "expires_at"}
	if len(data) != len(wantFields) {
		t.Fatalf("quote data keys=%v, want exactly %v", data, wantFields)
	}
	for _, key := range wantFields {
		if _, ok := data[key]; !ok {
			t.Fatalf("quote data keys=%v, want key %q", data, key)
		}
	}
}

func TestQuoteRejectsUnknownFieldsAndUnauthenticatedCaller(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth *authStub
		body string
		want int
	}{
		{name: "unknown field", auth: &authStub{principal: identity.Principal{AccountID: testActorID}}, body: `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z","return_url":"https://evil.invalid"}`, want: http.StatusBadRequest},
		{name: "client-supplied renter", auth: &authStub{principal: identity.Principal{AccountID: testActorID}}, body: `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z","renter_id":"44444444-4444-4444-8444-444444444444"}`, want: http.StatusBadRequest},
		{name: "client-supplied owner", auth: &authStub{principal: identity.Principal{AccountID: testActorID}}, body: `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z","owner_id":"44444444-4444-4444-8444-444444444444"}`, want: http.StatusBadRequest},
		{name: "unauthenticated", auth: &authStub{err: identity.ErrUnauthorized}, body: `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`, want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quote := &quoteStub{}
			h := newTestHandler(tc.auth, &availabilityStub{}, quote, &previewStub{})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want || quote.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, quote.calls, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control=%q, want no-store for %s response", w.Header().Get("Cache-Control"), tc.name)
			}
		})
	}
}

func TestQuoteMapsAuthenticatorOperationalFailureToServiceUnavailable(t *testing.T) {
	quote := &quoteStub{}
	auth := &authStub{err: errors.New("synthetic identity repository unavailable")}
	h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
	body := `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer session-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || auth.calls != 1 || quote.calls != 0 || w.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("status=%d auth_calls=%d quote_calls=%d challenge=%q body=%s", w.Code, auth.calls, quote.calls, w.Header().Get("WWW-Authenticate"), w.Body.String())
	}
}

func TestPreviewRequiresAuthenticatedParticipantAndSerializesAllowlistedDataNoStore(t *testing.T) {
	preview := &previewStub{result: rentalflow.ContractPreview{State: "preview", QuoteID: testQuoteID, SpaceID: testSpaceID,
		Parties: []rentalflow.ContractParty{{Role: "arrendador", LegalName: "Nombre Sintético", RUT: "12345678-5"}}}}
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	h := newTestHandler(auth, &availabilityStub{}, &quoteStub{}, preview)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || preview.calls != 1 || preview.actorID != testActorID || preview.quoteID != testQuoteID {
		t.Fatalf("status=%d preview=%+v body=%s", w.Code, preview, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), "Nombre Sintético") {
		t.Fatalf("headers/body=%v %s", w.Header(), w.Body.String())
	}
	assertKeys := func(fields map[string]json.RawMessage, expected ...string) {
		t.Helper()
		if len(fields) != len(expected) {
			t.Fatalf("JSON fields=%v, want exactly %v", fields, expected)
		}
		for _, key := range expected {
			if _, ok := fields[key]; !ok {
				t.Fatalf("JSON fields=%v, want key %q", fields, key)
			}
		}
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid preview response JSON: %v", err)
	}
	assertKeys(envelope, "data")
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatalf("invalid preview data JSON: %v", err)
	}
	assertKeys(data, "state", "quote_id", "space_id", "space_title", "rate_unit", "unit_price_clp", "units", "subtotal_clp", "currency", "start_at", "end_at", "time_zone", "parties", "generated_at")
	var parties []map[string]json.RawMessage
	if err := json.Unmarshal(data["parties"], &parties); err != nil {
		t.Fatalf("invalid preview parties JSON: %v", err)
	}
	if len(parties) != 1 {
		t.Fatalf("preview parties=%v, want one synthetic party", parties)
	}
	assertKeys(parties[0], "role", "legal_name", "rut")
}

func TestPreviewStopsOnAuthenticationFailureBeforeCallingService(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "unauthenticated", err: identity.ErrUnauthorized, want: http.StatusUnauthorized},
		{name: "forbidden", err: identity.ErrForbidden, want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{err: tc.err}
			preview := &previewStub{}
			h := newTestHandler(auth, &availabilityStub{}, &quoteStub{}, preview)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want || auth.calls != 1 || preview.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d auth=%d preview=%d cache=%q body=%s", w.Code, auth.calls, preview.calls, w.Header().Get("Cache-Control"), w.Body.String())
			}
			if tc.want == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("401 WWW-Authenticate=%q", w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRequestBodyPresentTreatsNilBodyAsAbsent(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Body = nil
	if requestBodyPresent(r) {
		t.Fatal("nil request body reported as present")
	}
}

func TestDomainErrorsMapToContractStatuses(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: rentalflow.ErrNotFound, want: http.StatusNotFound},
		{name: "conflict", err: rentalflow.ErrConflict, want: http.StatusConflict},
		{name: "identity required", err: rentalflow.ErrIdentityRequired, want: http.StatusConflict},
		{name: "unavailable", err: rentalflow.ErrUnavailable, want: http.StatusServiceUnavailable},
		{name: "identity unavailable", err: rentalflow.ErrIdentityUnavailable, want: http.StatusServiceUnavailable},
		{name: "invalid", err: rentalflow.ErrInvalid, want: http.StatusUnprocessableEntity},
		{name: "unknown", err: errors.New("sensitive internal detail"), want: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			availability := &availabilityStub{err: tc.err}
			h := newTestHandler(auth, availability, &quoteStub{}, &previewStub{})
			r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "sensitive internal detail") {
				t.Fatalf("status=%d cache=%q body=%s", w.Code, w.Header().Get("Cache-Control"), w.Body.String())
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("non-JSON response: %s (%v)", w.Body.String(), err)
			}
			if len(envelope) != 1 || envelope["error"] == nil {
				t.Fatalf("error envelope keys=%v, want only error", envelope)
			}
			var details map[string]json.RawMessage
			if err := json.Unmarshal(envelope["error"], &details); err != nil {
				t.Fatalf("invalid error object: %v", err)
			}
			if len(details) != 3 {
				t.Fatalf("error object keys=%v, want only code, message, request_id", details)
			}
			for _, key := range []string{"code", "message", "request_id"} {
				if _, ok := details[key]; !ok {
					t.Fatalf("error object keys=%v, missing %q", details, key)
				}
			}
		})
	}
}

func TestServiceErrorsDoNotExposePartialResults(t *testing.T) {
	quoteBody, err := json.Marshal(map[string]string{
		"space_id": testSpaceID,
		"start_at": "2030-01-02T13:00:00Z",
		"end_at":   "2030-01-02T14:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		marker string
	}{
		{
			name: "availability", method: http.MethodGet,
			path:   "/api/v1/spaces/" + testSpaceID + "/rental-availability?date=2030-01-02&duration=1",
			marker: "partial-availability-secret",
		},
		{name: "quote", method: http.MethodPost, path: "/api/v1/rental-quotes", body: string(quoteBody), marker: "partial-quote-title-secret"},
		{name: "preview", method: http.MethodPost, path: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview", marker: "partial-legal-name-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			availability := &availabilityStub{
				result: rentalflow.Availability{SpaceID: "partial-availability-secret", TimeZone: "America/Santiago", RateUnit: "hora"},
				err:    rentalflow.ErrUnavailable,
			}
			quote := &quoteStub{
				result: rentalflow.Quote{ID: testQuoteID, SpaceID: testSpaceID, SpaceTitle: "partial-quote-title-secret", Currency: "CLP"},
				err:    rentalflow.ErrUnavailable,
			}
			preview := &previewStub{
				result: rentalflow.ContractPreview{
					State: "preview", QuoteID: testQuoteID, SpaceTitle: "partial-preview-title-secret",
					Parties: []rentalflow.ContractParty{{Role: "renter", LegalName: "partial-legal-name-secret", RUT: "12345678-5"}},
				},
				err: rentalflow.ErrUnavailable,
			}
			h := newTestHandler(auth, availability, quote, preview)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method == http.MethodPost {
				r.Header.Set("Authorization", "Bearer token")
				if tc.body != "" {
					r.Header.Set("Content-Type", "application/json")
				}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v body=%q", err, w.Body.String())
			}
			if w.Code != http.StatusServiceUnavailable || len(envelope) != 1 || envelope["error"] == nil ||
				envelope["data"] != nil || strings.Contains(w.Body.String(), tc.marker) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d envelope_keys=%v marker=%q body=%s", w.Code, envelope, tc.marker, w.Body.String())
			}
		})
	}
}

func TestUnknownRouteReturnsContractNotFoundEnvelope(t *testing.T) {
	availability := &availabilityStub{}
	quote := &quoteStub{}
	preview := &previewStub{}
	h := newTestHandler(&authStub{}, availability, quote, preview)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/not-a-rental-route", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unknown route response is not JSON: status=%d body=%q err=%v", w.Code, w.Body.String(), err)
	}
	if w.Code != http.StatusNotFound || envelope.Error.Code != "not_found" || envelope.Error.RequestID == "" ||
		w.Header().Get("Cache-Control") != "no-store" || availability.calls != 0 || quote.calls != 0 || preview.calls != 0 {
		t.Fatalf("unknown route response status=%d envelope=%+v headers=%v calls=%d/%d/%d", w.Code, envelope, w.Header(), availability.calls, quote.calls, preview.calls)
	}
}

func TestQuoteAuthorizationFailuresFailClosedWithContractErrors(t *testing.T) {
	body := `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`
	cases := []struct {
		name       string
		auth       *authStub
		missing    bool
		wantStatus int
		wantCode   string
		wantWWW    string
	}{
		{name: "forbidden", auth: &authStub{err: identity.ErrForbidden}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "restricted mode", auth: &authStub{err: identity.ErrRestrictedMode}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "empty principal", auth: &authStub{}, wantStatus: http.StatusUnauthorized, wantCode: "unauthenticated", wantWWW: "Bearer"},
		{name: "authenticator failure", auth: &authStub{err: errors.New("private auth failure")}, wantStatus: http.StatusServiceUnavailable, wantCode: "service_unavailable"},
		{name: "missing authenticator", missing: true, wantStatus: http.StatusServiceUnavailable, wantCode: "service_unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := &quoteStub{}
			var authenticator Authenticator = tc.auth
			if tc.missing {
				authenticator = nil
			}
			h := New(authenticator, &availabilityStub{}, quote, &previewStub{}, nil)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v body=%s", err, w.Body.String())
			}
			if w.Code != tc.wantStatus || envelope.Error.Code != tc.wantCode || w.Header().Get("WWW-Authenticate") != tc.wantWWW || quote.calls != 0 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "private auth failure") {
				t.Fatalf("status=%d code=%q www=%q quote_calls=%d cache=%q body=%s", w.Code, envelope.Error.Code, w.Header().Get("WWW-Authenticate"), quote.calls, w.Header().Get("Cache-Control"), w.Body.String())
			}
		})
	}
}

func TestMalformedAuthorizationHeadersReachAuthenticatorAsEmptyBearer(t *testing.T) {
	for _, header := range []string{"", "Basic token", "Bearer"} {
		t.Run(header, func(t *testing.T) {
			auth := &authStub{err: identity.ErrUnauthorized}
			quote := &quoteStub{}
			h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(`{"space_id":"`+testSpaceID+`","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized || auth.calls != 1 || auth.token != "" || quote.calls != 0 || w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("header=%q status=%d auth_calls=%d token=%q quote_calls=%d www=%q", header, w.Code, auth.calls, auth.token, quote.calls, w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRentalFlowHandlersReturn503WhenServiceDependencyMissing(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "availability", method: http.MethodGet, path: "/api/v1/spaces/" + testSpaceID + "/rental-availability?date=2030-01-02&duration=1"},
		{name: "quote", method: http.MethodPost, path: "/api/v1/rental-quotes", body: `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`},
		{name: "preview", method: http.MethodPost, path: "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			h := New(auth, nil, nil, nil, nil)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method == http.MethodPost {
				r.Header.Set("Authorization", "Bearer token")
			}
			if tc.name == "quote" {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v body=%s", err, w.Body.String())
			}
			if w.Code != http.StatusServiceUnavailable || envelope.Error.Code != "service_unavailable" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d code=%q cache=%q body=%s", w.Code, envelope.Error.Code, w.Header().Get("Cache-Control"), w.Body.String())
			}
		})
	}
}
