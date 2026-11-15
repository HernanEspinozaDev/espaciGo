package rentaldemo

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

const DemoPrefix = "/api/v1/local/rental-demo"
const demoCookie = "espacigo_rental_demo"

type demoSession struct {
	Actor     string    `json:"actor"`
	ExpiresAt time.Time `json:"expires_at"`
}
type demoHandler struct {
	store    *Store
	now      func() time.Time
	mu       sync.Mutex
	sessions map[string]demoSession
}

// NewHandler is used exclusively by cmd/rentaldemo, never by the product API.
func NewHandler(store *Store, now func() time.Time) http.Handler {
	return &demoHandler{store: store, now: now, sessions: make(map[string]demoSession)}
}
func demoError(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}
func demoJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func mappedDemoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		demoError(w, 400, "invalid_request")
	case errors.Is(err, ErrForbidden):
		demoError(w, 403, "forbidden")
	case errors.Is(err, ErrNotFound):
		demoError(w, 404, "not_found")
	case errors.Is(err, ErrConflict):
		demoError(w, 409, "conflict")
	default:
		demoError(w, 503, "unavailable")
	}
}
func decodeDemo(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		demoError(w, 400, "invalid_request")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	if err != nil {
		demoError(w, 400, "invalid_request")
		return false
	}
	// All demo requests are flat objects. Reject duplicate keys and nested values
	// before struct decoding, rather than allowing encoding/json's last-key wins.
	tokens := json.NewDecoder(bytes.NewReader(body))
	start, err := tokens.Token()
	if err != nil || start != json.Delim('{') {
		demoError(w, 400, "invalid_request")
		return false
	}
	seen := map[string]bool{}
	for tokens.More() {
		key, err := tokens.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[strings.ToLower(name)] {
			demoError(w, 400, "invalid_request")
			return false
		}
		seen[strings.ToLower(name)] = true
		value, err := tokens.Token()
		if err != nil {
			demoError(w, 400, "invalid_request")
			return false
		}
		if _, nested := value.(json.Delim); nested {
			demoError(w, 400, "invalid_request")
			return false
		}
	}
	if end, err := tokens.Token(); err != nil || end != json.Delim('}') {
		demoError(w, 400, "invalid_request")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		demoError(w, 400, "invalid_request")
		return false
	}
	return true
}
func (h *demoHandler) actor(r *http.Request) (demoSession, bool) {
	values := []*http.Cookie{}
	for _, c := range r.Cookies() {
		if c.Name == demoCookie {
			values = append(values, c)
		}
	}
	if len(values) != 1 {
		return demoSession{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[values[0].Value]
	if !ok || !s.ExpiresAt.After(h.now()) {
		delete(h.sessions, values[0].Value)
		return demoSession{}, false
	}
	return s, true
}
func safeDemoHost(value string) bool {
	host, port, err := net.SplitHostPort(value)
	n, e := strconv.Atoi(port)
	return err == nil && host == "127.0.0.1" && e == nil && n > 0 && n <= 65535
}
func (h *demoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if h.store == nil || h.now == nil {
		demoError(w, 503, "unavailable")
		return
	}
	origins := r.Header.Values("Origin")
	if !safeDemoHost(r.Host) || len(origins) > 1 {
		demoError(w, 403, "forbidden")
		return
	}
	if len(origins) == 1 && origins[0] != "" {
		o, err := url.Parse(origins[0])
		if err != nil || o.Scheme != "http" || o.Host != r.Host || o.Path != "" || o.RawQuery != "" || o.Fragment != "" {
			demoError(w, 403, "forbidden")
			return
		}
	}
	if r.Header.Get("Authorization") != "" {
		demoError(w, 403, "forbidden")
		return
	}
	if !strings.HasPrefix(r.URL.Path, DemoPrefix+"/") {
		demoError(w, 404, "not_found")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, DemoPrefix)
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		demoError(w, 400, "invalid_request")
		return
	}
	isMonth := len(parts) > 2 && parts[len(parts)-1] == "month"
	if !isMonth && len(query) != 0 {
		demoError(w, 400, "invalid_request")
		return
	}
	method := r.Method
	if path == "/session" {
		switch method {
		case http.MethodGet:
			session, ok := h.actor(r)
			if !ok {
				demoError(w, 401, "unauthenticated")
				return
			}
			demoJSON(w, 200, session)
		case http.MethodPost:
			var in struct {
				Actor string `json:"actor"`
			}
			if !decodeDemo(w, r, &in) {
				return
			}
			if _, ok := actorID(in.Actor); !ok {
				demoError(w, 400, "invalid_request")
				return
			}
			token, err := (credentials.Generator{}).Token()
			if err != nil {
				demoError(w, 503, "unavailable")
				return
			}
			session := demoSession{in.Actor, h.now().UTC().Add(8 * time.Hour)}
			h.mu.Lock()
			for _, c := range r.Cookies() {
				if c.Name == demoCookie {
					delete(h.sessions, c.Value)
				}
			}
			h.sessions[string(token)] = session
			h.mu.Unlock()
			http.SetCookie(w, &http.Cookie{Name: demoCookie, Value: string(token), Path: "/api/rental-demo", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
			demoJSON(w, 200, session)
		case http.MethodDelete:
			h.mu.Lock()
			for _, c := range r.Cookies() {
				if c.Name == demoCookie {
					delete(h.sessions, c.Value)
				}
			}
			h.mu.Unlock()
			http.SetCookie(w, &http.Cookie{Name: demoCookie, Value: "", Path: "/api/rental-demo", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
			w.WriteHeader(204)
		default:
			demoError(w, 405, "method_not_allowed")
		}
		return
	}
	owner := parts[0] == "owner"
	actor := ""
	session, authenticated := h.actor(r)
	if authenticated {
		actor = session.Actor
	}
	public := !owner && parts[0] == "spaces" && method == http.MethodGet
	if !public && !authenticated {
		demoError(w, 401, "unauthenticated")
		return
	}
	if owner && actor != ActorHost {
		demoError(w, 403, "forbidden")
		return
	}
	offset := 0
	if owner {
		offset = 1
	}
	if len(parts) > offset && parts[offset] == "spaces" {
		p := parts[offset:]
		if len(p) == 1 && method == http.MethodGet {
			h.store.mu.RLock()
			spaces := make([]Space, 0, len(h.store.spaces))
			for _, s := range h.store.spaces {
				if owner || s.State == "activa" {
					spaces = append(spaces, s.Space)
				}
			}
			h.store.mu.RUnlock()
			sort.Slice(spaces, func(i, j int) bool { return spaces[i].ID < spaces[j].ID })
			if owner {
				demoJSON(w, 200, spaces)
			} else {
				items := make([]map[string]any, 0, len(spaces))
				for _, s := range spaces {
					items = append(items, map[string]any{"id": s.ID, "title": s.Title, "state": s.State, "time_zone": s.TimeZone, "rate_unit": s.RateUnit, "unit_price_clp": s.UnitPriceCLP})
				}
				demoJSON(w, 200, items)
			}
			return
		}
		if len(p) < 2 || !rentalflow.ValidUUID(p[1]) {
			demoError(w, 404, "not_found")
			return
		}
		id := p[1]
		if len(p) == 3 && p[2] == "month" && method == http.MethodGet {
			if len(query) != 2 || len(query["month"]) != 1 || len(query["duration"]) != 1 {
				demoError(w, 400, "invalid_request")
				return
			}
			duration, e := strconv.Atoi(query.Get("duration"))
			if e != nil {
				demoError(w, 400, "invalid_request")
				return
			}
			month, e := h.store.Month(r.Context(), actor, id, query.Get("month"), duration, owner)
			if e != nil {
				mappedDemoError(w, e)
				return
			}
			if owner {
				blocks := month.Blocks
				if blocks == nil {
					blocks = []Block{}
				}
				demoJSON(w, 200, struct {
					Month
					Blocks []Block `json:"blocks"`
				}{month, blocks})
			} else {
				demoJSON(w, 200, month)
			}
			return
		}
		if owner && len(p) == 2 && method == http.MethodPut {
			var in struct {
				Expected string `json:"expected_revision"`
				State    string `json:"state"`
				Price    string `json:"unit_price_clp"`
				Open     int    `json:"open_hour"`
				Close    int    `json:"close_hour"`
			}
			if !decodeDemo(w, r, &in) {
				return
			}
			s, e := h.store.UpdateSpace(r.Context(), actor, id, in.Expected, in.State, in.Price, in.Open, in.Close)
			if e != nil {
				mappedDemoError(w, e)
				return
			}
			demoJSON(w, 200, s)
			return
		}
		if owner && len(p) == 3 && p[2] == "blocks" && method == http.MethodPost {
			var in struct {
				Expected string `json:"expected_revision"`
				Start    string `json:"start_at"`
				End      string `json:"end_at"`
			}
			if !decodeDemo(w, r, &in) {
				return
			}
			b, e := h.store.Block(r.Context(), actor, id, in.Expected, in.Start, in.End)
			if e != nil {
				mappedDemoError(w, e)
				return
			}
			demoJSON(w, 201, map[string]string{"id": b.ID})
			return
		}
		if owner && len(p) == 4 && p[2] == "blocks" && method == http.MethodDelete {
			etags := r.Header.Values("If-Match")
			if len(etags) != 1 || len(etags[0]) < 3 || etags[0][0] != '"' || etags[0][len(etags[0])-1] != '"' {
				demoError(w, 400, "invalid_request")
				return
			}
			if e := h.store.Unblock(r.Context(), actor, id, p[3], etags[0][1:len(etags[0])-1]); e != nil {
				mappedDemoError(w, e)
				return
			}
			w.WriteHeader(204)
			return
		}
	}
	if path == "/quotes" && method == http.MethodPost {
		var in QuoteRequest
		if !decodeDemo(w, r, &in) {
			return
		}
		q, e := h.store.CreateQuote(r.Context(), actor, in)
		if e != nil {
			mappedDemoError(w, e)
			return
		}
		demoJSON(w, 201, q)
		return
	}
	if len(parts) == 3 && parts[0] == "quotes" && parts[2] == "confirm" && method == http.MethodPost {
		var in struct {
			Approved bool   `json:"approved_contract"`
			Intent   string `json:"intent_id"`
		}
		if !decodeDemo(w, r, &in) {
			return
		}
		c, e := h.store.Confirm(r.Context(), actor, parts[1], in.Approved, in.Intent)
		if e != nil {
			mappedDemoError(w, e)
			return
		}
		demoJSON(w, 200, c)
		return
	}
	if len(parts) == 2 && parts[0] == "attempts" && method == http.MethodGet {
		c, e := h.store.Attempt(r.Context(), actor, parts[1])
		if e != nil {
			mappedDemoError(w, e)
			return
		}
		demoJSON(w, 200, c)
		return
	}
	demoError(w, 404, "not_found")
}
