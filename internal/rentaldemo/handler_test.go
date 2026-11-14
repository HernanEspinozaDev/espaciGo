package rentaldemo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDemoHandlerSessionsPrivacyAndIsolation(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	store, _ := NewStore(func() time.Time { return now })
	h := NewHandler(store, func() time.Time { return now })
	call := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:18117/api/v1/local/rental-demo"+path, bytes.NewBufferString(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable demo")
		}
		return w
	}
	if got := call("GET", "/owner/spaces", "", nil); got.Code != 401 {
		t.Fatalf("owner anonymous %d", got.Code)
	}
	public := call("GET", "/spaces", "", nil)
	if public.Code != 200 || bytes.Contains(public.Body.Bytes(), []byte("revision")) {
		t.Fatalf("public metadata %d %s", public.Code, public.Body.String())
	}
	login := call("POST", "/session", `{"actor":"host"}`, nil)
	if login.Code != 200 || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login %d", login.Code)
	}
	c := login.Result().Cookies()[0]
	if !c.HttpOnly || c.Path != "/api/rental-demo" || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe cookie attributes")
	}
	if got := call("GET", "/owner/spaces", "", c); got.Code != 200 {
		t.Fatalf("owner %d", got.Code)
	}
	if got := call("GET", "/spaces/"+SpaceIDHour+"/month?month=2030-01&duration=1&duration=2", "", nil); got.Code != 400 {
		t.Fatalf("duplicate query %d", got.Code)
	}
	if got := call("POST", "/session", `{"actor":"host","actor":"renter"}`, nil); got.Code != 400 {
		t.Fatalf("duplicate JSON %d", got.Code)
	}
	if got := call("POST", "/session", `{"actor":"host","kyc_verified":true}`, nil); got.Code != 400 {
		t.Fatalf("unknown JSON %d", got.Code)
	}
	if got := call("DELETE", "/session", "", c); got.Code != 204 {
		t.Fatalf("logout %d", got.Code)
	}
	if got := call("GET", "/owner/spaces", "", c); got.Code != 401 {
		t.Fatalf("revoked session %d", got.Code)
	}
	renter := call("POST", "/session", `{"actor":"renter"}`, nil).Result().Cookies()[0]
	if got := call("GET", "/owner/spaces", "", renter); got.Code != 403 {
		t.Fatalf("renter owner %d", got.Code)
	}
	now = now.Add(8 * time.Hour)
	if got := call("GET", "/session", "", renter); got.Code != 401 {
		t.Fatalf("expired session %d", got.Code)
	}
	// Product state remains unchanged by demo sessions and reads.
	sp, _ := store.Space(context.Background(), ActorHost, SpaceIDHour, true)
	if sp.Revision != "1" {
		t.Fatalf("reads mutated revision %s", sp.Revision)
	}
	var session map[string]any
	if err := json.Unmarshal(login.Body.Bytes(), &session); err != nil || len(session) != 2 {
		t.Fatal("session DTO")
	}
}

func TestDemoHandlerRejectsRemoteHostAndOrigin(t *testing.T) {
	s, _ := NewStore(time.Now)
	h := NewHandler(s, time.Now)
	for _, tc := range []struct{ host, origin string }{{"example.com", ""}, {"127.0.0.1:18117", "https://evil.example"}} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/spaces", nil)
		r.Host = tc.host
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("host/origin accepted %d", w.Code)
		}
	}
}

