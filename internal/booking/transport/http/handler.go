package bookinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth         Authenticator
	service      *booking.Service
	conversation *conversation.Service
	origins      map[string]bool
}

func NewHandler(auth Authenticator, service *booking.Service, origins []string, conversations ...*conversation.Service) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	var conversationService *conversation.Service
	if len(conversations) > 0 {
		conversationService = conversations[0]
	}
	return &Handler{auth: auth, service: service, conversation: conversationService, origins: allowed}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rid, err := (credentials.Generator{}).ID()
	if err != nil {
		fail(w, 500, "internal_error")
		return
	}
	w.Header().Set("X-Request-ID", rid)
	w.Header().Set("X-Prototype-Safety", booking.SafetyBanner)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			fail(w, 403, "origin_denied")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	const base = "/api/v1/local/booking-trial"
	path := strings.TrimSuffix(r.URL.Path, "/")
	if r.URL.RawQuery != "" && path != base+"/catalog" && !strings.HasSuffix(path, "/messages") {
		fail(w, 400, "invalid_request")
		return
	}
	principal, e := h.auth.Authorize(r.Context(), identity.Secret(bearer(r.Header.Get("Authorization"))), "", identity.UserOperation)
	if e != nil {
		if errors.Is(e, identity.ErrForbidden) {
			fail(w, 403, "forbidden")
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, 401, "unauthenticated")
		}
		return
	}
	actor := principal.AccountID
	if path == base+"/fixture" && r.Method == http.MethodGet {
		v, e := h.service.Fixture(r.Context(), actor)
		h.reply(w, v, e)
		return
	}
	if path == base+"/catalog" && r.Method == http.MethodGet {
		filter, ok := catalogFilter(r.URL.Query())
		pageSize, cursor, pageOK := catalogPageParams(r.URL.Query())
		if !ok || !pageOK {
			fail(w, 422, "invalid_request")
			return
		}
		page, e := h.service.CatalogPage(r.Context(), actor, filter, pageSize, cursor)
		result := map[string]any{"items": page.Items, "safety_notice": booking.SafetyBanner}
		if page.NextCursor != "" {
			result["next_cursor"] = page.NextCursor
		}
		h.reply(w, result, e)
		return
	}
	if strings.HasPrefix(path, base+"/catalog/") && r.Method == http.MethodGet {
		spaceID := strings.TrimPrefix(path, base+"/catalog/")
		if strings.Contains(spaceID, "/") || spaceID == "" {
			fail(w, 404, "not_found")
			return
		}
		item, e := h.service.CatalogDetail(r.Context(), actor, spaceID)
		h.reply(w, item, e)
		return
	}
	if path == base+"/quotes" && r.Method == http.MethodPost {
		var in booking.QuoteInput
		if !decode(w, r, &in) {
			return
		}
		v, e := h.service.Quote(r.Context(), actor, in)
		h.reply(w, v, e)
		return
	}
	if path == base+"/reservations" {
		if r.Method == http.MethodGet {
			v, e := h.service.List(r.Context(), actor)
			h.reply(w, map[string]any{"items": v, "safety_notice": booking.SafetyBanner}, e)
			return
		}
		if r.Method == http.MethodPost {
			var in booking.RequestInput
			if !decode(w, r, &in) {
				return
			}
			v, e := h.service.Request(r.Context(), actor, in, r.Header.Get("Idempotency-Key"))
			if e != nil {
				h.reply(w, nil, e)
				return
			}
			write(w, 201, map[string]any{"data": v, "safety_notice": booking.SafetyBanner})
			return
		}
	}
	if strings.HasPrefix(path, base+"/reservations/") {
		parts := strings.Split(strings.TrimPrefix(path, base+"/reservations/"), "/")
		if len(parts) == 1 && r.Method == http.MethodGet {
			v, e := h.service.Get(r.Context(), actor, parts[0])
			h.reply(w, v, e)
			return
		}
		if len(parts) == 3 && parts[0] != "" && parts[1] == "messages" && parts[2] == "read" && r.Method == http.MethodPost {
			if h.conversation == nil {
				fail(w, 404, "not_found")
				return
			}
			var in struct {
				ThroughSequence int64 `json:"through_sequence"`
			}
			if !decode(w, r, &in) {
				return
			}
			cursor, err := h.conversation.MarkRead(r.Context(), actor, parts[0], in.ThroughSequence)
			h.replyConversation(w, map[string]any{"read_through_sequence": cursor}, err)
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "messages" {
			if h.conversation == nil {
				fail(w, 404, "not_found")
				return
			}
			if r.Method == http.MethodGet {
				before, limit, ok := messagePageQuery(r.URL.Query())
				if !ok {
					fail(w, 422, "invalid_request")
					return
				}
				page, err := h.conversation.List(r.Context(), actor, parts[0], before, limit)
				h.replyConversation(w, map[string]any{"items": page.Items, "older_cursor": page.OlderCursor}, err)
				return
			}
			if r.Method == http.MethodPost {
				var in struct {
					Body string `json:"body"`
				}
				if !decode(w, r, &in) {
					return
				}
				item, err := h.conversation.Send(r.Context(), actor, parts[0], r.Header.Get("Idempotency-Key"), in.Body)
				if err != nil {
					h.replyConversation(w, nil, err)
					return
				}
				write(w, http.StatusCreated, map[string]any{"data": item, "safety_notice": booking.SafetyBanner})
				return
			}
		}
		if len(parts) == 2 && parts[0] != "" {
			switch parts[1] {
			case "payment":
				if r.Method == http.MethodPost {
					var in booking.PaymentInput
					if !decode(w, r, &in) {
						return
					}
					v, e := h.service.Pay(r.Context(), actor, parts[0], in.Outcome, r.Header.Get("Idempotency-Key"))
					h.reply(w, v, e)
					return
				}
			case "decision":
				if r.Method == http.MethodPost {
					var in booking.DecisionInput
					if !decode(w, r, &in) {
						return
					}
					v, e := h.service.Decide(r.Context(), actor, parts[0], in.Decision)
					h.reply(w, v, e)
					return
				}
			case "cancel":
				if r.Method == http.MethodPost {
					if !emptyBody(r) {
						fail(w, 400, "invalid_request")
						return
					}
					v, e := h.service.Cancel(r.Context(), actor, parts[0])
					h.reply(w, v, e)
					return
				}
			}
		}
	}
	fail(w, 404, "not_found")
}

