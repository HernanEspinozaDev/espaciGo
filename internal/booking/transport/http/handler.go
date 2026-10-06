package bookinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *booking.Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, service *booking.Service, origins []string) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return &Handler{auth: auth, service: service, origins: allowed}
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
	if r.URL.RawQuery != "" {
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
	const base = "/api/v1/local/booking-trial"
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == base+"/fixture" && r.Method == http.MethodGet {
		v, e := h.service.Fixture(r.Context(), actor)
		h.reply(w, v, e)
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