func TestDemoHandlerSecurityAndWireSanitization(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	store, _ := NewStore(func() time.Time { return now })
	h := NewHandler(store, func() time.Time { return now })

	// 1. Oversized body (>16KiB) must be rejected with 400
	oversized := `{"actor":"` + strings.Repeat("x", 17000) + `"}`
	r := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", bytes.NewBufferString(oversized))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("oversized request accepted: code %d", w.Code)
	}

	// 2. Login as renter for authenticated endpoints
	rLogin := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", bytes.NewBufferString(`{"actor":"renter"}`))
	rLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	h.ServeHTTP(wLogin, rLogin)
	if wLogin.Code != 200 {
		t.Fatalf("renter login failed: %d", wLogin.Code)
	}
	cookie := wLogin.Result().Cookies()[0]

	// 3. Confirm with invalid UUID format rejected with 400
	rBadUUID := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes/some-quote/confirm", bytes.NewBufferString(`{"approved_contract":true,"intent_id":"not-a-valid-uuid"}`))
	rBadUUID.Header.Set("Content-Type", "application/json")
	rBadUUID.AddCookie(cookie)
	wBadUUID := httptest.NewRecorder()
	h.ServeHTTP(wBadUUID, rBadUUID)
	if wBadUUID.Code != 400 {
		t.Fatalf("bad uuid accepted: code %d", wBadUUID.Code)
	}

	// 4. Confirm without approval rejected with 400
	rNoApprove := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes/some-quote/confirm", bytes.NewBufferString(`{"approved_contract":false,"intent_id":"00000000-0000-4000-8000-000000000099"}`))
	rNoApprove.Header.Set("Content-Type", "application/json")
	rNoApprove.AddCookie(cookie)
	wNoApprove := httptest.NewRecorder()
	h.ServeHTTP(wNoApprove, rNoApprove)
	if wNoApprove.Code != 400 {
		t.Fatalf("unapproved confirm accepted: code %d", wNoApprove.Code)
	}

	// 5. Unknown endpoint returns 404 for authenticated caller, 401 for anonymous
	rAnonNotFound := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/unknown-route", nil)
	wAnonNotFound := httptest.NewRecorder()
	h.ServeHTTP(wAnonNotFound, rAnonNotFound)
	if wAnonNotFound.Code != 401 {
		t.Fatalf("anonymous unknown route got %d, want 401", wAnonNotFound.Code)
	}

	rAuthNotFound := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/unknown-route", nil)
	rAuthNotFound.AddCookie(cookie)
	wAuthNotFound := httptest.NewRecorder()
	h.ServeHTTP(wAuthNotFound, rAuthNotFound)
	if wAuthNotFound.Code != 404 {
		t.Fatalf("authenticated unknown route got %d, want 404", wAuthNotFound.Code)
	}

	// 6. DELETE block without If-Match or with unquoted etag returns 400
	rHostLogin := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", bytes.NewBufferString(`{"actor":"host"}`))
	rHostLogin.Header.Set("Content-Type", "application/json")
	wHostLogin := httptest.NewRecorder()
	h.ServeHTTP(wHostLogin, rHostLogin)
	hostCookie := wHostLogin.Result().Cookies()[0]

	rNoEtag := httptest.NewRequest("DELETE", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour+"/blocks/00000000-0000-4000-8000-000000000001", nil)
	rNoEtag.AddCookie(hostCookie)
	wNoEtag := httptest.NewRecorder()
	h.ServeHTTP(wNoEtag, rNoEtag)
	if wNoEtag.Code != 400 {
		t.Fatalf("DELETE block without If-Match got %d, want 400", wNoEtag.Code)
	}

	rUnquotedEtag := httptest.NewRequest("DELETE", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour+"/blocks/00000000-0000-4000-8000-000000000001", nil)
	rUnquotedEtag.AddCookie(hostCookie)
	rUnquotedEtag.Header.Set("If-Match", "unquoted-revision")
	wUnquotedEtag := httptest.NewRecorder()
	h.ServeHTTP(wUnquotedEtag, rUnquotedEtag)
	if wUnquotedEtag.Code != 400 {
		t.Fatalf("DELETE block with unquoted If-Match got %d, want 400", wUnquotedEtag.Code)
	}

	// 7. PUT /owner/spaces/{spaceID} authorization, validation and CAS
	rPutAnon := httptest.NewRequest("PUT", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour, bytes.NewBufferString(`{"expected_revision":"1","state":"activa","unit_price_clp":"15000","open_hour":8,"close_hour":20}`))
	rPutAnon.Header.Set("Content-Type", "application/json")
	wPutAnon := httptest.NewRecorder()
	h.ServeHTTP(wPutAnon, rPutAnon)
	if wPutAnon.Code != 401 {
		t.Fatalf("anonymous PUT space got %d, want 401", wPutAnon.Code)
	}

	rPutRenter := httptest.NewRequest("PUT", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour, bytes.NewBufferString(`{"expected_revision":"1","state":"activa","unit_price_clp":"15000","open_hour":8,"close_hour":20}`))
	rPutRenter.Header.Set("Content-Type", "application/json")
	rPutRenter.AddCookie(cookie)
	wPutRenter := httptest.NewRecorder()
	h.ServeHTTP(wPutRenter, rPutRenter)
	if wPutRenter.Code != 403 {
		t.Fatalf("renter PUT space got %d, want 403", wPutRenter.Code)
	}

	rPutBadHours := httptest.NewRequest("PUT", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour, bytes.NewBufferString(`{"expected_revision":"1","state":"activa","unit_price_clp":"15000","open_hour":20,"close_hour":8}`))
	rPutBadHours.Header.Set("Content-Type", "application/json")
	rPutBadHours.AddCookie(hostCookie)
	wPutBadHours := httptest.NewRecorder()
	h.ServeHTTP(wPutBadHours, rPutBadHours)
	if wPutBadHours.Code != 400 {
		t.Fatalf("bad hours PUT space got %d, want 400", wPutBadHours.Code)
	}

	rPutStale := httptest.NewRequest("PUT", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour, bytes.NewBufferString(`{"expected_revision":"999","state":"activa","unit_price_clp":"15000","open_hour":8,"close_hour":20}`))
	rPutStale.Header.Set("Content-Type", "application/json")
	rPutStale.AddCookie(hostCookie)
	wPutStale := httptest.NewRecorder()
	h.ServeHTTP(wPutStale, rPutStale)
	if wPutStale.Code != 409 {
		t.Fatalf("stale revision PUT space got %d, want 409", wPutStale.Code)
	}

	// 8. POST /quotes validation and authorization
	quoteBody := `{"space_id":"` + SpaceIDHour + `","start_at":"2030-01-16T12:00:00Z","end_at":"2030-01-16T13:00:00Z"}`
	rQuoteAnon := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes", bytes.NewBufferString(quoteBody))
	rQuoteAnon.Header.Set("Content-Type", "application/json")
	wQuoteAnon := httptest.NewRecorder()
	h.ServeHTTP(wQuoteAnon, rQuoteAnon)
	if wQuoteAnon.Code != 401 {
		t.Fatalf("anonymous quote got %d, want 401", wQuoteAnon.Code)
	}

	rQuoteHost := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes", bytes.NewBufferString(quoteBody))
	rQuoteHost.Header.Set("Content-Type", "application/json")
	rQuoteHost.AddCookie(hostCookie)
	wQuoteHost := httptest.NewRecorder()
	h.ServeHTTP(wQuoteHost, rQuoteHost)
	if wQuoteHost.Code != 403 {
		t.Fatalf("host quote got %d, want 403", wQuoteHost.Code)
	}

	rQuoteUnknownField := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes", bytes.NewBufferString(`{"space_id":"`+SpaceIDHour+`","start_at":"2030-01-16T12:00:00Z","end_at":"2030-01-16T13:00:00Z","discount":"50%"}`))
	rQuoteUnknownField.Header.Set("Content-Type", "application/json")
	rQuoteUnknownField.AddCookie(cookie)
	wQuoteUnknownField := httptest.NewRecorder()
	h.ServeHTTP(wQuoteUnknownField, rQuoteUnknownField)
	if wQuoteUnknownField.Code != 400 {
		t.Fatalf("unknown field quote got %d, want 400", wQuoteUnknownField.Code)
	}

	rQuoteValid := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes", bytes.NewBufferString(quoteBody))
	rQuoteValid.Header.Set("Content-Type", "application/json")
	rQuoteValid.AddCookie(cookie)
	wQuoteValid := httptest.NewRecorder()
	h.ServeHTTP(wQuoteValid, rQuoteValid)
	if wQuoteValid.Code != 201 {
		t.Fatalf("valid quote got %d, want 201", wQuoteValid.Code)
	}

	// 9. POST /owner/spaces/{spaceID}/blocks authorization and validation
	sp, _ := store.Space(context.Background(), ActorHost, SpaceIDHour, true)
	blockPayload := `{"expected_revision":"` + sp.Revision + `","start_at":"2030-01-17T14:00:00Z","end_at":"2030-01-17T16:00:00Z"}`
	rBlockRenter := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour+"/blocks", bytes.NewBufferString(blockPayload))
	rBlockRenter.Header.Set("Content-Type", "application/json")
	rBlockRenter.AddCookie(cookie)
	wBlockRenter := httptest.NewRecorder()
	h.ServeHTTP(wBlockRenter, rBlockRenter)
	if wBlockRenter.Code != 403 {
		t.Fatalf("renter creating block got %d, want 403", wBlockRenter.Code)
	}

	rBlockInverted := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour+"/blocks", bytes.NewBufferString(`{"expected_revision":"`+sp.Revision+`","start_at":"2030-01-17T16:00:00Z","end_at":"2030-01-17T14:00:00Z"}`))
	rBlockInverted.Header.Set("Content-Type", "application/json")
	rBlockInverted.AddCookie(hostCookie)
	wBlockInverted := httptest.NewRecorder()
	h.ServeHTTP(wBlockInverted, rBlockInverted)
	if wBlockInverted.Code != 400 {
		t.Fatalf("inverted block got %d, want 400", wBlockInverted.Code)
	}

	rBlockStale := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/owner/spaces/"+SpaceIDHour+"/blocks", bytes.NewBufferString(`{"expected_revision":"999","start_at":"2030-01-17T14:00:00Z","end_at":"2030-01-17T16:00:00Z"}`))
	rBlockStale.Header.Set("Content-Type", "application/json")
	rBlockStale.AddCookie(hostCookie)
	wBlockStale := httptest.NewRecorder()
	h.ServeHTTP(wBlockStale, rBlockStale)
	if wBlockStale.Code != 409 {
		t.Fatalf("stale revision block got %d, want 409", wBlockStale.Code)
	}

	// 10. GET /attempts/{intentID} authorization, actor isolation and method rejection
	rAttemptAnon := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/attempts/00000000-0000-4000-8000-000000000001", nil)
	wAttemptAnon := httptest.NewRecorder()
	h.ServeHTTP(wAttemptAnon, rAttemptAnon)
	if wAttemptAnon.Code != 401 {
		t.Fatalf("anonymous attempt got %d, want 401", wAttemptAnon.Code)
	}

	rAttemptNotFound := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/attempts/00000000-0000-4000-8000-000000000099", nil)
	rAttemptNotFound.AddCookie(cookie)
	wAttemptNotFound := httptest.NewRecorder()
	h.ServeHTTP(wAttemptNotFound, rAttemptNotFound)
	if wAttemptNotFound.Code != 404 {
		t.Fatalf("non-existent attempt got %d, want 404", wAttemptNotFound.Code)
	}

	rAttemptWrongMethod := httptest.NewRequest("DELETE", "http://127.0.0.1:18117/api/v1/local/rental-demo/attempts/00000000-0000-4000-8000-000000000001", nil)
	rAttemptWrongMethod.AddCookie(cookie)
	wAttemptWrongMethod := httptest.NewRecorder()
	h.ServeHTTP(wAttemptWrongMethod, rAttemptWrongMethod)
	if wAttemptWrongMethod.Code != 404 {
		t.Fatalf("DELETE attempt got %d, want 404", wAttemptWrongMethod.Code)
	}

	// 11. POST /quotes/{quoteID}/confirm authorization and validation
	rConfirmAnon := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes/test-quote-id/confirm", bytes.NewBufferString(`{"approved_contract":true,"intent_id":"00000000-0000-4000-8000-000000000001"}`))
	rConfirmAnon.Header.Set("Content-Type", "application/json")
	wConfirmAnon := httptest.NewRecorder()
	h.ServeHTTP(wConfirmAnon, rConfirmAnon)
	if wConfirmAnon.Code != 401 {
		t.Fatalf("anon confirm quote got %d, want 401", wConfirmAnon.Code)
	}

	rConfirmHost := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes/test-quote-id/confirm", bytes.NewBufferString(`{"approved_contract":true,"intent_id":"00000000-0000-4000-8000-000000000001"}`))
	rConfirmHost.Header.Set("Content-Type", "application/json")
	rConfirmHost.AddCookie(hostCookie)
	wConfirmHost := httptest.NewRecorder()
	h.ServeHTTP(wConfirmHost, rConfirmHost)
	if wConfirmHost.Code != 403 {
		t.Fatalf("host confirm quote got %d, want 403", wConfirmHost.Code)
	}

	rConfirmUnapproved := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes/test-quote-id/confirm", bytes.NewBufferString(`{"approved_contract":false,"intent_id":"00000000-0000-4000-8000-000000000001"}`))
	rConfirmUnapproved.Header.Set("Content-Type", "application/json")
	rConfirmUnapproved.AddCookie(cookie)
	wConfirmUnapproved := httptest.NewRecorder()
	h.ServeHTTP(wConfirmUnapproved, rConfirmUnapproved)
	if wConfirmUnapproved.Code != 400 {
		t.Fatalf("unapproved contract confirm got %d, want 400", wConfirmUnapproved.Code)
	}

	rConfirmBadIntent := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/quotes/test-quote-id/confirm", bytes.NewBufferString(`{"approved_contract":true,"intent_id":"not-a-uuid"}`))
	rConfirmBadIntent.Header.Set("Content-Type", "application/json")
	rConfirmBadIntent.AddCookie(cookie)
	wConfirmBadIntent := httptest.NewRecorder()
	h.ServeHTTP(wConfirmBadIntent, rConfirmBadIntent)
	if wConfirmBadIntent.Code != 400 {
		t.Fatalf("bad intent confirm got %d, want 400", wConfirmBadIntent.Code)
	}
}