func messagePageQuery(query url.Values) (*int64, int, bool) {
	for key, values := range query {
		if (key != "before" && key != "limit") || len(values) != 1 || values[0] == "" {
			return nil, 0, false
		}
	}
	limit := conversation.DefaultPageSize
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > conversation.MaxPageSize {
			return nil, 0, false
		}
		limit = parsed
	}
	var before *int64
	if raw := query.Get("before"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			return nil, 0, false
		}
		before = &parsed
	}
	return before, limit, true
}

func (h *Handler) replyConversation(w http.ResponseWriter, value any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, conversation.ErrInvalid):
			fail(w, http.StatusUnprocessableEntity, "invalid_request")
		case errors.Is(err, conversation.ErrNotFound):
			fail(w, http.StatusNotFound, "not_found")
		case errors.Is(err, conversation.ErrConflict):
			fail(w, http.StatusConflict, "conflict")
		default:
			fail(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}
	write(w, http.StatusOK, map[string]any{"data": value, "safety_notice": booking.SafetyBanner})
}

func catalogFilter(query url.Values) (booking.CatalogFilter, bool) {
	for key, values := range query {
		if (key != "category_code" && key != "start_at" && key != "end_at" && key != "min_total_clp" && key != "max_total_clp" && key != "profile_version" && key != "attributes" && key != "latitude" && key != "longitude" && key != "radius_km" && key != "page_size" && key != "cursor") || len(values) != 1 || (values[0] == "" && key != "latitude" && key != "longitude" && key != "radius_km") {
			return booking.CatalogFilter{}, false
		}
	}
	filter := booking.CatalogFilter{CategoryCode: query.Get("category_code")}
	startRaw, endRaw := query.Get("start_at"), query.Get("end_at")
	if (startRaw == "") != (endRaw == "") {
		return booking.CatalogFilter{}, false
	}
	if startRaw != "" {
		start, err := time.Parse(time.RFC3339Nano, startRaw)
		if err != nil {
			return booking.CatalogFilter{}, false
		}
		end, err := time.Parse(time.RFC3339Nano, endRaw)
		if err != nil || !end.After(start) {
			return booking.CatalogFilter{}, false
		}
		filter.StartAt, filter.EndAt = &start, &end
	}
	for key, dest := range map[string]**int64{"min_total_clp": &filter.MinTotalCLP, "max_total_clp": &filter.MaxTotalCLP} {
		if raw := query.Get(key); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return booking.CatalogFilter{}, false
			}
			*dest = &value
		}
	}
	if raw := query.Get("profile_version"); raw != "" {
		version, err := strconv.Atoi(raw)
		if err != nil || version < 1 {
			return booking.CatalogFilter{}, false
		}
		filter.ProfileVersion = version
	}
	if raw := query.Get("attributes"); raw != "" {
		if len(raw) > 16*1024 || json.Unmarshal([]byte(raw), &filter.Attributes) != nil || filter.Attributes == nil {
			return booking.CatalogFilter{}, false
		}
	}
	for key, dest := range map[string]**float64{"latitude": &filter.Latitude, "longitude": &filter.Longitude} {
		if values, present := query[key]; present {
			value, err := strconv.ParseFloat(values[0], 64)
			if values[0] == "" || err != nil {
				value = math.NaN()
			}
			*dest = &value
		}
	}
	if values, present := query["radius_km"]; present {
		radius, err := strconv.Atoi(values[0])
		if values[0] == "" || err != nil {
			radius = 0
		}
		filter.RadiusKM = &radius
	}
	return filter, true
}
func catalogPageParams(query url.Values) (int, string, bool) {
	pageSize := 0
	if raw, ok := query["page_size"]; ok {
		if len(raw) != 1 {
			return 0, "", false
		}
		value, err := strconv.Atoi(raw[0])
		if err != nil || value < 1 || value > 25 {
			return 0, "", false
		}
		pageSize = value
	}
	cursor := query.Get("cursor")
	if raw, ok := query["cursor"]; ok && len(raw) != 1 {
		return 0, "", false
	}
	if len(cursor) > 8192 {
		return 0, "", false
	}
	return pageSize, cursor, true
}
func (h *Handler) reply(w http.ResponseWriter, v any, err error) {
	if err != nil {
		if errors.Is(err, booking.ErrSimulatedNoResponse) {
			write(w, http.StatusGatewayTimeout, map[string]any{"data": v, "error": map[string]string{"code": "simulated_payment_timeout", "message": "El adaptador local no entregó respuesta; la reserva sigue pendiente."}, "safety_notice": booking.SafetyBanner})
			return
		}
		switch {
		case errors.Is(err, booking.ErrInvalid):
			fail(w, 422, "invalid_request")
		case errors.Is(err, booking.ErrNotFound):
			fail(w, 404, "not_found")
		case errors.Is(err, booking.ErrConflict):
			fail(w, 409, "conflict")
		default:
			fail(w, 500, "internal_error")
		}
		return
	}
	write(w, 200, map[string]any{"data": v, "safety_notice": booking.SafetyBanner})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		fail(w, 415, "unsupported_media_type")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		fail(w, 400, "invalid_request")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		fail(w, 400, "invalid_request")
		return false
	}
	return true
}
func emptyBody(r *http.Request) bool {
	var v any
	d := json.NewDecoder(io.LimitReader(r.Body, 1024))
	return d.Decode(&v) == io.EOF
}
func bearer(v string) string {
	p := strings.SplitN(v, " ", 2)
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
		return ""
	}
	return p[1]
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar el ensayo local."}, "safety_notice": booking.SafetyBanner})
}
