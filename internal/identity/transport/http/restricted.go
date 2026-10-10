package identityhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type restrictedAuthorizer interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

// RestrictedAccountMiddleware enforces the account-block allow-list before a
// domain route can perform work. Normal accounts pass through unchanged.
func RestrictedAccountMiddleware(auth restrictedAuthorizer, next http.Handler, allowedOrigins ...string) http.Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || next == nil || !strings.HasPrefix(r.URL.Path, "/api/v1/") || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			next.ServeHTTP(w, r)
			return
		}
		principal, err := auth.Authorize(r.Context(), identity.Secret(parts[1]), "", identity.AutomaticPolling)
		if err != nil || !principal.RestrictedMode {
			next.ServeHTTP(w, r)
			return
		}
		if !restrictedReservationRoute(r.Method, r.URL.Path) {
			restrictedError(w, r, origins, http.StatusForbidden, "restricted_account")
			return
		}
		r = r.WithContext(identity.WithRestrictedReservationAccess(r.Context(), principal.AccountID))
		next.ServeHTTP(w, r)
	})
}

func restrictedReservationRoute(method, path string) bool {
	const base = "/api/v1/local/booking-trial"
	path = strings.TrimSuffix(path, "/")
	if method == http.MethodGet && path == "/api/v1/auth/session" || method == http.MethodPost && path == "/api/v1/auth/logout" {
		return true
	}
	if strings.HasPrefix(path, base+"/contracts/") {
		parts := strings.Split(strings.TrimPrefix(path, base+"/contracts/"), "/")
		if len(parts) == 1 {
			return method == http.MethodGet && parts[0] != ""
		}
		if len(parts) == 2 && parts[0] != "" && (parts[1] == "sign" || parts[1] == "reject") {
			return method == http.MethodPost
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "document" {
			return method == http.MethodGet
		}
		return false
	}
	const disputeHistory = base + "/disputes/"
	if strings.HasPrefix(path, disputeHistory) && method == http.MethodGet {
		parts := strings.Split(strings.TrimPrefix(path, disputeHistory), "/")
		return len(parts) == 2 && parts[0] != "" && parts[1] == "history"
	}
	if path == base+"/reservations" {
		return method == http.MethodGet
	}
	const prefix = base + "/reservations/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	if len(parts) == 1 && parts[0] != "" {
		return method == http.MethodGet
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "payment", "decision", "cancel", "refund", "check-in", "check-out", "reception", "contract", "damage-claim", "disputes":
			return method == http.MethodPost || (method == http.MethodGet && (parts[1] == "damage-claim" || parts[1] == "disputes"))
		case "cancellation-preview", "operations", "guarantee":
			return method == http.MethodGet || (method == http.MethodPost && parts[1] == "guarantee")
		case "messages":
			return method == http.MethodGet || method == http.MethodPost
		}
	}
	if len(parts) == 3 && parts[1] == "messages" && parts[2] == "read" {
		return method == http.MethodPost
	}
	if len(parts) == 3 && parts[1] == "damage-claim" && parts[2] == "defense" {
		return method == http.MethodPost
	}
	if len(parts) == 3 && parts[1] == "disputes" {
		return method == http.MethodGet || method == http.MethodPost
	}
	if len(parts) == 3 && parts[1] == "evidence" {
		return method == http.MethodGet && parts[0] != "" && parts[2] != ""
	}
	return false
}

func restrictedError(w http.ResponseWriter, r *http.Request, allowedOrigins map[string]struct{}, status int, code string) {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		id = "unavailable"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", id)
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if _, allowed := allowedOrigins[origin]; allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "Esta cuenta solo puede operar sobre reservas existentes y según sus permisos actuales.", "request_id": id}})
}