func TestDemoHandlerSessionMethodAndActorValidation(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	store, _ := NewStore(func() time.Time { return now })
	h := NewHandler(store, func() time.Time { return now })

	// 1. PUT on /session returns 405 Method Not Allowed
	rPut := httptest.NewRequest("PUT", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", nil)
	wPut := httptest.NewRecorder()
	h.ServeHTTP(wPut, rPut)
	if wPut.Code != 405 {
		t.Fatalf("PUT /session got %d, want 405", wPut.Code)
	}

	// 2. Unknown actor returns 400 invalid_request
	for _, badActor := range []string{"admin", "guest", "", "HOST", "renter3"} {
		body := `{"actor":"` + badActor + `"}`
		r := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 || !bytes.Contains(w.Body.Bytes(), []byte("invalid_request")) {
			t.Fatalf("bad actor %q got code %d body %s, want 400 invalid_request", badActor, w.Code, w.Body.String())
		}
	}
}

func TestDemoHandlerSessionFullLifecycle(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	store, _ := NewStore(func() time.Time { return now })
	h := NewHandler(store, func() time.Time { return now })

	// 1. Initial GET /session -> 401
	rGetUnauth := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", nil)
	wGetUnauth := httptest.NewRecorder()
	h.ServeHTTP(wGetUnauth, rGetUnauth)
	if wGetUnauth.Code != 401 {
		t.Fatalf("initial session check got %d, want 401", wGetUnauth.Code)
	}

	// 2. POST /session with valid renter -> 200 + cookie
	rLogin := httptest.NewRequest("POST", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", bytes.NewBufferString(`{"actor":"renter"}`))
	rLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	h.ServeHTTP(wLogin, rLogin)
	if wLogin.Code != 200 {
		t.Fatalf("login got %d, want 200", wLogin.Code)
	}
	cookies := wLogin.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "espacigo_rental_demo" || cookies[0].Value == "" {
		t.Fatalf("expected espacigo_rental_demo cookie, got: %v", cookies)
	}
	sessionCookie := cookies[0]

	// 3. GET /session with valid cookie -> 200
	rGetAuth := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", nil)
	rGetAuth.AddCookie(sessionCookie)
	wGetAuth := httptest.NewRecorder()
	h.ServeHTTP(wGetAuth, rGetAuth)
	if wGetAuth.Code != 200 || !bytes.Contains(wGetAuth.Body.Bytes(), []byte(`"actor":"renter"`)) {
		t.Fatalf("authenticated session got %d body %s", wGetAuth.Code, wGetAuth.Body.String())
	}

	// 4. DELETE /session -> 204 + expired cookie
	rLogout := httptest.NewRequest("DELETE", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", nil)
	rLogout.AddCookie(sessionCookie)
	wLogout := httptest.NewRecorder()
	h.ServeHTTP(wLogout, rLogout)
	if wLogout.Code != 204 {
		t.Fatalf("logout got %d, want 204", wLogout.Code)
	}
	logoutCookies := wLogout.Result().Cookies()
	if len(logoutCookies) == 0 || logoutCookies[0].MaxAge != -1 {
		t.Fatalf("expected cookie expiry MaxAge=-1, got: %v", logoutCookies)
	}

	// 5. Subsequent GET /session with old cookie -> 401
	rGetStale := httptest.NewRequest("GET", "http://127.0.0.1:18117/api/v1/local/rental-demo/session", nil)
	rGetStale.AddCookie(sessionCookie)
	wGetStale := httptest.NewRecorder()
	h.ServeHTTP(wGetStale, rGetStale)
	if wGetStale.Code != 401 {
		t.Fatalf("stale session check got %d, want 401", wGetStale.Code)
	}
}
