package rentaldemo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOneHTTPSourceCoordinatesOwnerPublicQuoteAndPaidDemo(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	h := NewHandler(s, func() time.Time { return now })
	call := func(method, path string, body any, cookie *http.Cookie, etag string) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		if body == nil {
			data = nil
		}
		r := httptest.NewRequest(method, "http://127.0.0.1:18117"+DemoPrefix+path, bytes.NewReader(data))
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if etag != "" {
			r.Header.Set("If-Match", etag)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	login := func(actor string) *http.Cookie {
		w := call("POST", "/session", map[string]string{"actor": actor}, nil, "")
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		return w.Result().Cookies()[0]
	}
	owner := login(ActorHost)
	renter := login(ActorRenter)
	read := func(ownerMode bool) Month {
		prefix := "/spaces/"
		var cookie *http.Cookie
		if ownerMode {
			prefix = "/owner/spaces/"
			cookie = owner
		}
		w := call("GET", prefix+SpaceIDHour+"/month?month=2030-01&duration=1", nil, cookie, "")
		if w.Code != 200 {
			t.Fatalf("month %d %s", w.Code, w.Body.String())
		}
		if !ownerMode && (bytes.Contains(w.Body.Bytes(), []byte("blocks")) || bytes.Contains(w.Body.Bytes(), []byte("revision"))) {
			t.Fatal("private public calendar")
		}
		var m Month
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	before := read(false)
	slot := before.Days[15].Intervals[0]
	host := read(true)
	blocked := call("POST", "/owner/spaces/"+SpaceIDHour+"/blocks", map[string]string{"expected_revision": host.Revision, "start_at": slot.StartAt, "end_at": slot.EndAt}, owner, "")
	if blocked.Code != 201 {
		t.Fatal(blocked.Code)
	}
	var block struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(blocked.Body.Bytes(), &block)
	afterBlock := read(false)
	if len(afterBlock.Days[15].Intervals) >= len(before.Days[15].Intervals) {
		t.Fatal("host block not reflected publicly")
	}
	host = read(true)
	if w := call("DELETE", "/owner/spaces/"+SpaceIDHour+"/blocks/"+block.ID, nil, owner, `"`+host.Revision+`"`); w.Code != 204 {
		t.Fatal(w.Code)
	}
	restored := read(false)
	if len(restored.Days[15].Intervals) != len(before.Days[15].Intervals) {
		t.Fatal("unblock readback mismatch")
	}
	qw := call("POST", "/quotes", QuoteRequest{SpaceIDHour, slot.StartAt, slot.EndAt}, renter, "")
	if qw.Code != 201 {
		t.Fatalf("quote %d %s", qw.Code, qw.Body.String())
	}
	var q Quote
	_ = json.Unmarshal(qw.Body.Bytes(), &q)
	if len(read(false).Days[15].Intervals) != len(before.Days[15].Intervals) {
		t.Fatal("quote retained interval")
	}
	intent := "00000000-0000-4000-8000-000000000021"
	body := map[string]any{"approved_contract": true, "intent_id": intent}
	paid := call("POST", "/quotes/"+q.ID+"/confirm", body, renter, "")
	if paid.Code != 200 {
		t.Fatalf("confirm %d %s", paid.Code, paid.Body.String())
	}
	var c Confirmation
	_ = json.Unmarshal(paid.Body.Bytes(), &c)
	if c.SubtotalCLP != q.SubtotalCLP || c.PaymentStatus != "paid_simulated" || c.ID == "" {
		t.Fatal("payment detached from snapshot")
	}
	for _, m := range []Month{read(true), read(false)} {
		for _, i := range m.Days[15].Intervals {
			if i == slot {
				t.Fatal("confirmed slot still offered")
			}
		}
	}
	if w := call("POST", "/quotes/"+q.ID+"/confirm", body, renter, ""); w.Code != 200 {
		t.Fatal("same-intent reconciliation")
	}
	if len(s.occupied) != 1 {
		t.Fatal("duplicate occupancy")
	}
	if w := call("GET", "/attempts/"+intent, nil, renter, ""); w.Code != 200 {
		t.Fatal("missing attempt readback")
	}
}

func TestDemoOwnerCASAndPublicEligibility(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T12:00:00Z", "2030-01-16T13:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSpace(ctx, ActorRenter, SpaceIDHour, "1", "oculta", "12000", 8, 20); !errors.Is(err, ErrForbidden) {
		t.Fatal("renter changed owner config")
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, "0", "oculta", "12000", 8, 20); !errors.Is(err, ErrConflict) {
		t.Fatal("stale CAS accepted")
	}
	if _, err := s.UpdateSpace(ctx, ActorHost, SpaceIDHour, "1", "oculta", "24000", 8, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Month(ctx, "", SpaceIDHour, "2030-01", 1, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("hidden space offered publicly")
	}
	if _, err := s.Month(ctx, ActorHost, SpaceIDHour, "2030-01", 1, true); err != nil {
		t.Fatal("owner cannot manage hidden")
	}
	if _, err := s.Confirm(ctx, ActorRenter, q.ID, true, "00000000-0000-4000-8000-000000000022"); !errors.Is(err, ErrConflict) || len(s.occupied) != 0 {
		t.Fatal("obsolete snapshot confirmed")
	}
}

func TestSpacePriceOrScheduleChangeInvalidatesPendingQuotes(t *testing.T) {
	now := time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(func() time.Time { return now })
	ctx := context.Background()
	// Quote created with initial price
	q, err := s.CreateQuote(ctx, ActorRenter, QuoteRequest{SpaceIDHour, "2030-01-16T12:00:00Z", "2030-01-16T14:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	sp, _ := s.Space(ctx, ActorHost, SpaceIDHour, true)
	// Host changes price from 12000 to 18000
	if _, err := s.UpdateSpace(ctx, ActorHost, sp.ID, sp.Revision, sp.State, "18000", sp.OpenHour, sp.CloseHour); err != nil {
		t.Fatal(err)
	}
	// Pending quote at old price must be rejected with ErrConflict
	intent := "00000000-0000-4000-8000-000000000033"
	if _, err := s.Confirm(ctx, ActorRenter, q.ID, true, intent); !errors.Is(err, ErrConflict) || len(s.occupied) != 0 {
		t.Fatalf("quote with obsolete price accepted: %v, occupancy=%d", err, len(s.occupied))
	}
}
